package httpapi

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/crm"
)

// handleAnonymize efface les données personnelles d'un contact.
//
// Réservé aux administrateurs, et journalisé sur chaque dossier où la personne
// apparaît : un effacement est un acte de gestion dont l'organisme doit
// pouvoir rendre compte, pas une suppression discrète.
func handleAnonymize(deps Deps) http.HandlerFunc {
	type request struct {
		Reason string `json:"reason"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}

		contact, err := deps.CRM.Anonymize(r.Context(), crm.AnonymizeInput{
			OrgID: session.OrgID, ContactID: chi.URLParam(r, "contactID"),
			Reason: body.Reason, Actor: actorFrom(r, user.FullName()),
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"contact": contact,
			"note": "Les pièces à valeur probante sont conservées le temps légal, " +
				"rattachées au pseudonyme.",
		})
	}
}

// handlePortability remet à une personne tout ce que l'organisation détient
// sur elle.
func handlePortability(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		contactID := chi.URLParam(r, "contactID")

		// Un apprenant peut demander ses propres données ; un administrateur
		// peut les extraire pour lui. Personne ne peut demander celles d'un
		// autre apprenant.
		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}
		if !user.Role.CanManageCRM() && user.ContactID != contactID {
			writeError(w, http.StatusForbidden, "vous ne pouvez extraire que vos propres données")
			return
		}

		data, err := deps.CRM.Portability(r.Context(), session.OrgID, contactID)
		if err != nil {
			respondNotFound(w, err, "contact introuvable")
			return
		}

		w.Header().Set("Content-Disposition",
			fmt.Sprintf(`attachment; filename="donnees-%s.json"`, contactID))
		writeJSON(w, http.StatusOK, data)
	}
}

// handleQualiopiDashboard résume l'état de conformité de l'organisation.
//
// Ce que regarde un dirigeant avant un audit : combien de dossiers sont
// complets, lesquels ne le sont pas, et pourquoi.
func handleQualiopiDashboard(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		pipeline, err := deps.CRM.Pipeline(r.Context(), session.OrgID, 200)
		if err != nil {
			deps.Log.Error("tableau de bord", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		var total, complete int
		manquants := map[string]int{}
		gaps := make([]qualiopiGap, 0, 16)

		for _, files := range pipeline {
			for _, file := range files {
				total++
				percent := 0
				if file.Proof.Expected > 0 {
					percent = file.Proof.Present * 100 / file.Proof.Expected
				}
				if percent == 100 {
					complete++
					continue
				}
				for _, piece := range file.Proof.Missing {
					manquants[piece]++
				}
				gaps = append(gaps, qualiopiGap{
					Reference: file.Reference, Title: file.Title,
					Percent: percent, Missing: file.Proof.Missing,
				})
			}
		}

		// Les dossiers les plus incomplets d'abord : c'est par eux qu'un audit
		// commence, et c'est sur eux qu'un organisme doit travailler.
		sort.Slice(gaps, func(i, j int) bool { return gaps[i].Percent < gaps[j].Percent })

		writeJSON(w, http.StatusOK, map[string]any{
			"dossiers":           total,
			"complets":           complete,
			"tauxDeCompletude":   percentOf(complete, total),
			"piecesManquantes":   manquants,
			"dossiersIncomplets": firstN(gaps, 20),
		})
	}
}

// qualiopiGap est un dossier dont le dossier de preuve est incomplet.
type qualiopiGap struct {
	Reference string   `json:"reference"`
	Title     string   `json:"title"`
	Percent   int      `json:"percent"`
	Missing   []string `json:"missing"`
}

func percentOf(part, total int) int {
	if total == 0 {
		return 0
	}
	return part * 100 / total
}

func firstN[T any](items []T, n int) []T {
	if len(items) <= n {
		return items
	}
	return items[:n]
}
