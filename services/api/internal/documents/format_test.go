package documents

import "testing"

// Un avoir ne porte que des montants négatifs : leur mise en forme est le cas
// normal, pas un cas limite. Elle était fausse — les centimes ressortaient
// négatifs et le groupement des milliers plaçait une espace après le moins.
func TestFormatEURNegatif(t *testing.T) {
	// Les séparateurs sont ceux de l'usage français que le produit emploie :
	// espace fine insécable entre les milliers, insécable avant l'euro.
	cas := map[float64]string{
		-840:      "−840,00 €",
		-1008.50:  "−1 008,50 €",
		-0.99:     "−0,99 €",
		0:         "0,00 €",
		-12345.67: "−12 345,67 €",
	}
	for montant, attendu := range cas {
		if got := formatEUR(montant); got != attendu {
			t.Errorf("formatEUR(%v) = %q, attendu %q", montant, got, attendu)
		}
	}
}
