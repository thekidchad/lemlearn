package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/platform/audit"
)

// handleAdminContact rend la fiche d'une personne chez un organisme client.
//
// C'est l'écran qu'on ouvre en décrochant : qui elle est, chez qui, ce à quoi
// elle est inscrite, où en est son dossier, et si elle a un compte. Sans cette
// fiche, la recherche menait à la fiche de l'organisme — c'est-à-dire à côté.
//
// La consultation est journalisée chez le client. Lire les données de
// quelqu'un chez un tiers est un accès, et un accès se raconte.
func handleAdminContact(deps Deps) http.HandlerFunc {
	type inscription struct {
		SessionID    string `json:"sessionId"`
		SessionTitle string `json:"sessionTitle,omitempty"`
		StartsAt     string `json:"startsAt,omitempty"`
		Status       string `json:"status"`
		FileID       string `json:"fileId,omitempty"`
		FinalPassed  bool   `json:"finalPassed"`
		FinalPercent int    `json:"finalPercent"`
	}
	type compte struct {
		Email    string `json:"email"`
		Role     string `json:"role"`
		Disabled bool   `json:"disabled"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if deps.CRM == nil || deps.Identity == nil {
			writeError(w, http.StatusServiceUnavailable, "annuaire indisponible")
			return
		}

		orgID, contactID := chi.URLParam(r, "orgID"), chi.URLParam(r, "contactID")
		contact, err := deps.CRM.GetContact(r.Context(), orgID, contactID)
		if err != nil {
			respondNotFound(w, err, "fiche introuvable")
			return
		}
		org, err := deps.Identity.LoadOrg(r.Context(), orgID)
		if err != nil {
			respondNotFound(w, err, "organisation introuvable")
			return
		}

		response := map[string]any{
			"contact": contact,
			"org":     map[string]any{"id": org.ID, "name": org.Name, "plan": org.Plan},
		}

		// Le compte, s'il existe. Une fiche sans compte est le cas ordinaire :
		// on saisit un stagiaire bien avant de lui ouvrir un espace.
		if contact.Email != "" {
			if user, err := deps.Identity.UserByEmail(r.Context(), contact.Email); err == nil &&
				user.OrgID == orgID {
				response["compte"] = compte{
					Email: user.Email, Role: string(user.Role), Disabled: user.Disabled,
				}
			}
		}

		// Les dossiers où la personne figure, à quelque titre que ce soit :
		// c'est par là qu'on remonte à sa formation et à ses preuves.
		dossiers := make([]crm.File, 0, 4)
		for _, etape := range etapes {
			lot, err := deps.CRM.ListFilesByStage(r.Context(), orgID, etape, 200)
			if err != nil {
				continue
			}
			for _, file := range lot {
				if file.LearnerID == contactID || file.CompanyID == contactID ||
					file.FunderID == contactID {
					dossiers = append(dossiers, file)
				}
			}
		}
		response["dossiers"] = dossiers

		if deps.Catalog != nil && contact.Kind == crm.KindLearner {
			inscriptions := make([]inscription, 0, 4)
			if lot, err := deps.Catalog.ListLearnerEnrollments(r.Context(), orgID, contactID); err == nil {
				for _, e := range lot {
					ligne := inscription{
						SessionID: e.SessionID, Status: string(e.Status), FileID: e.FileID,
						FinalPassed: e.FinalPassed, FinalPercent: e.FinalPercent,
					}
					if session, err := deps.Catalog.GetSession(r.Context(), orgID, e.SessionID); err == nil {
						ligne.SessionTitle = session.Title
						ligne.StartsAt = session.StartsAt.Format("2006-01-02")
					}
					inscriptions = append(inscriptions, ligne)
				}
			}
			response["inscriptions"] = inscriptions
		}

		session, _ := sessionFrom(r)
		if _, err := deps.Identity.AuditOrg(r.Context(), orgID, audit.ActionImpersonated,
			actorFrom(r, session.UserID),
			map[string]any{"consultation": "fiche", "fiche": contact.DisplayName()},
		); err != nil {
			deps.Log.Error("journal de la consultation", "err", err)
		}

		writeJSON(w, http.StatusOK, response)
	}
}
