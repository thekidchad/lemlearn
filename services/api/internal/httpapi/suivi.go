package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/crm"
)

// Le suivi d'une fiche : notes, rappels, pièces jointes.
//
// Les trois se lisent en un seul appel : ils s'affichent ensemble, et trois
// requêtes pour un écran qui n'en montre qu'un feraient trois fois le travail.

func handleSuivi(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		contactID := chi.URLParam(r, "contactID")

		notes, err := deps.CRM.ListNotes(r.Context(), session.OrgID, contactID)
		if err != nil {
			deps.Log.Error("notes", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		rappels, err := deps.CRM.ListRappels(r.Context(), session.OrgID, contactID)
		if err != nil {
			deps.Log.Error("rappels", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		pieces, err := deps.CRM.ListPieces(r.Context(), session.OrgID, contactID)
		if err != nil {
			deps.Log.Error("pièces", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"notes": list(notes), "rappels": list(rappels), "pieces": list(pieces),
		})
	}
}

func handleAddNote(deps Deps) http.HandlerFunc {
	type request struct {
		Body string `json:"body"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		auteur, _ := deps.Identity.LoadUser(r.Context(), session)
		note, err := deps.CRM.AddNote(r.Context(), session.OrgID,
			chi.URLParam(r, "contactID"), body.Body, auteur.FullName())
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, note)
	}
}

func handleDeleteNote(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if err := deps.CRM.DeleteNote(r.Context(), session.OrgID,
			chi.URLParam(r, "contactID"), chi.URLParam(r, "noteID")); err != nil {
			respondNotFound(w, err, "note introuvable")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleAddRappel(deps Deps) http.HandlerFunc {
	type request struct {
		Title        string `json:"title"`
		DueOn        string `json:"dueOn"`
		AssigneeID   string `json:"assigneeId"`
		AssigneeName string `json:"assigneeName"`
		Comments     string `json:"comments"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		auteur, _ := deps.Identity.LoadUser(r.Context(), session)

		rappel, err := deps.CRM.AddRappel(r.Context(), crm.RappelInput{
			OrgID: session.OrgID, ContactID: chi.URLParam(r, "contactID"),
			Title: body.Title, DueOn: body.DueOn,
			AssigneeID: body.AssigneeID, AssigneeName: body.AssigneeName,
			Comments: body.Comments, Author: auteur.FullName(),
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, rappel)
	}
}

func handleCloseRappel(deps Deps) http.HandlerFunc {
	type request struct {
		Done bool `json:"done"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		auteur, _ := deps.Identity.LoadUser(r.Context(), session)

		rappel, err := deps.CRM.CloseRappel(r.Context(), session.OrgID,
			chi.URLParam(r, "contactID"), chi.URLParam(r, "rappelID"),
			body.Done, auteur.FullName())
		if err != nil {
			respondNotFound(w, err, "rappel introuvable")
			return
		}
		writeJSON(w, http.StatusOK, rappel)
	}
}

func handleDeleteRappel(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if err := deps.CRM.DeleteRappel(r.Context(), session.OrgID,
			chi.URLParam(r, "contactID"), chi.URLParam(r, "rappelID")); err != nil {
			respondNotFound(w, err, "rappel introuvable")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handlePreparePiece signe le dépôt d'une pièce jointe.
func handlePreparePiece(deps Deps) http.HandlerFunc {
	type request struct {
		Filename    string `json:"filename"`
		ContentType string `json:"contentType"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		url, key, err := deps.CRM.PreparePiece(r.Context(), session.OrgID,
			chi.URLParam(r, "contactID"), body.Filename, body.ContentType)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"uploadUrl": url, "key": key,
			"expiresInSeconds": int(crm.PieceUploadTTL.Seconds()),
		})
	}
}

// handleAttachPiece enregistre la pièce déposée.
func handleAttachPiece(deps Deps) http.HandlerFunc {
	type request struct {
		Key         string `json:"key"`
		Name        string `json:"name"`
		ContentType string `json:"contentType"`
		SizeBytes   int64  `json:"sizeBytes"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		auteur, _ := deps.Identity.LoadUser(r.Context(), session)

		piece, err := deps.CRM.AttachPiece(r.Context(), session.OrgID,
			chi.URLParam(r, "contactID"), body.Key, body.Name,
			body.ContentType, body.SizeBytes, auteur.FullName())
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, piece)
	}
}

// handlePieceURL rend un lien de lecture de courte durée.
func handlePieceURL(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		url, err := deps.CRM.PieceURL(r.Context(), session.OrgID,
			chi.URLParam(r, "contactID"), chi.URLParam(r, "pieceID"))
		if err != nil {
			respondNotFound(w, err, "pièce introuvable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"url": url, "expiresInSeconds": int(crm.LectureTTL.Seconds()),
		})
	}
}

func handleDeletePiece(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if err := deps.CRM.DeletePiece(r.Context(), session.OrgID,
			chi.URLParam(r, "contactID"), chi.URLParam(r, "pieceID")); err != nil {
			respondNotFound(w, err, "pièce introuvable")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
