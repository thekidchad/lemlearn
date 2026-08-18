package httpapi

import (
	"net/http"
	"sort"
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

// handleListQuizzes renvoie les questionnaires de l'organisation.
func handleListQuizzes(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Learning == nil {
			writeError(w, http.StatusServiceUnavailable, "questionnaires indisponibles")
			return
		}

		quizzes, err := deps.Learning.ListQuestionnaires(r.Context(), session.OrgID)
		if err != nil {
			deps.Log.Error("liste des questionnaires", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quizzes": list(quizzes)})
	}
}

// handleGetQuizVersions renvoie toutes les versions d'un questionnaire.
func handleGetQuizVersions(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Learning == nil {
			writeError(w, http.StatusServiceUnavailable, "questionnaires indisponibles")
			return
		}

		versions, err := deps.Learning.Versions(r.Context(), session.OrgID, chi.URLParam(r, "quizID"))
		if err != nil {
			deps.Log.Error("versions du questionnaire", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		if len(versions) == 0 {
			writeError(w, http.StatusNotFound, "questionnaire introuvable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"versions": list(versions)})
	}
}

// handleQuizResults agrège les passations d'un questionnaire.
//
// Deux lectures, parce qu'elles répondent à deux questions différentes : par
// apprenant, « qui a réussi » ; par question, « qu'est-ce qui n'est pas
// passé ». La seconde est celle qui améliore une formation — un distracteur
// que personne ne choisit ou une question que tout le monde rate en dit plus
// sur le cours que sur les apprenants.
func handleQuizResults(deps Deps) http.HandlerFunc {
	type perQuestion struct {
		QuestionID string         `json:"questionId"`
		Prompt     string         `json:"prompt"`
		Answered   int            `json:"answered"`
		Correct    int            `json:"correct"`
		Choices    map[string]int `json:"choices,omitempty"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Learning == nil {
			writeError(w, http.StatusServiceUnavailable, "questionnaires indisponibles")
			return
		}

		quizID := chi.URLParam(r, "quizID")
		attempts, err := deps.Learning.AttemptsFor(r.Context(), session.OrgID, quizID)
		if err != nil {
			deps.Log.Error("passations", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		versions, err := deps.Learning.Versions(r.Context(), session.OrgID, quizID)
		if err != nil || len(versions) == 0 {
			respondNotFound(w, err, "questionnaire introuvable")
			return
		}

		// Les énoncés viennent de la dernière version : un intitulé reformulé
		// depuis se lit mieux qu'un identifiant, et la copie d'origine reste
		// consultable par ailleurs.
		prompts := make(map[string]string)
		for _, question := range versions[0].Questions {
			prompts[question.ID] = question.Prompt
		}

		stats := make(map[string]*perQuestion)
		passed, total := 0, 0
		var durations int64
		for _, attempt := range attempts {
			if attempt.SubmittedAt.IsZero() {
				continue
			}
			total++
			durations += attempt.DurationMs
			if attempt.Passed {
				passed++
			}
			for _, answer := range attempt.Answers {
				line, ok := stats[answer.QuestionID]
				if !ok {
					line = &perQuestion{
						QuestionID: answer.QuestionID,
						Prompt:     prompts[answer.QuestionID],
						Choices:    make(map[string]int),
					}
					stats[answer.QuestionID] = line
				}
				line.Answered++
				if answer.IsCorrect {
					line.Correct++
				}
				for _, value := range answer.Values {
					line.Choices[value]++
				}
			}
		}

		questions := make([]perQuestion, 0, len(stats))
		for _, line := range stats {
			questions = append(questions, *line)
		}
		// Les questions les moins réussies d'abord : c'est ce qu'on vient
		// regarder.
		sort.Slice(questions, func(i, j int) bool {
			return successRate(questions[i].Correct, questions[i].Answered) <
				successRate(questions[j].Correct, questions[j].Answered)
		})

		average := int64(0)
		if total > 0 {
			average = durations / int64(total)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"quiz":              versions[0],
			"attempts":          attempts,
			"submitted":         total,
			"passed":            passed,
			"averageDurationMs": average,
			"questions":         questions,
		})
	}
}

func successRate(correct, answered int) float64 {
	if answered == 0 {
		return 1
	}
	return float64(correct) / float64(answered)
}
