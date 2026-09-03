package crm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// Le suivi d'une fiche : ce qu'on a dit, ce qu'on doit faire, ce qu'on a reçu.
//
// Trois choses qu'une assistante de formation ouvre plus souvent que tout le
// reste, et qui n'existaient pas. Sans elles, un échange téléphonique ne
// laissait aucune trace, un rappel se notait sur un papier, et une attestation
// employeur reçue par courriel restait dans une boîte.
//
// Elles vivent dans la partition de l'organisme, sous la clé du contact : les
// lire coûte une requête, et elles disparaissent avec la fiche le jour d'une
// anonymisation.

// PieceUploadTTL borne la durée d'un lien de dépôt.
const PieceUploadTTL = 10 * time.Minute

// LectureTTL borne la durée d'un lien de lecture. Court, parce qu'un lien de
// lecture qui traîne dans un historique de navigation est un document qui fuit.
const LectureTTL = 2 * time.Minute

func noteSK(contactID, id string) string   { return "CONTACT#" + contactID + "#NOTE#" + id }
func rappelSK(contactID, id string) string { return "CONTACT#" + contactID + "#RAPPEL#" + id }
func pieceSK(contactID, id string) string  { return "CONTACT#" + contactID + "#PIECE#" + id }

// Note est ce qu'on a dit ou entendu, daté et signé.
type Note struct {
	ddb.Record

	ID        string `dynamodbav:"id" json:"id"`
	OrgID     string `dynamodbav:"orgId" json:"orgId"`
	ContactID string `dynamodbav:"contactId" json:"contactId"`

	Body string `dynamodbav:"body" json:"body"`
	// Author est le nom de qui a écrit, pas son identifiant : une note se lit
	// des mois plus tard, parfois par quelqu'un d'autre.
	Author string `dynamodbav:"author" json:"author"`
}

// Rappel est quelque chose à faire, à une date, par quelqu'un.
type Rappel struct {
	ddb.Record

	ID        string `dynamodbav:"id" json:"id"`
	OrgID     string `dynamodbav:"orgId" json:"orgId"`
	ContactID string `dynamodbav:"contactId" json:"contactId"`

	Title string `dynamodbav:"title" json:"title"`
	// DueOn est une date sans heure : « rappeler jeudi » ne se pense pas à la
	// minute, et une heure inventée ferait sonner faux.
	DueOn string `dynamodbav:"dueOn" json:"dueOn"`
	// AssigneeID et AssigneeName désignent qui s'en occupe. Le nom est copié
	// pour que la liste se lise sans une requête par ligne.
	AssigneeID   string `dynamodbav:"assigneeId,omitempty" json:"assigneeId,omitempty"`
	AssigneeName string `dynamodbav:"assigneeName,omitempty" json:"assigneeName,omitempty"`

	DoneAt   *time.Time `dynamodbav:"doneAt,omitempty" json:"doneAt,omitempty"`
	DoneBy   string     `dynamodbav:"doneBy,omitempty" json:"doneBy,omitempty"`
	Author   string     `dynamodbav:"author" json:"author"`
	Comments string     `dynamodbav:"comments,omitempty" json:"comments,omitempty"`
}

// Piece est un document reçu, rattaché à la fiche.
//
// Ce n'est pas la pièce d'identité, qui vit dans un compartiment chiffré à part
// avec ses propres règles de conservation : c'est tout le reste — une
// attestation employeur, un accord de prise en charge, un devis signé à la
// main.
type Piece struct {
	ddb.Record

	ID        string `dynamodbav:"id" json:"id"`
	OrgID     string `dynamodbav:"orgId" json:"orgId"`
	ContactID string `dynamodbav:"contactId" json:"contactId"`

	Name        string `dynamodbav:"name" json:"name"`
	Key         string `dynamodbav:"key" json:"-"`
	ContentType string `dynamodbav:"contentType,omitempty" json:"contentType,omitempty"`
	SizeBytes   int64  `dynamodbav:"sizeBytes,omitempty" json:"sizeBytes,omitempty"`
	Author      string `dynamodbav:"author" json:"author"`
}

// --- Notes ---------------------------------------------------------------

// AddNote écrit une note sur une fiche.
func (s *Service) AddNote(ctx context.Context, orgID, contactID, body, author string) (Note, error) {
	if strings.TrimSpace(body) == "" {
		return Note{}, fmt.Errorf("une note vide n'apprend rien")
	}
	if _, err := s.GetContact(ctx, orgID, contactID); err != nil {
		return Note{}, err
	}

	now := s.now()
	id := identity.NewID()
	note := Note{
		Record: ddb.Record{
			PK: ddb.OrgPK(orgID), SK: noteSK(contactID, id),
			Type: "note", CreatedAt: now, UpdatedAt: now,
		},
		ID: id, OrgID: orgID, ContactID: contactID,
		Body: strings.TrimSpace(body), Author: author,
	}
	if err := ddb.Put(ctx, s.db, note); err != nil {
		return Note{}, err
	}
	return note, nil
}

// ListNotes rend les notes d'une fiche, la plus récente d'abord.
func (s *Service) ListNotes(ctx context.Context, orgID, contactID string) ([]Note, error) {
	return ddb.Query[Note](ctx, s.db, ddb.QuerySpec{
		PK: ddb.OrgPK(orgID), SKPrefix: "CONTACT#" + contactID + "#NOTE#",
		Descending: true,
	})
}

// DeleteNote retire une note.
func (s *Service) DeleteNote(ctx context.Context, orgID, contactID, noteID string) error {
	return ddb.Delete(ctx, s.db, ddb.OrgPK(orgID), noteSK(contactID, noteID))
}

// --- Rappels -------------------------------------------------------------

// RappelInput décrit un rappel à poser.
type RappelInput struct {
	OrgID        string
	ContactID    string
	Title        string
	DueOn        string
	AssigneeID   string
	AssigneeName string
	Comments     string
	Author       string
}

// AddRappel pose un rappel sur une fiche.
func (s *Service) AddRappel(ctx context.Context, in RappelInput) (Rappel, error) {
	if strings.TrimSpace(in.Title) == "" {
		return Rappel{}, fmt.Errorf("un rappel doit dire ce qu'il y a à faire")
	}
	if _, err := time.Parse("2006-01-02", in.DueOn); err != nil {
		return Rappel{}, fmt.Errorf("une date au format AAAA-MM-JJ est attendue")
	}
	if _, err := s.GetContact(ctx, in.OrgID, in.ContactID); err != nil {
		return Rappel{}, err
	}

	now := s.now()
	id := identity.NewID()
	rappel := Rappel{
		Record: ddb.Record{
			PK: ddb.OrgPK(in.OrgID), SK: rappelSK(in.ContactID, id),
			Type: "rappel", CreatedAt: now, UpdatedAt: now,
		},
		ID: id, OrgID: in.OrgID, ContactID: in.ContactID,
		Title: strings.TrimSpace(in.Title), DueOn: in.DueOn,
		AssigneeID: in.AssigneeID, AssigneeName: in.AssigneeName,
		Comments: in.Comments, Author: in.Author,
	}
	if err := ddb.Put(ctx, s.db, rappel); err != nil {
		return Rappel{}, err
	}
	return rappel, nil
}

// ListRappels rend les rappels d'une fiche, du plus ancien au plus récent.
//
// Dans cet ordre-ci, contrairement aux notes : un rappel se lit par échéance,
// et le plus urgent est le plus vieux.
func (s *Service) ListRappels(ctx context.Context, orgID, contactID string) ([]Rappel, error) {
	rappels, err := ddb.Query[Rappel](ctx, s.db, ddb.QuerySpec{
		PK: ddb.OrgPK(orgID), SKPrefix: "CONTACT#" + contactID + "#RAPPEL#",
	})
	if err != nil {
		return nil, err
	}
	// Le tri se fait sur l'échéance et non sur la clé : deux rappels posés le
	// même jour pour des dates différentes se liraient sinon à l'envers.
	for i := 1; i < len(rappels); i++ {
		for j := i; j > 0 && rappels[j].DueOn < rappels[j-1].DueOn; j-- {
			rappels[j], rappels[j-1] = rappels[j-1], rappels[j]
		}
	}
	return rappels, nil
}

// CloseRappel marque un rappel fait, ou le rouvre.
func (s *Service) CloseRappel(
	ctx context.Context, orgID, contactID, rappelID string, done bool, by string,
) (Rappel, error) {
	rappel, err := ddb.Get[Rappel](ctx, s.db, ddb.OrgPK(orgID), rappelSK(contactID, rappelID))
	if err != nil {
		return Rappel{}, err
	}
	now := s.now()
	if done {
		rappel.DoneAt, rappel.DoneBy = &now, by
	} else {
		rappel.DoneAt, rappel.DoneBy = nil, ""
	}
	rappel.UpdatedAt = now
	if err := ddb.Put(ctx, s.db, rappel); err != nil {
		return Rappel{}, err
	}
	return rappel, nil
}

// DeleteRappel retire un rappel.
func (s *Service) DeleteRappel(ctx context.Context, orgID, contactID, rappelID string) error {
	return ddb.Delete(ctx, s.db, ddb.OrgPK(orgID), rappelSK(contactID, rappelID))
}

// --- Pièces jointes ------------------------------------------------------

// PreparePiece signe le dépôt d'une pièce et rend sa clé.
func (s *Service) PreparePiece(
	ctx context.Context, orgID, contactID, filename, contentType string,
) (uploadURL, key string, err error) {
	if s.docs == nil {
		return "", "", fmt.Errorf("le dépôt des pièces n'est pas configuré")
	}
	if strings.TrimSpace(filename) == "" {
		return "", "", fmt.Errorf("le nom du fichier est nécessaire")
	}
	if _, err := s.GetContact(ctx, orgID, contactID); err != nil {
		return "", "", err
	}

	// La clé porte un identifiant que nous fabriquons : reprendre le nom du
	// fichier laisserait le client écrire où il veut dans le compartiment.
	extension := ".bin"
	if cut := strings.LastIndex(filename, "."); cut >= 0 && len(filename)-cut <= 6 {
		extension = strings.ToLower(filename[cut:])
	}
	key = fmt.Sprintf("orgs/%s/contacts/%s/pieces/%s%s", orgID, contactID, identity.NewID(), extension)

	uploadURL, err = s.docs.PresignedPut(ctx, key, contentType, PieceUploadTTL)
	if err != nil {
		return "", "", err
	}
	return uploadURL, key, nil
}

// AttachPiece enregistre la pièce déposée sur la fiche.
func (s *Service) AttachPiece(
	ctx context.Context, orgID, contactID, key, name, contentType string, size int64, author string,
) (Piece, error) {
	if _, err := s.GetContact(ctx, orgID, contactID); err != nil {
		return Piece{}, err
	}
	// La clé vient du client : sans cette vérification, il rattacherait à sa
	// fiche un document déposé pour quelqu'un d'autre.
	prefix := fmt.Sprintf("orgs/%s/contacts/%s/pieces/", orgID, contactID)
	if !strings.HasPrefix(key, prefix) {
		return Piece{}, fmt.Errorf("cette pièce n'appartient pas à ce contact")
	}

	now := s.now()
	id := identity.NewID()
	piece := Piece{
		Record: ddb.Record{
			PK: ddb.OrgPK(orgID), SK: pieceSK(contactID, id),
			Type: "piece", CreatedAt: now, UpdatedAt: now,
		},
		ID: id, OrgID: orgID, ContactID: contactID,
		Name: strings.TrimSpace(name), Key: key,
		ContentType: contentType, SizeBytes: size, Author: author,
	}
	if err := ddb.Put(ctx, s.db, piece); err != nil {
		return Piece{}, err
	}
	return piece, nil
}

// ListPieces rend les pièces d'une fiche, la plus récente d'abord.
func (s *Service) ListPieces(ctx context.Context, orgID, contactID string) ([]Piece, error) {
	return ddb.Query[Piece](ctx, s.db, ddb.QuerySpec{
		PK: ddb.OrgPK(orgID), SKPrefix: "CONTACT#" + contactID + "#PIECE#",
		Descending: true,
	})
}

// PieceURL rend un lien de lecture de courte durée.
func (s *Service) PieceURL(ctx context.Context, orgID, contactID, pieceID string) (string, error) {
	if s.docs == nil {
		return "", fmt.Errorf("le dépôt des pièces n'est pas configuré")
	}
	piece, err := ddb.Get[Piece](ctx, s.db, ddb.OrgPK(orgID), pieceSK(contactID, pieceID))
	if err != nil {
		return "", err
	}
	return s.docs.PresignedGet(ctx, piece.Key, LectureTTL)
}

// DeletePiece retire une pièce, de la fiche et du compartiment.
func (s *Service) DeletePiece(ctx context.Context, orgID, contactID, pieceID string) error {
	piece, err := ddb.Get[Piece](ctx, s.db, ddb.OrgPK(orgID), pieceSK(contactID, pieceID))
	if err != nil {
		return err
	}
	// L'article d'abord, le fichier ensuite : dans l'autre sens, un échec
	// laisserait une ligne qui pointe vers un objet disparu.
	if err := ddb.Delete(ctx, s.db, ddb.OrgPK(orgID), piece.SK); err != nil {
		return err
	}
	if s.docs != nil {
		_ = s.docs.Delete(ctx, piece.Key)
	}
	return nil
}
