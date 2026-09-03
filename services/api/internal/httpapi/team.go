package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/emailtpl"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/mail"
)

// L'équipe d'un organisme : qui a un accès, et ce qu'il peut faire.

// roleLabels nomme les rôles pour les gens plutôt que pour le code.
var roleLabels = map[identity.Role]string{
	identity.RoleOwner:   "propriétaire",
	identity.RoleAdmin:   "administrateur",
	identity.RoleTrainer: "formateur",
}

func handleListTeam(deps Deps) http.HandlerFunc {
	type membre struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Role      string `json:"role"`
		RoleLabel string `json:"roleLabel"`
		Disabled  bool   `json:"disabled"`
		// Pending distingue « suspendu » de « n'a jamais choisi son mot de
		// passe ». Les deux comptes sont désactivés, et les confondre ferait
		// relancer quelqu'un qu'on vient d'écarter.
		Pending     bool   `json:"pending"`
		LastLoginAt string `json:"lastLoginAt,omitempty"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		users, err := deps.Identity.TeamMembers(r.Context(), session.OrgID)
		if err != nil {
			deps.Log.Error("équipe", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		membres := make([]membre, 0, len(users))
		for _, user := range users {
			ligne := membre{
				ID: user.ID, Email: user.Email,
				FirstName: user.FirstName, LastName: user.LastName,
				Role: string(user.Role), RoleLabel: roleLabels[user.Role],
				Disabled: user.Disabled,
				Pending:  user.Disabled && user.PasswordHash == "",
			}
			if user.LastLoginAt != nil {
				ligne.LastLoginAt = user.LastLoginAt.Format("2006-01-02")
			}
			membres = append(membres, ligne)
		}
		writeJSON(w, http.StatusOK, map[string]any{"membres": membres})
	}
}

// handleInviteTeamMember ouvre un accès à un collègue.
func handleInviteTeamMember(deps Deps) http.HandlerFunc {
	type request struct {
		Email     string `json:"email"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Role      string `json:"role"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		role := identity.Role(strings.TrimSpace(body.Role))
		if role == "" {
			role = identity.RoleAdmin
		}

		user, token, err := deps.Identity.InviteTeamMember(r.Context(), identity.InviteTeamInput{
			OrgID: session.OrgID, Email: body.Email,
			FirstName: body.FirstName, LastName: body.LastName, Role: role,
		})
		if err != nil {
			if errors.Is(err, identity.ErrEmailTaken) {
				writeError(w, http.StatusConflict, "cette adresse est déjà utilisée")
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		org, _ := deps.Identity.LoadOrg(r.Context(), session.OrgID)
		link := strings.TrimSuffix(deps.Config.AppURL, "/") + "/invitation/" + token

		response := map[string]any{"user": user.Public(), "invitationUrl": link}

		// Le message part sous l'enseigne de l'organisme : c'est chez lui qu'on
		// invite, et notre nom n'aurait rien à y faire.
		if deps.Mailer != nil && deps.Emails != nil {
			data := map[string]any{
				"FirstName": body.FirstName,
				"OrgName":   org.Name,
				"RoleLabel": roleLabels[role],
				"Link":      link,
			}
			for champ, valeur := range publicBrand(r, deps, org.ID).MailData() {
				data[champ] = valeur
			}
			message, err := deps.Emails.Compose(r.Context(), emailtpl.KeyTeamInvitation, data)
			if err != nil {
				deps.Log.Error("composition de l'invitation", "err", err)
			} else {
				envoi := mail.WithSender(
					mail.WithContext(r.Context(), org.ID, emailtpl.KeyTeamInvitation),
					publicBrand(r, deps, org.ID).Name)
				if err := deps.Mailer.Send(envoi, user.Email, message.Subject, message.HTML); err != nil {
					deps.Log.Error("envoi de l'invitation", "err", err)
					response["warning"] = "accès ouvert, mais le courriel n'est pas parti : transmettez le lien."
					writeJSON(w, http.StatusAccepted, response)
					return
				}
				response["sentTo"] = user.Email
			}
		}

		auditTeam(r, deps, session, audit.ActionConsentGiven,
			map[string]any{"acces": "ouvert", "compte": user.Email, "role": string(role)})

		writeJSON(w, http.StatusCreated, response)
	}
}

// handleUpdateTeamMember change un rôle ou suspend un accès.
func handleUpdateTeamMember(deps Deps) http.HandlerFunc {
	type request struct {
		Role     *string `json:"role"`
		Disabled *bool   `json:"disabled"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		userID := chi.URLParam(r, "userID")

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		// Se retirer soi-même son propre accès est le meilleur moyen de se
		// retrouver dehors sans personne pour rouvrir la porte.
		if userID == session.UserID && body.Disabled != nil && *body.Disabled {
			writeError(w, http.StatusConflict, "vous ne pouvez pas suspendre votre propre accès")
			return
		}

		var role *identity.Role
		if body.Role != nil {
			converti := identity.Role(*body.Role)
			role = &converti
		}

		user, err := deps.Identity.UpdateTeamMember(r.Context(), session.OrgID, userID, role, body.Disabled)
		if err != nil {
			if errors.Is(err, identity.ErrLastOwner) || errors.Is(err, identity.ErrNeverActivated) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			respondNotFound(w, err, "compte introuvable")
			return
		}

		auditTeam(r, deps, session, audit.ActionConsentGiven, map[string]any{
			"acces": etatDe(user), "compte": user.Email, "role": string(user.Role),
		})

		writeJSON(w, http.StatusOK, map[string]any{"user": user.Public()})
	}
}

func etatDe(user identity.User) string {
	if user.Disabled {
		return "suspendu"
	}
	return "actif"
}

// auditTeam journalise un changement d'accès chez l'organisme.
//
// Ce n'est pas une donnée métier, mais c'est ce qu'un contrôle demande en
// premier : qui pouvait voir les dossiers, et depuis quand.
func auditTeam(r *http.Request, deps Deps, session identity.Session,
	action audit.Action, payload map[string]any,
) {
	if _, err := deps.Identity.AuditOrg(r.Context(), session.OrgID, action,
		actorFrom(r, session.UserID), payload); err != nil {
		deps.Log.Error("journal des accès", "err", err)
	}
}
