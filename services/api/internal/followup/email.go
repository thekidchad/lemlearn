package followup

import (
	"fmt"
	"html"
	"strings"
)

// coldSurveyEmail compose la relance.
//
// Elle dit pourquoi on écrit trois mois après : sans cette phrase, le message
// ressemble à une sollicitation commerciale et finit en indésirable, ce qui
// ruine le taux de retour — l'indicateur même que l'on cherche à produire.
func coldSurveyEmail(task Task, link string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="fr"><body style="margin:0;padding:24px;background:#f5f6f8;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;color:#10131a">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0"><tr><td align="center">
<table role="presentation" width="100%%" style="max-width:520px;background:#ffffff;border:1px solid #e3e6ec;border-radius:12px" cellpadding="0" cellspacing="0">
<tr><td style="padding:28px">
<h1 style="margin:0 0 16px;font-size:19px;font-weight:600;letter-spacing:-0.02em">Trois mois après, qu'en reste-t-il&nbsp;?</h1>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">Bonjour %s,</p>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">
Vous avez suivi la formation <strong>%s</strong> il y a trois mois. Nous vous
sollicitons maintenant, et pas à chaud, parce que c'est avec ce recul qu'on
sait ce qui a réellement servi.
</p>
<p style="margin:0 0 24px;font-size:14px;line-height:1.6">Cinq questions, deux minutes.</p>
<p style="margin:0 0 24px">
<a href="%s" style="display:inline-block;background:#4b37b8;color:#ffffff;text-decoration:none;padding:11px 20px;border-radius:8px;font-size:14px;font-weight:500">Répondre au questionnaire</a>
</p>
<p style="margin:0;font-size:12px;line-height:1.6;color:#5b6170">
Vos réponses sont conservées avec votre dossier de formation et peuvent être
consultées lors d'un contrôle de l'organisme.
</p>
</td></tr></table>
</td></tr></table>
</body></html>`,
		html.EscapeString(firstName(task.LearnerName)),
		html.EscapeString(task.CourseTitle),
		html.EscapeString(link))
}

func firstName(fullName string) string {
	if first, _, found := strings.Cut(strings.TrimSpace(fullName), " "); found {
		return first
	}
	return fullName
}
