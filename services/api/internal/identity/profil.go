package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lemlearn/api/internal/platform/ddb"
)

// Le compte, vu par celui à qui il appartient.
//
// Chacun doit pouvoir corriger son nom, changer son mot de passe et poser sa
// photo sans passer par un administrateur. Rien de tout cela n'existait : un
// prénom mal orthographié à l'invitation le restait, et le mot de passe choisi
// le premier jour ne pouvait plus changer — ce qui est la meilleure façon de
// faire durer un mot de passe compromis.

// ErrWrongPassword refuse un changement dont le mot de passe actuel est faux.
var ErrWrongPassword = errors.New("le mot de passe actuel est incorrect")

// UpdateSelf corrige l'identité de celui qui est connecté.
//
// L'adresse ne s'y trouve pas : elle est la clé de connexion et la réservation
// globale du compte. La changer soi-même permettrait de s'emparer d'une adresse
// libre chez un autre organisme, et de perdre l'accès si l'on se trompe.
func (s *Service) UpdateSelf(ctx context.Context, session Session, prenom, nom string) (User, error) {
	user, err := s.LoadUser(ctx, session)
	if err != nil {
		return User{}, err
	}
	if strings.TrimSpace(prenom) != "" {
		user.FirstName = strings.TrimSpace(prenom)
	}
	if strings.TrimSpace(nom) != "" {
		user.LastName = strings.TrimSpace(nom)
	}
	user.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, user); err != nil {
		return User{}, err
	}
	return user, nil
}

// ChangePassword remplace le mot de passe après vérification de l'actuel.
//
// L'ancien est exigé même si la session est valide : un poste laissé ouvert
// suffirait sinon à verrouiller le compte de son occupant.
func (s *Service) ChangePassword(ctx context.Context, session Session, actuel, nouveau string) error {
	user, err := s.LoadUser(ctx, session)
	if err != nil {
		return err
	}
	if !VerifyPassword(actuel, user.PasswordHash) {
		return ErrWrongPassword
	}
	if actuel == nouveau {
		return fmt.Errorf("le nouveau mot de passe est identique à l'ancien")
	}

	hash, err := HashPassword(nouveau)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	user.UpdatedAt = s.now()
	return ddb.Put(ctx, s.db, user)
}

// SetPhoto rattache — ou retire — la photo de profil.
func (s *Service) SetPhoto(ctx context.Context, session Session, key string) (User, error) {
	user, err := s.LoadUser(ctx, session)
	if err != nil {
		return User{}, err
	}
	// La clé vient du client : sans cette vérification, il afficherait sous sa
	// photo n'importe quel objet du compartiment.
	prefixe := PhotoPrefix(session.OrgID, session.UserID)
	if key != "" && !strings.HasPrefix(key, prefixe) {
		return User{}, fmt.Errorf("cette image n'a pas été déposée pour ce compte")
	}
	user.PhotoKey = key
	user.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, user); err != nil {
		return User{}, err
	}
	return user, nil
}

// PhotoPrefix est le dossier des photos d'un compte.
func PhotoPrefix(orgID, userID string) string {
	return fmt.Sprintf("photos/%s/%s/", orgID, userID)
}
