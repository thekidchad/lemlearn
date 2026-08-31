package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/lemlearn/api/internal/crm"
)

// Vue de toute la plateforme, tous organismes confondus.
//
// Il n'existe pas de requête « tous les stagiaires » : les données sont
// rangées par organisation, et c'est cette séparation qui garantit qu'un client
// ne peut pas lire un autre. Balayer la table entière la contournerait, et
// coûterait de plus en plus cher à mesure que le produit marche.
//
// On parcourt donc l'annuaire — qui est la liste de nos clients, donc bornée et
// connue — et on interroge la partition de chacun. Le curseur porte les deux
// niveaux : à quel organisme on en était, et où dans cet organisme. C'est ce
// qui permet de reprendre exactement là où l'on s'est arrêté sans tout relire.

// ligne est ce qu'on affiche, quelle que soit la nature.
//
// Une forme unique plutôt que six : l'écran n'a pas à savoir qu'une session a
// une date et un stagiaire une adresse, et six formes voudraient dire six
// rendus à maintenir.
type ligne struct {
	OrgID   string `json:"orgId"`
	OrgName string `json:"orgName"`
	ID      string `json:"id"`
	Label   string `json:"label"`
	Detail  string `json:"detail,omitempty"`

	// Ce qui rend la ligne actionnable. Entrer dans le compte d'un stagiaire
	// suppose qu'il en ait un — un contact invité mais jamais connecté n'a pas
	// de mot de passe, et il n'y a rien à ouvrir. Exporter suppose un dossier.
	// Les deux sont résolus ici plutôt que devinés par l'écran, qui n'a pas de
	// quoi le faire.
	CanImpersonate bool   `json:"canImpersonate,omitempty"`
	FileID         string `json:"fileId,omitempty"`
	FileReference  string `json:"fileReference,omitempty"`
}

type curseurGlobal struct {
	Org string `json:"org"`
	// Etape n'a de sens que pour les dossiers : ils sont rangés par étape du
	// pipeline, et une vue « tous les dossiers » doit les parcourir toutes.
	// Sans ce niveau, l'écran affichait zéro dossier alors que la plateforme en
	// contenait — un vide qui se lit comme une panne.
	Etape string `json:"etape,omitempty"`
	Inner string `json:"inner"`
}

// etapes énumère le pipeline dans l'ordre du parcours commercial.
var etapes = []crm.Stage{
	crm.StageProspect, crm.StageQuote, crm.StageAgreement,
	crm.StageInTraining, crm.StageClosed,
}

func handleAdminAll(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vue := r.URL.Query().Get("vue")
		if vue == "" {
			vue = "stagiaires"
		}
		limite, curseur := pageParams(r)

		orgs, err := deps.Identity.ListOrgs(r.Context())
		if err != nil {
			deps.Log.Error("annuaire", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		// L'ordre doit être stable d'un appel à l'autre, sans quoi le curseur
		// désignerait un organisme différent à chaque fois. L'identifiant est
		// immuable ; le nom, lui, se modifie.
		sort.Slice(orgs, func(i, j int) bool { return orgs[i].OrgID < orgs[j].OrgID })

		depart, etape, interne := "", "", ""
		if curseur != "" {
			var reprise curseurGlobal
			raw, err := base64.RawURLEncoding.DecodeString(curseur)
			if err != nil || json.Unmarshal(raw, &reprise) != nil {
				writeError(w, http.StatusBadRequest, "curseur illisible")
				return
			}
			depart, etape, interne = reprise.Org, reprise.Etape, reprise.Inner
		}

		lignes := make([]ligne, 0, limite)
		suivant := ""

		for _, org := range orgs {
			if depart != "" && org.OrgID < depart {
				continue
			}
			// Le curseur interne n'appartient qu'à l'organisme où l'on s'est
			// arrêté : le rejouer sur le suivant désignerait une clé d'une
			// autre partition.
			inner, etapeDepart := "", ""
			if org.OrgID == depart {
				inner, etapeDepart = interne, etape
			}

			// Les dossiers se parcourent étape par étape ; les autres natures
			// n'ont qu'un seul passage.
			passages := []string{""}
			if vue == "dossiers" {
				passages = passages[:0]
				for _, e := range etapes {
					passages = append(passages, string(e))
				}
			}

			for _, passage := range passages {
				if etapeDepart != "" && passage < etapeDepart {
					continue
				}
				curseurPassage := ""
				if passage == etapeDepart {
					curseurPassage = inner
				}

				for {
					reste := limite - int32(len(lignes))
					if reste <= 0 {
						break
					}

					page, cursor, err := lireVue(r, deps, vue, org.OrgID, passage, reste, curseurPassage)
					if err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
					for _, l := range page {
						l.OrgID, l.OrgName = org.OrgID, org.Name
						lignes = append(lignes, l)
					}

					curseurPassage = cursor
					if cursor == "" {
						break
					}
				}

				if int32(len(lignes)) >= limite && curseurPassage != "" {
					encoded, _ := json.Marshal(curseurGlobal{
						Org: org.OrgID, Etape: passage, Inner: curseurPassage,
					})
					suivant = base64.RawURLEncoding.EncodeToString(encoded)
					break
				}
			}

			if suivant != "" {
				break
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"lignes":     lignes,
			"cursor":     suivant,
			"organismes": len(orgs),
			"vue":        vue,
		})
	}
}

// lireVue interroge une nature dans un organisme et la projette en lignes.
func lireVue(r *http.Request, deps Deps, vue, orgID, etape string, limite int32, curseur string) ([]ligne, string, error) {
	switch vue {
	case "stagiaires", "entreprises", "financeurs":
		kinds := map[string]crm.Kind{
			"stagiaires":  crm.KindLearner,
			"entreprises": crm.KindCompany,
			"financeurs":  crm.KindFunder,
		}
		page, err := deps.CRM.ListContactsPage(r.Context(), orgID, kinds[vue], limite, curseur)
		if err != nil {
			return nil, "", err
		}
		out := make([]ligne, 0, len(page.Items))
		for _, c := range page.Items {
			l := ligne{
				ID:    c.ID,
				Label: c.DisplayName(),
				Detail: strings.TrimSpace(strings.Trim(strings.Join([]string{
					c.Email, c.Phone,
				}, " · "), " ·")),
			}
			// Un compte existe-t-il pour cette adresse ? C'est ce qui décide si
			// l'on peut entrer dans son espace.
			if c.Email != "" {
				if user, err := deps.Identity.UserByEmail(r.Context(), c.Email); err == nil && !user.Disabled {
					l.CanImpersonate = true
				}
			}
			// Son dossier, s'il en a un : c'est lui qui s'exporte. On passe
			// par ses inscriptions, qui sont indexées par apprenant — les
			// dossiers, eux, ne le sont que par étape du pipeline, et les
			// parcourir toutes pour chaque ligne multiplierait les lectures
			// par cinq.
			if enrollments, err := deps.Catalog.ListLearnerEnrollments(r.Context(), orgID, c.ID); err == nil {
				for _, e := range enrollments {
					if e.FileID != "" {
						if file, err := deps.CRM.GetFile(r.Context(), orgID, e.FileID); err == nil {
							l.FileID, l.FileReference = file.ID, file.Reference
						}
						break
					}
				}
			}
			out = append(out, l)
		}
		return out, page.Cursor, nil

	case "formations":
		page, err := deps.Catalog.ListCoursesPage(r.Context(), orgID, limite, curseur)
		if err != nil {
			return nil, "", err
		}
		out := make([]ligne, 0, len(page.Items))
		for _, c := range page.Items {
			out = append(out, ligne{ID: c.ID, Label: c.Title})
		}
		return out, page.Cursor, nil

	case "sessions":
		page, err := deps.Catalog.ListSessionsPage(r.Context(), orgID, limite, curseur)
		if err != nil {
			return nil, "", err
		}
		out := make([]ligne, 0, len(page.Items))
		for _, s := range page.Items {
			out = append(out, ligne{
				ID: s.ID, Label: s.Title,
				Detail: s.StartsAt.Format("02/01/2006"),
			})
		}
		return out, page.Cursor, nil

	case "dossiers":
		page, err := deps.CRM.ListFilesByStagePage(r.Context(), orgID, crm.Stage(etape), limite, curseur)
		if err != nil {
			return nil, "", err
		}
		out := make([]ligne, 0, len(page.Items))
		for _, f := range page.Items {
			out = append(out, ligne{ID: f.ID, Label: f.Title, Detail: f.Reference})
		}
		return out, page.Cursor, nil
	}
	return nil, "", errVueInconnue
}

var errVueInconnue = &vueInconnue{}

type vueInconnue struct{}

func (*vueInconnue) Error() string {
	return "vue inconnue : stagiaires, entreprises, financeurs, formations, sessions ou dossiers"
}

func orDefault(valeur, defaut string) string {
	if strings.TrimSpace(valeur) == "" {
		return defaut
	}
	return valeur
}
