// Package video porte l'hébergement et la diffusion des modules vidéo.
//
// Le chemin est celui d'AWS de bout en bout : le navigateur dépose le fichier
// directement dans S3 par URL présignée, MediaConvert le transcode en HLS
// multi-débit, et CloudFront le diffuse derrière des URL signées à durée
// courte. L'API ne voit jamais passer un octet de vidéo — la faire transiter
// par une Lambda imposerait de la dimensionner pour des fichiers dont elle
// n'a rien à faire.
//
// La protection ne vise pas l'impossible : un apprenant déterminé peut
// toujours filmer son écran. Elle vise le partage d'URL, qui est le risque
// réel — un lien de cours qui circule dans un groupe de messagerie.
package video

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/identity"
)

// Status est l'état d'un module vidéo.
type Status string

const (
	// StatusAwaiting : l'emplacement est réservé, le fichier n'est pas encore
	// déposé.
	StatusAwaiting Status = "awaiting_upload"
	// StatusEncoding : le transcodage est en cours.
	StatusEncoding Status = "encoding"
	// StatusReady : la vidéo est diffusable.
	StatusReady Status = "ready"
	// StatusFailed : le transcodage a échoué.
	StatusFailed Status = "failed"
)

// Asset est une vidéo de module.
type Asset struct {
	ID    string `dynamodbav:"id" json:"id"`
	OrgID string `dynamodbav:"orgId" json:"orgId"`

	Status Status `dynamodbav:"status" json:"status"`
	// SourceKey est la clé du fichier déposé, MasterKey celle du manifeste
	// HLS produit par le transcodage.
	SourceKey string `dynamodbav:"sourceKey" json:"-"`
	MasterKey string `dynamodbav:"masterKey,omitempty" json:"-"`

	DurationMs int64  `dynamodbav:"durationMs" json:"durationMs"`
	JobID      string `dynamodbav:"jobId,omitempty" json:"-"`
	Error      string `dynamodbav:"error,omitempty" json:"error,omitempty"`

	CreatedAt time.Time  `dynamodbav:"createdAt" json:"createdAt"`
	ReadyAt   *time.Time `dynamodbav:"readyAt,omitempty" json:"readyAt,omitempty"`
}

// NewAsset réserve un emplacement de vidéo.
func NewAsset(orgID string, now time.Time) Asset {
	id := identity.NewID()
	return Asset{
		ID: id, OrgID: orgID, Status: StatusAwaiting,
		SourceKey: fmt.Sprintf("sources/%s/%s.mp4", orgID, id),
		CreatedAt: now,
	}
}

// PlaybackTTL est la durée de validité d'une autorisation de lecture.
//
// Quinze minutes : assez pour lire un module d'une vingtaine de minutes en
// renouvelant une fois, assez court pour qu'un lien partagé soit périmé avant
// d'avoir circulé.
const PlaybackTTL = 15 * time.Minute

// Signer produit les URL signées de CloudFront.
//
// L'implémentation est écrite sur la bibliothèque standard : une URL signée
// CloudFront est une politique JSON, une signature RSA-SHA1 et trois
// paramètres d'URL. La spécification tient en une page et n'a pas bougé depuis
// quinze ans.
type Signer struct {
	domain    string
	keyPairID string
	key       *rsa.PrivateKey
}

// NewSigner construit le signataire depuis une clé privée au format PEM.
func NewSigner(domain, keyPairID string, keyPEM []byte) (*Signer, error) {
	if domain == "" || keyPairID == "" || len(keyPEM) == 0 {
		return nil, fmt.Errorf("video: domaine, identifiant de clé ou clé privée manquant")
	}

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("video: clé privée CloudFront illisible")
	}

	var key *rsa.PrivateKey
	if parsed, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = parsed
	} else {
		generic, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("video: format de clé privée non reconnu: %w", err)
		}
		rsaKey, ok := generic.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("video: CloudFront exige une clé RSA")
		}
		key = rsaKey
	}

	return &Signer{domain: strings.TrimSuffix(domain, "/"), keyPairID: keyPairID, key: key}, nil
}

// cannedPolicy est la politique la plus simple : une ressource, une échéance.
type cannedPolicy struct {
	Statement []struct {
		Resource  string `json:"Resource"`
		Condition struct {
			DateLessThan struct {
				EpochTime int64 `json:"AWS:EpochTime"`
			} `json:"DateLessThan"`
		} `json:"Condition"`
	} `json:"Statement"`
}

// Sign produit une URL de lecture valable pour la durée indiquée.
func (s *Signer) Sign(path string, ttl time.Duration) (string, error) {
	if s == nil {
		return "", fmt.Errorf("video: diffusion non configurée")
	}

	url := s.domain + "/" + strings.TrimPrefix(path, "/")
	expires := time.Now().Add(ttl).Unix()

	var policy cannedPolicy
	policy.Statement = make([]struct {
		Resource  string `json:"Resource"`
		Condition struct {
			DateLessThan struct {
				EpochTime int64 `json:"AWS:EpochTime"`
			} `json:"DateLessThan"`
		} `json:"Condition"`
	}, 1)
	policy.Statement[0].Resource = url
	policy.Statement[0].Condition.DateLessThan.EpochTime = expires

	document, err := json.Marshal(policy)
	if err != nil {
		return "", fmt.Errorf("video: encodage de la politique: %w", err)
	}

	// CloudFront impose RSA-SHA1 : ce n'est pas notre choix, et la signature
	// ne protège pas un secret — elle horodate une autorisation de lecture.
	digest := sha1.Sum(document)
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA1, digest[:])
	if err != nil {
		return "", fmt.Errorf("video: signature: %w", err)
	}

	return fmt.Sprintf("%s?Expires=%d&Signature=%s&Key-Pair-Id=%s",
		url, expires, cloudFrontEncode(signature), s.keyPairID), nil
}

// cloudFrontEncode applique l'alphabet base64 particulier de CloudFront.
//
// Les caractères +, = et / y sont remplacés par -, _ et ~ : c'est ce que la
// documentation appelle « base64 sûr pour les URL », et l'oublier produit des
// signatures rejetées sans message utile.
func cloudFrontEncode(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	return strings.NewReplacer("+", "-", "=", "_", "/", "~").Replace(encoded)
}

// Playback est l'autorisation remise au lecteur.
type Playback struct {
	// ManifestURL est l'URL signée du manifeste HLS.
	ManifestURL string `json:"manifestUrl"`
	// ExpiresAt permet au lecteur de renouveler avant expiration plutôt que
	// d'interrompre la lecture au milieu d'un module.
	ExpiresAt  time.Time `json:"expiresAt"`
	DurationMs int64     `json:"durationMs"`
	// HeartbeatMs indique au lecteur la cadence de ses signaux. La fixer côté
	// serveur permet de l'ajuster sans redéployer le client.
	HeartbeatMs int `json:"heartbeatMs"`
}

// Authorize produit l'autorisation de lecture d'un module.
func (s *Signer) Authorize(_ context.Context, asset Asset) (Playback, error) {
	if asset.Status != StatusReady || asset.MasterKey == "" {
		return Playback{}, fmt.Errorf("cette vidéo n'est pas encore diffusable (%s)", asset.Status)
	}

	url, err := s.Sign(asset.MasterKey, PlaybackTTL)
	if err != nil {
		return Playback{}, err
	}
	return Playback{
		ManifestURL: url,
		ExpiresAt:   time.Now().Add(PlaybackTTL),
		DurationMs:  asset.DurationMs,
		HeartbeatMs: 5000,
	}, nil
}
