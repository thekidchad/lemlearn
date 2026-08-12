package video_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/video"
)

func signer(t *testing.T) (*video.Signer, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	s, err := video.NewSigner("https://video.lemlearn.fr", "K123", encoded)
	if err != nil {
		t.Fatalf("signataire: %v", err)
	}
	return s, key
}

// La signature doit être vérifiable avec la clé publique correspondante, en
// reconstruisant exactement la politique que CloudFront reconstruira.
func TestSignedURLVerifiesAgainstPolicy(t *testing.T) {
	s, key := signer(t)

	signed, err := s.Sign("hls/asset-1/asset-1.m3u8", 15*time.Minute)
	if err != nil {
		t.Fatalf("signature: %v", err)
	}

	parsed, err := url.Parse(signed)
	if err != nil {
		t.Fatalf("url illisible: %v", err)
	}
	query := parsed.Query()

	expires, err := strconv.ParseInt(query.Get("Expires"), 10, 64)
	if err != nil {
		t.Fatalf("échéance illisible: %v", err)
	}
	if query.Get("Key-Pair-Id") != "K123" {
		t.Errorf("identifiant de clé = %q", query.Get("Key-Pair-Id"))
	}

	// CloudFront reconstruit la politique à partir de l'URL sans ses
	// paramètres, dans un ordre de champs imposé : Resource puis Condition.
	// La reconstruire ici avec une map échouerait — Go trie les clés — et
	// c'est précisément pourquoi le signataire utilise une structure et non
	// une map.
	resource := parsed.Scheme + "://" + parsed.Host + parsed.Path
	document := []byte(fmt.Sprintf(
		`{"Statement":[{"Resource":%q,"Condition":{"DateLessThan":{"AWS:EpochTime":%d}}}]}`,
		resource, expires))

	raw := strings.NewReplacer("-", "+", "_", "=", "~", "/").Replace(query.Get("Signature"))
	signature, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		t.Fatalf("signature illisible: %v", err)
	}

	digest := sha1.Sum(document)
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA1, digest[:], signature); err != nil {
		t.Fatalf("la signature ne vérifie pas la politique reconstruite: %v", err)
	}
}

// L'alphabet de CloudFront exclut +, = et / : les laisser passer produit des
// signatures rejetées sans message utile.
func TestSignatureUsesCloudFrontAlphabet(t *testing.T) {
	s, _ := signer(t)

	for range 20 {
		signed, err := s.Sign("hls/a/a.m3u8", time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		signature := strings.Split(strings.Split(signed, "Signature=")[1], "&")[0]
		if strings.ContainsAny(signature, "+=/") {
			t.Fatalf("caractère interdit dans la signature : %s", signature)
		}
	}
}

// Une vidéo non transcodée ne se diffuse pas.
func TestUnreadyAssetIsNotPlayable(t *testing.T) {
	s, _ := signer(t)

	for _, status := range []video.Status{video.StatusAwaiting, video.StatusEncoding, video.StatusFailed} {
		asset := video.Asset{ID: "a", Status: status, MasterKey: video.MasterKeyFor("a")}
		if _, err := s.Authorize(t.Context(), asset); err == nil {
			t.Errorf("une vidéo en état %q a été autorisée", status)
		}
	}

	ready := video.Asset{ID: "a", Status: video.StatusReady, MasterKey: video.MasterKeyFor("a"), DurationMs: 600_000}
	playback, err := s.Authorize(t.Context(), ready)
	if err != nil {
		t.Fatalf("vidéo prête refusée: %v", err)
	}
	if playback.HeartbeatMs != 5000 || playback.DurationMs != 600_000 {
		t.Errorf("autorisation incomplète: %+v", playback)
	}
	if !playback.ExpiresAt.After(time.Now().Add(10 * time.Minute)) {
		t.Error("échéance trop courte pour lire un module")
	}
}

func TestNewAssetReservesADistinctSource(t *testing.T) {
	first := video.NewAsset("ORG1", time.Now())
	second := video.NewAsset("ORG1", time.Now())

	if first.SourceKey == second.SourceKey {
		t.Fatal("deux emplacements partagent la même clé source")
	}
	if !strings.HasPrefix(first.SourceKey, "sources/ORG1/") {
		t.Errorf("clé source hors du préfixe de l'organisation: %s", first.SourceKey)
	}
	if first.Status != video.StatusAwaiting {
		t.Errorf("état initial = %q", first.Status)
	}
}
