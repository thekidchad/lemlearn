package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/signature"
)

// Ce que l'apprenant peut savoir de lui-même.
//
// Son espace ne montrait que son parcours. C'est le plus visible, mais pas le
// plus rassurant : quand on suit une formation financée, ce qu'on cherche
// souvent, c'est de vérifier ce que l'organisme a écrit sur soi, retrouver la
// convention qu'on a signée, et savoir qui est l'organisme en question. Rien
// de tout cela n'était accessible sans écrire au secrétariat.
//
// Ces deux routes ne prennent aucun identifiant : la fiche vient du compte.
// Une adresse qui porterait le numéro de la fiche inviterait à en essayer une
// autre.

// handleLearnerMe rend la fiche de l'apprenant, son organisme et ses pièces.
func handleLearnerMe(deps Deps) http.HandlerFunc {
	type piece struct {
		ID        string `json:"id"`
		Kind      string `json:"kind"`
		Reference string `json:"reference"`
		Status    string `json:"status"`
		SignedAt  string `json:"signedAt,omitempty"`
		// Empreinte du document scellé : c'est elle qu'un contrôleur
		// recalcule. L'afficher n'est pas de la décoration — c'est ce qui rend
		// la copie de l'apprenant vérifiable.
		SHA256       string `json:"sha256,omitempty"`
		Downloadable bool   `json:"downloadable"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}
		if user.ContactID == "" {
			writeError(w, http.StatusForbidden, "ce compte n'est rattaché à aucune fiche apprenant")
			return
		}

		contact, err := deps.CRM.GetContact(r.Context(), session.OrgID, user.ContactID)
		if err != nil {
			respondNotFound(w, err, "fiche introuvable")
			return
		}
		org, err := deps.Identity.LoadOrg(r.Context(), session.OrgID)
		if err != nil {
			respondNotFound(w, err, "organisme introuvable")
			return
		}

		response := map[string]any{
			"contact": contact,
			// L'identité complète de l'organisme, mentions légales comprises.
			// Un stagiaire a le droit de savoir chez qui il se forme, et c'est
			// la même information qui figure sur sa convention.
			"organisme": org.Public(),
		}

		// Les pièces : celles des dossiers où l'apprenant figure. On ne
		// remonte que les documents qui le concernent — un dossier peut porter
		// la signature de l'entreprise, elle ne le regarde pas.
		pieces := make([]piece, 0, 4)
		for _, file := range learnerFiles(r, deps, session.OrgID, user.ContactID) {
			if deps.Signature == nil {
				break
			}
			requests, err := deps.Signature.ListForFile(r.Context(), session.OrgID, file.ID)
			if err != nil {
				continue
			}
			for _, request := range requests {
				if !concerne(request, contact) {
					continue
				}
				ligne := piece{
					ID: request.ID, Kind: request.Kind, Reference: request.Reference,
					Status: string(request.Status),
				}
				if request.Proof != nil {
					ligne.SignedAt = request.Proof.SignedAt.Format("2006-01-02")
					ligne.SHA256 = request.Proof.SealedSHA256
					ligne.Downloadable = true
				}
				pieces = append(pieces, ligne)
			}
		}
		response["pieces"] = pieces

		writeJSON(w, http.StatusOK, response)
	}
}

// handleLearnerDocument rend à l'apprenant une pièce qu'il a signée.
//
// Le document est relu depuis l'archive et son empreinte recalculée avant
// d'être servi : ce que l'apprenant télécharge est exactement ce qui a été
// scellé, ou rien. Servir un fichier altéré en silence vaudrait moins que ne
// rien servir du tout.
func handleLearnerDocument(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		if deps.Signature == nil {
			writeError(w, http.StatusServiceUnavailable, "archivage indisponible")
			return
		}
		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil || user.ContactID == "" {
			writeError(w, http.StatusForbidden, "aucune fiche rattachée à ce compte")
			return
		}
		contact, err := deps.CRM.GetContact(r.Context(), session.OrgID, user.ContactID)
		if err != nil {
			respondNotFound(w, err, "fiche introuvable")
			return
		}

		// La pièce est retrouvée à travers les dossiers de l'apprenant, et non
		// par lecture directe de son identifiant : c'est l'appartenance qui
		// autorise, pas la connaissance du numéro.
		wanted := chi.URLParam(r, "requestID")
		for _, file := range learnerFiles(r, deps, session.OrgID, user.ContactID) {
			requests, err := deps.Signature.ListForFile(r.Context(), session.OrgID, file.ID)
			if err != nil {
				continue
			}
			for _, request := range requests {
				if request.ID != wanted || !concerne(request, contact) {
					continue
				}
				sealed, err := deps.Signature.Sealed(r.Context(), request)
				if err != nil {
					writeError(w, http.StatusConflict, err.Error())
					return
				}
				w.Header().Set("Content-Type", "application/pdf")
				w.Header().Set("Content-Disposition",
					fmt.Sprintf(`attachment; filename="%s.pdf"`, nomDeFichier(request)))
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(sealed)
				return
			}
		}

		writeError(w, http.StatusNotFound, "pièce introuvable")
	}
}

// learnerFiles rassemble les dossiers où l'apprenant figure.
func learnerFiles(r *http.Request, deps Deps, orgID, contactID string) []crm.File {
	files := make([]crm.File, 0, 4)
	for _, etape := range etapes {
		lot, err := deps.CRM.ListFilesByStage(r.Context(), orgID, etape, 200)
		if err != nil {
			continue
		}
		for _, file := range lot {
			if file.LearnerID == contactID {
				files = append(files, file)
			}
		}
	}
	return files
}

// concerne dit si une demande de signature est celle de l'apprenant.
//
// L'adresse plutôt que l'identifiant : le signataire est désigné par son
// adresse au moment de l'émission, et c'est cette adresse qui a reçu le lien.
func concerne(request signature.Request, contact crm.Contact) bool {
	if contact.Email == "" {
		return false
	}
	return strings.EqualFold(request.SignerEmail, contact.Email)
}

// nomDeFichier compose le nom du PDF enregistré.
//
// Une pièce ancienne peut n'avoir aucune référence : le champ a été ajouté
// après coup. Sans ce repli, le navigateur enregistrait un fichier nommé
// « .pdf », que le système range parmi les fichiers cachés.
func nomDeFichier(request signature.Request) string {
	if request.Reference != "" {
		return request.Reference
	}
	nom := request.Kind
	if nom == "" {
		nom = "document"
	}
	if request.Proof != nil {
		return nom + "-" + request.Proof.SignedAt.Format("2006-01-02")
	}
	return nom
}
