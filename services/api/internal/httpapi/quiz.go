package httpapi

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/quiz"
)

// handleSaveQuiz crée ou met à jour une version de questionnaire.
//
// Une version publiée est refusée en écriture : la modifier reviendrait à
// changer ce qui a été demandé à des apprenants qui l'ont déjà passée. Le
// client doit créer une nouvelle version.
func handleSaveQuiz(deps Deps) http.HandlerFunc {
	type request struct {
		ID               string          `json:"id"`
		Version          int             `json:"version"`
		Kind             quiz.Kind       `json:"kind"`
		Title            string          `json:"title"`
		ModuleID         string          `json:"moduleId"`
		CourseID         string          `json:"courseId"`
		Questions        []quiz.Question `json:"questions"`
		PassPercent      float64         `json:"passPercent"`
		MaxAttempts      int             `json:"maxAttempts"`
		TimeLimitS       int             `json:"timeLimitSeconds"`
		ShuffleQuestions bool            `json:"shuffleQuestions"`
		ShuffleOptions   bool            `json:"shuffleOptions"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Learning == nil {
			writeError(w, http.StatusServiceUnavailable, "questionnaires indisponibles")
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		questionnaire := quiz.NewQuestionnaire(session.OrgID, body.Kind, body.Title, deps.Now())
		if body.ID != "" {
			questionnaire.ID = body.ID
			questionnaire.Version = max(body.Version, 1)
			questionnaire.SK = quiz.QuizSK(body.ID, questionnaire.Version)
		}
		questionnaire.ModuleID = body.ModuleID
		questionnaire.Questions = body.Questions
		if body.PassPercent > 0 {
			questionnaire.PassPercent = body.PassPercent
		}
		if body.MaxAttempts > 0 {
			questionnaire.MaxAttempts = body.MaxAttempts
		}
		questionnaire.TimeLimitS = body.TimeLimitS
		questionnaire.ShuffleQuestions = body.ShuffleQuestions
		questionnaire.ShuffleOptions = body.ShuffleOptions

		saved, err := deps.Learning.SaveQuestionnaire(r.Context(), questionnaire, body.CourseID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, saved)
	}
}

// handlePublishQuiz fige une version.
func handlePublishQuiz(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Learning == nil {
			writeError(w, http.StatusServiceUnavailable, "questionnaires indisponibles")
			return
		}

		version, err := strconv.Atoi(chi.URLParam(r, "version"))
		if err != nil || version < 1 {
			writeError(w, http.StatusBadRequest, "numéro de version invalide")
			return
		}

		published, err := deps.Learning.PublishQuestionnaire(
			r.Context(), session.OrgID, chi.URLParam(r, "quizID"), version)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, published)
	}
}
