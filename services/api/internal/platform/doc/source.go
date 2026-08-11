package doc

import (
	"fmt"
	"strings"
)

// Source construit une source Typst ligne par ligne.
//
// Les gabarits sont écrits en Go plutôt que dans des fichiers .typ séparés :
// les données viennent de structures typées, et un gabarit interprété
// laisserait passer des erreurs de champ jusqu'au PDF signé.
type Source struct {
	buf strings.Builder
}

// Line écrit une ligne telle quelle.
func (s *Source) Line(line string) {
	s.buf.WriteString(line)
	s.buf.WriteByte('\n')
}

// Linef écrit une ligne formatée. Toute valeur issue de la base doit passer
// par Str, jamais être interpolée directement.
func (s *Source) Linef(format string, args ...any) {
	fmt.Fprintf(&s.buf, format, args...)
	s.buf.WriteByte('\n')
}

// Bytes renvoie la source compilable.
func (s *Source) Bytes() []byte {
	return []byte(s.buf.String())
}

// String renvoie la source sous forme de chaîne (tests, prévisualisation).
func (s *Source) String() string {
	return s.buf.String()
}

// Str encode une valeur en littéral chaîne Typst.
//
// C'est la seule barrière entre les données saisies par un utilisateur (nom
// d'apprenant, intitulé de formation, adresse) et le code compilé : un nom
// contenant une guillemet ou un antislash doit produire un document correct,
// pas une erreur de compilation ni une injection de code Typst.
func Str(value string) string {
	var b strings.Builder
	b.Grow(len(value) + 2)
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
