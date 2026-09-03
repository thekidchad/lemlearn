// Package library porte le catalogue de formations que lemlearn met à
// disposition de ses clients.
//
// Un organisme n'y puise pas une référence : il en prend une copie. La
// convention qu'il signera décrit la formation qu'il dispense réellement, et
// une formation qui changerait sous ses pieds parce que nous l'avons remaniée
// rendrait ses documents faux rétroactivement. La copie est donc le choix
// structurant, pas une facilité d'implémentation.
package library

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// PlatformPK range la bibliothèque hors de toute organisation.
const PlatformPK = "PLATFORM"

func courseSK(id string) string           { return "LIBCOURSE#" + id }
func moduleSK(courseID, id string) string { return "LIBCOURSE#" + courseID + "#MOD#" + id }

// Course est une formation de la bibliothèque.
type Course struct {
	ddb.Record

	ID    string `dynamodbav:"id" json:"id"`
	Title string `dynamodbav:"title" json:"title"`

	Goal          string   `dynamodbav:"goal,omitempty" json:"goal,omitempty"`
	Objectives    []string `dynamodbav:"objectives,omitempty" json:"objectives,omitempty"`
	Prerequisites string   `dynamodbav:"prerequisites,omitempty" json:"prerequisites,omitempty"`
	Audience      string   `dynamodbav:"audience,omitempty" json:"audience,omitempty"`
	Means         string   `dynamodbav:"means,omitempty" json:"means,omitempty"`
	Assessment    string   `dynamodbav:"assessment,omitempty" json:"assessment,omitempty"`
	Sanction      string   `dynamodbav:"sanction,omitempty" json:"sanction,omitempty"`
	Accessibility string   `dynamodbav:"accessibility,omitempty" json:"accessibility,omitempty"`
	DurationHours float64  `dynamodbav:"durationHours" json:"durationHours"`
	Tags          []string `dynamodbav:"tags,omitempty" json:"tags,omitempty"`

	// Summary décrit la formation aux organismes qui la parcourent, pas aux
	// apprenants : « pour qui, et ce qu'elle vous évite d'écrire ».
	Summary string `dynamodbav:"summary,omitempty" json:"summary,omitempty"`

	// Published ouvre la formation aux organismes. Une formation en cours
	// d'écriture ne doit pas apparaître dans leur bibliothèque.
	Published bool `dynamodbav:"published" json:"published"`
}

// Module est une séquence d'une formation de la bibliothèque.
type Module struct {
	ddb.Record

	ID       string `dynamodbav:"id" json:"id"`
	CourseID string `dynamodbav:"courseId" json:"courseId"`

	Position           int    `dynamodbav:"position" json:"position"`
	Title              string `dynamodbav:"title" json:"title"`
	Summary            string `dynamodbav:"summary,omitempty" json:"summary,omitempty"`
	DurationMs         int64  `dynamodbav:"durationMs" json:"durationMs"`
	MinCoveragePercent int    `dynamodbav:"minCoveragePercent" json:"minCoveragePercent"`

	// AssetID désigne une vidéo hébergée par lemlearn. Elle est recopiée à
	// l'import : les fichiers vivent dans le même compartiment, seule la fiche
	// de l'asset est dupliquée dans l'organisation.
	AssetID string `dynamodbav:"assetId,omitempty" json:"assetId,omitempty"`
}

// Service porte la bibliothèque.
type Service struct {
	db  *ddb.Client
	now func() time.Time
}

// NewService construit le service.
func NewService(db *ddb.Client, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{db: db, now: now}
}

// SaveCourse crée ou met à jour une formation de la bibliothèque.
func (s *Service) SaveCourse(ctx context.Context, course Course) (Course, error) {
	if course.Title == "" {
		return Course{}, fmt.Errorf("l'intitulé de la formation est obligatoire")
	}
	if course.Published {
		// Les mêmes exigences que pour un organisme : une formation publiée
		// sans public visé ni objectifs produirait des conventions
		// incomplètes chez tous ceux qui l'importent.
		if course.Audience == "" {
			return Course{}, fmt.Errorf("le public visé est obligatoire pour publier")
		}
		if len(course.Objectives) == 0 {
			return Course{}, fmt.Errorf("au moins un objectif pédagogique est nécessaire pour publier")
		}
		if course.DurationHours <= 0 {
			return Course{}, fmt.Errorf("la durée doit être renseignée pour publier")
		}
	}

	now := s.now()
	if course.ID == "" {
		course.ID = identity.NewID()
		course.CreatedAt = now
	}
	course.PK, course.SK = PlatformPK, courseSK(course.ID)
	course.Type = "library_course"
	course.UpdatedAt = now

	if err := ddb.Put(ctx, s.db, course); err != nil {
		return Course{}, err
	}
	return course, nil
}

// SaveModule ajoute ou modifie un module de la bibliothèque.
func (s *Service) SaveModule(ctx context.Context, module Module) (Module, error) {
	if module.CourseID == "" || module.Title == "" {
		return Module{}, fmt.Errorf("le module doit porter un intitulé et sa formation")
	}
	if module.AssetID != "" && module.DurationMs <= 0 {
		return Module{}, fmt.Errorf("un module vidéo doit porter sa durée")
	}

	now := s.now()
	if module.ID == "" {
		module.ID = identity.NewID()
		module.CreatedAt = now
	}
	module.PK, module.SK = PlatformPK, moduleSK(module.CourseID, module.ID)
	module.Type = "library_module"
	module.UpdatedAt = now
	if module.MinCoveragePercent == 0 {
		module.MinCoveragePercent = 80
	}

	if err := ddb.Put(ctx, s.db, module); err != nil {
		return Module{}, err
	}
	return module, nil
}

// ListCourses renvoie la bibliothèque. `publishedOnly` sert aux organismes.
func (s *Service) ListCourses(ctx context.Context, publishedOnly bool) ([]Course, error) {
	courses, err := ddb.Query[Course](ctx, s.db, ddb.QuerySpec{PK: PlatformPK, SKPrefix: "LIBCOURSE#"})
	if err != nil {
		return nil, err
	}

	kept := make([]Course, 0, len(courses))
	for _, course := range courses {
		// La requête ramène aussi les modules, dont la clé de tri commence par
		// le même préfixe : on les écarte sur le type.
		if course.Type != "library_course" {
			continue
		}
		if publishedOnly && !course.Published {
			continue
		}
		kept = append(kept, course)
	}
	return kept, nil
}

// Course renvoie une formation et ses modules.
func (s *Service) Course(ctx context.Context, id string) (Course, []Module, error) {
	course, err := ddb.Get[Course](ctx, s.db, PlatformPK, courseSK(id))
	if err != nil {
		return Course{}, nil, err
	}
	modules, err := ddb.Query[Module](ctx, s.db, ddb.QuerySpec{
		PK: PlatformPK, SKPrefix: "LIBCOURSE#" + id + "#MOD#",
	})
	if err != nil {
		return Course{}, nil, err
	}
	return course, modules, nil
}

// DeleteCourse retire une formation de la bibliothèque.
//
// Les copies déjà importées par les organismes ne bougent pas : ce sont leurs
// formations, et elles portent leurs sessions et leurs conventions.
func (s *Service) DeleteCourse(ctx context.Context, id string) error {
	_, modules, err := s.Course(ctx, id)
	if err != nil {
		return err
	}
	for _, module := range modules {
		if err := ddb.Delete(ctx, s.db, PlatformPK, moduleSK(id, module.ID)); err != nil {
			return err
		}
	}
	return ddb.Delete(ctx, s.db, PlatformPK, courseSK(id))
}

// Import recopie une formation de la bibliothèque dans une organisation.
//
// Renvoie la formation créée chez le client. Elle n'est pas publiée : le
// formateur la relit, l'adapte à ses moyens et à son public, puis la publie —
// c'est lui qui l'assume devant un auditeur, pas nous.
func (s *Service) Import(ctx context.Context, orgID, courseID string) (catalog.Course, int, error) {
	source, modules, err := s.Course(ctx, courseID)
	if err != nil {
		return catalog.Course{}, 0, err
	}
	if !source.Published {
		return catalog.Course{}, 0, errors.New("cette formation n'est pas ouverte aux organismes")
	}

	now := s.now()
	course := catalog.NewCourse(orgID, source.Title, now)
	course.Goal = source.Goal
	course.Objectives = source.Objectives
	course.Prerequisites = source.Prerequisites
	course.Audience = source.Audience
	course.Means = source.Means
	course.Assessment = source.Assessment
	course.Sanction = source.Sanction
	course.Accessibility = source.Accessibility
	course.DurationHours = source.DurationHours
	course.Tags = source.Tags
	course.Published = false
	course.Reindex()

	writes := []ddb.Write{{Item: course}}

	for _, source := range modules {
		module := catalog.NewModule(orgID, course.ID, source.Title, source.Position, now)
		module.Summary = source.Summary
		module.DurationMs = source.DurationMs
		module.MinCoveragePercent = source.MinCoveragePercent
		module.AssetID = source.AssetID
		writes = append(writes, ddb.Write{Item: module})

		// La vidéo suit : les fichiers sont déjà dans le compartiment, seule
		// la fiche de l'asset est recopiée sous l'organisation, sans quoi la
		// lecture serait refusée — un asset appartient à une organisation.
		if source.AssetID != "" {
			if asset, err := s.copyAsset(ctx, orgID, source.AssetID, now); err == nil {
				writes = append(writes, ddb.Write{Item: asset})
			}
		}
	}

	if err := s.db.Write(ctx, writes); err != nil {
		return catalog.Course{}, 0, err
	}
	return course, len(modules), nil
}

// copyAsset duplique la fiche d'une vidéo de la bibliothèque sous une
// organisation, en conservant les clés des fichiers.
func (s *Service) copyAsset(ctx context.Context, orgID, assetID string, now time.Time) (map[string]any, error) {
	raw, err := ddb.GetRaw(ctx, s.db, PlatformPK, ddb.AssetSK(assetID))
	if err != nil {
		return nil, err
	}
	raw["PK"] = ddb.OrgPK(orgID)
	raw["orgId"] = orgID
	raw["updatedAt"] = now.Format(time.RFC3339Nano)
	return raw, nil
}

// DeleteModule retire un module d'une formation de la bibliothèque.
//
// Rien ne casse chez les organismes qui ont déjà importé : l'import fabrique
// une copie, et c'est bien la raison d'être de cette copie — leur convention
// décrit ce qu'ils dispensent, pas ce que la bibliothèque est devenue depuis.
func (s *Service) DeleteModule(ctx context.Context, courseID, moduleID string) error {
	if courseID == "" || moduleID == "" {
		return fmt.Errorf("la formation et le module sont obligatoires")
	}
	return ddb.Delete(ctx, s.db, PlatformPK, moduleSK(courseID, moduleID))
}
