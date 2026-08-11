package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// Service porte les cas d'usage de l'identité.
type Service struct {
	db  *ddb.Client
	now func() time.Time
}

// NewService construit le service. `now` est injectable pour les tests.
func NewService(db *ddb.Client, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{db: db, now: now}
}

// RegisterInput décrit la création d'une organisation et de son propriétaire.
type RegisterInput struct {
	OrgName   string
	Email     string
	Password  string
	FirstName string
	LastName  string
	IP        string
	UserAgent string
}

// Register crée l'organisation, son compte propriétaire et la réservation
// d'e-mail — le tout dans une seule transaction, avec l'événement d'audit
// fondateur de l'organisation.
//
// Si l'adresse est déjà prise, rien n'est écrit : pas d'organisation orpheline
// à nettoyer plus tard.
func (s *Service) Register(ctx context.Context, in RegisterInput) (Org, User, error) {
	in.Email = ddb.NormalizeEmail(in.Email)
	if err := validateRegister(in); err != nil {
		return Org{}, User{}, err
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return Org{}, User{}, err
	}

	now := s.now()
	org := NewOrg(strings.TrimSpace(in.OrgName), now)
	user := NewUser(org.ID, in.Email, in.FirstName, in.LastName, RoleOwner, hash, now)
	pointer := NewEmailPointer(in.Email, org.ID, user.ID, now)

	_, err = s.db.WriteWithAudit(ctx, "org/"+org.ID,
		[]ddb.Write{
			{Item: org, Condition: "attribute_not_exists(PK)"},
			{Item: user},
			// La seule condition qui compte : l'adresse ne doit pas être
			// déjà réservée. Elle fait échouer toute la transaction.
			{Item: pointer, Condition: "attribute_not_exists(PK)"},
		},
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, "org/"+org.ID, now, audit.ActionFileCreated,
				audit.Actor{
					Type: audit.ActorUser, ID: user.ID, Label: user.FullName(),
					IP: in.IP, UserAgent: in.UserAgent,
				},
				map[string]any{
					"org":   org.Name,
					"owner": user.Email,
					"plan":  org.Plan,
				})
		})
	if err != nil {
		if errors.Is(err, ddb.ErrConflict) {
			return Org{}, User{}, ErrEmailTaken
		}
		return Org{}, User{}, err
	}

	return org, user, nil
}

// Login vérifie les identifiants et ouvre une session.
//
// Le jeton en clair n'est renvoyé qu'ici, une seule fois : il part dans un
// cookie httpOnly et n'est plus jamais consultable.
func (s *Service) Login(ctx context.Context, email, password, ip, userAgent string) (User, string, error) {
	email = ddb.NormalizeEmail(email)

	pointer, err := ddb.Get[EmailPointer](ctx, s.db, ddb.EmailPointerPK(email), ddb.EmailPointerSK)
	if err != nil {
		if errors.Is(err, ddb.ErrNotFound) {
			// Le mot de passe est tout de même vérifié contre une empreinte
			// factice : sans cela, un e-mail inconnu répondrait bien plus
			// vite qu'un mot de passe faux, ce qui permet d'énumérer les
			// comptes existants.
			VerifyPassword(password, decoyHash)
			return User{}, "", ErrInvalidCredentials
		}
		return User{}, "", err
	}

	user, err := ddb.Get[User](ctx, s.db, ddb.OrgPK(pointer.OrgID), ddb.UserSK(pointer.UserID))
	if err != nil {
		if errors.Is(err, ddb.ErrNotFound) {
			return User{}, "", ErrInvalidCredentials
		}
		return User{}, "", err
	}
	if !VerifyPassword(password, user.PasswordHash) {
		return User{}, "", ErrInvalidCredentials
	}
	if user.Disabled {
		return User{}, "", ErrDisabled
	}

	token, hash, err := NewToken()
	if err != nil {
		return User{}, "", err
	}

	now := s.now()
	session := NewSession(hash, user, ip, userAgent, "", now)
	if err := ddb.Put(ctx, s.db, session); err != nil {
		return User{}, "", err
	}

	user.LastLoginAt = &now
	user.UpdatedAt = now
	if err := ddb.Put(ctx, s.db, user); err != nil {
		return User{}, "", err
	}

	return user, token, nil
}

// Authenticate résout un jeton de session en session valide.
func (s *Service) Authenticate(ctx context.Context, token string) (Session, error) {
	if strings.TrimSpace(token) == "" {
		return Session{}, ErrInvalidCredentials
	}

	session, err := ddb.Get[Session](ctx, s.db, ddb.AuthSessionPK(HashToken(token)), ddb.AuthSessionSK)
	if err != nil {
		if errors.Is(err, ddb.ErrNotFound) {
			return Session{}, ErrInvalidCredentials
		}
		return Session{}, err
	}
	if session.Expired(s.now()) {
		return Session{}, ErrInvalidCredentials
	}
	return session, nil
}

// Logout révoque une session.
func (s *Service) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return ddb.Delete(ctx, s.db, ddb.AuthSessionPK(HashToken(token)), ddb.AuthSessionSK)
}

// LoadUser relit un utilisateur depuis sa session.
func (s *Service) LoadUser(ctx context.Context, session Session) (User, error) {
	return ddb.Get[User](ctx, s.db, ddb.OrgPK(session.OrgID), ddb.UserSK(session.UserID))
}

// LoadOrg relit l'organisation d'une session.
func (s *Service) LoadOrg(ctx context.Context, orgID string) (Org, error) {
	return ddb.Get[Org](ctx, s.db, ddb.OrgPK(orgID), ddb.OrgMetaSK)
}

func validateRegister(in RegisterInput) error {
	if strings.TrimSpace(in.OrgName) == "" {
		return fmt.Errorf("le nom de l'organisme est obligatoire")
	}
	if !strings.Contains(in.Email, "@") || strings.HasPrefix(in.Email, "@") || strings.HasSuffix(in.Email, "@") {
		return fmt.Errorf("adresse e-mail invalide")
	}
	if strings.TrimSpace(in.LastName) == "" {
		return fmt.Errorf("le nom du contact est obligatoire")
	}
	return nil
}

// decoyHash est l'empreinte d'un mot de passe qui n'est celui de personne.
// Elle sert uniquement à faire payer à un e-mail inconnu le même coût de
// calcul qu'à un compte existant.
const decoyHash = "$argon2id$v=19$m=19456,t=2,p=1$" +
	"c2VsZGVsZXVycmVwb3VybGVz$" +
	"ZW1wcmVpbnRlZGVjb21wYXJhaXNvbmNvbnN0YW50ZQ"
