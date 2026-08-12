package signature

import (
	"fmt"
	"html"
	"strings"
)

// Les courriels sont volontairement sobres et sans image : un message de
// signature doit ressembler à un acte administratif, pas à une infolettre.
// Les clients de messagerie d'entreprise dégradent de toute façon la plupart
// des mises en forme, et un lien qui ne s'affiche pas est un dossier bloqué.

const emailShell = `<!doctype html>
<html lang="fr"><body style="margin:0;padding:24px;background:#f5f6f8;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;color:#10131a">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0"><tr><td align="center">
<table role="presentation" width="100%%" style="max-width:520px;background:#ffffff;border:1px solid #e3e6ec;border-radius:12px" cellpadding="0" cellspacing="0">
<tr><td style="padding:28px 28px 8px">%s</td></tr>
<tr><td style="padding:0 28px 28px;border-top:1px solid #eef0f4">
<p style="margin:16px 0 0;font-size:11px;line-height:1.5;color:#8a90a0">%s</p>
</td></tr>
</table>
</td></tr></table>
</body></html>`

// invitationEmail annonce un document à signer.
func invitationEmail(req Request, link string) string {
	body := fmt.Sprintf(`
<p style="margin:0 0 6px;font-size:12px;color:#8a90a0">%s</p>
<h1 style="margin:0 0 16px;font-size:19px;font-weight:600;letter-spacing:-0.02em">Un document vous attend</h1>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">Bonjour %s,</p>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">
Vous êtes invité à signer électroniquement le document <strong>%s</strong>.
Vous pourrez le lire intégralement avant de signer ; un code de vérification
vous sera envoyé à cette même adresse au moment de la signature.
</p>
<p style="margin:0 0 24px">
<a href="%s" style="display:inline-block;background:#4b37b8;color:#ffffff;text-decoration:none;padding:11px 20px;border-radius:8px;font-size:14px;font-weight:500">Lire et signer le document</a>
</p>
<p style="margin:0;font-size:12px;line-height:1.6;color:#5b6170">
Ce lien est personnel, à usage unique, et expire le %s.
</p>`,
		html.EscapeString(req.Reference),
		html.EscapeString(firstName(req.SignerName)),
		html.EscapeString(documentLabel(req)),
		html.EscapeString(link),
		html.EscapeString(formatDeadline(req)),
	)

	return fmt.Sprintf(emailShell, body,
		"Ce message vous est adressé par lemlearn pour le compte de l'organisme de formation à l'origine du document. "+
			"Si vous n'êtes pas concerné, ignorez-le : sans signature, le lien expirera de lui-même.")
}

// otpEmail transmet le code de vérification.
func otpEmail(req Request, code string) string {
	body := fmt.Sprintf(`
<p style="margin:0 0 6px;font-size:12px;color:#8a90a0">%s</p>
<h1 style="margin:0 0 16px;font-size:19px;font-weight:600;letter-spacing:-0.02em">Votre code de signature</h1>
<p style="margin:0 0 20px;font-size:14px;line-height:1.6">
Saisissez ce code dans la page de signature pour confirmer votre identité :
</p>
<p style="margin:0 0 20px;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:30px;font-weight:600;letter-spacing:0.18em">%s</p>
<p style="margin:0;font-size:12px;line-height:1.6;color:#5b6170">
Ce code est valable dix minutes et ne peut servir qu'une fois.
</p>`,
		html.EscapeString(req.Reference),
		html.EscapeString(code),
	)

	return fmt.Sprintf(emailShell, body,
		"Vous n'êtes pas à l'origine de cette demande ? Ne communiquez ce code à personne et ignorez ce message : "+
			"sans saisie, aucune signature ne sera apposée.")
}

func documentLabel(req Request) string {
	switch req.Kind {
	case "convention":
		return "convention de formation " + req.Reference
	case "quote":
		return "devis " + req.Reference
	case "attendance":
		return "feuille d'émargement " + req.Reference
	default:
		return req.Reference
	}
}

func firstName(fullName string) string {
	if first, _, found := strings.Cut(strings.TrimSpace(fullName), " "); found {
		return first
	}
	return fullName
}

func formatDeadline(req Request) string {
	return fmt.Sprintf("%02d/%02d/%d", req.ExpiresAt.Day(), int(req.ExpiresAt.Month()), req.ExpiresAt.Year())
}
