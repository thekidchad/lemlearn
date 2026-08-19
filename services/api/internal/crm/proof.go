package crm

import (
	"context"

	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// ComputeProof déduit l'état du dossier de preuve de son journal d'audit.
//
// Le décompte n'est pas maintenu au fil des écritures mais recalculé : chaque
// pièce a déjà laissé un événement dans la chaîne, et déduire l'état de la
// chaîne garantit que l'affichage ne peut pas diverger de ce qui est réellement
// prouvable. Un compteur incrémenté à la main aurait fini par mentir — et le
// pourcentage de complétude est le premier chiffre que regarde un organisme.
func ComputeProof(events []audit.Event, learner Contact) ProofStatus {
	seen := make(map[audit.Action]bool, len(events))

	// Le type du document est porté par l'événement d'envoi, sa signature par
	// un événement ultérieur qui ne cite que la référence. Il faut donc les
	// relier : sans cela, une convention signée resterait comptée manquante,
	// ce qui est exactement le genre d'erreur qui ruine la confiance dans
	// l'indicateur.
	kindByReference := map[string]string{}
	signedKinds := map[string]bool{}
	// Les questionnaires portent leur usage dans l'événement de soumission :
	// c'est ce qui distingue un positionnement d'une satisfaction à froid, et
	// deux des treize pièces attendues ne sont rien d'autre que cela.
	quizKinds := map[string]bool{}

	for _, event := range events {
		seen[event.Action] = true

		reference, _ := event.Payload["reference"].(string)
		switch event.Action {
		case audit.ActionQuizSubmitted:
			if kind, ok := event.Payload["kind"].(string); ok {
				quizKinds[kind] = true
			}
		case audit.ActionDocumentSent:
			if kind, ok := event.Payload["kind"].(string); ok && reference != "" {
				kindByReference[reference] = kind
			}
		case audit.ActionDocumentSigned:
			if kind, ok := kindByReference[reference]; ok {
				signedKinds[kind] = true
			}
			signedKinds["any"] = true
		}
	}

	checks := []struct {
		piece string
		done  bool
	}{
		{"Identité de l'apprenant", learner.IdentityDocKey != ""},
		{"Consentement RGPD", seen[audit.ActionConsentGiven]},
		{"Devis", signedKinds["quote"]},
		{"Convention signée", signedKinds["convention"]},
		{"Programme de formation", seen[audit.ActionDocumentGenerated]},
		{"Évaluation de positionnement", quizKinds["positioning"]},
		{"Relevés de connexion", seen[audit.ActionWatchProgress] || seen[audit.ActionModuleCompleted]},
		{"Questionnaires post-module", quizKinds["post_module"]},
		{"Évaluation finale", quizKinds["final"]},
		{"Feuilles d'émargement", seen[audit.ActionAttendanceSigned]},
		{"Satisfaction à chaud", quizKinds["satisfaction_hot"]},
		{"Satisfaction à froid", quizKinds["satisfaction_cold"]},
		{"Attestation de fin de formation", seen[audit.ActionCertificateIssued]},
	}

	status := ProofStatus{Expected: len(checks)}
	for _, check := range checks {
		if check.done {
			status.Present++
			continue
		}
		status.Missing = append(status.Missing, check.piece)
	}
	return status
}

// FileWithProof relit un dossier avec sa complétude à jour.
//
// Le recalcul est persisté au passage : les écrans de liste, qui ne peuvent
// pas se permettre une lecture du journal par dossier, affichent ainsi une
// valeur fraîche dès qu'une fiche a été ouverte une fois.
func (s *Service) FileWithProof(ctx context.Context, orgID, fileID string) (File, []audit.Event, error) {
	file, err := s.GetFile(ctx, orgID, fileID)
	if err != nil {
		return File{}, nil, err
	}

	events, err := s.Timeline(ctx, fileID)
	if err != nil {
		return File{}, nil, err
	}

	var learner Contact
	if file.LearnerID != "" {
		learner, _ = s.GetContact(ctx, orgID, file.LearnerID)
	}

	computed := ComputeProof(events, learner)
	if computed.Present != file.Proof.Present || computed.Expected != file.Proof.Expected {
		file.Proof = computed
		file.UpdatedAt = s.now()
		if err := ddb.Put(ctx, s.db, file); err != nil {
			// L'échec de la persistance n'empêche pas d'afficher la valeur
			// juste : le dossier reste correct, seule sa mise en cache rate.
			return file, events, nil
		}
	}
	file.Proof = computed
	return file, events, nil
}
