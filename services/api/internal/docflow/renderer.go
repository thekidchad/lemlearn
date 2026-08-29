// Package docflow compose les données du CRM en documents.
//
// Il existe pour éviter que `signature` ne dépende de `crm` et que `documents`
// ne dépende de la base : les gabarits restent des fonctions pures de leurs
// données, la signature reste un mécanisme, et c'est ici — et seulement ici —
// que l'on sait qu'une convention se remplit à partir d'un dossier, de ses
// contacts et de l'organisation.
package docflow

import (
	"context"
	"fmt"
	"time"

	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/documents"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/doc"
	"github.com/lemlearn/api/internal/signature"
)

// Renderer produit les PDF des documents d'un dossier.
type Renderer struct {
	identity *identity.Service
	crm      *crm.Service
	compiler doc.Compiler
}

// NewRenderer construit le composeur.
func NewRenderer(ident *identity.Service, crmService *crm.Service, compiler doc.Compiler) *Renderer {
	return &Renderer{identity: ident, crm: crmService, compiler: compiler}
}

// Render satisfait signature.Renderer : il rend le document d'une demande,
// vierge ou signé selon `applied`.
func (r *Renderer) Render(ctx context.Context, req signature.Request, applied []doc.AppliedSignature) ([]byte, error) {
	if r.compiler == nil {
		return nil, fmt.Errorf("compilateur de documents indisponible")
	}

	switch req.Kind {
	case "convention":
		convention, err := r.buildConvention(ctx, req)
		if err != nil {
			return nil, err
		}
		convention.Signatures = applied
		return r.compiler.Compile(ctx, documents.RenderConvention(convention))
	default:
		return nil, fmt.Errorf("gabarit %q inconnu", req.Kind)
	}
}

// buildConvention remplit une convention depuis le dossier.
func (r *Renderer) buildConvention(ctx context.Context, req signature.Request) (documents.Convention, error) {
	org, err := r.identity.LoadOrg(ctx, req.OrgID)
	if err != nil {
		return documents.Convention{}, fmt.Errorf("organisation: %w", err)
	}
	file, err := r.crm.GetFile(ctx, req.OrgID, req.FileID)
	if err != nil {
		return documents.Convention{}, fmt.Errorf("dossier: %w", err)
	}

	convention := documents.Convention{
		Reference: req.Reference,
		// La date d'établissement est celle de l'émission de la demande, pas
		// l'heure courante : sans cela, le document rendu au signataire
		// différerait de celui dont l'empreinte a été figée, et la
		// vérification d'intégrité échouerait à chaque ouverture.
		IssuedOn: req.IssuedAt,
		Org: documents.Party{
			Name: org.Name, Address: org.Address,
			PostalCode: org.PostalCode, City: org.City, SIRET: org.SIRET,
			// L'identité juridique complète : elle ne sert qu'au pied de page,
			// mais c'est ce pied de page qui rend le document opposable.
			LegalForm: org.LegalForm, Capital: org.Capital, RCS: org.RCS,
			VATNumber: org.VATNumber, VATExempt: org.VATExempt,
			NDA: org.NDA, NDARegion: org.NDARegion,
			Represented: org.RepName, Role: org.RepRole,
		},
		CourseTitle:   file.Title,
		Audience:      "Salariés et personnes en formation professionnelle continue",
		Prerequisites: "Aucun",
		DurationHours: 14,
		Modalities:    "Distanciel asynchrone (modules vidéo) et classe virtuelle",
		Means:         "Plateforme de formation en ligne, supports téléchargeables, questionnaire après chaque module",
		Assessment:    "Évaluation de positionnement à l'entrée, questionnaire après chaque module, évaluation finale",
		Sanction:      "Attestation de fin de formation",
		PriceHT:       file.PriceHT,
		VATRate:       file.VATRate,
		PaymentTerms:  "Solde à 30 jours à réception de facture",
		SignedCity:    org.City,
	}

	if file.LearnerID != "" {
		learner, err := r.crm.GetContact(ctx, req.OrgID, file.LearnerID)
		if err != nil {
			return documents.Convention{}, fmt.Errorf("apprenant: %w", err)
		}
		convention.Learners = []documents.LearnerLine{
			{FullName: learner.DisplayName(), Position: learner.Position},
		}
		// Sans entreprise cliente, l'apprenant est lui-même le bénéficiaire
		// signataire : c'est le cas d'un particulier finançant sa formation.
		convention.Client = contactToParty(learner)
	}
	if file.CompanyID != "" {
		company, err := r.crm.GetContact(ctx, req.OrgID, file.CompanyID)
		if err != nil {
			return documents.Convention{}, fmt.Errorf("entreprise: %w", err)
		}
		convention.Client = contactToParty(company)
	}
	if file.FunderID != "" {
		funder, err := r.crm.GetContact(ctx, req.OrgID, file.FunderID)
		if err != nil {
			return documents.Convention{}, fmt.Errorf("financeur: %w", err)
		}
		convention.FunderName = funder.DisplayName()
	}

	return convention, nil
}

func contactToParty(contact crm.Contact) documents.Party {
	party := documents.Party{
		Name:       contact.DisplayName(),
		LegalForm:  contact.LegalForm,
		Address:    contact.Address.Line1,
		PostalCode: contact.Address.PostalCode,
		City:       contact.Address.City,
		SIRET:      contact.SIRET,
	}
	if contact.Kind != crm.KindLearner {
		party.Represented = contact.FirstName + " " + contact.LastName
		party.Role = contact.Position
	}
	return party
}

// Certificate compose la ligne de scellement imprimée sous une signature.
//
// Elle est délibérément lisible à l'œil nu : un auditeur doit pouvoir
// rapprocher le document papier de l'entrée du journal sans outil.
func Certificate(unsignedSHA256 string, signedAt time.Time, authority string) string {
	stamp := "horodatage serveur"
	if authority != "" {
		stamp = "horodatage " + authority
	}
	return fmt.Sprintf("Document %s… · %s · %s",
		unsignedSHA256[:min(16, len(unsignedSHA256))],
		signedAt.UTC().Format(time.RFC3339),
		stamp)
}
