// Package export assemble le dossier probatoire d'un apprenant.
//
// C'est l'aboutissement du produit : une archive que l'on remet telle quelle à
// un auditeur ou à un financeur, et qui se relit sans nous. Elle contient les
// pièces originales, un manifeste d'empreintes pour prouver qu'elles n'ont pas
// bougé, et le journal d'audit vérifié du dossier.
//
// Ce que l'archive ne contient pas est aussi important : les pièces manquantes
// sont listées explicitement plutôt que passées sous silence. Un dossier
// incomplet dont on connaît les trous vaut mieux qu'un dossier qui prétend
// être complet.
package export

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/documents"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/learning"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/doc"
	"github.com/lemlearn/api/internal/quiz"
	"github.com/lemlearn/api/internal/signature"
)

// Service construit les archives.
type Service struct {
	identity  *identity.Service
	crm       *crm.Service
	catalog   *catalog.Service
	learning  *learning.Service
	signature *signature.Service
	compiler  doc.Compiler
	now       func() time.Time
}

// Deps regroupe les dépendances.
type Deps struct {
	Identity  *identity.Service
	CRM       *crm.Service
	Catalog   *catalog.Service
	Learning  *learning.Service
	Signature *signature.Service
	Compiler  doc.Compiler
	Now       func() time.Time
}

// NewService construit le service d'export.
func NewService(deps Deps) *Service {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		identity: deps.Identity, crm: deps.CRM, catalog: deps.Catalog,
		learning: deps.Learning, signature: deps.Signature,
		compiler: deps.Compiler, now: now,
	}
}

// entry est une pièce de l'archive.
type entry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
	// Origin dit d'où vient la pièce : « archivée » pour un document scellé
	// relu tel quel, « générée » pour un relevé produit à l'export. La
	// distinction compte : une pièce archivée porte sa propre signature, une
	// pièce générée n'engage que la fidélité de nos données.
	Origin string `json:"origin"`
}

// manifest décrit l'archive.
type Manifest struct {
	Reference    string    `json:"reference"`
	Organization string    `json:"organization"`
	Learner      string    `json:"learner"`
	GeneratedAt  time.Time `json:"generatedAt"`
	Entries      []entry   `json:"entries"`
	// Missing énumère les pièces attendues et absentes.
	Missing []string `json:"missing"`
	// AuditVerified dit si la chaîne du dossier a passé la vérification. Une
	// archive dont la chaîne est rompue n'est pas produite du tout : voir
	// Build.
	AuditVerified bool `json:"auditVerified"`
	AuditEvents   int  `json:"auditEvents"`
}

// Build assemble l'archive d'un dossier.
func (s *Service) Build(ctx context.Context, orgID, fileID string) ([]byte, Manifest, error) {
	org, err := s.identity.LoadOrg(ctx, orgID)
	if err != nil {
		return nil, Manifest{}, fmt.Errorf("organisation: %w", err)
	}
	file, err := s.crm.GetFile(ctx, orgID, fileID)
	if err != nil {
		return nil, Manifest{}, fmt.Errorf("dossier: %w", err)
	}

	// Le journal est vérifié d'abord. Une chaîne rompue interrompt l'export :
	// livrer un dossier dont on sait que le journal a été altéré serait pire
	// que de ne rien livrer.
	events, err := s.crm.Timeline(ctx, fileID)
	if err != nil {
		return nil, Manifest{}, fmt.Errorf("journal d'audit: %w", err)
	}

	var learner crm.Contact
	if file.LearnerID != "" {
		learner, _ = s.crm.GetContact(ctx, orgID, file.LearnerID)
	}

	book := Manifest{
		Reference:     file.Reference,
		Organization:  org.Name,
		Learner:       learner.DisplayName(),
		GeneratedAt:   s.now(),
		AuditVerified: true,
		AuditEvents:   len(events),
	}

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)

	add := func(path, origin string, content []byte) error {
		writer, err := archive.Create(path)
		if err != nil {
			return err
		}
		if _, err := writer.Write(content); err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		book.Entries = append(book.Entries, entry{
			Path: path, SHA256: hex.EncodeToString(sum[:]),
			Bytes: len(content), Origin: origin,
		})
		return nil
	}

	// 1. La fiche du dossier, en JSON : c'est ce qui permet à un tiers de
	//    reprendre les données sans nous, et c'est notre réponse à
	//    l'exigence de portabilité du RGPD.
	record, err := json.MarshalIndent(map[string]any{
		"dossier":      file,
		"apprenant":    learner,
		"organisation": map[string]any{"nom": org.Name, "siret": org.SIRET, "nda": org.NDA},
	}, "", "  ")
	if err != nil {
		return nil, Manifest{}, err
	}
	if err := add("dossier.json", "générée", record); err != nil {
		return nil, Manifest{}, err
	}

	// 2. Les documents signés, relus depuis l'archivage et contrôlés.
	if err := s.addSignedDocuments(ctx, orgID, fileID, add, &book); err != nil {
		return nil, Manifest{}, err
	}

	// 3. Les relevés, générés à l'export.
	if err := s.addReports(ctx, org, file, learner, add, &book); err != nil {
		return nil, Manifest{}, err
	}

	// 4. Le journal d'audit, en CSV lisible par un tableur.
	trail, err := auditCSV(events)
	if err != nil {
		return nil, Manifest{}, err
	}
	if err := add("journal-audit.csv", "générée", trail); err != nil {
		return nil, Manifest{}, err
	}

	// 5. Le manifeste, en dernier : il décrit tout ce qui précède.
	book.Missing = dedupe(append(book.Missing, reconcile(book.Entries, events, learner)...))
	bookJSON, err := json.MarshalIndent(book, "", "  ")
	if err != nil {
		return nil, Manifest{}, err
	}
	writer, err := archive.Create("manifeste.json")
	if err != nil {
		return nil, Manifest{}, err
	}
	if _, err := writer.Write(bookJSON); err != nil {
		return nil, Manifest{}, err
	}

	if err := archive.Close(); err != nil {
		return nil, Manifest{}, err
	}
	return buffer.Bytes(), book, nil
}

// addSignedDocuments joint les documents scellés du dossier.
func (s *Service) addSignedDocuments(
	ctx context.Context, orgID, fileID string,
	add func(string, string, []byte) error, book *Manifest,
) error {
	if s.signature == nil {
		return nil
	}
	requests, err := s.signature.ListForFile(ctx, orgID, fileID)
	if err != nil {
		return fmt.Errorf("demandes de signature: %w", err)
	}

	for _, request := range requests {
		if request.Status != signature.StatusSigned || request.Proof == nil {
			book.Missing = append(book.Missing,
				fmt.Sprintf("%s — non signé (%s)", request.Reference, request.Status))
			continue
		}
		sealed, err := s.signature.Sealed(ctx, request)
		if err != nil {
			// L'intégrité rompue est signalée dans le manifeste, mais la
			// pièce n'est pas jointe : mieux vaut un trou documenté qu'un
			// document dont on sait qu'il a changé.
			book.Missing = append(book.Missing,
				fmt.Sprintf("%s — intégrité non vérifiée : %v", request.Reference, err))
			continue
		}
		if err := add("documents/"+request.Reference+".pdf", "archivée", sealed); err != nil {
			return err
		}
	}
	return nil
}

// addReports génère les relevés de connexion et d'évaluation.
func (s *Service) addReports(
	ctx context.Context, org identity.Org, file crm.File, learner crm.Contact,
	add func(string, string, []byte) error, book *Manifest,
) error {
	if s.compiler == nil || s.catalog == nil || s.learning == nil {
		book.Missing = append(book.Missing, "Relevés de connexion et d'évaluation — génération indisponible")
		return nil
	}
	if file.SessionID == "" || file.LearnerID == "" {
		book.Missing = append(book.Missing, "Relevés — le dossier n'est rattaché à aucune session")
		return nil
	}

	enrollment, err := s.catalog.GetEnrollment(ctx, org.ID, file.SessionID, file.LearnerID)
	if err != nil {
		book.Missing = append(book.Missing, "Relevés — inscription introuvable")
		return nil
	}
	trainingSession, err := s.catalog.GetSession(ctx, org.ID, file.SessionID)
	if err != nil {
		return nil
	}
	course, err := s.catalog.GetCourse(ctx, org.ID, trainingSession.CourseID)
	if err != nil {
		return nil
	}
	modules, err := s.catalog.ListModules(ctx, org.ID, course.ID)
	if err != nil {
		return nil
	}

	party := documents.Party{
		Name: org.Name, Address: org.Address,
		PostalCode: org.PostalCode, City: org.City, SIRET: org.SIRET,
	}
	now := s.now()

	// Relevé de connexion.
	report := documents.WatchReport{
		Reference:   file.Reference + "-CNX",
		IssuedOn:    now,
		Org:         party,
		LearnerName: learner.DisplayName(),
		CourseTitle: course.Title,
		SessionName: trainingSession.Title,
	}
	for _, module := range modules {
		if module.DurationMs == 0 {
			continue
		}
		coverage, err := s.learning.Coverage(ctx, learning.Target{
			OrgID: org.ID, SessionID: file.SessionID,
			ContactID: file.LearnerID, ModuleID: module.ID,
		})
		if err != nil {
			coverage.DurationMs = module.DurationMs
		}
		report.Modules = append(report.Modules, documents.WatchSession{
			ModuleTitle: module.Title,
			DurationMs:  module.DurationMs,
			WatchedMs:   coverage.WatchedMs,
			CoveredMs:   coverage.CoveredMs(),
			Percent:     coverage.Percent(),
			FirstAt:     coverage.FirstAt,
			LastAt:      coverage.LastAt,
			Sessions:    coverage.Sessions,
			Rejected:    coverage.Rejected,
			Gaps:        coverage.Gaps(),
		})
	}
	if len(report.Modules) > 0 {
		pdf, err := s.compiler.Compile(ctx, documents.RenderWatchReport(report))
		if err != nil {
			return fmt.Errorf("relevé de connexion: %w", err)
		}
		if err := add("releves/releve-de-connexion.pdf", "générée", pdf); err != nil {
			return err
		}
	}

	// Relevé d'évaluation.
	attempts, err := s.learning.AllAttempts(ctx, org.ID, file.SessionID+":"+file.LearnerID)
	if err != nil {
		return fmt.Errorf("passations: %w", err)
	}
	evaluation := documents.QuizReport{
		Reference: file.Reference + "-EVA", IssuedOn: now, Org: party,
		LearnerName: learner.DisplayName(), CourseTitle: course.Title,
	}
	for _, attempt := range attempts {
		version, err := s.learning.Version(ctx, org.ID, attempt.QuizID, attempt.Version)
		if err != nil {
			// Sans la version passée, la copie ne peut pas être réimprimée
			// fidèlement : on le dit plutôt que d'imprimer autre chose.
			book.Missing = append(book.Missing,
				fmt.Sprintf("Copie du questionnaire %s v%d — version introuvable", attempt.QuizID, attempt.Version))
			continue
		}
		evaluation.Attempts = append(evaluation.Attempts, attemptBlock(version, attempt))
	}
	pdf, err := s.compiler.Compile(ctx, documents.RenderQuizReport(evaluation))
	if err != nil {
		return fmt.Errorf("relevé d'évaluation: %w", err)
	}
	if err := add("releves/releve-evaluation.pdf", "générée", pdf); err != nil {
		return err
	}

	// Attestation, si et seulement si elle est délivrable.
	if err := enrollment.Certifiable(modules, course); err != nil {
		book.Missing = append(book.Missing, "Attestation de fin de formation — "+err.Error())
		return nil
	}

	var attendedMs int64
	for _, module := range report.Modules {
		attendedMs += module.CoveredMs
	}
	certificate := documents.Certificate{
		Reference: file.Reference + "-ATT", IssuedOn: now, Org: party,
		LearnerName: learner.DisplayName(), LearnerBirthDate: learner.BirthDate,
		CourseTitle: course.Title, Objectives: course.Objectives,
		Modalities: course.Assessment, Sanction: course.Sanction,
		StartedOn: trainingSession.StartsAt, EndedOn: trainingSession.EndsAt,
		DurationHours: course.DurationHours,
		AttendedHours: float64(attendedMs) / 3_600_000,
		FinalPassed:   enrollment.FinalPassed,
		ModulesTotal:  len(modules), ModulesDone: len(modules) * enrollment.CompletionPercent(modules) / 100,
		SignedCity: org.City,
	}
	attestation, err := s.compiler.Compile(ctx, documents.RenderCertificate(certificate))
	if err != nil {
		return fmt.Errorf("attestation: %w", err)
	}
	return add("attestation.pdf", "générée", attestation)
}

// attemptBlock met une passation en forme imprimable.
func attemptBlock(version quiz.Questionnaire, attempt quiz.Attempt) documents.QuizAttemptBlock {
	block := documents.QuizAttemptBlock{
		Title: version.Title, Kind: string(version.Kind),
		Version: attempt.Version, Number: attempt.Number,
		SubmittedAt: attempt.SubmittedAt, DurationMs: attempt.DurationMs,
		Score: attempt.Score, MaxScore: attempt.MaxScore,
		Percent: attempt.Percent(), Passed: attempt.Passed,
	}

	labels := map[string]map[string]string{}
	prompts := map[string]quiz.Question{}
	for _, question := range version.Questions {
		prompts[question.ID] = question
		options := map[string]string{}
		for _, option := range question.Options {
			options[option.ID] = option.Label
		}
		labels[question.ID] = options
	}

	for _, answer := range attempt.Answers {
		question := prompts[answer.QuestionID]
		block.Answers = append(block.Answers, documents.QuizAnswerLine{
			Prompt:      question.Prompt,
			Given:       joinLabels(labels[answer.QuestionID], answer.Values),
			Expected:    joinLabels(labels[answer.QuestionID], question.Correct),
			Scored:      answer.Scored,
			Correct:     answer.IsCorrect,
			Points:      answer.Points,
			MaxPoints:   answer.MaxPoints,
			TimeSpentMs: answer.TimeSpentMs,
			Changes:     answer.ChangeCount,
		})
	}
	return block
}

// joinLabels traduit des identifiants d'option en libellés lisibles.
//
// Imprimer « a, b » dans un relevé destiné à un auditeur ne prouve rien : il
// faut les intitulés tels que l'apprenant les a vus.
func joinLabels(labels map[string]string, values []string) string {
	out := ""
	for i, value := range values {
		if i > 0 {
			out += " ; "
		}
		if label, ok := labels[value]; ok && label != "" {
			out += label
		} else {
			out += value
		}
	}
	return out
}

// auditCSV rend le journal en CSV.
func auditCSV(events []audit.Event) ([]byte, error) {
	var buffer bytes.Buffer
	// BOM UTF-8 : sans lui, Excel massacre les accents d'un CSV, et le
	// destinataire de ce fichier l'ouvrira très probablement dans Excel.
	buffer.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(&buffer)
	writer.Comma = ';'

	if err := writer.Write([]string{
		"rang", "horodatage", "action", "acteur", "identifiant", "ip",
		"pour le compte de", "détail", "empreinte", "empreinte précédente",
	}); err != nil {
		return nil, err
	}

	for _, event := range events {
		detail, err := json.Marshal(event.Payload)
		if err != nil {
			return nil, err
		}
		if err := writer.Write([]string{
			strconv.FormatInt(event.Seq, 10),
			event.At.UTC().Format(time.RFC3339),
			string(event.Action),
			event.Actor.Label,
			event.Actor.ID,
			event.Actor.IP,
			event.Actor.OnBehalfOf,
			string(detail),
			event.Hash,
			event.PrevHash,
		}); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buffer.Bytes(), writer.Error()
}

// dedupe écarte les doublons en conservant l'ordre.
//
// Une même pièce peut être signalée deux fois : une fois avec son motif précis
// pendant l'assemblage — « attestation : le module X n'est pas validé » — et
// une fois par la réconciliation finale. La version motivée arrive en premier,
// c'est donc elle qui est conservée.
func dedupe(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		// Le rapprochement se fait sur le début du libellé, avant le tiret
		// qui introduit le motif.
		key := item
		if head, _, found := strings.Cut(item, " — "); found {
			key = head
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

// reconcile confronte les pièces attendues au contenu réellement assemblé.
//
// La liste statique portée par le dossier ne suffit pas : elle est posée à la
// création et ne sait pas ce que l'archive contient. Un manifeste qui
// déclarerait manquante une pièce présente ferait douter de tout le reste —
// c'est précisément la confiance que ce fichier doit établir.
func reconcile(entries []entry, events []audit.Event, learner crm.Contact) []string {
	present := make(map[string]bool, len(entries))
	hasSignedDocument := false
	for _, e := range entries {
		present[e.Path] = true
		if strings.HasPrefix(e.Path, "documents/") {
			hasSignedDocument = true
		}
	}

	seen := make(map[audit.Action]bool, len(events))
	for _, event := range events {
		seen[event.Action] = true
	}

	checks := []struct {
		piece  string
		fulfil bool
	}{
		{"Pièce d'identité de l'apprenant", learner.IdentityDocKey != ""},
		{"Consentement RGPD", seen[audit.ActionConsentGiven]},
		{"Convention signée", hasSignedDocument},
		{"Relevé de connexion", present["releves/releve-de-connexion.pdf"]},
		{"Relevé d'évaluation", present["releves/releve-evaluation.pdf"]},
		{"Feuilles d'émargement", seen[audit.ActionAttendanceSigned]},
		{"Attestation de fin de formation", present["attestation.pdf"]},
	}

	var missing []string
	for _, check := range checks {
		if !check.fulfil {
			missing = append(missing, check.piece)
		}
	}
	return missing
}
