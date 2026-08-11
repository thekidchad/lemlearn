package doc

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Polices Geist en TTF *statiques*.
//
// Pourquoi statiques et non variables : Typst ne rend pas encore les polices
// variables de façon fiable (constat repris de khwiz, api/.../document/fonts.go).
// Pourquoi embarquées : le rendu ne doit dépendre d'aucune police système —
// une Lambda n'en a pratiquement aucune, et un document contractuel ne peut pas
// se permettre un repli silencieux vers une autre fonte.
//
// Ce sont les mêmes fichiers que ceux servis à l'écran par next/font
// (node_modules/geist/dist/fonts) : l'application et le PDF partagent donc
// exactement la même typographie. Geist est publiée par Vercel sous licence MIT.
//
//go:embed fonts/*.ttf
var fontFiles embed.FS

var (
	fontOnce sync.Once
	fontDir  string
	fontErr  error
)

// materializeFonts renvoie le répertoire à passer à `typst --font-path`.
//
// TYPST_FONT_PATH court-circuite l'extraction : en Lambda, le layer monte déjà
// les polices en lecture seule sous /opt/fonts, inutile de les recopier.
// Sinon les polices embarquées sont écrites une seule fois par processus —
// une Lambda réutilisée ne paie l'extraction qu'au premier appel.
func materializeFonts(fallbackDir string) (string, error) {
	if path := os.Getenv("TYPST_FONT_PATH"); path != "" {
		return path, nil
	}

	fontOnce.Do(func() {
		dir := filepath.Join(os.TempDir(), "lemlearn-fonts")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			fontErr = fmt.Errorf("répertoire des polices: %w", err)
			return
		}
		entries, err := fs.ReadDir(fontFiles, "fonts")
		if err != nil {
			fontErr = fmt.Errorf("lecture des polices embarquées: %w", err)
			return
		}
		for _, entry := range entries {
			content, err := fontFiles.ReadFile("fonts/" + entry.Name())
			if err != nil {
				fontErr = fmt.Errorf("police %s: %w", entry.Name(), err)
				return
			}
			target := filepath.Join(dir, entry.Name())
			if stat, err := os.Stat(target); err == nil && stat.Size() == int64(len(content)) {
				continue
			}
			if err := os.WriteFile(target, content, 0o600); err != nil {
				fontErr = fmt.Errorf("écriture de la police %s: %w", entry.Name(), err)
				return
			}
		}
		fontDir = dir
	})

	if fontErr != nil {
		return "", fontErr
	}
	if fontDir == "" {
		return filepath.Join(fallbackDir, "fonts"), nil
	}
	return fontDir, nil
}
