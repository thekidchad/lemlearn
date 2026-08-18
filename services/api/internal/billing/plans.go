// Package billing porte les abonnements : ce que coûte un plan, ce qu'il
// autorise, et où en est chaque organisation.
//
// Les quotas ne bloquent rien pour l'instant — ils s'affichent. Couper l'accès
// d'un organisme en pleine session de formation parce qu'il a dépassé son
// nombre d'apprenants ferait plus de dégâts qu'un dépassement facturé, et la
// première chose que demande un commercial est de voir qui déborde, pas
// d'empêcher qui que ce soit de travailler.
package billing

import "fmt"

// Plan est une formule d'abonnement.
type Plan struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	// PriceCents est le prix mensuel hors taxes, en centimes. Les montants
	// sont en entiers : un prix en flottant finit toujours par produire une
	// facture à 29,989999 €.
	PriceCents int `json:"priceCents"`

	MaxLearners   int `json:"maxLearners"`
	MaxStorageGB  int `json:"maxStorageGb"`
	MaxSignatures int `json:"maxSignatures"`
	MaxVideoHours int `json:"maxVideoHours"`

	Description string `json:"description"`
}

// Plans est le catalogue, du plus petit au plus grand.
//
// Les paliers suivent la taille de l'organisme, pas le nombre de
// fonctionnalités : un organisme de trois personnes a besoin de la même chaîne
// de preuve qu'un centre de cent formateurs — c'est le volume qui change, et
// vendre la conformité en option serait vendre un produit qui ne sert à rien.
var Plans = []Plan{
	{
		Code: "trial", Label: "Essai", PriceCents: 0,
		MaxLearners: 10, MaxStorageGB: 2, MaxSignatures: 10, MaxVideoHours: 2,
		Description: "Trente jours pour monter un dossier complet, de bout en bout.",
	},
	{
		Code: "essentiel", Label: "Essentiel", PriceCents: 8900,
		MaxLearners: 100, MaxStorageGB: 50, MaxSignatures: 200, MaxVideoHours: 20,
		Description: "Un organisme indépendant, jusqu'à cent apprenants par an.",
	},
	{
		Code: "structure", Label: "Structuré", PriceCents: 19900,
		MaxLearners: 500, MaxStorageGB: 250, MaxSignatures: 1000, MaxVideoHours: 100,
		Description: "Plusieurs formateurs, un catalogue en ligne, des audits réguliers.",
	},
	{
		Code: "reseau", Label: "Réseau", PriceCents: 49900,
		MaxLearners: 5000, MaxStorageGB: 2000, MaxSignatures: 10000, MaxVideoHours: 500,
		Description: "Réseau de centres, volumes élevés, accompagnement dédié.",
	},
}

// PlanByCode renvoie une formule.
func PlanByCode(code string) (Plan, error) {
	for _, plan := range Plans {
		if plan.Code == code {
			return plan, nil
		}
	}
	return Plan{}, fmt.Errorf("formule %q inconnue", code)
}

// Usage est la consommation constatée d'une organisation.
type Usage struct {
	Learners   int   `json:"learners"`
	Files      int   `json:"files"`
	Sessions   int   `json:"sessions"`
	Signatures int   `json:"signatures"`
	VideoMs    int64 `json:"videoMs"`
	StorageMB  int64 `json:"storageMb"`
}

// Overage liste les quotas dépassés, en clair.
//
// Le message est écrit pour être lu par un commercial au téléphone, pas pour
// être décodé : « 128 apprenants sur 100 » se comprend, « quota_learners
// exceeded » demande une traduction.
func (u Usage) Overage(plan Plan) []string {
	var over []string
	if plan.MaxLearners > 0 && u.Learners > plan.MaxLearners {
		over = append(over, fmt.Sprintf("%d apprenants sur %d", u.Learners, plan.MaxLearners))
	}
	if plan.MaxSignatures > 0 && u.Signatures > plan.MaxSignatures {
		over = append(over, fmt.Sprintf("%d signatures sur %d", u.Signatures, plan.MaxSignatures))
	}
	if hours := int(u.VideoMs / 3_600_000); plan.MaxVideoHours > 0 && hours > plan.MaxVideoHours {
		over = append(over, fmt.Sprintf("%d heures de vidéo sur %d", hours, plan.MaxVideoHours))
	}
	if gb := int(u.StorageMB / 1024); plan.MaxStorageGB > 0 && gb > plan.MaxStorageGB {
		over = append(over, fmt.Sprintf("%d Go stockés sur %d", gb, plan.MaxStorageGB))
	}
	return over
}
