package documents

import (
	"fmt"
	"time"

	"github.com/lemlearn/api/internal/platform/doc"
)

// La facture, telle qu'elle doit sortir.
//
// Les mentions obligatoires ne sont pas décoratives : une facture à laquelle il
// en manque une est irrégulière, et son destinataire peut refuser de déduire la
// TVA. Le gabarit les porte toutes — numéro, dates, identité complète des deux
// parties, détail des prestations, taux, totaux, délai et pénalités de retard,
// indemnité forfaitaire de recouvrement.
//
// Les cas particuliers d'un organisme de formation sont deux : l'exonération de
// l'article 261-4-4° a du CGI, qui remplace la ligne de taxe par une mention ;
// et l'avoir, qui porte les mêmes mentions en négatif et référence la facture
// qu'il corrige.

// LigneFacture est une ligne du détail.
type LigneFacture struct {
	Label       string
	Quantity    float64
	UnitPriceHT float64
	VATRate     float64
}

// Facture est ce qu'un gabarit doit connaître pour sortir la pièce.
type Facture struct {
	Number   string
	IssuedOn time.Time
	DueOn    string

	Org    Party
	Client Party

	FileReference string
	Lines         []LigneFacture

	VATExempt bool
	TotalHT   float64
	TotalVAT  float64
	TotalTTC  float64

	PaymentTerms string
	Notes        string

	// CreditNoteFor, s'il est renseigné, fait de la pièce un avoir.
	CreditNoteFor string

	LogoAsset string
	LogoBytes map[string][]byte
}

// RenderFacture produit la facture ou l'avoir.
func RenderFacture(f Facture) doc.Document {
	var s doc.Source

	genre := "Facture"
	if f.CreditNoteFor != "" {
		genre = "Avoir"
	}

	chrome := doc.Chrome{
		OrgName:    f.Org.Name,
		OrgAddress: f.Org.addressLine(),
		LegalLine:  legalLine(f.Org),
		Reference:  f.Number,
		Kind:       genre,
		LogoAsset:  f.LogoAsset,
	}
	chrome.WritePreamble(&s)

	s.Linef(`#lem_h1[%s %s]`, genre, f.Number)
	s.Linef(`#lem_mono(%s)`, doc.Str(fmt.Sprintf("Émise le %s · échéance le %s",
		formatDate(f.IssuedOn), formatJour(f.DueOn))))
	if f.CreditNoteFor != "" {
		s.Line(`#v(4pt)`)
		s.Linef(`#lem_mono(%s)`, doc.Str("Annule et remplace la facture "+f.CreditNoteFor))
	}
	s.Line(`#v(10pt)`)

	// Les deux parties, l'émetteur à gauche comme sur toute facture.
	s.Line(`#grid(columns: (1fr, 1fr), column-gutter: 14pt,`)
	s.Linef(`  [#lem_field(%s, %s)], [#lem_field(%s, %s)],`,
		doc.Str("Émetteur"), doc.Str(partyBlock(f.Org)),
		doc.Str("Client"), doc.Str(partyBlock(f.Client)))
	s.Line(`)`)
	if f.FileReference != "" {
		s.Line(`#v(6pt)`)
		s.Linef(`#lem_field(%s, %s)`, doc.Str("Dossier"), doc.Str(f.FileReference))
	}

	// Le détail
	s.Line(`#v(14pt)`)
	s.Line(`#table(`)
	s.Line(`  columns: (1fr, auto, auto, auto, auto),`)
	s.Line(`  inset: 7pt, align: (left, right, right, right, right),`)
	s.Line(`  stroke: (x, y) => if y == 0 { (bottom: 0.6pt) } else { (bottom: 0.2pt) },`)
	s.Linef(`  [*Prestation*], [*Qté*], [*P.U. HT*], [*TVA*], [*Total HT*],`)
	for _, ligne := range f.Lines {
		taux := "—"
		if !f.VATExempt {
			taux = trimFloat(ligne.VATRate) + " %"
		}
		s.Linef(`  [%s], [%s], [%s], [%s], [%s],`,
			escapeParagraph(ligne.Label),
			trimFloat(ligne.Quantity),
			formatEUR(ligne.UnitPriceHT),
			taux,
			formatEUR(ligne.Quantity*ligne.UnitPriceHT))
	}
	s.Line(`)`)

	// Les totaux
	s.Line(`#v(10pt)`)
	s.Line(`#align(right, block(width: 46%, [`)
	s.Linef(`#lem_field(%s, %s)`, doc.Str("Total hors taxes"), doc.Str(formatEUR(f.TotalHT)))
	if f.VATExempt {
		s.Line(`#v(3pt)`)
		s.Linef(`#lem_mono(%s)`, doc.Str("TVA non applicable"))
	} else {
		s.Line(`#v(3pt)`)
		s.Linef(`#lem_field(%s, %s)`, doc.Str("TVA"), doc.Str(formatEUR(f.TotalVAT)))
	}
	s.Line(`#v(3pt)`)
	s.Linef(`#lem_field(%s, %s)`, doc.Str("Total à payer"), doc.Str(formatEUR(f.TotalTTC)))
	s.Line(`]))`)

	// Les mentions. L'exonération d'abord : c'est celle qui explique une
	// facture sans taxe, et son absence rend la facture irrégulière.
	s.Line(`#v(14pt)`)
	if f.VATExempt {
		s.Linef(`#lem_mono(%s)`, doc.Str(
			"TVA non applicable — exonération au titre de l'article 261-4-4° a du code général "+
				"des impôts (actions de formation professionnelle continue)."))
		s.Line(`#v(6pt)`)
	}

	if f.PaymentTerms != "" {
		s.Linef(`#lem_field(%s, %s)`, doc.Str("Conditions de règlement"), doc.Str(f.PaymentTerms))
		s.Line(`#v(6pt)`)
	}

	// Les deux mentions que le code de commerce impose sur toute facture entre
	// professionnels, et qu'on oublie systématiquement en les tapant à la main.
	s.Linef(`#lem_mono(%s)`, doc.Str(
		"En cas de retard de paiement, une pénalité égale à trois fois le taux d'intérêt légal "+
			"sera exigible (article L.441-10 du code de commerce), ainsi qu'une indemnité "+
			"forfaitaire pour frais de recouvrement de 40 euros (article D.441-5). "+
			"Aucun escompte n'est accordé pour paiement anticipé."))

	if f.Notes != "" {
		s.Line(`#v(8pt)`)
		s.Linef(`%s`, escapeParagraph(f.Notes))
	}

	return doc.Document{Source: s.Bytes(), Assets: f.LogoBytes}
}

// formatJour rend une date déjà au format AAAA-MM-JJ.
func formatJour(jour string) string {
	parsed, err := time.Parse("2006-01-02", jour)
	if err != nil {
		return jour
	}
	return formatDate(parsed)
}
