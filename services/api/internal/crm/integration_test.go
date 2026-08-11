package crm_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

type fixture struct {
	db       *ddb.Client
	identity *identity.Service
	crm      *crm.Service
	org      identity.Org
	owner    identity.User
	actor    audit.Actor
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	db := ddb.NewTestClient(t)
	ident := identity.NewService(db, nil)
	service := crm.NewService(db, nil)

	org, owner, err := ident.Register(context.Background(), identity.RegisterInput{
		OrgName: "Institut Vulcain", Email: "marie@vulcain.fr",
		Password: "correcte-agrafe-cheval-pile", FirstName: "Marie", LastName: "Dubreuil",
		IP: "82.65.14.3", UserAgent: "test",
	})
	if err != nil {
		t.Fatalf("inscription: %v", err)
	}

	return fixture{
		db: db, identity: ident, crm: service, org: org, owner: owner,
		actor: audit.Actor{Type: audit.ActorUser, ID: owner.ID, Label: owner.FullName(), IP: "82.65.14.3"},
	}
}

// Le parcours nominal : inscription, connexion, ouverture d'un dossier,
// déplacement dans le pipeline, relecture du journal.
func TestFileLifecycle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	file, err := f.crm.CreateFile(ctx, crm.CreateFileInput{
		OrgID: f.org.ID, Title: "SSIAP 1 — session de février",
		PriceHT: 1250, Actor: f.actor,
	})
	if err != nil {
		t.Fatalf("création du dossier: %v", err)
	}
	if file.Stage != crm.StageProspect {
		t.Errorf("étape initiale = %q, attendu prospect", file.Stage)
	}
	if file.Proof.Expected != len(crm.RequiredProofs) || file.Proof.Present != 0 {
		t.Errorf("complétude initiale = %d/%d, attendu 0/%d",
			file.Proof.Present, file.Proof.Expected, len(crm.RequiredProofs))
	}

	for _, stage := range []crm.Stage{crm.StageQuote, crm.StageAgreement, crm.StageInTraining} {
		if _, err := f.crm.MoveFile(ctx, f.org.ID, file.ID, stage, f.actor); err != nil {
			t.Fatalf("déplacement vers %s: %v", stage, err)
		}
	}

	reloaded, err := f.crm.GetFile(ctx, f.org.ID, file.ID)
	if err != nil {
		t.Fatalf("relecture: %v", err)
	}
	if reloaded.Stage != crm.StageInTraining {
		t.Errorf("étape finale = %q, attendu in_training", reloaded.Stage)
	}

	// Le dossier doit apparaître dans sa colonne, et seulement dans celle-là.
	inTraining, err := f.crm.ListFilesByStage(ctx, f.org.ID, crm.StageInTraining, 10)
	if err != nil {
		t.Fatalf("liste: %v", err)
	}
	if len(inTraining) != 1 || inTraining[0].ID != file.ID {
		t.Errorf("colonne « en formation » : %d dossier(s), attendu le nôtre", len(inTraining))
	}
	prospects, err := f.crm.ListFilesByStage(ctx, f.org.ID, crm.StageProspect, 10)
	if err != nil {
		t.Fatalf("liste: %v", err)
	}
	if len(prospects) != 0 {
		t.Errorf("le dossier est resté dans la colonne « prospect »")
	}
}

// Le test central de ce paquet : le journal doit rester vérifiable après un
// aller-retour complet par DynamoDB. Un encodage qui perdrait une nanoseconde
// ou réordonnerait une charge utile casserait les empreintes sans que rien ne
// le signale avant un audit.
func TestAuditChainSurvivesRoundTrip(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	file, err := f.crm.CreateFile(ctx, crm.CreateFileInput{
		OrgID: f.org.ID, Title: "Habilitation électrique B1V", PriceHT: 890, Actor: f.actor,
	})
	if err != nil {
		t.Fatalf("création: %v", err)
	}
	for _, stage := range []crm.Stage{crm.StageQuote, crm.StageAgreement, crm.StageInTraining, crm.StageClosed} {
		if _, err := f.crm.MoveFile(ctx, f.org.ID, file.ID, stage, f.actor); err != nil {
			t.Fatalf("déplacement: %v", err)
		}
	}

	// Timeline vérifie la chaîne : une erreur ici signifie que les empreintes
	// recalculées après relecture ne correspondent plus.
	events, err := f.crm.Timeline(ctx, file.ID)
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("%d événements, attendu 5 (création + 4 déplacements)", len(events))
	}
	if events[0].Action != audit.ActionFileCreated {
		t.Errorf("premier événement = %q", events[0].Action)
	}
	if got := events[4].Payload["to"]; got != string(crm.StageClosed) {
		t.Errorf("dernier déplacement vers %v, attendu closed", got)
	}
	// La vérification est refaite explicitement pour que l'échec pointe ici
	// et pas dans la couche d'accès.
	if err := audit.Verify(events); err != nil {
		t.Fatalf("chaîne relue invalide: %v", err)
	}
}

// Deux organisations ne doivent jamais se voir. L'isolation ne repose pas sur
// un filtre applicatif mais sur la clé de partition : la requête d'un client
// n'atteint pas la partition d'un autre.
func TestTenantIsolation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	other, _, err := f.identity.Register(ctx, identity.RegisterInput{
		OrgName: "Formation Concurrente", Email: "contact@concurrent.fr",
		Password: "autre-mot-de-passe-long", FirstName: "Paul", LastName: "Roux",
	})
	if err != nil {
		t.Fatalf("seconde organisation: %v", err)
	}

	file, err := f.crm.CreateFile(ctx, crm.CreateFileInput{
		OrgID: f.org.ID, Title: "Dossier confidentiel", Actor: f.actor,
	})
	if err != nil {
		t.Fatalf("création: %v", err)
	}

	if _, err := f.crm.GetFile(ctx, other.ID, file.ID); !errors.Is(err, ddb.ErrNotFound) {
		t.Fatalf("une autre organisation a pu lire le dossier: %v", err)
	}

	files, err := f.crm.ListFilesByStage(ctx, other.ID, crm.StageProspect, 10)
	if err != nil {
		t.Fatalf("liste: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("la seconde organisation voit %d dossier(s) qui ne lui appartiennent pas", len(files))
	}
}

// Une adresse e-mail ne peut être réservée qu'une fois, y compris entre deux
// organisations distinctes.
func TestEmailUniquenessAcrossOrgs(t *testing.T) {
	f := newFixture(t)

	_, _, err := f.identity.Register(context.Background(), identity.RegisterInput{
		OrgName: "Autre organisme", Email: "MARIE@vulcain.fr", // casse différente
		Password: "un-autre-mot-de-passe", FirstName: "Marie", LastName: "Dubreuil",
	})
	if !errors.Is(err, identity.ErrEmailTaken) {
		t.Fatalf("adresse déjà prise acceptée: %v", err)
	}
}

// Connexion, session, révocation.
func TestLoginAndLogout(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	user, token, err := f.identity.Login(ctx, "marie@vulcain.fr", "correcte-agrafe-cheval-pile", "1.2.3.4", "test")
	if err != nil {
		t.Fatalf("connexion: %v", err)
	}
	if user.ID != f.owner.ID {
		t.Errorf("utilisateur inattendu")
	}

	session, err := f.identity.Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("authentification: %v", err)
	}
	if session.OrgID != f.org.ID || session.Role != identity.RoleOwner {
		t.Errorf("session incohérente: %+v", session)
	}

	if err := f.identity.Logout(ctx, token); err != nil {
		t.Fatalf("déconnexion: %v", err)
	}
	if _, err := f.identity.Authenticate(ctx, token); !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Fatalf("le jeton révoqué est encore accepté: %v", err)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	f := newFixture(t)

	if _, _, err := f.identity.Login(context.Background(), "marie@vulcain.fr", "mauvais-mot-de-passe", "", ""); !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Fatalf("mot de passe faux accepté: %v", err)
	}
	// Un e-mail inconnu doit produire exactement la même erreur, sinon on
	// peut énumérer les comptes existants.
	if _, _, err := f.identity.Login(context.Background(), "inconnu@nulle-part.fr", "peu-importe-le-mot", "", ""); !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Fatalf("e-mail inconnu distingué du mot de passe faux: %v", err)
	}
}

// Déplacements concurrents : un seul doit réussir, et la chaîne d'audit doit
// rester intègre — pas d'événement écrasé, pas de rang en double.
func TestConcurrentMovesKeepChainIntact(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	file, err := f.crm.CreateFile(ctx, crm.CreateFileInput{
		OrgID: f.org.ID, Title: "Dossier disputé", Actor: f.actor,
	})
	if err != nil {
		t.Fatalf("création: %v", err)
	}

	var wg sync.WaitGroup
	results := make([]error, 4)
	for i, stage := range []crm.Stage{crm.StageQuote, crm.StageAgreement, crm.StageInTraining, crm.StageClosed} {
		wg.Add(1)
		go func(i int, stage crm.Stage) {
			defer wg.Done()
			_, results[i] = f.crm.MoveFile(ctx, f.org.ID, file.ID, stage, f.actor)
		}(i, stage)
	}
	wg.Wait()

	succeeded := 0
	for _, err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded == 0 {
		t.Fatalf("aucun déplacement n'a abouti: %v", results)
	}

	events, err := f.crm.Timeline(ctx, file.ID)
	if err != nil {
		t.Fatalf("chaîne rompue par la concurrence: %v", err)
	}
	if len(events) != succeeded+1 {
		t.Errorf("%d événements pour %d déplacement(s) réussi(s) + création", len(events), succeeded)
	}
}

// Un contact doit être retrouvable dans la liste de sa nature, et pas dans
// celle des autres.
func TestContactIndexing(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	learner := crm.NewContact(f.org.ID, crm.KindLearner, time.Now().UTC())
	learner.FirstName = "Léa"
	learner.LastName = "Bertrand"
	learner.Email = "lea.bertrand@example.fr"
	learner.BirthDate = "1991-04-17"
	if _, err := f.crm.CreateContact(ctx, learner); err != nil {
		t.Fatalf("création de l'apprenant: %v", err)
	}

	company := crm.NewContact(f.org.ID, crm.KindCompany, time.Now().UTC())
	company.CompanyName = "Groupe Aramis"
	company.SIRET = "51203847600024"
	if _, err := f.crm.CreateContact(ctx, company); err != nil {
		t.Fatalf("création de l'entreprise: %v", err)
	}

	learners, err := f.crm.ListContacts(ctx, f.org.ID, crm.KindLearner, 10)
	if err != nil {
		t.Fatalf("liste: %v", err)
	}
	if len(learners) != 1 || learners[0].DisplayName() != "Léa Bertrand" {
		t.Errorf("liste des apprenants inattendue: %+v", learners)
	}

	funders, err := f.crm.ListContacts(ctx, f.org.ID, crm.KindFunder, 10)
	if err != nil {
		t.Fatalf("liste: %v", err)
	}
	if len(funders) != 0 {
		t.Errorf("un contact apparaît dans la mauvaise liste")
	}
}

func TestContactValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	invalid := crm.NewContact(f.org.ID, crm.KindLearner, time.Now().UTC())
	invalid.FirstName = "Léa" // nom manquant
	if _, err := f.crm.CreateContact(ctx, invalid); err == nil {
		t.Error("un apprenant sans nom a été accepté")
	}

	badDate := crm.NewContact(f.org.ID, crm.KindLearner, time.Now().UTC())
	badDate.LastName = "Bertrand"
	badDate.BirthDate = "17/04/1991"
	if _, err := f.crm.CreateContact(ctx, badDate); err == nil {
		t.Error("une date de naissance au mauvais format a été acceptée")
	}
}
