package video

import (
	"context"
	"fmt"
	"time"

	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// record est la forme persistée d'un asset.
type record struct {
	ddb.Record
	Asset
}

// Uploader produit les URL de dépôt et de lecture directe sur S3.
type Uploader interface {
	PresignedPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
}

// Objects donne accès au contenu du compartiment vidéo. Seuls les manifestes
// y sont lus : la vidéo elle-même ne transite jamais par l'API.
type Objects interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

// Service porte le cycle de vie d'une vidéo : réservation, dépôt, transcodage,
// diffusion.
type Service struct {
	db       *ddb.Client
	uploader Uploader
	encoder  Encoder
	signer   *Signer
	blobs    Objects
	bucket   string
	now      func() time.Time
}

// Deps regroupe les dépendances.
type Deps struct {
	DB       *ddb.Client
	Uploader Uploader
	Encoder  Encoder
	Signer   *Signer
	Objects  Objects
	Bucket   string
	Now      func() time.Time
}

// NewService construit le service.
func NewService(deps Deps) *Service {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		db: deps.DB, uploader: deps.Uploader, encoder: deps.Encoder,
		signer: deps.Signer, blobs: deps.Objects, bucket: deps.Bucket, now: now,
	}
}

// UploadTTL borne la validité d'une URL de dépôt.
//
// Une heure : de quoi téléverser un module d'une heure sur une connexion
// modeste, sans laisser traîner une autorisation d'écriture dans un
// historique de navigation.
const UploadTTL = time.Hour

// Reserve crée un emplacement et renvoie l'URL de dépôt direct.
//
// Le fichier ne transite jamais par l'API : le navigateur écrit dans S3, et
// nous ne voyons que la confirmation. Faire passer une vidéo par une Lambda
// imposerait de la dimensionner pour des octets dont elle n'a rien à faire.
func (s *Service) Reserve(ctx context.Context, orgID, contentType string) (Asset, string, error) {
	if s.uploader == nil {
		return Asset{}, "", fmt.Errorf("video: dépôt indisponible")
	}
	if contentType == "" {
		contentType = "video/mp4"
	}

	asset := NewAsset(orgID, s.now())
	url, err := s.uploader.PresignedPut(ctx, asset.SourceKey, contentType, UploadTTL)
	if err != nil {
		return Asset{}, "", err
	}
	if err := s.save(ctx, asset); err != nil {
		return Asset{}, "", err
	}
	return asset, url, nil
}

// Uploaded déclenche le transcodage d'une source déposée.
//
// L'appel est conditionné à l'état : relancer un transcodage déjà en cours
// facturerait deux fois le même encodage, ce qui est le poste de coût
// principal d'une plateforme vidéo.
func (s *Service) Uploaded(ctx context.Context, orgID, assetID string, durationMs int64, actor audit.Actor) (Asset, error) {
	asset, err := s.Get(ctx, orgID, assetID)
	if err != nil {
		return Asset{}, err
	}
	if asset.Status != StatusAwaiting && asset.Status != StatusFailed {
		return asset, fmt.Errorf("cette vidéo est déjà %s", asset.Status)
	}
	if s.encoder == nil {
		return Asset{}, fmt.Errorf("video: transcodage indisponible")
	}

	jobID, err := s.encoder.Start(ctx, asset, s.bucket)
	if err != nil {
		asset.Status = StatusFailed
		asset.Error = err.Error()
		_ = s.save(ctx, asset)
		return asset, err
	}

	asset.Status = StatusEncoding
	asset.JobID = jobID
	asset.Error = ""
	if durationMs > 0 {
		asset.DurationMs = durationMs
	}
	if err := s.save(ctx, asset); err != nil {
		return Asset{}, err
	}
	return asset, nil
}

// Refresh interroge l'encodeur si le transcodage est en cours.
//
// L'état est rafraîchi à la lecture plutôt que poussé par une notification :
// c'est l'interface d'administration qui interroge, elle le fait de toute
// façon en boucle pendant l'encodage, et cela évite une file d'événements à
// exploiter et à surveiller pour un fait qu'on peut simplement demander.
func (s *Service) Refresh(ctx context.Context, orgID, assetID string) (Asset, error) {
	asset, err := s.Get(ctx, orgID, assetID)
	if err != nil {
		return Asset{}, err
	}
	if asset.Status != StatusEncoding || asset.JobID == "" {
		return asset, nil
	}

	watcher, ok := s.encoder.(JobWatcher)
	if !ok {
		return asset, nil
	}
	state, duration, err := watcher.Status(ctx, asset.JobID)
	if err != nil {
		// Une panne de l'encodeur ne doit pas faire disparaître la vidéo :
		// l'état connu est renvoyé tel quel.
		return asset, nil
	}

	switch state {
	case JobComplete:
		now := s.now()
		asset.Status = StatusReady
		asset.MasterKey = MasterKeyFor(asset.ID)
		asset.ReadyAt = &now
		if duration > 0 {
			asset.DurationMs = duration
		}
	case JobFailed:
		asset.Status = StatusFailed
		asset.Error = "le transcodage a échoué"
	default:
		return asset, nil
	}

	if err := s.save(ctx, asset); err != nil {
		return Asset{}, err
	}
	return asset, nil
}

// Playback autorise la lecture d'une vidéo prête.
func (s *Service) Playback(ctx context.Context, orgID, assetID string) (Playback, error) {
	asset, err := s.Refresh(ctx, orgID, assetID)
	if err != nil {
		return Playback{}, err
	}
	if s.signer == nil {
		return Playback{}, fmt.Errorf("video: diffusion non configurée")
	}
	return s.signer.Authorize(ctx, asset)
}

// Get lit un asset.
func (s *Service) Get(ctx context.Context, orgID, assetID string) (Asset, error) {
	item, err := ddb.Get[record](ctx, s.db, ddb.OrgPK(orgID), ddb.AssetSK(assetID))
	if err != nil {
		return Asset{}, err
	}
	return item.Asset, nil
}

// List renvoie les vidéos d'une organisation, la plus récente en tête.
func (s *Service) List(ctx context.Context, orgID string, limit int32) ([]Asset, error) {
	items, err := ddb.Query[record](ctx, s.db, ddb.QuerySpec{
		PK: ddb.OrgPK(orgID), SKPrefix: "ASSET#", Descending: true, Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	assets := make([]Asset, 0, len(items))
	for _, item := range items {
		assets = append(assets, item.Asset)
	}
	return assets, nil
}

func (s *Service) save(ctx context.Context, asset Asset) error {
	now := s.now()
	return ddb.Put(ctx, s.db, record{
		Record: ddb.Record{
			PK: ddb.OrgPK(asset.OrgID), SK: ddb.AssetSK(asset.ID),
			Type: "video_asset", CreatedAt: asset.CreatedAt, UpdatedAt: now,
		},
		Asset: asset,
	})
}
