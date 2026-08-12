// Package catalog porte les formations, leurs modules, les sessions
// planifiées et les inscriptions.
//
// La distinction qui structure tout : une *formation* est un contenu
// réutilisable, une *session* est une occurrence datée à laquelle on inscrit
// des apprenants. Confondre les deux — comme le font les outils qui attachent
// les apprenants au contenu — rend impossible de produire un émargement, qui
// porte toujours sur une session.
package catalog

import (
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// Mode est la modalité d'une session.
type Mode string

const (
	// ModeOnsite : présentiel. L'émargement se fait par demi-journée.
	ModeOnsite Mode = "onsite"
	// ModeVirtual : classe virtuelle synchrone. Émargement par créneau.
	ModeVirtual Mode = "virtual"
	// ModeAsync : distanciel asynchrone. L'assiduité vient des relevés de
	// connexion, l'émargement est contresigné par le formateur.
	ModeAsync Mode = "async"
	// ModeBlended : mixte.
	ModeBlended Mode = "blended"
)

// Valid indique si la modalité fait partie de la liste fermée.
func (m Mode) Valid() bool {
	switch m {
	case ModeOnsite, ModeVirtual, ModeAsync, ModeBlended:
		return true
	}
	return false
}

// Course est une formation du catalogue.
type Course struct {
	ddb.Record

	ID    string `dynamodbav:"id" json:"id"`
	OrgID string `dynamodbav:"orgId" json:"orgId"`

	Title         string   `dynamodbav:"title" json:"title"`
	Goal          string   `dynamodbav:"goal,omitempty" json:"goal,omitempty"`
	Objectives    []string `dynamodbav:"objectives,omitempty" json:"objectives,omitempty"`
	Prerequisites string   `dynamodbav:"prerequisites,omitempty" json:"prerequisites,omitempty"`
	Audience      string   `dynamodbav:"audience,omitempty" json:"audience,omitempty"`
	Means         string   `dynamodbav:"means,omitempty" json:"means,omitempty"`
	Assessment    string   `dynamodbav:"assessment,omitempty" json:"assessment,omitempty"`
	Sanction      string   `dynamodbav:"sanction,omitempty" json:"sanction,omitempty"`
	Accessibility string   `dynamodbav:"accessibility,omitempty" json:"accessibility,omitempty"`

	DurationHours float64  `dynamodbav:"durationHours" json:"durationHours"`
	PriceHT       float64  `dynamodbav:"priceHT" json:"priceHT"`
	Tags          []string `dynamodbav:"tags,omitempty" json:"tags,omitempty"`

	// PositioningQuizID est l'évaluation d'entrée, exigée par Qualiopi.
	PositioningQuizID string `dynamodbav:"positioningQuizId,omitempty" json:"positioningQuizId,omitempty"`
	// FinalQuizID est l'évaluation de sortie, qui conditionne l'attestation.
	FinalQuizID string `dynamodbav:"finalQuizId,omitempty" json:"finalQuizId,omitempty"`

	Published bool `dynamodbav:"published" json:"published"`
}

// NewCourse construit une formation.
func NewCourse(orgID, title string, now time.Time) Course {
	id := identity.NewID()
	course := Course{
		Record: ddb.Record{
			PK: ddb.OrgPK(orgID), SK: ddb.CourseSK(id), Type: "course",
			CreatedAt: now, UpdatedAt: now,
		},
		ID: id, OrgID: orgID, Title: title,
		Sanction: "Attestation de fin de formation",
	}
	course.Reindex()
	return course
}

// Reindex recalcule les clés d'index.
func (c *Course) Reindex() {
	c.GSI1PK = ddb.OrgPK(c.OrgID) + "#KIND#course"
	c.GSI1SK = ddb.SearchKey(c.Title)
	c.GSI2PK = ddb.GSI2Search(c.OrgID)
	c.GSI2SK = ddb.SearchKey(c.Title, strings.Join(c.Tags, " "))
}

// Validate refuse une formation impubliable.
func (c Course) Validate() error {
	if strings.TrimSpace(c.Title) == "" {
		return fmt.Errorf("l'intitulé de la formation est obligatoire")
	}
	if c.Published {
		// Les mentions ci-dessous figurent sur la convention et sont
		// contrôlées en audit : publier sans elles produirait des documents
		// incomplets, qu'il faudrait refaire.
		if strings.TrimSpace(c.Audience) == "" {
			return fmt.Errorf("le public visé est obligatoire pour publier")
		}
		if len(c.Objectives) == 0 {
			return fmt.Errorf("au moins un objectif pédagogique est nécessaire pour publier")
		}
		if c.DurationHours <= 0 {
			return fmt.Errorf("la durée doit être renseignée pour publier")
		}
	}
	return nil
}

// Module est une séquence d'une formation.
type Module struct {
	ddb.Record

	ID       string `dynamodbav:"id" json:"id"`
	OrgID    string `dynamodbav:"orgId" json:"orgId"`
	CourseID string `dynamodbav:"courseId" json:"courseId"`

	Position int    `dynamodbav:"position" json:"position"`
	Title    string `dynamodbav:"title" json:"title"`
	Summary  string `dynamodbav:"summary,omitempty" json:"summary,omitempty"`

	// AssetID désigne la vidéo encodée. Vide pour un module sans vidéo
	// (lecture, travail personnel).
	AssetID    string `dynamodbav:"assetId,omitempty" json:"assetId,omitempty"`
	DurationMs int64  `dynamodbav:"durationMs" json:"durationMs"`

	// QuizID est le contrôle après module.
	QuizID string `dynamodbav:"quizId,omitempty" json:"quizId,omitempty"`

	// MinCoveragePercent est la part du module qu'il faut avoir réellement
	// vue pour qu'il soit validé. 80 % par défaut : exiger 100 % sanctionne
	// l'apprenant qui saute un générique, mais 50 % ne prouve rien.
	MinCoveragePercent int `dynamodbav:"minCoveragePercent" json:"minCoveragePercent"`

	Resources []Resource `dynamodbav:"resources,omitempty" json:"resources,omitempty"`
}

// Resource est un document téléchargeable rattaché à un module.
type Resource struct {
	Label string `dynamodbav:"label" json:"label"`
	Key   string `dynamodbav:"key" json:"key"`
	Bytes int64  `dynamodbav:"bytes" json:"bytes"`
}

// NewModule construit un module.
func NewModule(orgID, courseID, title string, position int, now time.Time) Module {
	id := identity.NewID()
	return Module{
		Record: ddb.Record{
			PK: ddb.OrgPK(orgID), SK: ddb.ModuleSK(courseID, id), Type: "module",
			GSI1PK:    ddb.OrgPK(orgID) + "#COURSE#" + courseID,
			GSI1SK:    fmt.Sprintf("%04d", position),
			CreatedAt: now, UpdatedAt: now,
		},
		ID: id, OrgID: orgID, CourseID: courseID,
		Position: position, Title: title, MinCoveragePercent: 80,
	}
}

// Session est une occurrence datée d'une formation.
type Session struct {
	ddb.Record

	ID       string `dynamodbav:"id" json:"id"`
	OrgID    string `dynamodbav:"orgId" json:"orgId"`
	CourseID string `dynamodbav:"courseId" json:"courseId"`

	Title     string    `dynamodbav:"title" json:"title"`
	Mode      Mode      `dynamodbav:"mode" json:"mode"`
	StartsAt  time.Time `dynamodbav:"startsAt" json:"startsAt"`
	EndsAt    time.Time `dynamodbav:"endsAt" json:"endsAt"`
	Location  string    `dynamodbav:"location,omitempty" json:"location,omitempty"`
	TrainerID string    `dynamodbav:"trainerId,omitempty" json:"trainerId,omitempty"`
	Capacity  int       `dynamodbav:"capacity,omitempty" json:"capacity,omitempty"`

	// Tags libres : #présentiel, #certification, #OPCO-validé, #Q3…
	Tags []string `dynamodbav:"tags,omitempty" json:"tags,omitempty"`

	Closed   bool       `dynamodbav:"closed" json:"closed"`
	ClosedAt *time.Time `dynamodbav:"closedAt,omitempty" json:"closedAt,omitempty"`
}

// NewSession construit une session.
func NewSession(orgID, courseID, title string, mode Mode, startsAt, endsAt time.Time, now time.Time) Session {
	id := identity.NewID()
	session := Session{
		Record: ddb.Record{
			PK: ddb.OrgPK(orgID), SK: ddb.SessionSK(id), Type: "session",
			CreatedAt: now, UpdatedAt: now,
		},
		ID: id, OrgID: orgID, CourseID: courseID,
		Title: title, Mode: mode, StartsAt: startsAt, EndsAt: endsAt,
	}
	session.Reindex()
	return session
}

// Reindex recalcule les clés d'index : les sessions se listent par date.
func (s *Session) Reindex() {
	s.GSI1PK = ddb.GSI1Sessions(s.OrgID)
	s.GSI1SK = s.StartsAt.UTC().Format(time.RFC3339Nano)
}

// Validate refuse une session incohérente.
func (s Session) Validate() error {
	if !s.Mode.Valid() {
		return fmt.Errorf("modalité %q inconnue", s.Mode)
	}
	if strings.TrimSpace(s.Title) == "" {
		return fmt.Errorf("l'intitulé de la session est obligatoire")
	}
	if s.CourseID == "" {
		return fmt.Errorf("la session doit se rattacher à une formation")
	}
	if !s.EndsAt.After(s.StartsAt) {
		return fmt.Errorf("la session se termine avant d'avoir commencé")
	}
	if s.Mode != ModeAsync && s.Location == "" {
		// Une session synchrone sans lieu ni lien ne peut pas figurer sur une
		// convocation, ni sur la convention.
		return fmt.Errorf("le lieu ou le lien de connexion est obligatoire en %s", s.Mode)
	}
	return nil
}

// EnrollmentStatus est l'état d'une inscription.
type EnrollmentStatus string

const (
	StatusEnrolled  EnrollmentStatus = "enrolled"
	StatusInPogress EnrollmentStatus = "in_progress"
	StatusCompleted EnrollmentStatus = "completed"
	StatusAbandoned EnrollmentStatus = "abandoned"
)

// ModuleProgress résume l'avancement d'un apprenant sur un module.
//
// C'est le couplage assiduité × acquis : un module n'est validé que si
// l'apprenant a réellement vu la vidéo *et* réussi le contrôle. C'est ce
// couplage qui fait la solidité d'un dossier devant un financeur.
type ModuleProgress struct {
	ModuleID string `dynamodbav:"moduleId" json:"moduleId"`

	CoveragePercent int   `dynamodbav:"coveragePercent" json:"coveragePercent"`
	WatchedMs       int64 `dynamodbav:"watchedMs" json:"watchedMs"`

	QuizAttempts int     `dynamodbav:"quizAttempts" json:"quizAttempts"`
	QuizPassed   bool    `dynamodbav:"quizPassed" json:"quizPassed"`
	QuizPercent  int     `dynamodbav:"quizPercent" json:"quizPercent"`
	QuizScore    float64 `dynamodbav:"quizScore" json:"quizScore"`

	CompletedAt *time.Time `dynamodbav:"completedAt,omitempty" json:"completedAt,omitempty"`
}

// Complete indique si le module est validé au regard de ses exigences.
func (p ModuleProgress) Complete(module Module) bool {
	if module.DurationMs > 0 && p.CoveragePercent < module.MinCoveragePercent {
		return false
	}
	if module.QuizID != "" && !p.QuizPassed {
		return false
	}
	return true
}

// Enrollment est l'inscription d'un apprenant à une session.
type Enrollment struct {
	ddb.Record

	ID        string `dynamodbav:"id" json:"id"`
	OrgID     string `dynamodbav:"orgId" json:"orgId"`
	SessionID string `dynamodbav:"sessionId" json:"sessionId"`
	ContactID string `dynamodbav:"contactId" json:"contactId"`
	// FileID relie l'inscription au dossier, donc à sa chaîne de preuve.
	FileID string `dynamodbav:"fileId,omitempty" json:"fileId,omitempty"`

	Status      EnrollmentStatus `dynamodbav:"status" json:"status"`
	EnrolledAt  time.Time        `dynamodbav:"enrolledAt" json:"enrolledAt"`
	StartedAt   *time.Time       `dynamodbav:"startedAt,omitempty" json:"startedAt,omitempty"`
	CompletedAt *time.Time       `dynamodbav:"completedAt,omitempty" json:"completedAt,omitempty"`

	Progress []ModuleProgress `dynamodbav:"progress,omitempty" json:"progress,omitempty"`

	// PositioningDone et FinalPassed portent les évaluations d'entrée et de
	// sortie, les deux bornes exigées en audit.
	PositioningDone bool `dynamodbav:"positioningDone" json:"positioningDone"`
	FinalPassed     bool `dynamodbav:"finalPassed" json:"finalPassed"`
	FinalPercent    int  `dynamodbav:"finalPercent" json:"finalPercent"`

	CertificateKey string `dynamodbav:"certificateKey,omitempty" json:"-"`
}

// NewEnrollment construit une inscription.
func NewEnrollment(orgID, sessionID, contactID string, startsAt, now time.Time) Enrollment {
	return Enrollment{
		Record: ddb.Record{
			PK: ddb.OrgPK(orgID), SK: ddb.EnrollmentSK(sessionID, contactID),
			GSI1PK:    ddb.GSI1Enrollments(orgID, contactID),
			GSI1SK:    startsAt.UTC().Format(time.RFC3339Nano),
			Type:      "enrollment",
			CreatedAt: now, UpdatedAt: now,
		},
		ID: sessionID + ":" + contactID, OrgID: orgID,
		SessionID: sessionID, ContactID: contactID,
		Status: StatusEnrolled, EnrolledAt: now,
	}
}

// ProgressFor renvoie l'avancement d'un module, en le créant au besoin.
func (e *Enrollment) ProgressFor(moduleID string) *ModuleProgress {
	for i := range e.Progress {
		if e.Progress[i].ModuleID == moduleID {
			return &e.Progress[i]
		}
	}
	e.Progress = append(e.Progress, ModuleProgress{ModuleID: moduleID})
	return &e.Progress[len(e.Progress)-1]
}

// CompletionPercent est l'avancement global, en pourcentage de modules validés.
func (e Enrollment) CompletionPercent(modules []Module) int {
	if len(modules) == 0 {
		return 0
	}
	done := 0
	for _, module := range modules {
		for _, progress := range e.Progress {
			if progress.ModuleID == module.ID && progress.CompletedAt != nil {
				done++
				break
			}
		}
	}
	return done * 100 / len(modules)
}

// Certifiable indique si l'attestation peut être délivrée.
//
// Trois conditions, et pas une de moins : tous les modules validés,
// l'évaluation d'entrée passée, l'évaluation finale réussie. Délivrer une
// attestation sans l'une d'elles, c'est produire un document qu'un contrôle
// invalidera.
func (e Enrollment) Certifiable(modules []Module, course Course) error {
	for _, module := range modules {
		var progress *ModuleProgress
		for i := range e.Progress {
			if e.Progress[i].ModuleID == module.ID {
				progress = &e.Progress[i]
				break
			}
		}
		if progress == nil || progress.CompletedAt == nil {
			return fmt.Errorf("le module « %s » n'est pas validé", module.Title)
		}
	}
	if course.PositioningQuizID != "" && !e.PositioningDone {
		return fmt.Errorf("l'évaluation de positionnement n'a pas été passée")
	}
	if course.FinalQuizID != "" && !e.FinalPassed {
		return fmt.Errorf("l'évaluation finale n'est pas réussie")
	}
	return nil
}
