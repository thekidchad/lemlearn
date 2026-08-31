package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/lemlearn/api/internal/emailtpl"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/mail"
)

// handleOpenOrg ouvre l'espace d'un organisme client et invite son responsable.
//
// Jusqu'ici une organisation ne pouvait naître que par l'inscription libre :
// l'équipe n'avait aucun moyen de préparer un client avant de le lui remettre.
// C'était le premier geste commercial, et il n'existait pas.
//
// Aucun mot de passe n'est fabriqué ici : le compte est créé désactivé, et le
// responsable choisit le sien par le lien reçu. Un secret que nous aurions
// choisi et envoyé par courriel resterait dans sa boîte pour toujours.
func handleOpenOrg(deps Deps) http.HandlerFunc {
	type request struct {
		OrgName   string `json:"orgName"`
		Email     string `json:"email"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Plan      string `json:"plan"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Identity == nil {
			writeError(w, http.StatusServiceUnavailable, "annuaire indisponible")
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		org, user, token, err := deps.Identity.OpenOrg(r.Context(), identity.OpenOrgInput{
			OrgName: body.OrgName, Email: body.Email,
			FirstName: body.FirstName, LastName: body.LastName,
			Plan: body.Plan, By: actorFrom(r, session.UserID),
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		link := strings.TrimSuffix(deps.Config.AppURL, "/") + "/invitation/" + token

		// Le lien est renvoyé quoi qu'il arrive : si le courriel ne part pas,
		// l'équipe peut le transmettre elle-même plutôt que de recommencer
		// l'ouverture — qui échouerait, l'adresse étant désormais réservée.
		response := map[string]any{
			"org": org.Public(), "user": user.Public(), "invitationUrl": link,
		}

		if deps.Mailer != nil && deps.Emails != nil {
			// C'est le seul message du produit où notre nom est légitime : il
			// s'adresse à quelqu'un qui nous a acheté l'outil, pas à un
			// stagiaire qui ne nous connaît pas.
			message, err := deps.Emails.Compose(r.Context(), emailtpl.KeyOrgInvitation, map[string]any{
				"FirstName": body.FirstName,
				"OrgName":   org.Name,
				"Link":      link,
				"BrandName": "lemlearn",
			})
			if err != nil {
				deps.Log.Error("composition de l'invitation", "err", err)
			} else if err := deps.Mailer.Send(
				mail.WithSender(mail.WithContext(r.Context(), org.ID, emailtpl.KeyOrgInvitation), "lemlearn"),
				user.Email, message.Subject, message.HTML); err != nil {
				deps.Log.Error("envoi de l'invitation", "err", err)
				response["warning"] = fmt.Sprintf(
					"espace créé, mais le courriel n'est pas parti (%s) : transmettez le lien.", err)
				writeJSON(w, http.StatusAccepted, response)
				return
			}
			response["sentTo"] = user.Email
		}

		writeJSON(w, http.StatusCreated, response)
	}
}
