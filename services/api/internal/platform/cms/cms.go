// Package cms construit des signatures CMS/PKCS#7 détachées (RFC 5652),
// au profil exigé par CAdES-BES — donc utilisables dans un PDF signé PAdES.
//
// Écrit sur la bibliothèque standard seule (`encoding/asn1`, `crypto/x509`).
// Le format est entièrement spécifié et stable depuis vingt ans ; l'écrire
// ici évite une dépendance de plus dans une Lambda, et surtout rend visible
// et vérifiable ce qui compose exactement la signature apposée à un document
// contractuel.
//
// Le résultat est contrôlé indépendamment par `openssl cms -verify` dans les
// tests : ce paquet ne se valide pas lui-même.
package cms

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"math/big"
	"time"
)

// Identifiants d'objet du format. Ils sont écrits en clair plutôt
// qu'empruntés à une constante d'une bibliothèque : ce sont eux qui décident
// de ce qu'un vérificateur comprendra.
var (
	oidSignedData    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidData          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	oidSHA256        = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidRSAEncryption = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}

	oidAttrContentType   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}
	oidAttrMessageDigest = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
	oidAttrSigningTime   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 5}
	// signing-certificate-v2 : lie la signature au certificat exact qui l'a
	// produite. Sans cet attribut, un vérificateur ne peut pas écarter une
	// substitution de certificat, et le profil CAdES-BES n'est pas rempli.
	oidAttrSigningCertV2 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 47}
	// timeStampToken : jeton RFC 3161, attribut *non signé* — il est produit
	// après coup, sur la signature elle-même.
	oidAttrTimeStampToken = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 14}
)

// Options décrit la signature à produire.
type Options struct {
	// Certificate est le certificat du signataire — en production, le cachet
	// d'organisation de l'organisme de formation.
	Certificate *x509.Certificate
	// PrivateKey signe. crypto.Signer permet qu'elle reste dans KMS ou dans
	// un HSM : rien ici n'exige de détenir la clé en mémoire.
	PrivateKey crypto.Signer
	// Chain complète la chaîne de confiance, hors certificat du signataire.
	Chain []*x509.Certificate
	// SigningTime est l'heure déclarée par le signataire. Elle n'a pas de
	// valeur opposable : c'est le jeton d'horodatage qui l'apporte.
	SigningTime time.Time
	// Timestamper, s'il est fourni, reçoit l'empreinte de la signature et
	// renvoie un jeton RFC 3161 incorporé en attribut non signé — ce qui fait
	// passer le document de PAdES-B-B à PAdES-B-T.
	Timestamper func(signatureDigest []byte) ([]byte, error)
}

// --- Structures ASN.1 (RFC 5652) -----------------------------------------

type contentInfo struct {
	ContentType asn1.ObjectIdentifier
	// Content porte lui-même son enveloppe [0] EXPLICIT. Le marqueur
	// `asn1:"explicit,tag:0"` serait ignoré : quand un RawValue est marshalé
	// avec FullBytes renseigné, encoding/asn1 recopie ces octets tels quels
	// et n'ajoute aucune enveloppe. Le résultat était une structure qu'aucun
	// vérificateur n'acceptait.
	Content asn1.RawValue
}

type encapContentInfo struct {
	EContentType asn1.ObjectIdentifier
	// EContent est absent : la signature est détachée, le contenu signé n'est
	// pas recopié dans la structure.
	EContent asn1.RawValue `asn1:"optional"`
}

type signedData struct {
	Version          int
	DigestAlgorithms []pkix.AlgorithmIdentifier `asn1:"set"`
	EncapContentInfo encapContentInfo
	Certificates     asn1.RawValue `asn1:"optional,tag:0"`
	SignerInfos      []signerInfo  `asn1:"set"`
}

type issuerAndSerial struct {
	Issuer       asn1.RawValue
	SerialNumber *big.Int
}

type signerInfo struct {
	Version            int
	SID                issuerAndSerial
	DigestAlgorithm    pkix.AlgorithmIdentifier
	SignedAttrs        asn1.RawValue `asn1:"optional,tag:0"`
	SignatureAlgorithm pkix.AlgorithmIdentifier
	Signature          []byte
	UnsignedAttrs      asn1.RawValue `asn1:"optional,tag:1"`
}

type attribute struct {
	Type   asn1.ObjectIdentifier
	Values []asn1.RawValue `asn1:"set"`
}

type essCertIDv2 struct {
	// HashAlgorithm est omis : sa valeur par défaut est SHA-256, et
	// l'encodage DER interdit d'écrire une valeur par défaut.
	CertHash []byte
}

type signingCertificateV2 struct {
	Certs []essCertIDv2
}

// SignDetached produit la signature CMS détachée de `content`.
func SignDetached(content []byte, opts Options) ([]byte, error) {
	if opts.Certificate == nil || opts.PrivateKey == nil {
		return nil, fmt.Errorf("cms: certificat ou clé manquant")
	}

	digest := sha256.Sum256(content)
	signedAttrs, err := buildSignedAttrs(digest[:], opts)
	if err != nil {
		return nil, err
	}

	// Les attributs sont signés sous leur forme SET (0x31) mais incorporés
	// sous la forme [0] IMPLICIT (0xA0). Confondre les deux produit une
	// signature que rien ne vérifie — c'est le piège classique du format.
	signable, err := asn1.Marshal(struct {
		Attrs []attribute `asn1:"set"`
	}{signedAttrs})
	if err != nil {
		return nil, fmt.Errorf("cms: encodage des attributs signés: %w", err)
	}
	signable = unwrapSequence(signable)

	attrDigest := sha256.Sum256(signable)
	signature, err := opts.PrivateKey.Sign(rand.Reader, attrDigest[:], crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("cms: signature: %w", err)
	}

	implicitAttrs := make([]byte, len(signable))
	copy(implicitAttrs, signable)
	implicitAttrs[0] = 0xA0

	info := signerInfo{
		Version: 1,
		SID: issuerAndSerial{
			Issuer:       asn1.RawValue{FullBytes: opts.Certificate.RawIssuer},
			SerialNumber: opts.Certificate.SerialNumber,
		},
		DigestAlgorithm:    pkix.AlgorithmIdentifier{Algorithm: oidSHA256, Parameters: asn1.NullRawValue},
		SignedAttrs:        asn1.RawValue{FullBytes: implicitAttrs},
		SignatureAlgorithm: pkix.AlgorithmIdentifier{Algorithm: oidRSAEncryption, Parameters: asn1.NullRawValue},
		Signature:          signature,
	}

	if opts.Timestamper != nil {
		// Le jeton horodate la *signature*, pas le document : c'est ce qui
		// prouve que la signature existait à une date donnée, indépendamment
		// de l'horloge du signataire.
		signatureDigest := sha256.Sum256(signature)
		token, err := opts.Timestamper(signatureDigest[:])
		if err != nil {
			return nil, fmt.Errorf("cms: horodatage: %w", err)
		}
		unsigned, err := marshalUnsignedAttrs(token)
		if err != nil {
			return nil, err
		}
		info.UnsignedAttrs = asn1.RawValue{FullBytes: unsigned}
	}

	certs := [][]byte{opts.Certificate.Raw}
	for _, extra := range opts.Chain {
		certs = append(certs, extra.Raw)
	}

	body, err := asn1.Marshal(signedData{
		Version:          1,
		DigestAlgorithms: []pkix.AlgorithmIdentifier{{Algorithm: oidSHA256, Parameters: asn1.NullRawValue}},
		EncapContentInfo: encapContentInfo{EContentType: oidData},
		Certificates:     asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: concat(certs)},
		SignerInfos:      []signerInfo{info},
	})
	if err != nil {
		return nil, fmt.Errorf("cms: encodage signedData: %w", err)
	}

	envelope, err := asn1.Marshal(contentInfo{
		ContentType: oidSignedData,
		Content: asn1.RawValue{
			Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: body,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cms: encodage contentInfo: %w", err)
	}
	return envelope, nil
}

func buildSignedAttrs(digest []byte, opts Options) ([]attribute, error) {
	contentType, err := asn1.Marshal(oidData)
	if err != nil {
		return nil, err
	}
	messageDigest, err := asn1.Marshal(digest)
	if err != nil {
		return nil, err
	}

	signingTime := opts.SigningTime
	if signingTime.IsZero() {
		signingTime = time.Now()
	}
	// UTC et à la seconde : DER impose une représentation canonique, et une
	// fraction de seconde ferait échouer certains vérificateurs stricts.
	timeBytes, err := asn1.Marshal(signingTime.UTC().Truncate(time.Second))
	if err != nil {
		return nil, err
	}

	certHash := sha256.Sum256(opts.Certificate.Raw)
	certRef, err := asn1.Marshal(signingCertificateV2{Certs: []essCertIDv2{{CertHash: certHash[:]}}})
	if err != nil {
		return nil, err
	}

	return []attribute{
		{Type: oidAttrContentType, Values: []asn1.RawValue{{FullBytes: contentType}}},
		{Type: oidAttrSigningTime, Values: []asn1.RawValue{{FullBytes: timeBytes}}},
		{Type: oidAttrMessageDigest, Values: []asn1.RawValue{{FullBytes: messageDigest}}},
		{Type: oidAttrSigningCertV2, Values: []asn1.RawValue{{FullBytes: certRef}}},
	}, nil
}

func marshalUnsignedAttrs(token []byte) ([]byte, error) {
	attrs, err := asn1.Marshal(struct {
		Attrs []attribute `asn1:"set"`
	}{[]attribute{{
		Type:   oidAttrTimeStampToken,
		Values: []asn1.RawValue{{FullBytes: token}},
	}}})
	if err != nil {
		return nil, fmt.Errorf("cms: encodage des attributs non signés: %w", err)
	}
	attrs = unwrapSequence(attrs)
	attrs[0] = 0xA1
	return attrs, nil
}

// unwrapSequence retire l'enveloppe SEQUENCE ajoutée par asn1.Marshal autour
// d'une structure à champ unique, pour ne garder que le SET lui-même.
func unwrapSequence(der []byte) []byte {
	var raw asn1.RawValue
	if _, err := asn1.Unmarshal(der, &raw); err != nil {
		return der
	}
	return raw.Bytes
}

func concat(chunks [][]byte) []byte {
	total := 0
	for _, chunk := range chunks {
		total += len(chunk)
	}
	out := make([]byte, 0, total)
	for _, chunk := range chunks {
		out = append(out, chunk...)
	}
	return out
}
