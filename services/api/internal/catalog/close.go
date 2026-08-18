package catalog

import (
	"context"
	"fmt"
	"time"

	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// CloseSession clôt une session.
//
// La clôture n'est pas qu'un drapeau : c'est elle qui déclenche la satisfaction
// à froid, l'indicateur Qualiopi que les organismes oublient le plus souvent
// parce qu'il tombe trois mois après que tout le monde est passé à autre chose.
// La programmer ici, et pas dans un rappel d'agenda, est la seule façon qu'elle
// parte.
func (s *Service) CloseSession(ctx context.Context, orgID, sessionID string, actor audit.Actor) (Session, []Enrollment, error) {
	session, err := s.GetSession(ctx, orgID, sessionID)
	if err != nil {
		return Session{}, nil, err
	}
	if session.Closed {
		return Session{}, nil, fmt.Errorf("cette session est déjà clôturée")
	}

	enrollments, err := s.ListSessionEnrollments(ctx, orgID, sessionID)
	if err != nil {
		return Session{}, nil, err
	}

	now := s.now()
	session.Closed = true
	session.ClosedAt = &now
	session.UpdatedAt = now

	// La clôture est journalisée sur chaque dossier concerné : c'est le
	// dossier que l'auditeur ouvre, pas la session.
	subjects := make([]string, 0, len(enrollments))
	seen := make(map[string]bool, len(enrollments))
	for _, enrollment := range enrollments {
		if enrollment.FileID == "" || seen[enrollment.FileID] {
			continue
		}
		seen[enrollment.FileID] = true
		subjects = append(subjects, "file/"+enrollment.FileID)
	}

	if len(subjects) == 0 {
		if err := ddb.Put(ctx, s.db, session); err != nil {
			return Session{}, nil, err
		}
		return session, enrollments, nil
	}

	// La session ne s'écrit qu'une fois, avec le premier dossier : la répéter
	// dans chaque transaction n'ajouterait rien et ferait échouer la seconde.
	for i, subject := range subjects {
		var writes []ddb.Write
		if i == 0 {
			writes = []ddb.Write{{Item: session}}
		}
		if _, err := s.db.WriteWithAudit(ctx, subject, writes,
			func(prev audit.Event) (audit.Event, error) {
				return audit.Append(prev, subject, now, audit.ActionSessionClosed, actor,
					map[string]any{
						"sessionId": sessionID,
						"intitule":  session.Title,
						"finLe":     session.EndsAt.Format(time.RFC3339),
					})
			}); err != nil {
			return Session{}, nil, err
		}
	}

	return session, enrollments, nil
}
