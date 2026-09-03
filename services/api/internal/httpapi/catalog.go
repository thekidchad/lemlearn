package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/bpf"
	"github.com/lemlearn/api/internal/catalog"
)

// handleCreateCourse enregistre une formation au catalogue.
func handleCreateCourse(deps Deps) http.HandlerFunc {
	type request struct {
		Title         string   `json:"title"`
		Goal          string   `json:"goal"`
		Objectives    []string `json:"objectives"`
		Prerequisites string   `json:"prerequisites"`
		Audience      string   `json:"audience"`
		Means         string   `json:"means"`
		Assessment    string   `json:"assessment"`
		Sanction      string   `json:"sanction"`
		Accessibility string   `json:"accessibility"`
		DurationHours float64  `json:"durationHours"`
		PriceHT       float64  `json:"priceHT"`
		Tags          []string `json:"tags"`
		Published     bool     `json:"published"`
		// Les deux bornes de l'évaluation : celle d'entrée, exigée par
		// Qualiopi, et celle de sortie, qui conditionne l'attestation.
		PositioningQuizID string `json:"positioningQuizId"`
		FinalQuizID       string `json:"finalQuizId"`
		ObjectiveType     string `json:"objectiveType"`
		CertificationCode string `json:"certificationCode"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		course := catalog.NewCourse(session.OrgID, body.Title, deps.Now())
		course.Goal = body.Goal
		course.Objectives = body.Objectives
		course.Prerequisites = body.Prerequisites
		course.Audience = body.Audience
		course.Means = body.Means
		course.Assessment = body.Assessment
		if body.Sanction != "" {
			course.Sanction = body.Sanction
		}
		course.Accessibility = body.Accessibility
		course.DurationHours = body.DurationHours
		course.PriceHT = body.PriceHT
		course.Tags = body.Tags
		course.Published = body.Published
		course.PositioningQuizID = body.PositioningQuizID
		course.FinalQuizID = body.FinalQuizID
		course.ObjectiveType = body.ObjectiveType
		course.CertificationCode = body.CertificationCode

		created, err := deps.Catalog.CreateCourse(r.Context(), course)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, created)
	}
}

// handleListCourses renvoie le catalogue.
func handleListCourses(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		limite, curseur := pageParams(r)
		page, err := deps.Catalog.ListCoursesPage(r.Context(), session.OrgID, limite, curseur)
		if err != nil {
			deps.Log.Error("catalogue", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"courses": list(page.Items), "cursor": page.Cursor})
	}
}

// handleGetCourse renvoie une formation et ses modules.
func handleGetCourse(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		courseID := chi.URLParam(r, "courseID")

		course, err := deps.Catalog.GetCourse(r.Context(), session.OrgID, courseID)
		if err != nil {
			respondNotFound(w, err, "formation introuvable")
			return
		}
		modules, err := deps.Catalog.ListModules(r.Context(), session.OrgID, courseID)
		if err != nil {
			deps.Log.Error("modules", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"course": course, "modules": list(modules),
			"coverUrl": assetURL(deps, course.CoverKey),
		})
	}
}

// handleAddModule ajoute un module à une formation.
func handleAddModule(deps Deps) http.HandlerFunc {
	type request struct {
		Title              string `json:"title"`
		Summary            string `json:"summary"`
		Position           int    `json:"position"`
		AssetID            string `json:"assetId"`
		DurationMs         int64  `json:"durationMs"`
		QuizID             string `json:"quizId"`
		MinCoveragePercent int    `json:"minCoveragePercent"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		courseID := chi.URLParam(r, "courseID")

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		if _, err := deps.Catalog.GetCourse(r.Context(), session.OrgID, courseID); err != nil {
			respondNotFound(w, err, "formation introuvable")
			return
		}

		module := catalog.NewModule(session.OrgID, courseID, body.Title, body.Position, deps.Now())
		module.Summary = body.Summary
		module.AssetID = body.AssetID
		module.DurationMs = body.DurationMs
		module.QuizID = body.QuizID
		if body.MinCoveragePercent > 0 {
			module.MinCoveragePercent = body.MinCoveragePercent
		}

		created, err := deps.Catalog.AddModule(r.Context(), module)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, created)
	}
}

// handleUpdateModule modifie un module existant.
//
// Attacher une vidéo ou un contrôle après coup est le cas courant, pas
// l'exception : un contrôle après module doit désigner un module qui existe,
// donc il ne peut pas être créé en même temps que lui. Sans cette route, la
// seule façon d'attacher un questionnaire serait de recréer le module — et de
// perdre l'assiduité déjà enregistrée.
func handleUpdateModule(deps Deps) http.HandlerFunc {
	type request struct {
		Title              *string `json:"title"`
		Summary            *string `json:"summary"`
		Position           *int    `json:"position"`
		AssetID            *string `json:"assetId"`
		DurationMs         *int64  `json:"durationMs"`
		QuizID             *string `json:"quizId"`
		MinCoveragePercent *int    `json:"minCoveragePercent"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		courseID := chi.URLParam(r, "courseID")

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		module, err := deps.Catalog.GetModule(r.Context(), session.OrgID, courseID,
			chi.URLParam(r, "moduleID"))
		if err != nil {
			respondNotFound(w, err, "module introuvable")
			return
		}

		// Champs absents laissés tels quels : un PATCH qui remettrait à zéro
		// ce qu'il ne mentionne pas effacerait une vidéo au premier
		// changement de titre.
		if body.Title != nil {
			module.Title = *body.Title
		}
		if body.Summary != nil {
			module.Summary = *body.Summary
		}
		if body.Position != nil {
			module.Position = *body.Position
		}
		if body.AssetID != nil {
			module.AssetID = *body.AssetID
		}
		if body.DurationMs != nil {
			module.DurationMs = *body.DurationMs
		}
		if body.QuizID != nil {
			module.QuizID = *body.QuizID
		}
		if body.MinCoveragePercent != nil {
			module.MinCoveragePercent = *body.MinCoveragePercent
		}

		updated, err := deps.Catalog.SaveModule(r.Context(), module)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, updated)
	}
}

// handleCreateSession planifie une session.
func handleCreateSession(deps Deps) http.HandlerFunc {
	type request struct {
		CourseID  string       `json:"courseId"`
		Title     string       `json:"title"`
		Mode      catalog.Mode `json:"mode"`
		StartsAt  time.Time    `json:"startsAt"`
		EndsAt    time.Time    `json:"endsAt"`
		Location  string       `json:"location"`
		TrainerID string       `json:"trainerId"`
		Capacity  int          `json:"capacity"`
		Tags      []string     `json:"tags"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		planned := catalog.NewSession(session.OrgID, body.CourseID, body.Title,
			body.Mode, body.StartsAt, body.EndsAt, deps.Now())
		planned.Location = body.Location
		planned.TrainerID = body.TrainerID
		planned.Capacity = body.Capacity
		planned.Tags = body.Tags

		created, err := deps.Catalog.CreateSession(r.Context(), planned)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, created)
	}
}

// handleListSessions renvoie l'agenda.
func handleListSessions(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		limite, curseur := pageParams(r)
		page, err := deps.Catalog.ListSessionsPage(r.Context(), session.OrgID, limite, curseur)
		if err != nil {
			deps.Log.Error("agenda", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": list(page.Items), "cursor": page.Cursor})
	}
}

// handleEnroll inscrit un apprenant à une session.
func handleEnroll(deps Deps) http.HandlerFunc {
	type request struct {
		ContactID string `json:"contactId"`
		FileID    string `json:"fileId"`
		// Ce que réclameront la convention et le bilan.
		TraineeType    string  `json:"traineeType"`
		ContractStart  string  `json:"contractStart"`
		ContractEnd    string  `json:"contractEnd"`
		HoursElearning float64 `json:"hoursElearning"`
		HoursRemote    float64 `json:"hoursRemote"`
		HoursOnSite    float64 `json:"hoursOnSite"`
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

		// Le type de stagiaire est vérifié à la saisie plutôt qu'au dépôt du
		// bilan : une valeur inconnue passerait inaperçue jusqu'en avril, et
		// ressortirait alors comme une ligne « non classée » qu'il faudrait
		// reconstituer un an après.
		if body.TraineeType != "" && !bpf.TypeStagiaire(body.TraineeType).Valid() {
			writeError(w, http.StatusBadRequest, "type de stagiaire inconnu du bilan")
			return
		}

		enrollment, err := deps.Catalog.Enroll(r.Context(), catalog.EnrollInput{
			OrgID: session.OrgID, SessionID: chi.URLParam(r, "sessionID"),
			ContactID: body.ContactID, FileID: body.FileID,
			TraineeType:    body.TraineeType,
			ContractStart:  jourOuRien(body.ContractStart),
			ContractEnd:    jourOuRien(body.ContractEnd),
			HoursElearning: body.HoursElearning,
			HoursRemote:    body.HoursRemote,
			HoursOnSite:    body.HoursOnSite,
			Actor:          actorFrom(r, user.FullName()),
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, enrollment)
	}
}

// handleListEnrollments renvoie les inscrits d'une session — la base de la
// feuille d'émargement.
func handleListEnrollments(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		enrollments, err := deps.Catalog.ListSessionEnrollments(
			r.Context(), session.OrgID, chi.URLParam(r, "sessionID"))
		if err != nil {
			deps.Log.Error("inscrits", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"enrollments": list(enrollments)})
	}
}

// jourOuRien lit une date au format AAAA-MM-JJ, ou rend rien.
//
// Rien plutôt qu'une date par défaut : une période de contrat inventée
// figurerait telle quelle sur la convention, et personne ne verrait qu'elle a
// été devinée.
func jourOuRien(valeur string) *time.Time {
	if strings.TrimSpace(valeur) == "" {
		return nil
	}
	jour, err := time.Parse("2006-01-02", valeur)
	if err != nil {
		return nil
	}
	return &jour
}
