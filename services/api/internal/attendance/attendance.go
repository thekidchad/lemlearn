// Package attendance porte l'émargement numérique.
//
// L'émargement est la pièce qui justifie les heures facturées au financeur.
// Il porte toujours sur un *créneau* — jamais sur une formation, jamais sur un
// module : c'est la présence à un moment donné qui est attestée, et c'est ce
// qu'un contrôleur recompte.
//
// Trois modalités, trois façons de l'établir :
//   - présentiel : une signature par demi-journée, sur place ;
//   - classe virtuelle : une signature par créneau, depuis l'espace apprenant ;
//   - asynchrone : le créneau correspond à un module, et la présence est
//     établie par le relevé de connexion puis contresignée par le formateur.
//     C'est la seule modalité où l'apprenant ne signe pas lui-même — et c'est
//     pourquoi la contresignature du formateur y est obligatoire.
package attendance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// Slot est un créneau émargeable.
type Slot struct {
	ID    string    `dynamodbav:"id" json:"id"`
	Label string    `dynamodbav:"label" json:"label"`
	Start time.Time `dynamodbav:"start" json:"start"`
	End   time.Time `dynamodbav:"end" json:"end"`
	// ModuleID relie un créneau asynchrone à son module.
	ModuleID string `dynamodbav:"moduleId,omitempty" json:"moduleId,omitempty"`
}

// Hours est la durée du créneau en heures.
func (s Slot) Hours() float64 { return s.End.Sub(s.Start).Hours() }

// Sheet est la feuille d'émargement d'une session.
type Sheet struct {
	ddb.Record

	OrgID     string       `dynamodbav:"orgId" json:"orgId"`
	SessionID string       `dynamodbav:"sessionId" json:"sessionId"`
	Mode      catalog.Mode `dynamodbav:"mode" json:"mode"`
	Slots     []Slot       `dynamodbav:"slots" json:"slots"`

	// TrainerSignedAt et TrainerID portent la contresignature du formateur.
	// Sans elle, une feuille d'émargement n'atteste que des déclarations.
	TrainerID       string     `dynamodbav:"trainerId,omitempty" json:"trainerId,omitempty"`
	TrainerName     string     `dynamodbav:"trainerName,omitempty" json:"trainerName,omitempty"`
	TrainerSignedAt *time.Time `dynamodbav:"trainerSignedAt,omitempty" json:"trainerSignedAt,omitempty"`
}

// SheetSK est la clé de tri d'une feuille.
func SheetSK(sessionID string) string { return "SESSION#" + sessionID + "#SHEET" }

// Method dit comment une présence a été établie.
type Method string

const (
	// MethodSignature : l'apprenant a signé lui-même.
	MethodSignature Method = "signature"
	// MethodConnection : la présence est établie par le relevé de connexion,
	// puis contresignée par le formateur.
	MethodConnection Method = "connection"
	// MethodAbsent : absence constatée. Une absence consignée vaut mieux
	// qu'une case vide, qu'un contrôleur interprétera contre l'organisme.
	MethodAbsent Method = "absent"
)

// Entry est la présence d'un apprenant à un créneau.
type Entry struct {
	ddb.Record

	OrgID     string `dynamodbav:"orgId" json:"orgId"`
	SessionID string `dynamodbav:"sessionId" json:"sessionId"`
	SlotID    string `dynamodbav:"slotId" json:"slotId"`
	ContactID string `dynamodbav:"contactId" json:"contactId"`

	Method   Method    `dynamodbav:"method" json:"method"`
	SignedAt time.Time `dynamodbav:"signedAt" json:"signedAt"`

	// Preuve de l'acte, au même titre qu'une signature de convention.
	IP        string `dynamodbav:"ip,omitempty" json:"ip,omitempty"`
	UserAgent string `dynamodbav:"userAgent,omitempty" json:"userAgent,omitempty"`
	// DrawingKey est le tracé manuscrit, quand il y en a un.
	DrawingKey string `dynamodbav:"drawingKey,omitempty" json:"-"`
	// CoveragePercent justifie une présence établie par connexion.
	CoveragePercent int `dynamodbav:"coveragePercent,omitempty" json:"coveragePercent,omitempty"`
	// Comment porte le motif d'une absence.
	Comment string `dynamodbav:"comment,omitempty" json:"comment,omitempty"`
}

// EntrySK est la clé de tri d'une présence.
func EntrySK(sessionID, slotID, contactID string) string {
	return "SESSION#" + sessionID + "#SLOT#" + slotID + "#ATT#" + contactID
}

// Service porte les cas d'usage de l'émargement.
type Service struct {
	db      *ddb.Client
	catalog *catalog.Service
	now     func() time.Time
}

// NewService construit le service.
func NewService(db *ddb.Client, cat *catalog.Service, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{db: db, catalog: cat, now: now}
}

// halfDayCut est l'heure qui sépare matin et après-midi.
const halfDayCut = 13

// EnsureSheet construit la feuille d'une session, ou la relit.
//
// Les créneaux sont dérivés de la session plutôt que saisis : une feuille dont
// les créneaux ne correspondent pas à ce qui a été conventionné ne prouve rien.
func (s *Service) EnsureSheet(ctx context.Context, orgID, sessionID string) (Sheet, error) {
	if sheet, err := ddb.Get[Sheet](ctx, s.db, ddb.OrgPK(orgID), SheetSK(sessionID)); err == nil {
		return sheet, nil
	}

	session, err := s.catalog.GetSession(ctx, orgID, sessionID)
	if err != nil {
		return Sheet{}, fmt.Errorf("session introuvable: %w", err)
	}

	now := s.now()
	sheet := Sheet{
		Record: ddb.Record{
			PK: ddb.OrgPK(orgID), SK: SheetSK(sessionID), Type: "attendance_sheet",
			CreatedAt: now, UpdatedAt: now,
		},
		OrgID: orgID, SessionID: sessionID, Mode: session.Mode,
		TrainerID: session.TrainerID,
	}

	if session.Mode == catalog.ModeAsync {
		modules, err := s.catalog.ListModules(ctx, orgID, session.CourseID)
		if err != nil {
			return Sheet{}, err
		}
		sheet.Slots = asyncSlots(session, modules)
	} else {
		sheet.Slots = halfDaySlots(session)
	}

	if err := ddb.Put(ctx, s.db, sheet); err != nil {
		return Sheet{}, err
	}
	return sheet, nil
}

// halfDaySlots découpe une session synchrone en demi-journées.
func halfDaySlots(session catalog.Session) []Slot {
	var slots []Slot
	loc := parisOrUTC()

	for day := session.StartsAt.In(loc); !day.After(session.EndsAt.In(loc)); day = day.AddDate(0, 0, 1) {
		date := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
		morning := Slot{
			ID:    fmt.Sprintf("%s-am", date.Format("2006-01-02")),
			Label: fmt.Sprintf("%s — matin", date.Format("02/01/2006")),
			Start: date.Add(9 * time.Hour), End: date.Add(halfDayCut * time.Hour),
		}
		afternoon := Slot{
			ID:    fmt.Sprintf("%s-pm", date.Format("2006-01-02")),
			Label: fmt.Sprintf("%s — après-midi", date.Format("02/01/2006")),
			Start: date.Add(14 * time.Hour), End: date.Add(17 * time.Hour),
		}
		for _, slot := range []Slot{morning, afternoon} {
			// Un créneau entièrement hors de la session n'a pas lieu d'être
			// émargé : une feuille pleine de cases sans objet fait douter de
			// celles qui en ont.
			if slot.End.After(session.StartsAt) && slot.Start.Before(session.EndsAt) {
				slots = append(slots, slot)
			}
		}
	}
	return slots
}

// asyncSlots crée un créneau par module.
func asyncSlots(session catalog.Session, modules []catalog.Module) []Slot {
	slots := make([]Slot, 0, len(modules))
	for _, module := range modules {
		slots = append(slots, Slot{
			ID:       "mod-" + module.ID,
			Label:    fmt.Sprintf("Module %d — %s", module.Position, module.Title),
			Start:    session.StartsAt,
			End:      session.EndsAt,
			ModuleID: module.ID,
		})
	}
	return slots
}

// SignInput décrit un émargement.
type SignInput struct {
	OrgID     string
	SessionID string
	SlotID    string
	ContactID string
	FileID    string
	Method    Method
	// Coverage justifie une présence établie par connexion.
	Coverage   int
	Comment    string
	DrawingKey string
	IP         string
	UserAgent  string
	Actor      audit.Actor
}

// Sign enregistre une présence.
//
// L'écriture est conditionnée à l'absence d'entrée préexistante : une présence
// déjà signée ne se réécrit pas. Corriger une erreur passe par une absence
// motivée, qui laisse trace des deux états.
func (s *Service) Sign(ctx context.Context, in SignInput) (Entry, error) {
	sheet, err := s.EnsureSheet(ctx, in.OrgID, in.SessionID)
	if err != nil {
		return Entry{}, err
	}

	slot, ok := findSlot(sheet.Slots, in.SlotID)
	if !ok {
		return Entry{}, fmt.Errorf("créneau %q inconnu pour cette session", in.SlotID)
	}
	if in.Method == "" {
		return Entry{}, fmt.Errorf("le mode d'établissement de la présence est obligatoire")
	}
	if in.Method == MethodAbsent && strings.TrimSpace(in.Comment) == "" {
		// Une absence sans motif est une case vide déguisée.
		return Entry{}, fmt.Errorf("une absence doit être motivée")
	}

	now := s.now()
	entry := Entry{
		Record: ddb.Record{
			PK: ddb.OrgPK(in.OrgID), SK: EntrySK(in.SessionID, in.SlotID, in.ContactID),
			GSI1PK:    ddb.OrgPK(in.OrgID) + "#ATTSESSION#" + in.SessionID,
			GSI1SK:    in.SlotID + "#" + in.ContactID,
			Type:      "attendance",
			CreatedAt: now, UpdatedAt: now,
		},
		OrgID: in.OrgID, SessionID: in.SessionID, SlotID: in.SlotID, ContactID: in.ContactID,
		Method: in.Method, SignedAt: now,
		IP: in.IP, UserAgent: in.UserAgent, DrawingKey: in.DrawingKey,
		CoveragePercent: in.Coverage, Comment: in.Comment,
	}

	subject := "enrollment/" + in.SessionID + ":" + in.ContactID
	if in.FileID != "" {
		subject = "file/" + in.FileID
	}

	if _, err := s.db.WriteWithAudit(ctx, subject,
		[]ddb.Write{{Item: entry, Condition: "attribute_not_exists(SK)"}},
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, subject, now, audit.ActionAttendanceSigned, in.Actor,
				map[string]any{
					"sessionId": in.SessionID,
					"slot":      slot.Label,
					"hours":     slot.Hours(),
					"method":    string(in.Method),
					"coverage":  in.Coverage,
					"comment":   in.Comment,
				})
		}); err != nil {
		if errors.Is(err, ddb.ErrConflict) {
			return Entry{}, fmt.Errorf("ce créneau a déjà été émargé pour cet apprenant")
		}
		return Entry{}, err
	}

	return entry, nil
}

// Countersign appose la contresignature du formateur.
//
// Elle clôt la feuille : c'est elle qui transforme un relevé de déclarations
// en attestation de présence, et sans elle une feuille asynchrone n'a aucune
// valeur devant un financeur.
func (s *Service) Countersign(ctx context.Context, orgID, sessionID string, trainer identity.User, actor audit.Actor) (Sheet, error) {
	sheet, err := s.EnsureSheet(ctx, orgID, sessionID)
	if err != nil {
		return Sheet{}, err
	}
	if sheet.TrainerSignedAt != nil {
		return sheet, fmt.Errorf("cette feuille a déjà été contresignée")
	}

	now := s.now()
	sheet.TrainerID = trainer.ID
	sheet.TrainerName = trainer.FullName()
	sheet.TrainerSignedAt = &now
	sheet.UpdatedAt = now

	subject := "session/" + sessionID
	if _, err := s.db.WriteWithAudit(ctx, subject, []ddb.Write{{Item: sheet}},
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, subject, now, audit.ActionAttendanceSigned, actor,
				map[string]any{
					"event":     "countersignature",
					"sessionId": sessionID,
					"trainer":   trainer.FullName(),
					"slots":     len(sheet.Slots),
				})
		}); err != nil {
		return Sheet{}, err
	}
	return sheet, nil
}

// Entries renvoie toutes les présences d'une session.
func (s *Service) Entries(ctx context.Context, orgID, sessionID string) ([]Entry, error) {
	return ddb.Query[Entry](ctx, s.db, ddb.QuerySpec{
		Index: "GSI1", PK: ddb.OrgPK(orgID) + "#ATTSESSION#" + sessionID,
	})
}

// AttendedHours totalise les heures effectivement émargées par un apprenant.
//
// C'est le nombre facturable au financeur : une absence, même motivée, ne se
// facture pas.
func AttendedHours(sheet Sheet, entries []Entry, contactID string) float64 {
	var hours float64
	for _, entry := range entries {
		if entry.ContactID != contactID || entry.Method == MethodAbsent {
			continue
		}
		if slot, ok := findSlot(sheet.Slots, entry.SlotID); ok {
			hours += slot.Hours()
		}
	}
	return hours
}

func findSlot(slots []Slot, id string) (Slot, bool) {
	for _, slot := range slots {
		if slot.ID == id {
			return slot, true
		}
	}
	return Slot{}, false
}

func parisOrUTC() *time.Location {
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		return time.UTC
	}
	return loc
}
