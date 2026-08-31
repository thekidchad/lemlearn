package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/platform/audit"
)

// Le journal de la plateforme, dans l'ordre du temps.
//
// Le journal d'audit est rangé par sujet — c'est ce qui permet de vérifier
// qu'une chaîne n'a pas été altérée. Mais ce n'est pas la question qu'on pose
// quand quelque chose cloche : on demande « qu'est-il arrivé aujourd'hui »,
// pas « raconte-moi le dossier 143 ». D'où l'index par jour, et cette lecture
// qui traverse tous les sujets.
//
// La lecture remonte le temps d'elle-même : une journée creuse ne doit pas
// renvoyer un écran vide avec un bouton « encore » à cliquer six fois. Le
// parcours en arrière est borné — au-delà, on rend ce qu'on a et on dit
// jusqu'où on a regardé, plutôt que de tenir la requête ouverte sur une base
// qui n'a rien à dire.
const (
	// joursMax borne le nombre de journées visitées par requête.
	joursMax = 45
	// journalMax borne une page, filtre appliqué.
	journalMax = 100
)

// curseurJournal porte le jour en cours et la position dedans.
type curseurJournal struct {
	Jour  string `json:"j"`
	Inner string `json:"i,omitempty"`
}

func handleAdminJournal(deps Deps) http.HandlerFunc {
	type ligne struct {
		At      string         `json:"at"`
		Action  string         `json:"action"`
		Subject string         `json:"subject"`
		Actor   audit.Actor    `json:"actor"`
		Payload map[string]any `json:"payload,omitempty"`
		Seq     int64          `json:"seq"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		limite, curseur := pageParams(r)
		if limite > journalMax {
			limite = journalMax
		}

		// Les filtres. Ils s'appliquent après lecture : DynamoDB sait filtrer
		// côté serveur, mais il facture et compte la page avant filtrage —
		// une page « pleine » pourrait alors ne contenir aucune ligne retenue,
		// ce qui se lit comme un journal vide.
		action := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action")))
		famille := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("famille")))
		terme := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

		jour := deps.Now().UTC()
		inner := ""
		if curseur != "" {
			var etat curseurJournal
			raw, err := base64.RawURLEncoding.DecodeString(curseur)
			if err != nil || json.Unmarshal(raw, &etat) != nil {
				writeError(w, http.StatusBadRequest, "curseur illisible")
				return
			}
			parsed, err := time.Parse("2006-01-02", etat.Jour)
			if err != nil {
				writeError(w, http.StatusBadRequest, "curseur illisible")
				return
			}
			jour, inner = parsed, etat.Inner
		} else if demande := r.URL.Query().Get("jour"); demande != "" {
			parsed, err := time.Parse("2006-01-02", demande)
			if err != nil {
				writeError(w, http.StatusBadRequest, "date attendue au format AAAA-MM-JJ")
				return
			}
			jour = parsed
		}

		lignes := make([]ligne, 0, limite)
		suivant := ""
		visites := 0
		premier, dernier := jour.Format("2006-01-02"), jour.Format("2006-01-02")

		for len(lignes) < int(limite) && visites < joursMax {
			page, err := deps.Identity.Journal(r.Context(), jour, limite, inner)
			if err != nil {
				deps.Log.Error("journal", "err", err)
				writeError(w, http.StatusInternalServerError, "erreur interne")
				return
			}
			dernier = jour.Format("2006-01-02")

			for _, event := range page.Items {
				if !retenu(event, action, famille, terme) {
					continue
				}
				lignes = append(lignes, ligne{
					At: event.At.Format(time.RFC3339), Action: string(event.Action),
					Subject: event.Subject, Actor: event.Actor,
					Payload: event.Payload, Seq: event.Seq,
				})
			}

			if page.Cursor != "" {
				// La journée n'est pas épuisée : on s'arrête ici si la page est
				// pleine, sinon on continue dans le même jour.
				inner = page.Cursor
				if len(lignes) >= int(limite) {
					suivant = encoderJournal(jour, inner)
					break
				}
				continue
			}

			// Journée terminée : on passe à la veille.
			jour = jour.AddDate(0, 0, -1)
			inner = ""
			visites++
			if len(lignes) >= int(limite) {
				suivant = encoderJournal(jour, "")
				break
			}
		}

		// Un parcours interrompu par la borne rend un curseur : c'est ce qui
		// distingue « il n'y a plus rien » de « je n'ai pas fini de regarder ».
		if suivant == "" && visites >= joursMax {
			suivant = encoderJournal(jour, inner)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"lignes":  lignes,
			"cursor":  suivant,
			"depuis":  premier,
			"jusqua":  dernier,
			"actions": audit.Actions(),
		})
	}
}

// retenu applique les filtres à un événement.
func retenu(event audit.Event, action, famille, terme string) bool {
	nom := strings.ToLower(string(event.Action))
	if action != "" && nom != action {
		return false
	}
	// La famille est le préfixe de l'action : « signature », « auth »,
	// « document ». C'est le filtre qu'on veut quand on cherche un genre
	// d'événement sans savoir lequel exactement.
	if famille != "" && !strings.HasPrefix(nom, famille+".") {
		return false
	}
	if terme == "" {
		return true
	}
	foin := strings.ToLower(strings.Join([]string{
		event.Subject, nom, event.Actor.Label, event.Actor.ID, event.Actor.IP,
		event.Actor.UserAgent,
	}, " "))
	return strings.Contains(foin, terme)
}

// encoderJournal fabrique le curseur : un jour, et une position dedans.
func encoderJournal(jour time.Time, inner string) string {
	raw, err := json.Marshal(curseurJournal{Jour: jour.Format("2006-01-02"), Inner: inner})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}
