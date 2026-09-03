// Package bpf assemble le bilan pédagogique et financier.
//
// C'est une déclaration annuelle que tout organisme de formation doit déposer,
// et dont le défaut suspend l'exonération de TVA (art. R.6352-22 et suivants).
// L'échéance réglementaire est le 30 avril ; le ministère accorde chaque année
// un report au 31 mai.
//
// Le produit détient déjà tout ce qu'elle réclame : combien de stagiaires,
// combien d'heures, et d'où venaient les fonds. Le seul travail est de
// l'agréger selon les catégories du formulaire — pas selon les nôtres, sans
// quoi il faudrait retraduire chaque ligne au moment de la déclaration.
//
// Ce paquet ne produit pas le Cerfa : il produit les nombres à y reporter.
// Générer le formulaire lui-même supposerait de suivre ses révisions annuelles
// pour un gain nul — l'organisme saisit sur le portail Mon Activité Formation,
// et ce qui lui manque, ce sont les totaux.
package bpf

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/crm"
)

// Ligne est une origine de fonds et ce qu'elle a produit.
type Ligne struct {
	Source   crm.FundingSource `json:"source"`
	Label    string            `json:"label"`
	Dossiers int               `json:"dossiers"`
	// MontantHT est en euros. Le bilan se remplit en comptabilité
	// d'engagement — ce qui a été facturé sur l'exercice — et non sur les
	// encaissements ; c'est le montant du dossier qui compte, pas son
	// règlement.
	MontantHT float64 `json:"montantHT"`
}

// Bilan est le récapitulatif d'un exercice.
type Bilan struct {
	Annee int `json:"annee"`

	// Produits, ventilés selon les catégories du cadre C.
	Produits []Ligne `json:"produits"`
	TotalHT  float64 `json:"totalHT"`
	Dossiers int     `json:"dossiers"`

	// Stagiaires et heures-stagiaires : les deux unités du cadre F. Une heure
	// -stagiaire est une heure de formation suivie par une personne ; c'est le
	// produit de l'effectif par la durée, et c'est cette grandeur que le
	// formulaire demande, pas le nombre d'heures du catalogue.
	Stagiaires      int     `json:"stagiaires"`
	HeuresStagiaire float64 `json:"heuresStagiaire"`
	Sessions        int     `json:"sessions"`

	// Manquants nomme les dossiers dont l'origine des fonds n'est pas
	// renseignée. Les taire produirait un bilan faux d'apparence complète ;
	// les nommer permet de les corriger avant de déclarer.
	SansOrigine []string `json:"sansOrigine,omitempty"`

	// ParTypeStagiaire et ParObjectif sont les cadres E et F : qui a été
	// formé, et à quoi servait la formation. Ils manquaient entièrement — le
	// bilan s'arrêtait à l'argent, c'est-à-dire à la moitié du formulaire.
	ParTypeStagiaire []LigneVentilation `json:"parTypeStagiaire,omitempty"`
	ParObjectif      []LigneVentilation `json:"parObjectif,omitempty"`

	// SansTypeStagiaire et SansObjectif nomment ce qui n'a pas été classé.
	// Une ventilation qui range les inconnus dans « autres » ferait déposer un
	// chiffre faux sans que personne ne s'en aperçoive.
	SansTypeStagiaire int      `json:"sansTypeStagiaire,omitempty"`
	SansObjectif      []string `json:"sansObjectif,omitempty"`
}

// LigneVentilation est une ligne des cadres E ou F : un intitulé, un nombre de
// stagiaires, et les heures-stagiaires correspondantes.
type LigneVentilation struct {
	Code            string  `json:"code"`
	Label           string  `json:"label"`
	Stagiaires      int     `json:"stagiaires"`
	HeuresStagiaire float64 `json:"heuresStagiaire"`
}

// Libelles donne le nom de chaque origine, tel qu'il figure au formulaire.
var Libelles = map[crm.FundingSource]string{
	crm.FundingCompany:     "Entreprises (hors OPCO)",
	crm.FundingOPCO:        "Opérateurs de compétences",
	crm.FundingPublic:      "Fonds publics (État, Régions, France Travail)",
	crm.FundingIndividual:  "Contrats conclus avec des particuliers",
	crm.FundingSubcontract: "Autres organismes de formation (sous-traitance)",
	crm.FundingOther:       "Autres produits",
}

// ordre fixe la présentation, celle du formulaire. Un ordre alphabétique
// obligerait à chercher chaque ligne au moment de recopier.
var ordre = []crm.FundingSource{
	crm.FundingCompany, crm.FundingOPCO, crm.FundingPublic,
	crm.FundingIndividual, crm.FundingSubcontract, crm.FundingOther,
}

// Deps regroupe ce dont le calcul a besoin.
type Deps struct {
	CRM     *crm.Service
	Catalog *catalog.Service
}

// Compute assemble le bilan d'un exercice.
//
// L'exercice est l'année civile. Un organisme dont l'exercice comptable est
// décalé devra ajuster — le formulaire le permet, et le produit ne saurait pas
// deviner la date de clôture sans la demander.
func Compute(ctx context.Context, deps Deps, orgID string, annee int) (Bilan, error) {
	if deps.CRM == nil || deps.Catalog == nil {
		return Bilan{}, fmt.Errorf("bpf: services indisponibles")
	}

	debut := time.Date(annee, 1, 1, 0, 0, 0, 0, time.UTC)
	fin := debut.AddDate(1, 0, 0)

	bilan := Bilan{Annee: annee}
	montants := map[crm.FundingSource]float64{}
	compte := map[crm.FundingSource]int{}

	// Le pipeline sert de source : il balaie les cinq étapes, ce qui couvre
	// tous les dossiers de l'organisation sans scanner la table.
	pipeline, err := deps.CRM.Pipeline(ctx, orgID, 0)
	if err != nil {
		return Bilan{}, err
	}
	for _, colonne := range pipeline {
		for _, file := range colonne {
			// Un dossier compte pour l'exercice où il a été engagé. Un
			// prospect ne compte pas : rien n'a été conventionné, donc rien
			// n'a été produit.
			if file.CreatedAt.Before(debut) || !file.CreatedAt.Before(fin) {
				continue
			}
			if file.Stage == crm.StageProspect {
				continue
			}

			source := file.Funding
			if source == "" {
				bilan.SansOrigine = append(bilan.SansOrigine, file.Reference)
				source = crm.FundingOther
			}
			montants[source] += file.PriceHT
			compte[source]++
			bilan.TotalHT += file.PriceHT
			bilan.Dossiers++
		}
	}

	for _, source := range ordre {
		if compte[source] == 0 && montants[source] == 0 {
			continue
		}
		bilan.Produits = append(bilan.Produits, Ligne{
			Source: source, Label: Libelles[source],
			Dossiers: compte[source], MontantHT: montants[source],
		})
	}

	sessions, err := deps.Catalog.ListSessions(ctx, orgID, 0)
	if err != nil {
		return Bilan{}, err
	}
	parType := map[TypeStagiaire]*LigneVentilation{}
	parObjectif := map[Objectif]*LigneVentilation{}
	sansObjectif := map[string]bool{}

	for _, session := range sessions {
		if session.StartsAt.Before(debut) || !session.StartsAt.Before(fin) {
			continue
		}
		course, err := deps.Catalog.GetCourse(ctx, orgID, session.CourseID)
		if err != nil {
			continue
		}
		enrollments, err := deps.Catalog.ListSessionEnrollments(ctx, orgID, session.ID)
		if err != nil {
			continue
		}
		bilan.Sessions++
		bilan.Stagiaires += len(enrollments)
		bilan.HeuresStagiaire += float64(len(enrollments)) * course.DurationHours

		// L'objectif est porté par la formation : toutes ses inscriptions
		// tombent dans la même ligne du cadre F.
		objectif := Objectif(course.ObjectiveType)
		if !objectif.Valid() {
			if len(enrollments) > 0 {
				sansObjectif[course.Title] = true
			}
		} else {
			ligne := parObjectif[objectif]
			if ligne == nil {
				ligne = &LigneVentilation{Code: string(objectif), Label: LibellesObjectif[objectif]}
				parObjectif[objectif] = ligne
			}
			ligne.Stagiaires += len(enrollments)
			ligne.HeuresStagiaire += float64(len(enrollments)) * course.DurationHours
		}

		// Le type de stagiaire, lui, se compte inscription par inscription.
		for _, enrollment := range enrollments {
			nature := TypeStagiaire(enrollment.TraineeType)
			if !nature.Valid() {
				bilan.SansTypeStagiaire++
				continue
			}
			ligne := parType[nature]
			if ligne == nil {
				ligne = &LigneVentilation{Code: string(nature), Label: LibellesStagiaire[nature]}
				parType[nature] = ligne
			}
			ligne.Stagiaires++
			ligne.HeuresStagiaire += course.DurationHours
		}
	}

	for _, nature := range OrdreStagiaire {
		if ligne := parType[nature]; ligne != nil {
			bilan.ParTypeStagiaire = append(bilan.ParTypeStagiaire, *ligne)
		}
	}
	for _, objectif := range OrdreObjectif {
		if ligne := parObjectif[objectif]; ligne != nil {
			bilan.ParObjectif = append(bilan.ParObjectif, *ligne)
		}
	}
	for titre := range sansObjectif {
		bilan.SansObjectif = append(bilan.SansObjectif, titre)
	}
	sort.Strings(bilan.SansObjectif)

	sort.Strings(bilan.SansOrigine)
	return bilan, nil
}

// Echeance donne la date limite de dépôt pour un exercice.
//
// Le 30 avril est la date de l'article R.6352-23 ; le report au 31 mai est
// accordé chaque année par le ministère du travail. On annonce la date
// réglementaire, et on mentionne le report — l'inverse ferait manquer
// l'échéance l'année où il ne serait pas reconduit.
func Echeance(annee int) time.Time {
	return time.Date(annee+1, 4, 30, 0, 0, 0, 0, time.UTC)
}
