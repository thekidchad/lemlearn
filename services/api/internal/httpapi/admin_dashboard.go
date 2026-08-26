package httpapi

import (
	"net/http"
	"sort"
	"time"

	"github.com/lemlearn/api/internal/billing"
)

// handleAdminDashboard agrège ce que l'équipe regarde en arrivant.
//
// Tout y est calculé à partir de données réelles : aucun chiffre n'est estimé.
// Ce qui n'existe pas — l'historique des encaissements, faute de prestataire de
// paiement branché — n'est pas remplacé par une approximation, il est absent.
func handleAdminDashboard(deps Deps) http.HandlerFunc {
	type planLine struct {
		Code     string `json:"code"`
		Label    string `json:"label"`
		Orgs     int    `json:"orgs"`
		MRRCents int    `json:"mrrCents"`
	}
	type dayLine struct {
		Day    string `json:"day"`
		Sent   int    `json:"sent"`
		Failed int    `json:"failed"`
	}
	type monthLine struct {
		Month      string `json:"month"`
		Signatures int    `json:"signatures"`
	}
	type overage struct {
		OrgID   string   `json:"orgId"`
		Name    string   `json:"name"`
		Reasons []string `json:"reasons"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Identity == nil {
			writeError(w, http.StatusServiceUnavailable, "annuaire indisponible")
			return
		}

		entries, err := deps.Identity.ListOrgs(r.Context())
		if err != nil {
			deps.Log.Error("annuaire", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		now := deps.Now()
		byPlan := map[string]*planLine{}
		for _, plan := range billing.Plans {
			byPlan[plan.Code] = &planLine{Code: plan.Code, Label: plan.Label}
		}

		var clients, mrr, learners, files, sessions, signatures int
		var videoMs, storageMB int64
		overages := make([]overage, 0, 4)

		for _, entry := range entries {
			// L'organisation de l'équipe n'est pas un client.
			if entry.OrgID == session.OrgID {
				continue
			}
			clients++

			plan, err := billing.PlanByCode(entry.Plan)
			if err != nil {
				plan = billing.Plan{Code: entry.Plan, Label: entry.Plan}
			}
			mrr += plan.PriceCents
			if line, ok := byPlan[plan.Code]; ok {
				line.Orgs++
				line.MRRCents += plan.PriceCents
			}

			if deps.Billing == nil {
				continue
			}
			usage, err := deps.Billing.Usage(r.Context(), entry.OrgID)
			if err != nil {
				continue
			}
			learners += usage.Learners
			files += usage.Files
			sessions += usage.Sessions
			signatures += usage.Signatures
			videoMs += usage.VideoMs
			storageMB += usage.StorageMB

			if reasons := usage.Overage(plan); len(reasons) > 0 {
				overages = append(overages, overage{OrgID: entry.OrgID, Name: entry.Name, Reasons: reasons})
			}
		}

		plans := make([]planLine, 0, len(billing.Plans))
		for _, plan := range billing.Plans {
			plans = append(plans, *byPlan[plan.Code])
		}

		// Courriels des trente derniers jours, jour par jour. Les jours sans
		// envoi valent zéro et restent dans la série : une courbe qui saute
		// les jours creux ment sur le rythme.
		days := make([]dayLine, 0, 30)
		index := map[string]int{}
		for offset := 29; offset >= 0; offset-- {
			day := now.AddDate(0, 0, -offset).Format("2006-01-02")
			index[day] = len(days)
			days = append(days, dayLine{Day: day})
		}
		if deps.MailJournal != nil {
			journal, err := deps.MailJournal.Recent(r.Context(), now, "", "", 2000)
			if err == nil {
				for _, entry := range journal {
					position, ok := index[entry.SentAt.Format("2006-01-02")]
					if !ok {
						continue
					}
					if entry.Delivered {
						days[position].Sent++
					} else {
						days[position].Failed++
					}
				}
			}
		}

		// Signatures des six derniers mois : les compteurs mensuels de chaque
		// organisation, additionnés.
		months := make([]monthLine, 0, 6)
		for offset := 5; offset >= 0; offset-- {
			at := now.AddDate(0, -offset, 0)
			line := monthLine{Month: at.Format("2006-01")}
			if deps.Billing != nil {
				for _, entry := range entries {
					if entry.OrgID == session.OrgID {
						continue
					}
					line.Signatures += deps.Billing.SignaturesIn(r.Context(), entry.OrgID, at)
				}
			}
			months = append(months, line)
		}

		sort.Slice(overages, func(i, j int) bool { return overages[i].Name < overages[j].Name })

		writeJSON(w, http.StatusOK, map[string]any{
			"clients":            clients,
			"mrrCents":           mrr,
			"learners":           learners,
			"files":              files,
			"sessions":           sessions,
			"signatures":         signatures,
			"videoHours":         int(videoMs / 3_600_000),
			"storageGb":          int(storageMB / 1024),
			"plans":              plans,
			"emailsPerDay":       days,
			"signaturesPerMonth": months,
			"overages":           list(overages),
			"generatedAt":        now.Format(time.RFC3339),
		})
	}
}
