package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/documents"
	"github.com/lemlearn/api/internal/invoicing"
)

// La facturation, vue de l'API.
//
// Un brouillon se modifie et se supprime ; une facture émise, ni l'un ni
// l'autre. Cette asymétrie n'est pas une commodité d'implémentation : c'est
// tout ce qui distingue une comptabilité d'un tableur, et les routes la font
// respecter plutôt que de compter sur l'écran.

func handleListFactures(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Invoicing == nil {
			writeError(w, http.StatusServiceUnavailable, "facturation indisponible")
			return
		}
		limite, curseur := pageParams(r)
		page, err := deps.Invoicing.List(r.Context(), session.OrgID, limite, curseur)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"factures": list(page.Items), "cursor": page.Cursor,
		})
	}
}

func handleGetFacture(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		facture, err := deps.Invoicing.Get(r.Context(), session.OrgID, chi.URLParam(r, "factureID"))
		if err != nil {
			respondNotFound(w, err, "facture introuvable")
			return
		}
		writeJSON(w, http.StatusOK, facture)
	}
}

// requeteFacture porte ce qu'un brouillon accepte.
type requeteFacture struct {
	ClientID     string            `json:"clientId"`
	FileID       string            `json:"fileId"`
	Lines        []invoicing.Ligne `json:"lines"`
	PaymentTerms string            `json:"paymentTerms"`
	Notes        string            `json:"notes"`
	DueOn        string            `json:"dueOn"`
}

func handleCreateFacture(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Invoicing == nil {
			writeError(w, http.StatusServiceUnavailable, "facturation indisponible")
			return
		}
		var body requeteFacture
		if !decodeJSON(w, r, &body) {
			return
		}
		facture, err := deps.Invoicing.Create(r.Context(), invoicing.CreateInput{
			OrgID: session.OrgID, ClientID: body.ClientID, FileID: body.FileID,
			Lines: body.Lines, PaymentTerms: body.PaymentTerms,
			Notes: body.Notes, DueOn: body.DueOn,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, facture)
	}
}

func handleUpdateFacture(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		var body requeteFacture
		if !decodeJSON(w, r, &body) {
			return
		}
		facture, err := deps.Invoicing.Update(r.Context(), session.OrgID,
			chi.URLParam(r, "factureID"), func(f *invoicing.Facture) {
				if body.ClientID != "" {
					f.ClientID = body.ClientID
				}
				f.FileID = body.FileID
				if body.Lines != nil {
					f.Lines = body.Lines
				}
				f.PaymentTerms, f.Notes, f.DueOn = body.PaymentTerms, body.Notes, body.DueOn
			})
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, facture)
	}
}

func handleIssueFacture(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		auteur, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}
		facture, err := deps.Invoicing.Issue(r.Context(), session.OrgID,
			chi.URLParam(r, "factureID"), actorFrom(r, auteur.FullName()))
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, facture)
	}
}

func handlePayFacture(deps Deps) http.HandlerFunc {
	type request struct {
		Paid bool   `json:"paid"`
		Way  string `json:"way"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		facture, err := deps.Invoicing.MarkPaid(r.Context(), session.OrgID,
			chi.URLParam(r, "factureID"), body.Way, body.Paid)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, facture)
	}
}

func handleCreditNote(deps Deps) http.HandlerFunc {
	type request struct {
		Motif string `json:"motif"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}
		avoir, err := deps.Invoicing.CreditNote(r.Context(), session.OrgID,
			chi.URLParam(r, "factureID"), body.Motif)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, avoir)
	}
}

func handleDeleteFacture(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if err := deps.Invoicing.Delete(r.Context(), session.OrgID,
			chi.URLParam(r, "factureID")); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleFacturePDF compose la facture.
//
// Un brouillon se compose aussi : c'est le seul moyen de relire une facture
// avant de l'émettre, et l'émission étant irréversible, s'en priver reviendrait
// à demander de signer sans lire. La pièce porte alors la mention « brouillon »
// au lieu d'un numéro.
func handleFacturePDF(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Compiler == nil {
			writeError(w, http.StatusServiceUnavailable, "composition indisponible")
			return
		}

		facture, err := deps.Invoicing.Get(r.Context(), session.OrgID, chi.URLParam(r, "factureID"))
		if err != nil {
			respondNotFound(w, err, "facture introuvable")
			return
		}
		org, err := deps.Identity.LoadOrg(r.Context(), session.OrgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "organisation introuvable")
			return
		}

		numero := facture.Number
		if numero == "" {
			numero = "brouillon"
		}
		emise := time.Now()
		if facture.IssuedOn != "" {
			if parsed, err := time.Parse("2006-01-02", facture.IssuedOn); err == nil {
				emise = parsed
			}
		}

		lignes := make([]documents.LigneFacture, 0, len(facture.Lines))
		for _, ligne := range facture.Lines {
			lignes = append(lignes, documents.LigneFacture{
				Label: ligne.Label, Quantity: ligne.Quantity,
				UnitPriceHT: ligne.UnitPriceHT, VATRate: ligne.VATRate,
			})
		}

		modele := documents.Facture{
			Number: numero, IssuedOn: emise, DueOn: facture.DueOn,
			Org: documents.PartyFromOrg(org),
			Client: documents.Party{
				Name: facture.Client.Name, SIRET: facture.Client.SIRET,
				Address: facture.Client.Address, PostalCode: facture.Client.PostalCode,
				City: facture.Client.City,
			},
			FileReference: facture.FileReference, Lines: lignes,
			VATExempt: facture.VATExempt, TotalHT: facture.TotalHT,
			TotalVAT: facture.TotalVAT, TotalTTC: facture.TotalTTC,
			PaymentTerms: facture.PaymentTerms, Notes: facture.Notes,
			CreditNoteFor: facture.CreditNoteFor,
		}
		// L'en-tête porte le logo de l'organisme, comme toute pièce qui sort
		// de chez lui. L'échec est silencieux : la raison sociale suffit.
		if logo := logoOf(r, deps, session.OrgID); len(logo) > 0 {
			for nom := range logo {
				modele.LogoAsset = nom
			}
			modele.LogoBytes = logo
		}

		rendu, err := deps.Compiler.Compile(r.Context(), documents.RenderFacture(modele))
		if err != nil {
			deps.Log.Error("composition de la facture", "err", err)
			writeError(w, http.StatusInternalServerError, "la facture n'a pas pu être composée")
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `inline; filename="`+numero+`.pdf"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(rendu)
	}
}
