package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/platform/ddb"
)

// InvitationTTL borne la validité d'une invitation.
//
// Quatorze jours : une session de formation se prépare des semaines à
// l'avance, et un lien périmé avant le premier module obligerait l'organisme à
// tout renvoyer.
const InvitationTTL = 14 * 24 * time.Hour

// Invitation ouvre un compte à un apprenant.
//
// L'apprenant n'a pas de mot de passe tant qu'il n'en a pas choisi un :
// en fabriquer un à sa place obligerait à le lui transmettre, donc à le faire
// circuler en clair dans un courriel qui reste dans une boîte.
type Invitation struct {
	ddb.Record

	OrgID     string `dynamodbav:"orgId" json:"orgId"`
	UserID    string `dynamodbav:"userId" json:"userId"`
	ContactID string `dynamodbav:"contactId" json:"contactId"`
	Email     string `dynamodbav:"email" json:"email"`
	OrgName   string `dynamodbav:"orgName" json:"orgName"`
	Name      string `dynamodbav:"name" json:"name"`

	ExpiresOn time.Time  `dynamodbav:"expiresOn" json:"expiresAt"`
	UsedAt    *time.Time `dynamodbav:"usedAt,omitempty" json:"usedAt,omitempty"`
}

// NewInvitation construit l'article résolu par le jeton.
func NewInvitation(tokenHash, orgID, userID, contactID, email string, now time.Time) Invitation {
	expires := now.Add(InvitationTTL)
	return Invitation{
		Record: ddb.Record{
			PK: ddb.InvitationPK(tokenHash), SK: ddb.InvitationSK,
			Type: "invitation", CreatedAt: now, UpdatedAt: now,
			// Le jeton disparaît de lui-même un mois après son échéance : un
			// lien périmé ne doit pas rester indéfiniment résolvable.
			ExpiresAt: ddb.TTL(expires.Add(30 * 24 * time.Hour)),
		},
		OrgID: orgID, UserID: userID, ContactID: contactID,
		Email: email, ExpiresOn: expires,
	}
}

// Expired indique si l'invitation est périmée.
func (i Invitation) Expired(now time.Time) bool { return !now.Before(i.ExpiresOn) }

// InviteInput décrit l'ouverture d'un compte apprenant.
type InviteInput struct {
	OrgID     string
	ContactID string
	Email     string
	FirstName string
	LastName  string
}

// InviteLearner crée le compte de l'apprenant et renvoie le jeton du lien.
//
// Le compte est créé sans mot de passe et désactivé : tant que l'apprenant n'a
// pas ouvert son lien, il n'existe qu'en creux. C'est ce qui permet de
// réinviter sans se demander si quelqu'un a déjà répondu.
func (s *Service) InviteLearner(ctx context.Context, in InviteInput) (User, string, error) {
	email := ddb.NormalizeEmail(in.Email)
	if email == "" || !strings.Contains(email, "@") {
		return User{}, "", fmt.Errorf("cet apprenant n'a pas d'adresse de courriel : ajoutez-la à sa fiche")
	}

	now := s.now()

	// Une adresse déjà connue reprend son compte : le réinviter en créer un
	// second casserait la réservation d'adresse et laisserait deux fiches se
	// disputer la même personne.
	existing, err := ddb.Get[EmailPointer](ctx, s.db, ddb.EmailPointerPK(email), ddb.EmailPointerSK)
	switch {
	case err == nil && existing.OrgID != in.OrgID:
		return User{}, "", fmt.Errorf("cette adresse est déjà utilisée par un autre organisme")
	case err == nil:
		user, err := ddb.Get[User](ctx, s.db, ddb.OrgPK(existing.OrgID), ddb.UserSK(existing.UserID))
		if err != nil {
			return User{}, "", err
		}
		if user.Role != RoleLearner {
			return User{}, "", fmt.Errorf("cette adresse est celle d'un compte %s, pas d'un apprenant", user.Role)
		}
		token, hash, err := NewToken()
		if err != nil {
			return User{}, "", err
		}
		invitation := NewInvitation(hash, in.OrgID, user.ID, in.ContactID, email, now)
		if err := ddb.Put(ctx, s.db, invitation); err != nil {
			return User{}, "", err
		}
		return user, token, nil
	case !errors.Is(err, ddb.ErrNotFound):
		return User{}, "", err
	}

	user := NewUser(in.OrgID, email, in.FirstName, in.LastName, RoleLearner, "", now)
	user.ContactID = in.ContactID
	// Désactivé jusqu'à ce qu'un mot de passe soit choisi : un compte sans
	// empreinte accepterait n'importe quoi si la vérification changeait un
	// jour de forme.
	user.Disabled = true

	token, hash, err := NewToken()
	if err != nil {
		return User{}, "", err
	}
	invitation := NewInvitation(hash, in.OrgID, user.ID, in.ContactID, email, now)

	if err := s.db.Write(ctx, []ddb.Write{
		{Item: user},
		{Item: NewEmailPointer(email, in.OrgID, user.ID, now), Condition: "attribute_not_exists(PK)"},
		{Item: invitation},
	}); err != nil {
		if errors.Is(err, ddb.ErrConflict) {
			return User{}, "", ErrEmailTaken
		}
		return User{}, "", err
	}
	return user, token, nil
}

// ResolveInvitation retrouve l'invitation désignée par un jeton.
func (s *Service) ResolveInvitation(ctx context.Context, token string) (Invitation, error) {
	if strings.TrimSpace(token) == "" {
		return Invitation{}, ErrInvalidCredentials
	}

	invitation, err := ddb.Get[Invitation](ctx, s.db, ddb.InvitationPK(HashToken(token)), ddb.InvitationSK)
	if err != nil {
		if errors.Is(err, ddb.ErrNotFound) {
			return Invitation{}, fmt.Errorf("ce lien n'est plus valable")
		}
		return Invitation{}, err
	}
	if invitation.UsedAt != nil {
		return Invitation{}, fmt.Errorf("ce lien a déjà servi : connectez-vous avec votre mot de passe")
	}
	if invitation.Expired(s.now()) {
		return Invitation{}, fmt.Errorf("ce lien a expiré : demandez-en un nouveau à votre organisme")
	}
	return invitation, nil
}

// AcceptInvitation fixe le mot de passe et active le compte.
func (s *Service) AcceptInvitation(ctx context.Context, token, password string) (User, error) {
	invitation, err := s.ResolveInvitation(ctx, token)
	if err != nil {
		return User{}, err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}

	user, err := ddb.Get[User](ctx, s.db, ddb.OrgPK(invitation.OrgID), ddb.UserSK(invitation.UserID))
	if err != nil {
		return User{}, err
	}

	now := s.now()
	user.PasswordHash = hash
	user.Disabled = false
	user.ContactID = invitation.ContactID
	user.UpdatedAt = now

	invitation.UsedAt = &now
	invitation.UpdatedAt = now

	// Le compte et l'invitation basculent ensemble : un mot de passe posé sans
	// que le lien soit consommé le laisserait utilisable une seconde fois.
	if err := s.db.Write(ctx, []ddb.Write{{Item: user}, {Item: invitation}}); err != nil {
		return User{}, err
	}
	return user, nil
}

// ResolveInvitationUser retrouve le compte associé à une adresse.
//
// Sert à vérifier qu'une réinvitation reprend bien le même compte, plutôt
// qu'à faire confiance à ce que renvoie l'invitation.
func (s *Service) ResolveInvitationUser(ctx context.Context, orgID, email string) (User, error) {
	pointer, err := ddb.Get[EmailPointer](ctx, s.db, ddb.EmailPointerPK(ddb.NormalizeEmail(email)), ddb.EmailPointerSK)
	if err != nil {
		return User{}, err
	}
	if pointer.OrgID != orgID {
		return User{}, ddb.ErrNotFound
	}
	return ddb.Get[User](ctx, s.db, ddb.OrgPK(orgID), ddb.UserSK(pointer.UserID))
}
