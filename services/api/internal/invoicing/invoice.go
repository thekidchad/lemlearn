package invoicing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/audit"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// La facturation.
//
// Une facture n'est pas un document comme les autres : la loi lui impose des
// règles que le produit doit tenir, sans quoi il vaut mieux ne pas en émettre
// du tout.
//
//   - La numérotation est continue et sans trou (art. 242 nonies A de l'annexe
//     II au CGI). Le numéro est donc attribué à l'émission et non à la
//     création : un brouillon abandonné ne doit pas laisser de vide.
//   - Une facture émise ne se modifie plus. Une erreur se corrige par un avoir
//     qui la référence, jamais par une réécriture — c'est la différence entre
//     une comptabilité et un tableur.
//   - L'identité du client est figée à l'émission. Un client qui déménage
//     ensuite ne doit pas changer rétroactivement une facture déjà envoyée.

// Statut d'une facture.
type Statut string

const (
	// StatutBrouillon : modifiable, sans numéro, invisible du client.
	StatutBrouillon Statut = "brouillon"
	// StatutEmise : numérotée, figée, opposable.
	StatutEmise Statut = "emise"
	// StatutPayee : encaissée.
	StatutPayee Statut = "payee"
	// StatutAnnulee : corrigée par un avoir. La facture reste, l'avoir la
	// contrebalance — effacer une facture émise est précisément ce que la
	// numérotation continue interdit.
	StatutAnnulee Statut = "annulee"
)

// Ligne est une ligne de facture.
type Ligne struct {
	Label       string  `dynamodbav:"label" json:"label"`
	Quantity    float64 `dynamodbav:"quantity" json:"quantity"`
	UnitPriceHT float64 `dynamodbav:"unitPriceHt" json:"unitPriceHT"`
	VATRate     float64 `dynamodbav:"vatRate" json:"vatRate"`
}

// TotalHT rend le montant hors taxes de la ligne.
func (l Ligne) TotalHT() float64 { return l.Quantity * l.UnitPriceHT }

// Partie fige l'identité d'un client au moment de l'émission.
type Partie struct {
	Name       string `dynamodbav:"name" json:"name"`
	Address    string `dynamodbav:"address,omitempty" json:"address,omitempty"`
	PostalCode string `dynamodbav:"postalCode,omitempty" json:"postalCode,omitempty"`
	City       string `dynamodbav:"city,omitempty" json:"city,omitempty"`
	SIRET      string `dynamodbav:"siret,omitempty" json:"siret,omitempty"`
	Email      string `dynamodbav:"email,omitempty" json:"email,omitempty"`
}

// Facture est une facture ou un avoir.
type Facture struct {
	ddb.Record

	ID    string `dynamodbav:"id" json:"id"`
	OrgID string `dynamodbav:"orgId" json:"orgId"`

	// Number est vide tant que la facture est en brouillon.
	Number string `dynamodbav:"number,omitempty" json:"number,omitempty"`
	Status Statut `dynamodbav:"status" json:"status"`

	ClientID string `dynamodbav:"clientId,omitempty" json:"clientId,omitempty"`
	Client   Partie `dynamodbav:"client" json:"client"`

	FileID        string `dynamodbav:"fileId,omitempty" json:"fileId,omitempty"`
	FileReference string `dynamodbav:"fileReference,omitempty" json:"fileReference,omitempty"`

	Lines []Ligne `dynamodbav:"lines" json:"lines"`

	// VATExempt reprend le régime de l'organisme au moment de l'émission :
	// une facture exonérée doit porter la mention et ne porter aucune taxe.
	VATExempt bool    `dynamodbav:"vatExempt" json:"vatExempt"`
	TotalHT   float64 `dynamodbav:"totalHt" json:"totalHT"`
	TotalVAT  float64 `dynamodbav:"totalVat" json:"totalVAT"`
	TotalTTC  float64 `dynamodbav:"totalTtc" json:"totalTTC"`

	IssuedOn string     `dynamodbav:"issuedOn,omitempty" json:"issuedOn,omitempty"`
	DueOn    string     `dynamodbav:"dueOn,omitempty" json:"dueOn,omitempty"`
	PaidAt   *time.Time `dynamodbav:"paidAt,omitempty" json:"paidAt,omitempty"`
	PaidWay  string     `dynamodbav:"paidWay,omitempty" json:"paidWay,omitempty"`

	PaymentTerms string `dynamodbav:"paymentTerms,omitempty" json:"paymentTerms,omitempty"`
	Notes        string `dynamodbav:"notes,omitempty" json:"notes,omitempty"`

	// CreditNoteFor désigne la facture qu'un avoir corrige. Une pièce qui le
	// porte est un avoir, et ses montants sont négatifs.
	CreditNoteFor string `dynamodbav:"creditNoteFor,omitempty" json:"creditNoteFor,omitempty"`
	// CancelledBy désigne l'avoir qui a annulé cette facture.
	CancelledBy string `dynamodbav:"cancelledBy,omitempty" json:"cancelledBy,omitempty"`
}

// EstAvoir dit si la pièce est un avoir.
func (f Facture) EstAvoir() bool { return f.CreditNoteFor != "" }

// Modifiable dit si la pièce peut encore changer.
func (f Facture) Modifiable() bool { return f.Status == StatutBrouillon }

func factureSK(id string) string { return "FACTURE#" + id }

// Service porte la facturation.
type Service struct {
	db       *ddb.Client
	crm      *crm.Service
	identity *identity.Service
	now      func() time.Time
}

// NewService construit le service.
func NewService(db *ddb.Client, crmService *crm.Service, ident *identity.Service, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{db: db, crm: crmService, identity: ident, now: now}
}

// CreateInput décrit une facture à ouvrir.
type CreateInput struct {
	OrgID        string
	ClientID     string
	FileID       string
	Lines        []Ligne
	PaymentTerms string
	Notes        string
	DueOn        string
}

// Create ouvre une facture en brouillon.
func (s *Service) Create(ctx context.Context, in CreateInput) (Facture, error) {
	if len(in.Lines) == 0 {
		return Facture{}, fmt.Errorf("une facture sans ligne ne facture rien")
	}
	if in.ClientID == "" {
		return Facture{}, fmt.Errorf("une facture doit désigner son client")
	}

	now := s.now()
	facture := Facture{
		Record: ddb.Record{
			PK: ddb.OrgPK(in.OrgID), SK: factureSK(identity.NewID()),
			Type: "facture", CreatedAt: now, UpdatedAt: now,
		},
		OrgID: in.OrgID, Status: StatutBrouillon,
		ClientID: in.ClientID, FileID: in.FileID,
		Lines: in.Lines, PaymentTerms: in.PaymentTerms,
		Notes: in.Notes, DueOn: in.DueOn,
	}
	facture.ID = strings.TrimPrefix(facture.SK, "FACTURE#")

	if err := s.remplir(ctx, &facture); err != nil {
		return Facture{}, err
	}
	if err := ddb.Put(ctx, s.db, facture); err != nil {
		return Facture{}, err
	}
	return facture, nil
}

// Update corrige un brouillon.
func (s *Service) Update(ctx context.Context, orgID, id string, apply func(*Facture)) (Facture, error) {
	facture, err := s.Get(ctx, orgID, id)
	if err != nil {
		return Facture{}, err
	}
	if !facture.Modifiable() {
		return Facture{}, fmt.Errorf(
			"une facture émise ne se modifie plus : corrigez-la par un avoir")
	}
	apply(&facture)
	if err := s.remplir(ctx, &facture); err != nil {
		return Facture{}, err
	}
	facture.UpdatedAt = s.now()
	if err := ddb.Put(ctx, s.db, facture); err != nil {
		return Facture{}, err
	}
	return facture, nil
}

// remplit recalcule les totaux et fige le client.
func (s *Service) remplir(ctx context.Context, facture *Facture) error {
	org, err := s.identity.LoadOrg(ctx, facture.OrgID)
	if err != nil {
		return err
	}
	facture.VATExempt = org.VATExempt

	if facture.ClientID != "" {
		contact, err := s.crm.GetContact(ctx, facture.OrgID, facture.ClientID)
		if err != nil {
			return fmt.Errorf("client introuvable: %w", err)
		}
		facture.Client = Partie{
			Name: contact.DisplayName(), SIRET: contact.SIRET, Email: contact.Email,
			Address: contact.Address.Line1, PostalCode: contact.Address.PostalCode,
			City: contact.Address.City,
		}
	}
	if facture.FileID != "" && facture.FileReference == "" {
		if file, err := s.crm.GetFile(ctx, facture.OrgID, facture.FileID); err == nil {
			facture.FileReference = file.Reference
		}
	}

	facture.TotalHT, facture.TotalVAT = 0, 0
	for _, ligne := range facture.Lines {
		ht := ligne.TotalHT()
		facture.TotalHT += ht
		// L'exonération de l'article 261-4-4° a du CGI n'est pas un taux à
		// zéro : c'est l'absence de taxe, et la facture doit le dire.
		if !facture.VATExempt {
			facture.TotalVAT += ht * ligne.VATRate / 100
		}
	}
	facture.TotalTTC = facture.TotalHT + facture.TotalVAT
	return nil
}

// Get lit une facture.
func (s *Service) Get(ctx context.Context, orgID, id string) (Facture, error) {
	return ddb.Get[Facture](ctx, s.db, ddb.OrgPK(orgID), factureSK(id))
}

// List rend les factures d'un organisme, la plus récente d'abord.
func (s *Service) List(ctx context.Context, orgID string, limit int32, cursor string) (ddb.Page[Facture], error) {
	return ddb.QueryPage[Facture](ctx, s.db, ddb.QuerySpec{
		PK: ddb.OrgPK(orgID), SKPrefix: "FACTURE#", Descending: true, Limit: limit,
	}, cursor)
}

// Issue attribue un numéro et fige la facture.
//
// Le numéro vient d'un compteur incrémenté atomiquement : deux émissions
// simultanées ne peuvent pas obtenir le même, et aucune ne peut sauter un rang.
// C'est la seule écriture non transactionnelle du domaine, et c'est justement
// pour cela qu'elle passe par ADD plutôt que par un lire-modifier-écrire.
func (s *Service) Issue(ctx context.Context, orgID, id string, actor audit.Actor) (Facture, error) {
	facture, err := s.Get(ctx, orgID, id)
	if err != nil {
		return Facture{}, err
	}
	if facture.Status != StatutBrouillon {
		return Facture{}, fmt.Errorf("cette facture est déjà émise")
	}
	if len(facture.Lines) == 0 {
		return Facture{}, fmt.Errorf("une facture sans ligne ne facture rien")
	}

	// L'identité juridique doit être complète : une facture à laquelle il
	// manque le SIRET ou la forme juridique n'est pas conforme, et la corriger
	// après émission est impossible.
	org, err := s.identity.LoadOrg(ctx, orgID)
	if err != nil {
		return Facture{}, err
	}
	if manques := org.MissingLegal(); len(manques) > 0 {
		noms := make([]string, 0, len(manques))
		for _, manque := range manques {
			noms = append(noms, manque.Label)
		}
		return Facture{}, fmt.Errorf(
			"l'identité juridique de l'organisme est incomplète : %s", strings.Join(noms, ", "))
	}

	now := s.now()
	annee := now.Year()
	rang, err := s.db.Increment(ctx, ddb.OrgPK(orgID),
		fmt.Sprintf("COMPTEUR#FACTURE#%d", annee), "value", 1)
	if err != nil {
		return Facture{}, fmt.Errorf("numérotation: %w", err)
	}

	prefixe := "FA"
	if facture.EstAvoir() {
		prefixe = "AV"
	}
	facture.Number = fmt.Sprintf("%s-%d-%04d", prefixe, annee, rang)
	facture.Status = StatutEmise
	facture.IssuedOn = now.Format("2006-01-02")
	if facture.DueOn == "" {
		// Trente jours, le délai de droit commun de l'article L.441-10 du code
		// de commerce à défaut de stipulation.
		facture.DueOn = now.AddDate(0, 0, 30).Format("2006-01-02")
	}
	facture.UpdatedAt = now

	if err := s.remplir(ctx, &facture); err != nil {
		return Facture{}, err
	}

	// L'émission entre au journal du dossier quand il y en a un : c'est une
	// pièce du dossier probatoire au même titre que la convention.
	sujet := "org/" + orgID
	if facture.FileID != "" {
		sujet = "file/" + facture.FileID
	}
	if _, err := s.db.WriteWithAudit(ctx, sujet, []ddb.Write{{Item: facture}},
		func(prev audit.Event) (audit.Event, error) {
			return audit.Append(prev, sujet, now, audit.ActionDocumentGenerated, actor,
				map[string]any{
					"piece": "facture", "numero": facture.Number,
					"totalTTC": facture.TotalTTC, "client": facture.Client.Name,
				})
		}); err != nil {
		return Facture{}, err
	}
	return facture, nil
}

// MarkPaid enregistre l'encaissement.
func (s *Service) MarkPaid(ctx context.Context, orgID, id, moyen string, paid bool) (Facture, error) {
	facture, err := s.Get(ctx, orgID, id)
	if err != nil {
		return Facture{}, err
	}
	if facture.Status == StatutBrouillon {
		return Facture{}, fmt.Errorf("un brouillon ne s'encaisse pas : émettez-le d'abord")
	}
	now := s.now()
	if paid {
		facture.Status, facture.PaidAt, facture.PaidWay = StatutPayee, &now, moyen
	} else {
		facture.Status, facture.PaidAt, facture.PaidWay = StatutEmise, nil, ""
	}
	facture.UpdatedAt = now
	if err := ddb.Put(ctx, s.db, facture); err != nil {
		return Facture{}, err
	}
	return facture, nil
}

// CreditNote fabrique l'avoir qui corrige une facture.
//
// L'avoir reprend les lignes en négatif et référence la facture. Celle-ci
// passe en « annulée » mais reste : la faire disparaître romprait la
// numérotation continue, qui est précisément ce qu'un contrôle vérifie.
func (s *Service) CreditNote(ctx context.Context, orgID, id, motif string) (Facture, error) {
	source, err := s.Get(ctx, orgID, id)
	if err != nil {
		return Facture{}, err
	}
	if source.Status == StatutBrouillon {
		return Facture{}, fmt.Errorf("un brouillon n'a pas besoin d'avoir : corrigez-le")
	}
	if source.EstAvoir() {
		return Facture{}, fmt.Errorf("un avoir ne s'annule pas par un autre avoir")
	}
	if source.CancelledBy != "" {
		return Facture{}, fmt.Errorf("cette facture a déjà été annulée par un avoir")
	}

	lignes := make([]Ligne, 0, len(source.Lines))
	for _, ligne := range source.Lines {
		ligne.Quantity = -ligne.Quantity
		lignes = append(lignes, ligne)
	}

	now := s.now()
	avoir := Facture{
		Record: ddb.Record{
			PK: ddb.OrgPK(orgID), SK: factureSK(identity.NewID()),
			Type: "facture", CreatedAt: now, UpdatedAt: now,
		},
		OrgID: orgID, Status: StatutBrouillon,
		ClientID: source.ClientID, FileID: source.FileID,
		Lines: lignes, CreditNoteFor: source.Number,
		Notes: motif,
	}
	avoir.ID = strings.TrimPrefix(avoir.SK, "FACTURE#")
	if err := s.remplir(ctx, &avoir); err != nil {
		return Facture{}, err
	}

	source.Status = StatutAnnulee
	source.CancelledBy = avoir.ID
	source.UpdatedAt = now

	if err := s.db.Write(ctx, []ddb.Write{{Item: avoir}, {Item: source}}); err != nil {
		return Facture{}, err
	}
	return avoir, nil
}

// Delete retire un brouillon. Une facture émise ne se supprime jamais.
func (s *Service) Delete(ctx context.Context, orgID, id string) error {
	facture, err := s.Get(ctx, orgID, id)
	if err != nil {
		return err
	}
	if !facture.Modifiable() {
		return fmt.Errorf(
			"une facture émise ne se supprime pas : la numérotation doit rester continue")
	}
	return ddb.Delete(ctx, s.db, ddb.OrgPK(orgID), facture.SK)
}
