package emailtpl_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lemlearn/api/internal/emailtpl"
)

// Chaque gabarit livré doit se rendre avec ses propres valeurs d'exemple :
// c'est le minimum avant d'en confier la réécriture à quelqu'un.
func TestEveryDefaultRendersWithItsSample(t *testing.T) {
	for _, definition := range emailtpl.Defaults() {
		subject, body, err := emailtpl.Render(definition.Subject, definition.Body, definition.Sample())
		if err != nil {
			t.Fatalf("%s : %v", definition.Key, err)
		}
		if strings.Contains(subject, "{{") || strings.Contains(body, "{{") {
			t.Errorf("%s : une variable n'a pas été remplacée", definition.Key)
		}
		// Chaque variable annoncée doit servir, sinon l'éditeur affiche une
		// aide qui ne correspond à rien.
		for _, variable := range definition.Variables {
			if !strings.Contains(definition.Body+definition.Subject, "{{."+variable.Name+"}}") {
				t.Errorf("%s : la variable %s est annoncée mais inutilisée", definition.Key, variable.Name)
			}
		}
	}
}

// Les valeurs sont échappées : un nom contenant un chevron ne doit pas
// injecter de balise dans un message envoyé à un tiers.
func TestValuesAreEscaped(t *testing.T) {
	_, body, err := emailtpl.Render("x", "<p>{{.Name}}</p>", map[string]any{
		"Name": `<script>alert("xss")</script>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "<script>") {
		t.Fatalf("balise injectée : %s", body)
	}
}

// Un gabarit mal formé est refusé à l'écriture, pas au moment de l'envoi.
func TestBrokenTemplateIsRefusedUpFront(t *testing.T) {
	if _, err := emailtpl.Preview(emailtpl.KeySignatureOTP, "Code {{.Code", "corps"); err == nil {
		t.Fatal("un gabarit mal formé a été accepté")
	}
	if _, err := emailtpl.Preview("gabarit.inconnu", "x", "y"); err == nil {
		t.Fatal("une clé inconnue a été acceptée")
	}
}

// Sans base, le service rend les gabarits d'origine : c'est ce qu'il faut en
// local, et le repli qui protège les envois en production.
func TestServiceFallsBackToDefaults(t *testing.T) {
	service := emailtpl.NewService(nil, nil)

	message, err := service.Compose(context.Background(), emailtpl.KeySurveyCold, map[string]any{
		"FirstName":   "Camille",
		"CourseTitle": "Prévention des risques",
		"Link":        "https://exemple.fr/s/jeton",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message.Subject, "Prévention des risques") {
		t.Errorf("sujet = %q", message.Subject)
	}
	if !strings.Contains(message.HTML, "https://exemple.fr/s/jeton") {
		t.Error("le lien ne figure pas dans le message")
	}
}
