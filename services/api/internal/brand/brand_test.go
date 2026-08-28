package brand

import "testing"

func TestMonogram(t *testing.T) {
	cas := []struct {
		nom    string
		attend string
	}{
		{"Vulcain Formation", "VF"},
		{"Institut de la Formation", "IF"},
		{"Aubépine", "AU"},
		{"L'Atelier des Métiers", "AM"},
		{"3IL", "3I"},
		{"", "OF"},
		{"X", "X"},
	}
	for _, c := range cas {
		if got := Monogram(c.nom); got != c.attend {
			t.Errorf("Monogram(%q) = %q, attendu %q", c.nom, got, c.attend)
		}
	}
}

// Un organisme qui choisit une couleur claire doit obtenir du texte sombre.
// C'est la seule règle qui rende le monogramme lisible quel que soit le choix.
func TestInkOn(t *testing.T) {
	cas := map[string]string{
		"#6644E8": "#FFFFFF", // violet foncé
		"#0B0C0F": "#FFFFFF", // presque noir
		"#FFE066": "#10131A", // jaune clair
		"#FFFFFF": "#10131A",
		"pas une couleur": "#FFFFFF",
	}
	for couleur, attend := range cas {
		if got := inkOn(couleur); got != attend {
			t.Errorf("inkOn(%q) = %q, attendu %q", couleur, got, attend)
		}
	}
}

// Une marque vide doit produire une identité complète : c'est l'état d'un
// organisme qui vient d'ouvrir son compte, et l'application doit s'afficher.
func TestResolveRetombeSurOrganisation(t *testing.T) {
	got := Brand{}.Resolve("Vulcain Formation", "https://assets.test")
	if got.Name != "Vulcain Formation" {
		t.Errorf("nom = %q", got.Name)
	}
	if got.Accent != DefaultAccent {
		t.Errorf("accent = %q", got.Accent)
	}
	if got.Monogram != "VF" {
		t.Errorf("monogramme = %q", got.Monogram)
	}
	if got.LogoURL != "" {
		t.Errorf("logo = %q, attendu vide sans dépôt", got.LogoURL)
	}
}

func TestResolveComposeLAdresseDuLogo(t *testing.T) {
	b := Brand{Name: "Aubépine", LogoKey: LogoKey("ORG1", ".png"), Accent: "#FFE066"}
	got := b.Resolve("Raison sociale ignorée", "https://assets.test/")
	if got.LogoURL != "https://assets.test/brand/ORG1/logo.png" {
		t.Errorf("logo = %q", got.LogoURL)
	}
	if got.Name != "Aubépine" {
		t.Errorf("le nom choisi doit primer sur la raison sociale, obtenu %q", got.Name)
	}
	if got.AccentInk != "#10131A" {
		t.Errorf("encre = %q sur un accent clair", got.AccentInk)
	}
}

// Sans adresse de ressources, aucune URL n'est composée : mieux vaut un
// monogramme qu'une image cassée dans un courriel.
func TestResolveSansRessources(t *testing.T) {
	b := Brand{LogoKey: LogoKey("ORG1", ".png")}
	if got := b.Resolve("Vulcain", "").LogoURL; got != "" {
		t.Errorf("logo = %q, attendu vide", got)
	}
}

func TestValidate(t *testing.T) {
	if err := (Brand{Accent: "#fff"}).Validate(); err == nil {
		t.Error("une couleur à trois chiffres devrait être refusée")
	}
	if err := (Brand{Accent: "#6644E8"}).Validate(); err != nil {
		t.Errorf("couleur valide refusée : %v", err)
	}
	if err := (Brand{SupportEmail: "pas-une-adresse"}).Validate(); err == nil {
		t.Error("une adresse sans arobase devrait être refusée")
	}
	if err := (Brand{Domain: "https://exemple.fr/x"}).Validate(); err == nil {
		t.Error("un domaine avec chemin devrait être refusé")
	}
}
