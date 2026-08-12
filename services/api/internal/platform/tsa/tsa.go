// Package tsa demande un jeton d'horodatage à une autorité RFC 3161.
//
// C'est ce jeton, et lui seul, qui donne une date opposable à une signature.
// L'heure de notre serveur ne prouve rien : nous sommes partie prenante. Un
// tiers indépendant, lui, atteste que l'empreinte existait à un instant donné.
package tsa

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"
)

var (
	oidSHA256 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
)

// Client interroge une autorité d'horodatage.
type Client struct {
	url    string
	http   *http.Client
	policy asn1.ObjectIdentifier
}

// New construit le client. Une URL vide désactive l'horodatage : le champ
// correspondant du dossier de preuve restera vide, ce qui est préférable à
// une date que l'on présenterait à tort comme opposable.
func New(url string) *Client {
	if url == "" {
		return nil
	}
	return &Client{
		url: url,
		// Délai court : l'horodatage est un confort, pas un préalable. Son
		// échec ne doit pas bloquer une signature déjà consentie.
		http: &http.Client{Timeout: 8 * time.Second},
	}
}

// URL renvoie l'autorité interrogée.
func (c *Client) URL() string {
	if c == nil {
		return ""
	}
	return c.url
}

// --- Structures ASN.1 (RFC 3161) -----------------------------------------

type messageImprint struct {
	HashAlgorithm algorithmIdentifier
	HashedMessage []byte
}

type algorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

type timeStampReq struct {
	Version        int
	MessageImprint messageImprint
	ReqPolicy      asn1.ObjectIdentifier `asn1:"optional"`
	Nonce          *big.Int              `asn1:"optional"`
	CertReq        bool                  `asn1:"optional,default:false"`
}

type pkiStatusInfo struct {
	Status       int
	StatusString asn1.RawValue  `asn1:"optional"`
	FailInfo     asn1.BitString `asn1:"optional"`
}

type timeStampResp struct {
	Status         pkiStatusInfo
	TimeStampToken asn1.RawValue `asn1:"optional"`
}

// Stamp demande l'horodatage d'une empreinte SHA-256 et renvoie le jeton.
func (c *Client) Stamp(ctx context.Context, digest []byte) ([]byte, string, error) {
	if c == nil {
		return nil, "", fmt.Errorf("tsa: aucune autorité configurée")
	}
	if len(digest) != sha256.Size {
		return nil, "", fmt.Errorf("tsa: empreinte de %d octets, %d attendus", len(digest), sha256.Size)
	}

	// Le nonce lie la réponse à notre requête : sans lui, une réponse
	// interceptée et rejouée passerait pour un horodatage frais.
	nonce, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
	if err != nil {
		return nil, "", fmt.Errorf("tsa: nonce: %w", err)
	}

	request, err := asn1.Marshal(timeStampReq{
		Version: 1,
		MessageImprint: messageImprint{
			HashAlgorithm: algorithmIdentifier{Algorithm: oidSHA256, Parameters: asn1.NullRawValue},
			HashedMessage: digest,
		},
		Nonce: nonce,
		// CertReq demande que le certificat de l'autorité soit inclus dans le
		// jeton : sans lui, personne ne peut vérifier l'horodatage hors ligne.
		CertReq: true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("tsa: encodage de la requête: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(request))
	if err != nil {
		return nil, "", fmt.Errorf("tsa: requête: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/timestamp-query")

	res, err := c.http.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("tsa: appel de %s: %w", c.url, err)
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, "", fmt.Errorf("tsa: lecture de la réponse: %w", err)
	}
	if res.StatusCode >= 300 {
		return nil, "", fmt.Errorf("tsa: %s a répondu %d", c.url, res.StatusCode)
	}

	var response timeStampResp
	if _, err := asn1.Unmarshal(body, &response); err != nil {
		return nil, "", fmt.Errorf("tsa: réponse illisible: %w", err)
	}
	// 0 = granted, 1 = grantedWithMods. Tout le reste est un refus.
	if response.Status.Status > 1 {
		return nil, "", fmt.Errorf("tsa: horodatage refusé (statut %d)", response.Status.Status)
	}
	if len(response.TimeStampToken.FullBytes) == 0 {
		return nil, "", fmt.Errorf("tsa: réponse sans jeton")
	}

	return response.TimeStampToken.FullBytes, c.url, nil
}
