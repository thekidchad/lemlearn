package httpapi

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/config"
	"github.com/lemlearn/api/internal/platform/doc"
	"github.com/lemlearn/api/internal/signature"
)

// maxDrawingBytes borne l'image du tracé. Un canvas de signature produit
// quelques dizaines de kilooctets ; au-delà, ce n'est plus une signature.
const maxDrawingBytes = 512 << 10

// handleIssueSignature émet une demande de signature sur un dossier.
func handleIssueSignature(deps Deps) http.HandlerFunc {
	type request struct {
		Kind        string `json:"kind"`
		Reference   string `json:"reference"`
		Role        string `json:"role"`
		SignerName  string `json:"signerName"`
		SignerEmail string `json:"signerEmail"`
		SignerPhone string `json:"signerPhone"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Signature == nil {
			writeError(w, http.StatusServiceUnavailable, "signature indisponible")
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		fileID := chi.URLParam(r, "fileID")
		file, err := deps.CRM.GetFile(r.Context(), session.OrgID, fileID)
		if err != nil {
			respondNotFound(w, err, "dossier introuvable")
			return
		}

		// À défaut de référence, celle du dossier suffixée du type de
		// document. Elle nomme le fichier archivé : la laisser vide ferait
		// écrire deux conventions du même dossier sous la même clé.
		if strings.TrimSpace(body.Reference) == "" {
			body.Reference = file.Reference + "-" + body.Kind
		}
		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}

		req, token, err := deps.Signature.Issue(r.Context(), signature.IssueInput{
			OrgID: session.OrgID, FileID: fileID,
			Kind: body.Kind, Reference: body.Reference,
			Role:       doc.SignatureZoneRole(body.Role),
			SignerName: body.SignerName, SignerEmail: body.SignerEmail, SignerPhone: body.SignerPhone,
			Actor: actorFrom(r, user.FullName()),
		})
		if err == nil && deps.Billing != nil {
			// Le compteur suit l'émission, pas la signature : c'est
			// l'émission qui consomme la ressource, et un signataire qui
			// laisse traîner son lien coûte le même rendu documentaire.
			if err := deps.Billing.CountSignature(r.Context(), session.OrgID); err != nil {
				deps.Log.Error("compteur de signatures", "err", err)
			}
		}
		if err != nil {
			// La demande peut exister malgré l'échec de l'envoi : on le dit,
			// plutôt que de laisser croire que rien ne s'est passé.
			if req.ID != "" {
				writeJSON(w, http.StatusAccepted, map[string]any{
					"request": req,
					"warning": fmt.Sprintf("demande créée mais courriel non parti : %v", err),
				})
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Le jeton n'est jamais renvoyé à l'organisme : il n'appartient qu'au
		// signataire. Un administrateur qui pourrait le récupérer pourrait
		// signer à sa place, et le dossier de preuve ne le montrerait pas.
		//
		// Seule exception, explicitement bornée au développement local, où
		// aucun courriel ne part réellement : sans elle, le parcours de
		// signature ne serait pas exerçable sur un poste de développement.
		response := map[string]any{"request": req}
		if deps.Config.Env == config.EnvLocal {
			response["devLink"] = fmt.Sprintf("/v1/sign/%s", token)
		}
		writeJSON(w, http.StatusCreated, response)
	}
}

// handleListSignatures liste les demandes d'un dossier.
func handleListSignatures(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Signature == nil {
			writeError(w, http.StatusServiceUnavailable, "signature indisponible")
			return
		}

		requests, err := deps.Signature.ListForFile(r.Context(), session.OrgID, chi.URLParam(r, "fileID"))
		if err != nil {
			deps.Log.Error("liste des signatures", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"requests": requests})
	}
}

// --- Parcours public du signataire ---------------------------------------
//
// Ces routes ne sont pas authentifiées : le signataire n'a pas de compte. Sa
// légitimité vient du jeton du lien, puis du code envoyé à son adresse.

// handleSignOpen renvoie ce que le signataire doit voir avant de signer.
func handleSignOpen(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Signature == nil {
			writeError(w, http.StatusServiceUnavailable, "signature indisponible")
			return
		}

		req, _, err := deps.Signature.Open(r.Context(), chi.URLParam(r, "token"), clientIP(r), truncateUA(r.UserAgent()))
		if err != nil {
			respondSignatureError(w, deps, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"reference":  req.Reference,
			"kind":       req.Kind,
			"signerName": req.SignerName,
			"signerHint": maskEmail(req.SignerEmail),
			"status":     req.Status,
			"expiresAt":  req.ExpiresAt,
			"sha256":     req.UnsignedSHA256,
		})
	}
}

// handleSignDocument sert le PDF à signer.
func handleSignDocument(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Signature == nil {
			writeError(w, http.StatusServiceUnavailable, "signature indisponible")
			return
		}

		req, pdf, err := deps.Signature.Open(r.Context(), chi.URLParam(r, "token"), clientIP(r), truncateUA(r.UserAgent()))
		if err != nil {
			respondSignatureError(w, deps, err)
			return
		}
		servePDF(w, req.Reference, pdf)
	}
}

// handleSignOTP envoie le code de vérification.
func handleSignOTP(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Signature == nil {
			writeError(w, http.StatusServiceUnavailable, "signature indisponible")
			return
		}

		req, err := deps.Signature.SendOTP(r.Context(), chi.URLParam(r, "token"), clientIP(r), truncateUA(r.UserAgent()))
		if err != nil {
			respondSignatureError(w, deps, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"sentTo":    maskEmail(req.SignerEmail),
			"expiresIn": int(signature.OTPTTL.Seconds()),
			"attempts":  signature.MaxOTPAttempts,
		})
	}
}

// handleSignConfirm vérifie le code et appose la signature.
func handleSignConfirm(deps Deps) http.HandlerFunc {
	type request struct {
		OTP     string `json:"otp"`
		Mention string `json:"mention"`
		// DrawingPNG arrive en base64 : le canvas produit un data: URI, dont
		// le client retire l'en-tête.
		DrawingPNG  string `json:"drawingPng"`
		StrokeCount int    `json:"strokeCount"`
		DurationMs  int64  `json:"durationMs"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Signature == nil {
			writeError(w, http.StatusServiceUnavailable, "signature indisponible")
			return
		}

		var body request
		if !decodeJSONLimit(w, r, &body, maxDrawingBytes*2) {
			return
		}

		png, err := base64.StdEncoding.DecodeString(body.DrawingPNG)
		if err != nil {
			writeError(w, http.StatusBadRequest, "tracé de signature illisible")
			return
		}
		if len(png) > maxDrawingBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "tracé de signature trop volumineux")
			return
		}

		req, err := deps.Signature.Confirm(r.Context(), chi.URLParam(r, "token"), signature.ConfirmInput{
			OTP: body.OTP, Mention: body.Mention, DrawingPNG: png,
			StrokeCount: body.StrokeCount, DurationMs: body.DurationMs,
			IP: clientIP(r), UserAgent: truncateUA(r.UserAgent()),
		})
		if err != nil {
			respondSignatureError(w, deps, err)
			return
		}

		// Le récépissé reprend les empreintes : le signataire repart avec de
		// quoi vérifier lui-même, sans dépendre de nous.
		writeJSON(w, http.StatusOK, map[string]any{
			"reference":    req.Reference,
			"signedAt":     req.Proof.SignedAt,
			"sealedSha256": req.Proof.SealedSHA256,
			"timestampTsa": req.Proof.TimestampTSA,
		})
	}
}

// handleSignSealed sert le document signé au signataire.
func handleSignSealed(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Signature == nil {
			writeError(w, http.StatusServiceUnavailable, "signature indisponible")
			return
		}

		req, err := deps.Signature.Resolve(r.Context(), chi.URLParam(r, "token"))
		if err != nil {
			respondSignatureError(w, deps, err)
			return
		}

		sealed, err := deps.Signature.Sealed(r.Context(), req)
		if err != nil {
			// L'intégrité rompue est un incident : on refuse de servir le
			// fichier plutôt que de livrer un document dont on sait qu'il a
			// changé.
			deps.Log.Error("document scellé", "request", req.ID, "err", err)
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		servePDF(w, req.Reference+"-signe", sealed)
	}
}

func servePDF(w http.ResponseWriter, name string, pdf []byte) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", strconv.Itoa(len(pdf)))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s.pdf"`, name))
	_, _ = w.Write(pdf)
}

// respondSignatureError traduit les erreurs du parcours en réponses HTTP.
func respondSignatureError(w http.ResponseWriter, deps Deps, err error) {
	switch {
	case errors.Is(err, signature.ErrNotFound):
		// Un jeton inconnu et un jeton expiré ne doivent pas se distinguer
		// d'un coup d'œil : sinon on peut sonder les jetons valides.
		writeError(w, http.StatusNotFound, "lien de signature invalide")
	case errors.Is(err, signature.ErrLinkExpired):
		writeError(w, http.StatusGone, err.Error())
	case errors.Is(err, signature.ErrAlreadySigned):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, signature.ErrOTPInvalid), errors.Is(err, signature.ErrOTPExhausted):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, signature.ErrDrawingTooPoor):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		deps.Log.Error("parcours de signature", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
	}
}

// maskEmail n'expose que de quoi reconnaître son adresse.
//
// La page de signature est publique : afficher l'adresse complète permettrait
// de récolter des adresses à partir d'un lien intercepté.
func maskEmail(email string) string {
	at := -1
	for i, r := range email {
		if r == '@' {
			at = i
			break
		}
	}
	if at <= 0 {
		return "…"
	}
	local := email[:at]
	if len(local) <= 2 {
		return string(local[0]) + "…" + email[at:]
	}
	return local[:2] + "…" + email[at:]
}
