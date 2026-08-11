package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// handleCreateContact enregistre un apprenant, une entreprise ou un financeur.
func handleCreateContact(deps Deps) http.HandlerFunc {
	type request struct {
		Kind        crm.Kind    `json:"kind"`
		FirstName   string      `json:"firstName"`
		LastName    string      `json:"lastName"`
		BirthDate   string      `json:"birthDate"`
		BirthPlace  string      `json:"birthPlace"`
		CompanyName string      `json:"companyName"`
		SIRET       string      `json:"siret"`
		LegalForm   string      `json:"legalForm"`
		Email       string      `json:"email"`
		Phone       string      `json:"phone"`
		Position    string      `json:"position"`
		Notes       string      `json:"notes"`
		Address     crm.Address `json:"address"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		contact := crm.NewContact(session.OrgID, body.Kind, deps.Now())
		contact.FirstName = body.FirstName
		contact.LastName = body.LastName
		contact.BirthDate = body.BirthDate
		contact.BirthPlace = body.BirthPlace
		contact.CompanyName = body.CompanyName
		contact.SIRET = body.SIRET
		contact.LegalForm = body.LegalForm
		contact.Email = body.Email
		contact.Phone = body.Phone
		contact.Position = body.Position
		contact.Notes = body.Notes
		contact.Address = body.Address

		created, err := deps.CRM.CreateContact(r.Context(), contact)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, created)
	}
}

// handleListContacts liste les contacts d'une nature donnée.
func handleListContacts(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		kind := crm.Kind(r.URL.Query().Get("kind"))
		if kind == "" {
			kind = crm.KindLearner
		}

		contacts, err := deps.CRM.ListContacts(r.Context(), session.OrgID, kind, 200)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"contacts": contacts})
	}
}

// handleGetContact lit un contact.
func handleGetContact(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		// La clé de partition vient de la session, jamais de l'URL : demander
		// le contact d'une autre organisation ne produit pas une erreur
		// d'autorisation, mais une absence — la requête n'atteint jamais leur
		// partition.
		contact, err := deps.CRM.GetContact(r.Context(), session.OrgID, chi.URLParam(r, "contactID"))
		if err != nil {
			respondNotFound(w, err, "contact introuvable")
			return
		}
		writeJSON(w, http.StatusOK, contact)
	}
}

// handleCreateFile ouvre un dossier.
func handleCreateFile(deps Deps) http.HandlerFunc {
	type request struct {
		Title     string   `json:"title"`
		LearnerID string   `json:"learnerId"`
		CompanyID string   `json:"companyId"`
		FunderID  string   `json:"funderId"`
		CourseID  string   `json:"courseId"`
		PriceHT   float64  `json:"priceHT"`
		Tags      []string `json:"tags"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}

		file, err := deps.CRM.CreateFile(r.Context(), crm.CreateFileInput{
			OrgID: session.OrgID, Title: body.Title,
			LearnerID: body.LearnerID, CompanyID: body.CompanyID,
			FunderID: body.FunderID, CourseID: body.CourseID,
			PriceHT: body.PriceHT, Tags: body.Tags,
			Actor: actorFrom(r, user.FullName()),
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, file)
	}
}

// handlePipeline renvoie toutes les colonnes du pipeline.
func handlePipeline(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		pipeline, err := deps.CRM.Pipeline(r.Context(), session.OrgID, 50)
		if err != nil {
			deps.Log.Error("pipeline", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"pipeline": pipeline})
	}
}

// handleGetFile lit un dossier.
func handleGetFile(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		file, err := deps.CRM.GetFile(r.Context(), session.OrgID, chi.URLParam(r, "fileID"))
		if err != nil {
			respondNotFound(w, err, "dossier introuvable")
			return
		}
		writeJSON(w, http.StatusOK, file)
	}
}

// handleMoveFile déplace un dossier dans le pipeline.
func handleMoveFile(deps Deps) http.HandlerFunc {
	type request struct {
		Stage crm.Stage `json:"stage"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}

		file, err := deps.CRM.MoveFile(r.Context(), session.OrgID, chi.URLParam(r, "fileID"),
			body.Stage, actorFrom(r, user.FullName()))
		switch {
		case err == nil:
			writeJSON(w, http.StatusOK, file)
		case errors.Is(err, ddb.ErrNotFound):
			writeError(w, http.StatusNotFound, "dossier introuvable")
		case errors.Is(err, ddb.ErrConflict):
			// Quelqu'un d'autre a déplacé la carte entre-temps : l'interface
			// doit recharger plutôt qu'écraser.
			writeError(w, http.StatusConflict, "le dossier a été modifié entre-temps")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
	}
}

// handleFileTimeline renvoie le journal d'audit vérifié d'un dossier.
func handleFileTimeline(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		fileID := chi.URLParam(r, "fileID")

		// Le dossier est relu d'abord : sans cela, connaître un identifiant
		// suffirait à lire le journal d'une autre organisation, puisque le
		// journal est indexé par sujet et non par partition d'organisation.
		if _, err := deps.CRM.GetFile(r.Context(), session.OrgID, fileID); err != nil {
			respondNotFound(w, err, "dossier introuvable")
			return
		}

		events, err := deps.CRM.Timeline(r.Context(), fileID)
		if err != nil {
			// Une chaîne rompue est un incident, pas un détail d'affichage.
			deps.Log.Error("journal rompu", "file", fileID, "err", err)
			writeError(w, http.StatusInternalServerError, "journal d'audit incohérent")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"events": events})
	}
}

func respondNotFound(w http.ResponseWriter, err error, message string) {
	if errors.Is(err, ddb.ErrNotFound) {
		writeError(w, http.StatusNotFound, message)
		return
	}
	writeError(w, http.StatusInternalServerError, "erreur interne")
}
