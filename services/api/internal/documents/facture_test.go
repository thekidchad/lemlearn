package documents

import (
	"strings"
	"testing"
	"time"
)

func factureExemple() Facture {
	return Facture{
		Number:   "FA-2026-0002",
		IssuedOn: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC),
		DueOn:    "2026-10-03",
		Org: Party{
			Name: "Institut Vulcain", LegalForm: "SARL",
			Address: "12 rue de la Forge", PostalCode: "59000", City: "Lille",
			SIRET: "84291736500018", NDA: "32591234559", NDARegion: "Hauts-de-France",
			Represented: "Marie Dubreuil", Role: "gérante",
		},
		Client: Party{
			Name: "Aciéries du Nord", SIRET: "39876543200018",
			Address: "8 quai du Bassin", PostalCode: "59300", City: "Valenciennes",
		},
		FileReference: "DOS-2026-629361",
		Lines: []LigneFacture{
			{Label: "Formation Gestes et postures — 7 h", Quantity: 1, UnitPriceHT: 840, VATRate: 20},
		},
		TotalHT: 840, TotalVAT: 168, TotalTTC: 1008,
		PaymentTerms: "Virement à 30 jours",
	}
}

// Les mentions du code de commerce sont les plus souvent oubliées quand on tape
// une facture à la main, et leur absence la rend irrégulière.
func TestFactureCarriesMandatoryMentions(t *testing.T) {
	source := string(RenderFacture(factureExemple()).Source)

	for _, attendu := range []string{
		"FA-2026-0002",
		"Institut Vulcain",
		"Aciéries du Nord",
		"DOS-2026-629361",
		"L.441-10",
		"D.441-5",
		"40 euros",
		"Aucun escompte",
	} {
		if !strings.Contains(source, attendu) {
			t.Errorf("la facture ne porte pas %q", attendu)
		}
	}
}

// Un organisme exonéré ne doit porter aucune taxe, et doit dire pourquoi.
func TestFactureExonereeRemplaceLaTaxeParLaMention(t *testing.T) {
	facture := factureExemple()
	facture.VATExempt = true
	facture.TotalVAT, facture.TotalTTC = 0, 840

	source := string(RenderFacture(facture).Source)

	if !strings.Contains(source, "261-4-4° a du code général") {
		t.Error("la mention d'exonération manque")
	}
	if !strings.Contains(source, "TVA non applicable") {
		t.Error("la facture exonérée doit dire que la TVA ne s'applique pas")
	}
	// Le taux ne doit pas s'afficher : « 20 % » sur une facture exonérée est
	// une contradiction que le client verra avant nous.
	if strings.Contains(source, "20 %") {
		t.Error("un taux de TVA figure sur une facture exonérée")
	}
}

// Un avoir se nomme avoir, porte des montants négatifs et référence sa facture.
func TestAvoirReferenceLaFactureCorrigee(t *testing.T) {
	facture := factureExemple()
	facture.Number = "AV-2026-0005"
	facture.CreditNoteFor = "FA-2026-0002"
	facture.Lines[0].Quantity = -1
	facture.TotalHT, facture.TotalVAT, facture.TotalTTC = -840, -168, -1008

	source := string(RenderFacture(facture).Source)

	if !strings.Contains(source, "Avoir AV-2026-0005") {
		t.Error("la pièce ne s'annonce pas comme un avoir")
	}
	if !strings.Contains(source, "Annule et remplace la facture FA-2026-0002") {
		t.Error("l'avoir ne référence pas la facture corrigée")
	}
	if !strings.Contains(source, "-840") && !strings.Contains(source, "−840") {
		t.Error("les montants de l'avoir ne sont pas négatifs")
	}
}

// Le rendu doit rester reproductible : deux compilations de la même facture
// donnent la même source, donc la même empreinte.
func TestFactureRenderIsReproducible(t *testing.T) {
	premier := string(RenderFacture(factureExemple()).Source)
	second := string(RenderFacture(factureExemple()).Source)
	if premier != second {
		t.Error("deux rendus de la même facture diffèrent")
	}
}
