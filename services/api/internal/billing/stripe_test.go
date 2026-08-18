package billing_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/billing"
)

const secret = "whsec_exemple"

func sign(t *testing.T, payload string, at time.Time) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.%s", at.Unix(), payload)
	return fmt.Sprintf("t=%d,v1=%s", at.Unix(), hex.EncodeToString(mac.Sum(nil)))
}

// Un appel signé du bon secret passe ; tout le reste est refusé. C'est la
// seule chose qui empêche n'importe qui de nous annoncer qu'il vient de
// s'abonner à la formule la plus chère.
func TestWebhookAcceptsOnlyWhatStripeSigned(t *testing.T) {
	stripe := billing.NewStripe("sk_test", secret, "essentiel=price_1")
	now := time.Now()
	payload := `{"type":"checkout.session.completed"}`

	if err := stripe.VerifyWebhook([]byte(payload), sign(t, payload, now), now); err != nil {
		t.Fatalf("un appel légitime a été refusé: %v", err)
	}

	// Charge modifiée après signature.
	altered := `{"type":"checkout.session.completed","plan":"reseau"}`
	if err := stripe.VerifyWebhook([]byte(altered), sign(t, payload, now), now); err == nil {
		t.Error("une charge modifiée a été acceptée")
	}

	// Signature d'un autre secret.
	other := billing.NewStripe("sk_test", "whsec_autre", "")
	header := sign(t, payload, now)
	if err := other.VerifyWebhook([]byte(payload), header, now); err == nil {
		t.Error("une signature d'un autre secret a été acceptée")
	}

	// En-têtes malformés.
	for _, bad := range []string{"", "v1=abc", "t=zzz,v1=abc", fmt.Sprintf("t=%d", now.Unix())} {
		if err := stripe.VerifyWebhook([]byte(payload), bad, now); err == nil {
			t.Errorf("en-tête %q accepté", bad)
		}
	}
}

// Un événement rejoué garde une signature valable : seule la fenêtre de
// tolérance l'arrête. Sans elle, un appel capté une fois ferait redescendre
// une organisation de formule à volonté.
func TestReplayedWebhookIsRefused(t *testing.T) {
	stripe := billing.NewStripe("sk_test", secret, "")
	payload := `{"type":"customer.subscription.deleted"}`
	issued := time.Now().Add(-billing.WebhookTolerance - time.Minute)

	if err := stripe.VerifyWebhook([]byte(payload), sign(t, payload, issued), time.Now()); err == nil {
		t.Error("un événement rejoué hors fenêtre a été accepté")
	}
	// Daté du futur : même refus, sinon la fenêtre s'ouvrirait indéfiniment
	// vers l'avant.
	future := time.Now().Add(billing.WebhookTolerance + time.Minute)
	if err := stripe.VerifyWebhook([]byte(payload), sign(t, payload, future), time.Now()); err == nil {
		t.Error("un événement daté du futur a été accepté")
	}
}

// L'organisation vient de la métadonnée posée à l'ouverture du paiement, pas
// d'un paramètre d'URL de retour, qu'un client peut appeler lui-même.
func TestEventCarriesTheOrgAndPlan(t *testing.T) {
	change, ok, err := billing.ReadEvent([]byte(`{
      "type":"checkout.session.completed",
      "data":{"object":{"client_reference_id":"ORG1","metadata":{"orgId":"ORG1","plan":"structure"}}}}`))
	if err != nil || !ok {
		t.Fatalf("événement rejeté: %v", err)
	}
	if change.OrgID != "ORG1" || change.Plan != "structure" || !change.Active {
		t.Fatalf("changement = %+v", change)
	}
}

// Un abonnement impayé ne vaut pas un abonnement actif.
func TestUnpaidSubscriptionIsNotActive(t *testing.T) {
	change, ok, err := billing.ReadEvent([]byte(`{
      "type":"customer.subscription.updated",
      "data":{"object":{"status":"past_due","metadata":{"orgId":"ORG1","plan":"reseau"}}}}`))
	if err != nil || !ok {
		t.Fatalf("événement rejeté: %v", err)
	}
	if change.Active {
		t.Error("un abonnement impayé est passé pour actif")
	}
}

// Une résiliation ramène à l'essai, elle ne supprime rien : les documents
// scellés restent, et l'organisme garde accès à ses preuves.
func TestCancellationFallsBackToTrial(t *testing.T) {
	change, ok, err := billing.ReadEvent([]byte(`{
      "type":"customer.subscription.deleted",
      "data":{"object":{"metadata":{"orgId":"ORG1","plan":"reseau"}}}}`))
	if err != nil || !ok {
		t.Fatalf("événement rejeté: %v", err)
	}
	if change.Plan != "trial" {
		t.Errorf("formule après résiliation = %q", change.Plan)
	}
}

// Stripe envoie des dizaines de sortes d'événements : ceux qu'on ne traite pas
// se répondent sans erreur, sinon Stripe finit par désactiver le webhook.
func TestUnknownEventIsIgnoredWithoutError(t *testing.T) {
	_, ok, err := billing.ReadEvent([]byte(`{"type":"invoice.upcoming","data":{"object":{}}}`))
	if err != nil {
		t.Fatalf("événement inconnu en erreur: %v", err)
	}
	if ok {
		t.Error("un événement inconnu a été traité")
	}
}

// Sans clé, l'abonnement en libre-service n'existe pas — et le produit
// fonctionne quand même : un organisme peut être facturé sur devis.
func TestStripeIsOptional(t *testing.T) {
	if billing.NewStripe("", secret, "") != nil {
		t.Error("un client Stripe a été construit sans clé")
	}
}
