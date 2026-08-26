package emailtpl_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lemlearn/api/internal/emailtpl"
	"github.com/lemlearn/api/internal/platform/ddb"
)

// Une réécriture prend effet sans redéploiement — c'est tout l'intérêt — et le
// retour en arrière est toujours possible.
func TestOverrideTakesEffectAndCanBeReverted(t *testing.T) {
	service := emailtpl.NewService(ddb.NewTestClient(t), nil)
	ctx := context.Background()

	if _, err := service.Save(ctx, emailtpl.KeySurveyCold,
		"Un mot sur {{.CourseTitle}}",
		"<p>Bonjour {{.FirstName}}, <a href=\"{{.Link}}\">deux minutes</a> ?</p>",
		"equipe@lemlearn.fr"); err != nil {
		t.Fatalf("enregistrement: %v", err)
	}

	message, err := service.Compose(ctx, emailtpl.KeySurveyCold, map[string]any{
		"FirstName": "Camille", "CourseTitle": "SSIAP 1", "Link": "https://x.fr/j",
	})
	if err != nil {
		t.Fatal(err)
	}
	if message.Subject != "Un mot sur SSIAP 1" {
		t.Errorf("sujet = %q", message.Subject)
	}

	current, err := service.Get(ctx, emailtpl.KeySurveyCold)
	if err != nil {
		t.Fatal(err)
	}
	if !current.Overridden || current.UpdatedBy != "equipe@lemlearn.fr" {
		t.Errorf("état = %+v", current)
	}
	// Le gabarit d'origine reste disponible : revenir en arrière ne demande
	// pas d'aller consulter le dépôt.
	if !strings.Contains(current.DefaultBody, "Trois mois après") {
		t.Error("le gabarit d'origine n'accompagne pas la réécriture")
	}

	if _, err := service.Reset(ctx, emailtpl.KeySurveyCold); err != nil {
		t.Fatal(err)
	}
	restored, err := service.Compose(ctx, emailtpl.KeySurveyCold, map[string]any{
		"FirstName": "Camille", "CourseTitle": "SSIAP 1", "Link": "https://x.fr/j",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(restored.HTML, "Trois mois après") {
		t.Error("le gabarit d'origine n'est pas revenu")
	}
}

// Un gabarit qui ne s'exécute pas est refusé à l'enregistrement : l'erreur se
// découvre ici, pas au moment où un signataire attend son code.
func TestBrokenOverrideIsRefused(t *testing.T) {
	service := emailtpl.NewService(ddb.NewTestClient(t), nil)

	if _, err := service.Save(context.Background(), emailtpl.KeySignatureOTP,
		"Code {{.Code}}", "<p>{{.Code</p>", "equipe@lemlearn.fr"); err == nil {
		t.Fatal("un gabarit mal formé a été enregistré")
	}
}

// Une réécriture cassée en base ne bloque pas l'envoi : on retombe sur le
// gabarit d'origine. Save la refuserait, mais une donnée peut arriver
// autrement — migration, écriture manuelle — et un lien de signature ne doit
// pas rester en suspens pour autant.
func TestBrokenStoredTemplateFallsBackInsteadOfFailing(t *testing.T) {
	db := ddb.NewTestClient(t)
	service := emailtpl.NewService(db, nil)
	ctx := context.Background()

	// Écriture directe, sans passer par la validation.
	broken := emailtpl.Override{
		Record: ddb.Record{
			PK: emailtpl.PlatformPK, SK: "EMAIL#" + emailtpl.KeySignatureOTP,
			Type: "email_template",
		},
		Key: emailtpl.KeySignatureOTP, Subject: "Code {{.Code}}", Body: "<p>{{.Code</p>",
	}
	if err := ddb.Put(ctx, db, broken); err != nil {
		t.Fatal(err)
	}

	message, err := service.Compose(ctx, emailtpl.KeySignatureOTP, map[string]any{
		"Code": "482095", "Reference": "CONV-1",
	})
	if err != nil {
		t.Fatalf("l'envoi a échoué au lieu de retomber sur le gabarit d'origine: %v", err)
	}
	if !strings.Contains(message.HTML, "482095") {
		t.Error("le code ne figure pas dans le message de repli")
	}
}
