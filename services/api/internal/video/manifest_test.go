package video_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
	"github.com/lemlearn/api/internal/video"
)

// fakeObjects tient lieu de compartiment vidéo.
type fakeObjects struct{ items map[string][]byte }

func (f *fakeObjects) Size(_ context.Context, key string) (int64, error) {
	data, ok := f.items[key]
	if !ok {
		return 0, ddb.ErrNotFound
	}
	return int64(len(data)), nil
}

func (f *fakeObjects) Get(_ context.Context, key string) ([]byte, error) {
	data, ok := f.items[key]
	if !ok {
		return nil, ddb.ErrNotFound
	}
	return data, nil
}

// readyAsset mène un asset jusqu'à l'état diffusable et renvoie le service.
func readyAsset(t *testing.T, objects *fakeObjects) (*video.Service, video.Asset) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := video.NewSigner("https://video.lemlearn.fr", "K123",
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	if err != nil {
		t.Fatal(err)
	}

	enc := &fakeEncoder{state: video.JobComplete, duration: 600_000}
	s := video.NewService(video.Deps{
		DB: ddb.NewTestClient(t), Uploader: &fakeUploader{}, Encoder: enc,
		Signer: signer, Objects: objects, Bucket: "lemlearn-video-test",
	})

	ctx := context.Background()
	asset, _, err := s.Reserve(ctx, "ORG1", "video/mp4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Uploaded(ctx, "ORG1", asset.ID, 600_000,
		audit.Actor{Type: audit.ActorUser, ID: "USR-1"}); err != nil {
		t.Fatal(err)
	}
	ready, err := s.Refresh(ctx, "ORG1", asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, ready
}

// Le manifeste principal renvoie vers nous pour ses sous-manifestes : leurs
// segments doivent être réécrits à leur tour.
func TestMasterManifestRoutesRenditionsBackThroughUs(t *testing.T) {
	objects := &fakeObjects{items: map[string][]byte{}}
	s, asset := readyAsset(t, objects)
	objects.items[asset.MasterKey] = []byte(strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-STREAM-INF:BANDWIDTH=291274,RESOLUTION=660x360",
		asset.ID + "-360p.m3u8",
		"",
	}, "\n"))

	body, err := s.Manifest(context.Background(), "ORG1", asset.ID, "",
		func(name string) string { return "/lecture?rendu=" + name })
	if err != nil {
		t.Fatalf("manifeste: %v", err)
	}
	if want := "/lecture?rendu=" + asset.ID + "-360p.m3u8"; !strings.Contains(body, want) {
		t.Errorf("manifeste = %q, attendu un renvoi vers %q", body, want)
	}
	if strings.Contains(body, "video.lemlearn.fr") {
		t.Error("un sous-manifeste servi directement par le CDN aurait des segments non signés")
	}
}

// Les segments, eux, partent au CDN, signés — c'est tout l'intérêt : la vidéo
// ne transite jamais par l'API.
func TestRenditionManifestSignsItsSegments(t *testing.T) {
	objects := &fakeObjects{items: map[string][]byte{}}
	s, asset := readyAsset(t, objects)
	objects.items["hls/"+asset.ID+"/"+asset.ID+"-360p.m3u8"] = []byte(strings.Join([]string{
		"#EXTM3U",
		"#EXTINF:6.0,",
		asset.ID + "-360p_00001.ts",
		"#EXT-X-ENDLIST",
	}, "\n"))

	body, err := s.Manifest(context.Background(), "ORG1", asset.ID, asset.ID+"-360p.m3u8", nil)
	if err != nil {
		t.Fatalf("manifeste: %v", err)
	}
	segment := ""
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, ".ts") {
			segment = line
		}
	}
	if !strings.HasPrefix(segment, "https://video.lemlearn.fr/hls/"+asset.ID+"/") {
		t.Fatalf("segment = %q", segment)
	}
	for _, param := range []string{"Policy=", "Signature=", "Key-Pair-Id="} {
		if !strings.Contains(segment, param) {
			t.Errorf("segment sans %s : %q", param, segment)
		}
	}
	// Les balises restent intactes : un lecteur qui ne retrouve pas EXTINF
	// ne sait plus combien dure un segment.
	if !strings.Contains(body, "#EXTINF:6.0,") || !strings.Contains(body, "#EXT-X-ENDLIST") {
		t.Error("les balises du manifeste ont été altérées")
	}
}

// Un nom de rendu est un nom de fichier, pas un chemin : sans cette borne, la
// route deviendrait un lecteur universel du compartiment.
func TestRenditionNameCannotEscapeTheAssetFolder(t *testing.T) {
	objects := &fakeObjects{items: map[string][]byte{}}
	s, asset := readyAsset(t, objects)

	for _, name := range []string{
		"../../autre-org/secret.m3u8",
		"sous/dossier.m3u8",
		asset.ID + "-360p.ts",
	} {
		if _, err := s.Manifest(context.Background(), "ORG1", asset.ID, name, nil); err == nil {
			t.Errorf("le rendu %q a été accepté", name)
		}
	}
}
