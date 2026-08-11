package doc

import "fmt"

// SignatureZoneRole désigne qui doit signer une zone.
type SignatureZoneRole string

const (
	// RoleLearner : l'apprenant (convention tripartite, émargement).
	RoleLearner SignatureZoneRole = "learner"
	// RoleClient : l'entreprise cliente ou le financeur.
	RoleClient SignatureZoneRole = "client"
	// RoleTrainer : le formateur (contresignature d'émargement).
	RoleTrainer SignatureZoneRole = "trainer"
	// RoleOrganization : le représentant de l'organisme de formation.
	RoleOrganization SignatureZoneRole = "organization"
)

// SignatureZoneKind distingue une zone de signature d'une mention manuscrite
// guidée (« Bon pour accord », lieu, date) — cette dernière ne reçoit pas de
// tracé mais doit être mise en évidence pendant la saisie.
type SignatureZoneKind string

const (
	KindSignature          SignatureZoneKind = "signature"
	KindHandwrittenMention SignatureZoneKind = "handwritten_mention"
)

// SignatureZone est un emplacement déclaré par le gabarit Typst.
//
// Convention : page 1-based, origine en haut à gauche, unités en points PDF.
// C'est la convention de Typst `here().position()` — aucune conversion n'est
// faite, ce qui évite la classe de bugs « signature à l'envers ».
type SignatureZone struct {
	Role   SignatureZoneRole `json:"role"`
	Kind   SignatureZoneKind `json:"kind"`
	Page   int               `json:"page"`
	X      float64           `json:"x"`
	Y      float64           `json:"y"`
	Width  float64           `json:"width"`
	Height float64           `json:"height"`
}

// Validate refuse les zones qui ne permettraient pas d'apposer un tracé.
func (z SignatureZone) Validate() error {
	switch z.Role {
	case RoleLearner, RoleClient, RoleTrainer, RoleOrganization:
	default:
		return fmt.Errorf("zone de signature: rôle inconnu %q", z.Role)
	}
	switch z.Kind {
	case KindSignature, KindHandwrittenMention:
	default:
		return fmt.Errorf("zone de signature: type inconnu %q", z.Kind)
	}
	if z.Page < 1 {
		return fmt.Errorf("zone de signature: page %d invalide (1-based)", z.Page)
	}
	if z.Width <= 0 || z.Height <= 0 {
		return fmt.Errorf("zone de signature: dimensions %.1f×%.1f invalides", z.Width, z.Height)
	}
	return nil
}

// ZonesFor filtre les zones d'un rôle donné, signatures uniquement.
func ZonesFor(zones []SignatureZone, role SignatureZoneRole) []SignatureZone {
	out := make([]SignatureZone, 0, 1)
	for _, z := range zones {
		if z.Role == role && z.Kind == KindSignature {
			out = append(out, z)
		}
	}
	return out
}
