// Package seal assemble le scellement PAdES d'un document.
//
// Il réunit trois briques qui n'ont pas à se connaître : le certificat de
// l'organisme, l'autorité d'horodatage, et l'apposition de la signature dans
// le PDF. Le service de signature, lui, ne voit qu'une interface.
package seal

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/lemlearn/api/internal/platform/pdfsig"
	"github.com/lemlearn/api/internal/platform/tsa"
)

// Sealer scelle un PDF.
type Sealer interface {
	// Seal renvoie le document scellé et le nom de l'autorité d'horodatage
	// employée, vide si le document n'est pas horodaté.
	Seal(ctx context.Context, pdf []byte, meta Meta) (sealed []byte, authority string, err error)
}

// Meta décrit ce qui apparaîtra dans le panneau de signature du lecteur PDF.
type Meta struct {
	SignerName string
	Reason     string
	Location   string
	SignedAt   time.Time
}

// PAdES scelle par signature CAdES détachée incorporée au PDF.
type PAdES struct {
	certificate *x509.Certificate
	key         crypto.Signer
	chain       []*x509.Certificate
	timestamper *tsa.Client
}

// New construit le scelleur.
func New(cert *x509.Certificate, key crypto.Signer, chain []*x509.Certificate, timestamper *tsa.Client) *PAdES {
	return &PAdES{certificate: cert, key: key, chain: chain, timestamper: timestamper}
}

// Seal appose la signature.
func (p *PAdES) Seal(ctx context.Context, pdf []byte, meta Meta) ([]byte, string, error) {
	if p == nil || p.certificate == nil {
		return nil, "", fmt.Errorf("seal: aucun certificat de scellement configuré")
	}

	authority := ""
	opts := pdfsig.Options{
		Certificate: p.certificate,
		PrivateKey:  p.key,
		Chain:       p.chain,
		Name:        meta.SignerName,
		Reason:      meta.Reason,
		Location:    meta.Location,
		SignedAt:    meta.SignedAt,
	}

	if p.timestamper != nil {
		opts.Timestamper = func(digest []byte) ([]byte, error) {
			token, url, err := p.timestamper.Stamp(ctx, digest)
			if err != nil {
				return nil, err
			}
			authority = url
			return token, nil
		}
	}

	sealed, err := pdfsig.Sign(pdf, opts)
	if err != nil {
		if p.timestamper == nil {
			return nil, "", err
		}
		// L'autorité d'horodatage est un tiers : elle peut être injoignable.
		// Le document est alors scellé sans jeton — signature valide, date non
		// opposable — plutôt que de refuser une signature déjà consentie par
		// l'apprenant. Le dossier de preuve montrera l'absence d'horodatage.
		opts.Timestamper = nil
		authority = ""
		sealed, err = pdfsig.Sign(pdf, opts)
		if err != nil {
			return nil, "", err
		}
	}

	return sealed, authority, nil
}

// Certificate expose le certificat employé, pour l'afficher dans le dossier de
// preuve.
func (p *PAdES) Certificate() *x509.Certificate {
	if p == nil {
		return nil
	}
	return p.certificate
}

// LoadKeyPair lit un certificat et sa clé privée au format PEM.
//
// En production, ce sont les octets d'un cachet d'organisation, tenus hors du
// dépôt et injectés par Secrets Manager.
func LoadKeyPair(certPEM, keyPEM []byte) (*x509.Certificate, crypto.Signer, []*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := certPEM
	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = remaining
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("seal: certificat illisible: %w", err)
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, nil, nil, fmt.Errorf("seal: aucun certificat dans le PEM fourni")
	}

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, nil, nil, fmt.Errorf("seal: clé privée illisible")
	}
	key, err := parsePrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, nil, err
	}

	return certs[0], key, certs[1:], nil
}

func parsePrivateKey(der []byte) (crypto.Signer, error) {
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		signer, ok := key.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("seal: type de clé non signataire")
		}
		return signer, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("seal: format de clé privée non reconnu")
}

// Development produit un scelleur auto-signé.
//
// Il permet d'exercer et de vérifier tout le mécanisme sans attendre la
// commande d'un cachet d'organisation. Le certificat produit ne vaut rien
// devant un financeur : son nom le dit explicitement, pour qu'un document de
// développement ne puisse pas être pris pour un document contractuel.
func Development(orgName string, timestamper *tsa.Client) (*PAdES, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("seal: génération de clé: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName:   orgName + " (certificat de développement — sans valeur)",
			Organization: []string{orgName},
			Country:      []string{"FR"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("seal: certificat de développement: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("seal: relecture du certificat: %w", err)
	}

	return New(cert, key, nil, timestamper), nil
}
