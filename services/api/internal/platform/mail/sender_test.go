package mail

import (
	"context"
	"testing"
)

// Le nom d'expéditeur suit l'organisme, mais jamais l'adresse : celle-ci
// dépend d'un domaine vérifié chez le fournisseur d'envoi, et laisser un
// organisme en écrire une autre reviendrait à signer en son nom.
func TestSenderFrom(t *testing.T) {
	const configure = "lemlearn <ne-pas-repondre@lemlearn.fr>"

	cas := []struct {
		nom    string
		entree string
		attend string
	}{
		{"sans nom", "", configure},
		{"organisme", "Institut Vulcain", "Institut Vulcain <ne-pas-repondre@lemlearn.fr>"},
		{"adresse nue configurée", "Aubépine", "Aubépine <envoi@exemple.fr>"},
	}

	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			configuré := configure
			if c.nom == "adresse nue configurée" {
				configuré = "envoi@exemple.fr"
			}
			ctx := WithSender(context.Background(), c.entree)
			if got := SenderFrom(ctx, configuré); got != c.attend {
				t.Errorf("SenderFrom = %q, attendu %q", got, c.attend)
			}
		})
	}
}

// Un nom d'organisme est une donnée saisie : un retour à la ligne y injecterait
// un en-tête de courriel supplémentaire, ce qui est la façon classique de
// détourner un expéditeur.
func TestSenderFromNettoieLesEnTetes(t *testing.T) {
	ctx := WithSender(context.Background(), "Vulcain\r\nBcc: attaquant@exemple.fr")
	got := SenderFrom(ctx, "lemlearn <ne-pas-repondre@lemlearn.fr>")
	if got != "VulcainBcc: attaquant@exemple.fr <ne-pas-repondre@lemlearn.fr>" {
		t.Errorf("obtenu %q", got)
	}
	for _, interdit := range []string{"\r", "\n", "<Bcc"} {
		if contains(got, interdit) {
			t.Errorf("le caractère %q a survécu dans %q", interdit, got)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
