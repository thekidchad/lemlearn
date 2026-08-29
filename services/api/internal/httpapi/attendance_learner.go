package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/attendance"
)

// Émargement par l'apprenant.
//
// Jusqu'ici, la présence était cochée par l'organisme depuis la feuille de
// session. Une feuille signée du seul organisme n'atteste pourtant que ses
// propres déclarations : devant un financeur, c'est la signature du stagiaire
// qui fait la pièce. Ces deux routes la lui donnent, sans lui ouvrir quoi que
// ce soit d'autre.
//
// La fiche émargée n'est jamais lue dans la requête : elle vient du compte
// connecté. Sans cela, un apprenant pourrait émarger pour un camarade absent —
// ce qui est précisément la fraude que l'émargement existe pour empêcher.

// slotView est un créneau tel que le voit l'apprenant.
type slotView struct {
	ID    string    `json:"id"`
	Label string    `json:"label"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Hours float64   `json:"hours"`

	// Signed porte l'émargement déjà enregistré, quelle qu'en soit l'origine :
	// une présence établie par le formateur doit s'afficher comme acquise, pas
	// comme une action encore à faire.
	Signed   bool       `json:"signed"`
	SignedAt *time.Time `json:"signedAt,omitempty"`
	Method   string     `json:"method,omitempty"`

	// Signable et Reason disent si le bouton est actif, et sinon pourquoi. Un
	// bouton grisé sans explication est la première cause d'appel au support.
	Signable bool   `json:"signable"`
	Reason   string `json:"reason,omitempty"`
}

// handleLearnerSheet renvoie les créneaux de l'apprenant et leur état.
func handleLearnerSheet(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Attendance == nil {
			writeError(w, http.StatusServiceUnavailable, "émargement indisponible")
			return
		}

		target, _, err := learnerTarget(deps, r)
		if err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if _, err := deps.Catalog.GetEnrollment(r.Context(), session.OrgID,
			target.SessionID, target.ContactID); err != nil {
			writeError(w, http.StatusForbidden, "aucune inscription à cette session")
			return
		}

		sheet, err := deps.Attendance.EnsureSheet(r.Context(), session.OrgID, target.SessionID)
		if err != nil {
			respondNotFound(w, err, "feuille d'émargement introuvable")
			return
		}
		entries, err := deps.Attendance.Entries(r.Context(), session.OrgID, target.SessionID)
		if err != nil {
			deps.Log.Error("émargements", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		signed := map[string]attendance.Entry{}
		for _, entry := range entries {
			if entry.ContactID == target.ContactID {
				signed[entry.SlotID] = entry
			}
		}

		now := deps.Now()
		views := make([]slotView, 0, len(sheet.Slots))
		for _, slot := range sheet.Slots {
			view := slotView{
				ID: slot.ID, Label: slot.Label,
				Start: slot.Start, End: slot.End, Hours: slot.Hours(),
			}
			if entry, ok := signed[slot.ID]; ok {
				at := entry.SignedAt
				view.Signed, view.SignedAt, view.Method = true, &at, string(entry.Method)
			} else {
				view.Signable, view.Reason = attendance.LearnerCanSign(sheet.Mode, slot, now)
			}
			views = append(views, view)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"mode":              string(sheet.Mode),
			"slots":             views,
			"trainerSignedAt":   sheet.TrainerSignedAt,
			"trainerName":       sheet.TrainerName,
			"opensBeforeMinute": int(attendance.SignOpensBefore.Minutes()),
		})
	}
}

// handleLearnerSign enregistre l'émargement d'un apprenant pour un créneau.
func handleLearnerSign(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Attendance == nil {
			writeError(w, http.StatusServiceUnavailable, "émargement indisponible")
			return
		}

		target, user, err := learnerTarget(deps, r)
		if err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		enrollment, err := deps.Catalog.GetEnrollment(r.Context(), session.OrgID,
			target.SessionID, target.ContactID)
		if err != nil {
			writeError(w, http.StatusForbidden, "aucune inscription à cette session")
			return
		}

		sheet, err := deps.Attendance.EnsureSheet(r.Context(), session.OrgID, target.SessionID)
		if err != nil {
			respondNotFound(w, err, "feuille d'émargement introuvable")
			return
		}

		slotID := chi.URLParam(r, "slotID")
		var slot attendance.Slot
		found := false
		for _, candidate := range sheet.Slots {
			if candidate.ID == slotID {
				slot, found = candidate, true
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, "créneau inconnu pour cette session")
			return
		}

		// La fenêtre est revérifiée ici, et pas seulement à l'affichage : un
		// écran laissé ouvert toute la journée présenterait un bouton actif
		// bien après la clôture.
		if ok, reason := attendance.LearnerCanSign(sheet.Mode, slot, deps.Now()); !ok {
			writeError(w, http.StatusConflict, reason)
			return
		}

		entry, err := deps.Attendance.Sign(r.Context(), attendance.SignInput{
			OrgID: session.OrgID, SessionID: target.SessionID, SlotID: slot.ID,
			// La fiche vient du compte, jamais de la requête.
			ContactID: target.ContactID,
			FileID:    enrollment.FileID,
			Method:    attendance.MethodSignature,
			IP:        clientIP(r),
			UserAgent: truncateUA(r.UserAgent()),
			// L'auteur est l'apprenant lui-même : c'est ce qui distingue, au
			// journal d'audit, une présence signée d'une présence constatée.
			Actor: actorFrom(r, user.ID),
		})
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"slotId":   entry.SlotID,
			"signedAt": entry.SignedAt,
			"label":    slot.Label,
			"hours":    slot.Hours(),
		})
	}
}
