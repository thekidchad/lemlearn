package doc

// Habillage commun à tous les documents lemlearn.
//
// Les PDF sont clairs, à l'inverse de l'application : ils sont imprimés,
// annotés et relus par un auditeur ou un financeur. La sobriété y est une
// exigence de lisibilité, pas un parti pris esthétique.

// Chrome décrit l'en-tête et le pied de page communs.
type Chrome struct {
	// Identité de l'organisme de formation, imprimée en tête.
	OrgName    string
	OrgAddress string
	// Mentions légales de pied de page : SIRET, NDA, RCS…
	LegalLine string
	// Référence du document (ex. « CONV-2026-0143 »), imprimée en tête et
	// reprise en pied — c'est elle que citera un auditeur.
	Reference string
	// Titre affiché en tête à droite (ex. « Convention de formation »).
	Kind string
}

// WritePreamble écrit les réglages de page, la palette, les aides de zone de
// signature et l'en-tête/pied. À appeler en premier sur toute source.
func (c Chrome) WritePreamble(s *Source) {
	s.Line(`#set text(font: "Geist", size: 9.5pt, lang: "fr", hyphenate: false)`)
	s.Line(`#set par(justify: false, leading: 0.62em, spacing: 0.9em)`)
	s.Line(`#let ink = rgb("#10131A")`)
	s.Line(`#let muted = rgb("#5B6170")`)
	s.Line(`#let faint = rgb("#8A90A0")`)
	s.Line(`#let hairline = rgb("#DFE2E8")`)
	s.Line(`#let accent = rgb("#4B37B8")`)
	s.Line(`#set text(fill: ink)`)

	writeSigZoneHelpers(s)
	writeSignedZoneHelper(s)

	s.Line(`#let lem_h1(body) = block(above: 0pt, below: 10pt)[#text(size: 15pt, weight: 640, tracking: -0.4pt, body)]`)
	s.Line(`#let lem_h2(body) = block(above: 14pt, below: 6pt)[#text(size: 10pt, weight: 620, body)]`)
	s.Line(`#let lem_label(body) = text(size: 6.6pt, weight: 560, tracking: 0.06em, fill: faint, upper(body))`)
	s.Line(`#let lem_mono(body) = text(font: "Geist Mono", size: 7.4pt, fill: muted, body)`)
	s.Line(`#let lem_rule = line(length: 100%, stroke: 0.5pt + hairline)`)

	// Bloc « champ » : étiquette au-dessus, valeur en dessous. Utilisé pour
	// toutes les identités et coordonnées, pour que les mêmes informations
	// soient toujours au même endroit d'un document à l'autre.
	s.Line(`#let lem_field(label, value) = block(width: 100%)[`)
	s.Line(`  #lem_label(label)#linebreak()`)
	s.Line(`  #text(size: 9pt, value)`)
	s.Line(`]`)

	s.Linef(`#set page(paper: "a4", margin: (top: 26mm, bottom: 22mm, x: 18mm),`)
	s.Linef(`  header: block(width: 100%%)[`)
	s.Linef(`    #grid(columns: (1fr, auto), align: (left + top, right + top),`)
	s.Linef(`      [#text(size: 9pt, weight: 640, %s)#linebreak()#text(size: 6.8pt, fill: faint, %s)],`,
		Str(c.OrgName), Str(c.OrgAddress))
	s.Linef(`      [#text(size: 6.8pt, fill: faint, %s)#linebreak()#lem_mono(%s)],`,
		Str(c.Kind), Str(c.Reference))
	s.Linef(`    )`)
	s.Linef(`    #v(4pt)`)
	s.Linef(`    #line(length: 100%%, stroke: 0.5pt + hairline)`)
	s.Linef(`  ],`)
	s.Linef(`  footer: block(width: 100%%)[`)
	s.Linef(`    #line(length: 100%%, stroke: 0.5pt + hairline)`)
	s.Linef(`    #v(3pt)`)
	s.Linef(`    #grid(columns: (1fr, auto), align: (left + horizon, right + horizon),`)
	s.Linef(`      [#text(size: 6pt, fill: faint, %s)],`, Str(c.LegalLine))
	s.Linef(`      [#lem_mono[#context counter(page).display("1 / 1", both: true)]],`)
	s.Linef(`    )`)
	s.Linef(`  ],`)
	s.Linef(`)`)
}

// writeSigZoneHelpers déclare les trois aides de zone.
//
//   - lem_sig_mark(role, kind, w, h) : métadonnée pure, ne dessine rien. Le
//     coin haut-gauche de la zone est la position de mise en page courante.
//   - lem_sig_zone(role, …) : dessine le cadre de signature ET le déclare.
//     La métadonnée est le premier contenu de la boîte, donc sa position est
//     bien celle du coin haut-gauche ; layout() résout la largeur absolue.
//   - lem_mention_zone(role, body) : déclare une zone couvrant `body`, mesuré
//     et non estimé, en rendant `body` inchangé — le PDF est identique, seule
//     la saisie guidée sait où mettre en évidence la mention manuscrite.
//
// Repris de khwiz (render_typst_chrome.go) : ces trois lignes sont éprouvées
// en production, y compris le piège du `context` nécessaire à here().
func writeSigZoneHelpers(s *Source) {
	s.Line(`#let lem_sig_mark(role, kind, w, h) = context [#metadata((role: role, kind: kind, page: here().position().page, x: here().position().x.pt(), y: here().position().y.pt(), w: w.pt(), h: h.pt()))<sig-zone>]`)
	s.Line(`#let lem_sig_zone(role, height: 22mm, radius: 4pt, inset: 5pt, caption_size: 6pt, caption: "[SIGNATURE ÉLECTRONIQUE — OTP · HORODATAGE RFC 3161]") = layout(size => box(width: 100%, height: height)[`)
	s.Line(`  #lem_sig_mark(role, "signature", size.width, height)`)
	s.Line(`  #block(height: 100%, width: 100%, stroke: (paint: rgb("#C7CBD4"), thickness: 0.5pt, dash: "dashed"), radius: radius, inset: inset)[`)
	s.Line(`    #align(center + horizon)[#text(size: caption_size, tracking: 0.05em, fill: rgb("#8A90A0"), caption)]`)
	s.Line(`  ]`)
	s.Line(`])`)
	s.Line(`#let lem_mention_zone(role, body) = context layout(size => {`)
	s.Line(`  let m = measure(box(width: size.width)[#body])`)
	s.Line(`  [#lem_sig_mark(role, "handwritten_mention", size.width, m.height)]`)
	s.Line(`  body`)
	s.Line(`})`)
}
