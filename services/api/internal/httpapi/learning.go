package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/learning"
	"github.com/lemlearn/api/internal/lms"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
	"github.com/lemlearn/api/internal/quiz"
)

// learnerTarget résout l'apprenant concerné par une requête de l'espace
// apprenant.
//
// Un apprenant n'agit jamais que pour lui-même : l'identifiant du contact
// vient de son compte, jamais de l'URL. Un formateur ou un administrateur peut
// en revanche viser un apprenant précis — c'est ce qui permet de saisir un
// émargement ou de corriger une copie.
func learnerTarget(deps Deps, r *http.Request) (learning.Target, identity.User, error) {
	session, _ := sessionFrom(r)
	user, err := deps.Identity.LoadUser(r.Context(), session)
	if err != nil {
		return learning.Target{}, identity.User{}, err
	}

	contactID := user.ContactID
	if user.Role.CanTeach() {
		if requested := r.URL.Query().Get("contactId"); requested != "" {
			contactID = requested
		}
	}
	if contactID == "" {
		return learning.Target{}, user, errors.New("ce compte n'est rattaché à aucune fiche apprenant")
	}

	return learning.Target{
		OrgID:     session.OrgID,
		SessionID: chi.URLParam(r, "sessionID"),
		ContactID: contactID,
		CourseID:  chi.URLParam(r, "courseID"),
		ModuleID:  chi.URLParam(r, "moduleID"),
	}, user, nil
}

// handleLearnerCourse sert la formation et ses modules à un inscrit.
//
// L'espace apprenant ne peut pas passer par la route du catalogue : elle est
// réservée à l'équipe de l'organisme, et un apprenant y recevrait un 403 sur
// l'écran même de son module. Ce qu'il obtient ici est borné à la session où
// il est inscrit.
func handleLearnerCourse(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Catalog == nil {
			writeError(w, http.StatusServiceUnavailable, "catalogue indisponible")
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

		course, err := deps.Catalog.GetCourse(r.Context(), session.OrgID, target.CourseID)
		if err != nil {
			respondNotFound(w, err, "formation introuvable")
			return
		}
		modules, err := deps.Catalog.ListModules(r.Context(), session.OrgID, course.ID)
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

// handleLearnerSession sert une session et son parcours à un inscrit.
//
// Une session n'a qu'une formation : la faire figurer dans l'adresse était une
// redondance, et trois identifiants à la suite dans une URL sont surtout trois
// occasions de se tromper. L'apprenant navigue désormais par session et par
// numéro de module, et c'est ici que le reste se résout.
func handleLearnerSession(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Catalog == nil {
			writeError(w, http.StatusServiceUnavailable, "catalogue indisponible")
			return
		}

		target, _, err := learnerTarget(deps, r)
		if err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		enrollment, err := deps.Catalog.GetEnrollment(r.Context(), session.OrgID,
			target.SessionID, target.ContactID)
		if err != nil {
			writeError(w, http.StatusForbidden, "aucune inscription à cette session")
			return
		}

		trainingSession, err := deps.Catalog.GetSession(r.Context(), session.OrgID, target.SessionID)
		if err != nil {
			respondNotFound(w, err, "session introuvable")
			return
		}
		course, err := deps.Catalog.GetCourse(r.Context(), session.OrgID, trainingSession.CourseID)
		if err != nil {
			respondNotFound(w, err, "formation introuvable")
			return
		}
		modules, err := deps.Catalog.ListModules(r.Context(), session.OrgID, course.ID)
		if err != nil {
			deps.Log.Error("modules", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"session":    trainingSession,
			"course":     course,
			"modules":    list(modules),
			"enrollment": enrollment,
			"coverUrl":   assetURL(deps, course.CoverKey),
		})
	}
}

// handleHeartbeat intègre un signal du lecteur vidéo.
//
// Appelée toutes les cinq secondes pendant la lecture : c'est la route la plus
// sollicitée du produit, et la seule dont dépend la preuve d'assiduité.
func handleHeartbeat(deps Deps) http.HandlerFunc {
	type request struct {
		FromMs  int64   `json:"fromMs"`
		ToMs    int64   `json:"toMs"`
		Rate    float64 `json:"rate"`
		Focused bool    `json:"focused"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Learning == nil {
			writeError(w, http.StatusServiceUnavailable, "suivi pédagogique indisponible")
			return
		}

		target, _, err := learnerTarget(deps, r)
		if err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		coverage, accepted, err := deps.Learning.Heartbeat(r.Context(), target, lms.Beat{
			FromMs: body.FromMs, ToMs: body.ToMs, Rate: body.Rate, Focused: body.Focused,
		})
		if err != nil && !accepted {
			if errors.Is(err, ddb.ErrNotFound) {
				writeError(w, http.StatusNotFound, "module introuvable")
				return
			}
			// Un signal écarté n'est pas une erreur du client au sens HTTP :
			// le lecteur doit continuer à émettre, et c'est le serveur qui
			// décide de ce qui compte. On répond 200 avec `accepted: false`.
			writeJSON(w, http.StatusOK, map[string]any{
				"accepted": false,
				"reason":   err.Error(),
				"percent":  coverage.Percent(),
			})
			return
		}
		if err != nil {
			deps.Log.Error("progression", "err", err)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"accepted":  true,
			"percent":   coverage.Percent(),
			"watchedMs": coverage.WatchedMs,
			"coveredMs": coverage.CoveredMs(),
		})
	}
}

// handleModuleProgress renvoie l'état d'un module pour un apprenant.
func handleModuleProgress(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Learning == nil {
			writeError(w, http.StatusServiceUnavailable, "suivi pédagogique indisponible")
			return
		}

		target, _, err := learnerTarget(deps, r)
		if err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}

		coverage, err := deps.Learning.Coverage(r.Context(), target)
		if err != nil && !errors.Is(err, ddb.ErrNotFound) {
			deps.Log.Error("couverture", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"percent":   coverage.Percent(),
			"watchedMs": coverage.WatchedMs,
			"coveredMs": coverage.CoveredMs(),
			"lastPosMs": coverage.LastPos,
			"sessions":  coverage.Sessions,
			// Les trous permettent au lecteur de proposer « reprendre où vous
			// vous êtes arrêté » plutôt que de laisser l'apprenant chercher.
			"gaps": coverage.Gaps(),
		})
	}
}

// handleGetQuiz renvoie un questionnaire tel que l'apprenant doit le voir.
func handleGetQuiz(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Learning == nil {
			writeError(w, http.StatusServiceUnavailable, "questionnaires indisponibles")
			return
		}

		target, _, err := learnerTarget(deps, r)
		if err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}

		questionnaire, err := deps.Learning.LatestPublished(r.Context(), session.OrgID, chi.URLParam(r, "quizID"))
		if err != nil {
			respondNotFound(w, err, "questionnaire introuvable")
			return
		}

		number, err := deps.Learning.NextAttemptNumber(r.Context(),
			session.OrgID, target.SessionID+":"+target.ContactID, questionnaire.ID)
		if err != nil {
			deps.Log.Error("passations", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		if questionnaire.MaxAttempts > 0 && number > questionnaire.MaxAttempts {
			writeError(w, http.StatusTooManyRequests, "nombre de tentatives épuisé")
			return
		}

		attempt := quiz.NewAttempt(session.OrgID, target.SessionID+":"+target.ContactID,
			questionnaire, number, deps.Now())

		// La projection retire le corrigé : elle est faite dans le domaine,
		// pas ici, pour qu'aucune route ne puisse l'oublier.
		writeJSON(w, http.StatusOK, map[string]any{
			"questionnaire": questionnaire.ForLearner(attempt.Seed),
			"attempt":       number,
			"maxAttempts":   questionnaire.MaxAttempts,
			"seed":          attempt.Seed,
		})
	}
}

// handleSubmitQuiz corrige une passation.
func handleSubmitQuiz(deps Deps) http.HandlerFunc {
	type request struct {
		Answers []quiz.Submitted `json:"answers"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Learning == nil {
			writeError(w, http.StatusServiceUnavailable, "questionnaires indisponibles")
			return
		}

		target, user, err := learnerTarget(deps, r)
		if err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		questionnaire, err := deps.Learning.LatestPublished(r.Context(), session.OrgID, chi.URLParam(r, "quizID"))
		if err != nil {
			respondNotFound(w, err, "questionnaire introuvable")
			return
		}
		enrollmentID := target.SessionID + ":" + target.ContactID

		number, err := deps.Learning.NextAttemptNumber(r.Context(), session.OrgID, enrollmentID, questionnaire.ID)
		if err != nil {
			deps.Log.Error("passations", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		if questionnaire.MaxAttempts > 0 && number > questionnaire.MaxAttempts {
			writeError(w, http.StatusTooManyRequests, "nombre de tentatives épuisé")
			return
		}

		attempt := quiz.NewAttempt(session.OrgID, enrollmentID, questionnaire, number, deps.Now())
		graded, err := deps.Learning.SubmitQuiz(r.Context(), target, questionnaire, attempt, body.Answers,
			audit.Actor{
				Type: audit.ActorLearner, ID: target.ContactID, Label: user.FullName(),
				IP: clientIP(r), UserAgent: truncateUA(r.UserAgent()),
			})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Le corrigé accompagne la réponse : c'est la vocation d'un contrôle
		// après module, formatif et non sanctionnant.
		writeJSON(w, http.StatusOK, map[string]any{
			"attempt":       graded,
			"percent":       graded.Percent(),
			"passed":        graded.Passed,
			"questionnaire": questionnaire,
		})
	}
}

// handleLearnerDashboard renvoie le parcours d'un apprenant.
func handleLearnerDashboard(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}

		contactID := user.ContactID
		if user.Role.CanTeach() {
			if requested := r.URL.Query().Get("contactId"); requested != "" {
				contactID = requested
			}
		}
		if contactID == "" {
			writeError(w, http.StatusForbidden, "ce compte n'est rattaché à aucune fiche apprenant")
			return
		}

		enrollments, err := deps.Catalog.ListLearnerEnrollments(r.Context(), session.OrgID, contactID)
		if err != nil {
			deps.Log.Error("parcours", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		// Chaque inscription est enrichie de sa formation et de ses modules :
		// l'espace apprenant doit pouvoir s'afficher en un aller-retour.
		type entry struct {
			Enrollment catalog.Enrollment `json:"enrollment"`
			Session    catalog.Session    `json:"session"`
			Course     catalog.Course     `json:"course"`
			Modules    []catalog.Module   `json:"modules"`
			Percent    int                `json:"percent"`
			// CoverURL est résolue ici : le front n'a pas à connaître le nom du
			// compartiment, qui change d'un environnement à l'autre.
			CoverURL string `json:"coverUrl,omitempty"`
			// Done compte les modules achevés. C'est l'unité réelle d'un
			// parcours — aucun module n'est à moitié fait — et c'est elle qu'on
			// affiche à l'apprenant plutôt qu'un pourcentage inventé par
			// division.
			Done int `json:"done"`
		}

		entries := make([]entry, 0, len(enrollments))
		for _, enrollment := range enrollments {
			trainingSession, err := deps.Catalog.GetSession(r.Context(), session.OrgID, enrollment.SessionID)
			if err != nil {
				continue
			}
			course, err := deps.Catalog.GetCourse(r.Context(), session.OrgID, trainingSession.CourseID)
			if err != nil {
				continue
			}
			modules, err := deps.Catalog.ListModules(r.Context(), session.OrgID, course.ID)
			if err != nil {
				continue
			}
			done := 0
			for _, module := range modules {
				for _, progress := range enrollment.Progress {
					if progress.ModuleID == module.ID && progress.CompletedAt != nil {
						done++
					}
				}
			}

			entries = append(entries, entry{
				Enrollment: enrollment, Session: trainingSession, Course: course,
				CoverURL: assetURL(deps, course.CoverKey), Done: done,
				// Une formation sans module existe — elle vient d'être créée —
				// et son parcours doit s'afficher vide plutôt que de faire
				// tomber l'écran de l'apprenant.
				Modules: list(modules), Percent: enrollment.CompletionPercent(modules),
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{"enrollments": list(entries)})
	}
}
