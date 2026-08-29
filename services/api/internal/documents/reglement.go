package documents

import (
	"fmt"
	"strings"

	"github.com/lemlearn/api/internal/platform/doc"
)

// Règlement intérieur de l'organisme de formation.
//
// Tout organisme doit en établir un (art. L.6352-3) et le remettre au stagiaire
// avant son inscription définitive. Son contenu n'est pas libre : les articles
// R.6352-1 à R.6352-15 fixent ce qu'il doit couvrir — hygiène et sécurité,
// discipline, nature et échelle des sanctions, garanties de procédure, et les
// règles de représentation des stagiaires pour les formations de plus de cinq
// cents heures.
//
// Le texte est donc essentiellement dicté par le droit, et c'est délibérément
// nous qui le portons plutôt que chaque client : un organisme qui rédige le
// sien y oublie la moitié des garanties procédurales, et c'est précisément ce
// qu'un contrôle relève. Ne varie que ce qui lui appartient — son identité, et
// le fait qu'il dispense ou non des formations longues.

// Reglement décrit le règlement d'un organisme.
type Reglement struct {
	Org      Party
	IssuedOn timeLike

	// LongCourses dit que l'organisme dispense des formations de plus de cinq
	// cents heures. La section sur la représentation des stagiaires ne
	// s'impose qu'à celles-là ; l'inclure partout ferait promettre des
	// élections qui n'auront jamais lieu.
	LongCourses bool
}

// timeLike évite d'importer time dans la signature publique du gabarit sans
// raison : seule la date d'établissement est imprimée.
type timeLike interface {
	Format(layout string) string
	Unix() int64
}

// RenderReglement produit la source Typst du règlement intérieur.
func RenderReglement(r Reglement) doc.Document {
	var s doc.Source

	chrome := doc.Chrome{
		OrgName:    r.Org.Name,
		OrgAddress: r.Org.addressLine(),
		LegalLine:  legalLine(r.Org),
		Reference:  "RI",
		Kind:       "Règlement intérieur",
	}
	chrome.WritePreamble(&s)

	s.Line(`#lem_h1[Règlement intérieur]`)
	s.Linef(`#lem_mono(%s)`, doc.Str(fmt.Sprintf(
		"Établi conformément aux articles L.6352-3 à L.6352-5 et R.6352-1 à R.6352-15 du code du travail. En vigueur au %s.",
		r.IssuedOn.Format("02/01/2006"))))
	s.Line(`#v(10pt)`)

	s.Line(`#lem_h2[Article 1 — Objet et champ d'application]`)
	s.Linef(`Le présent règlement s'applique à toute personne suivant une action de formation dispensée par #text(weight: 600, %s), quelle qu'en soit la durée ou la modalité, y compris lorsque la formation se déroule dans des locaux mis à disposition par un tiers. Chaque stagiaire en reçoit un exemplaire avant son inscription définitive.`,
		doc.Str(r.Org.Name))

	s.Line(`#lem_h2[Article 2 — Hygiène et sécurité]`)
	s.Line(`Chaque stagiaire observe les règles d'hygiène et de sécurité en vigueur dans les locaux où se déroule la formation. Lorsque la formation se tient dans une entreprise ou un établissement déjà doté d'un règlement intérieur, ce sont les mesures de santé et de sécurité de cet établissement qui s'appliquent, en application de l'article R.6352-1 du code du travail.`)
	s.Line(`Le stagiaire prend connaissance des consignes d'évacuation et de l'emplacement des moyens de secours dès son arrivée. Tout accident, même bénin, survenu pendant la formation ou sur le trajet est signalé sans délai au responsable de l'organisme, qui procède aux déclarations qui lui incombent.`)
	s.Line(`Il est interdit de fumer et de vapoter dans les locaux de formation. L'introduction et la consommation de boissons alcoolisées y sont proscrites, hors autorisation expresse et encadrée.`)

	s.Line(`#lem_h2[Article 3 — Discipline]`)
	s.Line(`Le stagiaire se conforme aux horaires fixés et communiqués. Son assiduité est attestée par les feuilles d'émargement qu'il signe, ou par les relevés de connexion horodatés pour les séquences suivies à distance. Une absence, un retard ou un départ anticipé est justifié auprès de l'organisme, qui en informe le financeur lorsque la prise en charge en dépend.`)
	s.Line(`Le stagiaire respecte le matériel et les supports mis à sa disposition. Les documents pédagogiques lui sont remis pour son usage personnel : leur reproduction ou leur diffusion suppose l'accord écrit de l'organisme.`)
	s.Line(`Le comportement de chacun respecte la dignité d'autrui. Tout agissement constitutif de harcèlement moral ou sexuel, ainsi que tout propos ou comportement discriminatoire, est proscrit et expose son auteur aux sanctions prévues à l'article suivant, sans préjudice des poursuites judiciaires.`)

	s.Line(`#lem_h2[Article 4 — Nature et échelle des sanctions]`)
	s.Line(`Constitue une sanction toute mesure, autre qu'une observation verbale, prise par le responsable de l'organisme à la suite d'un agissement du stagiaire qu'il considère comme fautif. Selon la gravité des faits, la sanction peut être :`)
	writeList(&s, []string{
		"un avertissement écrit ;",
		"un blâme ;",
		"une exclusion temporaire de la formation ;",
		"une exclusion définitive de la formation.",
	})
	s.Line(`Les amendes et autres sanctions pécuniaires sont interdites (art. R.6352-3). L'organisme informe l'employeur et, le cas échéant, le financeur de la sanction prise.`)

	s.Line(`#lem_h2[Article 5 — Garanties de procédure]`)
	s.Line(`Aucune sanction ne peut être infligée sans que le stagiaire ait été informé au préalable, et par écrit, des griefs retenus contre lui. Lorsque la sanction envisagée a une incidence, immédiate ou non, sur sa présence dans la formation, le stagiaire est convoqué à un entretien : la convocation en indique l'objet, la date, l'heure et le lieu, et rappelle qu'il peut se faire assister par la personne de son choix, stagiaire ou salarié de l'organisme.`)
	s.Line(`Au cours de l'entretien, le motif de la sanction envisagée est exposé et les explications du stagiaire recueillies. La sanction ne peut intervenir moins d'un jour franc ni plus de quinze jours après l'entretien. Elle est notifiée par écrit et motivée.`)
	s.Line(`Lorsqu'un agissement rend indispensable une mesure conservatoire d'exclusion temporaire à effet immédiat, aucune sanction définitive n'est prise sans que la procédure ci-dessus ait été observée.`)

	if r.LongCourses {
		s.Line(`#lem_h2[Article 6 — Représentation des stagiaires]`)
		s.Line(`Pour les formations d'une durée supérieure à cinq cents heures, il est procédé à l'élection d'un délégué titulaire et d'un délégué suppléant, au scrutin uninominal à deux tours. Tous les stagiaires sont électeurs et éligibles. L'organisme organise le scrutin au plus tard vingt heures après le début de la formation ; si aucune candidature n'est présentée, il dresse un procès-verbal de carence.`)
		s.Line(`Les délégués sont élus pour la durée de la formation. Ils présentent les réclamations individuelles ou collectives relatives au déroulement de la formation, aux conditions de santé et de sécurité et à l'application du présent règlement, et font toute suggestion pour améliorer les conditions de vie des stagiaires.`)
	}

	numero := 6
	if r.LongCourses {
		numero = 7
	}
	s.Linef(`#lem_h2[Article %d — Réclamations]`, numero)
	s.Linef(`Toute réclamation relative au déroulement d'une formation peut être adressée à #text(%s), qui en accuse réception et y répond dans un délai de quinze jours. Un registre des réclamations et des suites qui leur sont données est tenu à disposition de tout organisme de contrôle.`,
		doc.Str(contactMention(r.Org)))

	s.Line(`#v(14pt)`)
	s.Linef(`#lem_mono(%s)`, doc.Str(fmt.Sprintf("Fait le %s.", r.IssuedOn.Format("02/01/2006"))))
	s.Line(`#v(8pt)`)
	s.Linef(`#text(size: 8pt, %s)`, doc.Str(signatoryLine(r.Org)))

	return doc.Document{Source: s.Bytes(), CreationUnix: r.IssuedOn.Unix()}
}

// writeList rend une énumération. Typst la produirait seul, mais l'écrire ici
// garantit la même présentation que les autres documents du produit.
func writeList(s *doc.Source, items []string) {
	s.Line(`#v(4pt)`)
	for _, item := range items {
		s.Linef(`- #text(%s)`, doc.Str(item))
	}
	s.Line(`#v(4pt)`)
}

// contactMention nomme l'interlocuteur d'une réclamation. Sans représentant
// renseigné, on désigne l'organisme : une adresse vaut mieux qu'un vide sur le
// seul article qui dit à qui se plaindre.
func contactMention(org Party) string {
	if name := strings.TrimSpace(org.Represented); name != "" {
		if role := strings.TrimSpace(org.Role); role != "" {
			return fmt.Sprintf("%s, %s de %s", name, role, org.Name)
		}
		return fmt.Sprintf("%s, pour %s", name, org.Name)
	}
	return org.Name
}
