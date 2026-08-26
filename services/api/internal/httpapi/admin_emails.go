package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/emailtpl"
)

// handleListEmailTemplates renvoie l'état des gabarits.
func handleListEmailTemplates(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Emails == nil {
			writeError(w, http.StatusServiceUnavailable, "gabarits indisponibles")
			return
		}

		templates, err := deps.Emails.List(r.Context())
		if err != nil {
			deps.Log.Error("gabarits", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"templates": templates})
	}
}

// handleSaveEmailTemplate enregistre une réécriture, après l'avoir rendue.
//
// Un gabarit qui ne s'exécute pas est refusé ici plutôt que découvert au
// moment où un signataire attend son code.
func handleSaveEmailTemplate(deps Deps) http.HandlerFunc {
	type request struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Emails == nil {
			writeError(w, http.StatusServiceUnavailable, "gabarits indisponibles")
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		author := session.UserID
		if user, err := deps.Identity.LoadUser(r.Context(), session); err == nil {
			author = user.Email
		}

		saved, err := deps.Emails.Save(r.Context(), chi.URLParam(r, "key"),
			body.Subject, body.Body, author)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}

// handleResetEmailTemplate revient au gabarit d'origine.
func handleResetEmailTemplate(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Emails == nil {
			writeError(w, http.StatusServiceUnavailable, "gabarits indisponibles")
			return
		}

		restored, err := deps.Emails.Reset(r.Context(), chi.URLParam(r, "key"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, restored)
	}
}

// handlePreviewEmailTemplate rend un gabarit sans l'enregistrer.
func handlePreviewEmailTemplate(deps Deps) http.HandlerFunc {
	type request struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		message, err := emailtpl.Preview(chi.URLParam(r, "key"), body.Subject, body.Body)
		if err != nil {
			// L'erreur de gabarit est renvoyée telle quelle : « fonction
			// "prenom" inconnue » se corrige, « erreur interne » non.
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, message)
	}
}

// handleMailJournal renvoie les courriels partis.
//
// C'est la question qu'on pose quand un apprenant dit n'avoir rien reçu : le
// message est-il parti, quand, à quelle adresse, et l'expéditeur l'a-t-il
// accepté. Le corps n'y est pas — il porte des liens de signature et des codes
// à usage unique.
func handleMailJournal(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.MailJournal == nil {
			writeError(w, http.StatusServiceUnavailable, "journal des envois indisponible")
			return
		}

		entries, err := deps.MailJournal.Recent(r.Context(), deps.Now(),
			r.URL.Query().Get("orgId"), r.URL.Query().Get("q"), 200)
		if err != nil {
			deps.Log.Error("journal des envois", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		delivered := 0
		for _, entry := range entries {
			if entry.Delivered {
				delivered++
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"entries":   list(entries),
			"delivered": delivered,
			"failed":    len(entries) - delivered,
			"since":     deps.Now().AddDate(0, -1, 0).Format(time.RFC3339),
		})
	}
}
