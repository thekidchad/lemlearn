package attendance_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/attendance"
	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

type fixture struct {
	ctx        context.Context
	db         *ddb.Client
	catalog    *catalog.Service
	attendance *attendance.Service
	org        identity.Org
	owner      identity.User
	actor      audit.Actor
	course     catalog.Course
}

func newFixture(t *testing.T, mode catalog.Mode) (fixture, catalog.Session) {
	t.Helper()

	ctx := context.Background()
	db := ddb.NewTestClient(t)
	ident := identity.NewService(db, nil)
	catalogService := catalog.NewService(db, nil)
	attendanceService := attendance.NewService(db, catalogService, nil)

	org, owner, err := ident.Register(ctx, identity.RegisterInput{
		OrgName: "Institut Vulcain", Email: "marie@vulcain.fr",
		Password: "correcte-agrafe-cheval-pile", FirstName: "Marie", LastName: "Dubreuil",
	})
	if err != nil {
		t.Fatalf("inscription: %v", err)
	}

	now := time.Now().UTC()
	course := catalog.NewCourse(org.ID, "SSIAP 1", now)
	course.Audience = "Agents"
	course.Objectives = []string{"o"}
	course.DurationHours = 14
	course.Published = true
	course, err = catalogService.CreateCourse(ctx, course)
	if err != nil {
		t.Fatalf("formation: %v", err)
	}

	loc, _ := time.LoadLocation("Europe/Paris")
	start := time.Date(2026, 2, 3, 9, 0, 0, 0, loc)
	end := time.Date(2026, 2, 4, 17, 0, 0, 0, loc)

	session := catalog.NewSession(org.ID, course.ID, "Session de février", mode, start, end, now)
	session.Location = "12 rue des Écoles, Paris"
	if mode == catalog.ModeAsync {
		for i := 1; i <= 3; i++ {
			module := catalog.NewModule(org.ID, course.ID, "Module "+string(rune('0'+i)), i, now)
			module.DurationMs = 600_000
			module.AssetID = "a"
			if _, err := catalogService.AddModule(ctx, module); err != nil {
				t.Fatalf("module: %v", err)
			}
		}
	}
	session, err = catalogService.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("session: %v", err)
	}

	return fixture{
		ctx: ctx, db: db, catalog: catalogService, attendance: attendanceService,
		org: org, owner: owner, course: course,
		actor: audit.Actor{Type: audit.ActorUser, ID: owner.ID, Label: owner.FullName()},
	}, session
}

// Une session en présentiel se découpe en demi-journées : c'est l'unité que
// recompte un contrôleur.
func TestOnsiteSheetIsSplitIntoHalfDays(t *testing.T) {
	f, session := newFixture(t, catalog.ModeOnsite)

	sheet, err := f.attendance.EnsureSheet(f.ctx, f.org.ID, session.ID)
	if err != nil {
		t.Fatalf("feuille: %v", err)
	}

	// Deux jours × deux demi-journées, moins le créneau du premier matin qui
	// commence à l'heure de début.
	if len(sheet.Slots) != 4 {
		var labels []string
		for _, slot := range sheet.Slots {
			labels = append(labels, slot.Label)
		}
		t.Fatalf("%d créneau(x) : %v", len(sheet.Slots), labels)
	}
	if !strings.Contains(sheet.Slots[0].Label, "matin") {
		t.Errorf("premier créneau = %q", sheet.Slots[0].Label)
	}
	if hours := sheet.Slots[0].Hours(); hours != 4 {
		t.Errorf("durée du matin = %.0f h, attendu 4", hours)
	}
}

// Une session asynchrone s'émarge module par module.
func TestAsyncSheetHasOneSlotPerModule(t *testing.T) {
	f, session := newFixture(t, catalog.ModeAsync)

	sheet, err := f.attendance.EnsureSheet(f.ctx, f.org.ID, session.ID)
	if err != nil {
		t.Fatalf("feuille: %v", err)
	}
	if len(sheet.Slots) != 3 {
		t.Fatalf("%d créneau(x) pour 3 modules", len(sheet.Slots))
	}
	for _, slot := range sheet.Slots {
		if slot.ModuleID == "" {
			t.Errorf("le créneau %q ne désigne aucun module", slot.Label)
		}
	}
}

// La feuille est stable : deux appels ne doivent pas produire deux découpages.
func TestSheetIsIdempotent(t *testing.T) {
	f, session := newFixture(t, catalog.ModeOnsite)

	first, err := f.attendance.EnsureSheet(f.ctx, f.org.ID, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.attendance.EnsureSheet(f.ctx, f.org.ID, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Slots) != len(second.Slots) || first.Slots[0].ID != second.Slots[0].ID {
		t.Error("le découpage a changé entre deux appels")
	}
}

func TestSignAndCount(t *testing.T) {
	f, session := newFixture(t, catalog.ModeOnsite)
	sheet, _ := f.attendance.EnsureSheet(f.ctx, f.org.ID, session.ID)

	for _, slot := range sheet.Slots[:3] {
		if _, err := f.attendance.Sign(f.ctx, attendance.SignInput{
			OrgID: f.org.ID, SessionID: session.ID, SlotID: slot.ID,
			ContactID: "LEA", Method: attendance.MethodSignature,
			IP: "78.192.44.10", Actor: f.actor,
		}); err != nil {
			t.Fatalf("émargement de %s: %v", slot.Label, err)
		}
	}

	entries, err := f.attendance.Entries(f.ctx, f.org.ID, session.ID)
	if err != nil {
		t.Fatalf("relevé: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("%d présence(s) enregistrée(s)", len(entries))
	}

	// 4 h + 3 h + 4 h : le matin dure quatre heures, l'après-midi trois.
	if hours := attendance.AttendedHours(sheet, entries, "LEA"); hours != 11 {
		t.Errorf("heures émargées = %.0f, attendu 11", hours)
	}
}

// Un créneau déjà émargé ne se réécrit pas : corriger passe par une absence
// motivée, qui laisse trace des deux états.
func TestDoubleSignatureIsRefused(t *testing.T) {
	f, session := newFixture(t, catalog.ModeOnsite)
	sheet, _ := f.attendance.EnsureSheet(f.ctx, f.org.ID, session.ID)
	slot := sheet.Slots[0]

	in := attendance.SignInput{
		OrgID: f.org.ID, SessionID: session.ID, SlotID: slot.ID,
		ContactID: "LEA", Method: attendance.MethodSignature, Actor: f.actor,
	}
	if _, err := f.attendance.Sign(f.ctx, in); err != nil {
		t.Fatalf("premier émargement: %v", err)
	}
	if _, err := f.attendance.Sign(f.ctx, in); err == nil {
		t.Fatal("un second émargement du même créneau a été accepté")
	}
}

// Une absence sans motif est une case vide déguisée.
func TestAbsenceRequiresReason(t *testing.T) {
	f, session := newFixture(t, catalog.ModeOnsite)
	sheet, _ := f.attendance.EnsureSheet(f.ctx, f.org.ID, session.ID)

	if _, err := f.attendance.Sign(f.ctx, attendance.SignInput{
		OrgID: f.org.ID, SessionID: session.ID, SlotID: sheet.Slots[0].ID,
		ContactID: "LEA", Method: attendance.MethodAbsent, Actor: f.actor,
	}); err == nil {
		t.Fatal("une absence sans motif a été acceptée")
	}

	if _, err := f.attendance.Sign(f.ctx, attendance.SignInput{
		OrgID: f.org.ID, SessionID: session.ID, SlotID: sheet.Slots[0].ID,
		ContactID: "LEA", Method: attendance.MethodAbsent,
		Comment: "Arrêt maladie, justificatif transmis", Actor: f.actor,
	}); err != nil {
		t.Fatalf("absence motivée refusée: %v", err)
	}
}

// Une absence ne se facture pas.
func TestAbsenceIsNotBilled(t *testing.T) {
	f, session := newFixture(t, catalog.ModeOnsite)
	sheet, _ := f.attendance.EnsureSheet(f.ctx, f.org.ID, session.ID)

	if _, err := f.attendance.Sign(f.ctx, attendance.SignInput{
		OrgID: f.org.ID, SessionID: session.ID, SlotID: sheet.Slots[0].ID,
		ContactID: "LEA", Method: attendance.MethodSignature, Actor: f.actor,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.attendance.Sign(f.ctx, attendance.SignInput{
		OrgID: f.org.ID, SessionID: session.ID, SlotID: sheet.Slots[1].ID,
		ContactID: "LEA", Method: attendance.MethodAbsent,
		Comment: "Absent", Actor: f.actor,
	}); err != nil {
		t.Fatal(err)
	}

	entries, _ := f.attendance.Entries(f.ctx, f.org.ID, session.ID)
	if hours := attendance.AttendedHours(sheet, entries, "LEA"); hours != 4 {
		t.Errorf("heures facturables = %.0f, attendu 4 (l'absence ne compte pas)", hours)
	}
}

// La contresignature clôt la feuille et ne se rejoue pas.
func TestCountersignature(t *testing.T) {
	f, session := newFixture(t, catalog.ModeAsync)

	sheet, err := f.attendance.Countersign(f.ctx, f.org.ID, session.ID, f.owner, f.actor)
	if err != nil {
		t.Fatalf("contresignature: %v", err)
	}
	if sheet.TrainerSignedAt == nil || sheet.TrainerName != "Marie Dubreuil" {
		t.Fatalf("contresignature incomplète: %+v", sheet)
	}

	if _, err := f.attendance.Countersign(f.ctx, f.org.ID, session.ID, f.owner, f.actor); err == nil {
		t.Fatal("une seconde contresignature a été acceptée")
	}

	// Et l'acte doit figurer au journal de la session.
	events, err := f.db.AuditChain(f.ctx, "session/"+session.ID)
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	if len(events) != 1 || events[0].Action != audit.ActionAttendanceSigned {
		t.Errorf("journal inattendu: %+v", events)
	}
}

func TestUnknownSlotIsRefused(t *testing.T) {
	f, session := newFixture(t, catalog.ModeOnsite)

	if _, err := f.attendance.Sign(f.ctx, attendance.SignInput{
		OrgID: f.org.ID, SessionID: session.ID, SlotID: "2099-01-01-am",
		ContactID: "LEA", Method: attendance.MethodSignature, Actor: f.actor,
	}); err == nil {
		t.Fatal("un créneau inconnu a été émargé")
	}
}

// Un émargement ne vaut que s'il est contemporain du créneau : c'est
// précisément ce qu'un contrôleur vérifie sur une feuille de présence.
func TestLearnerCanSign(t *testing.T) {
	debut := time.Date(2026, 3, 12, 9, 0, 0, 0, time.UTC)
	creneau := attendance.Slot{
		ID: "2026-03-12-am", Label: "12/03/2026 — matin",
		Start: debut, End: debut.Add(4 * time.Hour),
	}

	cas := []struct {
		nom     string
		instant time.Time
		permis  bool
	}{
		{"la veille", debut.Add(-24 * time.Hour), false},
		{"une heure avant", debut.Add(-time.Hour), false},
		{"dix minutes avant", debut.Add(-10 * time.Minute), true},
		{"pendant", debut.Add(2 * time.Hour), true},
		{"deux heures après la fin", debut.Add(6 * time.Hour), true},
		{"le lendemain", debut.Add(30 * time.Hour), false},
	}

	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			permis, motif := attendance.LearnerCanSign(catalog.ModeOnsite, creneau, c.instant)
			if permis != c.permis {
				t.Errorf("LearnerCanSign = %v (%s), attendu %v", permis, motif, c.permis)
			}
			if !permis && motif == "" {
				t.Error("un refus doit être motivé : l'apprenant doit savoir quoi faire")
			}
		})
	}
}

// En asynchrone, la présence vient du relevé de connexion. Demander en plus
// une signature ferait attester l'apprenant d'un horaire qu'il n'a pas suivi.
func TestLearnerCannotSignAsynchronousSlots(t *testing.T) {
	debut := time.Date(2026, 3, 12, 9, 0, 0, 0, time.UTC)
	creneau := attendance.Slot{ID: "m1", Start: debut, End: debut.Add(2 * time.Hour)}

	if permis, motif := attendance.LearnerCanSign(catalog.ModeAsync, creneau, debut.Add(time.Hour)); permis {
		t.Errorf("l'émargement asynchrone devrait être refusé (%s)", motif)
	}
}
