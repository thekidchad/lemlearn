package identity

import (
	"context"
	"errors"
	"time"

	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// DirectoryPK est la partition qui liste les organisations clientes.
//
// Une partition dédiée plutôt qu'un index : la vue super-admin est le seul
// endroit du produit qui traverse les organisations, et un GSI sur la table
// principale coûterait une écriture supplémentaire à chaque mutation de
// n'importe quelle entité pour servir un écran consulté une fois par jour.
const DirectoryPK = "DIRECTORY#ORGS"

// DirectoryEntry est la fiche d'annuaire d'une organisation.
type DirectoryEntry struct {
	ddb.Record

	OrgID string `dynamodbav:"orgId" json:"orgId"`
	Name  string `dynamodbav:"name" json:"name"`
	Plan  string `dynamodbav:"plan" json:"plan"`
	Owner string `dynamodbav:"owner,omitempty" json:"owner,omitempty"`
}

// NewDirectoryEntry construit la fiche d'annuaire.
//
// La date de la fiche est celle de l'organisation, pas celle de son
// inscription à l'annuaire : « client depuis » doit dire depuis quand il est
// client, pas depuis quand nous savons le compter.
func NewDirectoryEntry(org Org, owner string, now time.Time) DirectoryEntry {
	created := org.CreatedAt
	if created.IsZero() {
		created = now
	}
	return DirectoryEntry{
		Record: ddb.Record{
			PK: DirectoryPK, SK: "ORG#" + org.ID, Type: "org_directory",
			CreatedAt: created, UpdatedAt: now,
		},
		OrgID: org.ID, Name: org.Name, Plan: org.Plan, Owner: owner,
	}
}

// EnsureDirectory inscrit l'organisation à l'annuaire si elle n'y est pas.
//
// Appelée à chaque connexion : une écriture conditionnelle qui échoue coûte
// une unité de capacité et évite d'avoir à migrer les organisations créées
// avant l'annuaire. Une organisation absente de l'annuaire n'est pas une
// anomalie tant que personne ne s'y est connecté.
func (s *Service) EnsureDirectory(ctx context.Context, orgID, owner string) error {
	org, err := s.LoadOrg(ctx, orgID)
	if err != nil {
		return err
	}

	entry := NewDirectoryEntry(org, owner, s.now())
	if err := ddb.PutNew(ctx, s.db, entry); err != nil {
		if errors.Is(err, ddb.ErrConflict) {
			return nil
		}
		return err
	}
	return nil
}

// SyncDirectory met la fiche d'annuaire à jour après un changement de plan ou
// de raison sociale.
func (s *Service) SyncDirectory(ctx context.Context, org Org) error {
	existing, err := ddb.Get[DirectoryEntry](ctx, s.db, DirectoryPK, "ORG#"+org.ID)
	if err != nil && !errors.Is(err, ddb.ErrNotFound) {
		return err
	}

	entry := NewDirectoryEntry(org, existing.Owner, s.now())
	if !existing.CreatedAt.IsZero() {
		entry.CreatedAt = existing.CreatedAt
	}
	return ddb.Put(ctx, s.db, entry)
}

// ListOrgs renvoie l'annuaire des organisations.
func (s *Service) ListOrgs(ctx context.Context) ([]DirectoryEntry, error) {
	return ddb.Query[DirectoryEntry](ctx, s.db, ddb.QuerySpec{PK: DirectoryPK})
}

// SetPlan change le plan d'une organisation.
func (s *Service) SetPlan(ctx context.Context, orgID, plan string) (Org, error) {
	org, err := s.LoadOrg(ctx, orgID)
	if err != nil {
		return Org{}, err
	}
	org.Plan = plan
	org.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, org); err != nil {
		return Org{}, err
	}
	if err := s.SyncDirectory(ctx, org); err != nil {
		return Org{}, err
	}
	return org, nil
}

// OpenSessionFor ouvre une session sur un compte, au nom d'un tiers.
//
// C'est le mécanisme d'impersonation : la session porte le nom de celui qui
// l'a ouverte, et ce champ est recopié dans chaque événement d'audit. Une
// impersonation ne peut donc pas être discrète, ce qui est le seul garde-fou
// qui tienne quand un accès total est techniquement nécessaire au support.
func (s *Service) OpenSessionFor(ctx context.Context, user User, by, ip, userAgent string) (string, error) {
	if by == "" {
		return "", errors.New("une impersonation doit nommer son auteur")
	}

	token, hash, err := NewToken()
	if err != nil {
		return "", err
	}
	if err := ddb.Put(ctx, s.db, NewSession(hash, user, ip, userAgent, by, s.now())); err != nil {
		return "", err
	}
	return token, nil
}

// FirstOwner renvoie le compte propriétaire d'une organisation.
func (s *Service) FirstOwner(ctx context.Context, orgID string) (User, error) {
	users, err := ddb.Query[User](ctx, s.db, ddb.QuerySpec{
		PK: ddb.OrgPK(orgID), SKPrefix: "USER#",
	})
	if err != nil {
		return User{}, err
	}
	for _, user := range users {
		if user.Role == RoleOwner && !user.Disabled {
			return user, nil
		}
	}
	for _, user := range users {
		if user.Role.CanManageCRM() && !user.Disabled {
			return user, nil
		}
	}
	return User{}, ddb.ErrNotFound
}

// AuditOrg journalise un fait qui concerne l'organisation elle-même —
// changement de formule, impersonation — sur sa propre chaîne.
//
// Ces faits n'appartiennent à aucun dossier : les rattacher à l'un d'eux
// mélangerait la relation commerciale et la preuve de formation, et fausserait
// l'export du dossier.
func (s *Service) AuditOrg(ctx context.Context, orgID string, action audit.Action,
	actor audit.Actor, payload map[string]any) (audit.Event, error) {
	subject := "org/" + orgID
	return s.db.WriteWithAudit(ctx, subject, nil,
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, subject, s.now(), action, actor, payload)
		})
}

// Promote change le rôle d'un compte et celui de la session qui vient de
// s'ouvrir.
//
// La session porte une copie du rôle — c'est ce qui évite une lecture du
// compte à chaque requête — donc promouvoir sans la mettre à jour laisserait
// l'intéressé avec ses anciens droits jusqu'à sa prochaine connexion, ce qui
// se lit comme une promotion qui n'a pas marché.
func (s *Service) Promote(ctx context.Context, user User, role Role, token string) (User, error) {
	if !role.Valid() {
		return User{}, errors.New("rôle inconnu")
	}

	user.Role = role
	user.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, user); err != nil {
		return User{}, err
	}

	if token != "" {
		session, err := ddb.Get[Session](ctx, s.db, ddb.AuthSessionPK(HashToken(token)), ddb.AuthSessionSK)
		if err != nil {
			return User{}, err
		}
		session.Role = role
		session.UpdatedAt = s.now()
		if err := ddb.Put(ctx, s.db, session); err != nil {
			return User{}, err
		}
	}
	return user, nil
}

// ListUsers renvoie les comptes d'une organisation.
func (s *Service) ListUsers(ctx context.Context, orgID string) ([]PublicUser, error) {
	users, err := ddb.Query[User](ctx, s.db, ddb.QuerySpec{
		PK: ddb.OrgPK(orgID), SKPrefix: "USER#",
	})
	if err != nil {
		return nil, err
	}
	public := make([]PublicUser, 0, len(users))
	for _, user := range users {
		public = append(public, user.Public())
	}
	return public, nil
}

// UserByEmail retrouve un compte par son adresse, toutes organisations
// confondues.
func (s *Service) UserByEmail(ctx context.Context, email string) (User, error) {
	pointer, err := ddb.Get[EmailPointer](ctx, s.db, ddb.EmailPointerPK(ddb.NormalizeEmail(email)), ddb.EmailPointerSK)
	if err != nil {
		return User{}, err
	}
	return ddb.Get[User](ctx, s.db, ddb.OrgPK(pointer.OrgID), ddb.UserSK(pointer.UserID))
}

// OrgTimeline relit la chaîne d'audit de l'organisation elle-même.
func (s *Service) OrgTimeline(ctx context.Context, orgID string, limit int) ([]audit.Event, error) {
	events, err := s.db.AuditChain(ctx, "org/"+orgID)
	if err != nil {
		return nil, err
	}
	// Le plus récent d'abord : c'est ce qu'on vient voir.
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}
