package httpapi

import (
	"net/http"
	"strings"

	"github.com/lemlearn/api/internal/identity"
)

// Identité juridique de l'organisme.
//
// Ce n'est pas de l'administratif décoratif : c'est ce qui rend une convention
// opposable. L'article R.6351-6 du code du travail impose la mention de
// déclaration d'activité, numéro compris ; une facture d'organisme exonéré doit
// porter l'article du CGI qui le dispense ; et un document signé par personne
// de nommé se conteste.
//
// Les champs sont saisis une fois et repris sur chaque pièce, plutôt que
// ressaisis à chaque convention — c'est aussi ce qui garantit qu'ils y sont
// tous, et identiques.
func handleGetOrgLegal(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		org, err := deps.Identity.LoadOrg(r.Context(), session.OrgID)
		if err != nil {
			writeError(w, http.StatusNotFound, "organisation introuvable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"org": org.Public()})
	}
}

func handleSaveOrgLegal(deps Deps) http.HandlerFunc {
	type request struct {
		Name       string `json:"name"`
		LegalForm  string `json:"legalForm"`
		Capital    string `json:"capital"`
		RCS        string `json:"rcs"`
		SIRET      string `json:"siret"`
		VATNumber  string `json:"vatNumber"`
		VATExempt  bool   `json:"vatExempt"`
		NDA        string `json:"nda"`
		NDARegion  string `json:"ndaRegion"`
		RepName    string `json:"repName"`
		RepRole    string `json:"repRole"`
		Address    string `json:"address"`
		PostalCode string `json:"postalCode"`
		City       string `json:"city"`

		QualiopiCertified bool   `json:"qualiopiCertified"`
		QualiopiNumber    string `json:"qualiopiNumber"`
		QualiopiBody      string `json:"qualiopiBody"`
		QualiopiExpiresOn string `json:"qualiopiExpiresOn"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		// Le SIRET fait quatorze chiffres, le SIREN les neuf premiers. On
		// refuse une saisie approximative plutôt que de l'imprimer sur une
		// convention : un numéro faux se remarque au contrôle, pas avant.
		siret := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, body.SIRET)
		if siret != "" && len(siret) != 14 {
			writeError(w, http.StatusBadRequest, "le SIRET compte quatorze chiffres")
			return
		}

		org, err := deps.Identity.UpdateOrg(r.Context(), session.OrgID, func(org *identity.Org) {
			if name := strings.TrimSpace(body.Name); name != "" {
				org.Name = name
			}
			org.LegalForm = strings.TrimSpace(body.LegalForm)
			org.Capital = strings.TrimSpace(body.Capital)
			org.RCS = strings.TrimSpace(body.RCS)
			org.SIRET = siret
			org.VATNumber = strings.TrimSpace(body.VATNumber)
			org.VATExempt = body.VATExempt
			org.NDA = strings.TrimSpace(body.NDA)
			org.NDARegion = strings.TrimSpace(body.NDARegion)
			org.RepName = strings.TrimSpace(body.RepName)
			org.RepRole = strings.TrimSpace(body.RepRole)
			org.Address = strings.TrimSpace(body.Address)
			org.PostalCode = strings.TrimSpace(body.PostalCode)
			org.City = strings.TrimSpace(body.City)
			org.QualiopiCertified = body.QualiopiCertified
			org.QualiopiNumber = strings.TrimSpace(body.QualiopiNumber)
			org.QualiopiBody = strings.TrimSpace(body.QualiopiBody)
			org.QualiopiExpiresOn = strings.TrimSpace(body.QualiopiExpiresOn)
		})
		if err != nil {
			deps.Log.Error("identité juridique", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"org": org.Public()})
	}
}
