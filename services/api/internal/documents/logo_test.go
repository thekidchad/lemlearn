package documents

import (
	"context"
	"os/exec"
	"testing"

	"github.com/lemlearn/api/internal/platform/doc"
)

// Le logo doit réellement atteindre le PDF.
//
// On ne peut pas le vérifier en comptant les images incorporées : Typst rend un
// SVG en vectoriel, pas en objet image. On compare donc deux compilations de la
// même source, avec et sans logo — si le document est identique à l'octet près,
// c'est que l'image n'y est pas entrée.
func TestLogoReachesTheRenderedDocument(t *testing.T) {
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("binaire typst absent")
	}
	compiler, err := doc.NewBinaryCompiler()
	if err != nil {
		t.Fatalf("compilateur: %v", err)
	}

	const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 32">` +
		`<rect width="160" height="32" rx="6" fill="#0A7C5A"/></svg>`

	sans, err := compiler.Compile(context.Background(), RenderConvention(sampleConvention()))
	if err != nil {
		t.Fatalf("compilation sans logo: %v", err)
	}

	avec := sampleConvention()
	avec.LogoAsset = "logo.svg"
	avec.LogoBytes = map[string][]byte{"logo.svg": []byte(svg)}
	rendu, err := compiler.Compile(context.Background(), RenderConvention(avec))
	if err != nil {
		t.Fatalf("compilation avec logo: %v", err)
	}

	if len(rendu) == len(sans) {
		t.Error("le document est de taille identique avec et sans logo : l'image n'y est pas entrée")
	}
}

// Un logo illisible ne doit jamais empêcher un document de se produire : c'est
// la raison sociale qui fait foi, pas l'image.
func TestDocumentRendersWithoutALogo(t *testing.T) {
	source := string(RenderConvention(sampleConvention()).Source)
	if !contains(source, "Institut Vulcain") {
		t.Error("sans logo, l'en-tête doit porter la raison sociale")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
