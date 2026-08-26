// Package emailtpl porte les gabarits des courriels transactionnels.
//
// Ils sont modifiables par l'équipe lemlearn sans redéploiement : une formule
// maladroite dans un message envoyé à des signataires ne doit pas attendre la
// prochaine version pour être corrigée. Les valeurs par défaut restent dans le
// code — ce sont elles qui servent tant que personne n'a rien changé, et c'est
// vers elles qu'on revient.
package emailtpl

import (
	"bytes"
	"fmt"
	"html/template"
	"sort"
	"strings"
)

// logoVariable est disponible dans tous les gabarits : le service l'injecte
// sans que l'appelant ait à y penser.
var logoVariable = Variable{
	Name:    "LogoURL",
	Purpose: "logo lemlearn, servi par l'application",
	Sample:  "https://app.lemlearn.fr/brand/lemlearn-courriel.png",
}

// Variable documente un champ disponible dans un gabarit.
type Variable struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
	Sample  string `json:"sample"`
}

// Definition est un gabarit et ce qu'il sait faire.
type Definition struct {
	Key       string     `json:"key"`
	Label     string     `json:"label"`
	Purpose   string     `json:"purpose"`
	Subject   string     `json:"subject"`
	Body      string     `json:"body"`
	Variables []Variable `json:"variables"`
}

// Sample compose un jeu de valeurs de démonstration.
//
// Il sert à prévisualiser, mais surtout à valider : un gabarit qu'on ne peut
// pas rendre avec ces valeurs ne sera pas enregistré, plutôt que de casser un
// envoi réel des semaines plus tard.
func (d Definition) Sample() map[string]any {
	data := make(map[string]any, len(d.Variables))
	for _, variable := range d.Variables {
		data[variable.Name] = variable.Sample
	}
	return data
}

// Clés des gabarits. Elles sont stables : elles servent d'identifiant en base.
const (
	KeySignatureInvitation = "signature.invitation"
	KeySignatureOTP        = "signature.otp"
	KeyLearnerInvitation   = "learner.invitation"
	KeySurveyCold          = "survey.cold"
)

// shell est l'habillage commun.
//
// Sobre et sans image, à dessein : un message de signature doit ressembler à
// un acte administratif, pas à une infolettre. Les messageries d'entreprise
// dégradent de toute façon la plupart des mises en forme, et un lien qui ne
// s'affiche pas est un dossier bloqué.
const shell = `<!doctype html>
<html lang="fr"><body style="margin:0;padding:24px;background:#f5f6f8;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;color:#10131a">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0"><tr><td align="center">
<table role="presentation" width="100%%" style="max-width:520px;background:#ffffff;border:1px solid #e3e6ec;border-radius:12px" cellpadding="0" cellspacing="0">
<tr><td style="padding:24px 28px 0">
<img src="{{.LogoURL}}" alt="lemlearn" width="100" height="20" style="display:block;border:0;height:20px;width:100px">
</td></tr>
<tr><td style="padding:20px 28px 8px">%s</td></tr>
<tr><td style="padding:0 28px 28px;border-top:1px solid #eef0f4">
<p style="margin:16px 0 0;font-size:11px;line-height:1.5;color:#8a90a0">%s</p>
</td></tr>
</table>
</td></tr></table>
</body></html>`

func wrap(body, footer string) string { return fmt.Sprintf(shell, body, footer) }

// Defaults renvoie les gabarits livrés avec le produit.
func Defaults() []Definition {
	return []Definition{
		{
			Key:     KeySignatureInvitation,
			Label:   "Document à signer",
			Purpose: "Part à l'émission d'une demande de signature. C'est le premier contact avec un signataire qui n'a pas de compte.",
			Subject: "Document à signer — {{.Reference}}",
			Variables: []Variable{
				logoVariable,
				{"SignerName", "prénom du signataire", "Léa"},
				{"Reference", "référence du document", "CONV-2026-0143"},
				{"DocumentLabel", "libellé complet du document", "convention de formation CONV-2026-0143"},
				{"Link", "lien personnel de signature", "https://app.lemlearn.fr/signer/jeton"},
				{"Deadline", "date d'expiration du lien", "26/08/2026"},
			},
			Body: wrap(`
<p style="margin:0 0 6px;font-size:12px;color:#8a90a0">{{.Reference}}</p>
<h1 style="margin:0 0 16px;font-size:19px;font-weight:600;letter-spacing:-0.02em">Un document vous attend</h1>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">Bonjour {{.SignerName}},</p>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">
Vous êtes invité à signer électroniquement le document <strong>{{.DocumentLabel}}</strong>.
Vous pourrez le lire intégralement avant de signer ; un code de vérification
vous sera envoyé à cette même adresse au moment de la signature.
</p>
<p style="margin:0 0 24px">
<a href="{{.Link}}" style="display:inline-block;background:#4b37b8;color:#ffffff;text-decoration:none;padding:11px 20px;border-radius:8px;font-size:14px;font-weight:500">Lire et signer le document</a>
</p>
<p style="margin:0;font-size:12px;line-height:1.6;color:#5b6170">
Ce lien est personnel, à usage unique, et expire le {{.Deadline}}.
</p>`,
				"Ce message vous est adressé par lemlearn pour le compte de l'organisme de formation à l'origine du document. "+
					"Si vous n'êtes pas concerné, ignorez-le : sans signature, le lien expirera de lui-même."),
		},
		{
			Key:     KeySignatureOTP,
			Label:   "Code de signature",
			Purpose: "Transmet le code à six chiffres au moment de signer. C'est lui qui fait la valeur juridique de la signature : il prouve que le signataire tient l'adresse.",
			Subject: "Votre code de signature : {{.Code}}",
			Variables: []Variable{
				logoVariable,
				{"Code", "code à six chiffres", "482095"},
				{"Reference", "référence du document", "CONV-2026-0143"},
			},
			Body: wrap(`
<p style="margin:0 0 6px;font-size:12px;color:#8a90a0">{{.Reference}}</p>
<h1 style="margin:0 0 16px;font-size:19px;font-weight:600;letter-spacing:-0.02em">Votre code de signature</h1>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">
Saisissez ce code dans la page de signature pour confirmer votre identité :
</p>
<p style="margin:0 0 20px;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:30px;font-weight:600;letter-spacing:0.18em">{{.Code}}</p>
<p style="margin:0;font-size:12px;line-height:1.6;color:#5b6170">
Ce code est valable dix minutes et ne peut servir qu'une fois.
</p>`,
				"Vous n'êtes pas à l'origine de cette demande ? Ne communiquez ce code à personne et ignorez ce message : "+
					"sans saisie, aucune signature ne sera apposée."),
		},
		{
			Key:     KeyLearnerInvitation,
			Label:   "Accès à l'espace apprenant",
			Purpose: "Ouvre l'accès d'un apprenant à ses modules. Envoyé par l'organisme depuis la fiche du contact.",
			Subject: "Votre espace de formation — {{.OrgName}}",
			Variables: []Variable{
				logoVariable,
				{"FirstName", "prénom de l'apprenant", "Léa"},
				{"OrgName", "nom de l'organisme", "Institut Vulcain"},
				{"Link", "lien pour choisir son mot de passe", "https://app.lemlearn.fr/invitation/jeton"},
			},
			Body: wrap(`
<h1 style="margin:0 0 16px;font-size:19px;font-weight:600;letter-spacing:-0.02em">Votre espace de formation</h1>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">Bonjour {{.FirstName}},</p>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">
<strong>{{.OrgName}}</strong> vous ouvre l'accès à votre espace. Vous y trouverez vos
modules vidéo, les questionnaires qui les accompagnent et votre progression —
c'est aussi là que votre attestation deviendra disponible.
</p>
<p style="margin:0 0 24px">
<a href="{{.Link}}" style="display:inline-block;background:#4b37b8;color:#ffffff;text-decoration:none;padding:11px 20px;border-radius:8px;font-size:14px;font-weight:500">Choisir mon mot de passe</a>
</p>
<p style="margin:0;font-size:12px;line-height:1.6;color:#5b6170">
Ce lien est personnel et expire dans quatorze jours.
</p>`,
				"Votre temps de visionnage est enregistré : il constitue la preuve d'assiduité que votre organisme "+
					"doit pouvoir présenter en cas de contrôle."),
		},
		{
			Key:     KeySurveyCold,
			Label:   "Satisfaction à froid",
			Purpose: "Part trois mois après la fin de la session. Le taux de retour est un indicateur audité : la formulation compte.",
			Subject: "Votre avis sur « {{.CourseTitle}} », trois mois après",
			Variables: []Variable{
				logoVariable,
				{"FirstName", "prénom de l'apprenant", "Camille"},
				{"CourseTitle", "intitulé de la formation", "Prévention des risques"},
				{"Link", "lien vers le questionnaire", "https://app.lemlearn.fr/satisfaction/jeton"},
			},
			Body: wrap(`
<h1 style="margin:0 0 16px;font-size:19px;font-weight:600;letter-spacing:-0.02em">Trois mois après, qu'en reste-t-il&nbsp;?</h1>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">Bonjour {{.FirstName}},</p>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">
Vous avez suivi la formation <strong>{{.CourseTitle}}</strong> il y a trois mois. Nous vous
sollicitons maintenant, et pas à chaud, parce que c'est avec ce recul qu'on
sait ce qui a réellement servi.
</p>
<p style="margin:0 0 24px;font-size:14px;line-height:1.6">Cinq questions, deux minutes.</p>
<p style="margin:0 0 24px">
<a href="{{.Link}}" style="display:inline-block;background:#4b37b8;color:#ffffff;text-decoration:none;padding:11px 20px;border-radius:8px;font-size:14px;font-weight:500">Répondre au questionnaire</a>
</p>`,
				"Vos réponses sont conservées avec votre dossier de formation et peuvent être consultées "+
					"lors d'un contrôle de l'organisme."),
		},
	}
}

// DefaultFor renvoie le gabarit d'origine d'une clé.
func DefaultFor(key string) (Definition, bool) {
	for _, definition := range Defaults() {
		if definition.Key == key {
			return definition, true
		}
	}
	return Definition{}, false
}

// Render exécute un sujet et un corps avec les valeurs fournies.
//
// html/template échappe les valeurs : un nom d'apprenant contenant un chevron
// ne peut pas injecter de balise dans un message envoyé à un tiers.
func Render(subject, body string, data map[string]any) (string, string, error) {
	renderedSubject, err := execute("sujet", subject, data)
	if err != nil {
		return "", "", err
	}
	renderedBody, err := execute("corps", body, data)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(renderedSubject), renderedBody, nil
}

func execute(name, source string, data map[string]any) (string, error) {
	parsed, err := template.New(name).Option("missingkey=zero").Parse(source)
	if err != nil {
		return "", fmt.Errorf("gabarit (%s) : %w", name, err)
	}
	var out bytes.Buffer
	if err := parsed.Execute(&out, data); err != nil {
		return "", fmt.Errorf("gabarit (%s) : %w", name, err)
	}
	return out.String(), nil
}

// Keys renvoie les clés connues, triées.
func Keys() []string {
	definitions := Defaults()
	keys := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		keys = append(keys, definition.Key)
	}
	sort.Strings(keys)
	return keys
}
