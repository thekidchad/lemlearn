// Package brand porte l'identité visible d'un organisme de formation.
//
// lemlearn est vendu en marque blanche : l'apprenant qui suit une formation et
// le signataire qui appose sa signature ne doivent voir que l'organisme, pas
// l'outil. Ce n'est pas une coquetterie commerciale — un stagiaire qui reçoit
// un courriel au nom d'une société inconnue le classe en indésirable, et un
// financeur qui lit une convention veut y trouver l'organisme conventionné.
//
// La marque est donc une donnée de l'organisation, modifiable à chaud depuis
// l'application : ouvrir un nouveau client ne demande aucun déploiement.
// Seule la vue super-admin reste à l'enseigne de lemlearn.
package brand

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/lemlearn/api/internal/platform/ddb"
)

// SK est la clé de tri de la marque, sous la partition de l'organisation.
const SK = "BRAND"

// UploadTTL borne la validité d'une URL de dépôt de logo. Cinq minutes : le
// temps de choisir un fichier et de l'envoyer, pas celui d'oublier un lien
// signé dans un historique.
const UploadTTL = 5 * time.Minute

// Accent par défaut. C'est la teinte de lemlearn : un organisme qui n'a pas
// encore choisi la sienne obtient une interface cohérente, pas une interface
// grise.
const DefaultAccent = "#6644E8"

// Brand est l'identité d'un organisme.
type Brand struct {
	ddb.Record

	// Name est le nom affiché. Vide, on retombe sur la raison sociale de
	// l'organisation : un organisme n'a pas à ressaisir son propre nom pour
	// que l'application cesse d'afficher le nôtre.
	Name string `dynamodbav:"name,omitempty" json:"name,omitempty"`
	// LogoKey est la clé S3 du logo dans le compartiment public, sous le
	// préfixe `brand/`. On stocke la clé et non l'URL : le nom du
	// compartiment change d'un environnement à l'autre, l'objet non.
	LogoKey string `dynamodbav:"logoKey,omitempty" json:"logoKey,omitempty"`
	// Accent est la couleur d'accent, en hexadécimal. Une seule suffit :
	// l'interface n'en emploie qu'une, et en laisser choisir davantage
	// produirait surtout des combinaisons illisibles.
	Accent string `dynamodbav:"accent,omitempty" json:"accent,omitempty"`
	// SupportEmail est l'adresse à laquelle un apprenant répond. Sans elle,
	// les courriels renvoient vers le néant — et c'est le premier réflexe
	// d'un stagiaire perdu.
	SupportEmail string `dynamodbav:"supportEmail,omitempty" json:"supportEmail,omitempty"`
	// Theme est le thème par défaut : "system", "light" ou "dark". Il ne
	// s'impose pas — un apprenant qui a choisi le sien le garde — mais il
	// décide de ce que voit quelqu'un qui arrive pour la première fois, et un
	// organisme dont la charte est sombre ne veut pas d'une page blanche.
	Theme string `dynamodbav:"theme,omitempty" json:"theme,omitempty"`
	// Domain est le domaine propre de l'organisme, quand il en a un. Il n'est
	// pas encore servi, mais il est résolu : la marque se retrouve par
	// domaine autant que par session.
	Domain string `dynamodbav:"domain,omitempty" json:"domain,omitempty"`
	// UpdatedBy nomme l'auteur du dernier changement. L'identité visible par
	// les apprenants ne doit pas changer anonymement.
	UpdatedBy string `dynamodbav:"updatedBy,omitempty" json:"updatedBy,omitempty"`
}

// Public est la marque telle que la consomment l'application et les courriels.
//
// Elle est toujours complète : les valeurs par défaut sont résolues ici, une
// fois, plutôt que dans chaque gabarit et chaque écran. Un appelant n'a jamais
// à se demander quoi afficher quand un champ est vide.
type Public struct {
	Name string `json:"name"`
	// LogoURL est vide quand l'organisme n'a pas déposé de logo. L'appelant
	// affiche alors le monogramme, qui n'est pas un pis-aller : deux lettres
	// dans la couleur de l'organisme valent mieux qu'un logo emprunté.
	LogoURL  string `json:"logoUrl,omitempty"`
	Monogram string `json:"monogram"`
	Accent   string `json:"accent"`
	// AccentInk est la couleur du texte posé sur l'accent, noire ou blanche
	// selon la luminance. Calculée ici : la laisser au front produirait du
	// texte blanc sur jaune le jour où un organisme choisira du jaune.
	AccentInk    string `json:"accentInk"`
	SupportEmail string `json:"supportEmail,omitempty"`
	// Theme vaut toujours l'une des trois valeurs : l'appelant n'a pas à
	// traiter le cas vide.
	Theme string `json:"theme"`
}

// Thèmes acceptés. "system" suit le réglage du système d'exploitation du
// visiteur, ce qui reste le meilleur défaut : il correspond à ce qu'il a déjà
// choisi pour tout le reste.
var themes = map[string]bool{"system": true, "light": true, "dark": true}

// MailData projette la marque dans les champs attendus par les gabarits de
// courriel. Les noms sont ceux du gabarit : les composer ici évite qu'ils
// divergent entre trois appelants, et qu'un message parte sans enseigne parce
// qu'un champ a été mal orthographié.
func (p Public) MailData() map[string]any {
	return map[string]any{
		"LogoURL":     p.LogoURL,
		"BrandName":   p.Name,
		"BrandAccent": p.Accent,
		"BrandInk":    p.AccentInk,
	}
}

// hexColor n'accepte que la forme à six chiffres. Les formes courtes et les
// noms CSS passeraient dans une variable de style, mais pas dans un PDF ni
// dans un courriel, où la couleur est composée à la main.
var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Validate refuse ce qui casserait un rendu ailleurs.
func (b Brand) Validate() error {
	if len(b.Name) > 60 {
		return fmt.Errorf("brand: le nom dépasse 60 caractères")
	}
	if b.Accent != "" && !hexColor.MatchString(b.Accent) {
		return fmt.Errorf("brand: la couleur doit être de la forme #RRGGBB")
	}
	if b.SupportEmail != "" && !strings.Contains(b.SupportEmail, "@") {
		return fmt.Errorf("brand: adresse de contact invalide")
	}
	if b.Theme != "" && !themes[b.Theme] {
		return fmt.Errorf("brand: le thème doit être system, light ou dark")
	}
	if b.Domain != "" && (strings.Contains(b.Domain, "/") || strings.Contains(b.Domain, " ")) {
		return fmt.Errorf("brand: le domaine ne doit être qu'un nom d'hôte")
	}
	return nil
}

// Resolve complète la marque avec le nom de l'organisation et l'adresse des
// ressources publiques.
func (b Brand) Resolve(orgName, assetsURL string) Public {
	name := strings.TrimSpace(b.Name)
	if name == "" {
		name = orgName
	}
	accent := b.Accent
	if accent == "" {
		accent = DefaultAccent
	}
	logo := ""
	if b.LogoKey != "" && assetsURL != "" {
		logo = strings.TrimSuffix(assetsURL, "/") + "/" + strings.TrimPrefix(b.LogoKey, "/")
	}
	theme := b.Theme
	if !themes[theme] {
		theme = "system"
	}
	return Public{
		Name:         name,
		Theme:        theme,
		LogoURL:      logo,
		Monogram:     Monogram(name),
		Accent:       accent,
		AccentInk:    inkOn(accent),
		SupportEmail: b.SupportEmail,
	}
}

// Monogram tire deux lettres du nom.
//
// Deux mots donnent leurs initiales, un seul mot ses deux premières lettres.
// On ignore les mots de liaison, sans quoi « Institut de la Formation »
// deviendrait « ID » — juste, et illisible.
func Monogram(name string) string {
	liaison := map[string]bool{
		"de": true, "du": true, "des": true, "la": true, "le": true,
		"les": true, "et": true, "d": true, "l": true, "en": true,
	}
	var mots []string
	for _, mot := range strings.FieldsFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if !liaison[strings.ToLower(mot)] {
			mots = append(mots, mot)
		}
	}
	switch {
	case len(mots) == 0:
		return "OF"
	case len(mots) == 1:
		lettres := []rune(mots[0])
		if len(lettres) == 1 {
			return strings.ToUpper(string(lettres[0]))
		}
		return strings.ToUpper(string(lettres[0:2]))
	default:
		return strings.ToUpper(string([]rune(mots[0])[0:1]) + string([]rune(mots[1])[0:1]))
	}
}

// inkOn choisit le noir ou le blanc sur une couleur de fond.
//
// Le seuil porte sur la luminance perçue, pas sur la moyenne des composantes :
// l'œil est bien plus sensible au vert qu'au bleu, et une moyenne simple
// mettrait du texte blanc sur un vert vif.
func inkOn(hex string) string {
	if !hexColor.MatchString(hex) {
		return "#FFFFFF"
	}
	var r, g, b int
	fmt.Sscanf(strings.ToUpper(hex), "#%02X%02X%02X", &r, &g, &b)
	luminance := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 255
	if luminance > 0.62 {
		return "#10131A"
	}
	return "#FFFFFF"
}

// LogoKey compose la clé S3 du logo d'un organisme.
//
// L'extension fait partie de la clé : servi depuis S3, le type de contenu est
// celui posé au dépôt, et un nom parlant aide à relire un compartiment à la
// main. L'identifiant d'organisation isole les organismes entre eux.
func LogoKey(orgID, ext string) string {
	return "brand/" + orgID + "/logo" + ext
}
