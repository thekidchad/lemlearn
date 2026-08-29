package httpapi

import (
	"net/http"
	"sort"
	"strings"

	"github.com/lemlearn/api/internal/emailtpl"
)

// Recherche unique de la vue super-admin.
//
// Un seul appel plutôt qu'un par catégorie : la palette interroge à chaque
// frappe, et quatre requêtes par touche coûteraient plus cher que tout le
// reste de l'écran réuni.
//
// Les catégories n'ont pas les mêmes règles, et c'est délibéré :
//
//   - Les organisations et la bibliothèque sont nos propres données. On y
//     cherche sur un fragment, comme partout ailleurs.
//   - Les apprenants ne se cherchent que sur leur adresse exacte. Parcourir
//     les contacts de tous les clients pour trouver « Marie » coûterait un
//     balayage complet, et donnerait surtout au support la capacité de lister
//     les apprenants de n'importe qui — dans un produit qui vend la protection
//     des données, c'est exactement ce qu'on ne doit pas pouvoir faire.
//
// Quand la requête n'est pas une adresse, on le dit plutôt que de rendre une
// liste vide : un résultat absent sans explication se lit comme une panne.

// searchLimit borne chaque catégorie. Au-delà, on ne cherche plus, on
// parcourt — et ce n'est plus le bon outil.
const searchLimit = 6

func handleAdminSearch(deps Deps) http.HandlerFunc {
	type orgHit struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Plan string `json:"plan"`
	}
	type learnerHit struct {
		OrgID      string `json:"orgId"`
		OrgName    string `json:"orgName"`
		Name       string `json:"name"`
		Email      string `json:"email"`
		HasAccount bool   `json:"hasAccount"`
	}
	type courseHit struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	type templateHit struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if len(query) < 2 {
			writeJSON(w, http.StatusOK, map[string]any{})
			return
		}
		needle := strings.ToLower(query)

		orgs := make([]orgHit, 0, searchLimit)
		learners := make([]learnerHit, 0, 2)
		courses := make([]courseHit, 0, searchLimit)
		templates := make([]templateHit, 0, searchLimit)
		hint := ""

		if deps.Identity != nil {
			if directory, err := deps.Identity.ListOrgs(r.Context()); err == nil {
				for _, entry := range directory {
					if len(orgs) >= searchLimit {
						break
					}
					if strings.Contains(strings.ToLower(entry.Name), needle) {
						orgs = append(orgs, orgHit{ID: entry.OrgID, Name: entry.Name, Plan: entry.Plan})
					}
				}
				sort.Slice(orgs, func(i, j int) bool { return orgs[i].Name < orgs[j].Name })
			} else {
				deps.Log.Error("annuaire", "err", err)
			}
		}

		// L'apprenant ne se retrouve que par son adresse complète. Sans
		// arobase, on n'interroge rien et on explique pourquoi.
		if strings.Contains(needle, "@") {
			if user, err := deps.Identity.UserByEmail(r.Context(), needle); err == nil {
				org, _ := deps.Identity.LoadOrg(r.Context(), user.OrgID)
				learners = append(learners, learnerHit{
					OrgID: user.OrgID, OrgName: org.Name,
					Name:  strings.TrimSpace(user.FirstName + " " + user.LastName),
					Email: user.Email, HasAccount: true,
				})
			}
		}

		if deps.Library != nil {
			if published, err := deps.Library.ListCourses(r.Context(), false); err == nil {
				for _, course := range published {
					if len(courses) >= searchLimit {
						break
					}
					if strings.Contains(strings.ToLower(course.Title), needle) {
						courses = append(courses, courseHit{ID: course.ID, Title: course.Title})
					}
				}
			}
		}

		for _, definition := range emailtpl.Defaults() {
			if len(templates) >= searchLimit {
				break
			}
			if strings.Contains(strings.ToLower(definition.Label), needle) ||
				strings.Contains(strings.ToLower(definition.Key), needle) {
				templates = append(templates, templateHit{Key: definition.Key, Label: definition.Label})
			}
		}

		// L'indication ne s'affiche que si rien d'autre n'a répondu. La
		// rappeler sous des résultats pertinents en ferait un reproche
		// permanent, alors qu'elle ne sert qu'à expliquer un vide.
		if !strings.Contains(needle, "@") &&
			len(orgs)+len(courses)+len(templates) == 0 {
			hint = "Pour retrouver un apprenant, saisissez son adresse complète."
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"organisations": orgs,
			"apprenants":    learners,
			"formations":    courses,
			"gabarits":      templates,
			"hint":          hint,
		})
	}
}
