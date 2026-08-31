package httpapi

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/audit"
)

// Agir sur une ligne de la plateforme, depuis la vue de l'équipe.
//
// Deux gestes seulement, et ce sont ceux qu'on fait quand quelqu'un appelle :
// entrer dans son espace pour voir ce qu'il voit, et sortir son dossier pour
// le lui envoyer. Le reste passe par l'écran du client.

// handleImpersonateContact ouvre une session sur le compte d'un contact.
//
// L'impersonation existante vise le propriétaire de l'organisme : utile pour
// régler un problème d'administration, inutile quand un stagiaire dit ne pas
// voir son module. Celle-ci vise la personne dont on parle.
func handleImpersonateContact(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		orgID := chi.URLParam(r, "orgID")

		contact, err := deps.CRM.GetContact(r.Context(), orgID, chi.URLParam(r, "contactID"))
		if err != nil {
			respondNotFound(w, err, "contact introuvable")
			return
		}
		if contact.Email == "" {
			writeError(w, http.StatusConflict, "cette fiche n'a pas d'adresse : aucun compte n'y est rattaché")
			return
		}

		cible, err := deps.Identity.UserByEmail(r.Context(), contact.Email)
		if err != nil {
			writeError(w, http.StatusConflict,
				"aucun compte pour cette adresse : invitez d'abord la personne depuis sa fiche")
			return
		}
		if cible.Disabled {
			writeError(w, http.StatusConflict,
				"cette personne n'a pas encore choisi son mot de passe : son espace n'existe pas encore")
			return
		}
		if cible.OrgID != orgID {
			// Ne devrait pas arriver — l'adresse est réservée par organisation —
			// mais le vérifier coûte moins qu'une session ouverte au mauvais
			// endroit.
			writeError(w, http.StatusConflict, "ce compte appartient à un autre organisme")
			return
		}

		auteur, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}

		token, err := deps.Identity.Impersonate(r.Context(), cible,
			session.UserID, auteur.Email, clientIP(r), truncateUA(r.UserAgent()))
		if err != nil {
			deps.Log.Error("impersonation", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		if _, err := deps.Identity.AuditOrg(r.Context(), orgID, audit.ActionImpersonated,
			actorFrom(r, session.UserID),
			map[string]any{"compte": cible.Email, "role": string(cible.Role)},
		); err != nil {
			deps.Log.Error("journal de l'impersonation", "err", err)
		}

		setSessionCookie(w, deps.Config, token)
		writeJSON(w, http.StatusOK, map[string]any{
			"user": cible.Public(),
			// Où atterrir : un stagiaire n'a rien à faire sur le pipeline.
			"landing": landingFor(cible.Role),
		})
	}
}

// handleAdminExport sort le dossier d'un apprenant depuis la vue de l'équipe.
//
// C'est le même assemblage que celui du client, sur son organisation à lui.
// L'export reste journalisé chez lui : savoir qui a extrait un dossier, et
// quand, fait partie de ce qu'un contrôle peut demander — et « l'éditeur » est
// une réponse acceptable, « personne » ne l'est pas.
func handleAdminExport(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Export == nil {
			writeError(w, http.StatusServiceUnavailable, "export indisponible")
			return
		}

		orgID, fileID := chi.URLParam(r, "orgID"), chi.URLParam(r, "fileID")
		file, err := deps.CRM.GetFile(r.Context(), orgID, fileID)
		if err != nil {
			respondNotFound(w, err, "dossier introuvable")
			return
		}
		auteur, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}

		archive, manifest, err := deps.Export.Build(r.Context(), orgID, fileID)
		if err != nil {
			deps.Log.Error("export", "file", fileID, "err", err)
			writeError(w, http.StatusConflict, err.Error())
			return
		}

		if _, auditErr := deps.CRM.RecordExport(r.Context(), orgID, fileID,
			actorFrom(r, auteur.FullName()),
			map[string]any{
				"pieces": len(manifest.Entries), "missing": len(manifest.Missing),
				"bytes": len(archive), "events": manifest.AuditEvents,
				"par": "équipe lemlearn",
			}); auditErr != nil {
			deps.Log.Error("journalisation de l'export", "err", auditErr)
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition",
			fmt.Sprintf(`attachment; filename="dossier-%s.zip"`, file.Reference))
		w.Header().Set("X-Lemlearn-Pieces", fmt.Sprint(len(manifest.Entries)))
		w.Header().Set("X-Lemlearn-Missing", fmt.Sprint(len(manifest.Missing)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	}
}

// landingFor dit où atterrir après être entré dans un compte. Un stagiaire
// n'a rien à faire sur le pipeline : il y recevrait un 403 sur le premier
// écran.
func landingFor(role identity.Role) string {
	if role == identity.RoleLearner {
		return "/apprenant"
	}
	return "/pipeline"
}
