package httpapi

import (
	"io"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/billing"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/audit"
)

// handleListOrgs renvoie l'annuaire des organisations clientes.
//
// C'est l'écran du support et du commerce : qui déborde de son plan, qui n'a
// rien produit depuis son inscription, qui vaut un appel. Les chiffres y sont
// des ordres de grandeur assumés — le détail se lit dans l'organisation.
func handleListOrgs(deps Deps) http.HandlerFunc {
	type row struct {
		identity.DirectoryEntry
		PriceCents int            `json:"priceCents"`
		Usage      *billing.Usage `json:"usage,omitempty"`
		Overage    []string       `json:"overage,omitempty"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Identity == nil {
			writeError(w, http.StatusServiceUnavailable, "annuaire indisponible")
			return
		}

		entries, err := deps.Identity.ListOrgs(r.Context())
		if err != nil {
			deps.Log.Error("annuaire des organisations", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		rows := make([]row, 0, len(entries))
		mrr := 0
		for _, entry := range entries {
			plan, err := billing.PlanByCode(entry.Plan)
			if err != nil {
				// Un plan inconnu — renommé, retiré du catalogue — ne doit
				// pas faire disparaître l'organisation de l'écran : c'est
				// justement celle qu'il faut regarder.
				plan = billing.Plan{Code: entry.Plan, Label: entry.Plan}
			}
			mrr += plan.PriceCents

			line := row{DirectoryEntry: entry, PriceCents: plan.PriceCents}
			if deps.Billing != nil {
				if usage, err := deps.Billing.Usage(r.Context(), entry.OrgID); err == nil {
					line.Usage = &usage
					line.Overage = usage.Overage(plan)
				}
			}
			rows = append(rows, line)
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

		writeJSON(w, http.StatusOK, map[string]any{
			"orgs":     rows,
			"mrrCents": mrr,
			"plans":    billing.Plans,
		})
	}
}

// handleSetPlan change la formule d'une organisation.
func handleSetPlan(deps Deps) http.HandlerFunc {
	type request struct {
		Plan string `json:"plan"`
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
		plan, err := billing.PlanByCode(body.Plan)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		orgID := chi.URLParam(r, "orgID")
		before, err := deps.Identity.LoadOrg(r.Context(), orgID)
		if err != nil {
			respondNotFound(w, err, "organisation introuvable")
			return
		}

		org, err := deps.Identity.SetPlan(r.Context(), orgID, plan.Code)
		if err != nil {
			deps.Log.Error("changement de formule", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		// Le changement est journalisé sur l'organisation, pas sur un dossier :
		// c'est un fait commercial, et il doit être opposable au client comme
		// à nous.
		if _, err := deps.Identity.AuditOrg(r.Context(), orgID, audit.ActionPlanChanged,
			actorFrom(r, session.UserID),
			map[string]any{"avant": before.Plan, "apres": org.Plan, "prixCents": plan.PriceCents},
		); err != nil {
			deps.Log.Error("journal du changement de formule", "err", err)
		}

		writeJSON(w, http.StatusOK, map[string]any{"org": org.Public(), "plan": plan})
	}
}

// handleImpersonate ouvre une session sur une organisation cliente.
//
// L'accès total du support est un besoin réel — personne ne dépanne un dossier
// qu'il ne peut pas voir — et un danger tout aussi réel. Le garde-fou n'est pas
// de l'interdire mais de le rendre impossible à dissimuler : la session porte
// le nom de son auteur, et chaque événement qu'elle produit le recopie.
func handleImpersonate(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Identity == nil {
			writeError(w, http.StatusServiceUnavailable, "annuaire indisponible")
			return
		}

		orgID := chi.URLParam(r, "orgID")
		target, err := deps.Identity.FirstOwner(r.Context(), orgID)
		if err != nil {
			respondNotFound(w, err, "aucun compte administrateur dans cette organisation")
			return
		}

		token, err := deps.Identity.OpenSessionFor(r.Context(), target,
			session.UserID, clientIP(r), truncateUA(r.UserAgent()))
		if err != nil {
			deps.Log.Error("impersonation", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		if _, err := deps.Identity.AuditOrg(r.Context(), orgID, audit.ActionImpersonated,
			actorFrom(r, session.UserID),
			map[string]any{"compte": target.Email, "role": string(target.Role)},
		); err != nil {
			deps.Log.Error("journal de l'impersonation", "err", err)
		}

		setSessionCookie(w, deps.Config, token)
		writeJSON(w, http.StatusOK, map[string]any{
			"org":   orgID,
			"as":    target.Email,
			"trace": "cette session porte le nom de son auteur dans chaque événement",
		})
	}
}

// handleStripeWebhook applique les changements d'abonnement annoncés par
// Stripe.
//
// La route est publique et le restera : Stripe n'a pas de session. Sa
// légitimité tient entièrement à la signature de l'appel, vérifiée avant
// d'ouvrir la charge utile.
func handleStripeWebhook(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Stripe == nil || deps.Identity == nil {
			writeError(w, http.StatusServiceUnavailable, "abonnement en libre-service non configuré")
			return
		}

		payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, "charge illisible")
			return
		}
		if err := deps.Stripe.VerifyWebhook(payload, r.Header.Get("Stripe-Signature"), deps.Now()); err != nil {
			// Le motif n'est pas renvoyé : à un appelant non authentifié, il
			// n'apprendrait rien d'utile et beaucoup à qui cherche.
			deps.Log.Warn("webhook stripe refusé", "err", err)
			writeError(w, http.StatusBadRequest, "signature invalide")
			return
		}

		change, handled, err := billing.ReadEvent(payload)
		if err != nil {
			deps.Log.Error("événement stripe", "err", err)
			writeError(w, http.StatusBadRequest, "événement illisible")
			return
		}
		if !handled {
			// Répondre 200 à ce qu'on ne traite pas : un webhook qui échoue
			// sur des événements qu'il ignore finit désactivé par Stripe.
			writeJSON(w, http.StatusOK, map[string]any{"ignored": true})
			return
		}

		// Un abonnement impayé ne change pas la formule : il la laisse en
		// place et se règle par relance, pas en coupant l'accès aux preuves
		// d'un organisme en pleine session.
		if !change.Active && change.Type != "customer.subscription.deleted" {
			deps.Log.Warn("abonnement en défaut", "org", change.OrgID, "type", change.Type)
			writeJSON(w, http.StatusOK, map[string]any{"noted": change.Type})
			return
		}

		plan, err := billing.PlanByCode(change.Plan)
		if err != nil {
			deps.Log.Error("formule stripe inconnue", "plan", change.Plan)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if _, err := deps.Identity.SetPlan(r.Context(), change.OrgID, plan.Code); err != nil {
			deps.Log.Error("application de la formule", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		if _, err := deps.Identity.AuditOrg(r.Context(), change.OrgID, audit.ActionPlanChanged,
			audit.Actor{Type: audit.ActorSystem, ID: "stripe"},
			map[string]any{"formule": plan.Code, "evenement": change.Type},
		); err != nil {
			deps.Log.Error("journal du changement de formule", "err", err)
		}

		writeJSON(w, http.StatusOK, map[string]any{"applied": plan.Code})
	}
}

// handleCheckout ouvre une page de paiement pour l'organisation connectée.
func handleCheckout(deps Deps) http.HandlerFunc {
	type request struct {
		Plan string `json:"plan"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Stripe == nil {
			writeError(w, http.StatusServiceUnavailable,
				"l'abonnement en ligne n'est pas ouvert : votre contact lemlearn s'en occupe")
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		plan, err := billing.PlanByCode(body.Plan)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		url, err := deps.Stripe.Checkout(r.Context(), session.OrgID, plan.Code,
			deps.Config.AppURL+"/abonnement?paye=1", deps.Config.AppURL+"/abonnement")
		if err != nil {
			deps.Log.Error("ouverture du paiement", "err", err)
			writeError(w, http.StatusBadGateway, "le paiement n'a pas pu être ouvert")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"url": url})
	}
}
