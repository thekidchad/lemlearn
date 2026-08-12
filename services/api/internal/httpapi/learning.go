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
			entries = append(entries, entry{
				Enrollment: enrollment, Session: trainingSession, Course: course,
				Modules: modules, Percent: enrollment.CompletionPercent(modules),
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{"enrollments": entries})
	}
}
