package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/crm"
)

// Contenu d'un organisme, vu par l'équipe.
//
// C'est un changement de posture assumé : jusqu'ici le support ne pouvait
// retrouver un apprenant que sur son adresse exacte, précisément pour qu'il ne
// puisse pas parcourir les données de tous les clients. Ces routes lèvent cette
// borne, organisme par organisme.
//
// Elles n'ouvrent pas un balayage de la table pour autant : chaque appel
// interroge la partition d'un organisme désigné, comme le ferait ce client
// lui-même. Le coût reste celui d'une lecture ciblée, et l'accès reste nommé —
// on sait toujours de quel organisme on regarde les données.
//
// Elles sont en lecture seule. Modifier chez un client passe par
// l'impersonation, qui laisse une trace dans son propre journal d'audit et un
// bandeau à son écran : un support qui peut écrire sans que le client le sache
// n'est pas un support, c'est une porte dérobée.

func handleAdminOrgContacts(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := chi.URLParam(r, "orgID")

		kind := crm.Kind(r.URL.Query().Get("kind"))
		if kind == "" {
			kind = crm.KindLearner
		}

		limite, curseur := pageParams(r)
		page, err := deps.CRM.ListContactsPage(r.Context(), orgID, kind, limite, curseur)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// La consultation est journalisée sur l'organisation concernée : lire
		// les données d'un client ne doit pas être plus discret que d'y écrire.
		session, _ := sessionFrom(r)
		if _, err := deps.Identity.AuditOrg(r.Context(), orgID, "admin.viewed",
			actorFrom(r, session.UserID),
			map[string]any{"nature": string(kind), "lignes": len(page.Items)},
		); err != nil {
			deps.Log.Error("journal de consultation", "err", err)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"contacts": list(page.Items), "cursor": page.Cursor,
		})
	}
}

func handleAdminOrgSessions(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limite, curseur := pageParams(r)
		page, err := deps.Catalog.ListSessionsPage(r.Context(), chi.URLParam(r, "orgID"), limite, curseur)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"sessions": list(page.Items), "cursor": page.Cursor,
		})
	}
}

func handleAdminOrgCourses(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limite, curseur := pageParams(r)
		page, err := deps.Catalog.ListCoursesPage(r.Context(), chi.URLParam(r, "orgID"), limite, curseur)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"courses": list(page.Items), "cursor": page.Cursor,
		})
	}
}

func handleAdminOrgFiles(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := chi.URLParam(r, "orgID")
		stage := crm.Stage(r.URL.Query().Get("etape"))
		if stage == "" {
			stage = crm.StageAgreement
		}

		limite, curseur := pageParams(r)
		page, err := deps.CRM.ListFilesByStagePage(r.Context(), orgID, stage, limite, curseur)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"files": list(page.Items), "cursor": page.Cursor,
		})
	}
}
