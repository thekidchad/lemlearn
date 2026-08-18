package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Stripe branche l'abonnement en libre-service.
//
// Le client Stripe officiel n'est pas utilisé : deux appels REST et une
// vérification HMAC ne justifient pas une dépendance qui tire tout le
// catalogue de l'API. Ce qui compte ici — la vérification de signature — se
// fait sur la bibliothèque standard, et se teste.
type Stripe struct {
	key           string
	webhookSecret string
	prices        map[string]string
	http          *http.Client
}

// NewStripe construit le client. Renvoie nil si la clé n'est pas configurée :
// l'abonnement en libre-service est une option, le produit fonctionne sans —
// un organisme peut très bien être facturé sur devis.
func NewStripe(key, webhookSecret, priceMap string) *Stripe {
	if key == "" {
		return nil
	}
	return &Stripe{
		key: key, webhookSecret: webhookSecret,
		prices: parsePrices(priceMap),
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

// parsePrices lit « essentiel=price_123,structure=price_456 ».
func parsePrices(value string) map[string]string {
	prices := make(map[string]string)
	for _, pair := range strings.Split(value, ",") {
		code, price, found := strings.Cut(strings.TrimSpace(pair), "=")
		if found && code != "" && price != "" {
			prices[code] = price
		}
	}
	return prices
}

// Checkout ouvre une page de paiement pour une formule.
//
// L'identifiant de l'organisation voyage en métadonnée : c'est lui qui
// permettra au webhook de savoir qui vient de payer, sans faire confiance à
// l'URL de retour, qu'un client peut appeler lui-même.
func (s *Stripe) Checkout(ctx context.Context, orgID, planCode, successURL, cancelURL string) (string, error) {
	price, ok := s.prices[planCode]
	if !ok {
		return "", fmt.Errorf("aucun tarif Stripe configuré pour la formule %q", planCode)
	}

	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("line_items[0][price]", price)
	form.Set("line_items[0][quantity]", "1")
	form.Set("success_url", successURL)
	form.Set("cancel_url", cancelURL)
	form.Set("client_reference_id", orgID)
	form.Set("metadata[orgId]", orgID)
	form.Set("metadata[plan]", planCode)
	form.Set("subscription_data[metadata][orgId]", orgID)
	form.Set("subscription_data[metadata][plan]", planCode)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.stripe.com/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := s.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("stripe: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode >= 300 {
		return "", fmt.Errorf("stripe a répondu %d", response.StatusCode)
	}

	var session struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &session); err != nil {
		return "", fmt.Errorf("stripe: réponse illisible: %w", err)
	}
	if session.URL == "" {
		return "", fmt.Errorf("stripe n'a pas renvoyé d'URL de paiement")
	}
	return session.URL, nil
}

// WebhookTolerance borne l'âge d'un événement accepté.
//
// Sans cette borne, un événement légitime capté une fois pourrait être rejoué
// indéfiniment : la signature resterait valable, et une organisation
// redescendrait de formule sur commande.
const WebhookTolerance = 5 * time.Minute

// VerifyWebhook contrôle la signature d'un appel de Stripe.
func (s *Stripe) VerifyWebhook(payload []byte, header string, now time.Time) error {
	if s.webhookSecret == "" {
		return fmt.Errorf("aucun secret de webhook configuré")
	}

	var timestamp string
	var signatures []string
	for _, part := range strings.Split(header, ",") {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		switch key {
		case "t":
			timestamp = value
		case "v1":
			signatures = append(signatures, value)
		}
	}
	if timestamp == "" || len(signatures) == 0 {
		return fmt.Errorf("en-tête de signature illisible")
	}

	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("horodatage de signature illisible")
	}
	age := now.Sub(time.Unix(seconds, 0))
	if age < -WebhookTolerance || age > WebhookTolerance {
		return fmt.Errorf("événement trop ancien ou daté du futur")
	}

	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(payload)
	expected := mac.Sum(nil)

	for _, candidate := range signatures {
		given, err := hex.DecodeString(candidate)
		if err != nil {
			continue
		}
		// Comparaison à temps constant : une comparaison naïve laisserait
		// deviner la signature octet par octet.
		if hmac.Equal(given, expected) {
			return nil
		}
	}
	return fmt.Errorf("signature invalide")
}

// Change est ce qu'un événement Stripe dit d'un abonnement.
type Change struct {
	OrgID  string
	Plan   string
	Active bool
	Type   string
}

// ReadEvent extrait d'un événement ce qui nous concerne.
//
// Stripe en envoie des dizaines de sortes ; trois nous intéressent, et les
// autres se répondent 200 sans rien faire — un webhook qui échoue sur un
// événement qu'il ne connaît pas finit désactivé par Stripe.
func ReadEvent(payload []byte) (Change, bool, error) {
	var event struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ClientReferenceID string            `json:"client_reference_id"`
				Metadata          map[string]string `json:"metadata"`
				Status            string            `json:"status"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return Change{}, false, fmt.Errorf("événement illisible: %w", err)
	}

	object := event.Data.Object
	orgID := object.Metadata["orgId"]
	if orgID == "" {
		orgID = object.ClientReferenceID
	}

	switch event.Type {
	case "checkout.session.completed",
		"customer.subscription.created",
		"customer.subscription.updated":
		if orgID == "" || object.Metadata["plan"] == "" {
			return Change{}, false, fmt.Errorf("événement %s sans organisation ni formule", event.Type)
		}
		// Un abonnement impayé ou en cours d'annulation ne vaut pas un
		// abonnement actif : la formule ne s'applique que s'il est en règle.
		active := object.Status == "" || object.Status == "active" ||
			object.Status == "trialing" || object.Status == "complete"
		return Change{OrgID: orgID, Plan: object.Metadata["plan"], Active: active, Type: event.Type}, true, nil

	case "customer.subscription.deleted":
		if orgID == "" {
			return Change{}, false, fmt.Errorf("résiliation sans organisation")
		}
		// Une résiliation ne supprime rien : elle ramène à l'essai. Les
		// documents scellés restent sous Object Lock, et l'organisme garde
		// accès à ses preuves — les lui retirer serait indéfendable.
		return Change{OrgID: orgID, Plan: "trial", Active: false, Type: event.Type}, true, nil
	}

	return Change{}, false, nil
}
