package httpapi

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/brand"
)

// Visuel d'une formation.
//
// Une liste de titres se parcourt mal : un stagiaire reconnaît sa formation à
// son sujet avant son intitulé, et un espace sans image ressemble à un
// tableur. Le visuel n'est donc pas de la décoration — c'est ce qui rend la
// liste lisible d'un coup d'œil.
//
// Pas de SVG ici, contrairement au logo : c'est une photographie, et laisser
// déposer du SVG ouvrirait un vecteur de script sur un objet servi en public.
var coverTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

// handlePrepareCover signe le dépôt du visuel d'une formation.
func handlePrepareCover(deps Deps) http.HandlerFunc {
	type request struct {
		ContentType string `json:"contentType"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Assets == nil {
			writeError(w, http.StatusServiceUnavailable, "dépôt de visuel indisponible")
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		ext, ok := coverTypes[strings.ToLower(strings.TrimSpace(body.ContentType))]
		if !ok {
			writeError(w, http.StatusBadRequest, "format accepté : JPEG, PNG ou WebP")
			return
		}

		courseID := chi.URLParam(r, "courseID")
		if _, err := deps.Catalog.GetCourse(r.Context(), session.OrgID, courseID); err != nil {
			respondNotFound(w, err, "formation introuvable")
			return
		}

		key := CoverKey(session.OrgID, courseID, ext)
		url, err := deps.Assets.PresignedPut(r.Context(), key, body.ContentType, brand.UploadTTL)
		if err != nil {
			deps.Log.Error("signature du dépôt de visuel", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"uploadUrl":        url,
			"key":              key,
			"expiresInSeconds": int(brand.UploadTTL.Seconds()),
		})
	}
}

// handleAttachCover rattache le visuel déposé, ou le retire.
func handleAttachCover(deps Deps) http.HandlerFunc {
	type request struct {
		Key string `json:"key"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		courseID := chi.URLParam(r, "courseID")
		key := ""
		// La clé n'est pas reprise telle quelle : inventée, elle ferait
		// afficher sur une formation le visuel déposé par une autre
		// organisation. On la recompose depuis la seule extension.
		if strings.TrimSpace(body.Key) != "" {
			ext := strings.ToLower(filepath.Ext(body.Key))
			valide := false
			for _, connue := range coverTypes {
				if ext == connue {
					valide = true
				}
			}
			if !valide {
				writeError(w, http.StatusBadRequest, "visuel dans un format inattendu")
				return
			}
			key = CoverKey(session.OrgID, courseID, ext)
		}

		course, err := deps.Catalog.SetCover(r.Context(), session.OrgID, courseID, key)
		if err != nil {
			respondNotFound(w, err, "formation introuvable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"course":   course,
			"coverUrl": assetURL(deps, course.CoverKey),
		})
	}
}

// assetURL compose l'adresse publique d'une ressource, ou rend une chaîne vide.
//
// Vide plutôt qu'une URL cassée : un visuel absent se remplace par une bande
// aux couleurs de l'organisme, ce qui est mieux qu'une image brisée.
func assetURL(deps Deps, key string) string {
	if key == "" || deps.Brand == nil || deps.Brand.AssetsURL() == "" {
		return ""
	}
	return deps.Brand.AssetsURL() + "/" + strings.TrimPrefix(key, "/")
}
