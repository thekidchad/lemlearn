package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/config"
	"github.com/lemlearn/api/internal/documents"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/doc"
)

// handleDocumentPreview compile un gabarit avec des données de démonstration et
// renvoie le PDF en ligne. Réservé au développement : les gabarits évoluent en
// permanence et une boucle de rendu rapide vaut mieux qu'un aller-retour par
// l'interface.
//
// `?zones=1` renvoie les zones de signature extraites plutôt que le PDF, ce qui
// permet de vérifier leur position sans ouvrir le document.
// logoOf relit le logo d'un organisme pour l'incorporer à un aperçu. L'échec
// est silencieux : l'en-tête retombe sur la raison sociale.
func logoOf(r *http.Request, deps Deps, orgID string) map[string][]byte {
	if deps.Assets == nil || deps.Brand == nil {
		return nil
	}
	marque, err := deps.Brand.Get(r.Context(), orgID)
	if err != nil || marque.LogoKey == "" {
		return nil
	}
	octets, err := deps.Assets.Get(r.Context(), marque.LogoKey)
	if err != nil || len(octets) == 0 {
		return nil
	}
	return map[string][]byte{documents.LogoAsset(marque.LogoKey): octets}
}

func handleDocumentPreview(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Config.Env == config.EnvProd {
			writeError(w, http.StatusNotFound, "indisponible")
			return
		}
		if deps.Compiler == nil {
			writeError(w, http.StatusServiceUnavailable, "compilateur typst indisponible")
			return
		}

		template := chi.URLParam(r, "template")
		document, err := demoDocument(template)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		// L'aperçu porte l'identité réelle de l'organisme connecté : un
		// exemplaire dont l'en-tête ne ressemble pas à ce qu'on enverra ne
		// sert qu'à valider une mise en page, pas à se relire.
		session, _ := sessionFrom(r)
		if deps.Identity != nil && session.OrgID != "" {
			if org, err := deps.Identity.LoadOrg(r.Context(), session.OrgID); err == nil {
				document, err = demoDocumentFor(template, org, logoOf(r, deps, session.OrgID))
				if err != nil {
					writeError(w, http.StatusNotFound, err.Error())
					return
				}
			}
		}

		pdf, zones, err := doc.CompileWithZones(r.Context(), deps.Compiler, document)
		if err != nil {
			deps.Log.Error("prévisualisation", "template", template, "err", err)
			writeError(w, http.StatusInternalServerError, "compilation impossible")
			return
		}

		if r.URL.Query().Get("zones") != "" {
			writeJSON(w, http.StatusOK, map[string]any{"template": template, "zones": zones})
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Length", strconv.Itoa(len(pdf)))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s.pdf"`, template))
		_, _ = w.Write(pdf)
	}
}

// demoDocument renvoie le jeu de démonstration d'un gabarit.
func demoDocument(template string) (doc.Document, error) {
	switch template {
	case "convention":
		return documents.RenderConvention(demoConvention()), nil
	default:
		return doc.Document{}, fmt.Errorf("gabarit %q inconnu", template)
	}
}

// demoDocumentFor rend le même exemplaire, sous l'identité réelle de
// l'organisme : même contenu fictif, mais l'en-tête, le logo et la mention
// légale sont ceux qui partiront.
func demoDocumentFor(template string, org identity.Org, logo map[string][]byte) (doc.Document, error) {
	switch template {
	case "convention":
		convention := demoConvention()
		convention.Org = documents.PartyFromOrg(org)
		for nom := range logo {
			convention.LogoAsset = nom
		}
		convention.LogoBytes = logo
		return documents.RenderConvention(convention), nil
	default:
		return doc.Document{}, fmt.Errorf("gabarit %q inconnu", template)
	}
}

// at construit un créneau à l'heure de Paris — les horaires imprimés sur une
// convention sont ceux du stagiaire, pas ceux du serveur.
func at(year int, month time.Month, day, hour int) time.Time {
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		loc = time.UTC
	}
	return time.Date(year, month, day, hour, 0, 0, 0, loc)
}

func demoConvention() documents.Convention {
	issued := time.Date(2026, 1, 14, 9, 0, 0, 0, time.UTC)
	return documents.Convention{
		Reference: "CONV-2026-0143",
		IssuedOn:  issued,
		// L'identité vient de l'organisation connectée : l'aperçu doit montrer
		// ce que le client enverra, pas un exemplaire de démonstration dont
		// l'en-tête ne lui ressemble pas. Seul le contenu reste fictif.
		Org: documents.Party{
			Name: "Institut Vulcain", LegalForm: "SAS",
			Address: "12 rue des Écoles", PostalCode: "75005", City: "Paris",
			SIRET: "84291736500018", Represented: "Marie Dubreuil", Role: "présidente",
			Capital: "10 000 €", RCS: "Paris B 842 917 365", VATNumber: "FR12842917365",
			NDA: "11756789012", NDARegion: "Île-de-France",
		},
		Client: documents.Party{
			Name: "Groupe Aramis", LegalForm: "SARL",
			Address: "8 avenue Foch", PostalCode: "69006", City: "Lyon",
			SIRET: "51203847600024", Represented: "Léa Bertrand", Role: "directrice des ressources humaines",
		},
		CourseTitle: "Sécurité incendie — SSIAP 1",
		CourseGoal:  "Former les agents à la prévention et à l'intervention de premier niveau sur un système de sécurité incendie.",
		Objectives: []string{
			"Identifier les composants d'un système de sécurité incendie",
			"Appliquer la conduite à tenir à la réception d'une alarme",
			"Rédiger un rapport d'intervention exploitable",
		},
		Prerequisites:   "Aucun prérequis. Maîtrise du français lu et écrit.",
		Audience:        "Agents de sécurité et personnel technique",
		DurationHours:   14,
		Modalities:      "Distanciel asynchrone (modules vidéo) et classe virtuelle",
		Means:           "Plateforme de formation en ligne, supports téléchargeables, questionnaire après chaque module, assistance par courriel sous 24 h ouvrées",
		Assessment:      "Évaluation de positionnement à l'entrée, questionnaire après chaque module, évaluation finale notée sur 20. Seuil de validation : 14/20.",
		Sanction:        "Attestation de fin de formation mentionnant les objectifs atteints",
		AccessibilityPS: "Référent handicap joignable à accessibilite@institut-vulcain.fr ; adaptation des supports et du rythme sur demande.",
		Learners: []documents.LearnerLine{
			{FullName: "Léa Bertrand", Position: "Agent de sécurité"},
			{FullName: "Karim Nasri", Position: "Technicien de maintenance"},
			{FullName: "Sophie Meunier", Position: "Assistante d'exploitation"},
		},
		Sessions: []documents.SessionLine{
			{Start: at(2026, 2, 3, 9), End: at(2026, 2, 3, 12), Mode: "Classe virtuelle", Location: "Lien transmis par courriel"},
			{Start: at(2026, 2, 10, 9), End: at(2026, 2, 10, 12), Mode: "Classe virtuelle", Location: "Lien transmis par courriel"},
		},
		PriceHT:      1250,
		VATRate:      20,
		PaymentTerms: "Subrogation de paiement OPCO EP. Solde à 30 jours à réception de facture.",
		FunderName:   "OPCO EP — dossier n° 2026-00841",
		SignedCity:   "Paris",
	}
}
