package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/video"
)

// handleReserveVideo réserve un emplacement et renvoie l'URL de dépôt direct.
//
// Le navigateur écrit ensuite dans S3 sans repasser par l'API : c'est ce qui
// permet de téléverser une heure de vidéo sans dimensionner la Lambda pour.
func handleReserveVideo(deps Deps) http.HandlerFunc {
	type request struct {
		ContentType string `json:"contentType"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Video == nil {
			writeError(w, http.StatusServiceUnavailable, "hébergement vidéo indisponible")
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		asset, uploadURL, err := deps.Video.Reserve(r.Context(), session.OrgID, body.ContentType)
		if err != nil {
			deps.Log.Error("réservation vidéo", "err", err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"asset": asset, "uploadUrl": uploadURL,
			"expiresInSeconds": int(video.UploadTTL.Seconds()),
		})
	}
}

// handleVideoUploaded déclenche le transcodage une fois le dépôt terminé.
func handleVideoUploaded(deps Deps) http.HandlerFunc {
	type request struct {
		DurationMs int64 `json:"durationMs"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Video == nil {
			writeError(w, http.StatusServiceUnavailable, "hébergement vidéo indisponible")
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}

		asset, err := deps.Video.Uploaded(r.Context(), session.OrgID,
			chi.URLParam(r, "assetID"), body.DurationMs, actorFrom(r, user.FullName()))
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, asset)
	}
}

// handleGetVideo renvoie l'état d'une vidéo, rafraîchi auprès de l'encodeur.
func handleGetVideo(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Video == nil {
			writeError(w, http.StatusServiceUnavailable, "hébergement vidéo indisponible")
			return
		}

		asset, err := deps.Video.Refresh(r.Context(), session.OrgID, chi.URLParam(r, "assetID"))
		if err != nil {
			respondNotFound(w, err, "vidéo introuvable")
			return
		}
		writeJSON(w, http.StatusOK, asset)
	}
}

// handleListVideos liste les vidéos de l'organisation.
func handleListVideos(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Video == nil {
			writeError(w, http.StatusServiceUnavailable, "hébergement vidéo indisponible")
			return
		}

		assets, err := deps.Video.List(r.Context(), session.OrgID, 200)
		if err != nil {
			deps.Log.Error("liste vidéo", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"assets": list(assets)})
	}
}

// handlePlayback autorise la lecture d'un module par un apprenant inscrit.
//
// L'autorisation est courte et refaite à chaque ouverture : un lien de lecture
// qui circulerait dans un groupe de messagerie serait périmé avant d'avoir
// servi.
func handlePlayback(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Video == nil || deps.Catalog == nil {
			writeError(w, http.StatusServiceUnavailable, "diffusion indisponible")
			return
		}

		// L'inscription est vérifiée avant de signer : c'est elle qui fait la
		// différence entre un apprenant et quelqu'un qui connaît une URL.
		target, _, err := learnerTarget(deps, r)
		if err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if _, err := deps.Catalog.GetEnrollment(r.Context(), session.OrgID,
			target.SessionID, target.ContactID); err != nil {
			writeError(w, http.StatusForbidden, "aucune inscription à cette session")
			return
		}

		module, err := deps.Catalog.GetModule(r.Context(), session.OrgID, target.CourseID, target.ModuleID)
		if err != nil {
			respondNotFound(w, err, "module introuvable")
			return
		}
		if module.AssetID == "" {
			writeError(w, http.StatusNotFound, "ce module ne porte aucune vidéo")
			return
		}

		playback, err := deps.Video.Playback(r.Context(), session.OrgID, module.AssetID)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, playback)
	}
}

// handleManifest sert le manifeste HLS d'un module à un apprenant inscrit.
//
// Le manifeste passe par l'API, les segments non : c'est la seule façon de
// rendre le flux lisible partout, y compris par le lecteur natif de Safari sur
// iPhone, qui ne laisse aucune prise à JavaScript sur ses requêtes.
func handleManifest(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Video == nil || deps.Catalog == nil {
			writeError(w, http.StatusServiceUnavailable, "diffusion indisponible")
			return
		}

		target, _, err := learnerTarget(deps, r)
		if err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if _, err := deps.Catalog.GetEnrollment(r.Context(), session.OrgID,
			target.SessionID, target.ContactID); err != nil {
			writeError(w, http.StatusForbidden, "aucune inscription à cette session")
			return
		}

		module, err := deps.Catalog.GetModule(r.Context(), session.OrgID, target.CourseID, target.ModuleID)
		if err != nil {
			respondNotFound(w, err, "module introuvable")
			return
		}
		if module.AssetID == "" {
			writeError(w, http.StatusNotFound, "ce module ne porte aucune vidéo")
			return
		}

		// Les sous-manifestes repassent par cette même route : leurs segments
		// doivent être réécrits à leur tour. Le renvoi ne porte que la
		// requête, sans chemin : il se résout donc contre l'URL par laquelle
		// le lecteur nous a joints, que ce soit l'API en direct ou le relais
		// de l'application. Écrire un chemin absolu ici casserait l'un des
		// deux.
		asked := r.URL.Query()
		body, err := deps.Video.Manifest(r.Context(), session.OrgID, module.AssetID,
			asked.Get("rendu"),
			func(name string) string {
				query := r.URL.Query()
				query.Set("rendu", name)
				return "?" + query.Encode()
			})
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}

		w.Header().Set("Content-Type", video.ManifestContentType)
		// Le manifeste porte des URL signées à échéance courte : le mettre en
		// cache le ferait resservir périmé.
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(body))
	}
}
