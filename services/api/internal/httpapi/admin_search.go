package httpapi

import (
	"net/http"
	"sort"
	"strings"

	"github.com/lemlearn/api/internal/emailtpl"
	"github.com/lemlearn/api/internal/platform/audit"
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
//   - Les fiches appartiennent aux clients. On les cherche sur un fragment
//     aussi, parce qu'on tape le nom qu'on entend au téléphone et non une
//     adresse complète — mais chaque organisme dont une fiche ressort le voit
//     dans son propre journal. La contrepartie d'un accès étendu n'est pas de
//     le restreindre au point de le rendre inutile : c'est de le rendre
//     visible de celui qu'il concerne.

// searchLimit borne chaque catégorie. Au-delà, on ne cherche plus, on
// parcourt — et ce n'est plus le bon outil.
const searchLimit = 6

func handleAdminSearch(deps Deps) http.HandlerFunc {
	type orgHit struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Plan string `json:"plan"`
	}
	type contactHit struct {
		OrgID     string `json:"orgId"`
		OrgName   string `json:"orgName"`
		ContactID string `json:"contactId"`
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		Email     string `json:"email"`
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
		contacts := make([]contactHit, 0, searchLimit)
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

		// Les fiches de tous les organismes, sur un fragment de nom, d'adresse
		// ou de SIRET. Chaque organisme dont une fiche ressort est journalisé :
		// chercher quelqu'un dans les données d'un client est un accès, pas une
		// consultation d'annuaire.
		if deps.CRM != nil && deps.Identity != nil {
			touches := map[string]bool{}
			if directory, err := deps.Identity.ListOrgs(r.Context()); err == nil {
				for _, entry := range directory {
					if len(contacts) >= searchLimit {
						break
					}
					found, err := deps.CRM.SearchContacts(r.Context(), entry.OrgID, needle, searchLimit)
					if err != nil {
						deps.Log.Error("recherche de fiches", "org", entry.OrgID, "err", err)
						continue
					}
					for _, contact := range found {
						if len(contacts) >= searchLimit {
							break
						}
						contacts = append(contacts, contactHit{
							OrgID: entry.OrgID, OrgName: entry.Name,
							ContactID: contact.ID, Kind: string(contact.Kind),
							Name: contact.DisplayName(), Email: contact.Email,
						})
						touches[entry.OrgID] = true
					}
				}
			}

			session, _ := sessionFrom(r)
			for orgID := range touches {
				if _, err := deps.Identity.AuditOrg(r.Context(), orgID,
					audit.ActionImpersonated, actorFrom(r, session.UserID),
					map[string]any{"recherche": "fiche", "terme": query},
				); err != nil {
					deps.Log.Error("journal de la recherche", "err", err)
				}
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
		if len(orgs)+len(contacts)+len(courses)+len(templates) == 0 {
			hint = "Rien ne porte ce nom : ni organisme, ni fiche, ni formation."
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"organisations": orgs,
			"contacts":      contacts,
			"formations":    courses,
			"gabarits":      templates,
			"hint":          hint,
		})
	}
}
