package documents

import (
	"fmt"
	"time"

	"github.com/lemlearn/api/internal/platform/doc"
)

// --- Attestation de fin de formation --------------------------------------

// Certificate porte les données d'une attestation.
//
// L'attestation est le document que l'apprenant garde ; c'est aussi celui que
// le financeur réclame en premier. Elle doit énoncer les objectifs atteints et
// les résultats obtenus, pas seulement une présence.
type Certificate struct {
	Reference string
	IssuedOn  time.Time

	Org Party

	LearnerName      string
	LearnerBirthDate string
	CourseTitle      string
	Objectives       []string

	// Modalities et Sanction reprennent ce qui figure à la convention : un
	// écart entre les deux documents est relevé en audit.
	Modalities string
	Sanction   string

	StartedOn     time.Time
	EndedOn       time.Time
	DurationHours float64
	// AttendedHours est la durée réellement suivie, reconstituée à partir des
	// relevés de connexion et des émargements. C'est elle qui est facturée au
	// financeur, pas la durée théorique.
	AttendedHours float64

	FinalScore   float64
	FinalMax     float64
	FinalPassed  bool
	ModulesTotal int
	ModulesDone  int

	SignedCity string
	Signatures []doc.AppliedSignature
}

// RenderCertificate produit l'attestation.
func RenderCertificate(c Certificate) doc.Document {
	var s doc.Source

	chrome := doc.Chrome{
		OrgName: c.Org.Name, OrgAddress: c.Org.addressLine(),
		LegalLine: legalLine(c.Org),
		Reference: c.Reference, Kind: "Attestation de fin de formation",
	}
	chrome.WritePreamble(&s)

	s.Line(`#lem_h1[Attestation de fin de formation]`)
	s.Linef(`#lem_mono(%s)`, doc.Str(fmt.Sprintf("%s · délivrée le %s", c.Reference, formatDate(c.IssuedOn))))
	s.Line(`#v(12pt)`)

	s.Linef(`Je soussigné, représentant de #text(weight: 600, %s), atteste que :`, doc.Str(c.Org.Name))
	s.Line(`#v(8pt)`)

	identity := c.LearnerName
	if c.LearnerBirthDate != "" {
		identity += fmt.Sprintf(" (né(e) le %s)", c.LearnerBirthDate)
	}
	s.Linef(`#lem_field(%s, %s)`, doc.Str("Bénéficiaire"), doc.Str(identity))
	s.Line(`#v(8pt)`)
	s.Linef(`a suivi l'action de formation #text(weight: 600, %s), qui s'est déroulée du %s au %s.`,
		doc.Str(c.CourseTitle), doc.Str(formatDate(c.StartedOn)), doc.Str(formatDate(c.EndedOn)))

	// Assiduité
	s.Line(`#lem_h2[Assiduité constatée]`)
	s.Line(`#grid(columns: (1fr, 1fr, 1fr), column-gutter: 14pt,`)
	s.Linef(`  [#lem_field(%s, %s)], [#lem_field(%s, %s)], [#lem_field(%s, %s)],`,
		doc.Str("Durée prévue"), doc.Str(formatHours(c.DurationHours)),
		doc.Str("Durée réellement suivie"), doc.Str(formatHours(c.AttendedHours)),
		doc.Str("Modules validés"), doc.Str(fmt.Sprintf("%d sur %d", c.ModulesDone, c.ModulesTotal)))
	s.Line(`)`)
	s.Line(`#v(6pt)`)
	s.Line(`#text(size: 7.5pt, fill: muted)[La durée réellement suivie est établie à partir des relevés de connexion horodatés et des feuilles d'émargement signées, joints au dossier de l'action.]`)

	// Objectifs
	if len(c.Objectives) > 0 {
		s.Line(`#lem_h2[Objectifs pédagogiques visés]`)
		s.Line(`#list(marker: [•], spacing: 4pt,`)
		for _, objective := range c.Objectives {
			s.Linef(`  [%s],`, escapeParagraph(objective))
		}
		s.Line(`)`)
	}

	// Résultats
	s.Line(`#lem_h2[Résultats de l'évaluation des acquis]`)
	if c.FinalMax > 0 {
		verdict := "Objectifs non atteints"
		if c.FinalPassed {
			verdict = "Objectifs atteints"
		}
		s.Line(`#grid(columns: (1fr, 1fr), column-gutter: 14pt,`)
		s.Linef(`  [#lem_field(%s, %s)], [#lem_field(%s, %s)],`,
			doc.Str("Évaluation finale"), doc.Str(fmt.Sprintf("%s / %s", trimFloat(c.FinalScore), trimFloat(c.FinalMax))),
			doc.Str("Appréciation"), doc.Str(verdict))
		s.Line(`)`)
	} else {
		s.Linef(`#text(size: 9pt, fill: muted, %s)`, doc.Str("Aucune évaluation finale n'était prévue pour cette action."))
	}

	s.Line(`#v(6pt)`)
	s.Linef(`#lem_field(%s, %s)`, doc.Str("Modalités"), doc.Str(orDash(c.Modalities)))
	s.Line(`#v(6pt)`)
	s.Linef(`#lem_field(%s, %s)`, doc.Str("Sanction de la formation"), doc.Str(orDash(c.Sanction)))

	s.Line(`#v(16pt)`)
	s.Linef(`#lem_mono(%s)`, doc.Str(fmt.Sprintf("Fait à %s, le %s.", c.SignedCity, formatDate(c.IssuedOn))))
	s.Line(`#v(10pt)`)
	s.Line(`#block(breakable: false, width: 100%)[`)
	s.Line(`#grid(columns: (1fr, 1fr), column-gutter: 18pt, row-gutter: 5pt,`)
	s.Linef(`  [#lem_label(%s)], [],`, doc.Str("Pour l'organisme de formation"))
	s.Linef(`  [#text(size: 8pt, %s)], [],`, doc.Str(signatoryLine(c.Org)))
	s.Line(`  [`)
	doc.WriteSignatureSlot(&s, doc.RoleOrganization, c.Signatures)
	s.Line(`  ], [],`)
	s.Line(`)`)
	s.Line(`]`)

	return doc.Document{
		Source: s.Bytes(), Assets: doc.Assets(c.Signatures),
		CreationUnix: c.IssuedOn.Unix(),
	}
}

// --- Relevé de connexion --------------------------------------------------

// WatchSession est une séance de visionnage.
type WatchSession struct {
	ModuleTitle string
	DurationMs  int64
	WatchedMs   int64
	CoveredMs   int64
	Percent     int
	FirstAt     time.Time
	LastAt      time.Time
	Sessions    int
	Rejected    int
	// Gaps sont les intervalles non vus, en millisecondes.
	Gaps [][2]int64
}

// WatchReport est le relevé de connexion d'un apprenant.
//
// C'est la pièce que demande un auditeur pour une formation à distance, et
// celle qu'aucun tableur ne peut produire de façon crédible.
type WatchReport struct {
	Reference   string
	IssuedOn    time.Time
	Org         Party
	LearnerName string
	CourseTitle string
	SessionName string
	Modules     []WatchSession
}

// RenderWatchReport produit le relevé de connexion.
func RenderWatchReport(r WatchReport) doc.Document {
	var s doc.Source

	chrome := doc.Chrome{
		OrgName: r.Org.Name, OrgAddress: r.Org.addressLine(),
		LegalLine: legalLine(r.Org),
		Reference: r.Reference, Kind: "Relevé de connexion",
	}
	chrome.WritePreamble(&s)

	s.Line(`#lem_h1[Relevé de connexion]`)
	s.Linef(`#lem_mono(%s)`, doc.Str(fmt.Sprintf("%s · édité le %s", r.Reference, formatDate(r.IssuedOn))))
	s.Line(`#v(10pt)`)

	s.Line(`#grid(columns: (1fr, 1fr), column-gutter: 14pt,`)
	s.Linef(`  [#lem_field(%s, %s)], [#lem_field(%s, %s)],`,
		doc.Str("Apprenant"), doc.Str(r.LearnerName),
		doc.Str("Formation"), doc.Str(r.CourseTitle))
	s.Line(`)`)
	if r.SessionName != "" {
		s.Line(`#v(6pt)`)
		s.Linef(`#lem_field(%s, %s)`, doc.Str("Session"), doc.Str(r.SessionName))
	}

	s.Line(`#lem_h2[Détail par module]`)
	s.Line(`#table(columns: (1fr, auto, auto, auto, auto), stroke: none, inset: (x: 0pt, y: 4pt), column-gutter: 10pt,`)
	s.Line(`  table.hline(stroke: 0.5pt + hairline),`)
	s.Linef(`  [#lem_label(%s)], [#lem_label(%s)], [#lem_label(%s)], [#lem_label(%s)], [#lem_label(%s)],`,
		doc.Str("Module"), doc.Str("Durée"), doc.Str("Suivi"), doc.Str("Couverture"), doc.Str("Séances"))
	s.Line(`  table.hline(stroke: 0.5pt + hairline),`)

	var totalCovered, totalWatched int64
	for _, module := range r.Modules {
		totalCovered += module.CoveredMs
		totalWatched += module.WatchedMs
		s.Linef(`  [#text(size: 9pt, %s)], [#lem_mono(%s)], [#lem_mono(%s)], [#lem_mono(%s)], [#lem_mono(%s)],`,
			doc.Str(module.ModuleTitle),
			doc.Str(formatDuration(module.DurationMs)),
			doc.Str(formatDuration(module.CoveredMs)),
			doc.Str(fmt.Sprintf("%d %%", module.Percent)),
			doc.Str(fmt.Sprintf("%d", module.Sessions)))
	}
	s.Line(`  table.hline(stroke: 0.5pt + hairline),`)
	s.Linef(`  [#text(size: 9pt, weight: 600, %s)], [], [#lem_mono(%s)], [], [],`,
		doc.Str("Total suivi"), doc.Str(formatDuration(totalCovered)))
	s.Line(`  table.hline(stroke: 0.5pt + hairline),`)
	s.Line(`)`)

	s.Line(`#v(8pt)`)
	s.Line(`#text(size: 7.5pt, fill: muted)[La colonne « Suivi » comptabilise la durée #emph[unique] réellement visionnée : un passage revu plusieurs fois n'est compté qu'une fois. Le temps de lecture cumulé, revisionnages inclus, s'élève à ` +
		fmt.Sprintf("%s", formatDuration(totalWatched)) + `.]`)

	// Périodes et trous, module par module.
	s.Line(`#lem_h2[Périodes de connexion]`)
	for _, module := range r.Modules {
		s.Linef(`#lem_label(%s)`, doc.Str(module.ModuleTitle))
		s.Line(`#v(2pt)`)
		if module.FirstAt.IsZero() {
			s.Linef(`#text(size: 8pt, fill: muted, %s)`, doc.Str("Aucune connexion enregistrée."))
		} else {
			s.Linef(`#lem_mono(%s)`, doc.Str(fmt.Sprintf(
				"du %s au %s · %d séance(s) · %d signal(aux) écarté(s)",
				formatDateTime(module.FirstAt), formatDateTime(module.LastAt),
				module.Sessions, module.Rejected)))
			if len(module.Gaps) > 0 {
				s.Line(`#linebreak()`)
				s.Linef(`#text(size: 7.5pt, fill: muted, %s)`, doc.Str("Intervalles non visionnés : "+formatGaps(module.Gaps)))
			}
		}
		s.Line(`#v(8pt)`)
	}

	return doc.Document{Source: s.Bytes(), CreationUnix: r.IssuedOn.Unix()}
}

// --- Relevé d'évaluation --------------------------------------------------

// QuizAnswerLine est une réponse telle qu'elle sera imprimée.
type QuizAnswerLine struct {
	Prompt      string
	Given       string
	Expected    string
	Scored      bool
	Correct     bool
	Points      float64
	MaxPoints   float64
	TimeSpentMs int64
	Changes     int
}

// QuizAttemptBlock est une passation imprimée.
type QuizAttemptBlock struct {
	Title       string
	Kind        string
	Version     int
	Number      int
	SubmittedAt time.Time
	DurationMs  int64
	Score       float64
	MaxScore    float64
	Percent     int
	Passed      bool
	Answers     []QuizAnswerLine
}

// QuizReport est le relevé d'évaluation d'un apprenant.
type QuizReport struct {
	Reference   string
	IssuedOn    time.Time
	Org         Party
	LearnerName string
	CourseTitle string
	Attempts    []QuizAttemptBlock
}

// RenderQuizReport produit le relevé d'évaluation.
//
// Chaque passation est réimprimée avec les questions de *sa* version : c'est
// tout l'intérêt du versionnement, et ce qu'un auditeur vient vérifier.
func RenderQuizReport(r QuizReport) doc.Document {
	var s doc.Source

	chrome := doc.Chrome{
		OrgName: r.Org.Name, OrgAddress: r.Org.addressLine(),
		LegalLine: legalLine(r.Org),
		Reference: r.Reference, Kind: "Relevé d'évaluation",
	}
	chrome.WritePreamble(&s)

	s.Line(`#lem_h1[Relevé d'évaluation des acquis]`)
	s.Linef(`#lem_mono(%s)`, doc.Str(fmt.Sprintf("%s · édité le %s", r.Reference, formatDate(r.IssuedOn))))
	s.Line(`#v(10pt)`)
	s.Line(`#grid(columns: (1fr, 1fr), column-gutter: 14pt,`)
	s.Linef(`  [#lem_field(%s, %s)], [#lem_field(%s, %s)],`,
		doc.Str("Apprenant"), doc.Str(r.LearnerName),
		doc.Str("Formation"), doc.Str(r.CourseTitle))
	s.Line(`)`)

	if len(r.Attempts) == 0 {
		s.Line(`#v(12pt)`)
		s.Linef(`#text(size: 9pt, fill: muted, %s)`, doc.Str("Aucune évaluation enregistrée à ce jour."))
		return doc.Document{Source: s.Bytes(), CreationUnix: r.IssuedOn.Unix()}
	}

	for _, attempt := range r.Attempts {
		s.Linef(`#lem_h2[%s]`, escapeParagraph(attempt.Title))
		s.Linef(`#lem_mono(%s)`, doc.Str(fmt.Sprintf(
			"%s · version %d · tentative %d · %s · durée %s",
			attempt.Kind, attempt.Version, attempt.Number,
			formatDateTime(attempt.SubmittedAt), formatDuration(attempt.DurationMs))))
		s.Line(`#v(6pt)`)

		if attempt.MaxScore > 0 {
			verdict := "non atteint"
			if attempt.Passed {
				verdict = "atteint"
			}
			s.Linef(`#lem_field(%s, %s)`, doc.Str("Résultat"),
				doc.Str(fmt.Sprintf("%s / %s — %d %% — seuil %s",
					trimFloat(attempt.Score), trimFloat(attempt.MaxScore), attempt.Percent, verdict)))
			s.Line(`#v(6pt)`)
		}

		s.Line(`#table(columns: (1fr, auto, auto, auto), stroke: none, inset: (x: 0pt, y: 3pt), column-gutter: 10pt,`)
		s.Line(`  table.hline(stroke: 0.5pt + hairline),`)
		s.Linef(`  [#lem_label(%s)], [#lem_label(%s)], [#lem_label(%s)], [#lem_label(%s)],`,
			doc.Str("Question et réponse"), doc.Str("Points"), doc.Str("Temps"), doc.Str("Reprises"))
		s.Line(`  table.hline(stroke: 0.5pt + hairline),`)

		for _, answer := range attempt.Answers {
			mark := ""
			if answer.Scored {
				mark = " ✗"
				if answer.Correct {
					mark = " ✓"
				}
			}
			cell := answer.Prompt + "\n" + "Réponse : " + orDash(answer.Given)
			if answer.Scored && !answer.Correct && answer.Expected != "" {
				cell += "\nAttendu : " + answer.Expected
			}
			points := "—"
			if answer.MaxPoints > 0 {
				points = fmt.Sprintf("%s / %s%s", trimFloat(answer.Points), trimFloat(answer.MaxPoints), mark)
			}
			s.Linef(`  [#text(size: 8.5pt, %s)], [#lem_mono(%s)], [#lem_mono(%s)], [#lem_mono(%s)],`,
				doc.Str(cell), doc.Str(points),
				doc.Str(formatDuration(answer.TimeSpentMs)),
				doc.Str(fmt.Sprintf("%d", answer.Changes)))
			s.Line(`  table.hline(stroke: 0.25pt + hairline),`)
		}
		s.Line(`)`)
		s.Line(`#v(4pt)`)
		s.Line(`#text(size: 7pt, fill: faint)[« Reprises » indique le nombre de fois où l'apprenant a modifié sa réponse avant de valider.]`)
	}

	return doc.Document{Source: s.Bytes(), CreationUnix: r.IssuedOn.Unix()}
}

// --- Mise en forme partagée ------------------------------------------------

// formatDuration rend une durée en heures et minutes, ou en minutes et
// secondes en deçà de l'heure.
func formatDuration(ms int64) string {
	if ms <= 0 {
		return "—"
	}
	total := ms / 1000
	hours, minutes, seconds := total/3600, (total%3600)/60, total%60
	if hours > 0 {
		return fmt.Sprintf("%d h %02d", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d min %02d s", minutes, seconds)
	}
	return fmt.Sprintf("%d s", seconds)
}

// formatDateTime rend un horodatage à la minute, heure de Paris.
func formatDateTime(at time.Time) string {
	if at.IsZero() {
		return "—"
	}
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		loc = time.UTC
	}
	local := at.In(loc)
	return fmt.Sprintf("%02d/%02d/%d à %02d:%02d",
		local.Day(), int(local.Month()), local.Year(), local.Hour(), local.Minute())
}

// formatGaps rend les intervalles non visionnés sous forme lisible.
func formatGaps(gaps [][2]int64) string {
	const maxShown = 6
	out := ""
	for i, gap := range gaps {
		if i == maxShown {
			out += fmt.Sprintf(" et %d autre(s)", len(gaps)-maxShown)
			break
		}
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%s–%s", formatClock(gap[0]), formatClock(gap[1]))
	}
	return out
}

// formatClock rend une position dans une vidéo, mm:ss.
func formatClock(ms int64) string {
	total := ms / 1000
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}
