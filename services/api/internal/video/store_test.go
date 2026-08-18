package video_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
	"github.com/lemlearn/api/internal/video"
)

// fakeUploader remplace S3 : ce qui est testé ici est le cycle de vie d'un
// asset, pas la génération d'une URL présignée par le SDK.
type fakeUploader struct{ calls int }

func (f *fakeUploader) PresignedPut(_ context.Context, key, _ string, _ time.Duration) (string, error) {
	f.calls++
	return "https://s3.example/" + key + "?signed", nil
}

// fakeEncoder simule MediaConvert, avec un état pilotable par le test.
type fakeEncoder struct {
	started  int
	state    video.JobState
	duration int64
	failNow  bool
}

func (f *fakeEncoder) Start(context.Context, video.Asset, string) (string, error) {
	if f.failNow {
		return "", errors.New("quota de transcodage dépassé")
	}
	f.started++
	return "job-1", nil
}

func (f *fakeEncoder) Status(context.Context, string) (video.JobState, int64, error) {
	return f.state, f.duration, nil
}

func newService(t *testing.T) (*video.Service, *fakeUploader, *fakeEncoder) {
	t.Helper()
	db := ddb.NewTestClient(t)
	up := &fakeUploader{}
	enc := &fakeEncoder{state: video.JobRunning}
	return video.NewService(video.Deps{
		DB: db, Uploader: up, Encoder: enc, Bucket: "lemlearn-video-test",
	}), up, enc
}

var actor = audit.Actor{Type: audit.ActorUser, ID: "USR-1"}

func TestReserveThenEncodeThenReady(t *testing.T) {
	s, up, enc := newService(t)
	ctx := context.Background()

	asset, url, err := s.Reserve(ctx, "ORG1", "video/mp4")
	if err != nil {
		t.Fatalf("réservation: %v", err)
	}
	if up.calls != 1 || url == "" {
		t.Fatalf("aucune URL de dépôt produite")
	}
	if asset.Status != video.StatusAwaiting {
		t.Errorf("état initial = %q", asset.Status)
	}

	if _, err := s.Uploaded(ctx, "ORG1", asset.ID, 600_000, actor); err != nil {
		t.Fatalf("déclenchement du transcodage: %v", err)
	}
	if enc.started != 1 {
		t.Fatalf("%d transcodage(s) lancé(s)", enc.started)
	}

	// Tant que le travail tourne, la vidéo n'est pas diffusable.
	running, err := s.Refresh(ctx, "ORG1", asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if running.Status != video.StatusEncoding {
		t.Errorf("état pendant l'encodage = %q", running.Status)
	}

	enc.state, enc.duration = video.JobComplete, 612_345
	ready, err := s.Refresh(ctx, "ORG1", asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Status != video.StatusReady || ready.ReadyAt == nil {
		t.Fatalf("état après transcodage = %q", ready.Status)
	}
	// La durée mesurée à l'encodage doit primer sur celle déclarée par le
	// navigateur : c'est elle qui sert de dénominateur à l'assiduité.
	if ready.DurationMs != 612_345 {
		t.Errorf("durée = %d ms, attendu celle de l'encodeur (612 345)", ready.DurationMs)
	}
}

// Relancer un transcodage en cours facturerait deux fois le même encodage,
// qui est le poste de coût principal d'une plateforme vidéo.
func TestSecondEncodeIsRefused(t *testing.T) {
	s, _, enc := newService(t)
	ctx := context.Background()

	asset, _, _ := s.Reserve(ctx, "ORG1", "")
	if _, err := s.Uploaded(ctx, "ORG1", asset.ID, 0, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Uploaded(ctx, "ORG1", asset.ID, 0, actor); err == nil {
		t.Fatal("un second transcodage a été accepté")
	}
	if enc.started != 1 {
		t.Errorf("%d transcodage(s) lancé(s)", enc.started)
	}
}

// Un échec de l'encodeur doit laisser la vidéo dans un état réessayable, pas
// bloquée : l'organisme doit pouvoir relancer sans redéposer le fichier.
func TestFailedEncodeCanBeRetried(t *testing.T) {
	s, _, enc := newService(t)
	ctx := context.Background()

	asset, _, _ := s.Reserve(ctx, "ORG1", "")
	enc.failNow = true
	if _, err := s.Uploaded(ctx, "ORG1", asset.ID, 0, actor); err == nil {
		t.Fatal("l'échec de l'encodeur n'a pas été signalé")
	}

	failed, _ := s.Get(ctx, "ORG1", asset.ID)
	if failed.Status != video.StatusFailed || failed.Error == "" {
		t.Fatalf("état après échec = %q, motif %q", failed.Status, failed.Error)
	}

	enc.failNow = false
	if _, err := s.Uploaded(ctx, "ORG1", asset.ID, 0, actor); err != nil {
		t.Fatalf("la reprise après échec est refusée: %v", err)
	}
}

// Une vidéo non transcodée ne se diffuse pas, même à un apprenant inscrit.
func TestPlaybackRefusedBeforeReady(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()

	asset, _, _ := s.Reserve(ctx, "ORG1", "")
	if _, err := s.Playback(ctx, "ORG1", asset.ID); err == nil {
		t.Fatal("une vidéo non prête a été autorisée à la lecture")
	}
}

// Les vidéos d'une organisation ne sont pas visibles d'une autre.
func TestAssetsAreScopedToOrg(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()

	asset, _, _ := s.Reserve(ctx, "ORG1", "")
	if _, err := s.Get(ctx, "ORG2", asset.ID); !errors.Is(err, ddb.ErrNotFound) {
		t.Fatalf("une autre organisation a lu la vidéo: %v", err)
	}

	mine, err := s.List(ctx, "ORG1", 10)
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := s.List(ctx, "ORG2", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 1 || len(theirs) != 0 {
		t.Errorf("cloisonnement rompu : %d ici, %d ailleurs", len(mine), len(theirs))
	}
}
