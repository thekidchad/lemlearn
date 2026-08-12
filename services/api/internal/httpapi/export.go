package httpapi

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// handleExportFile assemble et renvoie le dossier probatoire.
//
// L'archive est diffusée directement plutôt que déposée puis présignée : un
// dossier pèse quelques mégaoctets, l'assemblage prend moins d'une seconde, et
// un lien temporaire de plus serait un lien de plus à sécuriser.
func handleExportFile(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Export == nil {
			writeError(w, http.StatusServiceUnavailable, "export indisponible")
			return
		}

		fileID := chi.URLParam(r, "fileID")
		file, err := deps.CRM.GetFile(r.Context(), session.OrgID, fileID)
		if err != nil {
			respondNotFound(w, err, "dossier introuvable")
			return
		}
		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}

		archive, manifest, err := deps.Export.Build(r.Context(), session.OrgID, fileID)
		if err != nil {
			// Une chaîne d'audit rompue interrompt l'export : livrer un
			// dossier dont on sait que le journal a été altéré serait pire
			// que de ne rien livrer.
			deps.Log.Error("export", "file", fileID, "err", err)
			writeError(w, http.StatusConflict, err.Error())
			return
		}

		// L'export est lui-même un événement de la chaîne : savoir qui a
		// extrait un dossier, et quand, fait partie de ce qu'un contrôle peut
		// demander.
		if _, auditErr := deps.CRM.RecordExport(r.Context(), session.OrgID, fileID,
			actorFrom(r, user.FullName()),
			map[string]any{
				"pieces":  len(manifest.Entries),
				"missing": len(manifest.Missing),
				"bytes":   len(archive),
				"events":  manifest.AuditEvents,
			}); auditErr != nil {
			deps.Log.Error("journalisation de l'export", "err", auditErr)
		}

		name := fmt.Sprintf("dossier-%s.zip", file.Reference)
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
		// Le manifeste est repris en en-tête : un client peut afficher les
		// pièces manquantes sans ouvrir l'archive.
		w.Header().Set("X-Lemlearn-Pieces", strconv.Itoa(len(manifest.Entries)))
		w.Header().Set("X-Lemlearn-Missing", strconv.Itoa(len(manifest.Missing)))
		_, _ = w.Write(archive)
	}
}
