package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/bpf"
	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/platform/audit"
)

// Attribuer une formation à quelqu'un.
//
// C'est le geste que fait un organisme, et il n'existait pas. Le produit
// n'offrait que les trois actes techniques qui le composent — ouvrir un
// dossier, créer une session, inscrire — dispersés sur trois écrans, dont un
// qu'il fallait deviner : on créait une fiche, puis on cherchait le bouton
// « inscrire » sur la fiche, où il n'était pas, avant de comprendre qu'il
// fallait passer par Sessions.
//
// Le geste métier est ici recomposé en une seule route. Elle assemble ce qui
// manque et réutilise ce qui existe : une session ouverte de la même formation
// est reprise plutôt que dédoublée, et un dossier déjà ouvert pour ce stagiaire
// sur cette formation aussi. Sans quoi inscrire deux fois la même personne
// fabriquerait deux dossiers concurrents pour une seule formation.

// handleAttribuerFormation inscrit un stagiaire à une formation du catalogue.
func handleAttribuerFormation(deps Deps) http.HandlerFunc {
	type request struct {
		CourseID string `json:"courseId"`
		// SessionID reprend une session existante. Vide, une session est
		// ouverte aux dates données.
		SessionID string `json:"sessionId"`
		StartsAt  string `json:"startsAt"`
		EndsAt    string `json:"endsAt"`
		Mode      string `json:"mode"`

		// Le dossier. Sans lui l'inscription existe, mais n'alimente aucune
		// chaîne de preuve — c'est le dossier qui porte la convention et
		// l'export probatoire.
		SansDossier bool    `json:"sansDossier"`
		PriceHT     float64 `json:"priceHT"`
		Funding     string  `json:"funding"`
		CompanyID   string  `json:"companyId"`
		FunderID    string  `json:"funderId"`

		// Ce que réclameront la convention et le bilan.
		TraineeType    string  `json:"traineeType"`
		HoursElearning float64 `json:"hoursElearning"`
		HoursRemote    float64 `json:"hoursRemote"`
		HoursOnSite    float64 `json:"hoursOnSite"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		contactID := chi.URLParam(r, "contactID")

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		if strings.TrimSpace(body.CourseID) == "" {
			writeError(w, http.StatusBadRequest, "choisissez une formation du catalogue")
			return
		}
		if body.TraineeType != "" && !bpf.TypeStagiaire(body.TraineeType).Valid() {
			writeError(w, http.StatusBadRequest, "type de stagiaire inconnu du bilan")
			return
		}

		contact, err := deps.CRM.GetContact(r.Context(), session.OrgID, contactID)
		if err != nil {
			respondNotFound(w, err, "fiche introuvable")
			return
		}
		if contact.Kind != crm.KindLearner {
			writeError(w, http.StatusConflict,
				"seule une personne s'inscrit à une formation : une entreprise ou un financeur "+
					"se rattache au dossier")
			return
		}

		course, err := deps.Catalog.GetCourse(r.Context(), session.OrgID, body.CourseID)
		if err != nil {
			respondNotFound(w, err, "formation introuvable")
			return
		}
		if !course.Published {
			writeError(w, http.StatusConflict,
				"cette formation est en brouillon : publiez-la avant d'y inscrire quelqu'un")
			return
		}

		auteur, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}
		acteur := actorFrom(r, auteur.FullName())

		// 1. La session. Celle qu'on désigne, celle qui existe déjà, ou une
		// nouvelle. Rouvrir une session ouverte plutôt qu'en créer une seconde
		// évite de disperser une même promotion sur deux feuilles d'émargement.
		cible, err := resoudreSession(r, deps, session.OrgID, course, body.SessionID,
			body.StartsAt, body.EndsAt, body.Mode)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}

		// 2. Le dossier, sauf si on n'en veut pas. Un dossier déjà ouvert pour
		// ce stagiaire sur cette formation est repris : deux dossiers pour une
		// formation se disputeraient les mêmes pièces.
		var dossier crm.File
		if !body.SansDossier {
			dossier, err = resoudreDossier(r, deps, session.OrgID, contact, course, body.PriceHT,
				body.CompanyID, body.FunderID, body.Funding, acteur)
			if err != nil {
				deps.Log.Error("dossier", "err", err)
				writeError(w, http.StatusInternalServerError, "le dossier n'a pas pu être ouvert")
				return
			}
		}

		// 3. L'inscription elle-même. On vérifie d'abord qu'elle n'existe pas :
		// l'écriture conditionnelle la refuserait bien, mais avec un message de
		// stockage — « conflit d'écriture » — devant lequel on ne peut que
		// recommencer. Être déjà inscrit n'est pas une panne, c'est une redite.
		if deja, err := deps.Catalog.GetEnrollment(
			r.Context(), session.OrgID, cible.ID, contactID); err == nil && deja.ID != "" {
			writeJSON(w, http.StatusOK, map[string]any{
				"enrollment": deja,
				"session":    cible,
				"course":     map[string]any{"id": course.ID, "title": course.Title},
				"file":       map[string]any{"id": dossier.ID, "reference": dossier.Reference},
				"deja":       true,
			})
			return
		}

		debut, fin := jourOuRien(body.StartsAt), jourOuRien(body.EndsAt)
		inscription, err := deps.Catalog.Enroll(r.Context(), catalog.EnrollInput{
			OrgID: session.OrgID, SessionID: cible.ID, ContactID: contactID,
			FileID:         dossier.ID,
			TraineeType:    body.TraineeType,
			ContractStart:  debut,
			ContractEnd:    fin,
			HoursElearning: body.HoursElearning,
			HoursRemote:    body.HoursRemote,
			HoursOnSite:    body.HoursOnSite,
			Actor:          acteur,
		})
		if err != nil {
			deps.Log.Error("inscription", "err", err)
			writeError(w, http.StatusConflict, err.Error())
			return
		}

		reponse := map[string]any{
			"enrollment": inscription,
			"session":    cible,
			"course":     map[string]any{"id": course.ID, "title": course.Title},
		}
		if dossier.ID != "" {
			reponse["file"] = map[string]any{"id": dossier.ID, "reference": dossier.Reference}
		}
		writeJSON(w, http.StatusCreated, reponse)
	}
}

// resoudreSession trouve la session d'accueil, ou l'ouvre.
func resoudreSession(
	r *http.Request, deps Deps, orgID string, course catalog.Course,
	sessionID, debut, fin, mode string,
) (catalog.Session, error) {
	if sessionID != "" {
		cible, err := deps.Catalog.GetSession(r.Context(), orgID, sessionID)
		if err != nil {
			return catalog.Session{}, fmt.Errorf("session introuvable")
		}
		if cible.Closed {
			return catalog.Session{}, fmt.Errorf("cette session est clôturée")
		}
		return cible, nil
	}

	// Une session ouverte de la même formation ? On la reprend.
	if existantes, err := deps.Catalog.ListSessions(r.Context(), orgID, 200); err == nil {
		for _, candidate := range existantes {
			if candidate.CourseID == course.ID && !candidate.Closed {
				return candidate, nil
			}
		}
	}

	depart := time.Now()
	if parsed := jourOuRien(debut); parsed != nil {
		depart = *parsed
	}
	arrivee := depart.AddDate(0, 1, 0)
	if parsed := jourOuRien(fin); parsed != nil {
		arrivee = *parsed
	}

	nature := catalog.Mode(mode)
	if nature == "" {
		nature = catalog.ModeAsync
	}

	nouvelle := catalog.NewSession(orgID, course.ID, course.Title, nature, depart, arrivee, time.Now())
	return deps.Catalog.CreateSession(r.Context(), nouvelle)
}

// resoudreDossier reprend le dossier du stagiaire sur cette formation, ou l'ouvre.
func resoudreDossier(
	r *http.Request, deps Deps, orgID string, contact crm.Contact, course catalog.Course,
	prix float64, companyID, funderID, funding string, acteur audit.Actor,
) (crm.File, error) {
	for _, etape := range etapes {
		lot, err := deps.CRM.ListFilesByStage(r.Context(), orgID, etape, 200)
		if err != nil {
			continue
		}
		for _, file := range lot {
			if file.LearnerID == contact.ID && file.CourseID == course.ID {
				return file, nil
			}
		}
	}

	if prix == 0 {
		prix = course.PriceHT
	}
	dossier, err := deps.CRM.CreateFile(r.Context(), crm.CreateFileInput{
		OrgID:     orgID,
		Title:     course.Title + " — " + contact.DisplayName(),
		LearnerID: contact.ID, CompanyID: companyID, FunderID: funderID,
		CourseID: course.ID, PriceHT: prix,
		Actor: acteur,
	})
	if err != nil {
		return crm.File{}, err
	}

	// L'origine des fonds est posée à l'ouverture quand on la connaît : c'est
	// elle que le bilan annuel ventile, et la renseigner un an plus tard sur
	// deux cents dossiers ne se fait pas.
	if funding != "" {
		if maj, err := deps.CRM.SetFunding(r.Context(), orgID, dossier.ID, crm.FundingSource(funding)); err == nil {
			dossier = maj
		}
	}
	return dossier, nil
}
