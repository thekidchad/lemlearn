package followup_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/followup"
	"github.com/lemlearn/api/internal/platform/ddb"
)

type sentMail struct{ to, subject, html string }

type fakeMailer struct {
	sent []sentMail
	fail bool
}

func (f *fakeMailer) Send(_ context.Context, to, subject, html string) error {
	if f.fail {
		return context.DeadlineExceeded
	}
	f.sent = append(f.sent, sentMail{to, subject, html})
	return nil
}

// tokenIn extrait le jeton du lien posé dans le courriel.
func tokenIn(t *testing.T, html string) string {
	t.Helper()
	_, after, found := strings.Cut(html, "/satisfaction/")
	if !found {
		t.Fatalf("aucun lien de questionnaire dans le message: %q", html)
	}
	token, _, _ := strings.Cut(after, "\"")
	return token
}

var endsAt = time.Date(2026, 3, 10, 17, 0, 0, 0, time.UTC)

func newService(t *testing.T, now time.Time) (*followup.Service, *fakeMailer) {
	t.Helper()
	mailer := &fakeMailer{}
	clock := now
	return followup.NewService(ddb.NewTestClient(t), mailer, "https://app.lemlearn.fr",
		func() time.Time { return clock }), mailer
}

func schedule(t *testing.T, s *followup.Service) followup.Task {
	t.Helper()
	task, err := s.Schedule(context.Background(), followup.ScheduleInput{
		OrgID: "ORG1", SessionID: "SES1", ContactID: "CON1", FileID: "FIL1",
		QuizID: "QZ1", Email: "camille@exemple.fr", LearnerName: "Camille Roux",
		CourseTitle: "Prévention des risques", EndsAt: endsAt,
	})
	if err != nil {
		t.Fatalf("programmation: %v", err)
	}
	return task
}

// La relance tombe trois mois après la fin de la session, pas après sa
// clôture : c'est la formation qui date, pas la saisie administrative.
func TestScheduledThreeMonthsAfterTheSessionEnds(t *testing.T) {
	s, _ := newService(t, endsAt)
	task := schedule(t, s)

	if want := endsAt.Add(followup.Delay); !task.DueAt.Equal(want) {
		t.Errorf("échéance = %s, attendu %s", task.DueAt, want)
	}
	if task.Status != followup.StatusPlanned {
		t.Errorf("état = %q", task.Status)
	}
}

// Une relance déjà programmée ne se reprogramme pas : clore deux fois une
// session enverrait deux questionnaires à la même personne.
func TestSchedulingTwiceIsRefused(t *testing.T) {
	s, _ := newService(t, endsAt)
	schedule(t, s)

	if _, err := s.Schedule(context.Background(), followup.ScheduleInput{
		OrgID: "ORG1", SessionID: "SES1", ContactID: "CON1", QuizID: "QZ1",
		Email: "camille@exemple.fr", LearnerName: "Camille Roux", EndsAt: endsAt,
	}); err == nil {
		t.Fatal("la seconde programmation a été acceptée")
	}
}

// Sans adresse ni questionnaire, la tâche n'aboutirait jamais : elle n'est pas
// créée, plutôt que d'encombrer le traitement quotidien.
func TestScheduleRefusesWhatItCannotSend(t *testing.T) {
	s, _ := newService(t, endsAt)

	for _, in := range []followup.ScheduleInput{
		{OrgID: "ORG1", SessionID: "S", ContactID: "C", QuizID: "QZ1", EndsAt: endsAt},
		{OrgID: "ORG1", SessionID: "S", ContactID: "C", Email: "a@b.fr", EndsAt: endsAt},
	} {
		if _, err := s.Schedule(context.Background(), in); err == nil {
			t.Errorf("programmation acceptée sans %v", in)
		}
	}
}

// Avant l'échéance, rien ne part.
func TestNothingIsSentBeforeTheDueDate(t *testing.T) {
	s, mailer := newService(t, endsAt)
	task := schedule(t, s)

	sent, failed, err := s.Run(context.Background(), task.DueAt.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if sent != 0 || failed != 0 || len(mailer.sent) != 0 {
		t.Fatalf("%d envoi(s) prématuré(s)", len(mailer.sent))
	}
}

// À l'échéance, la relance part une fois, et une seule : le tour suivant ne la
// reprend pas.
func TestDueTaskIsSentExactlyOnce(t *testing.T) {
	s, mailer := newService(t, endsAt)
	task := schedule(t, s)

	sent, failed, err := s.Run(context.Background(), task.DueAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if sent != 1 || failed != 0 {
		t.Fatalf("envoyées = %d, échecs = %d", sent, failed)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("%d courriel(s)", len(mailer.sent))
	}

	message := mailer.sent[0]
	if message.to != "camille@exemple.fr" {
		t.Errorf("destinataire = %q", message.to)
	}
	if !strings.Contains(message.subject, "Prévention des risques") {
		t.Errorf("objet = %q, attendu l'intitulé de la formation", message.subject)
	}
	// Le lien porte un jeton, pas les identifiants du dossier : l'apprenant
	// répond trois mois après, sans compte à retrouver, et une URL devinable
	// laisserait n'importe qui répondre à sa place.
	token := tokenIn(t, message.html)
	resolved, err := s.Resolve(context.Background(), token)
	if err != nil {
		t.Fatalf("le lien du courriel ne résout pas: %v", err)
	}
	if resolved.ContactID != "CON1" || resolved.QuizID != "QZ1" || resolved.SessionID != "SES1" {
		t.Errorf("jeton résolu vers %+v", resolved)
	}
	if _, err := s.Resolve(context.Background(), "jeton-invente"); err == nil {
		t.Error("un jeton inventé a été accepté")
	}
	// Le prénom seul, pas le nom complet : c'est une relance, pas un courrier.
	if !strings.Contains(message.html, "Bonjour Camille") {
		t.Error("le message n'appelle pas l'apprenant par son prénom")
	}

	sent, _, err = s.Run(context.Background(), task.DueAt.Add(48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if sent != 0 || len(mailer.sent) != 1 {
		t.Fatalf("la relance est repartie une seconde fois (%d courriels)", len(mailer.sent))
	}
}

// Un envoi qui échoue laisse la tâche planifiée : elle repart au tour suivant
// plutôt que de disparaître silencieusement.
func TestFailedSendKeepsTheTaskForTheNextRun(t *testing.T) {
	s, mailer := newService(t, endsAt)
	task := schedule(t, s)
	mailer.fail = true

	sent, failed, err := s.Run(context.Background(), task.DueAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if sent != 0 || failed != 1 {
		t.Fatalf("envoyées = %d, échecs = %d", sent, failed)
	}

	mailer.fail = false
	if sent, _, err = s.Run(context.Background(), task.DueAt.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if sent != 1 {
		t.Fatal("la relance n'a pas été reprise après l'échec")
	}
}

// Une échéance manquée — panne, fonction non déployée — est rattrapée le mois
// suivant, pas perdue.
func TestAMissedDueDateIsCaughtUpTheFollowingMonth(t *testing.T) {
	s, mailer := newService(t, endsAt)
	task := schedule(t, s)

	if _, _, err := s.Run(context.Background(), task.DueAt.AddDate(0, 0, 20)); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("%d courriel(s) : l'échéance du mois précédent n'a pas été rattrapée", len(mailer.sent))
	}
}

// Une relance annulée ne part pas : un apprenant qui abandonne ou qui demande
// l'effacement de ses données n'a pas à recevoir un questionnaire.
func TestCancelledTaskIsNotSent(t *testing.T) {
	s, mailer := newService(t, endsAt)
	task := schedule(t, s)

	if err := s.Cancel(context.Background(), task.DueAt, "SES1", "CON1"); err != nil {
		t.Fatalf("annulation: %v", err)
	}
	if _, _, err := s.Run(context.Background(), task.DueAt.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 0 {
		t.Fatal("une relance annulée est partie")
	}
}

// Une réponse enregistrée se compte : le taux de retour de la satisfaction à
// froid est un indicateur audité, et une relance honorée ne doit pas rester
// « envoyée ».
func TestAnsweringClosesTheTask(t *testing.T) {
	s, mailer := newService(t, endsAt)
	task := schedule(t, s)

	if _, _, err := s.Run(context.Background(), task.DueAt.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	sent, err := s.Resolve(context.Background(), tokenIn(t, mailer.sent[0].html))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Answered(context.Background(), sent); err != nil {
		t.Fatalf("enregistrement de la réponse: %v", err)
	}

	answered, err := s.Resolve(context.Background(), tokenIn(t, mailer.sent[0].html))
	if err != nil {
		t.Fatal(err)
	}
	if answered.Status != followup.StatusAnswered {
		t.Errorf("état = %q après réponse", answered.Status)
	}
}

// Un jeton annulé ne résout plus : un apprenant qui a demandé l'effacement de
// ses données ne doit pas pouvoir être sollicité par un lien encore en boîte.
func TestCancelledTokenNoLongerResolves(t *testing.T) {
	s, mailer := newService(t, endsAt)
	task := schedule(t, s)

	if _, _, err := s.Run(context.Background(), task.DueAt.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	token := tokenIn(t, mailer.sent[0].html)
	if err := s.Cancel(context.Background(), task.DueAt, "SES1", "CON1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(context.Background(), token); err == nil {
		t.Error("un lien annulé résout encore")
	}
}
