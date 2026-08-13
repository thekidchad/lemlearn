// Package mail envoie les courriels transactionnels par Resend.
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Sender est le contrat commun aux implémentations.
type Sender interface {
	Send(ctx context.Context, to, subject, html string) error
}

// Resend appelle l'API Resend directement en HTTP.
//
// Pas de SDK : l'appel tient en une requête JSON, et une dépendance de plus
// dans une Lambda se paie en taille d'artefact et en surface de mise à jour.
type Resend struct {
	apiKey string
	from   string
	client *http.Client
}

// NewResend construit l'expéditeur.
func NewResend(apiKey, from string) *Resend {
	return &Resend{
		apiKey: apiKey,
		from:   from,
		// Délai serré : un courriel qui n'part pas doit remonter vite. La
		// demande de signature existe déjà en base, elle est relançable.
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Send expédie un courriel.
func (r *Resend) Send(ctx context.Context, to, subject, html string) error {
	payload, err := json.Marshal(map[string]any{
		"from":    r.from,
		"to":      []string{to},
		"subject": subject,
		"html":    html,
	})
	if err != nil {
		return fmt.Errorf("mail: encodage: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("mail: requête: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("mail: envoi: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode >= 300 {
		// Le corps est lu et rapporté : « 422 » seul ne dit pas qu'un domaine
		// n'est pas vérifié, et c'est la cause la plus fréquente.
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("mail: resend a répondu %d: %s", res.StatusCode, bytes.TrimSpace(body))
	}
	return nil
}

// Log écrit les courriels au journal au lieu de les envoyer.
//
// C'est l'expéditeur du développement local et des environnements de recette :
// le parcours de signature doit être exerçable de bout en bout sans compte
// Resend ni domaine vérifié.
type Log struct {
	log *slog.Logger
	// verbose journalise aussi le corps du message. Un courriel d'invitation
	// contient le lien de signature, c'est-à-dire un jeton d'accès : le
	// consigner rend la recette exerçable mais serait une fuite en
	// production. D'où le drapeau, jamais activé là-bas.
	verbose bool

	mu   sync.Mutex
	sent []Sent
}

// Sent est un courriel capturé.
type Sent struct {
	To      string
	Subject string
	HTML    string
}

// NewLog construit l'expéditeur de développement.
func NewLog(log *slog.Logger) *Log { return &Log{log: log} }

// NewLogVerbose journalise en plus le corps du message, lien de signature
// compris. Réservé aux environnements sans données réelles.
func NewLogVerbose(log *slog.Logger) *Log { return &Log{log: log, verbose: true} }

// Send journalise le courriel et le conserve.
func (l *Log) Send(_ context.Context, to, subject, html string) error {
	l.mu.Lock()
	l.sent = append(l.sent, Sent{To: to, Subject: subject, HTML: html})
	l.mu.Unlock()

	if l.log != nil {
		// Le sujet contient le code OTP : c'est ce qui rend le parcours
		// exerçable depuis les journaux.
		fields := []any{"to", to, "subject", subject}
		if l.verbose {
			fields = append(fields, "html", html)
		}
		l.log.Info("courriel non envoyé (expéditeur de recette)", fields...)
	}
	return nil
}

// Sent renvoie les courriels capturés.
func (l *Log) Sent() []Sent {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Sent, len(l.sent))
	copy(out, l.sent)
	return out
}
