// Package documents contient les gabarits métier de lemlearn : convention de
// formation, devis, feuille d'émargement, relevé de connexion, attestation,
// dossier de preuve. Chaque gabarit produit une source Typst ; la compilation
// et l'extraction des zones de signature sont assurées par platform/doc.
package documents

import (
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/platform/doc"
)

// Party est une personne morale ou physique partie au contrat.
type Party struct {
	Name        string
	LegalForm   string // SARL, SAS, association…
	Address     string
	PostalCode  string
	City        string
	SIRET       string
	Represented string // nom du représentant signataire
	Role        string // qualité du représentant (gérant, DRH…)

	// Ce qui suit ne concerne que l'organisme de formation, et n'apparaît
	// qu'en pied de page.
	NDA       string
	NDARegion string
	Capital   string
	RCS       string
	VATNumber string
	VATExempt bool
}

// addressLine compose l'adresse sur une ligne en omettant ce qui manque.
//
// Un fmt.Sprintf naïf imprimait « , » tout seul pour un organisme dont
// l'adresse n'est pas encore renseignée — une virgule orpheline en tête de
// convention, sur tous les documents d'un client qui vient de s'inscrire.
func (p Party) addressLine() string {
	parts := make([]string, 0, 2)
	if street := strings.TrimSpace(p.Address); street != "" {
		parts = append(parts, street)
	}
	if locality := strings.TrimSpace(p.PostalCode + " " + p.City); locality != "" {
		parts = append(parts, locality)
	}
	return strings.Join(parts, ", ")
}

// LearnerLine est un apprenant inscrit à la session.
type LearnerLine struct {
	FullName string
	Position string // fonction dans l'entreprise
}

// SessionLine est un créneau planifié.
type SessionLine struct {
	Start    time.Time
	End      time.Time
	Mode     string // « Présentiel », « Distanciel synchrone », « Asynchrone »
	Location string
}

// Convention porte toutes les données d'une convention de formation
// professionnelle (art. L6353-1 et suivants du code du travail).
type Convention struct {
	Reference string
	IssuedOn  time.Time

	Org    Party
	Client Party

	CourseTitle     string
	CourseGoal      string
	Objectives      []string
	Prerequisites   string
	Audience        string
	DurationHours   float64
	Modalities      string
	Means           string
	Assessment      string
	Sanction        string
	AccessibilityPS string // modalités d'accès aux personnes en situation de handicap

	Learners []LearnerLine
	Sessions []SessionLine

	PriceHT      float64
	VATRate      float64
	PaymentTerms string
	FunderName   string // OPCO ou financeur ; vide si financement direct

	SignedCity string

	// IndividualFunding dit que le bénéficiaire finance lui-même sa formation.
	// Le document change alors de nature : ce n'est plus une convention entre
	// personnes morales mais un contrat de formation professionnelle, et le
	// code du travail y attache des protections qu'aucune clause ne peut
	// écarter.
	IndividualFunding bool

	// LogoAsset et LogoBytes portent le logo de l'organisme, incorporé au
	// document. Il est embarqué et non référencé : un document scellé ne peut
	// pas dépendre d'une image qu'un lecteur irait chercher sur le réseau des
	// années plus tard.
	LogoAsset string
	LogoBytes map[string][]byte

	// Signatures apposées. Vide, le gabarit rend des cadres vierges ; c'est
	// le document présenté au signataire. Renseigné, il rend le document
	// signé — même gabarit, même code, aucun post-traitement.
	Signatures []doc.AppliedSignature
}

// RenderConvention produit la source Typst de la convention.
//
// Les deux zones de signature (organisme et client) sont déclarées dans le
// gabarit : c'est lui qui sait où elles tombent après mise en page, pas le
// code d'envoi. Une mention manuscrite guidée précède la signature du client.
func RenderConvention(c Convention) doc.Document {
	var s doc.Source

	chrome := doc.Chrome{
		OrgName:    c.Org.Name,
		OrgAddress: c.Org.addressLine(),
		LegalLine:  legalLine(c.Org),
		Reference:  c.Reference,
		Kind:       "Convention de formation professionnelle",
		LogoAsset:  c.LogoAsset,
	}
	chrome.WritePreamble(&s)

	s.Linef(`#lem_h1[Convention de formation professionnelle]`)
	// Une chaîne placée dans un bloc de contenu Typst `[...]` est rendue avec
	// ses guillemets typographiques (« CONV-2026-0143 ») : les valeurs
	// doivent être passées en *argument* des aides, jamais en contenu.
	s.Linef(`#lem_mono(%s)`, doc.Str(fmt.Sprintf("%s · établie le %s", c.Reference, formatDate(c.IssuedOn))))
	s.Line(`#v(10pt)`)

	// Parties
	s.Line(`#grid(columns: (1fr, 1fr), column-gutter: 14pt,`)
	s.Linef(`  [#lem_field(%s, %s)], [#lem_field(%s, %s)],`,
		doc.Str("L'organisme de formation"), doc.Str(partyBlock(c.Org)),
		doc.Str("Le bénéficiaire"), doc.Str(partyBlock(c.Client)))
	s.Line(`)`)
	if c.FunderName != "" {
		s.Line(`#v(6pt)`)
		s.Linef(`#lem_field(%s, %s)`, doc.Str("Financeur"), doc.Str(c.FunderName))
	}

	// Article 1 — objet
	s.Line(`#lem_h2[Article 1 — Objet de la convention]`)
	s.Linef(`L'organisme organise l'action de formation intitulée #text(weight: 600, %s), d'une durée de #text(weight: 600, %s), au bénéfice des participants désignés à l'article 3.`,
		doc.Str(c.CourseTitle), doc.Str(formatHours(c.DurationHours)))
	if c.CourseGoal != "" {
		s.Linef(`#v(4pt)%s`, escapeParagraph(c.CourseGoal))
	}

	// Article 2 — nature et objectifs
	s.Line(`#lem_h2[Article 2 — Nature, objectifs et modalités]`)
	s.Line(`#grid(columns: (1fr, 1fr), column-gutter: 14pt, row-gutter: 8pt,`)
	s.Linef(`  [#lem_field(%s, %s)], [#lem_field(%s, %s)],`,
		doc.Str("Public visé"), doc.Str(c.Audience),
		doc.Str("Prérequis"), doc.Str(orDash(c.Prerequisites)))
	s.Linef(`  [#lem_field(%s, %s)], [#lem_field(%s, %s)],`,
		doc.Str("Modalités"), doc.Str(c.Modalities),
		doc.Str("Sanction de la formation"), doc.Str(c.Sanction))
	s.Line(`)`)

	if len(c.Objectives) > 0 {
		s.Line(`#v(8pt)`)
		s.Linef(`#lem_label(%s)`, doc.Str("Objectifs pédagogiques"))
		s.Line(`#v(3pt)`)
		s.Line(`#list(marker: [•], spacing: 4pt,`)
		for _, objective := range c.Objectives {
			s.Linef(`  [%s],`, escapeParagraph(objective))
		}
		s.Line(`)`)
	}

	s.Line(`#v(6pt)`)
	s.Linef(`#lem_field(%s, %s)`, doc.Str("Moyens pédagogiques et techniques"), doc.Str(c.Means))
	s.Line(`#v(6pt)`)
	s.Linef(`#lem_field(%s, %s)`, doc.Str("Modalités d'évaluation des acquis"), doc.Str(c.Assessment))
	if c.AccessibilityPS != "" {
		s.Line(`#v(6pt)`)
		s.Linef(`#lem_field(%s, %s)`, doc.Str("Accessibilité aux personnes en situation de handicap"), doc.Str(c.AccessibilityPS))
	}

	// Article 3 — participants et calendrier
	s.Line(`#lem_h2[Article 3 — Participants et calendrier]`)
	writeLearnersTable(&s, c.Learners)
	if len(c.Sessions) > 0 {
		s.Line(`#v(8pt)`)
		writeSessionsTable(&s, c.Sessions)
	}

	// Article 4 — dispositions financières
	s.Line(`#lem_h2[Article 4 — Dispositions financières]`)
	// Un organisme exonéré ne facture pas de TVA, et sa facture doit le dire.
	// Imprimer « TVA 0 % » laisserait croire à un taux nul appliqué, ce qui
	// n'est pas la même chose qu'une opération hors champ.
	if c.Org.VATExempt {
		s.Line(`#grid(columns: (1fr, 1fr), column-gutter: 14pt,`)
		s.Linef(`  [#lem_field(%s, %s)], [#lem_field(%s, %s)],`,
			doc.Str("Prix net"), doc.Str(formatEUR(c.PriceHT)),
			doc.Str("TVA"), doc.Str("exonérée — art. 261-4-4° a du CGI"))
		s.Line(`)`)
	} else {
		ttc := c.PriceHT * (1 + c.VATRate/100)
		s.Line(`#grid(columns: (1fr, 1fr, 1fr), column-gutter: 14pt,`)
		s.Linef(`  [#lem_field(%s, %s)], [#lem_field(%s, %s)], [#lem_field(%s, %s)],`,
			doc.Str("Prix net HT"), doc.Str(formatEUR(c.PriceHT)),
			doc.Str(fmt.Sprintf("TVA %s %%", trimFloat(c.VATRate))), doc.Str(formatEUR(ttc-c.PriceHT)),
			doc.Str("Total TTC"), doc.Str(formatEUR(ttc)))
		s.Line(`)`)
	}
	s.Line(`#v(6pt)`)
	s.Linef(`#lem_field(%s, %s)`, doc.Str("Modalités de règlement"), doc.Str(c.PaymentTerms))

	// Article 5 — assiduité (le cœur de la preuve)
	s.Line(`#lem_h2[Article 5 — Assiduité et justification de la réalisation]`)
	s.Line(`La réalisation de l'action est justifiée par les feuilles d'émargement signées électroniquement pour les séquences en présentiel ou en classe virtuelle, et par les relevés de connexion horodatés pour les séquences asynchrones. Ces relevés détaillent, pour chaque participant, la durée réellement visionnée de chaque module ainsi que les résultats aux évaluations. Ils sont tenus à disposition du financeur et de tout organisme de contrôle.`)

	// Article 6 — dédit
	s.Line(`#lem_h2[Article 6 — Dédit et abandon]`)
	s.Line(`En cas de renoncement du bénéficiaire moins de dix jours ouvrés avant le début de l'action, l'organisme retient 30 % du prix convenu à titre de dédommagement. En cas d'abandon en cours de formation, seules les heures réellement suivies sont facturées au financeur ; le solde reste à la charge du bénéficiaire.`)

	// Article 6 bis — protections du particulier qui finance lui-même.
	//
	// Elles ne sont pas négociables : l'article L.6353-5 du code du travail
	// ouvre dix jours de rétractation au stagiaire qui contracte à titre
	// individuel, l'article L.6353-6 interdit d'exiger quoi que ce soit avant
	// l'expiration de ce délai, puis plafonne à 30 % du prix ce qui peut être
	// demandé ensuite. Un contrat qui les tait est attaquable, et c'est le
	// document même qui fonde la créance.
	if c.IndividualFunding {
		s.Line(`#lem_h2[Article 7 — Rétractation et échelonnement du paiement]`)
		s.Line(`Le bénéficiaire finançant lui-même sa formation dispose d'un délai de rétractation de dix jours à compter de la signature du présent contrat. Il exerce ce droit par lettre recommandée avec accusé de réception. Aucune somme ne peut lui être exigée avant l'expiration de ce délai. À l'issue de celui-ci, il ne peut être demandé plus de 30 % du prix convenu ; le solde est réglé au fur et à mesure du déroulement de l'action, selon l'échéancier figurant à l'article 4.`)
		s.Line(`#lem_h2[Article 8 — Données personnelles]`)
	} else {
		s.Line(`#lem_h2[Article 7 — Données personnelles]`)
	}
	s.Line(`Les données collectées sont traitées pour l'exécution de la présente convention et la justification de la réalisation de l'action. Elles sont conservées le temps requis par les obligations légales d'archivage puis anonymisées. Le bénéficiaire dispose d'un droit d'accès, de rectification, de portabilité et d'effacement, exerçable auprès de l'organisme.`)

	// Signatures
	writeSignatureBlock(&s, c)

	assets := doc.Assets(c.Signatures)
	for nom, octets := range c.LogoBytes {
		if assets == nil {
			assets = map[string][]byte{}
		}
		assets[nom] = octets
	}

	return doc.Document{
		Source:       s.Bytes(),
		Assets:       assets,
		CreationUnix: c.IssuedOn.Unix(),
	}
}

func writeSignatureBlock(s *doc.Source, c Convention) {
	s.Line(`#v(16pt)`)
	s.Linef(`#lem_mono(%s)`, doc.Str(fmt.Sprintf(
		"Fait à %s, le %s. Établi en un exemplaire électronique original.",
		c.SignedCity, formatDate(c.IssuedOn))))
	s.Line(`#v(10pt)`)
	// breakable: false — un bloc de signature coupé en deux laisse les
	// libellés « Pour l'organisme » sur une page et les cadres sur la
	// suivante. Un document contractuel ne peut pas se présenter ainsi.
	s.Line(`#block(breakable: false, width: 100%)[`)
	s.Line(`#grid(columns: (1fr, 1fr), column-gutter: 18pt, row-gutter: 5pt,`)
	s.Linef(`  [#lem_label(%s)], [#lem_label(%s)],`,
		doc.Str("Pour l'organisme de formation"),
		doc.Str("Pour le bénéficiaire"))
	s.Linef(`  [#text(size: 8pt, %s)], [#text(size: 8pt, %s)],`,
		doc.Str(signatoryLine(c.Org)), doc.Str(signatoryLine(c.Client)))

	// La mention manuscrite n'est demandée qu'au bénéficiaire : c'est la
	// partie dont l'engagement doit être caractérisé face à un financeur.
	// Une fois signée, on imprime la mention réellement saisie plutôt que la
	// consigne.
	if signed, ok := doc.FindSignature(c.Signatures, doc.RoleClient); ok && signed.Mention != "" {
		s.Linef(`  [#v(4pt)], [#lem_mention_zone("client")[#text(size: 7.5pt, style: "italic", %s)]],`,
			doc.Str(signed.Mention))
	} else {
		s.Line(`  [#v(4pt)], [#lem_mention_zone("client")[#text(size: 7.5pt, fill: muted)[Faire précéder la signature de la mention « Lu et approuvé, bon pour accord »]]],`)
	}

	s.Line(`  [`)
	doc.WriteSignatureSlot(s, doc.RoleOrganization, c.Signatures)
	s.Line(`  ], [`)
	doc.WriteSignatureSlot(s, doc.RoleClient, c.Signatures)
	s.Line(`  ],`)
	s.Line(`)`)
	s.Line(`]`)
}

func writeLearnersTable(s *doc.Source, learners []LearnerLine) {
	s.Line(`#table(columns: (auto, 1fr, 1fr), stroke: none, inset: (x: 0pt, y: 4pt), column-gutter: 12pt,`)
	s.Line(`  table.hline(stroke: 0.5pt + hairline),`)
	s.Linef(`  [#lem_label(%s)], [#lem_label(%s)], [#lem_label(%s)],`,
		doc.Str("№"), doc.Str("Participant"), doc.Str("Fonction"))
	s.Line(`  table.hline(stroke: 0.5pt + hairline),`)
	for i, learner := range learners {
		s.Linef(`  [#lem_mono(%s)], [#text(size: 9pt, %s)], [#text(size: 9pt, fill: muted, %s)],`,
			doc.Str(fmt.Sprintf("%02d", i+1)), doc.Str(learner.FullName), doc.Str(orDash(learner.Position)))
	}
	s.Line(`  table.hline(stroke: 0.5pt + hairline),`)
	s.Line(`)`)
}

func writeSessionsTable(s *doc.Source, sessions []SessionLine) {
	s.Line(`#table(columns: (auto, auto, auto, 1fr), stroke: none, inset: (x: 0pt, y: 4pt), column-gutter: 12pt,`)
	s.Line(`  table.hline(stroke: 0.5pt + hairline),`)
	s.Linef(`  [#lem_label(%s)], [#lem_label(%s)], [#lem_label(%s)], [#lem_label(%s)],`,
		doc.Str("Date"), doc.Str("Horaires"), doc.Str("Modalité"), doc.Str("Lieu / accès"))
	s.Line(`  table.hline(stroke: 0.5pt + hairline),`)
	for _, session := range sessions {
		s.Linef(`  [#text(size: 9pt, %s)], [#lem_mono(%s)], [#text(size: 9pt, %s)], [#text(size: 9pt, fill: muted, %s)],`,
			doc.Str(formatDate(session.Start)),
			doc.Str(fmt.Sprintf("%s – %s", session.Start.Format("15:04"), session.End.Format("15:04"))),
			doc.Str(session.Mode),
			doc.Str(orDash(session.Location)))
	}
	s.Line(`  table.hline(stroke: 0.5pt + hairline),`)
	s.Line(`)`)
}

// legalLine compose la mention de pied de page.
//
// La forme de la mention de déclaration d'activité n'est pas libre :
// l'article R.6351-6 du code du travail la fixe mot pour mot — « déclaration
// d'activité enregistrée sous le numéro … auprès du préfet de région de … » —
// et impose de la faire suivre de « Cet enregistrement ne vaut pas agrément de
// l'État ». Une mention amputée de son numéro ne remplit pas l'obligation.
//
// Chaque élément est omis lorsqu'il manque plutôt que rendu avec un vide à la
// suite : « SIRET  » sur une convention se remarque plus qu'une absence.
func legalLine(org Party) string {
	parts := []string{org.Name}
	if forme := strings.TrimSpace(org.LegalForm); forme != "" {
		mention := forme
		if capital := strings.TrimSpace(org.Capital); capital != "" {
			mention += " au capital de " + capital
		}
		parts = append(parts, mention)
	}
	if siret := strings.TrimSpace(org.SIRET); siret != "" {
		parts = append(parts, "SIRET "+siret)
	}
	if rcs := strings.TrimSpace(org.RCS); rcs != "" {
		parts = append(parts, "RCS "+rcs)
	}
	if tva := strings.TrimSpace(org.VATNumber); tva != "" {
		parts = append(parts, "TVA "+tva)
	}

	line := strings.Join(parts, " — ")

	if nda := strings.TrimSpace(org.NDA); nda != "" {
		mention := " — déclaration d'activité enregistrée sous le numéro " + nda
		if region := strings.TrimSpace(org.NDARegion); region != "" {
			mention += " auprès du préfet de région " + region
		}
		line += mention + ". Cet enregistrement ne vaut pas agrément de l'État."
	}

	if org.VATExempt {
		line += " Exonéré de TVA — art. 261-4-4° a du CGI."
	}
	return line
}

func partyBlock(p Party) string {
	lines := []string{p.Name}
	if p.LegalForm != "" {
		lines[0] = fmt.Sprintf("%s (%s)", p.Name, p.LegalForm)
	}
	lines = append(lines, p.addressLine())
	if p.SIRET != "" {
		lines = append(lines, "SIRET "+p.SIRET)
	}
	if p.Represented != "" {
		represented := "Représenté par " + p.Represented
		if p.Role != "" {
			represented += ", " + p.Role
		}
		lines = append(lines, represented)
	}
	return strings.Join(lines, "\n")
}

func signatoryLine(p Party) string {
	if p.Represented == "" {
		return p.Name
	}
	return fmt.Sprintf("%s — %s", p.Represented, p.Name)
}
