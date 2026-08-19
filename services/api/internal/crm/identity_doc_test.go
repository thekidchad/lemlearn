package crm_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// fakeDocs remplace le compartiment chiffré : ce qui est testé ici est la
// règle d'accès, pas la signature d'URL par le SDK.
type fakeDocs struct {
	put, get, deleted []string
	ttl               time.Duration
}

func (f *fakeDocs) PresignedPut(_ context.Context, key, _ string, _ time.Duration) (string, error) {
	f.put = append(f.put, key)
	return "https://s3.example/" + key + "?put", nil
}

func (f *fakeDocs) PresignedGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	f.get = append(f.get, key)
	f.ttl = ttl
	return "https://s3.example/" + key + "?get", nil
}

func (f *fakeDocs) Delete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

func learnerWithDocs(t *testing.T) (*crm.Service, *fakeDocs, string, string) {
	t.Helper()
	docs := &fakeDocs{}
	service := crm.NewService(ddb.NewTestClient(t), nil).WithDocs(docs)

	contact := crm.NewContact("ORG1", crm.KindLearner, time.Now().UTC())
	contact.FirstName, contact.LastName = "Léa", "Bertrand"
	contact.Email = "lea@example.fr"
	saved, err := service.CreateContact(context.Background(), contact)
	if err != nil {
		t.Fatalf("création du contact: %v", err)
	}
	return service, docs, "ORG1", saved.ID
}

var agent = audit.Actor{Type: audit.ActorUser, ID: "USR-1", Label: "Marie Dubreuil"}

// La pièce se dépose directement sur S3, sous une clé qui porte
// l'organisation et le contact : c'est cette clé qui empêche un organisme de
// lire les pièces d'un autre.
func TestIdentityDocIsScopedToItsOwner(t *testing.T) {
	service, docs, org, contact := learnerWithDocs(t)

	url, key, err := service.PrepareIdentityDoc(context.Background(), org, contact,
		"carte.jpg", "image/jpeg")
	if err != nil {
		t.Fatalf("préparation: %v", err)
	}
	if url == "" || len(docs.put) != 1 {
		t.Fatal("aucune URL de dépôt produite")
	}
	if !strings.HasPrefix(key, "orgs/"+org+"/contacts/"+contact+"/") {
		t.Errorf("clé = %q, hors de l'espace du contact", key)
	}
	// L'extension est conservée : un PDF annoncé en JPEG se télécharge au
	// lieu de s'afficher, et l'agent recommence.
	if !strings.HasSuffix(key, ".jpg") {
		t.Errorf("clé = %q, extension perdue", key)
	}
}

// Un format arbitraire est refusé : le compartiment est présigné en lecture,
// y accepter n'importe quel fichier en ferait un hébergement ouvert.
func TestOnlyIdentityFormatsAreAccepted(t *testing.T) {
	service, _, org, contact := learnerWithDocs(t)

	for _, contentType := range []string{"text/html", "application/zip", ""} {
		if _, _, err := service.PrepareIdentityDoc(context.Background(), org, contact,
			"piece", contentType); err == nil {
			t.Errorf("format %q accepté", contentType)
		}
	}
}

// Rattacher la pièce d'un autre apprenant à sa propre fiche doit échouer : la
// clé vient du client, elle ne fait pas foi.
func TestAttachRefusesAKeyFromAnotherContact(t *testing.T) {
	service, _, org, contact := learnerWithDocs(t)

	if _, err := service.AttachIdentityDoc(context.Background(), org, contact,
		"orgs/"+org+"/contacts/AUTRE/piece-identite.jpg", agent); err == nil {
		t.Fatal("une pièce d'un autre contact a été rattachée")
	}
}

// Le lien de consultation est court et la consultation est tracée : savoir qui
// a ouvert une carte d'identité, et quand, fait partie de ce qu'un contrôle
// CNIL peut demander.
func TestConsultationIsShortLivedAndTraced(t *testing.T) {
	service, docs, org, contact := learnerWithDocs(t)

	_, key, err := service.PrepareIdentityDoc(context.Background(), org, contact,
		"carte.pdf", "application/pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AttachIdentityDoc(context.Background(), org, contact, key, agent); err != nil {
		t.Fatalf("rattachement: %v", err)
	}

	url, err := service.IdentityDocURL(context.Background(), org, contact, agent)
	if err != nil {
		t.Fatalf("consultation: %v", err)
	}
	if url == "" || len(docs.get) != 1 {
		t.Fatal("aucun lien de consultation produit")
	}
	if docs.ttl > time.Minute {
		t.Errorf("validité du lien = %s : de quoi le faire circuler", docs.ttl)
	}

	// La fiche ne porte que la clé, jamais une URL : une URL stockée serait
	// un accès permanent à une donnée sensible.
	saved, err := service.GetContact(context.Background(), org, contact)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(saved.IdentityDocKey, "http") {
		t.Errorf("la fiche porte une URL : %q", saved.IdentityDocKey)
	}
}

// La suppression anticipée est possible : la CNIL recommande de ne pas
// conserver une pièce au-delà de la vérification du dossier.
func TestIdentityDocCanBeDeletedBeforeExpiry(t *testing.T) {
	service, docs, org, contact := learnerWithDocs(t)

	_, key, _ := service.PrepareIdentityDoc(context.Background(), org, contact, "c.png", "image/png")
	if _, err := service.AttachIdentityDoc(context.Background(), org, contact, key, agent); err != nil {
		t.Fatal(err)
	}

	after, err := service.DeleteIdentityDoc(context.Background(), org, contact, agent)
	if err != nil {
		t.Fatalf("suppression: %v", err)
	}
	if after.IdentityDocKey != "" {
		t.Error("la fiche référence encore une pièce supprimée")
	}
	if len(docs.deleted) != 1 || docs.deleted[0] != key {
		t.Errorf("objet supprimé = %v, attendu %q", docs.deleted, key)
	}

	// Consulter ce qui n'existe plus doit échouer clairement, pas produire un
	// lien mort.
	if _, err := service.IdentityDocURL(context.Background(), org, contact, agent); err == nil {
		t.Error("un lien a été produit pour une pièce supprimée")
	}
}
