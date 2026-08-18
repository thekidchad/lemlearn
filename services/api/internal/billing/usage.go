package billing

import (
	"context"
	"errors"
	"time"

	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/platform/ddb"
	"github.com/lemlearn/api/internal/signature"
	"github.com/lemlearn/api/internal/video"
)

// Service calcule la consommation d'une organisation.
type Service struct {
	db        *ddb.Client
	crm       *crm.Service
	catalog   *catalog.Service
	video     *video.Service
	signature *signature.Service
	now       func() time.Time
}

// Deps regroupe les services interrogés.
type Deps struct {
	DB        *ddb.Client
	Now       func() time.Time
	CRM       *crm.Service
	Catalog   *catalog.Service
	Video     *video.Service
	Signature *signature.Service
}

// NewService construit le service.
func NewService(deps Deps) *Service {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		db: deps.DB, crm: deps.CRM, catalog: deps.Catalog,
		video: deps.Video, signature: deps.Signature, now: now,
	}
}

// CounterSK est la clé du compteur d'usage d'un mois.
//
// Les signatures se comptent au fil de l'eau plutôt qu'en relisant les
// dossiers : c'est un chiffre de facturation, il doit être exact sous
// concurrence, et le recompter demanderait une requête par dossier.
func CounterSK(at time.Time) string { return "USAGE#" + at.UTC().Format("2006-01") }

// CountSignature incrémente le compteur du mois.
func (s *Service) CountSignature(ctx context.Context, orgID string) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.Increment(ctx, ddb.OrgPK(orgID), CounterSK(s.now()), "signatures", 1)
	return err
}

// counter relit le compteur du mois.
type counter struct {
	Signatures int `dynamodbav:"signatures"`
}

// usageLimit borne chaque comptage.
//
// La vue super-admin affiche un ordre de grandeur, pas une facture au
// centime : parcourir cinquante mille inscriptions pour afficher « 50 000+ »
// coûterait plus cher que ce que l'écran rapporte. Le jour où la facturation
// s'appuiera dessus, le compteur viendra des Streams, pas d'une lecture à la
// volée.
const usageLimit = 2000

// Usage constate la consommation d'une organisation.
func (s *Service) Usage(ctx context.Context, orgID string) (Usage, error) {
	var usage Usage

	if s.crm != nil {
		learners, err := s.crm.ListContacts(ctx, orgID, crm.KindLearner, usageLimit)
		if err != nil {
			return Usage{}, err
		}
		usage.Learners = len(learners)

		pipeline, err := s.crm.Pipeline(ctx, orgID, usageLimit)
		if err != nil {
			return Usage{}, err
		}
		for _, files := range pipeline {
			usage.Files += len(files)
		}
	}

	if s.db != nil {
		month, err := ddb.Get[counter](ctx, s.db, ddb.OrgPK(orgID), CounterSK(s.now()))
		if err != nil && !errors.Is(err, ddb.ErrNotFound) {
			return Usage{}, err
		}
		usage.Signatures = month.Signatures
	}

	if s.catalog != nil {
		sessions, err := s.catalog.ListSessions(ctx, orgID, usageLimit)
		if err != nil {
			return Usage{}, err
		}
		usage.Sessions = len(sessions)
	}

	if s.video != nil {
		assets, err := s.video.List(ctx, orgID, usageLimit)
		if err != nil {
			return Usage{}, err
		}
		for _, asset := range assets {
			usage.VideoMs += asset.DurationMs
			// La source déposée, pas les rendus : c'est ce qui est mesuré au
			// dépôt, et les rendus HLS sont du même ordre de grandeur.
			usage.StorageMB += asset.SizeBytes / (1024 * 1024)
		}
	}

	return usage, nil
}
