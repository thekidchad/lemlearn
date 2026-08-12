package quiz

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// Submitted est une réponse telle que le client la transmet.
type Submitted struct {
	QuestionID string   `json:"questionId"`
	Values     []string `json:"values"`
	// TimeSpentMs et ChangeCount caractérisent la réflexion. Ils ne notent
	// rien, mais distinguent un apprenant qui réfléchit d'un questionnaire
	// cliqué en huit secondes — et cette distinction figure au dossier.
	TimeSpentMs int64     `json:"timeSpentMs"`
	ChangeCount int       `json:"changeCount"`
	AnsweredAt  time.Time `json:"answeredAt"`
}

// Answer est une réponse corrigée, telle qu'elle est conservée.
type Answer struct {
	QuestionID string   `dynamodbav:"questionId" json:"questionId"`
	Values     []string `dynamodbav:"values" json:"values"`

	// Scored distingue « faux » de « non corrigeable » : une réponse de
	// satisfaction n'est pas une mauvaise réponse.
	Scored    bool    `dynamodbav:"scored" json:"scored"`
	IsCorrect bool    `dynamodbav:"isCorrect" json:"isCorrect"`
	Points    float64 `dynamodbav:"points" json:"points"`
	MaxPoints float64 `dynamodbav:"maxPoints" json:"maxPoints"`

	TimeSpentMs int64     `dynamodbav:"timeSpentMs" json:"timeSpentMs"`
	ChangeCount int       `dynamodbav:"changeCount" json:"changeCount"`
	AnsweredAt  time.Time `dynamodbav:"answeredAt,omitempty" json:"answeredAt,omitempty"`
}

// Attempt est une passation.
type Attempt struct {
	ddb.Record

	ID           string `dynamodbav:"id" json:"id"`
	OrgID        string `dynamodbav:"orgId" json:"orgId"`
	EnrollmentID string `dynamodbav:"enrollmentId" json:"enrollmentId"`
	QuizID       string `dynamodbav:"quizId" json:"quizId"`
	// Version fige ce qui a été demandé. Une passation ne suit pas les
	// évolutions ultérieures du questionnaire : c'est ce qui permet de
	// réimprimer la copie telle qu'elle a été passée.
	Version  int    `dynamodbav:"version" json:"version"`
	Number   int    `dynamodbav:"number" json:"number"`
	ModuleID string `dynamodbav:"moduleId,omitempty" json:"moduleId,omitempty"`

	StartedAt   time.Time `dynamodbav:"startedAt" json:"startedAt"`
	SubmittedAt time.Time `dynamodbav:"submittedAt,omitempty" json:"submittedAt,omitempty"`
	DurationMs  int64     `dynamodbav:"durationMs" json:"durationMs"`

	Answers  []Answer `dynamodbav:"answers" json:"answers"`
	Score    float64  `dynamodbav:"score" json:"score"`
	MaxScore float64  `dynamodbav:"maxScore" json:"maxScore"`
	Passed   bool     `dynamodbav:"passed" json:"passed"`

	// Seed rend le mélange reproductible : un auditeur peut réafficher la
	// copie dans l'ordre exact où elle a été présentée.
	Seed int64 `dynamodbav:"seed" json:"seed"`

	// WatchedMsAtStart relie la passation à l'assiduité : un contrôle passé
	// sans avoir regardé la vidéo se voit.
	WatchedMsAtStart int64 `dynamodbav:"watchedMsAtStart" json:"watchedMsAtStart"`

	IP        string `dynamodbav:"ip,omitempty" json:"ip,omitempty"`
	UserAgent string `dynamodbav:"userAgent,omitempty" json:"userAgent,omitempty"`
}

// AttemptSK est la clé de tri d'une passation.
func AttemptSK(enrollmentID, quizID string, number int) string {
	return fmt.Sprintf("ENR#%s#ATT#%s#%03d", enrollmentID, quizID, number)
}

// NewAttempt ouvre une passation.
func NewAttempt(orgID, enrollmentID string, q Questionnaire, number int, now time.Time) Attempt {
	attempt := Attempt{
		Record: ddb.Record{
			PK:        ddb.OrgPK(orgID),
			SK:        AttemptSK(enrollmentID, q.ID, number),
			GSI1PK:    ddb.OrgPK(orgID) + "#QUIZ#" + q.ID,
			GSI1SK:    now.UTC().Format(time.RFC3339Nano),
			Type:      "quiz_attempt",
			CreatedAt: now,
			UpdatedAt: now,
		},
		ID: identity.NewID(), OrgID: orgID, EnrollmentID: enrollmentID,
		QuizID: q.ID, Version: q.Version, Number: number, ModuleID: q.ModuleID,
		StartedAt: now, MaxScore: q.MaxScore(),
	}
	// Graine dérivée de l'inscription, du questionnaire et du rang : deux
	// apprenants voient un ordre différent, et le même apprenant retrouve
	// exactement le sien.
	attempt.Seed = seedFrom(enrollmentID, q.ID, number)
	return attempt
}

func seedFrom(parts ...any) int64 {
	var seed int64 = 1469598103934665603 // FNV-1a 64 bits
	for _, part := range parts {
		for _, b := range []byte(fmt.Sprint(part)) {
			seed ^= int64(b)
			seed *= 1099511628211
		}
	}
	if seed < 0 {
		seed = -seed
	}
	return seed
}

// Grade corrige une passation.
//
// La correction est une fonction pure de la version du questionnaire et des
// réponses : elle ne consulte rien, ne dépend d'aucune horloge, et peut donc
// être rejouée à l'identique des années plus tard sur les données archivées.
func Grade(q Questionnaire, attempt Attempt, submitted []Submitted, at time.Time) (Attempt, error) {
	if attempt.Version != q.Version {
		return attempt, fmt.Errorf(
			"correction avec la version %d alors que la passation porte sur la version %d",
			q.Version, attempt.Version)
	}

	byID := make(map[string]Submitted, len(submitted))
	for _, answer := range submitted {
		byID[answer.QuestionID] = answer
	}

	answers := make([]Answer, 0, len(q.Questions))
	var score float64

	for _, question := range q.Questions {
		given, answered := byID[question.ID]
		if !answered && question.Required && question.Type != TypeText {
			return attempt, fmt.Errorf("la question « %s » est obligatoire", truncate(question.Prompt))
		}

		answer := Answer{
			QuestionID:  question.ID,
			Values:      normalize(given.Values),
			MaxPoints:   question.Points,
			TimeSpentMs: given.TimeSpentMs,
			ChangeCount: given.ChangeCount,
			AnsweredAt:  given.AnsweredAt,
		}
		if answer.AnsweredAt.IsZero() && answered {
			answer.AnsweredAt = at
		}

		if question.Type.Scorable() && question.Points > 0 {
			answer.Scored = true
			answer.IsCorrect = isCorrect(question, answer.Values)
			if answer.IsCorrect {
				answer.Points = question.Points
				score += question.Points
			}
		}
		answers = append(answers, answer)
	}

	attempt.Answers = answers
	attempt.Score = score
	attempt.MaxScore = q.MaxScore()
	attempt.SubmittedAt = at
	attempt.DurationMs = at.Sub(attempt.StartedAt).Milliseconds()
	attempt.UpdatedAt = at

	if q.Kind.Graded() && attempt.MaxScore > 0 {
		attempt.Passed = attempt.Score*100/attempt.MaxScore >= q.PassPercent
	} else {
		// Un questionnaire de satisfaction est « réussi » dès qu'il est
		// rempli : l'indicateur qui compte est le taux de retour.
		attempt.Passed = true
	}
	return attempt, nil
}

// isCorrect applique le barème d'une question.
func isCorrect(question Question, values []string) bool {
	switch question.Type {
	case TypeSingle, TypeBoolean:
		return len(values) == 1 && len(question.Correct) == 1 && values[0] == question.Correct[0]

	case TypeMultiple:
		// Tout ou rien : un barème partiel récompense la sélection de toutes
		// les options, qui garantirait des points sans rien démontrer.
		if len(values) != len(question.Correct) {
			return false
		}
		expected := append([]string(nil), question.Correct...)
		sort.Strings(expected)
		for i := range values {
			if values[i] != expected[i] {
				return false
			}
		}
		return true

	case TypeNumeric:
		if len(values) != 1 {
			return false
		}
		value, err := strconv.ParseFloat(strings.ReplaceAll(values[0], ",", "."), 64)
		if err != nil {
			return false
		}
		return math.Abs(value-question.Expected) <= question.Tolerance
	}
	return false
}

// normalize trie et déduplique les valeurs, pour que l'ordre de sélection
// n'entre pas dans la correction.
func normalize(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// Shuffle réordonne questions et options de façon déterministe.
func Shuffle(q Questionnaire, seed int64) Questionnaire {
	if seed == 0 || (!q.ShuffleQuestions && !q.ShuffleOptions) {
		return q
	}
	source := rand.New(rand.NewPCG(uint64(seed), uint64(seed>>32)))

	if q.ShuffleQuestions {
		questions := append([]Question(nil), q.Questions...)
		source.Shuffle(len(questions), func(i, j int) {
			questions[i], questions[j] = questions[j], questions[i]
		})
		q.Questions = questions
	}
	if q.ShuffleOptions {
		questions := append([]Question(nil), q.Questions...)
		for i := range questions {
			options := append([]Option(nil), questions[i].Options...)
			source.Shuffle(len(options), func(a, b int) {
				options[a], options[b] = options[b], options[a]
			})
			questions[i].Options = options
		}
		q.Questions = questions
	}
	return q
}

// Percent est la note en pourcentage.
func (a Attempt) Percent() int {
	if a.MaxScore == 0 {
		return 0
	}
	return int(a.Score*100/a.MaxScore + 0.5)
}

func truncate(text string) string {
	if len(text) <= 60 {
		return text
	}
	return text[:60] + "…"
}
