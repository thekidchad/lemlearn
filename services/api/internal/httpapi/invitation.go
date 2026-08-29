package httpapi

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/emailtpl"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/mail"
)

// handleInviteLearner ouvre un accès à l'espace apprenant.
//
// C'est le seul chemin par lequel un apprenant obtient un compte : il ne
// s'inscrit pas lui-même. Un espace apprenant en libre inscription laisserait
// n'importe qui se déclarer stagiaire d'un organisme.
func handleInviteLearner(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Identity == nil || deps.CRM == nil {
			writeError(w, http.StatusServiceUnavailable, "espace apprenant indisponible")
			return
		}

		contact, err := deps.CRM.GetContact(r.Context(), session.OrgID, chi.URLParam(r, "contactID"))
		if err != nil {
			respondNotFound(w, err, "contact introuvable")
			return
		}
		if contact.Anonymized {
			writeError(w, http.StatusConflict, "cette fiche a été anonymisée")
			return
		}

		user, token, err := deps.Identity.InviteLearner(r.Context(), identity.InviteInput{
			OrgID: session.OrgID, ContactID: contact.ID, Email: contact.Email,
			FirstName: contact.FirstName, LastName: contact.LastName,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		org, err := deps.Identity.LoadOrg(r.Context(), session.OrgID)
		if err != nil {
			deps.Log.Error("organisation", "err", err)
		}

		link := strings.TrimSuffix(deps.Config.AppURL, "/") + "/invitation/" + token
		if deps.Mailer != nil {
			message := mail.Composed{
				Subject: fmt.Sprintf("Votre espace de formation — %s", org.Name),
				HTML:    invitationEmail(contact.FirstName, org.Name, link),
			}
			if deps.Emails != nil {
				data := map[string]any{
					"FirstName": contact.FirstName,
					"OrgName":   org.Name,
					"Link":      link,
				}
				for champ, valeur := range publicBrand(r, deps, org.ID).MailData() {
					data[champ] = valeur
				}
				if rendered, err := deps.Emails.Compose(r.Context(), emailtpl.KeyLearnerInvitation, data); err == nil {
					message = rendered
				}
			}

			envoi := mail.WithSender(
				mail.WithContext(r.Context(), org.ID, emailtpl.KeyLearnerInvitation),
				publicBrand(r, deps, org.ID).Name)
			if err := deps.Mailer.Send(envoi,
				user.Email, message.Subject, message.HTML); err != nil {
				deps.Log.Error("envoi de l'invitation", "err", err)
				writeJSON(w, http.StatusAccepted, map[string]any{
					"user":    user.Public(),
					"warning": "compte créé mais courriel non parti : " + err.Error(),
				})
				return
			}
		}

		// L'ouverture d'un accès est journalisée sur les dossiers de
		// l'apprenant : savoir depuis quand il pouvait se connecter éclaire
		// une contestation d'assiduité.
		if _, err := deps.CRM.RecordAccess(r.Context(), session.OrgID, contact.ID,
			actorFrom(r, session.UserID),
			map[string]any{"acces": "espace apprenant", "adresse": user.Email},
		); err != nil {
			deps.Log.Error("journal de l'invitation", "err", err)
		}

		writeJSON(w, http.StatusOK, map[string]any{"user": user.Public(), "sentTo": user.Email})
	}
}

// handleInvitationOpen dit ce que porte un lien d'invitation.
func handleInvitationOpen(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Identity == nil {
			writeError(w, http.StatusServiceUnavailable, "espace apprenant indisponible")
			return
		}

		invitation, err := deps.Identity.ResolveInvitation(r.Context(), chi.URLParam(r, "token"))
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		org, _ := deps.Identity.LoadOrg(r.Context(), invitation.OrgID)
		writeJSON(w, http.StatusOK, map[string]any{
			"email":     maskEmail(invitation.Email),
			"org":       org.Name,
			"expiresAt": invitation.ExpiresOn,
			"brand":     publicBrand(r, deps, invitation.OrgID),
		})
	}
}

// handleInvitationAccept fixe le mot de passe et ouvre la session.
func handleInvitationAccept(deps Deps) http.HandlerFunc {
	type request struct {
		Password string `json:"password"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Identity == nil {
			writeError(w, http.StatusServiceUnavailable, "espace apprenant indisponible")
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		user, err := deps.Identity.AcceptInvitation(r.Context(), chi.URLParam(r, "token"), body.Password)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// La session s'ouvre dans la foulée : demander de se reconnecter
		// juste après avoir choisi un mot de passe est une étape que personne
		// ne comprend.
		token, err := deps.Identity.OpenSession(r.Context(), user,
			clientIP(r), truncateUA(r.UserAgent()))
		if err != nil {
			deps.Log.Error("ouverture de session", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		setSessionCookie(w, deps.Config, token)
		writeJSON(w, http.StatusOK, map[string]any{"user": user.Public()})
	}
}

// invitationEmail compose le courriel d'accueil.
//
// Il dit ce que l'apprenant va y faire, pas seulement qu'un compte l'attend :
// un message qui annonce « votre accès » sans dire à quoi ressemble la suite
// se fait ignorer.
func invitationEmail(firstName, orgName, link string) string {
	greeting := "Bonjour"
	if trimmed := strings.TrimSpace(firstName); trimmed != "" {
		greeting += " " + html.EscapeString(trimmed)
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="fr"><body style="margin:0;padding:24px;background:#f5f6f8;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;color:#10131a">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0"><tr><td align="center">
<table role="presentation" width="100%%" style="max-width:520px;background:#ffffff;border:1px solid #e3e6ec;border-radius:12px" cellpadding="0" cellspacing="0">
<tr><td style="padding:28px">
<h1 style="margin:0 0 16px;font-size:19px;font-weight:600;letter-spacing:-0.02em">Votre espace de formation</h1>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">%s,</p>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">
<strong>%s</strong> vous ouvre l'accès à votre espace. Vous y trouverez vos
modules vidéo, les questionnaires qui les accompagnent et votre progression —
c'est aussi là que votre attestation deviendra disponible.
</p>
<p style="margin:0 0 24px">
<a href="%s" style="display:inline-block;background:#4b37b8;color:#ffffff;text-decoration:none;padding:11px 20px;border-radius:8px;font-size:14px;font-weight:500">Choisir mon mot de passe</a>
</p>
<p style="margin:0;font-size:12px;line-height:1.6;color:#5b6170">
Ce lien est personnel et expire dans quatorze jours. Votre temps de visionnage
est enregistré : il constitue la preuve d'assiduité que votre organisme doit
pouvoir présenter.
</p>
</td></tr></table>
</td></tr></table>
</body></html>`, greeting, html.EscapeString(orgName), html.EscapeString(link))
}
