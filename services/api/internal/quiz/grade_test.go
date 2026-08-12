package quiz_test

import (
	"strings"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/quiz"
)

var now = time.Date(2026, 2, 3, 19, 5, 0, 0, time.UTC)

func ssiap() quiz.Questionnaire {
	q := quiz.NewQuestionnaire("ORG1", quiz.KindPostModule, "SSIAP 1 — module 2", now)
	q.ModuleID = "MOD2"
	q.PassPercent = 70
	q.Questions = []quiz.Question{
		{
			ID: "q1", Type: quiz.TypeSingle, Points: 4,
			Prompt: "Quel est le premier geste à la réception d'une alarme feu ?",
			Options: []quiz.Option{
				{ID: "a", Label: "Déclencher l'alarme générale"},
				{ID: "b", Label: "Prévenir les secours extérieurs"},
				{ID: "c", Label: "Lever le doute"},
			},
			Correct:     []string{"c"},
			Explanation: "La levée de doute précède toute autre action.",
		},
		{
			ID: "q2", Type: quiz.TypeMultiple, Points: 6,
			Prompt: "Quels éléments composent un SSI ?",
			Options: []quiz.Option{
				{ID: "a", Label: "Système de détection incendie"},
				{ID: "b", Label: "Système de mise en sécurité"},
				{ID: "c", Label: "Groupe électrogène"},
			},
			Correct: []string{"a", "b"},
		},
		{
			ID: "q3", Type: quiz.TypeNumeric, Points: 4,
			Prompt:   "Durée maximale de la temporisation, en secondes ?",
			Expected: 300, Tolerance: 30,
		},
		{
			ID: "q4", Type: quiz.TypeLikert, Points: 0,
			Prompt:  "Ce module vous a-t-il paru clair ?",
			Options: []quiz.Option{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "5"}},
		},
	}
	return q
}

func attemptFor(q quiz.Questionnaire) quiz.Attempt {
	return quiz.NewAttempt("ORG1", "ENR9", q, 1, now)
}

func TestPerfectScorePasses(t *testing.T) {
	q := ssiap()
	graded, err := quiz.Grade(q, attemptFor(q), []quiz.Submitted{
		{QuestionID: "q1", Values: []string{"c"}, TimeSpentMs: 14_000, ChangeCount: 1},
		{QuestionID: "q2", Values: []string{"b", "a"}, TimeSpentMs: 22_000},
		{QuestionID: "q3", Values: []string{"290"}, TimeSpentMs: 9_000},
		{QuestionID: "q4", Values: []string{"5"}},
	}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("correction: %v", err)
	}

	if graded.Score != 14 || graded.MaxScore != 14 {
		t.Errorf("note = %.0f/%.0f, attendu 14/14", graded.Score, graded.MaxScore)
	}
	if !graded.Passed {
		t.Error("un sans-faute est déclaré échoué")
	}
	if graded.Percent() != 100 {
		t.Errorf("pourcentage = %d", graded.Percent())
	}
	if graded.DurationMs != 120_000 {
		t.Errorf("durée = %d ms, attendue 120 000", graded.DurationMs)
	}
}

// L'ordre de sélection d'un choix multiple ne doit pas entrer dans la
// correction : l'apprenant qui coche b puis a a la même réponse.
func TestMultipleChoiceIgnoresSelectionOrder(t *testing.T) {
	q := ssiap()
	for _, values := range [][]string{{"a", "b"}, {"b", "a"}, {"b", "a", "b"}} {
		graded, err := quiz.Grade(q, attemptFor(q), []quiz.Submitted{
			{QuestionID: "q2", Values: values},
		}, now)
		if err != nil {
			t.Fatalf("correction: %v", err)
		}
		if graded.Score != 6 {
			t.Errorf("sélection %v notée %.0f, attendu 6", values, graded.Score)
		}
	}
}

// Tout ou rien : cocher toutes les options ne doit rien rapporter, sinon
// c'est la stratégie gagnante et le questionnaire ne prouve plus rien.
func TestSelectingEverythingScoresZero(t *testing.T) {
	q := ssiap()
	graded, err := quiz.Grade(q, attemptFor(q), []quiz.Submitted{
		{QuestionID: "q2", Values: []string{"a", "b", "c"}},
	}, now)
	if err != nil {
		t.Fatalf("correction: %v", err)
	}
	if graded.Score != 0 {
		t.Errorf("cocher tout rapporte %.0f point(s)", graded.Score)
	}
}

func TestNumericToleranceIsApplied(t *testing.T) {
	q := ssiap()
	// La virgule décimale doit être acceptée : c'est ainsi qu'un apprenant
	// français saisit un nombre. 271,5 est à 28,5 de 300, donc dans la
	// tolérance ; 269,5 est à 30,5, donc dehors.
	cases := map[string]bool{
		"300": true, "290": true, "330": true,
		"271,5": true, "269,5": false, "331": false, "0": false, "abc": false,
	}
	for value, want := range cases {
		graded, err := quiz.Grade(q, attemptFor(q), []quiz.Submitted{
			{QuestionID: "q3", Values: []string{value}},
		}, now)
		if err != nil {
			t.Fatalf("correction: %v", err)
		}
		if got := graded.Score > 0; got != want {
			t.Errorf("valeur %q : correcte=%v, attendu %v", value, got, want)
		}
	}
}

// Une question de satisfaction n'est ni juste ni fausse.
func TestUnscorableQuestionIsNotCountedWrong(t *testing.T) {
	q := ssiap()
	graded, err := quiz.Grade(q, attemptFor(q), []quiz.Submitted{
		{QuestionID: "q4", Values: []string{"2"}},
	}, now)
	if err != nil {
		t.Fatalf("correction: %v", err)
	}
	for _, answer := range graded.Answers {
		if answer.QuestionID == "q4" {
			if answer.Scored {
				t.Error("une question d'appréciation a été notée")
			}
			if answer.IsCorrect {
				t.Error("une question d'appréciation est déclarée correcte")
			}
		}
	}
}

// Le temps de réflexion et les changements d'avis doivent être conservés :
// c'est ce qui distingue une passation sérieuse d'un questionnaire cliqué.
func TestReflectionMetadataIsKept(t *testing.T) {
	q := ssiap()
	graded, err := quiz.Grade(q, attemptFor(q), []quiz.Submitted{
		{QuestionID: "q1", Values: []string{"a"}, TimeSpentMs: 14_000, ChangeCount: 3},
	}, now)
	if err != nil {
		t.Fatalf("correction: %v", err)
	}

	var found bool
	for _, answer := range graded.Answers {
		if answer.QuestionID == "q1" {
			found = true
			if answer.TimeSpentMs != 14_000 || answer.ChangeCount != 3 {
				t.Errorf("métadonnées perdues: %+v", answer)
			}
			if answer.IsCorrect {
				t.Error("une mauvaise réponse est déclarée correcte")
			}
		}
	}
	if !found {
		t.Fatal("la réponse n'a pas été conservée")
	}
}

// Une question obligatoire non répondue doit bloquer la soumission.
func TestRequiredQuestionBlocksSubmission(t *testing.T) {
	q := ssiap()
	q.Questions[0].Required = true

	if _, err := quiz.Grade(q, attemptFor(q), []quiz.Submitted{
		{QuestionID: "q2", Values: []string{"a", "b"}},
	}, now); err == nil {
		t.Fatal("une question obligatoire non répondue a été acceptée")
	}
}

// Le seuil de réussite doit être appliqué au barème, pas au nombre de
// questions.
func TestPassThresholdUsesPoints(t *testing.T) {
	q := ssiap()
	// 6 points sur 14 = 43 %, sous le seuil de 70 %, alors même que la
	// question la mieux notée est juste.
	graded, err := quiz.Grade(q, attemptFor(q), []quiz.Submitted{
		{QuestionID: "q1", Values: []string{"a"}},
		{QuestionID: "q2", Values: []string{"a", "b"}},
		{QuestionID: "q3", Values: []string{"10"}},
	}, now)
	if err != nil {
		t.Fatalf("correction: %v", err)
	}
	if graded.Passed {
		t.Errorf("6/14 (%d %%) déclaré réussi avec un seuil à 70 %%", graded.Percent())
	}
}

// Une passation corrigée avec une autre version que la sienne doit échouer :
// c'est la garantie du versionnement.
func TestGradingAgainstWrongVersionFails(t *testing.T) {
	q := ssiap()
	attempt := attemptFor(q)

	v2 := q.NextVersion(now)
	v2.Questions[0].Correct = []string{"a"} // le corrigé a changé

	if _, err := quiz.Grade(v2, attempt, []quiz.Submitted{
		{QuestionID: "q1", Values: []string{"c"}},
	}, now); err == nil {
		t.Fatal("une passation a été corrigée avec une version qui n'est pas la sienne")
	}
}

// Une nouvelle version ne doit pas modifier l'ancienne.
func TestNextVersionIsIndependent(t *testing.T) {
	v1 := ssiap()
	v2 := v1.NextVersion(now)
	v2.Questions[0].Prompt = "Énoncé réécrit"
	v2.Questions[0].Correct = []string{"a"}

	if v1.Questions[0].Prompt == "Énoncé réécrit" {
		t.Error("modifier la version 2 a modifié la version 1")
	}
	if v1.Questions[0].Correct[0] != "c" {
		t.Error("le corrigé de la version 1 a changé")
	}
	if v2.Version != 2 || v2.Published {
		t.Errorf("version 2 mal initialisée : v=%d publiée=%v", v2.Version, v2.Published)
	}
}

// Le corrigé ne doit jamais partir vers l'apprenant avant sa soumission.
func TestLearnerViewHidesAnswers(t *testing.T) {
	q := ssiap()
	presented := q.ForLearner(12345)

	for _, question := range presented.Questions {
		if len(question.Correct) > 0 {
			t.Errorf("la question %s expose son corrigé", question.ID)
		}
		if question.Explanation != "" {
			t.Errorf("la question %s expose son explication", question.ID)
		}
		if question.Expected != 0 {
			t.Errorf("la question %s expose la valeur attendue", question.ID)
		}
	}
	// Et la version d'origine ne doit pas avoir été vidée au passage.
	if len(q.Questions[0].Correct) == 0 {
		t.Fatal("la projection a effacé le corrigé de la version d'origine")
	}
}

// Le mélange doit être reproductible : un auditeur doit pouvoir réafficher la
// copie dans l'ordre exact où elle a été présentée.
func TestShuffleIsDeterministic(t *testing.T) {
	q := ssiap()
	q.ShuffleQuestions = true
	q.ShuffleOptions = true

	first := quiz.Shuffle(q, 987654321)
	second := quiz.Shuffle(q, 987654321)
	other := quiz.Shuffle(q, 123456789)

	order := func(x quiz.Questionnaire) string {
		var ids []string
		for _, question := range x.Questions {
			ids = append(ids, question.ID)
		}
		return strings.Join(ids, ",")
	}

	if order(first) != order(second) {
		t.Errorf("même graine, ordres différents : %s puis %s", order(first), order(second))
	}
	if order(first) == order(other) && order(first) == "q1,q2,q3,q4" {
		t.Error("le mélange semble sans effet")
	}
}

func TestValidateRejectsIncoherentQuestionnaire(t *testing.T) {
	cases := map[string]func(*quiz.Questionnaire){
		"sans question":              func(q *quiz.Questionnaire) { q.Questions = nil },
		"notée mais sans corrigé":    func(q *quiz.Questionnaire) { q.Questions[0].Correct = nil },
		"corrigé hors des options":   func(q *quiz.Questionnaire) { q.Questions[0].Correct = []string{"z"} },
		"choix unique à deux vraies": func(q *quiz.Questionnaire) { q.Questions[0].Correct = []string{"a", "b"} },
		"appréciation notée":         func(q *quiz.Questionnaire) { q.Questions[3].Points = 2 },
		"identifiants en double":     func(q *quiz.Questionnaire) { q.Questions[1].ID = "q1" },
		"seuil hors bornes":          func(q *quiz.Questionnaire) { q.PassPercent = 140 },
		"module manquant":            func(q *quiz.Questionnaire) { q.ModuleID = "" },
	}

	for name, breakIt := range cases {
		q := ssiap()
		breakIt(&q)
		if err := q.Validate(); err == nil {
			t.Errorf("cas « %s » accepté à tort", name)
		}
	}

	if err := ssiap().Validate(); err != nil {
		t.Errorf("un questionnaire correct est refusé: %v", err)
	}
}
