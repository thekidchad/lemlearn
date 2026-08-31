package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// OpenOrgInput décrit l'ouverture d'un organisme par l'équipe.
type OpenOrgInput struct {
	OrgName   string
	Email     string
	FirstName string
	LastName  string
	Plan      string
	// By nomme le membre de l'équipe qui ouvre le compte : une organisation
	// apparue sans qu'on sache qui l'a créée est un trou dans le journal.
	By audit.Actor
}

// OpenOrg crée un organisme et invite son responsable.
//
// C'est le pendant commercial de l'inscription libre : jusqu'ici une
// organisation ne pouvait naître que si quelqu'un s'inscrivait de lui-même sur
// le site, ce qui interdisait de préparer un client avant de le lui remettre.
//
// Le compte du responsable est créé désactivé et sans mot de passe, comme celui
// d'un apprenant invité : c'est lui qui choisira le sien par le lien reçu. Nous
// ne fabriquons donc jamais de mot de passe pour un client, et aucun secret ne
// transite par un courriel.
//
// L'organisation existe dès cet instant, avant même que le responsable se
// connecte : c'est ce qui permet à l'équipe de l'habiller — logo, couleur,
// identité juridique — pour qu'il trouve son enseigne à sa première visite.
func (s *Service) OpenOrg(ctx context.Context, in OpenOrgInput) (Org, User, string, error) {
	email := ddb.NormalizeEmail(in.Email)
	if email == "" || !strings.Contains(email, "@") {
		return Org{}, User{}, "", fmt.Errorf("l'adresse du responsable est obligatoire")
	}
	if strings.TrimSpace(in.OrgName) == "" {
		return Org{}, User{}, "", fmt.Errorf("le nom de l'organisme est obligatoire")
	}

	now := s.now()
	org := NewOrg(strings.TrimSpace(in.OrgName), now)
	if plan := strings.TrimSpace(in.Plan); plan != "" {
		org.Plan = plan
	}

	user := NewUser(org.ID, email, in.FirstName, in.LastName, RoleOwner, "", now)
	// Désactivé jusqu'à ce qu'un mot de passe soit choisi : un compte sans
	// empreinte accepterait n'importe quoi si la vérification changeait un
	// jour de forme.
	user.Disabled = true

	token, hash, err := NewToken()
	if err != nil {
		return Org{}, User{}, "", err
	}

	_, err = s.db.WriteWithAudit(ctx, "org/"+org.ID,
		[]ddb.Write{
			{Item: org, Condition: "attribute_not_exists(PK)"},
			{Item: user},
			// L'annuaire dans la même transaction : une organisation créée mais
			// absente de l'annuaire serait invisible du support, donc
			// introuvable le jour où elle appelle.
			{Item: NewDirectoryEntry(org, user.Email, now)},
			// La seule condition qui compte : l'adresse ne doit pas être déjà
			// réservée. Elle fait échouer toute la transaction.
			{Item: NewEmailPointer(email, org.ID, user.ID, now), Condition: "attribute_not_exists(PK)"},
			{Item: NewInvitation(hash, org.ID, user.ID, "", email, now)},
		},
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, "org/"+org.ID, now, audit.ActionFileCreated, in.By,
				map[string]any{
					"organisme":   org.Name,
					"responsable": email,
					"ouvert_par":  "équipe",
				})
		})
	if err != nil {
		if errors.Is(err, ddb.ErrConflict) {
			return Org{}, User{}, "", ErrEmailTaken
		}
		return Org{}, User{}, "", err
	}

	return org, user, token, nil
}
