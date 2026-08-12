package signature

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
	"github.com/lemlearn/api/internal/platform/doc"
)

// Renderer produit le PDF d'une demande. `applied` vide rend le document
// vierge présenté au signataire ; renseigné, il rend le document signé.
//
// C'est le même gabarit dans les deux cas : la signature est rendue *dans* sa
// zone plutôt qu'apposée après coup sur un PDF existant. Le document signé est
// donc un rendu de plein droit, reproductible à l'octet près, et non le
// résultat d'un post-traitement dont il faudrait prouver la fidélité.
type Renderer interface {
	Render(ctx context.Context, req Request, applied []doc.AppliedSignature) ([]byte, error)
}

// BlobStore range les fichiers volumineux hors de la base : tracé de
// signature, PDF scellé.
type BlobStore interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// Mailer envoie les courriels transactionnels.
type Mailer interface {
	Send(ctx context.Context, to, subject, html string) error
}

// Timestamper obtient un jeton d'horodatage RFC 3161 sur une empreinte.
//
// Facultatif : sans autorité configurée, le champ d'horodatage du dossier de
// preuve reste vide. Un horodatage serveur ne serait pas opposable, et le
// présenter comme tel serait pire que de ne rien afficher.
type Timestamper interface {
	Stamp(ctx context.Context, sha256sum []byte) (token []byte, authority string, err error)
}

// Service porte le parcours de signature.
type Service struct {
	db       *ddb.Client
	renderer Renderer
	blobs    BlobStore
	mailer   Mailer
	stamper  Timestamper
	appURL   string
	now      func() time.Time
}

// Deps regroupe les dépendances du service.
type Deps struct {
	DB       *ddb.Client
	Renderer Renderer
	Blobs    BlobStore
	Mailer   Mailer
	Stamper  Timestamper
	AppURL   string
	Now      func() time.Time
}

// NewService construit le service.
func NewService(deps Deps) *Service {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		db: deps.DB, renderer: deps.Renderer, blobs: deps.Blobs,
		mailer: deps.Mailer, stamper: deps.Stamper,
		appURL: strings.TrimRight(deps.AppURL, "/"), now: now,
	}
}

// subject renvoie le sujet d'audit : le dossier, pas la demande.
//
// Toute la chaîne de preuve d'un dossier doit tenir dans un seul journal ; une
// signature qui aurait sa propre chaîne isolée obligerait un auditeur à
// recoller deux histoires.
func subject(fileID string) string { return "file/" + fileID }

// IssueInput décrit l'émission d'une demande de signature.
type IssueInput struct {
	OrgID       string
	FileID      string
	Kind        string
	Reference   string
	Role        doc.SignatureZoneRole
	SignerName  string
	SignerEmail string
	SignerPhone string
	Actor       audit.Actor
}

// Issue crée la demande, fige l'empreinte du document à signer et envoie le
// lien.
func (s *Service) Issue(ctx context.Context, in IssueInput) (Request, string, error) {
	if strings.TrimSpace(in.SignerEmail) == "" || !strings.Contains(in.SignerEmail, "@") {
		return Request{}, "", fmt.Errorf("adresse e-mail du signataire invalide")
	}
	if strings.TrimSpace(in.SignerName) == "" {
		return Request{}, "", fmt.Errorf("le nom du signataire est obligatoire")
	}

	now := s.now()
	req := NewRequest(in.OrgID, in.FileID, in.Kind, in.Reference, in.Role, now)
	req.SignerName = in.SignerName
	req.SignerEmail = ddb.NormalizeEmail(in.SignerEmail)
	req.SignerPhone = in.SignerPhone

	// Le document vierge est rendu tout de suite pour figer son empreinte :
	// ce qui sera signé est exactement ce qui aura été montré.
	unsigned, err := s.renderer.Render(ctx, req, nil)
	if err != nil {
		return Request{}, "", fmt.Errorf("rendu du document: %w", err)
	}
	req.UnsignedSHA256 = sha256hex(unsigned)

	token, tokenHash, err := identity.NewToken()
	if err != nil {
		return Request{}, "", err
	}
	req.TokenHash = tokenHash
	pointer := NewTokenPointer(tokenHash, in.OrgID, req.ID, now, req.ExpiresAt)

	_, err = s.db.WriteWithAudit(ctx, subject(in.FileID),
		[]ddb.Write{
			{Item: req, Condition: "attribute_not_exists(SK)"},
			{Item: pointer, Condition: "attribute_not_exists(PK)"},
		},
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, subject(in.FileID), now, audit.ActionDocumentSent, in.Actor,
				map[string]any{
					"requestId":      req.ID,
					"kind":           req.Kind,
					"reference":      req.Reference,
					"role":           string(req.Role),
					"signer":         req.SignerEmail,
					"unsignedSha256": req.UnsignedSHA256,
					"expiresAt":      req.ExpiresAt.Format(time.RFC3339),
				})
		})
	if err != nil {
		return Request{}, "", err
	}

	if s.mailer != nil {
		link := fmt.Sprintf("%s/signer/%s", s.appURL, token)
		if err := s.mailer.Send(ctx, req.SignerEmail,
			fmt.Sprintf("Document à signer — %s", req.Reference),
			invitationEmail(req, link)); err != nil {
			// L'envoi échoue mais la demande existe : la relance est
			// possible, alors qu'une demande perdue ne l'est pas. L'erreur
			// remonte quand même pour être affichée à l'organisme.
			return req, token, fmt.Errorf("envoi du lien: %w", err)
		}
	}

	return req, token, nil
}

// Resolve retrouve une demande à partir du jeton du lien.
func (s *Service) Resolve(ctx context.Context, token string) (Request, error) {
	if strings.TrimSpace(token) == "" {
		return Request{}, ErrNotFound
	}

	pointer, err := ddb.Get[TokenPointer](ctx, s.db,
		ddb.SignatureTokenPK(identity.HashToken(token)), ddb.SignatureTokenSK)
	if err != nil {
		if errors.Is(err, ddb.ErrNotFound) {
			return Request{}, ErrNotFound
		}
		return Request{}, err
	}

	req, err := ddb.Get[Request](ctx, s.db, ddb.OrgPK(pointer.OrgID), ddb.SignatureSK(pointer.RequestID))
	if err != nil {
		if errors.Is(err, ddb.ErrNotFound) {
			return Request{}, ErrNotFound
		}
		return Request{}, err
	}
	return req, nil
}

// Open enregistre l'ouverture du lien et renvoie le document à signer.
func (s *Service) Open(ctx context.Context, token, ip, userAgent string) (Request, []byte, error) {
	req, err := s.Resolve(ctx, token)
	if err != nil {
		return Request{}, nil, err
	}
	if err := req.Signable(s.now()); err != nil {
		return req, nil, err
	}

	unsigned, err := s.renderer.Render(ctx, req, nil)
	if err != nil {
		return req, nil, fmt.Errorf("rendu du document: %w", err)
	}
	// Le document remis doit être celui dont l'empreinte a été figée à
	// l'émission. S'il a dérivé — gabarit modifié, données du dossier
	// changées — la signature ne porterait pas sur ce qui a été promis.
	if got := sha256hex(unsigned); got != req.UnsignedSHA256 {
		return req, nil, fmt.Errorf(
			"le document a changé depuis l'émission du lien (%s puis %s) : annulez la demande et réémettez-la",
			short(req.UnsignedSHA256), short(got))
	}

	if req.Status == StatusPending {
		now := s.now()
		req.Status = StatusOpened
		req.UpdatedAt = now
		if _, err := s.db.WriteWithAudit(ctx, subject(req.FileID),
			[]ddb.Write{{Item: req}},
			func(prev audit.Event) (audit.Event, error) {
				return audit.Append(prev, subject(req.FileID), now, audit.ActionSignatureOpened,
					signerActor(req, ip, userAgent),
					map[string]any{"requestId": req.ID})
			}); err != nil {
			return req, nil, err
		}
	}

	return req, unsigned, nil
}

// SendOTP émet un code à usage unique.
func (s *Service) SendOTP(ctx context.Context, token, ip, userAgent string) (Request, error) {
	req, err := s.Resolve(ctx, token)
	if err != nil {
		return Request{}, err
	}
	now := s.now()
	if err := req.Signable(now); err != nil {
		return req, err
	}

	code, err := identity.NewOTP()
	if err != nil {
		return req, err
	}

	req.OTPHash = identity.HashToken(code)
	req.OTPExpiresAt = now.Add(OTPTTL)
	req.OTPAttempts = 0
	req.Status = StatusOTPSent
	req.UpdatedAt = now

	if _, err := s.db.WriteWithAudit(ctx, subject(req.FileID),
		[]ddb.Write{{Item: req}},
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, subject(req.FileID), now, audit.ActionSignatureOTPSent,
				signerActor(req, ip, userAgent),
				// Le code n'apparaît évidemment pas au journal : seul le
				// fait qu'un code ait été envoyé, et à quelle adresse.
				map[string]any{"requestId": req.ID, "channel": "email", "to": req.SignerEmail})
		}); err != nil {
		return req, err
	}

	if s.mailer != nil {
		if err := s.mailer.Send(ctx, req.SignerEmail,
			fmt.Sprintf("Votre code de signature : %s", code),
			otpEmail(req, code)); err != nil {
			return req, fmt.Errorf("envoi du code: %w", err)
		}
	}

	return req, nil
}

// ConfirmInput porte la signature soumise par le signataire.
type ConfirmInput struct {
	OTP     string
	Mention string
	// DrawingPNG est l'image du tracé produite par le canvas.
	DrawingPNG []byte
	// StrokeCount et DurationMs caractérisent le geste.
	StrokeCount int
	DurationMs  int64
	IP          string
	UserAgent   string
}

// Confirm vérifie le code, appose la signature et scelle le document.
func (s *Service) Confirm(ctx context.Context, token string, in ConfirmInput) (Request, error) {
	req, err := s.Resolve(ctx, token)
	if err != nil {
		return Request{}, err
	}
	now := s.now()
	if err := req.Signable(now); err != nil {
		return req, err
	}

	if err := s.verifyOTP(ctx, &req, in, now); err != nil {
		return req, err
	}
	if err := validateDrawing(in); err != nil {
		return req, err
	}

	// Le tracé est rangé avant le rendu : le document signé y fait référence,
	// et un tracé perdu rendrait le document invérifiable.
	drawingKey := fmt.Sprintf("orgs/%s/files/%s/signatures/%s-drawing.png", req.OrgID, req.FileID, req.ID)
	if s.blobs != nil {
		if err := s.blobs.Put(ctx, drawingKey, "image/png", in.DrawingPNG); err != nil {
			return req, fmt.Errorf("stockage du tracé: %w", err)
		}
	}

	applied := []doc.AppliedSignature{{
		Role:     req.Role,
		Name:     req.SignerName,
		Mention:  in.Mention,
		SignedAt: now,
		Drawing:  in.DrawingPNG,
	}}
	sealed, err := s.renderer.Render(ctx, req, applied)
	if err != nil {
		return req, fmt.Errorf("rendu du document signé: %w", err)
	}

	sealedSum := sha256.Sum256(sealed)
	sealedKey := fmt.Sprintf("orgs/%s/files/%s/documents/%s-signe.pdf", req.OrgID, req.FileID, req.Reference)
	if s.blobs != nil {
		if err := s.blobs.Put(ctx, sealedKey, "application/pdf", sealed); err != nil {
			return req, fmt.Errorf("archivage du document signé: %w", err)
		}
	}

	proof := &Proof{
		SignedAt:      now,
		Mention:       in.Mention,
		StrokeCount:   in.StrokeCount,
		DurationMs:    in.DurationMs,
		DrawingKey:    drawingKey,
		DrawingSHA256: sha256hex(in.DrawingPNG),
		IP:            in.IP,
		UserAgent:     in.UserAgent,
		OTPChannel:    "email",
		SealedSHA256:  hex.EncodeToString(sealedSum[:]),
		SealedKey:     sealedKey,
	}

	// L'horodatage vient d'un tiers ou n'existe pas. Son échec n'annule pas
	// la signature : le document reste scellé et son empreinte journalisée,
	// et le dossier de preuve indiquera l'absence d'horodatage qualifié
	// plutôt que d'en inventer un.
	if s.stamper != nil {
		if stamp, authority, err := s.stamper.Stamp(ctx, sealedSum[:]); err == nil {
			proof.TimestampToken = string(stamp)
			proof.TimestampTSA = authority
		}
	}

	req.Proof = proof
	req.Status = StatusSigned
	req.OTPHash = ""
	req.UpdatedAt = now

	_, err = s.db.WriteWithAudit(ctx, subject(req.FileID),
		[]ddb.Write{{
			Item: req,
			// Deux soumissions simultanées ne peuvent pas produire deux
			// signatures : la seconde trouve un statut qui n'est plus celui
			// attendu et échoue.
			Condition: "#s <> :signed",
			Names:     map[string]string{"#s": "status"},
			Values:    ddb.StringValues(map[string]string{":signed": string(StatusSigned)}),
		}},
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, subject(req.FileID), now, audit.ActionDocumentSigned,
				signerActor(req, in.IP, in.UserAgent),
				map[string]any{
					"requestId":      req.ID,
					"reference":      req.Reference,
					"role":           string(req.Role),
					"mention":        in.Mention,
					"strokeCount":    in.StrokeCount,
					"durationMs":     in.DurationMs,
					"drawingSha256":  proof.DrawingSHA256,
					"unsignedSha256": req.UnsignedSHA256,
					"sealedSha256":   proof.SealedSHA256,
					"timestampTsa":   proof.TimestampTSA,
				})
		})
	if err != nil {
		if errors.Is(err, ddb.ErrConflict) {
			return req, ErrAlreadySigned
		}
		return req, err
	}

	return req, nil
}

// verifyOTP contrôle le code et consigne les échecs.
//
// Un échec est journalisé au même titre qu'une réussite : un dossier de preuve
// qui ne montrerait que les tentatives abouties serait une reconstitution, pas
// un journal.
func (s *Service) verifyOTP(ctx context.Context, req *Request, in ConfirmInput, now time.Time) error {
	fail := func(reason string) error {
		req.OTPAttempts++
		req.UpdatedAt = now
		attempts := req.OTPAttempts
		if attempts >= MaxOTPAttempts {
			// Le code est brûlé : il faut en redemander un. Sans cela, trois
			// essais deviendraient une infinité d'essais.
			req.OTPHash = ""
		}
		if _, err := s.db.WriteWithAudit(ctx, subject(req.FileID),
			[]ddb.Write{{Item: *req}},
			func(prev audit.Event) (audit.Event, error) {
				return audit.Append(prev, subject(req.FileID), now, audit.ActionSignatureOTPFail,
					signerActor(*req, in.IP, in.UserAgent),
					map[string]any{"requestId": req.ID, "reason": reason, "attempt": attempts})
			}); err != nil {
			return err
		}
		if attempts >= MaxOTPAttempts {
			return ErrOTPExhausted
		}
		return ErrOTPInvalid
	}

	switch {
	case req.OTPHash == "":
		return ErrOTPExhausted
	case req.OTPAttempts >= MaxOTPAttempts:
		return ErrOTPExhausted
	case now.After(req.OTPExpiresAt):
		return fail("expiré")
	}

	// Comparaison à temps constant : un code à six chiffres est court, et une
	// comparaison ordinaire laisserait fuir sa progression par le temps.
	if subtle.ConstantTimeCompare([]byte(identity.HashToken(in.OTP)), []byte(req.OTPHash)) != 1 {
		return fail("code incorrect")
	}

	req.OTPHash = ""
	if _, err := s.db.WriteWithAudit(ctx, subject(req.FileID),
		[]ddb.Write{{Item: *req}},
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, subject(req.FileID), now, audit.ActionSignatureOTPOK,
				signerActor(*req, in.IP, in.UserAgent),
				map[string]any{"requestId": req.ID})
		}); err != nil {
		return err
	}
	return nil
}

// validateDrawing refuse ce qui ne peut pas être présenté comme un tracé
// manuscrit.
func validateDrawing(in ConfirmInput) error {
	switch {
	case len(in.DrawingPNG) < 256:
		return ErrDrawingTooPoor
	case in.StrokeCount < 1:
		return ErrDrawingTooPoor
	case in.DurationMs < 200:
		// Moins de deux dixièmes de seconde : c'est un clic, pas une
		// signature. Le seuil est bas à dessein — il écarte l'automate, pas
		// la personne pressée.
		return ErrDrawingTooPoor
	}
	return nil
}

// ListForFile renvoie les demandes d'un dossier, la plus récente en tête.
func (s *Service) ListForFile(ctx context.Context, orgID, fileID string) ([]Request, error) {
	return ddb.Query[Request](ctx, s.db, ddb.QuerySpec{
		Index:      "GSI1",
		PK:         ddb.OrgPK(orgID) + "#SIGFILE#" + fileID,
		Descending: true,
	})
}

// Sealed relit le document signé archivé et contrôle son empreinte.
//
// C'est la fonction de vérification : elle refait exactement ce que ferait un
// contrôleur, en recalculant l'empreinte du fichier archivé et en la comparant
// à celle inscrite au journal au moment de la signature.
func (s *Service) Sealed(ctx context.Context, req Request) ([]byte, error) {
	if req.Proof == nil || req.Proof.SealedKey == "" {
		return nil, fmt.Errorf("ce document n'a pas encore été signé")
	}
	if s.blobs == nil {
		return nil, fmt.Errorf("archivage indisponible")
	}

	sealed, err := s.blobs.Get(ctx, req.Proof.SealedKey)
	if err != nil {
		return nil, fmt.Errorf("lecture du document archivé: %w", err)
	}
	if got := sha256hex(sealed); got != req.Proof.SealedSHA256 {
		return nil, fmt.Errorf(
			"intégrité rompue : le document archivé a pour empreinte %s, %s était attendue",
			short(got), short(req.Proof.SealedSHA256))
	}
	return sealed, nil
}

func signerActor(req Request, ip, userAgent string) audit.Actor {
	return audit.Actor{
		Type:      audit.ActorLearner,
		ID:        req.ID,
		Label:     req.SignerName + " <" + req.SignerEmail + ">",
		IP:        ip,
		UserAgent: userAgent,
	}
}

func sha256hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func short(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12] + "…"
}
