package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/attendance"
)

// handleGetSheet renvoie la feuille d'émargement d'une session, créneaux et
// présences réunis — c'est la grille que voit le formateur.
func handleGetSheet(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Attendance == nil {
			writeError(w, http.StatusServiceUnavailable, "émargement indisponible")
			return
		}
		sessionID := chi.URLParam(r, "sessionID")

		sheet, err := deps.Attendance.EnsureSheet(r.Context(), session.OrgID, sessionID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		entries, err := deps.Attendance.Entries(r.Context(), session.OrgID, sessionID)
		if err != nil {
			deps.Log.Error("émargement", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		enrollments, err := deps.Catalog.ListSessionEnrollments(r.Context(), session.OrgID, sessionID)
		if err != nil {
			deps.Log.Error("inscrits", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		// Les heures émargées accompagnent la grille : c'est le nombre que
		// l'organisme reporte sur sa facture, il ne doit pas être recalculé
		// à la main par le client.
		hours := make(map[string]float64, len(enrollments))
		for _, enrollment := range enrollments {
			hours[enrollment.ContactID] = attendance.AttendedHours(sheet, entries, enrollment.ContactID)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"sheet": sheet, "entries": entries,
			"enrollments": enrollments, "attendedHours": hours,
		})
	}
}

// handleSignAttendance enregistre une présence sur un créneau.
func handleSignAttendance(deps Deps) http.HandlerFunc {
	type request struct {
		SlotID    string            `json:"slotId"`
		ContactID string            `json:"contactId"`
		FileID    string            `json:"fileId"`
		Method    attendance.Method `json:"method"`
		Coverage  int               `json:"coveragePercent"`
		Comment   string            `json:"comment"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Attendance == nil {
			writeError(w, http.StatusServiceUnavailable, "émargement indisponible")
			return
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}

		// Un apprenant ne peut émarger que pour lui-même. Seul un formateur
		// peut consigner la présence d'un tiers — et c'est bien son nom qui
		// figurera au journal, pas celui de l'apprenant.
		contactID := body.ContactID
		if !user.Role.CanTeach() {
			contactID = user.ContactID
			if contactID == "" {
				writeError(w, http.StatusForbidden, "ce compte n'est rattaché à aucune fiche apprenant")
				return
			}
		}

		entry, err := deps.Attendance.Sign(r.Context(), attendance.SignInput{
			OrgID: session.OrgID, SessionID: chi.URLParam(r, "sessionID"),
			SlotID: body.SlotID, ContactID: contactID, FileID: body.FileID,
			Method: body.Method, Coverage: body.Coverage, Comment: body.Comment,
			IP: clientIP(r), UserAgent: truncateUA(r.UserAgent()),
			Actor: actorFrom(r, user.FullName()),
		})
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, entry)
	}
}

// handleCountersign clôt la feuille par la signature du formateur.
func handleCountersign(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Attendance == nil {
			writeError(w, http.StatusServiceUnavailable, "émargement indisponible")
			return
		}

		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}
		if !user.Role.CanTeach() {
			writeError(w, http.StatusForbidden, "seul un formateur peut contresigner")
			return
		}

		sheet, err := deps.Attendance.Countersign(r.Context(), session.OrgID,
			chi.URLParam(r, "sessionID"), user, actorFrom(r, user.FullName()))
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, sheet)
	}
}
