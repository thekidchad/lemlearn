// Package learning relie l'assiduité, les questionnaires et la progression.
//
// Il existe parce que la règle centrale du produit traverse trois domaines :
// un module n'est validé que si l'apprenant a réellement vu la vidéo *et*
// réussi son contrôle. Laisser cette règle à `lms` ou à `quiz` obligerait l'un
// à connaître l'autre ; la mettre ici les garde tous deux ignorants et
// testables séparément.
package learning

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/lms"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
	"github.com/lemlearn/api/internal/quiz"
)

// WatchRecord persiste la couverture d'un apprenant sur un module.
//
// Un élément par couple inscription-module, en dehors de l'inscription
// elle-même : le bitmap est réécrit à chaque signal, et le faire porter par
// l'inscription provoquerait des écritures concurrentes sur le même élément
// dès qu'un apprenant regarde deux modules à la suite.
type WatchRecord struct {
	ddb.Record

	OrgID        string `dynamodbav:"orgId"`
	EnrollmentID string `dynamodbav:"enrollmentId"`
	ModuleID     string `dynamodbav:"moduleId"`

	lms.Coverage
}

// Service porte les cas d'usage du suivi pédagogique.
type Service struct {
	db      *ddb.Client
	catalog *catalog.Service
	now     func() time.Time
}

// NewService construit le service.
func NewService(db *ddb.Client, cat *catalog.Service, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{db: db, catalog: cat, now: now}
}

// Target désigne un apprenant sur un module.
type Target struct {
	OrgID     string
	SessionID string
	ContactID string
	CourseID  string
	ModuleID  string
}

func (t Target) enrollmentID() string { return t.SessionID + ":" + t.ContactID }

// Heartbeat intègre un signal du lecteur vidéo et renvoie la couverture à jour.
//
// L'heure du signal est celle du serveur, jamais celle du client : c'est elle
// qui borne ce qu'il peut prétendre avoir joué.
func (s *Service) Heartbeat(ctx context.Context, target Target, beat lms.Beat) (lms.Coverage, bool, error) {
	module, err := s.catalog.GetModule(ctx, target.OrgID, target.CourseID, target.ModuleID)
	if err != nil {
		return lms.Coverage{}, false, fmt.Errorf("module introuvable: %w", err)
	}

	record, err := s.loadWatch(ctx, target, module)
	if err != nil {
		return lms.Coverage{}, false, err
	}

	beat.At = s.now()
	accepted, reason := record.Coverage.Apply(beat)

	record.UpdatedAt = beat.At
	if err := ddb.Put(ctx, s.db, record); err != nil {
		return lms.Coverage{}, false, err
	}
	if !accepted {
		// Le signal refusé est tout de même persisté : le compteur de refus
		// fait partie du dossier de preuve, et un apprenant dont la moitié
		// des signaux sont écartés mérite un regard.
		return record.Coverage, false, fmt.Errorf("signal écarté : %s", reason)
	}

	if err := s.refreshProgress(ctx, target, module, &record.Coverage, nil); err != nil {
		return record.Coverage, true, err
	}
	return record.Coverage, true, nil
}

// Coverage relit le relevé d'un module.
func (s *Service) Coverage(ctx context.Context, target Target) (lms.Coverage, error) {
	record, err := ddb.Get[WatchRecord](ctx, s.db,
		ddb.OrgPK(target.OrgID), ddb.WatchSK(target.enrollmentID(), target.ModuleID))
	if err != nil {
		return lms.Coverage{}, err
	}
	return record.Coverage, nil
}

func (s *Service) loadWatch(ctx context.Context, target Target, module catalog.Module) (WatchRecord, error) {
	record, err := ddb.Get[WatchRecord](ctx, s.db,
		ddb.OrgPK(target.OrgID), ddb.WatchSK(target.enrollmentID(), target.ModuleID))
	if err == nil {
		// La durée peut avoir changé si la vidéo a été ré-encodée : on
		// conserve le bitmap, qui sera étendu au besoin par Apply.
		record.Coverage.DurationMs = module.DurationMs
		return record, nil
	}
	if !errors.Is(err, ddb.ErrNotFound) {
		return WatchRecord{}, err
	}

	now := s.now()
	return WatchRecord{
		Record: ddb.Record{
			PK: ddb.OrgPK(target.OrgID), SK: ddb.WatchSK(target.enrollmentID(), target.ModuleID),
			Type: "watch", CreatedAt: now, UpdatedAt: now,
		},
		OrgID: target.OrgID, EnrollmentID: target.enrollmentID(), ModuleID: target.ModuleID,
		Coverage: lms.NewCoverage(module.DurationMs),
	}, nil
}

// SubmitQuiz corrige une passation et met la progression à jour.
func (s *Service) SubmitQuiz(
	ctx context.Context,
	target Target,
	questionnaire quiz.Questionnaire,
	attempt quiz.Attempt,
	answers []quiz.Submitted,
	actor audit.Actor,
) (quiz.Attempt, error) {
	now := s.now()

	graded, err := quiz.Grade(questionnaire, attempt, answers, now)
	if err != nil {
		return quiz.Attempt{}, err
	}

	enrollment, err := s.catalog.GetEnrollment(ctx, target.OrgID, target.SessionID, target.ContactID)
	if err != nil {
		return quiz.Attempt{}, fmt.Errorf("inscription introuvable: %w", err)
	}

	writes := []ddb.Write{{Item: graded}}
	subject := "enrollment/" + target.enrollmentID()
	if enrollment.FileID != "" {
		// Quand le dossier est connu, la passation rejoint sa chaîne de
		// preuve plutôt que d'en ouvrir une parallèle.
		subject = "file/" + enrollment.FileID
	}

	// La progression est recalculée puis écrite dans la même transaction que
	// la passation : un score enregistré sans la progression qu'il débloque
	// laisserait un apprenant bloqué devant un module qu'il a réussi.
	var completedModule *catalog.Module
	switch questionnaire.Kind {
	case quiz.KindPostModule:
		module, err := s.catalog.GetModule(ctx, target.OrgID, target.CourseID, questionnaire.ModuleID)
		if err != nil {
			return quiz.Attempt{}, fmt.Errorf("module du contrôle introuvable: %w", err)
		}
		coverage, _ := s.Coverage(ctx, Target{
			OrgID: target.OrgID, SessionID: target.SessionID,
			ContactID: target.ContactID, ModuleID: module.ID,
		})
		if applyProgress(&enrollment, module, &coverage, &graded, now) {
			completedModule = &module
		}
	case quiz.KindPositioning:
		enrollment.PositioningDone = true
	case quiz.KindFinal:
		enrollment.FinalPassed = graded.Passed
		enrollment.FinalPercent = graded.Percent()
	}

	enrollment.UpdatedAt = now
	writes = append(writes, ddb.Write{Item: enrollment})

	// Deux faits distincts, une seule transaction : la soumission, puis — le
	// cas échéant — la validation du module qu'elle débloque. C'est le cas le
	// plus courant, la vidéo étant regardée avant le contrôle ; les séparer
	// laisserait une fenêtre où le module est validé sans que le journal le
	// dise.
	if _, err := s.db.WriteWithAuditChain(ctx, subject, writes,
		func(prev audit.Event) ([]audit.Event, error) {
			submitted, err := audit.Append(prev, subject, now, audit.ActionQuizSubmitted, actor,
				map[string]any{
					"quizId":      questionnaire.ID,
					"version":     questionnaire.Version,
					"kind":        string(questionnaire.Kind),
					"attempt":     graded.Number,
					"score":       graded.Score,
					"maxScore":    graded.MaxScore,
					"percent":     graded.Percent(),
					"passed":      graded.Passed,
					"durationMs":  graded.DurationMs,
					"answerCount": len(graded.Answers),
				})
			if err != nil {
				return nil, err
			}
			if completedModule == nil {
				return []audit.Event{submitted}, nil
			}

			progress := enrollment.ProgressFor(completedModule.ID)
			completed, err := audit.Append(submitted, subject, now, audit.ActionModuleCompleted,
				audit.Actor{Type: audit.ActorLearner, ID: target.ContactID},
				map[string]any{
					"moduleId":        completedModule.ID,
					"moduleTitle":     completedModule.Title,
					"coveragePercent": progress.CoveragePercent,
					"watchedMs":       progress.WatchedMs,
					"quizPercent":     progress.QuizPercent,
					"required":        completedModule.MinCoveragePercent,
				})
			if err != nil {
				return nil, err
			}
			return []audit.Event{submitted, completed}, nil
		}); err != nil {
		return quiz.Attempt{}, err
	}

	return graded, nil
}

// refreshProgress met à jour l'inscription après un visionnage.
func (s *Service) refreshProgress(
	ctx context.Context, target Target, module catalog.Module,
	coverage *lms.Coverage, attempt *quiz.Attempt,
) error {
	enrollment, err := s.catalog.GetEnrollment(ctx, target.OrgID, target.SessionID, target.ContactID)
	if err != nil {
		return fmt.Errorf("inscription introuvable: %w", err)
	}

	now := s.now()
	completed := applyProgress(&enrollment, module, coverage, attempt, now)

	if !completed {
		return s.catalog.SaveEnrollment(ctx, enrollment)
	}

	// La validation d'un module est un événement de preuve : elle atteste que
	// l'assiduité *et* les acquis sont réunis, ce qui est exactement ce que
	// vérifie un auditeur.
	subject := "enrollment/" + target.enrollmentID()
	if enrollment.FileID != "" {
		subject = "file/" + enrollment.FileID
	}
	progress := enrollment.ProgressFor(module.ID)

	_, err = s.db.WriteWithAudit(ctx, subject, []ddb.Write{{Item: enrollment}},
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, subject, now, audit.ActionModuleCompleted,
				audit.Actor{Type: audit.ActorLearner, ID: target.ContactID},
				map[string]any{
					"moduleId":        module.ID,
					"moduleTitle":     module.Title,
					"coveragePercent": progress.CoveragePercent,
					"watchedMs":       progress.WatchedMs,
					"quizPercent":     progress.QuizPercent,
					"required":        module.MinCoveragePercent,
				})
		})
	return err
}

// applyProgress reporte couverture et note sur l'inscription, et dit si le
// module vient d'être validé.
func applyProgress(
	enrollment *catalog.Enrollment, module catalog.Module,
	coverage *lms.Coverage, attempt *quiz.Attempt, now time.Time,
) bool {
	progress := enrollment.ProgressFor(module.ID)
	wasComplete := progress.CompletedAt != nil

	if coverage != nil && coverage.DurationMs > 0 {
		progress.CoveragePercent = coverage.Percent()
		progress.WatchedMs = coverage.WatchedMs
	}
	if attempt != nil {
		progress.QuizAttempts = attempt.Number
		progress.QuizScore = attempt.Score
		progress.QuizPercent = attempt.Percent()
		// Une réussite ne se perd pas : un apprenant qui repasse un contrôle
		// déjà réussi et le rate ne doit pas voir son module se réouvrir.
		progress.QuizPassed = progress.QuizPassed || attempt.Passed
	}

	if enrollment.Status == catalog.StatusEnrolled {
		enrollment.Status = catalog.StatusInPogress
		if enrollment.StartedAt == nil {
			started := now
			enrollment.StartedAt = &started
		}
	}

	if !wasComplete && progress.Complete(module) {
		completed := now
		progress.CompletedAt = &completed
		return true
	}
	return false
}
