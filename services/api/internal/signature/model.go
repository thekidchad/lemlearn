// Package signature porte la signature électronique interne de lemlearn.
//
// L'objectif n'est pas d'imiter un prestataire tiers mais de réunir les quatre
// éléments qui donnent sa valeur probante à une signature devant un financeur :
//
//  1. authentification forte du signataire (lien à usage unique + code OTP) ;
//  2. horodatage de chaque étape ;
//  3. dossier de preuve complet (IP, appareil, adresse certifiée, tracé) ;
//  4. intégrité du document scellé, vérifiable après coup.
//
// Le point 4 est ici assuré par une empreinte SHA-256 inscrite au journal
// d'audit chaîné. Le passage au scellement PAdES — signature cryptographique
// incorporée au PDF, vérifiable dans un lecteur — demande un certificat de
// cachet d'organisation et une bibliothèque de signature ; l'interface Sealer
// est prévue pour, sans que rien d'autre n'ait à changer.
package signature

import (
	"fmt"
	"time"

	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/ddb"
	"github.com/lemlearn/api/internal/platform/doc"
)

// Status est l'état d'une demande de signature.
type Status string

const (
	// StatusPending : lien émis, jamais ouvert.
	StatusPending Status = "pending"
	// StatusOpened : le signataire a ouvert le lien et vu le document.
	StatusOpened Status = "opened"
	// StatusOTPSent : un code a été envoyé et attend d'être saisi.
	StatusOTPSent Status = "otp_sent"
	// StatusSigned : document signé et scellé.
	StatusSigned Status = "signed"
	// StatusCancelled : demande annulée par l'organisme.
	StatusCancelled Status = "cancelled"
)

// Durées de vie. Elles sont volontairement courtes : un lien de signature qui
// traîne un mois dans une boîte mail est un risque, pas un service.
const (
	// LinkTTL est la validité du lien de signature.
	LinkTTL = 7 * 24 * time.Hour
	// OTPTTL est la validité d'un code à usage unique.
	OTPTTL = 10 * time.Minute
	// MaxOTPAttempts borne les essais avant invalidation du code.
	MaxOTPAttempts = 3
)

// Request est une demande de signature adressée à une personne.
type Request struct {
	ddb.Record

	ID     string `dynamodbav:"id" json:"id"`
	OrgID  string `dynamodbav:"orgId" json:"orgId"`
	FileID string `dynamodbav:"fileId" json:"fileId"`

	// Kind identifie le type de document signé : convention, devis,
	// émargement. Il détermine le gabarit re-rendu à la signature.
	Kind      string `dynamodbav:"kind" json:"kind"`
	Reference string `dynamodbav:"reference" json:"reference"`

	// Role est la zone du gabarit que remplira ce signataire.
	Role doc.SignatureZoneRole `dynamodbav:"role" json:"role"`

	SignerName  string `dynamodbav:"signerName" json:"signerName"`
	SignerEmail string `dynamodbav:"signerEmail" json:"signerEmail"`
	SignerPhone string `dynamodbav:"signerPhone,omitempty" json:"signerPhone,omitempty"`

	Status    Status    `dynamodbav:"status" json:"status"`
	IssuedAt  time.Time `dynamodbav:"issuedAt" json:"issuedAt"`
	ExpiresAt time.Time `dynamodbav:"expiresAtTime" json:"expiresAt"`

	// TokenHash est l'empreinte du jeton contenu dans le lien. Le jeton en
	// clair n'existe que dans l'e-mail envoyé au signataire.
	TokenHash string `dynamodbav:"tokenHash" json:"-"`

	// UnsignedSHA256 est l'empreinte du document présenté au signataire.
	// Elle est figée à l'émission : ce qui est signé est exactement ce qui a
	// été montré, et le prouver ne dépend pas de la conservation du fichier.
	UnsignedSHA256 string `dynamodbav:"unsignedSha256" json:"unsignedSha256"`

	// OTPHash, OTPExpiresAt et OTPAttempts pilotent l'authentification forte.
	// Le code n'est jamais stocké en clair : une copie de la base ne permet
	// pas de signer à la place de quelqu'un.
	OTPHash      string    `dynamodbav:"otpHash,omitempty" json:"-"`
	OTPExpiresAt time.Time `dynamodbav:"otpExpiresAt,omitempty" json:"-"`
	OTPAttempts  int       `dynamodbav:"otpAttempts" json:"-"`

	// Proof est renseigné une fois la signature apposée.
	Proof *Proof `dynamodbav:"proof,omitempty" json:"proof,omitempty"`
}

// Proof est le dossier de preuve d'une signature accomplie.
//
// Tout y est consigné au moment de l'acte : reconstituer ces éléments plus
// tard, à partir de journaux applicatifs, n'a aucune valeur devant un
// financeur.
type Proof struct {
	SignedAt time.Time `dynamodbav:"signedAt" json:"signedAt"`

	// Mention est la mention manuscrite saisie par le signataire.
	Mention string `dynamodbav:"mention" json:"mention"`

	// StrokeCount et DurationMs caractérisent le tracé : un tracé de deux
	// points en dix millisecondes n'est pas une signature manuscrite.
	StrokeCount int   `dynamodbav:"strokeCount" json:"strokeCount"`
	DurationMs  int64 `dynamodbav:"durationMs" json:"durationMs"`

	// DrawingKey est la clé S3 de l'image du tracé.
	DrawingKey    string `dynamodbav:"drawingKey" json:"-"`
	DrawingSHA256 string `dynamodbav:"drawingSha256" json:"drawingSha256"`

	IP        string `dynamodbav:"ip" json:"ip"`
	UserAgent string `dynamodbav:"userAgent" json:"userAgent"`

	// OTPChannel indique par quel canal l'authentification a eu lieu.
	OTPChannel string `dynamodbav:"otpChannel" json:"otpChannel"`

	// SealedSHA256 est l'empreinte du document signé, celle qui sera
	// recalculée lors d'un contrôle.
	SealedSHA256 string `dynamodbav:"sealedSha256" json:"sealedSha256"`
	SealedKey    string `dynamodbav:"sealedKey" json:"-"`

	// TimestampToken est le jeton d'horodatage RFC 3161, encodé en base64,
	// lorsqu'une autorité est configurée. Vide sinon : mieux vaut un champ
	// vide qu'un horodatage serveur présenté comme opposable.
	TimestampToken string `dynamodbav:"timestampToken,omitempty" json:"-"`
	TimestampTSA   string `dynamodbav:"timestampTsa,omitempty" json:"timestampTsa,omitempty"`
}

// NewRequest construit une demande prête à écrire.
func NewRequest(orgID, fileID, kind, reference string, role doc.SignatureZoneRole, now time.Time) Request {
	id := identity.NewID()
	return Request{
		Record: ddb.Record{
			PK:        ddb.OrgPK(orgID),
			SK:        ddb.SignatureSK(id),
			GSI1PK:    ddb.OrgPK(orgID) + "#SIGFILE#" + fileID,
			GSI1SK:    now.UTC().Format(time.RFC3339Nano),
			Type:      "signature_request",
			CreatedAt: now,
			UpdatedAt: now,
		},
		ID: id, OrgID: orgID, FileID: fileID,
		Kind: kind, Reference: reference, Role: role,
		Status:   StatusPending,
		IssuedAt: now, ExpiresAt: now.Add(LinkTTL),
	}
}

// TokenPointer indexe une demande par l'empreinte de son jeton, hors partition
// d'organisation : le signataire n'est pas authentifié et ne connaît pas
// l'organisation dont relève le document.
type TokenPointer struct {
	ddb.Record

	OrgID     string `dynamodbav:"orgId"`
	RequestID string `dynamodbav:"requestId"`
}

// NewTokenPointer construit la réservation d'un jeton.
func NewTokenPointer(tokenHash, orgID, requestID string, now, expiresAt time.Time) TokenPointer {
	return TokenPointer{
		Record: ddb.Record{
			PK:        ddb.SignatureTokenPK(tokenHash),
			SK:        ddb.SignatureTokenSK,
			Type:      "signature_token",
			CreatedAt: now,
			UpdatedAt: now,
			// Le pointeur disparaît de lui-même après l'échéance du lien :
			// un lien périmé ne doit pas rester indéfiniment résolvable.
			ExpiresAt: ddb.TTL(expiresAt.Add(30 * 24 * time.Hour)),
		},
		OrgID: orgID, RequestID: requestID,
	}
}

// Expired indique si le lien n'est plus valable.
func (r Request) Expired(now time.Time) bool { return !now.Before(r.ExpiresAt) }

// Signable indique si la demande peut encore recevoir une signature.
func (r Request) Signable(now time.Time) error {
	switch {
	case r.Status == StatusSigned:
		return ErrAlreadySigned
	case r.Status == StatusCancelled:
		return fmt.Errorf("cette demande de signature a été annulée")
	case r.Expired(now):
		return ErrLinkExpired
	}
	return nil
}

// Erreurs du parcours de signature.
var (
	ErrLinkExpired   = fmt.Errorf("ce lien de signature a expiré")
	ErrAlreadySigned = fmt.Errorf("ce document a déjà été signé")
	ErrOTPInvalid    = fmt.Errorf("code incorrect ou expiré")
	ErrOTPExhausted  = fmt.Errorf("trop d'essais : demandez un nouveau code")
	ErrNotFound      = fmt.Errorf("demande de signature introuvable")
	// ErrDrawingTooPoor refuse un tracé qui ne peut pas être présenté comme
	// une signature manuscrite : un simple clic n'en est pas une.
	ErrDrawingTooPoor = fmt.Errorf("le tracé de signature est insuffisant")
)
