package brand

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/platform/ddb"
)

// Service lit et écrit la marque d'un organisme.
type Service struct {
	db        *ddb.Client
	assetsURL string
	now       func() time.Time
}

// NewService construit le service.
func NewService(db *ddb.Client, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{db: db, now: now}
}

// WithAssets indique où sont servies les ressources publiques : c'est de là
// que se compose l'adresse du logo.
func (s *Service) WithAssets(assetsURL string) *Service {
	s.assetsURL = strings.TrimSuffix(assetsURL, "/")
	return s
}

// AssetsURL expose l'adresse publique, pour les appelants qui composent
// eux-mêmes une URL de ressource.
func (s *Service) AssetsURL() string { return s.assetsURL }

// Get lit la marque brute. Une organisation qui n'en a jamais posé renvoie
// une marque vide, pas une erreur : l'absence de personnalisation est l'état
// normal d'un organisme qui vient d'ouvrir son compte.
func (s *Service) Get(ctx context.Context, orgID string) (Brand, error) {
	b, err := ddb.Get[Brand](ctx, s.db, ddb.OrgPK(orgID), SK)
	if errors.Is(err, ddb.ErrNotFound) {
		return Brand{}, nil
	}
	if err != nil {
		return Brand{}, fmt.Errorf("brand: lecture: %w", err)
	}
	return b, nil
}

// Resolve lit la marque et la complète, en retombant sur le nom de
// l'organisation.
func (s *Service) Resolve(ctx context.Context, orgID, orgName string) (Public, error) {
	b, err := s.Get(ctx, orgID)
	if err != nil {
		return Public{}, err
	}
	return b.Resolve(orgName, s.assetsURL), nil
}

// Save écrit la marque.
//
// L'écriture est entière et non partielle : la marque tient en cinq champs,
// et un formulaire qui en renvoie quatre a supprimé le cinquième — c'est
// justement ainsi qu'on retire un logo.
func (s *Service) Save(ctx context.Context, orgID string, in Brand, by string) (Brand, error) {
	if err := in.Validate(); err != nil {
		return Brand{}, err
	}
	now := s.now()

	existing, err := s.Get(ctx, orgID)
	if err != nil {
		return Brand{}, err
	}
	created := existing.CreatedAt
	if created.IsZero() {
		created = now
	}

	in.Record = ddb.Record{
		PK:        ddb.OrgPK(orgID),
		SK:        SK,
		Type:      "brand",
		CreatedAt: created,
		UpdatedAt: now,
	}
	in.UpdatedBy = by

	if err := ddb.Put(ctx, s.db, in); err != nil {
		return Brand{}, fmt.Errorf("brand: écriture: %w", err)
	}
	return in, nil
}
