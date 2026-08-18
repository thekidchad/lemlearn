// Package followup programme les questionnaires de satisfaction à froid.
//
// Qualiopi demande une mesure de satisfaction à distance de la formation —
// trois à six mois — et c'est l'indicateur que les organismes oublient le plus
// souvent, précisément parce qu'il tombe longtemps après que tout le monde est
// passé à autre chose. Le programmer à la clôture de la session est la seule
// façon de ne pas s'en remettre à la mémoire de quelqu'un.
package followup

import (
	"context"
	"fmt"
	"time"

	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// Delay est le décalage par défaut : trois mois après la fin de la session.
//
// Le bas de la fourchette réglementaire, à dessein : plus on attend, moins les
// apprenants répondent, et un taux de retour famélique est un indicateur aussi
// embarrassant qu'une absence de mesure.
const Delay = 90 * 24 * time.Hour

// Status est l'état d'une relance.
type Status string

const (
	StatusPlanned   Status = "planned"
	StatusSent      Status = "sent"
	StatusAnswered  Status = "answered"
	StatusCancelled Status = "cancelled"
)

// Task est une relance programmée.
//
// Les tâches sont rangées par mois d'échéance : le traitement quotidien
// n'interroge que la partition du mois courant, jamais la table entière. Un
// balayage complet deviendrait le premier poste de coût le jour où
// l'organisation compte quelques milliers de dossiers.
type Task struct {
	ddb.Record

	OrgID       string `dynamodbav:"orgId" json:"orgId"`
	SessionID   string `dynamodbav:"sessionId" json:"sessionId"`
	ContactID   string `dynamodbav:"contactId" json:"contactId"`
	FileID      string `dynamodbav:"fileId,omitempty" json:"fileId,omitempty"`
	QuizID      string `dynamodbav:"quizId" json:"quizId"`
	Email       string `dynamodbav:"email" json:"email"`
	LearnerName string `dynamodbav:"learnerName" json:"learnerName"`
	CourseTitle string `dynamodbav:"courseTitle" json:"courseTitle"`

	DueAt time.Time `dynamodbav:"dueAt" json:"dueAt"`
	// TokenHash est l'empreinte du jeton posé dans le courriel. Le jeton lui
	// -même n'est jamais stocké : une fuite de la table ne donnerait pas accès
	// aux questionnaires.
	TokenHash string     `dynamodbav:"tokenHash,omitempty" json:"-"`
	Status    Status     `dynamodbav:"status" json:"status"`
	SentAt    *time.Time `dynamodbav:"sentAt,omitempty" json:"sentAt,omitempty"`
	Reminders int        `dynamodbav:"reminders" json:"reminders"`
}

// SchedulePK range les relances par mois d'échéance.
func SchedulePK(due time.Time) string {
	return "SCHEDULE#" + due.UTC().Format("2006-01")
}

// ScheduleSK ordonne les relances d'un mois par jour puis par inscription.
func ScheduleSK(due time.Time, sessionID, contactID string) string {
	return fmt.Sprintf("%s#%s#%s", due.UTC().Format("2006-01-02"), sessionID, contactID)
}

// pointer résout un jeton de relance vers sa tâche.
//
// Le pointeur est un article à part, comme pour les liens de signature :
// interroger la table par empreinte de jeton sans index secondaire suppose
// que le jeton soit la clé de partition.
type pointer struct {
	ddb.Record

	OrgID  string `dynamodbav:"orgId"`
	TaskPK string `dynamodbav:"taskPk"`
	TaskSK string `dynamodbav:"taskSk"`
}

// Mailer envoie la relance.
type Mailer interface {
	Send(ctx context.Context, to, subject, html string) error
}

// Service programme et traite les relances.
type Service struct {
	db     *ddb.Client
	mailer Mailer
	appURL string
	now    func() time.Time
}

// NewService construit le service.
func NewService(db *ddb.Client, mailer Mailer, appURL string, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{db: db, mailer: mailer, appURL: appURL, now: now}
}

// ScheduleInput décrit une relance à programmer.
type ScheduleInput struct {
	OrgID       string
	SessionID   string
	ContactID   string
	FileID      string
	QuizID      string
	Email       string
	LearnerName string
	CourseTitle string
	EndsAt      time.Time
}

// Schedule programme la relance d'un apprenant.
//
// Sans adresse, la relance n'est pas programmée : une tâche qui ne peut pas
// aboutir encombrerait le traitement quotidien sans rien produire.
func (s *Service) Schedule(ctx context.Context, in ScheduleInput) (Task, error) {
	if in.Email == "" {
		return Task{}, fmt.Errorf("aucune adresse pour relancer %s", in.LearnerName)
	}
	if in.QuizID == "" {
		return Task{}, fmt.Errorf("aucun questionnaire de satisfaction à froid n'est configuré")
	}

	due := in.EndsAt.Add(Delay)
	now := s.now()
	task := Task{
		Record: ddb.Record{
			PK: SchedulePK(due), SK: ScheduleSK(due, in.SessionID, in.ContactID),
			Type: "followup", CreatedAt: now, UpdatedAt: now,
		},
		OrgID: in.OrgID, SessionID: in.SessionID, ContactID: in.ContactID,
		FileID: in.FileID, QuizID: in.QuizID, Email: in.Email,
		LearnerName: in.LearnerName, CourseTitle: in.CourseTitle,
		DueAt: due, Status: StatusPlanned,
	}

	// Écriture sous condition : reprogrammer une relance déjà planifiée
	// enverrait deux questionnaires à la même personne.
	if err := ddb.PutNew(ctx, s.db, task); err != nil {
		return Task{}, err
	}
	return task, nil
}

// Due renvoie les relances échues à la date indiquée, non encore envoyées.
func (s *Service) Due(ctx context.Context, at time.Time) ([]Task, error) {
	// Le mois courant et le précédent : une relance dont l'échéance tombait
	// fin de mois et qu'une panne aurait fait manquer doit être rattrapée,
	// pas perdue.
	var due []Task
	for _, month := range []time.Time{at, at.AddDate(0, -1, 0)} {
		tasks, err := ddb.Query[Task](ctx, s.db, ddb.QuerySpec{PK: SchedulePK(month)})
		if err != nil {
			return nil, err
		}
		for _, task := range tasks {
			if task.Status == StatusPlanned && !task.DueAt.After(at) {
				due = append(due, task)
			}
		}
	}
	return due, nil
}

// Run traite les relances échues et renvoie le nombre d'envois.
func (s *Service) Run(ctx context.Context, at time.Time) (sent int, failed int, err error) {
	tasks, err := s.Due(ctx, at)
	if err != nil {
		return 0, 0, err
	}

	for _, task := range tasks {
		// Le jeton n'est tiré qu'à l'envoi : l'apprenant répond trois mois
		// après la formation, sans compte ni mot de passe à retrouver, et un
		// lien qui aurait dormi trois mois dans la table serait un secret de
		// plus à protéger pour rien.
		token, hash, err := identity.NewToken()
		if err != nil {
			failed++
			continue
		}
		link := s.appURL + "/satisfaction/" + token
		if s.mailer != nil {
			if err := s.mailer.Send(ctx, task.Email,
				fmt.Sprintf("Votre avis sur « %s », trois mois après", task.CourseTitle),
				coldSurveyEmail(task, link)); err != nil {
				failed++
				continue
			}
		}

		now := s.now()
		task.Status = StatusSent
		task.SentAt = &now
		task.UpdatedAt = now
		task.TokenHash = hash

		// Le lien vaut trois mois : au-delà, la réponse n'a plus de valeur
		// d'indicateur, et le pointeur s'efface de lui-même.
		resolve := pointer{
			Record: ddb.Record{
				PK: ddb.SurveyTokenPK(hash), SK: ddb.SurveyTokenSK,
				Type: "followup_token", CreatedAt: now, UpdatedAt: now,
				ExpiresAt: ddb.TTL(now.Add(90 * 24 * time.Hour)),
			},
			OrgID: task.OrgID, TaskPK: task.PK, TaskSK: task.SK,
		}

		// L'envoi est journalisé sur le dossier : le taux de retour de la
		// satisfaction à froid est un indicateur audité, et il faut pouvoir
		// montrer que la sollicitation a bien eu lieu.
		if task.FileID != "" {
			if _, err := s.db.WriteWithAudit(ctx, "file/"+task.FileID,
				[]ddb.Write{{Item: task}, {Item: resolve}},
				func(prev audit.Event) (audit.Event, error) {
					return audit.Append(prev, "file/"+task.FileID, now,
						audit.ActionDocumentSent,
						audit.Actor{Type: audit.ActorSystem, ID: "relance-satisfaction"},
						map[string]any{
							"type":         "satisfaction à froid",
							"quizId":       task.QuizID,
							"destinataire": task.Email,
							"echeance":     task.DueAt.Format(time.RFC3339),
						})
				}); err != nil {
				failed++
				continue
			}
		} else if err := ddb.Put(ctx, s.db, task); err != nil {
			failed++
			continue
		} else if err := ddb.Put(ctx, s.db, resolve); err != nil {
			failed++
			continue
		}
		sent++
	}

	return sent, failed, nil
}

// Resolve retrouve la relance désignée par un jeton.
//
// Le jeton clair n'est jamais comparé à autre chose qu'à son empreinte : c'est
// la même règle que pour les liens de signature.
func (s *Service) Resolve(ctx context.Context, token string) (Task, error) {
	if token == "" {
		return Task{}, fmt.Errorf("lien de questionnaire invalide")
	}

	hash := identity.HashToken(token)
	found, err := ddb.Get[pointer](ctx, s.db, ddb.SurveyTokenPK(hash), ddb.SurveyTokenSK)
	if err != nil {
		return Task{}, fmt.Errorf("ce lien n'est plus valable")
	}

	task, err := ddb.Get[Task](ctx, s.db, found.TaskPK, found.TaskSK)
	if err != nil {
		return Task{}, err
	}
	// Comparaison de l'empreinte : le pointeur pourrait survivre à une tâche
	// reprogrammée, et resservirait alors un lien périmé.
	if task.TokenHash != hash {
		return Task{}, fmt.Errorf("ce lien n'est plus valable")
	}
	if task.Status == StatusCancelled {
		return Task{}, fmt.Errorf("ce questionnaire a été annulé")
	}
	return task, nil
}

// Answered marque la relance comme honorée.
//
// Le taux de retour de la satisfaction à froid est un indicateur audité :
// c'est ici qu'il se compte.
func (s *Service) Answered(ctx context.Context, task Task) error {
	if task.Status == StatusAnswered {
		return nil
	}
	task.Status = StatusAnswered
	task.UpdatedAt = s.now()
	return ddb.Put(ctx, s.db, task)
}

// Cancel annule une relance — abandon en cours de formation, anonymisation.
func (s *Service) Cancel(ctx context.Context, due time.Time, sessionID, contactID string) error {
	task, err := ddb.Get[Task](ctx, s.db, SchedulePK(due), ScheduleSK(due, sessionID, contactID))
	if err != nil {
		return err
	}
	task.Status = StatusCancelled
	task.UpdatedAt = s.now()
	return ddb.Put(ctx, s.db, task)
}
