package library_test

import (
	"context"
	"testing"

	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/library"
	"github.com/lemlearn/api/internal/platform/ddb"
)

func published(t *testing.T, s *library.Service) library.Course {
	t.Helper()
	course, err := s.SaveCourse(context.Background(), library.Course{
		Title: "Gestes et postures", Goal: "Prévenir les TMS",
		Objectives: []string{"Identifier les situations à risque"},
		Audience:   "Personnel de manutention", DurationHours: 7,
		Sanction: "Attestation de fin de formation", Published: true,
	})
	if err != nil {
		t.Fatalf("enregistrement: %v", err)
	}
	if _, err := s.SaveModule(context.Background(), library.Module{
		CourseID: course.ID, Title: "Anatomie du dos", Position: 1, DurationMs: 600_000,
	}); err != nil {
		t.Fatalf("module: %v", err)
	}
	return course
}

// Une formation publiée doit porter les mentions exigées en audit : elle
// produira des conventions chez tous ceux qui l'importent.
func TestPublishingDemandsTheAuditMentions(t *testing.T) {
	s := library.NewService(ddb.NewTestClient(t), nil)

	for _, incomplete := range []library.Course{
		{Title: "Sans public", Objectives: []string{"x"}, DurationHours: 7, Published: true},
		{Title: "Sans objectif", Audience: "x", DurationHours: 7, Published: true},
		{Title: "Sans durée", Audience: "x", Objectives: []string{"y"}, Published: true},
	} {
		if _, err := s.SaveCourse(context.Background(), incomplete); err == nil {
			t.Errorf("publication acceptée : %s", incomplete.Title)
		}
	}

	// En brouillon, en revanche, on écrit ce qu'on veut : la formation se
	// compose en plusieurs fois.
	if _, err := s.SaveCourse(context.Background(), library.Course{Title: "En cours d'écriture"}); err != nil {
		t.Errorf("brouillon refusé : %v", err)
	}
}

// Un organisme ne voit que ce qui lui est ouvert.
func TestDraftsStayHiddenFromOrganisations(t *testing.T) {
	s := library.NewService(ddb.NewTestClient(t), nil)
	published(t, s)
	if _, err := s.SaveCourse(context.Background(), library.Course{Title: "Brouillon"}); err != nil {
		t.Fatal(err)
	}

	forTeam, err := s.ListCourses(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	forOrgs, err := s.ListCourses(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(forTeam) != 2 || len(forOrgs) != 1 {
		t.Fatalf("équipe : %d formations, organismes : %d", len(forTeam), len(forOrgs))
	}
}

// L'import est une copie, pas une référence : la formation devient celle de
// l'organisme, et nos remaniements ultérieurs ne la touchent plus.
func TestImportCopiesRatherThanReferences(t *testing.T) {
	db := ddb.NewTestClient(t)
	s := library.NewService(db, nil)
	source := published(t, s)

	imported, modules, err := s.Import(context.Background(), "ORG1", source.ID)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if imported.ID == source.ID {
		t.Error("la copie porte l'identifiant de l'original")
	}
	if imported.OrgID != "ORG1" || modules != 1 {
		t.Fatalf("copie = %+v, %d modules", imported, modules)
	}
	// Elle arrive en brouillon : le formateur l'assume avant de la publier.
	if imported.Published {
		t.Error("la copie est publiée d'office")
	}
	if imported.Audience != source.Audience || len(imported.Objectives) != 1 {
		t.Error("les mentions n'ont pas suivi")
	}

	// La modifier chez nous ne doit pas la modifier chez lui.
	source.Title = "Titre remanié"
	if _, err := s.SaveCourse(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ddb.Get[catalog.Course](context.Background(), db,
		ddb.OrgPK("ORG1"), ddb.CourseSK(imported.ID))
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Title != "Gestes et postures" {
		t.Errorf("la formation de l'organisme a changé sous ses pieds : %q", reloaded.Title)
	}
}

// Une formation non publiée ne s'importe pas, même si on connaît son
// identifiant.
func TestDraftCannotBeImported(t *testing.T) {
	s := library.NewService(ddb.NewTestClient(t), nil)
	draft, err := s.SaveCourse(context.Background(), library.Course{Title: "Brouillon"})
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.Import(context.Background(), "ORG1", draft.ID); err == nil {
		t.Fatal("un brouillon a été importé")
	}
}
