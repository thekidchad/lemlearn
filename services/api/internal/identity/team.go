package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lemlearn/api/internal/platform/ddb"
)

// L'équipe d'un organisme.
//
// Un organisme n'était jusqu'ici qu'un seul compte : le propriétaire, et
// personne d'autre. Aucune assistante, aucun formateur, personne à qui confier
// un dossier ni à qui faire contresigner une feuille d'émargement. Pour un
// produit qui prétend outiller un organisme de formation, c'était le manque
// qui en cachait dix autres.

// ErrLastOwner refuse de retirer le dernier propriétaire.
var ErrLastOwner = errors.New("un organisme doit garder au moins un propriétaire actif")

// ErrNeverActivated distingue un accès suspendu d'un accès jamais ouvert.
var ErrNeverActivated = errors.New(
	"ce compte n'a jamais choisi son mot de passe : renvoyez-lui une invitation")

// InviteTeamInput décrit l'ouverture d'un accès à un collègue.
type InviteTeamInput struct {
	OrgID     string
	Email     string
	FirstName string
	LastName  string
	Role      Role
}

// InviteTeamMember ouvre un accès et rend le jeton de l'invitation.
//
// Le compte est créé désactivé : il n'aura d'empreinte de mot de passe que
// lorsque l'intéressé en aura choisi un. Nous n'en fabriquons jamais — un
// secret que nous aurions inventé et envoyé par courriel resterait dans sa
// boîte pour toujours.
func (s *Service) InviteTeamMember(ctx context.Context, in InviteTeamInput) (User, string, error) {
	email := ddb.NormalizeEmail(in.Email)
	if !strings.Contains(email, "@") {
		return User{}, "", fmt.Errorf("une adresse de courriel est nécessaire")
	}
	switch in.Role {
	case RoleOwner, RoleAdmin, RoleTrainer:
	default:
		return User{}, "", fmt.Errorf("rôle %q inconnu pour un collaborateur", in.Role)
	}

	now := s.now()

	// Une adresse déjà connue reprend son compte plutôt que d'en créer un
	// second : deux comptes pour une personne se disputeraient la réservation
	// d'adresse, et l'un des deux serait inaccessible.
	existing, err := ddb.Get[EmailPointer](ctx, s.db, ddb.EmailPointerPK(email), ddb.EmailPointerSK)
	switch {
	case err == nil && existing.OrgID != in.OrgID:
		return User{}, "", fmt.Errorf("cette adresse est déjà utilisée par un autre organisme")
	case err == nil:
		user, err := ddb.Get[User](ctx, s.db, ddb.OrgPK(existing.OrgID), ddb.UserSK(existing.UserID))
		if err != nil {
			return User{}, "", err
		}
		if user.Role == RoleLearner {
			return User{}, "", fmt.Errorf(
				"cette adresse est celle d'un stagiaire : un même compte ne peut pas être des deux côtés du bureau")
		}
		token, hash, err := NewToken()
		if err != nil {
			return User{}, "", err
		}
		user.Role = in.Role
		user.UpdatedAt = now
		invitation := NewInvitation(hash, in.OrgID, user.ID, "", email, now)
		if err := s.db.Write(ctx, []ddb.Write{{Item: user}, {Item: invitation}}); err != nil {
			return User{}, "", err
		}
		return user, token, nil
	case !errors.Is(err, ddb.ErrNotFound):
		return User{}, "", err
	}

	user := NewUser(in.OrgID, email, in.FirstName, in.LastName, in.Role, "", now)
	user.Disabled = true

	token, hash, err := NewToken()
	if err != nil {
		return User{}, "", err
	}
	invitation := NewInvitation(hash, in.OrgID, user.ID, "", email, now)

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

// UpdateTeamMember change le rôle d'un collaborateur ou suspend son accès.
//
// Le dernier propriétaire actif est protégé dans les deux sens : le rétrograder
// comme le suspendre laisserait un organisme sans personne pour gérer ses
// accès, et il faudrait alors nous appeler pour rentrer chez soi.
func (s *Service) UpdateTeamMember(
	ctx context.Context, orgID, userID string, role *Role, disabled *bool,
) (User, error) {
	user, err := ddb.Get[User](ctx, s.db, ddb.OrgPK(orgID), ddb.UserSK(userID))
	if err != nil {
		return User{}, err
	}
	if user.Role == RoleLearner {
		return User{}, fmt.Errorf("ce compte est celui d'un stagiaire : son accès se gère depuis sa fiche")
	}

	apres := user
	if role != nil {
		switch *role {
		case RoleOwner, RoleAdmin, RoleTrainer:
			apres.Role = *role
		default:
			return User{}, fmt.Errorf("rôle %q inconnu pour un collaborateur", *role)
		}
	}
	if disabled != nil {
		// Rétablir un compte qui n'a jamais choisi son mot de passe le
		// marquerait actif sans le rendre utilisable : la connexion échouerait
		// contre une empreinte vide, et personne ne comprendrait pourquoi. Ce
		// compte-là se relance, il ne se rétablit pas.
		if !*disabled && user.PasswordHash == "" {
			return User{}, ErrNeverActivated
		}
		apres.Disabled = *disabled
	}

	// On ne compte les propriétaires que si le changement en retire un.
	perdOwner := user.Role == RoleOwner && !user.Disabled &&
		(apres.Role != RoleOwner || apres.Disabled)
	if perdOwner {
		autres, err := s.RawUsers(ctx, orgID)
		if err != nil {
			return User{}, err
		}
		restants := 0
		for _, autre := range autres {
			if autre.ID != userID && autre.Role == RoleOwner && !autre.Disabled {
				restants++
			}
		}
		if restants == 0 {
			return User{}, ErrLastOwner
		}
	}

	apres.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, apres); err != nil {
		return User{}, err
	}
	return apres, nil
}

// TeamMembers liste les comptes de l'organisme, stagiaires exclus.
func (s *Service) TeamMembers(ctx context.Context, orgID string) ([]User, error) {
	users, err := s.RawUsers(ctx, orgID)
	if err != nil {
		return nil, err
	}
	equipe := make([]User, 0, len(users))
	for _, user := range users {
		if user.Role != RoleLearner {
			equipe = append(equipe, user)
		}
	}
	return equipe, nil
}
