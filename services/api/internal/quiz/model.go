// Package quiz porte le moteur de questionnaires : positionnement à l'entrée,
// contrôle après chaque module, évaluation finale, satisfaction à chaud et à
// froid.
//
// Un seul moteur pour tous ces usages, parce qu'ils partagent la même exigence
// probatoire : pouvoir montrer, des années plus tard, ce qui a été demandé, ce
// qui a été répondu, et quand. D'où le versionnement obligatoire — un
// questionnaire modifié ne remplace pas l'ancien, il en crée une nouvelle
// version, et chaque passation reste attachée à la sienne.
package quiz

import (
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// Kind distingue les usages d'un questionnaire.
type Kind string

const (
	// KindPositioning : évaluation d'entrée, obligatoire au titre de Qualiopi.
	KindPositioning Kind = "positioning"
	// KindPostModule : contrôle formatif après une vidéo. Le corrigé et son
	// explication s'affichent après la soumission.
	KindPostModule Kind = "post_module"
	// KindIntermediate : contrôle en cours de parcours.
	KindIntermediate Kind = "intermediate"
	// KindFinal : évaluation de sortie, sanctionnante.
	KindFinal Kind = "final"
	// KindSatisfactionHot : satisfaction en fin de formation.
	KindSatisfactionHot Kind = "satisfaction_hot"
	// KindSatisfactionCold : satisfaction à M+3, programmée automatiquement.
	KindSatisfactionCold Kind = "satisfaction_cold"
)

// Graded indique si le questionnaire produit une note.
//
// Les questionnaires de satisfaction n'ont ni bonne ni mauvaise réponse : leur
// noter un score n'aurait aucun sens et fausserait les indicateurs.
func (k Kind) Graded() bool {
	switch k {
	case KindPositioning, KindPostModule, KindIntermediate, KindFinal:
		return true
	}
	return false
}

// Valid indique si l'usage fait partie de la liste fermée.
func (k Kind) Valid() bool {
	switch k {
	case KindPositioning, KindPostModule, KindIntermediate, KindFinal,
		KindSatisfactionHot, KindSatisfactionCold:
		return true
	}
	return false
}

// QuestionType est la forme d'une question.
type QuestionType string

const (
	TypeSingle   QuestionType = "single"   // choix unique
	TypeMultiple QuestionType = "multiple" // choix multiple
	TypeBoolean  QuestionType = "boolean"  // vrai/faux
	TypeLikert   QuestionType = "likert"   // échelle 1 à 5
	TypeNumeric  QuestionType = "numeric"  // valeur numérique
	TypeText     QuestionType = "text"     // texte libre
)

// Scorable indique si la forme se corrige automatiquement.
func (t QuestionType) Scorable() bool {
	switch t {
	case TypeSingle, TypeMultiple, TypeBoolean, TypeNumeric:
		return true
	}
	return false
}

// Option est un choix proposé.
type Option struct {
	ID    string `dynamodbav:"id" json:"id"`
	Label string `dynamodbav:"label" json:"label"`
}

// Question est une question d'un questionnaire.
type Question struct {
	ID     string       `dynamodbav:"id" json:"id"`
	Type   QuestionType `dynamodbav:"type" json:"type"`
	Prompt string       `dynamodbav:"prompt" json:"prompt"`

	Options []Option `dynamodbav:"options,omitempty" json:"options,omitempty"`
	// Correct liste les identifiants d'option attendus. Jamais renvoyé à
	// l'apprenant avant sa soumission : voir ForLearner.
	Correct []string `dynamodbav:"correct,omitempty" json:"correct,omitempty"`

	// Expected et Tolerance corrigent les questions numériques.
	Expected  float64 `dynamodbav:"expected,omitempty" json:"expected,omitempty"`
	Tolerance float64 `dynamodbav:"tolerance,omitempty" json:"tolerance,omitempty"`

	Points   float64 `dynamodbav:"points" json:"points"`
	Required bool    `dynamodbav:"required" json:"required"`
	// Explanation s'affiche après la soumission. C'est ce qui rend un
	// contrôle après module formatif plutôt que sanctionnant.
	Explanation string `dynamodbav:"explanation,omitempty" json:"explanation,omitempty"`
}

// Validate refuse une question incorrigeable.
func (q Question) Validate() error {
	if strings.TrimSpace(q.Prompt) == "" {
		return fmt.Errorf("question %s : l'énoncé est obligatoire", q.ID)
	}
	switch q.Type {
	case TypeSingle, TypeMultiple, TypeBoolean:
		if len(q.Options) < 2 {
			return fmt.Errorf("question %s : au moins deux options sont nécessaires", q.ID)
		}
		if q.Points > 0 && len(q.Correct) == 0 {
			return fmt.Errorf("question %s : notée mais sans réponse attendue", q.ID)
		}
		known := make(map[string]bool, len(q.Options))
		for _, option := range q.Options {
			known[option.ID] = true
		}
		for _, id := range q.Correct {
			if !known[id] {
				return fmt.Errorf("question %s : la réponse attendue %q ne fait pas partie des options", q.ID, id)
			}
		}
		if q.Type == TypeSingle && len(q.Correct) > 1 {
			return fmt.Errorf("question %s : choix unique avec %d réponses attendues", q.ID, len(q.Correct))
		}
	case TypeNumeric:
		if q.Tolerance < 0 {
			return fmt.Errorf("question %s : tolérance négative", q.ID)
		}
	case TypeLikert, TypeText:
		if q.Points > 0 {
			return fmt.Errorf("question %s : une question non corrigeable ne peut pas être notée", q.ID)
		}
	default:
		return fmt.Errorf("question %s : type %q inconnu", q.ID, q.Type)
	}
	return nil
}

// Questionnaire est une version figée d'un questionnaire.
//
// Une modification ne réécrit jamais une version publiée : elle en crée une
// nouvelle. Les passations en cours continuent sur la leur, et une passation
// archivée reste lisible telle qu'elle a été vécue.
type Questionnaire struct {
	ddb.Record

	ID      string `dynamodbav:"id" json:"id"`
	OrgID   string `dynamodbav:"orgId" json:"orgId"`
	Version int    `dynamodbav:"version" json:"version"`

	Kind  Kind   `dynamodbav:"kind" json:"kind"`
	Title string `dynamodbav:"title" json:"title"`
	// ModuleID rattache un contrôle après module à sa vidéo.
	ModuleID string `dynamodbav:"moduleId,omitempty" json:"moduleId,omitempty"`

	Questions []Question `dynamodbav:"questions" json:"questions"`

	// PassPercent est le seuil de réussite, en pourcentage du barème.
	PassPercent float64 `dynamodbav:"passPercent" json:"passPercent"`
	MaxAttempts int     `dynamodbav:"maxAttempts" json:"maxAttempts"`
	TimeLimitS  int     `dynamodbav:"timeLimitSeconds,omitempty" json:"timeLimitSeconds,omitempty"`

	// ShuffleQuestions et ShuffleOptions mélangent la présentation. Le tirage
	// est déterministe (voir Attempt.Seed) : il reste reproductible pour un
	// auditeur qui veut revoir la copie telle qu'elle a été présentée.
	ShuffleQuestions bool `dynamodbav:"shuffleQuestions" json:"shuffleQuestions"`
	ShuffleOptions   bool `dynamodbav:"shuffleOptions" json:"shuffleOptions"`

	Published   bool       `dynamodbav:"published" json:"published"`
	PublishedAt *time.Time `dynamodbav:"publishedAt,omitempty" json:"publishedAt,omitempty"`
}

// NewQuestionnaire construit la première version d'un questionnaire.
func NewQuestionnaire(orgID string, kind Kind, title string, now time.Time) Questionnaire {
	id := identity.NewID()
	q := Questionnaire{
		Record: ddb.Record{
			PK:        ddb.OrgPK(orgID),
			SK:        QuizSK(id, 1),
			Type:      "quiz",
			CreatedAt: now,
			UpdatedAt: now,
		},
		ID: id, OrgID: orgID, Version: 1,
		Kind: kind, Title: title,
		PassPercent: 70, MaxAttempts: 3,
	}
	q.Reindex()
	return q
}

// NextVersion produit la version suivante d'un questionnaire publié.
func (q Questionnaire) NextVersion(now time.Time) Questionnaire {
	next := q
	next.Version = q.Version + 1
	next.SK = QuizSK(q.ID, next.Version)
	next.Published = false
	next.PublishedAt = nil
	next.CreatedAt = now
	next.UpdatedAt = now
	// Copie profonde des questions : sans elle, modifier la nouvelle version
	// modifierait aussi l'ancienne, et la promesse du versionnement tomberait.
	next.Questions = append([]Question(nil), q.Questions...)
	next.Reindex()
	return next
}

// Reindex recalcule les clés d'index.
func (q *Questionnaire) Reindex() {
	q.GSI1PK = ddb.OrgPK(q.OrgID) + "#QUIZKIND#" + string(q.Kind)
	q.GSI1SK = q.Title
	if q.ModuleID != "" {
		q.GSI2PK = ddb.OrgPK(q.OrgID) + "#QUIZMOD#" + q.ModuleID
		q.GSI2SK = fmt.Sprintf("%06d", q.Version)
	}
}

// QuizSK est la clé de tri d'une version de questionnaire.
func QuizSK(quizID string, version int) string {
	return fmt.Sprintf("QUIZ#%s#V%06d", quizID, version)
}

// MaxScore est le barème total.
func (q Questionnaire) MaxScore() float64 {
	var total float64
	for _, question := range q.Questions {
		total += question.Points
	}
	return total
}

// Validate refuse un questionnaire impubliable.
func (q Questionnaire) Validate() error {
	if !q.Kind.Valid() {
		return fmt.Errorf("usage de questionnaire %q inconnu", q.Kind)
	}
	if strings.TrimSpace(q.Title) == "" {
		return fmt.Errorf("le titre est obligatoire")
	}
	if len(q.Questions) == 0 {
		return fmt.Errorf("un questionnaire sans question ne prouve rien")
	}
	if q.Kind == KindPostModule && q.ModuleID == "" {
		return fmt.Errorf("un contrôle après module doit désigner son module")
	}

	seen := make(map[string]bool, len(q.Questions))
	for _, question := range q.Questions {
		if question.ID == "" {
			return fmt.Errorf("chaque question doit porter un identifiant")
		}
		if seen[question.ID] {
			return fmt.Errorf("identifiant de question en double : %s", question.ID)
		}
		seen[question.ID] = true
		if err := question.Validate(); err != nil {
			return err
		}
	}

	if q.Kind.Graded() && q.MaxScore() == 0 {
		return fmt.Errorf("un questionnaire noté doit porter au moins un point")
	}
	if q.PassPercent < 0 || q.PassPercent > 100 {
		return fmt.Errorf("seuil de réussite hors bornes : %.0f %%", q.PassPercent)
	}
	return nil
}

// ForLearner renvoie la version présentable à l'apprenant : sans les réponses
// attendues ni les explications.
//
// La projection est faite ici plutôt que dans le handler HTTP : c'est le seul
// moyen de garantir qu'aucune route ne renverra le corrigé par inadvertance.
func (q Questionnaire) ForLearner(seed int64) Questionnaire {
	presented := q
	presented.Questions = make([]Question, len(q.Questions))
	for i, question := range q.Questions {
		question.Correct = nil
		question.Explanation = ""
		question.Expected = 0
		question.Tolerance = 0
		question.Options = append([]Option(nil), question.Options...)
		presented.Questions[i] = question
	}
	return Shuffle(presented, seed)
}
