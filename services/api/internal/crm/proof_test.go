package crm_test

import (
	"testing"
	"time"

	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/platform/audit"
)

// event compose un événement de journal, tel que le produit le service.
func event(action audit.Action, payload map[string]any) audit.Event {
	return audit.Event{
		Action:  action,
		At:      time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC),
		Payload: payload,
	}
}

// Les treize pièces attendues se déduisent du journal. Deux d'entre elles —
// les satisfactions — ne se distinguent des autres questionnaires que par
// l'usage porté dans l'événement : les compter sur la seule présence d'une
// soumission ferait passer un contrôle après module pour une enquête de
// satisfaction.
func TestSatisfactionIsCountedByItsKind(t *testing.T) {
	learner := crm.Contact{}

	postModule := crm.ComputeProof([]audit.Event{
		event(audit.ActionQuizSubmitted, map[string]any{"kind": "post_module"}),
	}, learner)
	if contains(postModule.Missing, "Questionnaires post-module") {
		t.Error("un contrôle après module soumis reste compté manquant")
	}
	if !contains(postModule.Missing, "Satisfaction à chaud") {
		t.Error("un contrôle après module a été compté comme satisfaction")
	}

	complete := crm.ComputeProof([]audit.Event{
		event(audit.ActionQuizSubmitted, map[string]any{"kind": "satisfaction_hot"}),
		event(audit.ActionQuizSubmitted, map[string]any{"kind": "satisfaction_cold"}),
	}, learner)
	for _, piece := range []string{"Satisfaction à chaud", "Satisfaction à froid"} {
		if contains(complete.Missing, piece) {
			t.Errorf("%s reste comptée manquante après réponse", piece)
		}
	}
}

// Un dossier réellement complet doit pouvoir atteindre treize sur treize —
// sans quoi l'indicateur décourage au lieu de guider.
func TestACompleteFileReachesEveryPiece(t *testing.T) {
	learner := crm.Contact{IdentityDocKey: "orgs/O/contacts/C/piece-identite.jpg"}

	status := crm.ComputeProof([]audit.Event{
		event(audit.ActionConsentGiven, nil),
		event(audit.ActionDocumentGenerated, nil),
		event(audit.ActionDocumentSent, map[string]any{"kind": "quote", "reference": "DEV-1"}),
		event(audit.ActionDocumentSigned, map[string]any{"reference": "DEV-1"}),
		event(audit.ActionDocumentSent, map[string]any{"kind": "convention", "reference": "CONV-1"}),
		event(audit.ActionDocumentSigned, map[string]any{"reference": "CONV-1"}),
		event(audit.ActionQuizSubmitted, map[string]any{"kind": "positioning"}),
		event(audit.ActionQuizSubmitted, map[string]any{"kind": "post_module"}),
		event(audit.ActionQuizSubmitted, map[string]any{"kind": "final"}),
		event(audit.ActionQuizSubmitted, map[string]any{"kind": "satisfaction_hot"}),
		event(audit.ActionQuizSubmitted, map[string]any{"kind": "satisfaction_cold"}),
		event(audit.ActionWatchProgress, nil),
		event(audit.ActionAttendanceSigned, nil),
		event(audit.ActionCertificateIssued, nil),
	}, learner)

	if status.Present != status.Expected {
		t.Fatalf("%d pièces sur %d, il manque %v", status.Present, status.Expected, status.Missing)
	}
}

// Un dossier vide ne prétend rien : toutes les pièces sont annoncées
// manquantes, nommément.
func TestAnEmptyFileNamesEveryMissingPiece(t *testing.T) {
	status := crm.ComputeProof(nil, crm.Contact{})
	if status.Present != 0 || len(status.Missing) != status.Expected {
		t.Fatalf("%d présentes, %d manquantes sur %d attendues",
			status.Present, len(status.Missing), status.Expected)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
