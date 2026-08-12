package learning_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/learning"
	"github.com/lemlearn/api/internal/lms"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
	"github.com/lemlearn/api/internal/quiz"
)

const moduleDuration = 600_000 // dix minutes

// clock est une horloge que le test fait avancer lui-même.
//
// Nécessaire parce que le service borne la progression par le temps réellement
// écoulé : des signaux envoyés dans une boucle serrée prétendraient jouer cinq
// secondes de vidéo en trois millisecondes, et seraient refusés — à juste
// titre. Faire avancer l'horloge à la vitesse du visionnage simulé reproduit
// un apprenant réel.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type fixture struct {
	ctx      context.Context
	clock    *clock
	db       *ddb.Client
	catalog  *catalog.Service
	learning *learning.Service

	org      identity.Org
	course   catalog.Course
	module   catalog.Module
	session  catalog.Session
	learner  crm.Contact
	file     crm.File
	postQuiz quiz.Questionnaire
	actor    audit.Actor
}

func (f fixture) target() learning.Target {
	return learning.Target{
		OrgID: f.org.ID, SessionID: f.session.ID, ContactID: f.learner.ID,
		CourseID: f.course.ID, ModuleID: f.module.ID,
	}
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	ctx := context.Background()
	db := ddb.NewTestClient(t)
	tick := &clock{now: time.Date(2026, 2, 3, 18, 0, 0, 0, time.UTC)}

	// Tous les services partagent la même horloge : le journal d'audit refuse
	// les horodatages non croissants, et mélanger l'heure réelle avec celle du
	// test suffirait à casser la chaîne.
	ident := identity.NewService(db, tick.Now)
	crmService := crm.NewService(db, tick.Now)
	catalogService := catalog.NewService(db, tick.Now)
	learningService := learning.NewService(db, catalogService, tick.Now)

	org, owner, err := ident.Register(ctx, identity.RegisterInput{
		OrgName: "Institut Vulcain", Email: "marie@vulcain.fr",
		Password: "correcte-agrafe-cheval-pile", FirstName: "Marie", LastName: "Dubreuil",
	})
	if err != nil {
		t.Fatalf("inscription: %v", err)
	}
	actor := audit.Actor{Type: audit.ActorUser, ID: owner.ID, Label: owner.FullName()}

	now := tick.Now()

	learner := crm.NewContact(org.ID, crm.KindLearner, now)
	learner.FirstName, learner.LastName = "Léa", "Bertrand"
	learner, err = crmService.CreateContact(ctx, learner)
	if err != nil {
		t.Fatalf("apprenant: %v", err)
	}

	file, err := crmService.CreateFile(ctx, crm.CreateFileInput{
		OrgID: org.ID, Title: "SSIAP 1", LearnerID: learner.ID, Actor: actor,
	})
	if err != nil {
		t.Fatalf("dossier: %v", err)
	}

	course := catalog.NewCourse(org.ID, "Sécurité incendie — SSIAP 1", now)
	course.Audience = "Agents de sécurité"
	course.Objectives = []string{"Identifier les composants d'un SSI"}
	course.DurationHours = 14
	course.Published = true
	course, err = catalogService.CreateCourse(ctx, course)
	if err != nil {
		t.Fatalf("formation: %v", err)
	}

	postQuiz := quiz.NewQuestionnaire(org.ID, quiz.KindPostModule, "Contrôle module 1", now)
	postQuiz.PassPercent = 70
	postQuiz.Questions = []quiz.Question{{
		ID: "q1", Type: quiz.TypeSingle, Points: 10,
		Prompt:  "Premier geste à la réception d'une alarme ?",
		Options: []quiz.Option{{ID: "a", Label: "Lever le doute"}, {ID: "b", Label: "Évacuer"}},
		Correct: []string{"a"},
	}}

	module := catalog.NewModule(org.ID, course.ID, "Le SSI et ses composants", 1, now)
	module.DurationMs = moduleDuration
	module.AssetID = "asset-1"
	module.MinCoveragePercent = 80
	module.QuizID = postQuiz.ID
	postQuiz.ModuleID = module.ID
	postQuiz.Reindex()
	if err := ddb.Put(ctx, db, postQuiz); err != nil {
		t.Fatalf("questionnaire: %v", err)
	}
	module, err = catalogService.AddModule(ctx, module)
	if err != nil {
		t.Fatalf("module: %v", err)
	}

	session := catalog.NewSession(org.ID, course.ID, "Session de février",
		catalog.ModeAsync, now.AddDate(0, 0, 7), now.AddDate(0, 0, 14), now)
	session, err = catalogService.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("session: %v", err)
	}

	if _, err := catalogService.Enroll(ctx, catalog.EnrollInput{
		OrgID: org.ID, SessionID: session.ID, ContactID: learner.ID,
		FileID: file.ID, Actor: actor,
	}); err != nil {
		t.Fatalf("inscription à la session: %v", err)
	}

	return fixture{
		ctx: ctx, clock: tick, db: db, catalog: catalogService, learning: learningService,
		org: org, course: course, module: module, session: session,
		learner: learner, file: file, postQuiz: postQuiz, actor: actor,
	}
}

// watch envoie des signaux de cinq secondes jusqu'à la position voulue.
func (f fixture) watch(t *testing.T, toMs int64) {
	t.Helper()
	const step = 5_000
	for pos := int64(0); pos < toMs; pos += step {
		end := min(pos+step, toMs)
		// L'horloge avance de la durée jouée : c'est ce que fait un apprenant
		// qui regarde vraiment.
		f.clock.advance(time.Duration(end-pos) * time.Millisecond)
		if _, _, err := f.learning.Heartbeat(f.ctx, f.target(), lms.Beat{
			FromMs: pos, ToMs: end, Rate: 1, Focused: true,
		}); err != nil {
			t.Fatalf("signal [%d %d]: %v", pos, end, err)
		}
	}
}

func (f fixture) submitQuiz(t *testing.T, answer string) quiz.Attempt {
	t.Helper()
	attempt := quiz.NewAttempt(f.org.ID, f.session.ID+":"+f.learner.ID, f.postQuiz, 1, f.clock.Now())
	graded, err := f.learning.SubmitQuiz(f.ctx, f.target(), f.postQuiz, attempt,
		[]quiz.Submitted{{QuestionID: "q1", Values: []string{answer}, TimeSpentMs: 12_000}},
		audit.Actor{Type: audit.ActorLearner, ID: f.learner.ID})
	if err != nil {
		t.Fatalf("soumission: %v", err)
	}
	return graded
}

func (f fixture) progress(t *testing.T) catalog.ModuleProgress {
	t.Helper()
	enrollment, err := f.catalog.GetEnrollment(f.ctx, f.org.ID, f.session.ID, f.learner.ID)
	if err != nil {
		t.Fatalf("inscription: %v", err)
	}
	for _, p := range enrollment.Progress {
		if p.ModuleID == f.module.ID {
			return p
		}
	}
	return catalog.ModuleProgress{}
}

// La règle centrale : assiduité suffisante ET contrôle réussi.
func TestModuleCompletesWhenBothConditionsAreMet(t *testing.T) {
	f := newFixture(t)

	f.watch(t, moduleDuration)
	f.submitQuiz(t, "a")

	progress := f.progress(t)
	if progress.CoveragePercent != 100 {
		t.Errorf("couverture = %d %%", progress.CoveragePercent)
	}
	if !progress.QuizPassed {
		t.Error("le contrôle n'est pas marqué réussi")
	}
	if progress.CompletedAt == nil {
		t.Fatal("le module n'est pas validé alors que les deux conditions sont réunies")
	}
}

// Assiduité insuffisante : le contrôle réussi ne suffit pas. C'est exactement
// ce qu'un auditeur vérifie — un apprenant ne valide pas un module en
// répondant juste sans avoir suivi la formation.
func TestQuizPassedWithoutAttendanceDoesNotComplete(t *testing.T) {
	f := newFixture(t)

	f.watch(t, moduleDuration/2) // 50 %, sous le seuil de 80 %
	f.submitQuiz(t, "a")

	progress := f.progress(t)
	if progress.CoveragePercent != 50 {
		t.Errorf("couverture = %d %%, attendu 50 %%", progress.CoveragePercent)
	}
	if !progress.QuizPassed {
		t.Error("le contrôle devrait être réussi")
	}
	if progress.CompletedAt != nil {
		t.Fatal("module validé malgré une assiduité de 50 %")
	}
}

// L'inverse : assiduité complète mais contrôle raté.
func TestAttendanceWithoutPassingQuizDoesNotComplete(t *testing.T) {
	f := newFixture(t)

	f.watch(t, moduleDuration)
	graded := f.submitQuiz(t, "b")

	if graded.Passed {
		t.Fatal("une mauvaise réponse est déclarée réussie")
	}
	if progress := f.progress(t); progress.CompletedAt != nil {
		t.Fatal("module validé malgré un contrôle raté")
	}
}

// Une réussite acquise ne se perd pas : repasser un contrôle déjà réussi et le
// rater ne doit pas rouvrir un module validé.
func TestPassedQuizIsNotLostOnRetry(t *testing.T) {
	f := newFixture(t)

	f.watch(t, moduleDuration)
	f.submitQuiz(t, "a")
	completedAt := f.progress(t).CompletedAt
	if completedAt == nil {
		t.Fatal("le module aurait dû être validé")
	}

	// Deuxième passation, ratée.
	f.clock.advance(time.Minute)
	attempt := quiz.NewAttempt(f.org.ID, f.session.ID+":"+f.learner.ID, f.postQuiz, 2, f.clock.Now())
	if _, err := f.learning.SubmitQuiz(f.ctx, f.target(), f.postQuiz, attempt,
		[]quiz.Submitted{{QuestionID: "q1", Values: []string{"b"}}},
		audit.Actor{Type: audit.ActorLearner, ID: f.learner.ID}); err != nil {
		t.Fatalf("seconde soumission: %v", err)
	}

	progress := f.progress(t)
	if !progress.QuizPassed {
		t.Error("la réussite acquise a été perdue")
	}
	if progress.CompletedAt == nil || !progress.CompletedAt.Equal(*completedAt) {
		t.Error("la date de validation a bougé")
	}
}

// La couverture doit survivre entre deux séances : un apprenant qui reprend
// le lendemain repart d'où il s'était arrêté.
func TestCoveragePersistsAcrossSessions(t *testing.T) {
	f := newFixture(t)

	f.watch(t, 300_000)
	if got := f.progress(t).CoveragePercent; got != 50 {
		t.Fatalf("après la première séance : %d %%", got)
	}

	// Reprise : les signaux repartent de la position atteinte.
	const step = 5_000
	for pos := int64(300_000); pos < moduleDuration; pos += step {
		f.clock.advance(step * time.Millisecond)
		if _, _, err := f.learning.Heartbeat(f.ctx, f.target(), lms.Beat{
			FromMs: pos, ToMs: pos + step, Rate: 1, Focused: true,
		}); err != nil {
			t.Fatalf("reprise: %v", err)
		}
	}

	if got := f.progress(t).CoveragePercent; got != 100 {
		t.Errorf("après reprise : %d %%, attendu 100 %%", got)
	}
}

// Un signal impossible doit être écarté, et laisser une trace.
func TestImpossibleBeatIsRejectedAndCounted(t *testing.T) {
	f := newFixture(t)

	f.watch(t, 10_000)
	// Dix secondes de temps réel pour prétendre en avoir joué 580.
	f.clock.advance(10 * time.Second)
	_, accepted, err := f.learning.Heartbeat(f.ctx, f.target(), lms.Beat{
		FromMs: 10_000, ToMs: 590_000, Rate: 1, Focused: true,
	})
	if accepted {
		t.Fatal("un signal impossible a été accepté")
	}
	if err == nil || !strings.Contains(err.Error(), "écarté") {
		t.Errorf("erreur peu explicite: %v", err)
	}

	coverage, err := f.learning.Coverage(f.ctx, f.target())
	if err != nil {
		t.Fatalf("relecture: %v", err)
	}
	if coverage.Rejected != 1 {
		t.Errorf("compteur de refus = %d, attendu 1", coverage.Rejected)
	}
	if coverage.Percent() > 10 {
		t.Errorf("le signal refusé a compté : %d %%", coverage.Percent())
	}
}

// Tout doit arriver au journal du dossier, dans une chaîne vérifiable.
func TestProgressIsAudited(t *testing.T) {
	f := newFixture(t)

	f.watch(t, moduleDuration)
	f.submitQuiz(t, "a")

	events, err := f.db.AuditChain(f.ctx, "file/"+f.file.ID)
	if err != nil {
		t.Fatalf("journal: %v", err)
	}

	seen := map[audit.Action]int{}
	for _, event := range events {
		seen[event.Action]++
	}
	if seen[audit.ActionQuizSubmitted] != 1 {
		t.Errorf("%d soumission(s) de questionnaire au journal", seen[audit.ActionQuizSubmitted])
	}
	if seen[audit.ActionModuleCompleted] != 1 {
		t.Errorf("%d validation(s) de module au journal", seen[audit.ActionModuleCompleted])
	}
	if err := audit.Verify(events); err != nil {
		t.Fatalf("chaîne invalide: %v", err)
	}
}

// L'attestation ne peut pas être délivrée tant que tout n'est pas réuni.
func TestCertifiableRequiresEverything(t *testing.T) {
	f := newFixture(t)

	course := f.course
	course.FinalQuizID = "FINAL"
	modules := []catalog.Module{f.module}

	enrollment, err := f.catalog.GetEnrollment(f.ctx, f.org.ID, f.session.ID, f.learner.ID)
	if err != nil {
		t.Fatalf("inscription: %v", err)
	}
	if err := enrollment.Certifiable(modules, course); err == nil {
		t.Fatal("attestation délivrable sans aucun module validé")
	}

	f.watch(t, moduleDuration)
	f.submitQuiz(t, "a")

	enrollment, _ = f.catalog.GetEnrollment(f.ctx, f.org.ID, f.session.ID, f.learner.ID)
	err = enrollment.Certifiable(modules, course)
	if err == nil {
		t.Fatal("attestation délivrable sans évaluation finale")
	}
	if !strings.Contains(err.Error(), "finale") {
		t.Errorf("motif inattendu: %v", err)
	}
}

// Une session synchrone sans lieu ni lien ne peut pas figurer sur une
// convocation.
func TestSessionValidation(t *testing.T) {
	f := newFixture(t)
	now := f.clock.Now()

	invalid := catalog.NewSession(f.org.ID, f.course.ID, "Classe virtuelle",
		catalog.ModeVirtual, now, now.Add(3*time.Hour), now)
	if _, err := f.catalog.CreateSession(f.ctx, invalid); err == nil {
		t.Error("une classe virtuelle sans lien a été acceptée")
	}

	backwards := catalog.NewSession(f.org.ID, f.course.ID, "À l'envers",
		catalog.ModeAsync, now.Add(3*time.Hour), now, now)
	if _, err := f.catalog.CreateSession(f.ctx, backwards); err == nil {
		t.Error("une session qui finit avant de commencer a été acceptée")
	}
}
