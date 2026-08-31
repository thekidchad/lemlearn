package crm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// Service porte les cas d'usage du CRM.
type Service struct {
	db   *ddb.Client
	docs DocStore
	now  func() time.Time
}

// NewService construit le service.
func NewService(db *ddb.Client, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{db: db, now: now}
}

// FileSubject est le sujet d'audit d'un dossier. Une seule fonction le
// construit : une chaîne d'audit dont le sujet varie selon l'appelant se
// scinderait en deux journaux sans que personne ne s'en aperçoive.
func FileSubject(fileID string) string { return "file/" + fileID }

// CreateContact enregistre un contact.
//
// Le contact n'a pas de chaîne d'audit propre : il n'est pas une preuve en
// lui-même. Ce qui est audité, ce sont les dossiers dans lesquels il apparaît.
func (s *Service) CreateContact(ctx context.Context, contact Contact) (Contact, error) {
	if err := contact.Validate(); err != nil {
		return Contact{}, err
	}
	now := s.now()
	contact.Reindex(now)
	if err := ddb.Put(ctx, s.db, contact); err != nil {
		return Contact{}, err
	}
	return contact, nil
}

// UpdateContact remplace un contact existant.
func (s *Service) UpdateContact(ctx context.Context, contact Contact) (Contact, error) {
	if err := contact.Validate(); err != nil {
		return Contact{}, err
	}
	now := s.now()
	contact.Reindex(now)
	if err := ddb.Put(ctx, s.db, contact); err != nil {
		return Contact{}, err
	}
	return contact, nil
}

// GetContact lit un contact.
func (s *Service) GetContact(ctx context.Context, orgID, contactID string) (Contact, error) {
	return ddb.Get[Contact](ctx, s.db, ddb.OrgPK(orgID), ddb.ContactSK(contactID))
}

// ListContacts liste les contacts d'une nature donnée, par ordre alphabétique.
func (s *Service) ListContacts(ctx context.Context, orgID string, kind Kind, limit int32) ([]Contact, error) {
	if !kind.Valid() {
		return nil, fmt.Errorf("nature de contact %q inconnue", kind)
	}
	return ddb.Query[Contact](ctx, s.db, ddb.QuerySpec{
		Index: "GSI1",
		PK:    ddb.GSI1Contacts(orgID, string(kind)),
		Limit: limit,
	})
}

// SearchContacts retrouve des contacts d'un organisme sur un fragment.
//
// La lecture se fait par nature, sur l'index qui sert déjà les listes, et le
// filtre est appliqué en mémoire. Deux raisons plutôt qu'une :
//
//   - DynamoDB ne sait comparer que des préfixes, or personne ne cherche par le
//     début d'une clé : on tape « bertrand » en pensant à Léa Bertrand, et
//     c'est au milieu.
//   - La partition d'autocomplétion (GSI2) ne projette que trois attributs,
//     dont un qui n'est pas stocké — elle ne rend donc pas de fiche lisible.
//     La corriger imposerait de reconstruire l'index sur la table vivante ;
//     tant que le volume tient dans une lecture bornée, ce n'est pas le moment.
//
// Le parcours est borné et ne touche qu'un organisme. À l'échelle où un
// organisme compterait des dizaines de milliers de fiches, il faudra un vrai
// index de recherche, pas une lecture plus longue.
func (s *Service) SearchContacts(ctx context.Context, orgID, needle string, limit int) ([]Contact, error) {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return nil, nil
	}

	found := make([]Contact, 0, limit)
	for _, kind := range []Kind{KindLearner, KindCompany, KindFunder} {
		if len(found) >= limit {
			break
		}
		items, err := s.ListContacts(ctx, orgID, kind, searchDepth)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if len(found) >= limit {
				break
			}
			haystack := strings.ToLower(strings.Join([]string{
				item.DisplayName(), item.Email, item.CompanyName, item.Phone, item.SIRET,
			}, " "))
			if strings.Contains(haystack, needle) {
				found = append(found, item)
			}
		}
	}
	return found, nil
}

// searchDepth borne la lecture des contacts d'un organisme, par nature.
const searchDepth = 500

// ListContactsPage lit une tranche de contacts et rend le curseur de la
// suivante.
//
// La pagination est par curseur et non par numéro de page : DynamoDB ne sait
// pas sauter au millième élément sans lire les neuf cent quatre-vingt-dix-neuf
// premiers, et un écran qui promettrait « page 7 » paierait ce parcours à
// chaque clic.
func (s *Service) ListContactsPage(ctx context.Context, orgID string, kind Kind, limit int32, cursor string) (ddb.Page[Contact], error) {
	if !kind.Valid() {
		return ddb.Page[Contact]{}, fmt.Errorf("nature de contact %q inconnue", kind)
	}
	return ddb.QueryPage[Contact](ctx, s.db, ddb.QuerySpec{
		Index: "GSI1",
		PK:    ddb.GSI1Contacts(orgID, string(kind)),
		Limit: limit,
	}, cursor)
}

// ListFilesByStagePage lit une tranche de dossiers d'une étape.
func (s *Service) ListFilesByStagePage(ctx context.Context, orgID string, stage Stage, limit int32, cursor string) (ddb.Page[File], error) {
	return ddb.QueryPage[File](ctx, s.db, ddb.QuerySpec{
		Index:      "GSI1",
		PK:         ddb.GSI1Files(orgID, string(stage)),
		Descending: true,
		Limit:      limit,
	}, cursor)
}

// CreateFileInput décrit l'ouverture d'un dossier.
type CreateFileInput struct {
	OrgID     string
	Title     string
	LearnerID string
	CompanyID string
	FunderID  string
	CourseID  string
	PriceHT   float64
	Tags      []string
	Actor     audit.Actor
}

// CreateFile ouvre un dossier et pose le premier maillon de sa chaîne d'audit.
func (s *Service) CreateFile(ctx context.Context, in CreateFileInput) (File, error) {
	if strings.TrimSpace(in.Title) == "" {
		return File{}, fmt.Errorf("l'intitulé du dossier est obligatoire")
	}

	now := s.now()
	file := NewFile(in.OrgID, s.nextReference(now), in.Title, now)
	file.LearnerID = in.LearnerID
	file.CompanyID = in.CompanyID
	file.FunderID = in.FunderID
	file.CourseID = in.CourseID
	file.PriceHT = in.PriceHT
	file.Tags = in.Tags
	file.OwnerID = in.Actor.ID
	file.Reindex(now)

	_, err := s.db.WriteWithAudit(ctx, FileSubject(file.ID),
		[]ddb.Write{{Item: file, Condition: "attribute_not_exists(SK)"}},
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, FileSubject(file.ID), now, audit.ActionFileCreated, in.Actor,
				map[string]any{
					"reference": file.Reference,
					"title":     file.Title,
					"learnerId": file.LearnerID,
					"priceHT":   file.PriceHT,
				})
		})
	if err != nil {
		return File{}, err
	}
	return file, nil
}

// GetFile lit un dossier.
func (s *Service) GetFile(ctx context.Context, orgID, fileID string) (File, error) {
	return ddb.Get[File](ctx, s.db, ddb.OrgPK(orgID), ddb.FileSK(fileID))
}

// ListFilesByStage liste une colonne du pipeline, la plus récemment modifiée
// en tête.
func (s *Service) ListFilesByStage(ctx context.Context, orgID string, stage Stage, limit int32) ([]File, error) {
	if !stage.Valid() {
		return nil, fmt.Errorf("étape %q inconnue", stage)
	}
	return ddb.Query[File](ctx, s.db, ddb.QuerySpec{
		Index:      "GSI1",
		PK:         ddb.GSI1Files(orgID, string(stage)),
		Descending: true,
		Limit:      limit,
	})
}

// Pipeline renvoie toutes les colonnes, dans l'ordre du parcours commercial.
func (s *Service) Pipeline(ctx context.Context, orgID string, perStage int32) (map[Stage][]File, error) {
	stages := []Stage{StageProspect, StageQuote, StageAgreement, StageInTraining, StageClosed}
	out := make(map[Stage][]File, len(stages))
	for _, stage := range stages {
		files, err := s.ListFilesByStage(ctx, orgID, stage, perStage)
		if err != nil {
			return nil, err
		}
		out[stage] = files
	}
	return out, nil
}

// SetFunding renseigne l'origine des fonds d'un dossier.
func (s *Service) SetFunding(ctx context.Context, orgID, fileID string, source FundingSource) (File, error) {
	file, err := s.GetFile(ctx, orgID, fileID)
	if err != nil {
		return File{}, err
	}
	file.Funding = source
	file.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, file); err != nil {
		return File{}, err
	}
	return file, nil
}

// MoveFile change l'étape d'un dossier.
//
// L'écriture est conditionnée à l'étape de départ : deux utilisateurs qui
// déplacent la même carte en même temps ne peuvent pas produire un dossier
// dont l'historique dit une chose et l'état une autre.
func (s *Service) MoveFile(ctx context.Context, orgID, fileID string, to Stage, actor audit.Actor) (File, error) {
	if !to.Valid() {
		return File{}, fmt.Errorf("étape %q inconnue", to)
	}

	file, err := s.GetFile(ctx, orgID, fileID)
	if err != nil {
		return File{}, err
	}
	from := file.Stage
	if from == to {
		return file, nil
	}

	now := s.now()
	file.Stage = to
	if to == StageClosed {
		file.ClosedAt = &now
	}
	file.Reindex(now)

	_, err = s.db.WriteWithAudit(ctx, FileSubject(fileID),
		[]ddb.Write{{
			Item:      file,
			Condition: "stage = :from",
			Values:    ddb.StringValues(map[string]string{":from": string(from)}),
		}},
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, FileSubject(fileID), now, audit.ActionFileStageChanged, actor,
				map[string]any{"from": string(from), "to": string(to)})
		})
	if err != nil {
		return File{}, err
	}
	return file, nil
}

// RecordExport journalise l'extraction d'un dossier.
//
// Savoir qui a extrait un dossier, quand, et avec combien de pièces fait
// partie de ce qu'un contrôle peut demander — et c'est aussi la trace qui
// permet de répondre à un apprenant qui s'inquiète de la diffusion de ses
// données.
func (s *Service) RecordExport(
	ctx context.Context, orgID, fileID string,
	actor audit.Actor, details map[string]any,
) (audit.Event, error) {
	if _, err := s.GetFile(ctx, orgID, fileID); err != nil {
		return audit.Event{}, err
	}
	now := s.now()
	return s.db.WriteWithAudit(ctx, FileSubject(fileID), nil,
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, FileSubject(fileID), now, audit.ActionDossierExported, actor, details)
		})
}

// Timeline renvoie le journal vérifié d'un dossier.
func (s *Service) Timeline(ctx context.Context, fileID string) ([]audit.Event, error) {
	return s.db.AuditChain(ctx, FileSubject(fileID))
}

// nextReference compose une référence lisible : DOS-2026-XXXXXX.
//
// Le suffixe vient de l'horloge et non d'un compteur : un compteur global
// imposerait une écriture sérialisée par organisation à chaque création de
// dossier, pour un gain purement cosmétique.
func (s *Service) nextReference(now time.Time) string {
	return fmt.Sprintf("DOS-%d-%06d", now.Year(), now.Unix()%1000000)
}
