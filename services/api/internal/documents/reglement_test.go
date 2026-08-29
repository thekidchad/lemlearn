package documents

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/platform/doc"
)

func sampleReglement() Reglement {
	return Reglement{
		Org: Party{
			Name: "Institut Vulcain", LegalForm: "SAS",
			Address: "12 rue des Écoles", PostalCode: "75005", City: "Paris",
			SIRET: "84291736500018", Represented: "Marie Dubreuil", Role: "présidente",
			NDA: "11756789012", NDARegion: "Île-de-France",
		},
		IssuedOn: time.Date(2026, 1, 14, 9, 0, 0, 0, time.UTC),
	}
}

// Le contenu du règlement n'est pas libre : les articles R.6352-1 et suivants
// fixent ce qu'il doit couvrir. En omettre une part n'est pas une lacune de
// rédaction, c'est un manquement que relève un contrôle.
func TestReglementCoversWhatTheCodeRequires(t *testing.T) {
	source := string(RenderReglement(sampleReglement()).Source)

	for _, exigence := range []string{
		"hygiène et de sécurité",       // R.6352-1
		"amendes",                      // R.6352-3 : sanctions pécuniaires interdites
		"informé au préalable",         // R.6352-4 : garanties de procédure
		"se faire assister",            // R.6352-5
		"moins d'un jour franc",        // R.6352-6 : délai de notification
		"exclusion définitive",         // échelle des sanctions
		"réclamation",
	} {
		if !strings.Contains(source, exigence) {
			t.Errorf("le règlement ne couvre pas : %q", exigence)
		}
	}
}

// La représentation des stagiaires ne s'impose qu'aux formations de plus de
// cinq cents heures. La promettre à tout le monde ferait annoncer des élections
// qui n'auront jamais lieu — et un stagiaire qui les réclamerait aurait raison.
func TestStudentRepresentationOnlyForLongCourses(t *testing.T) {
	court := string(RenderReglement(sampleReglement()).Source)
	if strings.Contains(court, "délégué titulaire") {
		t.Error("la représentation des stagiaires apparaît sans formation longue")
	}

	long := sampleReglement()
	long.LongCourses = true
	if !strings.Contains(string(RenderReglement(long).Source), "délégué titulaire") {
		t.Error("la représentation des stagiaires manque sur une formation longue")
	}
}

// Le règlement doit se compiler : un gabarit qui casse ne se découvre pas au
// moment où un stagiaire le réclame.
func TestReglementCompiles(t *testing.T) {
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("binaire typst absent")
	}
	compiler, err := doc.NewBinaryCompiler()
	if err != nil {
		t.Fatalf("compilateur: %v", err)
	}

	long := sampleReglement()
	long.LongCourses = true
	for nom, r := range map[string]Reglement{"court": sampleReglement(), "long": long} {
		pdf, err := compiler.Compile(context.Background(), RenderReglement(r))
		if err != nil {
			t.Fatalf("compilation (%s): %v", nom, err)
		}
		if len(pdf) < 1000 {
			t.Errorf("PDF suspect (%s) : %d octets", nom, len(pdf))
		}
	}
}
