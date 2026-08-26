package httpapi

import (
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/billing"
	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/platform/audit"
)

// handleOrgDetail renvoie la fiche d'une organisation cliente.
//
// Ce que le support regarde en décrochant : quelle formule, quelle
// consommation, qui sont les comptes, et ce qui s'est passé récemment sur le
// plan commercial — changements de palier et accès de l'équipe compris.
func handleOrgDetail(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Identity == nil {
			writeError(w, http.StatusServiceUnavailable, "annuaire indisponible")
			return
		}

		orgID := chi.URLParam(r, "orgID")
		org, err := deps.Identity.LoadOrg(r.Context(), orgID)
		if err != nil {
			respondNotFound(w, err, "organisation introuvable")
			return
		}

		plan, err := billing.PlanByCode(org.Plan)
		if err != nil {
			plan = billing.Plan{Code: org.Plan, Label: org.Plan}
		}

		response := map[string]any{"org": org.Public(), "plan": plan, "plans": billing.Plans}

		if deps.Billing != nil {
			if usage, err := deps.Billing.Usage(r.Context(), orgID); err == nil {
				response["usage"] = usage
				response["overage"] = usage.Overage(plan)
			}
		}
		if users, err := deps.Identity.ListUsers(r.Context(), orgID); err == nil {
			response["users"] = list(users)
		}
		if events, err := deps.Identity.OrgTimeline(r.Context(), orgID, 40); err == nil {
			response["timeline"] = list(events)
		}

		writeJSON(w, http.StatusOK, response)
	}
}

// handleFindLearner retrouve un apprenant à travers les organisations.
//
// La recherche se fait sur l'adresse exacte, pas sur un fragment de nom :
// c'est la seule clé que l'on peut résoudre sans parcourir les données de tous
// les clients — et un support qui peut lister les apprenants de tout le monde
// n'a pas sa place dans un produit qui vend la protection des données.
func handleFindLearner(deps Deps) http.HandlerFunc {
	type match struct {
		OrgID      string `json:"orgId"`
		OrgName    string `json:"orgName"`
		ContactID  string `json:"contactId"`
		Name       string `json:"name"`
		Email      string `json:"email"`
		HasAccount bool   `json:"hasAccount"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Identity == nil || deps.CRM == nil {
			writeError(w, http.StatusServiceUnavailable, "annuaire indisponible")
			return
		}

		email := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
		if !strings.Contains(email, "@") {
			writeError(w, http.StatusBadRequest,
				"la recherche se fait sur l'adresse exacte de l'apprenant")
			return
		}

		matches := make([]match, 0, 2)

		// Deux pistes : un compte portant cette adresse, et les fiches de
		// contact qui la portent dans les organisations connues.
		if user, err := deps.Identity.UserByEmail(r.Context(), email); err == nil {
			org, _ := deps.Identity.LoadOrg(r.Context(), user.OrgID)
			matches = append(matches, match{
				OrgID: user.OrgID, OrgName: org.Name, ContactID: user.ContactID,
				Name: user.FullName(), Email: user.Email, HasAccount: true,
			})
		}

		entries, err := deps.Identity.ListOrgs(r.Context())
		if err != nil {
			deps.Log.Error("annuaire", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		for _, entry := range entries {
			contacts, err := deps.CRM.ListContacts(r.Context(), entry.OrgID, crm.KindLearner, 500)
			if err != nil {
				continue
			}
			for _, contact := range contacts {
				if strings.ToLower(contact.Email) != email {
					continue
				}
				already := false
				for _, found := range matches {
					if found.ContactID == contact.ID {
						already = true
					}
				}
				if already {
					continue
				}
				matches = append(matches, match{
					OrgID: entry.OrgID, OrgName: entry.Name, ContactID: contact.ID,
					Name: contact.DisplayName(), Email: contact.Email,
				})
			}
		}

		sort.Slice(matches, func(i, j int) bool { return matches[i].OrgName < matches[j].OrgName })

		// La recherche est elle-même journalisée : chercher quelqu'un dans les
		// données d'un client est un accès, pas une consultation d'annuaire.
		session, _ := sessionFrom(r)
		for _, found := range matches {
			if _, err := deps.Identity.AuditOrg(r.Context(), found.OrgID,
				audit.ActionImpersonated, actorFrom(r, session.UserID),
				map[string]any{"recherche": "apprenant", "adresse": email},
			); err != nil {
				deps.Log.Error("journal de la recherche", "err", err)
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{"matches": matches})
	}
}
