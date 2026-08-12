package learning

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/platform/ddb"
	"github.com/lemlearn/api/internal/quiz"
)

// SaveQuestionnaire écrit une version de questionnaire.
//
// Une version publiée n'est jamais réécrite : la modifier reviendrait à
// changer ce qui a été demandé à des apprenants qui l'ont déjà passée.
func (s *Service) SaveQuestionnaire(ctx context.Context, q quiz.Questionnaire, courseID string) (quiz.Questionnaire, error) {
	if err := q.Validate(); err != nil {
		return quiz.Questionnaire{}, err
	}

	// Un contrôle après module doit désigner un module qui existe. Sans cette
	// vérification, l'erreur n'apparaît qu'à la première soumission d'un
	// apprenant — c'est-à-dire au pire moment.
	if q.Kind == quiz.KindPostModule {
		if courseID == "" {
			return quiz.Questionnaire{}, fmt.Errorf(
				"un contrôle après module doit indiquer la formation de son module")
		}
		if _, err := s.catalog.GetModule(ctx, q.OrgID, courseID, q.ModuleID); err != nil {
			return quiz.Questionnaire{}, fmt.Errorf(
				"le module %s n'existe pas dans cette formation : créez le module avant son contrôle", q.ModuleID)
		}
	}

	existing, err := ddb.Get[quiz.Questionnaire](ctx, s.db, ddb.OrgPK(q.OrgID), ddb.QuizSKFor(q.ID, q.Version))
	if err == nil && existing.Published {
		return quiz.Questionnaire{}, fmt.Errorf(
			"la version %d est publiée : créez une nouvelle version plutôt que de la modifier", q.Version)
	}

	q.Reindex()
	q.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, q); err != nil {
		return quiz.Questionnaire{}, err
	}
	return q, nil
}

// PublishQuestionnaire fige une version.
func (s *Service) PublishQuestionnaire(ctx context.Context, orgID, quizID string, version int) (quiz.Questionnaire, error) {
	q, err := ddb.Get[quiz.Questionnaire](ctx, s.db, ddb.OrgPK(orgID), ddb.QuizSKFor(quizID, version))
	if err != nil {
		return quiz.Questionnaire{}, err
	}
	if err := q.Validate(); err != nil {
		return quiz.Questionnaire{}, err
	}

	now := s.now()
	q.Published = true
	q.PublishedAt = &now
	q.UpdatedAt = now
	if err := ddb.Put(ctx, s.db, q); err != nil {
		return quiz.Questionnaire{}, err
	}
	return q, nil
}

// LatestPublished renvoie la dernière version publiée d'un questionnaire.
//
// C'est celle que passera un nouvel apprenant. Les passations en cours
// continuent sur la leur : une version publiée pendant qu'un apprenant
// répond ne doit pas changer ses questions sous ses yeux.
func (s *Service) LatestPublished(ctx context.Context, orgID, quizID string) (quiz.Questionnaire, error) {
	versions, err := ddb.Query[quiz.Questionnaire](ctx, s.db, ddb.QuerySpec{
		PK: ddb.OrgPK(orgID), SKPrefix: "QUIZ#" + quizID + "#V", Descending: true,
	})
	if err != nil {
		return quiz.Questionnaire{}, err
	}
	for _, version := range versions {
		if version.Published {
			return version, nil
		}
	}
	return quiz.Questionnaire{}, ddb.ErrNotFound
}

// Version renvoie une version précise, publiée ou non.
func (s *Service) Version(ctx context.Context, orgID, quizID string, version int) (quiz.Questionnaire, error) {
	return ddb.Get[quiz.Questionnaire](ctx, s.db, ddb.OrgPK(orgID), ddb.QuizSKFor(quizID, version))
}

// NextAttemptNumber renvoie le rang de la prochaine passation.
func (s *Service) NextAttemptNumber(ctx context.Context, orgID, enrollmentID, quizID string) (int, error) {
	attempts, err := s.Attempts(ctx, orgID, enrollmentID, quizID)
	if err != nil {
		return 0, err
	}
	return len(attempts) + 1, nil
}

// Attempts renvoie les passations d'un apprenant sur un questionnaire.
func (s *Service) Attempts(ctx context.Context, orgID, enrollmentID, quizID string) ([]quiz.Attempt, error) {
	return ddb.Query[quiz.Attempt](ctx, s.db, ddb.QuerySpec{
		PK:       ddb.OrgPK(orgID),
		SKPrefix: fmt.Sprintf("ENR#%s#ATT#%s#", enrollmentID, quizID),
	})
}

// AllAttempts renvoie toutes les passations d'une inscription, tous
// questionnaires confondus — la matière du relevé d'évaluation.
func (s *Service) AllAttempts(ctx context.Context, orgID, enrollmentID string) ([]quiz.Attempt, error) {
	return ddb.Query[quiz.Attempt](ctx, s.db, ddb.QuerySpec{
		PK: ddb.OrgPK(orgID), SKPrefix: "ENR#" + enrollmentID + "#ATT#",
	})
}

// unusedTime garde l'import time utile si les signatures évoluent.
var _ = time.Time{}
var _ = strings.TrimSpace
