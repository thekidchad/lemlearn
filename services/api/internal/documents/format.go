package documents

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var frenchMonths = [...]string{
	"janvier", "février", "mars", "avril", "mai", "juin",
	"juillet", "août", "septembre", "octobre", "novembre", "décembre",
}

// formatDate rend une date en toutes lettres à la française : « 14 janvier 2026 ».
// Les documents contractuels ne s'écrivent pas en 14/01/2026, qui se lit
// différemment selon le pays du lecteur.
func formatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	day := strconv.Itoa(t.Day())
	if t.Day() == 1 {
		day = "1er"
	}
	return fmt.Sprintf("%s %s %d", day, frenchMonths[t.Month()-1], t.Year())
}

// formatHours rend une durée pédagogique : « 14 heures », « 3,5 heures ».
func formatHours(hours float64) string {
	if hours == 1 {
		return "1 heure"
	}
	return trimFloat(hours) + " heures"
}

// formatEUR rend un montant en euros avec espace insécable fine comme
// séparateur de milliers et virgule décimale, conformément à l'usage français.
func formatEUR(amount float64) string {
	// Le signe est mis de côté avant tout calcul. Sans cela, les centimes
	// d'un montant négatif ressortent négatifs — « 1 008,-50 € » — et le
	// groupement des milliers place une espace juste après le moins. Un avoir
	// ne porte que des montants négatifs : c'est le cas normal, pas un cas
	// limite.
	negatif := amount < 0
	if negatif {
		amount = -amount
	}

	whole := int64(amount)
	cents := int64((amount-float64(whole))*100 + 0.5)
	if cents == 100 {
		whole++
		cents = 0
	}

	digits := strconv.FormatInt(whole, 10)
	var grouped strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped.WriteRune(' ') // espace fine insécable
		}
		grouped.WriteRune(r)
	}
	signe := ""
	if negatif && (whole != 0 || cents != 0) {
		// Le moins typographique, pas le trait d'union : c'est celui qu'un
		// lecteur de PDF rend à la bonne chasse.
		signe = "−"
	}
	return fmt.Sprintf("%s%s,%02d €", signe, grouped.String(), cents)
}

// trimFloat affiche un nombre sans zéro décimal inutile, virgule à la française.
func trimFloat(value float64) string {
	s := strconv.FormatFloat(value, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	return strings.Replace(s, ".", ",", 1)
}

func orDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

// escapeParagraph rend un texte libre comme contenu Typst en neutralisant les
// caractères qui y ont une signification (#, *, _, @, $…). Sans cela, une
// description de formation contenant « #1 » ou « 100 $ » casserait la
// compilation, ou pire, injecterait du code.
func escapeParagraph(text string) string {
	var b strings.Builder
	b.Grow(len(text) + 8)
	for _, r := range text {
		switch r {
		case '#', '*', '_', '@', '$', '\\', '<', '>', '`', '[', ']':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(" \\\n")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
