package crm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// IdentityDocTTL borne la validité d'un lien vers une pièce d'identité.
//
// Une minute : le temps d'ouvrir le document, pas celui de le faire circuler.
// Un lien plus long finirait dans un historique de navigation, un presse-papier
// ou un message — et une carte d'identité qui circule est exactement ce que la
// CNIL reproche.
const IdentityDocTTL = time.Minute

// IdentityUploadTTL borne la validité d'un lien de dépôt.
const IdentityUploadTTL = 10 * time.Minute

// DocStore présigne les accès au compartiment chiffré des pièces d'identité.
type DocStore interface {
	PresignedPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
	PresignedGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Delete(ctx context.Context, key string) error
}

// WithDocs branche le compartiment des pièces d'identité.
func (s *Service) WithDocs(store DocStore) *Service {
	s.docs = store
	return s
}

// identityDocKey range la pièce sous l'organisation et le contact.
//
// L'extension est conservée : un navigateur qui reçoit un PDF annoncé en JPEG
// propose de le télécharger au lieu de l'afficher, et l'agent recommence.
func identityDocKey(orgID, contactID, filename string) string {
	extension := ".bin"
	if cut := strings.LastIndex(filename, "."); cut >= 0 && len(filename)-cut <= 6 {
		extension = strings.ToLower(filename[cut:])
	}
	return fmt.Sprintf("orgs/%s/contacts/%s/piece-identite%s", orgID, contactID, extension)
}

// allowedIdentityTypes borne ce qu'on accepte comme pièce.
//
// Refuser le reste n'est pas du zèle : un fichier arbitraire dans un
// compartiment qu'on présigne ensuite en lecture est un hébergement ouvert.
var allowedIdentityTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/heic":      true,
	"application/pdf": true,
}

// PrepareIdentityDoc renvoie l'URL de dépôt de la pièce d'identité.
//
// Le fichier monte directement vers S3, chiffré par la clé KMS du
// compartiment : le faire transiter par l'API mettrait une carte d'identité
// dans les journaux d'une fonction au premier incident.
func (s *Service) PrepareIdentityDoc(ctx context.Context, orgID, contactID, filename, contentType string) (uploadURL, key string, err error) {
	if s.docs == nil {
		return "", "", fmt.Errorf("le dépôt des pièces d'identité n'est pas configuré")
	}
	if !allowedIdentityTypes[contentType] {
		return "", "", fmt.Errorf("format %q refusé : une pièce d'identité se dépose en JPEG, PNG, HEIC ou PDF", contentType)
	}
	if _, err := s.GetContact(ctx, orgID, contactID); err != nil {
		return "", "", err
	}

	key = identityDocKey(orgID, contactID, filename)
	uploadURL, err = s.docs.PresignedPut(ctx, key, contentType, IdentityUploadTTL)
	if err != nil {
		return "", "", err
	}
	return uploadURL, key, nil
}

// AttachIdentityDoc enregistre la pièce déposée sur la fiche du contact.
func (s *Service) AttachIdentityDoc(ctx context.Context, orgID, contactID, key string, actor audit.Actor) (Contact, error) {
	contact, err := s.GetContact(ctx, orgID, contactID)
	if err != nil {
		return Contact{}, err
	}
	if key != identityDocKey(orgID, contactID, key) && !strings.HasPrefix(key, fmt.Sprintf("orgs/%s/contacts/%s/", orgID, contactID)) {
		// La clé vient du client : sans cette vérification, il pourrait
		// rattacher la pièce d'un autre apprenant à sa propre fiche.
		return Contact{}, fmt.Errorf("cette pièce n'appartient pas à ce contact")
	}

	contact.IdentityDocKey = key
	contact.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, contact); err != nil {
		return Contact{}, err
	}

	// Le dépôt est journalisé sur les dossiers de l'apprenant : la pièce
	// d'identité est une des treize pièces attendues d'un dossier complet, et
	// son arrivée doit se voir dans la chaîne, pas seulement sur la fiche.
	s.auditOnFiles(ctx, orgID, contactID, audit.ActionConsentGiven, actor, map[string]any{
		"piece": "identité",
		"etat":  "déposée",
	})
	return contact, nil
}

// IdentityDocURL produit un lien de lecture d'une minute, et le journalise.
//
// Consulter une pièce d'identité est un accès à une donnée sensible : savoir
// qui l'a ouverte, et quand, fait partie de ce qu'un contrôle CNIL peut
// demander.
func (s *Service) IdentityDocURL(ctx context.Context, orgID, contactID string, actor audit.Actor) (string, error) {
	if s.docs == nil {
		return "", fmt.Errorf("le dépôt des pièces d'identité n'est pas configuré")
	}
	contact, err := s.GetContact(ctx, orgID, contactID)
	if err != nil {
		return "", err
	}
	if contact.IdentityDocKey == "" {
		return "", fmt.Errorf("aucune pièce d'identité déposée pour ce contact")
	}

	url, err := s.docs.PresignedGet(ctx, contact.IdentityDocKey, IdentityDocTTL)
	if err != nil {
		return "", err
	}

	s.auditOnFiles(ctx, orgID, contactID, audit.ActionDocumentSent, actor, map[string]any{
		"piece":    "identité",
		"etat":     "consultée",
		"validite": IdentityDocTTL.String(),
	})
	return url, nil
}

// DeleteIdentityDoc retire la pièce du compartiment et de la fiche.
//
// La recommandation CNIL est de ne pas conserver une pièce d'identité au-delà
// de la vérification du dossier. Le compartiment l'efface seul au bout de
// quatre-vingt-dix jours ; ceci permet de le faire plus tôt, à la main.
func (s *Service) DeleteIdentityDoc(ctx context.Context, orgID, contactID string, actor audit.Actor) (Contact, error) {
	contact, err := s.GetContact(ctx, orgID, contactID)
	if err != nil {
		return Contact{}, err
	}
	if contact.IdentityDocKey == "" {
		return contact, nil
	}

	if s.docs != nil {
		if err := s.docs.Delete(ctx, contact.IdentityDocKey); err != nil {
			return Contact{}, err
		}
	}
	contact.IdentityDocKey = ""
	contact.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, contact); err != nil {
		return Contact{}, err
	}

	s.auditOnFiles(ctx, orgID, contactID, audit.ActionLearnerAnonymized, actor, map[string]any{
		"piece": "identité",
		"etat":  "supprimée",
	})
	return contact, nil
}

// auditOnFiles journalise un fait sur chaque dossier où l'apprenant figure.
//
// Sans dossier, le fait n'est pas perdu : il n'a simplement pas de chaîne à
// rejoindre, et la fiche du contact en porte la trace par sa date de mise à
// jour.
func (s *Service) auditOnFiles(ctx context.Context, orgID, contactID string,
	action audit.Action, actor audit.Actor, payload map[string]any) {
	files, err := s.filesOfLearner(ctx, orgID, contactID)
	if err != nil {
		return
	}
	now := s.now()
	for _, fileID := range files {
		subject := FileSubject(fileID)
		_, _ = s.db.WriteWithAudit(ctx, subject, nil,
			func(prev audit.Event) (audit.Event, error) {
				return audit.Append(prev, subject, now, action, actor, payload)
			})
	}
}

// RecordAccess journalise l'ouverture d'un accès à l'espace apprenant.
//
// Depuis quand un apprenant pouvait se connecter éclaire une contestation
// d'assiduité : « il n'a jamais reçu ses accès » se vérifie.
func (s *Service) RecordAccess(ctx context.Context, orgID, contactID string,
	actor audit.Actor, payload map[string]any) (int, error) {
	files, err := s.filesOfLearner(ctx, orgID, contactID)
	if err != nil {
		return 0, err
	}

	now := s.now()
	for _, fileID := range files {
		subject := FileSubject(fileID)
		if _, err := s.db.WriteWithAudit(ctx, subject, nil,
			func(prev audit.Event) (audit.Event, error) {
				return audit.Append(prev, subject, now, audit.ActionConsentGiven, actor, payload)
			}); err != nil {
			return 0, err
		}
	}
	return len(files), nil
}
