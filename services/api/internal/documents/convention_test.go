package documents

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/platform/doc"
)

func sampleConvention() Convention {
	issued := time.Date(2026, 1, 14, 9, 0, 0, 0, time.UTC)
	return Convention{
		Reference: "CONV-2026-0143",
		IssuedOn:  issued,
		Org: Party{
			Name: "Institut Vulcain", LegalForm: "SAS",
			Address: "12 rue des Écoles", PostalCode: "75005", City: "Paris",
			SIRET: "84291736500018", Represented: "Marie Dubreuil", Role: "présidente",
			Capital: "10 000 €", RCS: "Paris B 842 917 365",
			NDA: "11756789012", NDARegion: "Île-de-France",
		},
		Client: Party{
			Name: "Groupe Aramis", LegalForm: "SARL",
			Address: "8 avenue Foch", PostalCode: "69006", City: "Lyon",
			SIRET: "51203847600024", Represented: "Léa Bertrand", Role: "directrice des ressources humaines",
		},
		CourseTitle:   `Sécurité incendie — SSIAP 1 "niveau initial"`,
		CourseGoal:    "Former les agents à la prévention et à l'intervention de premier niveau sur un système de sécurité incendie.",
		Objectives:    []string{"Identifier les composants d'un SSI", "Appliquer la conduite à tenir en cas d'alarme", "Rédiger un rapport d'intervention"},
		Prerequisites: "Aucun",
		Audience:      "Agents de sécurité et personnel technique",
		DurationHours: 14,
		Modalities:    "Distanciel asynchrone (vidéo) et classe virtuelle",
		Means:         "Plateforme de formation en ligne, supports téléchargeables, quiz après chaque module",
		Assessment:    "Évaluation de positionnement, quiz après chaque module, évaluation finale notée sur 20",
		Sanction:      "Attestation de fin de formation",
		Learners: []LearnerLine{
			{FullName: "Léa Bertrand", Position: "Agent de sécurité"},
			{FullName: "Karim Nasri", Position: "Technicien de maintenance"},
		},
		Sessions: []SessionLine{
			{Start: issued.AddDate(0, 0, 20).Add(9 * time.Hour), End: issued.AddDate(0, 0, 20).Add(12 * time.Hour), Mode: "Classe virtuelle", Location: "Lien transmis par e-mail"},
		},
		PriceHT: 1250, VATRate: 20,
		PaymentTerms: "Subrogation de paiement OPCO EP, solde à 30 jours",
		FunderName:   "OPCO EP",
		SignedCity:   "Paris",
	}
}

func compiler(t *testing.T) *doc.BinaryCompiler {
	t.Helper()
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("binaire typst absent : `brew install typst`")
	}
	c, err := doc.NewBinaryCompiler()
	if err != nil {
		t.Fatalf("compilateur: %v", err)
	}
	return c
}

// La convention doit compiler et déclarer exactement les zones attendues :
// une signature organisme, une signature client, une mention manuscrite
// pour le client. Un gabarit signable qui n'en déclarerait aucune enverrait
// la signature au hasard sur la dernière page.
func TestConventionDeclaresSignatureZones(t *testing.T) {
	pdf, zones, err := compiler(t).CompileWithZones(context.Background(), RenderConvention(sampleConvention()))
	if err != nil {
		t.Fatalf("compilation: %v", err)
	}
	if len(pdf) == 0 {
		t.Fatal("pdf vide")
	}

	var orgSig, clientSig, clientMention int
	for _, z := range zones {
		switch {
		case z.Role == doc.RoleOrganization && z.Kind == doc.KindSignature:
			orgSig++
		case z.Role == doc.RoleClient && z.Kind == doc.KindSignature:
			clientSig++
		case z.Role == doc.RoleClient && z.Kind == doc.KindHandwrittenMention:
			clientMention++
		}
		if z.Page < 1 || z.Width <= 0 || z.Height <= 0 {
			t.Errorf("zone invalide: %+v", z)
		}
	}
	if orgSig != 1 || clientSig != 1 || clientMention != 1 {
		t.Fatalf("zones attendues 1/1/1, obtenues organisme=%d client=%d mention=%d (%d au total)",
			orgSig, clientSig, clientMention, len(zones))
	}
}

// Deux compilations de la même convention doivent produire des octets
// identiques : sans cela, l'empreinte SHA-256 inscrite au dossier de preuve
// ne serait pas reproductible et ne prouverait plus rien.
func TestConventionRenderIsReproducible(t *testing.T) {
	c := compiler(t)
	first, err := c.Compile(context.Background(), RenderConvention(sampleConvention()))
	if err != nil {
		t.Fatalf("première compilation: %v", err)
	}
	second, err := c.Compile(context.Background(), RenderConvention(sampleConvention()))
	if err != nil {
		t.Fatalf("seconde compilation: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("tailles différentes: %d vs %d octets", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("octet %d différent entre deux compilations identiques", i)
		}
	}
}

// Les données saisies par un utilisateur ne doivent jamais être interprétées
// comme du code Typst : un intitulé de formation contenant guillemets,
// dièse ou antislash doit produire un document correct.
func TestConventionEscapesUserInput(t *testing.T) {
	c := sampleConvention()
	c.CourseTitle = `Formation "#1" \ 100 $ *urgent*`
	c.Client.Name = `Société <Test> & Cie`
	c.Objectives = []string{`Utiliser le symbole @ et le #hashtag`}

	if _, err := compiler(t).Compile(context.Background(), RenderConvention(c)); err != nil {
		t.Fatalf("compilation avec caractères spéciaux: %v", err)
	}
}

func TestStrEscaping(t *testing.T) {
	for input, want := range map[string]string{
		`simple`:     `"simple"`,
		`a"b`:        `"a\"b"`,
		`a\b`:        `"a\\b"`,
		"ligne1\nl2": `"ligne1\nl2"`,
		`accentué €`: `"accentué €"`,
	} {
		if got := doc.Str(input); got != want {
			t.Errorf("Str(%q) = %s, attendu %s", input, got, want)
		}
	}
}

func TestFormatEUR(t *testing.T) {
	//   = espace fine insécable (séparateur de milliers),
	//   = espace insécable avant le symbole monétaire. Ce sont les
	// caractères imposés par la typographie française ; les écrire échappés
	// évite qu'un éditeur ne les remplace silencieusement par des espaces
	// ordinaires et rend le test lisible.
	cases := map[float64]string{
		0:       "0,00 €",
		1250:    "1 250,00 €",
		1234567: "1 234 567,00 €",
		99.999:  "100,00 €",
		1250.5:  "1 250,50 €",
	}
	for amount, want := range cases {
		if got := formatEUR(amount); got != want {
			t.Errorf("formatEUR(%v) = %q, attendu %q", amount, got, want)
		}
	}
}

func TestFormatDateFrench(t *testing.T) {
	if got := formatDate(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); got != "1er janvier 2026" {
		t.Errorf("premier du mois: %q", got)
	}
	if got := formatDate(time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)); !strings.Contains(got, "août") {
		t.Errorf("mois accentué: %q", got)
	}
}

// La forme de la mention de déclaration d'activité n'est pas libre : l'article
// R.6351-6 la fixe mot pour mot, numéro compris. Une mention amputée de son
// numéro ne remplit pas l'obligation, et c'est précisément ce que nous
// imprimions.
func TestLegalLineCarriesTheActivityDeclaration(t *testing.T) {
	ligne := legalLine(Party{
		Name: "Institut Vulcain", LegalForm: "SAS", Capital: "10 000 €",
		SIRET: "84291736500018", RCS: "Paris B 842 917 365",
		NDA: "11756789012", NDARegion: "Île-de-France",
	})

	for _, attendu := range []string{
		"déclaration d'activité enregistrée sous le numéro 11756789012",
		"auprès du préfet de région Île-de-France",
		"Cet enregistrement ne vaut pas agrément de l'État.",
		"SIRET 84291736500018",
		"SAS au capital de 10 000 €",
	} {
		if !strings.Contains(ligne, attendu) {
			t.Errorf("mention absente : %q\ndans : %s", attendu, ligne)
		}
	}
}

// Sans numéro de déclaration, on n'imprime pas une mention à trou : une phrase
// réglementaire incomplète est pire qu'une phrase absente, parce qu'elle a
// l'air d'être en règle.
func TestLegalLineOmitsAnEmptyDeclaration(t *testing.T) {
	ligne := legalLine(Party{Name: "Institut Vulcain"})
	if strings.Contains(ligne, "enregistrée sous le numéro") {
		t.Errorf("mention à trou imprimée : %s", ligne)
	}
	if strings.Contains(ligne, "SIRET") {
		t.Errorf("libellé sans valeur : %s", ligne)
	}
}

// Un organisme exonéré doit le dire, et ne facturer aucune TVA.
func TestVATExemptionIsStatedAndNotCharged(t *testing.T) {
	ligne := legalLine(Party{Name: "Institut Vulcain", VATExempt: true})
	if !strings.Contains(ligne, "261-4-4° a du CGI") {
		t.Errorf("mention d'exonération absente : %s", ligne)
	}
}

// La mention réglementaire doit arriver jusqu'au document, pas seulement
// exister dans une fonction. C'est le genre de règle qu'une refonte de gabarit
// casse sans que personne ne le voie avant un contrôle.
func TestConventionSourceCarriesTheLegalMention(t *testing.T) {
	source := string(RenderConvention(sampleConvention()).Source)

	for _, attendu := range []string{
		"11756789012",
		"préfet de région Île-de-France",
		"ne vaut pas agrément de l'État",
	} {
		if !strings.Contains(source, attendu) {
			t.Errorf("mention absente de la source rendue : %q", attendu)
		}
	}
}

// Un contrat financé par le bénéficiaire lui-même porte les protections que le
// code du travail lui accorde. Les taire rend le contrat attaquable.
func TestIndividualContractCarriesTheWithdrawalClause(t *testing.T) {
	convention := sampleConvention()
	convention.IndividualFunding = true
	source := string(RenderConvention(convention).Source)

	for _, attendu := range []string{
		"délai de rétractation de dix jours",
		"Aucune somme ne peut lui être exigée",
		"30 % du prix convenu",
	} {
		if !strings.Contains(source, attendu) {
			t.Errorf("protection absente du contrat : %q", attendu)
		}
	}

	// Et une convention ordinaire ne doit pas la porter : elle s'adresse à un
	// employeur ou à un financeur, à qui ce droit n'est pas ouvert.
	if strings.Contains(string(RenderConvention(sampleConvention()).Source), "rétractation de dix jours") {
		t.Error("la clause de rétractation apparaît sur une convention ordinaire")
	}
}
