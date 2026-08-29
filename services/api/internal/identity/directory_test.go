package identity_test

import (
	"context"
	"testing"

	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

func register(t *testing.T, s *identity.Service, name, email string) identity.Org {
	t.Helper()
	org, _, err := s.Register(context.Background(), identity.RegisterInput{
		OrgName: name, Email: email, Password: "correcte-agrafe-cheval-pile",
		FirstName: "Marie", LastName: "Vulcain",
	})
	if err != nil {
		t.Fatalf("inscription: %v", err)
	}
	return org
}

// Une organisation inscrite doit apparaître à l'annuaire tout de suite : celle
// qui n'y figure pas est invisible du support, donc introuvable le jour où
// elle appelle.
func TestRegisteredOrgAppearsInTheDirectory(t *testing.T) {
	s := identity.NewService(ddb.NewTestClient(t), nil)
	org := register(t, s, "Vulcain Formation", "marie@vulcain.fr")

	entries, err := s.ListOrgs(context.Background())
	if err != nil {
		t.Fatalf("annuaire: %v", err)
	}
	if len(entries) != 1 || entries[0].OrgID != org.ID {
		t.Fatalf("annuaire = %+v", entries)
	}
	if entries[0].Owner != "marie@vulcain.fr" || entries[0].Plan != "trial" {
		t.Errorf("fiche = %+v", entries[0])
	}
}

// L'annuaire se répare de lui-même : une organisation créée avant qu'il
// n'existe y entre à la première connexion, sans migration.
func TestDirectoryHealsWithoutMigration(t *testing.T) {
	db := ddb.NewTestClient(t)
	s := identity.NewService(db, nil)
	org := register(t, s, "Vulcain Formation", "marie@vulcain.fr")

	if err := ddb.Delete(context.Background(), db, identity.DirectoryPK, "ORG#"+org.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureDirectory(context.Background(), org.ID, "marie@vulcain.fr"); err != nil {
		t.Fatalf("réparation: %v", err)
	}

	entries, err := s.ListOrgs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("annuaire = %d fiches", len(entries))
	}

	// Deux appels ne créent pas deux fiches : la connexion suivante ne doit
	// pas dupliquer l'organisation.
	if err := s.EnsureDirectory(context.Background(), org.ID, "marie@vulcain.fr"); err != nil {
		t.Fatal(err)
	}
	if entries, _ := s.ListOrgs(context.Background()); len(entries) != 1 {
		t.Fatalf("annuaire = %d fiches après un second passage", len(entries))
	}
}

// Un changement de formule doit se voir des deux côtés : la fiche de
// l'organisation et l'annuaire du support.
func TestPlanChangeReachesTheDirectory(t *testing.T) {
	s := identity.NewService(ddb.NewTestClient(t), nil)
	org := register(t, s, "Vulcain Formation", "marie@vulcain.fr")

	if _, err := s.SetPlan(context.Background(), org.ID, "structure"); err != nil {
		t.Fatalf("changement de formule: %v", err)
	}

	reloaded, err := s.LoadOrg(context.Background(), org.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Plan != "structure" {
		t.Errorf("formule de l'organisation = %q", reloaded.Plan)
	}
	entries, _ := s.ListOrgs(context.Background())
	if entries[0].Plan != "structure" {
		t.Errorf("formule à l'annuaire = %q", entries[0].Plan)
	}
	// La date d'inscription ne doit pas bouger : c'est elle qui dit depuis
	// quand le client est client.
	if !entries[0].CreatedAt.Equal(org.CreatedAt) {
		t.Errorf("date d'inscription réécrite : %s au lieu de %s",
			entries[0].CreatedAt, org.CreatedAt)
	}
}

// Une impersonation ne peut pas être discrète : la session porte le nom de son
// auteur, et c'est le seul garde-fou qui tienne quand un accès total est
// techniquement nécessaire au support.
func TestImpersonationNamesItsAuthor(t *testing.T) {
	s := identity.NewService(ddb.NewTestClient(t), nil)
	org := register(t, s, "Vulcain Formation", "marie@vulcain.fr")

	owner, err := s.FirstOwner(context.Background(), org.ID)
	if err != nil {
		t.Fatalf("propriétaire: %v", err)
	}

	if _, err := s.Impersonate(context.Background(), owner, "", "", "127.0.0.1", "test"); err == nil {
		t.Error("une impersonation anonyme a été acceptée")
	}

	token, err := s.Impersonate(context.Background(), owner,
		"USR-support", "equipe@lemlearn.fr", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("impersonation: %v", err)
	}
	session, err := s.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("session ouverte inutilisable: %v", err)
	}
	if session.ImpersonatedBy != "USR-support" {
		t.Errorf("session.ImpersonatedBy = %q", session.ImpersonatedBy)
	}
	if session.ImpersonatorEmail != "equipe@lemlearn.fr" {
		t.Errorf("session.ImpersonatorEmail = %q : sans elle, on ne peut pas ressortir",
			session.ImpersonatorEmail)
	}
	if session.OrgID != org.ID {
		t.Errorf("session ouverte sur %q au lieu de %q", session.OrgID, org.ID)
	}
}

// Une session ordinaire ne porte aucune marque d'impersonation. Sans cela, un
// apprenant qui vient d'accepter son invitation verrait un bandeau lui
// annonçant que l'équipe agit à sa place.
func TestOrdinarySessionCarriesNoImpersonation(t *testing.T) {
	s := identity.NewService(ddb.NewTestClient(t), nil)
	org := register(t, s, "Vulcain Formation", "marie@vulcain.fr")
	owner, err := s.FirstOwner(context.Background(), org.ID)
	if err != nil {
		t.Fatalf("propriétaire: %v", err)
	}

	token, err := s.OpenSession(context.Background(), owner, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	session, err := s.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("session inutilisable: %v", err)
	}
	if session.ImpersonatedBy != "" || session.ImpersonatorEmail != "" {
		t.Errorf("session ordinaire marquée comme impersonation : %q / %q",
			session.ImpersonatedBy, session.ImpersonatorEmail)
	}
}

// On ne peut pas sortir d'une session qui n'est pas une impersonation : sinon
// n'importe quel client pourrait demander à « revenir » vers un compte de
// l'équipe.
func TestEndImpersonationRefusesAnOrdinarySession(t *testing.T) {
	s := identity.NewService(ddb.NewTestClient(t), nil)
	org := register(t, s, "Vulcain Formation", "marie@vulcain.fr")
	owner, _ := s.FirstOwner(context.Background(), org.ID)

	token, err := s.OpenSession(context.Background(), owner, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	session, _ := s.Authenticate(context.Background(), token)

	if _, _, err := s.EndImpersonation(context.Background(), session, token, "127.0.0.1", "test"); err == nil {
		t.Error("une session ordinaire a pu « revenir » vers un compte de l'équipe")
	}
}

// Les faits commerciaux vont sur la chaîne de l'organisation, pas sur celle
// d'un dossier : les mélanger fausserait l'export du dossier probatoire.
func TestOrgAuditIsChainedApartFromFiles(t *testing.T) {
	s := identity.NewService(ddb.NewTestClient(t), nil)
	org := register(t, s, "Vulcain Formation", "marie@vulcain.fr")

	actor := audit.Actor{Type: audit.ActorUser, ID: "USR-support"}
	first, err := s.AuditOrg(context.Background(), org.ID, audit.ActionPlanChanged, actor,
		map[string]any{"avant": "trial", "apres": "essentiel"})
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	second, err := s.AuditOrg(context.Background(), org.ID, audit.ActionImpersonated, actor, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.PrevHash != first.Hash {
		t.Error("la chaîne de l'organisation est rompue")
	}
}
