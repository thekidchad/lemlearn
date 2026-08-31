// Package audit tient le journal horodaté de lemlearn.
//
// Chaque écriture métier — création de dossier, envoi de convention, signature,
// visionnage validé, soumission de questionnaire, émargement, export — y dépose
// un événement. Le journal est en ajout seul et **chaîné par hash** : chaque
// événement porte l'empreinte du précédent, si bien qu'une modification ou une
// suppression a posteriori casse la chaîne de façon détectable.
//
// C'est ce qui distingue un journal d'un simple historique : un historique
// modifiable ne prouve rien.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Action est le verbe d'un événement. Liste fermée : un journal dont le
// vocabulaire dérive n'est plus exploitable en audit.
type Action string

const (
	ActionFileCreated       Action = "file.created"
	ActionFileStageChanged  Action = "file.stage_changed"
	ActionConsentGiven      Action = "consent.given"
	ActionDocumentGenerated Action = "document.generated"
	ActionDocumentSent      Action = "document.sent"
	ActionSignatureOpened   Action = "signature.opened"
	ActionSignatureOTPSent  Action = "signature.otp_sent"
	ActionSignatureOTPOK    Action = "signature.otp_verified"
	ActionSignatureOTPFail  Action = "signature.otp_failed"
	ActionDocumentSigned    Action = "document.signed"
	ActionDocumentSealed    Action = "document.sealed"
	ActionWatchProgress     Action = "watch.progress"
	ActionModuleCompleted   Action = "module.completed"
	ActionQuizStarted       Action = "quiz.started"
	ActionQuizSubmitted     Action = "quiz.submitted"
	ActionAttendanceSigned  Action = "attendance.signed"
	ActionSessionClosed     Action = "session.closed"
	ActionFollowUpScheduled Action = "followup.scheduled"
	ActionCertificateIssued Action = "certificate.issued"
	ActionDossierExported   Action = "dossier.exported"
	ActionLearnerAnonymized Action = "learner.anonymized"
	ActionPlanChanged       Action = "billing.plan_changed"
	ActionImpersonated      Action = "admin.impersonated"

	// Les entrées et sorties de session. Elles ne décrivent aucune donnée
	// métier, mais ce sont les premières qu'on cherche quand on se demande qui
	// est passé, d'où, et quand — et un journal qui ne les porte pas oblige à
	// aller les chercher dans les traces techniques, où elles ne sont gardées
	// que quelques jours.
	ActionSignedIn     Action = "auth.signed_in"
	ActionSignInFailed Action = "auth.sign_in_failed"
	ActionSignedOut    Action = "auth.signed_out"
)

// ActorType distingue qui agit : un humain connecté, un signataire authentifié
// par lien et OTP, ou le système lui-même (tâche planifiée, webhook).
type ActorType string

const (
	ActorUser    ActorType = "user"
	ActorLearner ActorType = "learner"
	ActorSystem  ActorType = "system"
)

// Actor identifie l'auteur d'un événement.
type Actor struct {
	Type      ActorType `json:"type"`
	ID        string    `json:"id"`
	Label     string    `json:"label,omitempty"`
	IP        string    `json:"ip,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	// OnBehalfOf est renseigné lorsqu'un super-administrateur agit en
	// impersonation : l'action est imputée à l'utilisateur, mais on garde
	// trace de qui l'a réellement déclenchée.
	OnBehalfOf string `json:"on_behalf_of,omitempty"`
}

// Event est une entrée du journal. Les champs sont ordonnés : la sérialisation
// canonique dépend de cet ordre, le modifier invaliderait les chaînes existantes.
type Event struct {
	// Subject identifie la chose auditée : "file/DOS-2026-0143",
	// "enrollment/ENR-91", "org/ORG-7". C'est la clé de partition du journal.
	Subject string `json:"subject"`
	// Seq est le rang dans la chaîne du sujet, à partir de 1.
	Seq int64 `json:"seq"`
	// At est l'heure serveur. Elle sert au tri et à l'affichage, jamais de
	// preuve à elle seule : la date opposable vient du jeton RFC 3161
	// déposé dans Payload lors d'un scellement.
	At      time.Time      `json:"at"`
	Action  Action         `json:"action"`
	Actor   Actor          `json:"actor"`
	Payload map[string]any `json:"payload,omitempty"`

	// PrevHash est l'empreinte de l'événement précédent du même sujet, vide
	// pour le premier. Hash est calculée sur le reste, PrevHash inclus.
	PrevHash string `json:"prev_hash,omitempty"`
	Hash     string `json:"hash"`
}

// GenesisHash est le PrevHash conventionnel du premier événement d'un sujet.
const GenesisHash = ""

// Append construit l'événement suivant d'une chaîne et calcule son empreinte.
//
// prev est le dernier événement connu du sujet, ou le zéro d'Event pour
// commencer une chaîne.
func Append(prev Event, subject string, at time.Time, action Action, actor Actor, payload map[string]any) (Event, error) {
	if subject == "" {
		return Event{}, fmt.Errorf("audit: sujet vide")
	}
	if action == "" {
		return Event{}, fmt.Errorf("audit: action vide")
	}
	if prev.Hash != "" && prev.Subject != subject {
		return Event{}, fmt.Errorf("audit: chaînage entre sujets différents (%q puis %q)", prev.Subject, subject)
	}

	event := Event{
		Subject:  subject,
		Seq:      prev.Seq + 1,
		At:       at.UTC().Truncate(time.Millisecond),
		Action:   action,
		Actor:    actor,
		Payload:  payload,
		PrevHash: prev.Hash,
	}
	hash, err := event.computeHash()
	if err != nil {
		return Event{}, err
	}
	event.Hash = hash
	return event, nil
}

// computeHash renvoie SHA-256(prev_hash ‖ sérialisation canonique).
//
// La sérialisation canonique est le JSON de l'événement *sans* son propre
// champ Hash. encoding/json trie les clés de map, donc un même Payload produit
// toujours les mêmes octets, quel que soit l'ordre d'insertion.
func (e Event) computeHash() (string, error) {
	type canonical struct {
		Subject  string         `json:"subject"`
		Seq      int64          `json:"seq"`
		At       string         `json:"at"`
		Action   Action         `json:"action"`
		Actor    Actor          `json:"actor"`
		Payload  map[string]any `json:"payload,omitempty"`
		PrevHash string         `json:"prev_hash,omitempty"`
	}

	body, err := json.Marshal(canonical{
		Subject: e.Subject,
		Seq:     e.Seq,
		// RFC 3339 en nanosecondes UTC : représentation stable, indépendante
		// du fuseau du serveur qui a écrit l'événement.
		At:       e.At.UTC().Format(time.RFC3339Nano),
		Action:   e.Action,
		Actor:    e.Actor,
		Payload:  e.Payload,
		PrevHash: e.PrevHash,
	})
	if err != nil {
		return "", fmt.Errorf("audit: sérialisation canonique: %w", err)
	}

	sum := sha256.New()
	sum.Write([]byte(e.PrevHash))
	sum.Write(body)
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// Verify contrôle une chaîne complète, dans l'ordre.
//
// C'est la fonction appelée à l'export d'un dossier : le ZIP livré à l'auditeur
// ne doit contenir qu'un journal vérifié.
func Verify(chain []Event) error {
	var prevHash string
	for i, event := range chain {
		if event.Seq != int64(i+1) {
			return fmt.Errorf("audit: rang %d attendu à la position %d, trouvé %d", i+1, i, event.Seq)
		}
		if event.PrevHash != prevHash {
			return fmt.Errorf("audit: rupture de chaîne au rang %d (%s) : empreinte précédente attendue %q, trouvée %q",
				event.Seq, event.Action, truncate(prevHash), truncate(event.PrevHash))
		}
		expected, err := event.computeHash()
		if err != nil {
			return err
		}
		if expected != event.Hash {
			return fmt.Errorf("audit: événement altéré au rang %d (%s) : empreinte %q, recalculée %q",
				event.Seq, event.Action, truncate(event.Hash), truncate(expected))
		}
		if i > 0 && event.At.Before(chain[i-1].At) {
			return fmt.Errorf("audit: horodatage non croissant au rang %d", event.Seq)
		}
		prevHash = event.Hash
	}
	return nil
}

func truncate(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12] + "…"
}

// Actions énumère les actions connues du journal.
//
// La liste sert l'écran de filtre : la déduire des événements présents ferait
// disparaître une action des choix possibles le jour où personne ne l'a
// déclenchée, c'est-à-dire précisément le jour où on la cherche.
func Actions() []Action {
	return []Action{
		ActionSignedIn, ActionSignInFailed, ActionSignedOut,
		ActionFileCreated, ActionFileStageChanged,
		ActionConsentGiven,
		ActionDocumentGenerated, ActionDocumentSent,
		ActionSignatureOpened, ActionSignatureOTPSent,
		ActionSignatureOTPOK, ActionSignatureOTPFail,
		ActionDocumentSigned, ActionDocumentSealed,
		ActionWatchProgress, ActionModuleCompleted,
		ActionQuizStarted, ActionQuizSubmitted,
		ActionAttendanceSigned, ActionSessionClosed,
		ActionFollowUpScheduled, ActionCertificateIssued,
		ActionDossierExported, ActionLearnerAnonymized,
		ActionPlanChanged, ActionImpersonated,
	}
}
