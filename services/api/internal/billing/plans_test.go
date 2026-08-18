package billing_test

import (
	"strings"
	"testing"

	"github.com/lemlearn/api/internal/billing"
)

// Le catalogue doit rester cohérent : un palier plus cher qui offrirait moins
// se remarquerait sur la page de tarifs avant de se remarquer ici.
func TestPlansGrowMonotonically(t *testing.T) {
	for i := 1; i < len(billing.Plans); i++ {
		lower, upper := billing.Plans[i-1], billing.Plans[i]
		if upper.PriceCents <= lower.PriceCents {
			t.Errorf("%s (%d) n'est pas plus cher que %s (%d)",
				upper.Code, upper.PriceCents, lower.Code, lower.PriceCents)
		}
		for _, quota := range []struct {
			name         string
			lower, upper int
		}{
			{"apprenants", lower.MaxLearners, upper.MaxLearners},
			{"stockage", lower.MaxStorageGB, upper.MaxStorageGB},
			{"signatures", lower.MaxSignatures, upper.MaxSignatures},
			{"vidéo", lower.MaxVideoHours, upper.MaxVideoHours},
		} {
			if quota.upper <= quota.lower {
				t.Errorf("%s : %s (%d) n'offre pas plus que %s (%d)",
					quota.name, upper.Code, quota.upper, lower.Code, quota.lower)
			}
		}
	}
}

// Le dépassement se dit en français, avec les deux nombres : c'est un
// commercial qui le lit au téléphone, pas un analyseur de journaux.
func TestOverageIsReadableAndCountsEachQuotaOnce(t *testing.T) {
	plan, err := billing.PlanByCode("essentiel")
	if err != nil {
		t.Fatal(err)
	}

	usage := billing.Usage{
		Learners:   128,
		Signatures: plan.MaxSignatures + 1,
		VideoMs:    int64(plan.MaxVideoHours+3) * 3_600_000,
		StorageMB:  int64(plan.MaxStorageGB+1) * 1024,
	}

	over := usage.Overage(plan)
	if len(over) != 4 {
		t.Fatalf("dépassements = %v, attendu les quatre quotas", over)
	}
	if !strings.Contains(over[0], "128 apprenants sur 100") {
		t.Errorf("libellé = %q", over[0])
	}

	// Une organisation dans les clous ne doit produire aucune ligne : un
	// écran qui signale tout le monde ne signale personne.
	if within := (billing.Usage{Learners: 10}).Overage(plan); len(within) != 0 {
		t.Errorf("dépassements signalés à tort : %v", within)
	}
}

// Un plan retiré du catalogue ne doit pas être résolu en silence : la vue
// super-admin doit pouvoir le distinguer d'un plan connu.
func TestUnknownPlanIsRefused(t *testing.T) {
	if _, err := billing.PlanByCode("gratuit-a-vie"); err == nil {
		t.Fatal("une formule inconnue a été acceptée")
	}
}
