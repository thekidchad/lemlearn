package doc

import (
	"fmt"
	"time"
)

// AppliedSignature est une signature à rendre dans sa zone.
//
// Le gabarit rend la signature *à sa place* plutôt que de la tamponner après
// coup sur un PDF existant. Deux conséquences : le document signé est un rendu
// de plein droit, reproductible à l'octet près, et il n'y a pas de
// post-traitement dont il faudrait démontrer la fidélité au document présenté.
type AppliedSignature struct {
	Role SignatureZoneRole
	Name string
	// Mention est la mention manuscrite saisie, imprimée au-dessus du tracé.
	Mention string
	// SignedAt est l'horodatage affiché sous le tracé.
	SignedAt time.Time
	// Drawing est l'image PNG du tracé.
	Drawing []byte
	// Certificate est la ligne de scellement imprimée sous la signature :
	// empreinte du document, autorité d'horodatage. Composée par l'appelant,
	// qui seul connaît le résultat du scellement.
	Certificate string
}

// AssetName est le nom du fichier d'image associé à une signature.
func (a AppliedSignature) AssetName() string {
	return fmt.Sprintf("signature-%s.png", a.Role)
}

// FindSignature retrouve la signature d'un rôle.
func FindSignature(applied []AppliedSignature, role SignatureZoneRole) (AppliedSignature, bool) {
	for _, signature := range applied {
		if signature.Role == role {
			return signature, true
		}
	}
	return AppliedSignature{}, false
}

// Assets rassemble les images de tracé pour le compilateur.
func Assets(applied []AppliedSignature) map[string][]byte {
	if len(applied) == 0 {
		return nil
	}
	assets := make(map[string][]byte, len(applied))
	for _, signature := range applied {
		if len(signature.Drawing) > 0 {
			assets[signature.AssetName()] = signature.Drawing
		}
	}
	return assets
}

// WriteSignatureSlot écrit soit le cadre vierge, soit la signature apposée.
//
// Les deux passent par `lem_sig_zone` / `lem_sig_mark`, donc la zone est
// déclarée dans les deux cas : un document signé conserve les coordonnées de
// ses cadres, ce qui permet de le recontrôler des années plus tard.
func WriteSignatureSlot(s *Source, role SignatureZoneRole, applied []AppliedSignature) {
	signature, ok := FindSignature(applied, role)
	if !ok {
		s.Linef(`#lem_sig_zone(%s)`, Str(string(role)))
		return
	}

	s.Linef(`#lem_signed_zone(%s, %s, %s, %s, %s)`,
		Str(string(role)),
		Str(signature.AssetName()),
		Str(signature.Name),
		Str(formatSignedAt(signature.SignedAt)),
		Str(signature.Certificate),
	)
}

// writeSignedZoneHelper déclare l'aide de rendu d'une signature apposée.
func writeSignedZoneHelper(s *Source) {
	s.Line(`#let lem_signed_zone(role, asset, name, stamp, certificate, height: 22mm) = layout(size => box(width: 100%, height: height)[`)
	s.Line(`  #lem_sig_mark(role, "signature", size.width, height)`)
	s.Line(`  #block(height: 100%, width: 100%, stroke: (paint: rgb("#C7CBD4"), thickness: 0.5pt), radius: 4pt, inset: 4pt)[`)
	s.Line(`    #grid(columns: (auto, 1fr), column-gutter: 6pt, align: (left + horizon, left + top),`)
	s.Line(`      [#image(asset, height: 13mm)],`)
	s.Line(`      [`)
	s.Line(`        #text(size: 7.5pt, weight: 560, name)#linebreak()`)
	s.Line(`        #text(size: 6.2pt, fill: muted, stamp)#linebreak()`)
	s.Line(`        #text(font: "Geist Mono", size: 5.2pt, fill: faint, certificate)`)
	s.Line(`      ],`)
	s.Line(`    )`)
	s.Line(`  ]`)
	s.Line(`])`)
}

// formatSignedAt rend l'horodatage affiché sous une signature, à la seconde et
// en heure de Paris — c'est l'heure que reconnaîtra le signataire.
func formatSignedAt(at time.Time) string {
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		loc = time.UTC
	}
	local := at.In(loc)
	return fmt.Sprintf("Signé électroniquement le %02d/%02d/%d à %02d:%02d:%02d",
		local.Day(), int(local.Month()), local.Year(),
		local.Hour(), local.Minute(), local.Second())
}
