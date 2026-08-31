package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/platform/audit"
)

// La sortie du brouillon d'une formation.
//
// Le champ existait au modèle, l'écran l'affichait, et rien ne permettait d'en
// changer : une formation créée en brouillon y restait pour toujours.

// handlePublishCourse fait sortir une formation du brouillon, ou l'y remet.
//
// Le brouillon n'est pas une coquetterie : une formation sert de base à une
// convention et à un programme, deux pièces qui engagent. Tant qu'elle est en
// brouillon, on peut la corriger sans se demander qui l'a déjà reçue.
//
// Dépublier reste possible et ne défait rien : les sessions ouvertes continuent,
// les conventions signées restent signées. Cela retire la formation de ce qu'on
// propose, pas de ce qui a eu lieu.
func handlePublishCourse(deps Deps) http.HandlerFunc {
	type request struct {
		Published bool `json:"published"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		courseID := chi.URLParam(r, "courseID")

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		course, err := deps.Catalog.GetCourse(r.Context(), session.OrgID, courseID)
		if err != nil {
			respondNotFound(w, err, "formation introuvable")
			return
		}

		// Ce qu'un programme de formation doit porter (art. L.6353-1) : sans
		// ces mentions, la pièce qu'on en tirera sera incomplète, et on ne s'en
		// apercevra qu'au contrôle. On refuse donc de publier, en disant quoi.
		if body.Published {
			if manque := programmeIncomplet(course); len(manque) > 0 {
				writeJSON(w, http.StatusConflict, map[string]any{
					"error":    "le programme est incomplet : " + strings.Join(manque, ", "),
					"manquant": manque,
				})
				return
			}
			if modules, err := deps.Catalog.ListModules(r.Context(), session.OrgID, courseID); err == nil &&
				len(modules) == 0 {
				writeError(w, http.StatusConflict,
					"cette formation n'a aucun module : il n'y aurait rien à suivre")
				return
			}
		}

		saved, err := deps.Catalog.SetPublished(r.Context(), session.OrgID, courseID, body.Published)
		if err != nil {
			deps.Log.Error("publication", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		action := "dépubliée"
		if body.Published {
			action = "publiée"
		}
		if _, err := deps.Identity.AuditOrg(r.Context(), session.OrgID,
			audit.ActionDocumentGenerated, actorFrom(r, session.UserID),
			map[string]any{"formation": saved.Title, "etat": action},
		); err != nil {
			deps.Log.Error("journal de la publication", "err", err)
		}

		writeJSON(w, http.StatusOK, map[string]any{"course": saved})
	}
}

// programmeIncomplet nomme les mentions absentes d'un programme.
//
// Ce sont celles de l'article L.6353-1 : l'objectif, le public, les modalités
// d'évaluation, la sanction et la durée. Un programme sans elles n'est pas un
// programme, et la convention qui s'y adosse est attaquable.
func programmeIncomplet(course catalog.Course) []string {
	manque := make([]string, 0, 5)
	if strings.TrimSpace(course.Goal) == "" {
		manque = append(manque, "l'objectif")
	}
	if strings.TrimSpace(course.Audience) == "" {
		manque = append(manque, "le public visé")
	}
	if strings.TrimSpace(course.Assessment) == "" {
		manque = append(manque, "les modalités d'évaluation")
	}
	if strings.TrimSpace(course.Sanction) == "" {
		manque = append(manque, "la sanction de la formation")
	}
	if course.DurationHours <= 0 {
		manque = append(manque, "la durée")
	}
	return manque
}

// handleUpdateCourse corrige le programme d'une formation.
//
// Il manquait entièrement : une formation était figée dès sa création, et la
// seule façon de corriger une faute de frappe dans un objectif était d'en créer
// une autre. C'est aussi ce qui rendait la publication inatteignable — on ne
// pouvait pas renseigner après coup ce qu'elle exige.
//
// Les champs absents du corps sont laissés tels quels : un PATCH qui écrirait
// le zéro des champs non transmis effacerait la moitié du programme à chaque
// correction de titre.
func handleUpdateCourse(deps Deps) http.HandlerFunc {
	type request struct {
		Title         *string   `json:"title"`
		Goal          *string   `json:"goal"`
		Objectives    *[]string `json:"objectives"`
		Prerequisites *string   `json:"prerequisites"`
		Audience      *string   `json:"audience"`
		Means         *string   `json:"means"`
		Assessment    *string   `json:"assessment"`
		Sanction      *string   `json:"sanction"`
		Accessibility *string   `json:"accessibility"`
		DurationHours *float64  `json:"durationHours"`
		PriceHT       *float64  `json:"priceHT"`
		Tags          *[]string `json:"tags"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		courseID := chi.URLParam(r, "courseID")

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		saved, err := deps.Catalog.UpdateCourse(r.Context(), session.OrgID, courseID,
			func(course *catalog.Course) {
				poser := func(cible *string, valeur *string) {
					if valeur != nil {
						*cible = strings.TrimSpace(*valeur)
					}
				}
				poser(&course.Title, body.Title)
				poser(&course.Goal, body.Goal)
				poser(&course.Prerequisites, body.Prerequisites)
				poser(&course.Audience, body.Audience)
				poser(&course.Means, body.Means)
				poser(&course.Assessment, body.Assessment)
				poser(&course.Sanction, body.Sanction)
				poser(&course.Accessibility, body.Accessibility)
				if body.Objectives != nil {
					course.Objectives = *body.Objectives
				}
				if body.Tags != nil {
					course.Tags = *body.Tags
				}
				if body.DurationHours != nil {
					course.DurationHours = *body.DurationHours
				}
				if body.PriceHT != nil {
					course.PriceHT = *body.PriceHT
				}
			})
		if err != nil {
			respondNotFound(w, err, "formation introuvable")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"course": saved})
	}
}
