package catalog

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// Service porte les cas d'usage du catalogue.
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

// CreateCourse enregistre une formation.
func (s *Service) CreateCourse(ctx context.Context, course Course) (Course, error) {
	if err := course.Validate(); err != nil {
		return Course{}, err
	}
	course.Reindex()
	course.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, course); err != nil {
		return Course{}, err
	}
	return course, nil
}

// GetCourse lit une formation.
func (s *Service) GetCourse(ctx context.Context, orgID, courseID string) (Course, error) {
	return ddb.Get[Course](ctx, s.db, ddb.OrgPK(orgID), ddb.CourseSK(courseID))
}

// SetCover rattache le visuel d'une formation, ou le retire quand la clé est
// vide. Retirer est un usage à part entière : un organisme qui change de
// charte veut pouvoir revenir à la bande unie de sa couleur.
func (s *Service) SetCover(ctx context.Context, orgID, courseID, key string) (Course, error) {
	course, err := s.GetCourse(ctx, orgID, courseID)
	if err != nil {
		return Course{}, err
	}
	course.CoverKey = key
	course.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, course); err != nil {
		return Course{}, err
	}
	return course, nil
}

// ListCourses liste le catalogue par ordre alphabétique.
func (s *Service) ListCourses(ctx context.Context, orgID string, limit int32) ([]Course, error) {
	return ddb.Query[Course](ctx, s.db, ddb.QuerySpec{
		Index: "GSI1", PK: ddb.OrgPK(orgID) + "#KIND#course", Limit: limit,
	})
}

// ListCoursesPage et ListSessionsPage paginent par curseur : un catalogue
// grossit sans limite, et charger dix ans de sessions pour en afficher vingt
// coûte à chaque ouverture d'écran.
func (s *Service) ListCoursesPage(ctx context.Context, orgID string, limit int32, cursor string) (ddb.Page[Course], error) {
	return ddb.QueryPage[Course](ctx, s.db, ddb.QuerySpec{
		Index: "GSI1", PK: ddb.OrgPK(orgID) + "#KIND#course", Limit: limit,
	}, cursor)
}

func (s *Service) ListSessionsPage(ctx context.Context, orgID string, limit int32, cursor string) (ddb.Page[Session], error) {
	return ddb.QueryPage[Session](ctx, s.db, ddb.QuerySpec{
		Index: "GSI1", PK: ddb.GSI1Sessions(orgID), Descending: true, Limit: limit,
	}, cursor)
}

// AddModule ajoute un module à une formation.
func (s *Service) AddModule(ctx context.Context, module Module) (Module, error) {
	if module.Title == "" {
		return Module{}, fmt.Errorf("l'intitulé du module est obligatoire")
	}
	if module.AssetID != "" && module.DurationMs <= 0 {
		// Sans durée, aucune couverture n'est calculable : le module
		// paraîtrait validé sans qu'on ait rien mesuré.
		return Module{}, fmt.Errorf("un module vidéo doit porter sa durée")
	}
	module.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, module); err != nil {
		return Module{}, err
	}
	return module, nil
}

// SaveModule enregistre un module modifié.
//
// Les mêmes règles qu'à la création : une vidéo sans durée rendrait la
// couverture incalculable, et le module paraîtrait validé sans qu'on ait rien
// mesuré.
func (s *Service) SaveModule(ctx context.Context, module Module) (Module, error) {
	return s.AddModule(ctx, module)
}

// ListModules renvoie les modules d'une formation, dans l'ordre pédagogique.
func (s *Service) ListModules(ctx context.Context, orgID, courseID string) ([]Module, error) {
	return ddb.Query[Module](ctx, s.db, ddb.QuerySpec{
		Index: "GSI1", PK: ddb.OrgPK(orgID) + "#COURSE#" + courseID,
	})
}

// GetModule lit un module.
func (s *Service) GetModule(ctx context.Context, orgID, courseID, moduleID string) (Module, error) {
	return ddb.Get[Module](ctx, s.db, ddb.OrgPK(orgID), ddb.ModuleSK(courseID, moduleID))
}

// CreateSession planifie une session.
func (s *Service) CreateSession(ctx context.Context, session Session) (Session, error) {
	if err := session.Validate(); err != nil {
		return Session{}, err
	}
	if _, err := s.GetCourse(ctx, session.OrgID, session.CourseID); err != nil {
		return Session{}, fmt.Errorf("formation introuvable: %w", err)
	}
	session.Reindex()
	session.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, session); err != nil {
		return Session{}, err
	}
	return session, nil
}

// GetSession lit une session.
func (s *Service) GetSession(ctx context.Context, orgID, sessionID string) (Session, error) {
	return ddb.Get[Session](ctx, s.db, ddb.OrgPK(orgID), ddb.SessionSK(sessionID))
}

// ListSessions renvoie l'agenda, par date croissante.
func (s *Service) ListSessions(ctx context.Context, orgID string, limit int32) ([]Session, error) {
	return ddb.Query[Session](ctx, s.db, ddb.QuerySpec{
		Index: "GSI1", PK: ddb.GSI1Sessions(orgID), Limit: limit,
	})
}

// EnrollInput décrit une inscription.
type EnrollInput struct {
	OrgID     string
	SessionID string
	ContactID string
	FileID    string

	// Ce que la convention doit porter et que le bilan réclamera : la nature
	// du stagiaire, la période du contrat, la ventilation des heures. Vides,
	// ils ne bloquent pas l'inscription — on inscrit d'abord, on complète
	// ensuite — mais la convention et le bilan les redemanderont.
	TraineeType    string
	ContractStart  *time.Time
	ContractEnd    *time.Time
	HoursElearning float64
	HoursRemote    float64
	HoursOnSite    float64

	Actor audit.Actor
}

// Enroll inscrit un apprenant à une session.
//
// L'inscription est journalisée sur le dossier lorsqu'il est connu : c'est le
// dossier qui porte la chaîne de preuve, et une inscription qui n'y figurerait
// pas serait une pièce manquante à l'audit.
func (s *Service) Enroll(ctx context.Context, in EnrollInput) (Enrollment, error) {
	session, err := s.GetSession(ctx, in.OrgID, in.SessionID)
	if err != nil {
		return Enrollment{}, fmt.Errorf("session introuvable: %w", err)
	}
	if session.Closed {
		return Enrollment{}, fmt.Errorf("cette session est clôturée")
	}

	now := s.now()
	enrollment := NewEnrollment(in.OrgID, in.SessionID, in.ContactID, session.StartsAt, now)
	enrollment.FileID = in.FileID
	enrollment.TraineeType = in.TraineeType
	enrollment.ContractStart, enrollment.ContractEnd = in.ContractStart, in.ContractEnd
	enrollment.HoursElearning = in.HoursElearning
	enrollment.HoursRemote = in.HoursRemote
	enrollment.HoursOnSite = in.HoursOnSite

	if in.FileID == "" {
		// Sans dossier, l'inscription existe quand même — un organisme peut
		// inscrire avant d'avoir monté le dossier administratif — mais elle
		// n'alimente aucune chaîne de preuve tant qu'elle n'y est pas reliée.
		if err := ddb.PutNew(ctx, s.db, enrollment); err != nil {
			return Enrollment{}, err
		}
		return enrollment, nil
	}

	// Le dossier apprend de quelle session il relève : sans ce lien, l'export
	// ne saurait pas quels relevés produire, et les pièces d'assiduité
	// manqueraient au dossier probatoire sans que rien ne le signale.
	writes := []ddb.Write{{Item: enrollment, Condition: "attribute_not_exists(SK)"}}
	if linked, err := s.linkFile(ctx, in.OrgID, in.FileID, session, now); err == nil {
		writes = append(writes, ddb.Write{Item: linked})
	}

	_, err = s.db.WriteWithAudit(ctx, "file/"+in.FileID, writes,
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, "file/"+in.FileID, now, audit.ActionFileStageChanged, in.Actor,
				map[string]any{
					"event":     "enrollment",
					"sessionId": in.SessionID,
					"contactId": in.ContactID,
					"startsAt":  session.StartsAt.Format(time.RFC3339),
				})
		})
	if err != nil {
		return Enrollment{}, err
	}
	return enrollment, nil
}

// linkFile rattache le dossier à la session et à sa formation.
//
// Renvoie une erreur si le dossier est introuvable : l'appelant l'ignore alors
// et l'inscription se fait sans lien, ce qui reste un état valide — on peut
// inscrire avant d'avoir monté le dossier administratif.
func (s *Service) linkFile(ctx context.Context, orgID, fileID string, session Session, now time.Time) (map[string]any, error) {
	file, err := ddb.GetRaw(ctx, s.db, ddb.OrgPK(orgID), ddb.FileSK(fileID))
	if err != nil {
		return nil, err
	}
	file["sessionId"] = session.ID
	file["courseId"] = session.CourseID
	file["updatedAt"] = now.UTC().Format(time.RFC3339Nano)
	return file, nil
}

// GetEnrollment lit une inscription.
func (s *Service) GetEnrollment(ctx context.Context, orgID, sessionID, contactID string) (Enrollment, error) {
	return ddb.Get[Enrollment](ctx, s.db, ddb.OrgPK(orgID), ddb.EnrollmentSK(sessionID, contactID))
}

// SaveEnrollment réécrit une inscription.
func (s *Service) SaveEnrollment(ctx context.Context, enrollment Enrollment) error {
	enrollment.UpdatedAt = s.now()
	return ddb.Put(ctx, s.db, enrollment)
}

// ListSessionEnrollments renvoie les inscrits d'une session — la liste
// d'émargement.
func (s *Service) ListSessionEnrollments(ctx context.Context, orgID, sessionID string) ([]Enrollment, error) {
	return ddb.Query[Enrollment](ctx, s.db, ddb.QuerySpec{
		PK: ddb.OrgPK(orgID), SKPrefix: "SESSION#" + sessionID + "#ENR#",
	})
}

// ListLearnerEnrollments renvoie le parcours d'un apprenant.
func (s *Service) ListLearnerEnrollments(ctx context.Context, orgID, contactID string) ([]Enrollment, error) {
	return ddb.Query[Enrollment](ctx, s.db, ddb.QuerySpec{
		Index: "GSI1", PK: ddb.GSI1Enrollments(orgID, contactID), Descending: true,
	})
}

// SetPublished fait sortir une formation du brouillon, ou l'y remet.
//
// L'écriture relit la formation plutôt que d'écrire un article partiel : un
// PutItem porte l'objet entier, et l'écrire depuis une copie ancienne
// effacerait ce qui a changé entre-temps.
func (s *Service) SetPublished(ctx context.Context, orgID, courseID string, published bool) (Course, error) {
	course, err := s.GetCourse(ctx, orgID, courseID)
	if err != nil {
		return Course{}, err
	}
	course.Published = published
	course.UpdatedAt = s.now()
	course.Reindex()
	if err := ddb.Put(ctx, s.db, course); err != nil {
		return Course{}, err
	}
	return course, nil
}

// UpdateCourse applique une correction à une formation.
//
// La relecture puis l'écriture entière plutôt qu'une mise à jour d'attributs :
// l'article porte des champs dérivés — les clés d'index en dépendent — et les
// écrire séparément les laisserait désaccordés du reste.
func (s *Service) UpdateCourse(
	ctx context.Context, orgID, courseID string, apply func(*Course),
) (Course, error) {
	course, err := s.GetCourse(ctx, orgID, courseID)
	if err != nil {
		return Course{}, err
	}
	apply(&course)
	if err := course.Validate(); err != nil {
		return Course{}, err
	}
	course.UpdatedAt = s.now()
	course.Reindex()
	if err := ddb.Put(ctx, s.db, course); err != nil {
		return Course{}, err
	}
	return course, nil
}

// DuplicateCourse recopie une formation et ses modules.
//
// C'est le geste le plus fréquent d'un catalogue : une même formation revient
// en version courte, en intensif, en intra-entreprise. La réécrire à chaque
// fois fait diverger les mentions obligatoires, qu'on ne relit jamais deux
// fois avec la même attention.
//
// La copie part en brouillon, quoi qu'il arrive : publier automatiquement un
// duplicata mettrait au catalogue une formation dont le titre est encore
// « (copie) ».
func (s *Service) DuplicateCourse(ctx context.Context, orgID, courseID, titre string) (Course, error) {
	source, err := s.GetCourse(ctx, orgID, courseID)
	if err != nil {
		return Course{}, err
	}
	modules, err := s.ListModules(ctx, orgID, courseID)
	if err != nil {
		return Course{}, err
	}

	now := s.now()
	if strings.TrimSpace(titre) == "" {
		titre = source.Title + " (copie)"
	}

	copie := NewCourse(orgID, titre, now)
	copie.Goal, copie.Objectives = source.Goal, source.Objectives
	copie.Prerequisites, copie.Audience = source.Prerequisites, source.Audience
	copie.Means, copie.Assessment = source.Means, source.Assessment
	copie.Sanction, copie.Accessibility = source.Sanction, source.Accessibility
	copie.DurationHours, copie.PriceHT = source.DurationHours, source.PriceHT
	copie.Tags = source.Tags
	copie.ObjectiveType, copie.CertificationCode = source.ObjectiveType, source.CertificationCode
	copie.PositioningQuizID, copie.FinalQuizID = source.PositioningQuizID, source.FinalQuizID
	// Le visuel n'est pas recopié : deux formations au même sujet portant la
	// même image se confondent dans la liste, et c'est justement le moment de
	// leur en donner deux.
	copie.Published = false

	created, err := s.CreateCourse(ctx, copie)
	if err != nil {
		return Course{}, err
	}

	// Les modules suivent, avec leur vidéo : le fichier n'est pas dupliqué,
	// seule la fiche du module pointe vers le même asset.
	for _, module := range modules {
		clone := NewModule(orgID, created.ID, module.Title, module.Position, now)
		clone.Summary = module.Summary
		clone.DurationMs, clone.MinCoveragePercent = module.DurationMs, module.MinCoveragePercent
		clone.AssetID, clone.QuizID = module.AssetID, module.QuizID
		if _, err := s.AddModule(ctx, clone); err != nil {
			return created, err
		}
	}
	return created, nil
}
