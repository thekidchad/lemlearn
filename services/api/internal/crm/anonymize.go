package crm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// AnonymizeInput décrit une demande d'effacement.
type AnonymizeInput struct {
	OrgID     string
	ContactID string
	// Reason porte le motif : demande de la personne, expiration de la durée
	// de conservation, décision de l'organisme. Il figure au journal.
	Reason string
	Actor  audit.Actor
}

// Anonymize efface les données personnelles d'un contact sans détruire les
// preuves qui le concernent.
//
// C'est la seule lecture qui concilie le droit à l'effacement et l'obligation
// de conservation : les conventions signées, les émargements et les
// attestations doivent survivre le temps d'un contrôle, mais rien n'oblige à
// ce qu'ils restent rattachés à une identité en clair. Le contact devient un
// pseudonyme stable, et les pièces scellées gardent leur valeur probante
// puisqu'on n'y touche pas.
//
// Le pseudonyme est dérivé de l'identifiant, donc reproductible : deux
// exports du même dossier à un an d'intervalle désignent la même personne
// anonymisée, ce qu'un contrôleur peut recouper.
func (s *Service) Anonymize(ctx context.Context, in AnonymizeInput) (Contact, error) {
	if strings.TrimSpace(in.Reason) == "" {
		return Contact{}, fmt.Errorf("le motif de l'anonymisation est obligatoire")
	}

	contact, err := s.GetContact(ctx, in.OrgID, in.ContactID)
	if err != nil {
		return Contact{}, err
	}
	if contact.Anonymized {
		return contact, nil
	}

	now := s.now()
	// Ce qui est effacé est journalisé par sa nature, jamais par sa valeur :
	// écrire l'adresse supprimée dans le journal la conserverait.
	erased := erasedFields(contact)

	pseudonym := Pseudonym(contact.ID)
	contact.FirstName = ""
	contact.LastName = pseudonym
	contact.CompanyName = ""
	contact.Email = ""
	contact.Phone = ""
	contact.BirthDate = ""
	contact.BirthPlace = ""
	contact.Address = Address{}
	contact.Position = ""
	contact.Notes = ""
	// La clé de la pièce d'identité disparaît de l'index ; l'objet lui-même
	// est purgé par la règle de cycle de vie du compartiment.
	contact.IdentityDocKey = ""
	contact.Anonymized = true
	contact.Reindex(now)

	// L'anonymisation est journalisée sur chaque dossier où la personne
	// apparaît : c'est là que se trouvent les preuves, et c'est là qu'un
	// contrôleur cherchera l'explication du pseudonyme.
	files, err := s.filesOfLearner(ctx, in.OrgID, in.ContactID)
	if err != nil {
		return Contact{}, err
	}

	payload := map[string]any{
		"pseudonyme": pseudonym,
		"motif":      in.Reason,
		"champs":     erased,
	}

	if len(files) == 0 {
		if err := ddb.Put(ctx, s.db, contact); err != nil {
			return Contact{}, err
		}
		return contact, nil
	}

	// Le contact n'est écrit qu'une fois, avec le premier dossier ; les
	// suivants ne portent que l'événement. Réécrire le contact à chaque
	// dossier n'ajouterait rien et multiplierait les occasions d'échec.
	for i, file := range files {
		writes := []ddb.Write{}
		if i == 0 {
			writes = append(writes, ddb.Write{Item: contact})
		}
		if _, err := s.db.WriteWithAudit(ctx, FileSubject(file), writes,
			func(prev audit.Event) (audit.Event, error) {
				return audit.Append(prev, FileSubject(file), now,
					audit.ActionLearnerAnonymized, in.Actor, payload)
			}); err != nil {
			return Contact{}, err
		}
	}

	return contact, nil
}

// Pseudonym dérive un identifiant lisible et stable d'un contact effacé.
func Pseudonym(contactID string) string {
	sum := sha256.Sum256([]byte("lemlearn/pseudonyme/" + contactID))
	return "Apprenant anonymisé " + strings.ToUpper(hex.EncodeToString(sum[:3]))
}

// erasedFields nomme les catégories de données effacées, sans leur valeur.
func erasedFields(contact Contact) []string {
	var fields []string
	add := func(name string, present bool) {
		if present {
			fields = append(fields, name)
		}
	}
	add("nom", contact.LastName != "" || contact.FirstName != "")
	add("raison sociale", contact.CompanyName != "")
	add("adresse électronique", contact.Email != "")
	add("téléphone", contact.Phone != "")
	add("date de naissance", contact.BirthDate != "")
	add("lieu de naissance", contact.BirthPlace != "")
	add("adresse postale", contact.Address.Line1 != "" || contact.Address.City != "")
	add("fonction", contact.Position != "")
	add("notes", contact.Notes != "")
	add("pièce d'identité", contact.IdentityDocKey != "")
	return fields
}

// filesOfLearner liste les dossiers où un contact figure comme apprenant.
func (s *Service) filesOfLearner(ctx context.Context, orgID, contactID string) ([]string, error) {
	var ids []string
	for _, stage := range []Stage{StageProspect, StageQuote, StageAgreement,
		StageInTraining, StageClosed, StageLost} {
		files, err := s.ListFilesByStage(ctx, orgID, stage, 500)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if file.LearnerID == contactID {
				ids = append(ids, file.ID)
			}
		}
	}
	return ids, nil
}

// Portability rassemble tout ce que l'organisation détient sur une personne.
//
// Réponse à l'exigence de portabilité : un format lisible et repreneur, remis
// sans condition et sans délai — nous n'avons aucune raison de faire attendre
// quelqu'un pour des données qui sont les siennes.
func (s *Service) Portability(ctx context.Context, orgID, contactID string) (map[string]any, error) {
	contact, err := s.GetContact(ctx, orgID, contactID)
	if err != nil {
		return nil, err
	}

	files, err := s.filesOfLearner(ctx, orgID, contactID)
	if err != nil {
		return nil, err
	}

	dossiers := make([]map[string]any, 0, len(files))
	for _, id := range files {
		file, err := s.GetFile(ctx, orgID, id)
		if err != nil {
			continue
		}
		events, err := s.Timeline(ctx, id)
		if err != nil {
			continue
		}
		dossiers = append(dossiers, map[string]any{
			"dossier": file, "journal": events,
		})
	}

	return map[string]any{
		"personne":  contact,
		"dossiers":  dossiers,
		"extraitLe": time.Now().UTC().Format(time.RFC3339),
	}, nil
}
