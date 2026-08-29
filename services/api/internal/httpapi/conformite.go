package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/bpf"
	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/documents"
	"github.com/lemlearn/api/internal/identity"
)

// handleBPF assemble le bilan pédagogique et financier d'un exercice.
//
// Il ne produit pas le Cerfa : il produit les nombres à y reporter. L'organisme
// saisit sur le portail Mon Activité Formation, et ce qui lui manque au moment
// de le faire, ce sont les totaux — pas un formulaire de plus.
func handleBPF(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		annee := time.Now().UTC().Year() - 1
		if demande := r.URL.Query().Get("annee"); demande != "" {
			valeur, err := strconv.Atoi(demande)
			if err != nil || valeur < 2000 || valeur > 2100 {
				writeError(w, http.StatusBadRequest, "année invalide")
				return
			}
			annee = valeur
		}

		bilan, err := bpf.Compute(r.Context(), bpf.Deps{CRM: deps.CRM, Catalog: deps.Catalog},
			session.OrgID, annee)
		if err != nil {
			deps.Log.Error("bilan pédagogique et financier", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"bilan":    bilan,
			"echeance": bpf.Echeance(annee).Format("2006-01-02"),
		})
	}
}

// handleReglement rend le règlement intérieur de l'organisme.
//
// Il est obligatoire (art. L.6352-3) et se remet au stagiaire avant son
// inscription définitive. Son contenu étant dicté par le code du travail, il
// est produit à la demande depuis l'identité de l'organisme plutôt que rédigé
// et téléversé : un règlement rédigé à la main oublie la moitié des garanties
// de procédure, et c'est exactement ce qu'un contrôle relève.
func handleReglement(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Compiler == nil {
			writeError(w, http.StatusServiceUnavailable, "compilateur indisponible")
			return
		}

		org, err := deps.Identity.LoadOrg(r.Context(), session.OrgID)
		if err != nil {
			writeError(w, http.StatusNotFound, "organisation introuvable")
			return
		}

		// La section sur la représentation des stagiaires ne s'impose qu'aux
		// formations de plus de cinq cents heures : on regarde le catalogue
		// plutôt que de le demander, et on ne promet pas d'élections qui
		// n'auront pas lieu.
		longues := false
		if deps.Catalog != nil {
			if courses, err := deps.Catalog.ListCourses(r.Context(), session.OrgID, 0); err == nil {
				for _, course := range courses {
					if course.DurationHours > 500 {
						longues = true
					}
				}
			}
		}

		pdf, err := deps.Compiler.Compile(r.Context(), documents.RenderReglement(documents.Reglement{
			Org:         orgParty(org),
			IssuedOn:    deps.Now(),
			LongCourses: longues,
		}))
		if err != nil {
			deps.Log.Error("règlement intérieur", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `inline; filename="reglement-interieur.pdf"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pdf)
	}
}

// orgParty projette l'organisation dans la partie contractante des gabarits.
//
// La conversion est faite une fois ici plutôt qu'à chaque appelant : c'est
// ainsi qu'un champ ajouté à l'identité juridique atteint tous les documents,
// et pas seulement celui auquel on pensait ce jour-là.
func orgParty(org identity.Org) documents.Party {
	return documents.Party{
		Name: org.Name, Address: org.Address,
		PostalCode: org.PostalCode, City: org.City, SIRET: org.SIRET,
		LegalForm: org.LegalForm, Capital: org.Capital, RCS: org.RCS,
		VATNumber: org.VATNumber, VATExempt: org.VATExempt,
		NDA: org.NDA, NDARegion: org.NDARegion,
		Represented: org.RepName, Role: org.RepRole,
	}
}

// handleSetFunding renseigne l'origine des fonds d'un dossier.
//
// C'est le seul renseignement du bilan annuel qu'on ne peut pas reconstituer
// après coup : douze mois plus tard, personne ne se souvient si telle formation
// a été prise en charge par l'OPCO ou payée par l'entreprise. On le demande
// donc au moment où on le sait.
func handleSetFunding(deps Deps) http.HandlerFunc {
	type request struct {
		Funding crm.FundingSource `json:"funding"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		if _, connue := bpf.Libelles[body.Funding]; !connue && body.Funding != "" {
			writeError(w, http.StatusBadRequest, "origine de fonds inconnue")
			return
		}

		file, err := deps.CRM.SetFunding(r.Context(), session.OrgID,
			chi.URLParam(r, "fileID"), body.Funding)
		if err != nil {
			respondNotFound(w, err, "dossier introuvable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"file": file})
	}
}
