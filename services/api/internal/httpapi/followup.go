package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/followup"
	"github.com/lemlearn/api/internal/learning"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/quiz"
)

// handleCloseSession clôt une session et programme la satisfaction à froid.
//
// Les deux vont ensemble : une clôture qui ne programmerait rien laisserait
// l'indicateur le plus oublié de Qualiopi à la mémoire de quelqu'un.
func handleCloseSession(deps Deps) http.HandlerFunc {
	type scheduled struct {
		ContactID string `json:"contactId"`
		Email     string `json:"email"`
		DueAt     string `json:"dueAt"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Catalog == nil {
			writeError(w, http.StatusServiceUnavailable, "catalogue indisponible")
			return
		}

		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}

		closed, enrollments, err := deps.Catalog.CloseSession(r.Context(), session.OrgID,
			chi.URLParam(r, "sessionID"), actorFrom(r, user.FullName()))
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}

		response := map[string]any{"session": closed, "scheduled": []scheduled{}}

		// La clôture reste acquise même si la relance ne peut pas être
		// programmée : elle est journalisée, et refuser la clôture pour un
		// questionnaire non configuré bloquerait l'organisme sur un détail
		// rattrapable.
		coldQuizID := ""
		if deps.Learning != nil {
			quizzes, err := deps.Learning.PublishedByKind(r.Context(), session.OrgID,
				quiz.KindSatisfactionCold)
			if err == nil && len(quizzes) > 0 {
				coldQuizID = quizzes[0].ID
			}
		}
		if deps.FollowUp == nil || coldQuizID == "" {
			response["warning"] = "aucun questionnaire de satisfaction à froid publié : " +
				"la relance à trois mois n'a pas été programmée"
			writeJSON(w, http.StatusOK, response)
			return
		}

		title := closed.Title
		if deps.Catalog != nil {
			if course, err := deps.Catalog.GetCourse(r.Context(), session.OrgID, closed.CourseID); err == nil {
				title = course.Title
			}
		}

		var planned []scheduled
		var skipped []string
		for _, enrollment := range enrollments {
			contact, err := deps.CRM.GetContact(r.Context(), session.OrgID, enrollment.ContactID)
			if err != nil || contact.Email == "" {
				skipped = append(skipped, enrollment.ContactID)
				continue
			}

			task, err := deps.FollowUp.Schedule(r.Context(), followup.ScheduleInput{
				OrgID: session.OrgID, SessionID: closed.ID, ContactID: contact.ID,
				FileID: enrollment.FileID, QuizID: coldQuizID,
				Email: contact.Email, LearnerName: contact.DisplayName(),
				CourseTitle: title, EndsAt: closed.EndsAt,
			})
			if err != nil {
				skipped = append(skipped, enrollment.ContactID)
				continue
			}
			planned = append(planned, scheduled{
				ContactID: contact.ID, Email: contact.Email,
				DueAt: task.DueAt.Format("2006-01-02"),
			})
		}

		response["scheduled"] = planned
		if len(skipped) > 0 {
			// Nommer qui n'a pas été programmé : un taux de retour calculé sur
			// une liste amputée sans le dire serait un indicateur faux.
			response["skipped"] = skipped
		}
		writeJSON(w, http.StatusOK, response)
	}
}

// handleSurveyOpen ouvre le questionnaire de satisfaction à froid depuis le
// lien reçu par courriel.
//
// La route est publique : trois mois après la formation, exiger un mot de
// passe oublié depuis longtemps ferait tomber le taux de retour à rien — et
// c'est le taux de retour qui est audité. La légitimité vient du jeton, tiré
// sur 256 bits et envoyé à une adresse vérifiée.
func handleSurveyOpen(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		task, questionnaire, ok := resolveSurvey(deps, w, r)
		if !ok {
			return
		}

		attempt := quiz.NewAttempt(task.OrgID, task.SessionID+":"+task.ContactID,
			questionnaire, 1, deps.Now())

		writeJSON(w, http.StatusOK, map[string]any{
			"questionnaire": questionnaire.ForLearner(attempt.Seed),
			"seed":          attempt.Seed,
			"learner":       task.LearnerName,
			"course":        task.CourseTitle,
			"answered":      task.Status == followup.StatusAnswered,
			"brand":         publicBrand(r, deps, task.OrgID),
		})
	}
}

// handleSurveySubmit enregistre les réponses.
func handleSurveySubmit(deps Deps) http.HandlerFunc {
	type request struct {
		Answers []quiz.Submitted `json:"answers"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		task, questionnaire, ok := resolveSurvey(deps, w, r)
		if !ok {
			return
		}
		if task.Status == followup.StatusAnswered {
			// Répondre deux fois fausserait l'indicateur, et le second envoi
			// est presque toujours un double-clic.
			writeError(w, http.StatusConflict, "vous avez déjà répondu à ce questionnaire")
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		course := ""
		if session, err := deps.Catalog.GetSession(r.Context(), task.OrgID, task.SessionID); err == nil {
			course = session.CourseID
		}

		enrollmentID := task.SessionID + ":" + task.ContactID
		number, err := deps.Learning.NextAttemptNumber(r.Context(), task.OrgID, enrollmentID, questionnaire.ID)
		if err != nil {
			deps.Log.Error("passations", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		attempt := quiz.NewAttempt(task.OrgID, enrollmentID, questionnaire, number, deps.Now())
		if _, err := deps.Learning.SubmitQuiz(r.Context(),
			learning.Target{
				OrgID: task.OrgID, SessionID: task.SessionID,
				ContactID: task.ContactID, CourseID: course,
			},
			questionnaire, attempt, body.Answers,
			audit.Actor{
				Type: audit.ActorLearner, ID: task.ContactID, Label: task.LearnerName,
				IP: clientIP(r), UserAgent: truncateUA(r.UserAgent()),
			}); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := deps.FollowUp.Answered(r.Context(), task); err != nil {
			// La réponse est enregistrée ; seul le compteur de retours est en
			// retard, et la relance suivante ne repartira pas puisqu'elle est
			// déjà marquée envoyée.
			deps.Log.Error("clôture de la relance", "err", err)
		}

		writeJSON(w, http.StatusOK, map[string]any{"recorded": true})
	}
}

// resolveSurvey résout le jeton et charge le questionnaire, ou répond.
func resolveSurvey(deps Deps, w http.ResponseWriter, r *http.Request) (followup.Task, quiz.Questionnaire, bool) {
	if deps.FollowUp == nil || deps.Learning == nil || deps.Catalog == nil {
		writeError(w, http.StatusServiceUnavailable, "questionnaires indisponibles")
		return followup.Task{}, quiz.Questionnaire{}, false
	}

	task, err := deps.FollowUp.Resolve(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return followup.Task{}, quiz.Questionnaire{}, false
	}

	questionnaire, err := deps.Learning.LatestPublished(r.Context(), task.OrgID, task.QuizID)
	if err != nil {
		respondNotFound(w, err, "questionnaire introuvable")
		return followup.Task{}, quiz.Questionnaire{}, false
	}
	return task, questionnaire, true
}
