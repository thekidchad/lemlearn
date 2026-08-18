package crm_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

func learnerWithFile(t *testing.T, f fixture) (crm.Contact, crm.File) {
	t.Helper()
	ctx := context.Background()

	contact := crm.NewContact(f.org.ID, crm.KindLearner, time.Now().UTC())
	contact.FirstName, contact.LastName = "Léa", "Bertrand"
	contact.Email, contact.Phone = "lea.bertrand@example.fr", "0612345678"
	contact.BirthDate, contact.BirthPlace = "1991-04-17", "Lyon"
	contact.Address = crm.Address{Line1: "8 avenue Foch", PostalCode: "69006", City: "Lyon"}
	contact.IdentityDocKey = "identity/ORG1/piece.jpg"
	contact, err := f.crm.CreateContact(ctx, contact)
	if err != nil {
		t.Fatalf("apprenant: %v", err)
	}

	file, err := f.crm.CreateFile(ctx, crm.CreateFileInput{
		OrgID: f.org.ID, Title: "SSIAP 1", LearnerID: contact.ID, Actor: f.actor,
	})
	if err != nil {
		t.Fatalf("dossier: %v", err)
	}
	return contact, file
}

// L'effacement doit vider toute donnée personnelle, sans exception oubliée.
func TestAnonymizeErasesEveryPersonalField(t *testing.T) {
	f := newFixture(t)
	contact, _ := learnerWithFile(t, f)

	after, err := f.crm.Anonymize(context.Background(), crm.AnonymizeInput{
		OrgID: f.org.ID, ContactID: contact.ID,
		Reason: "demande de la personne", Actor: f.actor,
	})
	if err != nil {
		t.Fatalf("anonymisation: %v", err)
	}

	for name, value := range map[string]string{
		"prénom":            after.FirstName,
		"courriel":          after.Email,
		"téléphone":         after.Phone,
		"date de naissance": after.BirthDate,
		"lieu de naissance": after.BirthPlace,
		"adresse":           after.Address.Line1,
		"ville":             after.Address.City,
		"pièce d'identité":  after.IdentityDocKey,
		"notes":             after.Notes,
	} {
		if value != "" {
			t.Errorf("le champ %s subsiste : %q", name, value)
		}
	}
	if !after.Anonymized {
		t.Error("le contact n'est pas marqué anonymisé")
	}
	if !strings.HasPrefix(after.LastName, "Apprenant anonymisé") {
		t.Errorf("pseudonyme inattendu : %q", after.LastName)
	}
}

// Le pseudonyme doit être stable : deux exports du même dossier à un an
// d'intervalle doivent désigner la même personne, ce qu'un contrôleur recoupe.
func TestPseudonymIsStableAndDistinct(t *testing.T) {
	first := crm.Pseudonym("CONTACT-A")
	if first != crm.Pseudonym("CONTACT-A") {
		t.Error("le pseudonyme d'un même contact varie")
	}
	if first == crm.Pseudonym("CONTACT-B") {
		t.Error("deux contacts partagent le même pseudonyme")
	}
}

// L'effacement est un acte de gestion : il doit figurer au journal du dossier,
// avec son motif, et sans y recopier les données effacées.
func TestAnonymizeIsAuditedWithoutLeaking(t *testing.T) {
	f := newFixture(t)
	contact, file := learnerWithFile(t, f)
	ctx := context.Background()

	if _, err := f.crm.Anonymize(ctx, crm.AnonymizeInput{
		OrgID: f.org.ID, ContactID: contact.ID,
		Reason: "expiration de la durée de conservation", Actor: f.actor,
	}); err != nil {
		t.Fatalf("anonymisation: %v", err)
	}

	events, err := f.crm.Timeline(ctx, file.ID)
	if err != nil {
		t.Fatalf("journal: %v", err)
	}

	var found bool
	for _, event := range events {
		if event.Action != audit.ActionLearnerAnonymized {
			continue
		}
		found = true
		if event.Payload["motif"] != "expiration de la durée de conservation" {
			t.Errorf("motif absent du journal : %v", event.Payload["motif"])
		}
		// Le journal nomme les catégories effacées, jamais leurs valeurs :
		// les y recopier reviendrait à les conserver.
		blob := strings.ToLower(dumpPayload(event.Payload))
		for _, secret := range []string{"lea.bertrand@example.fr", "0612345678", "1991-04-17", "avenue foch"} {
			if strings.Contains(blob, secret) {
				t.Errorf("une donnée effacée subsiste au journal : %s", secret)
			}
		}
	}
	if !found {
		t.Fatal("l'anonymisation ne figure pas au journal du dossier")
	}
	if err := audit.Verify(events); err != nil {
		t.Fatalf("chaîne rompue: %v", err)
	}
}

// Anonymiser deux fois ne doit ni échouer ni réécrire un pseudonyme différent.
func TestAnonymizeIsIdempotent(t *testing.T) {
	f := newFixture(t)
	contact, _ := learnerWithFile(t, f)
	ctx := context.Background()

	first, err := f.crm.Anonymize(ctx, crm.AnonymizeInput{
		OrgID: f.org.ID, ContactID: contact.ID, Reason: "demande", Actor: f.actor,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.crm.Anonymize(ctx, crm.AnonymizeInput{
		OrgID: f.org.ID, ContactID: contact.ID, Reason: "demande", Actor: f.actor,
	})
	if err != nil {
		t.Fatalf("seconde anonymisation: %v", err)
	}
	if first.LastName != second.LastName {
		t.Errorf("le pseudonyme a changé : %q puis %q", first.LastName, second.LastName)
	}
}

// Un effacement sans motif n'est pas traçable : il est refusé.
func TestAnonymizeRequiresReason(t *testing.T) {
	f := newFixture(t)
	contact, _ := learnerWithFile(t, f)

	if _, err := f.crm.Anonymize(context.Background(), crm.AnonymizeInput{
		OrgID: f.org.ID, ContactID: contact.ID, Actor: f.actor,
	}); err == nil {
		t.Fatal("un effacement sans motif a été accepté")
	}
}

// La portabilité doit rendre les dossiers et leur journal, pas seulement la
// fiche : c'est l'historique qui a de la valeur pour la personne.
func TestPortabilityIncludesFilesAndJournal(t *testing.T) {
	f := newFixture(t)
	contact, file := learnerWithFile(t, f)

	data, err := f.crm.Portability(context.Background(), f.org.ID, contact.ID)
	if err != nil {
		t.Fatalf("portabilité: %v", err)
	}

	dossiers, _ := data["dossiers"].([]map[string]any)
	if len(dossiers) != 1 {
		t.Fatalf("%d dossier(s) extrait(s)", len(dossiers))
	}
	extracted, _ := dossiers[0]["dossier"].(crm.File)
	if extracted.ID != file.ID {
		t.Error("le dossier extrait n'est pas celui de la personne")
	}
	events, _ := dossiers[0]["journal"].([]audit.Event)
	if len(events) == 0 {
		t.Error("le journal du dossier n'est pas joint")
	}
}

// dumpPayload rend une charge utile en texte, pour y chercher des fuites.
func dumpPayload(payload map[string]any) string {
	var b strings.Builder
	for key, value := range payload {
		b.WriteString(key)
		b.WriteString(":")
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				b.WriteString(" ")
				b.WriteString(toString(item))
			}
		default:
			b.WriteString(toString(value))
		}
		b.WriteString(" ")
	}
	return b.String()
}

func toString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

var _ = identity.NewID
var _ = ddb.ErrNotFound
