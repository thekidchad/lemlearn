package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/brand"
	"github.com/lemlearn/api/internal/identity"
)

// Le compte, vu par celui à qui il appartient.
//
// Un écran manquait à chacun des trois espaces : le stagiaire, l'organisme et
// l'équipe. Personne ne pouvait corriger son nom, changer son mot de passe ni
// poser sa photo — un prénom mal orthographié à l'invitation le restait, et le
// mot de passe du premier jour ne pouvait plus changer, ce qui est la meilleure
// façon de faire durer un mot de passe compromis.

// photoTypes borne ce qu'on accepte comme photo.
//
// Pas de SVG : c'est une photographie, et un SVG servi depuis le compartiment
// public serait un document exécutable.
var photoTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

// handleProfil rend le compte de celui qui est connecté.
func handleProfil(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}
		org, err := deps.Identity.LoadOrg(r.Context(), session.OrgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "organisation introuvable")
			return
		}

		compte := map[string]any{
			"id": user.ID, "email": user.Email,
			"firstName": user.FirstName, "lastName": user.LastName,
			"role": string(user.Role), "roleLabel": roleLabels[user.Role],
			"contactId": user.ContactID,
			"photoUrl":  photoURL(deps, user),
		}
		if user.LastLoginAt != nil {
			compte["lastLoginAt"] = user.LastLoginAt.Format("2006-01-02T15:04:05Z07:00")
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"compte": compte,
			"org":    map[string]any{"id": org.ID, "name": org.Name},
			// L'impersonation se voit ici comme partout ailleurs : on doit
			// savoir de quel compte on est en train de modifier le mot de passe.
			"impersonatedBy": session.ImpersonatedBy,
		})
	}
}

// handleUpdateProfil corrige le nom de celui qui est connecté.
func handleUpdateProfil(deps Deps) http.HandlerFunc {
	type request struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		user, err := deps.Identity.UpdateSelf(r.Context(), session, body.FirstName, body.LastName)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": user.Public()})
	}
}

// handleChangePassword remplace le mot de passe.
func handleChangePassword(deps Deps) http.HandlerFunc {
	type request struct {
		Current string `json:"current"`
		Next    string `json:"next"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		// Pendant une impersonation, le mot de passe change appartiendrait au
		// client : l'équipe verrouillerait son compte en croyant régler le
		// sien. Le geste est donc refusé tant qu'on est chez quelqu'un d'autre.
		if session.ImpersonatedBy != "" {
			writeError(w, http.StatusConflict,
				"vous êtes dans le compte de quelqu'un d'autre : son mot de passe lui appartient")
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		if err := deps.Identity.ChangePassword(r.Context(), session, body.Current, body.Next); err != nil {
			if errors.Is(err, identity.ErrWrongPassword) {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handlePreparePhoto signe le dépôt d'une photo de profil.
func handlePreparePhoto(deps Deps) http.HandlerFunc {
	type request struct {
		ContentType string `json:"contentType"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Assets == nil {
			writeError(w, http.StatusServiceUnavailable, "dépôt d'image indisponible")
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		ext, ok := photoTypes[strings.ToLower(strings.TrimSpace(body.ContentType))]
		if !ok {
			writeError(w, http.StatusBadRequest, "format accepté : JPEG, PNG ou WebP")
			return
		}

		// Un identifiant neuf à chaque dépôt : sans lui, remplacer sa photo
		// laisserait l'ancienne dans les caches, et l'écran continuerait
		// d'afficher un visage qu'on vient de changer.
		key := identity.PhotoPrefix(session.OrgID, session.UserID) + identity.NewID() + ext
		url, err := deps.Assets.PresignedPut(r.Context(), key, body.ContentType, brand.UploadTTL)
		if err != nil {
			deps.Log.Error("signature du dépôt de photo", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"uploadUrl": url, "key": key,
			"expiresInSeconds": int(brand.UploadTTL.Seconds()),
		})
	}
}

// handleAttachPhoto rattache la photo déposée, ou la retire.
func handleAttachPhoto(deps Deps) http.HandlerFunc {
	type request struct {
		Key string `json:"key"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		ancienne := ""
		if avant, err := deps.Identity.LoadUser(r.Context(), session); err == nil {
			ancienne = avant.PhotoKey
		}

		user, err := deps.Identity.SetPhoto(r.Context(), session, strings.TrimSpace(body.Key))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// L'ancienne image est effacée après coup : la garder ferait grossir le
		// compartiment d'un objet par changement d'avis.
		if ancienne != "" && ancienne != user.PhotoKey && deps.Assets != nil {
			if effaceur, ok := deps.Assets.(interface {
				Delete(ctx context.Context, key string) error
			}); ok {
				_ = effaceur.Delete(r.Context(), ancienne)
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"user": user.Public(), "photoUrl": photoURL(deps, user),
		})
	}
}

// PhotoTTL borne la durée d'un lien de photo.
//
// Une heure : assez pour la visite en cours, assez court pour qu'un lien
// recopié dans un historique de navigation cesse vite de valoir.
const PhotoTTL = time.Hour

// photoURL signe un lien de lecture sur la photo, ou rend rien.
//
// Signé et non public, contrairement au logo d'un organisme. Un logo est fait
// pour être vu de tous ; le visage d'une personne est une donnée personnelle,
// et le compartiment public de ce produit est ouvert préfixe par préfixe
// précisément pour qu'on ne l'y verse pas par commodité.
func photoURL(deps Deps, user identity.User) string {
	if user.PhotoKey == "" || deps.Assets == nil {
		return ""
	}
	signeur, ok := deps.Assets.(interface {
		PresignedGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	})
	if !ok {
		return ""
	}
	url, err := signeur.PresignedGet(context.Background(), user.PhotoKey, PhotoTTL)
	if err != nil {
		return ""
	}
	return url
}
