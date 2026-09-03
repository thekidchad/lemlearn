// Package crm porte les contacts et les dossiers.
//
// Un « dossier » n'est pas une opportunité commerciale au sens d'un CRM
// classique : c'est l'unité de preuve. Il commence en prospect et finit en
// archive exportable, et tout ce qui se produit entre les deux y est rattaché.
package crm

import (
	"fmt"
	"strings"
	"time"

	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// Kind distingue les trois natures de contact.
type Kind string

const (
	// KindLearner : la personne physique formée.
	KindLearner Kind = "learner"
	// KindCompany : l'entreprise cliente qui commande la formation.
	KindCompany Kind = "company"
	// KindFunder : l'OPCO, France Travail, la Caisse des Dépôts…
	KindFunder Kind = "funder"
)

// FundingSource nomme l'origine des fonds d'un dossier.
//
// Les catégories sont celles du cadre C du bilan pédagogique et financier :
// s'en écarter obligerait à retraduire chaque ligne au moment de la
// déclaration, c'est-à-dire au pire moment.
type FundingSource string

const (
	// FundingCompany : l'entreprise paie directement la formation de son
	// salarié, hors dispositif mutualisé.
	FundingCompany FundingSource = "entreprise"
	// FundingOPCO : un opérateur de compétences prend en charge.
	FundingOPCO FundingSource = "opco"
	// FundingPublic : État, Région, France Travail, collectivités.
	FundingPublic FundingSource = "public"
	// FundingIndividual : le stagiaire paie lui-même. C'est la catégorie qui
	// déclenche le contrat de formation et ses protections.
	FundingIndividual FundingSource = "particulier"
	// FundingSubcontract : un autre organisme de formation nous sous-traite
	// l'action. Le cadre G du bilan l'isole.
	FundingSubcontract FundingSource = "sous-traitance"
	// FundingOther couvre le reste plutôt que de laisser un vide : une ligne
	// « autres produits » existe au formulaire.
	FundingOther FundingSource = "autre"
)

// Valid indique si la nature fait partie de la liste fermée.
func (k Kind) Valid() bool {
	switch k {
	case KindLearner, KindCompany, KindFunder:
		return true
	}
	return false
}

// Stage est l'étape d'un dossier dans le pipeline.
type Stage string

const (
	StageProspect   Stage = "prospect"
	StageQuote      Stage = "quote"
	StageAgreement  Stage = "agreement"
	StageInTraining Stage = "in_training"
	StageClosed     Stage = "closed"
	StageLost       Stage = "lost"
)

// Order donne le rang d'une étape, pour ordonner les colonnes du pipeline.
func (s Stage) Order() int {
	switch s {
	case StageProspect:
		return 0
	case StageQuote:
		return 1
	case StageAgreement:
		return 2
	case StageInTraining:
		return 3
	case StageClosed:
		return 4
	case StageLost:
		return 5
	}
	return 99
}

// Valid indique si l'étape existe.
func (s Stage) Valid() bool { return s.Order() != 99 }

// Address est une adresse postale française.
type Address struct {
	Line1      string `dynamodbav:"line1,omitempty" json:"line1,omitempty"`
	Line2      string `dynamodbav:"line2,omitempty" json:"line2,omitempty"`
	PostalCode string `dynamodbav:"postalCode,omitempty" json:"postalCode,omitempty"`
	City       string `dynamodbav:"city,omitempty" json:"city,omitempty"`
	Country    string `dynamodbav:"country,omitempty" json:"country,omitempty"`
}

// Contact est un apprenant, une entreprise cliente ou un financeur.
type Contact struct {
	ddb.Record

	ID    string `dynamodbav:"id" json:"id"`
	OrgID string `dynamodbav:"orgId" json:"orgId"`
	Kind  Kind   `dynamodbav:"kind" json:"kind"`

	// Personne physique
	FirstName string `dynamodbav:"firstName,omitempty" json:"firstName,omitempty"`
	LastName  string `dynamodbav:"lastName,omitempty" json:"lastName,omitempty"`
	// BirthDate est requise pour l'attestation et le dossier de financement.
	// Stockée en AAAA-MM-JJ : une date de naissance n'a pas d'heure et n'a
	// surtout pas de fuseau.
	BirthDate  string `dynamodbav:"birthDate,omitempty" json:"birthDate,omitempty"`
	BirthPlace string `dynamodbav:"birthPlace,omitempty" json:"birthPlace,omitempty"`

	// Personne morale
	CompanyName string `dynamodbav:"companyName,omitempty" json:"companyName,omitempty"`
	SIRET       string `dynamodbav:"siret,omitempty" json:"siret,omitempty"`
	LegalForm   string `dynamodbav:"legalForm,omitempty" json:"legalForm,omitempty"`

	Email    string  `dynamodbav:"email,omitempty" json:"email,omitempty"`
	Phone    string  `dynamodbav:"phone,omitempty" json:"phone,omitempty"`
	Address  Address `dynamodbav:"address,omitempty" json:"address,omitempty"`
	Position string  `dynamodbav:"position,omitempty" json:"position,omitempty"`
	Notes    string  `dynamodbav:"notes,omitempty" json:"notes,omitempty"`

	// MarketingSource dit d'où vient la personne : salon, recommandation,
	// site, campagne. Sans elle, on ne sait pas ce qui remplit le carnet, et
	// on continue de payer ce qui n'apporte rien.
	MarketingSource string `dynamodbav:"marketingSource,omitempty" json:"marketingSource,omitempty"`
	// ConvertedOn est le jour où le prospect est devenu client. Il ne se
	// déduit pas de la date de création : une fiche se saisit souvent des
	// semaines avant que quoi que ce soit ne soit signé.
	ConvertedOn string `dynamodbav:"convertedOn,omitempty" json:"convertedOn,omitempty"`

	// IdentityDocKey est la clé S3 de la pièce d'identité, dans le
	// compartiment chiffré. Jamais d'URL ici : seulement une clé, résolue en
	// URL présignée de soixante secondes au moment de l'affichage.
	IdentityDocKey string `dynamodbav:"identityDocKey,omitempty" json:"identityDocKey,omitempty"`

	// Anonymized marque un contact effacé au titre du RGPD. Les pièces
	// probatoires qui lui sont rattachées survivent sous pseudonyme.
	Anonymized bool `dynamodbav:"anonymized" json:"anonymized"`
}

// DisplayName est le libellé affiché, selon la nature du contact.
func (c Contact) DisplayName() string {
	if c.Anonymized {
		return "Apprenant anonymisé"
	}
	if c.Kind == KindLearner {
		return strings.TrimSpace(c.FirstName + " " + c.LastName)
	}
	if c.CompanyName != "" {
		return c.CompanyName
	}
	return strings.TrimSpace(c.FirstName + " " + c.LastName)
}

// NewContact construit un contact prêt à écrire.
func NewContact(orgID string, kind Kind, now time.Time) Contact {
	id := identity.NewID()
	return Contact{
		Record: ddb.Record{
			PK:        ddb.OrgPK(orgID),
			SK:        ddb.ContactSK(id),
			GSI1PK:    ddb.GSI1Contacts(orgID, string(kind)),
			Type:      "contact",
			CreatedAt: now,
			UpdatedAt: now,
		},
		ID: id, OrgID: orgID, Kind: kind,
	}
}

// Reindex recalcule les clés d'index dérivées des champs affichés.
//
// À appeler après toute modification : oublier de le faire ferait disparaître
// le contact des listes sans qu'aucune erreur ne se produise.
func (c *Contact) Reindex(now time.Time) {
	c.GSI1PK = ddb.GSI1Contacts(c.OrgID, string(c.Kind))
	c.GSI1SK = ddb.SearchKey(c.LastName, c.FirstName, c.CompanyName)
	c.GSI2PK = ddb.GSI2Search(c.OrgID)
	c.GSI2SK = ddb.SearchKey(c.DisplayName(), c.Email)
	c.UpdatedAt = now
}

// Validate refuse un contact inexploitable.
func (c Contact) Validate() error {
	if !c.Kind.Valid() {
		return fmt.Errorf("nature de contact %q inconnue", c.Kind)
	}
	switch c.Kind {
	case KindLearner:
		if strings.TrimSpace(c.LastName) == "" {
			return fmt.Errorf("le nom de l'apprenant est obligatoire")
		}
	case KindCompany, KindFunder:
		if strings.TrimSpace(c.CompanyName) == "" {
			return fmt.Errorf("la raison sociale est obligatoire")
		}
	}
	if c.Email != "" && !strings.Contains(c.Email, "@") {
		return fmt.Errorf("adresse e-mail invalide")
	}
	if c.BirthDate != "" {
		if _, err := time.Parse("2006-01-02", c.BirthDate); err != nil {
			return fmt.Errorf("date de naissance attendue au format AAAA-MM-JJ")
		}
	}
	return nil
}

// File est un dossier : l'unité de preuve du produit.
type File struct {
	ddb.Record

	ID    string `dynamodbav:"id" json:"id"`
	OrgID string `dynamodbav:"orgId" json:"orgId"`
	// Reference est l'identifiant lisible cité par un auditeur.
	Reference string `dynamodbav:"reference" json:"reference"`
	Stage     Stage  `dynamodbav:"stage" json:"stage"`

	LearnerID string `dynamodbav:"learnerId,omitempty" json:"learnerId,omitempty"`
	// CatalogPriceHT et DiscountHT expliquent le prix : PriceHT est le net,
	// celui qui figure à la convention et qu'on facture. Sans le prix
	// catalogue ni la remise, une ristourne consentie au téléphone ne laisse
	// aucune trace, et personne ne sait six mois plus tard si le tarif
	// affiché a bougé ou si un geste a été fait.
	CatalogPriceHT float64 `dynamodbav:"catalogPriceHt,omitempty" json:"catalogPriceHT,omitempty"`
	DiscountHT     float64 `dynamodbav:"discountHt,omitempty" json:"discountHT,omitempty"`

	CompanyID string `dynamodbav:"companyId,omitempty" json:"companyId,omitempty"`
	FunderID  string `dynamodbav:"funderId,omitempty" json:"funderId,omitempty"`
	CourseID  string `dynamodbav:"courseId,omitempty" json:"courseId,omitempty"`
	SessionID string `dynamodbav:"sessionId,omitempty" json:"sessionId,omitempty"`

	Title   string  `dynamodbav:"title" json:"title"`
	PriceHT float64 `dynamodbav:"priceHT" json:"priceHT"`
	// Funding dit d'où vient l'argent. C'est la ventilation qu'exige le cadre
	// C du bilan pédagogique et financier, et le seul renseignement du dossier
	// qu'on ne peut pas reconstituer après coup : douze mois plus tard,
	// personne ne se souvient si telle formation a été payée par l'OPCO ou par
	// l'entreprise elle-même.
	Funding FundingSource `dynamodbav:"funding,omitempty" json:"funding,omitempty"`
	VATRate float64       `dynamodbav:"vatRate" json:"vatRate"`
	OwnerID string        `dynamodbav:"ownerId,omitempty" json:"ownerId,omitempty"`

	// Tags libres : #présentiel, #distanciel, #certification, #OPCO-validé…
	Tags []string `dynamodbav:"tags,omitempty" json:"tags,omitempty"`

	// Proof est le décompte des pièces attendues et réunies. Il est
	// recalculé à chaque événement plutôt que d'être interrogé à la volée :
	// c'est la valeur affichée en tête de fiche et triée dans le pipeline.
	Proof ProofStatus `dynamodbav:"proof" json:"proof"`

	ClosedAt *time.Time `dynamodbav:"closedAt,omitempty" json:"closedAt,omitempty"`
}

// ProofStatus résume la complétude du dossier de preuve.
type ProofStatus struct {
	Expected int      `dynamodbav:"expected" json:"expected"`
	Present  int      `dynamodbav:"present" json:"present"`
	Missing  []string `dynamodbav:"missing,omitempty" json:"missing,omitempty"`
}

// Percent est la complétude en pourcentage entier.
func (p ProofStatus) Percent() int {
	if p.Expected == 0 {
		return 0
	}
	return p.Present * 100 / p.Expected
}

// NewFile construit un dossier prêt à écrire.
func NewFile(orgID, reference, title string, now time.Time) File {
	id := identity.NewID()
	file := File{
		Record: ddb.Record{
			PK:        ddb.OrgPK(orgID),
			SK:        ddb.FileSK(id),
			Type:      "file",
			CreatedAt: now,
			UpdatedAt: now,
		},
		ID: id, OrgID: orgID, Reference: reference,
		Title: title, Stage: StageProspect, VATRate: 20,
		Proof: ProofStatus{Expected: len(RequiredProofs), Missing: append([]string(nil), RequiredProofs...)},
	}
	file.Reindex(now)
	return file
}

// Reindex recalcule les clés d'index du dossier.
func (f *File) Reindex(now time.Time) {
	f.GSI1PK = ddb.GSI1Files(f.OrgID, string(f.Stage))
	// Le plus récemment modifié en tête de colonne : c'est l'ordre dans
	// lequel on veut voir un pipeline.
	f.GSI1SK = now.UTC().Format(time.RFC3339Nano)
	f.GSI2PK = ddb.GSI2Search(f.OrgID)
	f.GSI2SK = ddb.SearchKey(f.Reference, f.Title)
	f.UpdatedAt = now
}

// RequiredProofs énumère les pièces attendues au dossier d'un apprenant, dans
// l'ordre où un auditeur les demande.
var RequiredProofs = []string{
	"Identité de l'apprenant",
	"Consentement RGPD",
	"Devis",
	"Convention signée",
	"Programme de formation",
	"Évaluation de positionnement",
	"Relevés de connexion",
	"Questionnaires post-module",
	"Évaluation finale",
	"Feuilles d'émargement",
	"Satisfaction à chaud",
	"Satisfaction à froid",
	"Attestation de fin de formation",
}
