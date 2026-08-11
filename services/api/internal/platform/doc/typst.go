package doc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// SigZoneLabel est l'étiquette de métadonnée posée par l'aide `lem_sig_mark`.
// `typst query` sur cette étiquette renvoie une entrée par zone déclarée.
const SigZoneLabel = "<sig-zone>"

// BinaryCompiler appelle le binaire Typst.
//
// En Lambda, le binaire et les polices sont montés par un layer (/opt/bin/typst,
// /opt/fonts) et TYPST_PATH pointe dessus. En local, la découverte par PATH
// suffit (`brew install typst`).
type BinaryCompiler struct {
	path string
}

// NewBinaryCompiler localise le binaire Typst.
func NewBinaryCompiler() (*BinaryCompiler, error) {
	path := strings.TrimSpace(os.Getenv("TYPST_PATH"))
	if path == "" {
		found, err := exec.LookPath("typst")
		if err != nil {
			return nil, fmt.Errorf("binaire typst introuvable (installez-le ou renseignez TYPST_PATH): %w", err)
		}
		path = found
	}
	return &BinaryCompiler{path: path}, nil
}

// Compile renvoie le PDF produit par la source.
func (c *BinaryCompiler) Compile(ctx context.Context, d Document) ([]byte, error) {
	pdf, _, err := c.compile(ctx, d, false)
	return pdf, err
}

// CompileWithZones compile ET extrait les zones de signature déclarées par
// `lem_sig_zone` / `lem_mention_zone`.
//
// Un document sans aucune déclaration renvoie une tranche vide non nulle :
// « volontairement aucune zone », ce qui se distingue de nil, « zones
// inconnues » (cf. CompileWithZones au niveau paquet).
func (c *BinaryCompiler) CompileWithZones(ctx context.Context, d Document) ([]byte, []SignatureZone, error) {
	return c.compile(ctx, d, true)
}

// zoneMeta reflète le dictionnaire émis par lem_sig_mark côté Typst.
type zoneMeta struct {
	Role string  `json:"role"`
	Kind string  `json:"kind"`
	Page int     `json:"page"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
}

func (c *BinaryCompiler) compile(ctx context.Context, d Document, withZones bool) ([]byte, []SignatureZone, error) {
	if c == nil || strings.TrimSpace(c.path) == "" {
		return nil, nil, fmt.Errorf("compilateur typst: chemin du binaire indisponible")
	}
	if len(d.Source) == 0 {
		return nil, nil, fmt.Errorf("compilateur typst: source vide")
	}

	// Les fichiers temporaires contiennent des données personnelles
	// (identité de l'apprenant, adresse, montants) : répertoire privé,
	// permissions restreintes, suppression garantie au retour.
	dir, err := os.MkdirTemp("", "lemlearn-typst-*")
	if err != nil {
		return nil, nil, fmt.Errorf("répertoire temporaire: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	sourcePath := filepath.Join(dir, "document.typ")
	pdfPath := filepath.Join(dir, "document.pdf")
	if err := os.WriteFile(sourcePath, d.Source, 0o600); err != nil {
		return nil, nil, fmt.Errorf("écriture de la source: %w", err)
	}
	if err := writeAssets(dir, d.Assets); err != nil {
		return nil, nil, err
	}

	fontPath, err := materializeFonts(dir)
	if err != nil {
		return nil, nil, err
	}

	env := os.Environ()
	if d.CreationUnix > 0 {
		// Rend la compilation reproductible : même source + même date =
		// mêmes octets, donc même empreinte SHA-256.
		env = append(env, "SOURCE_DATE_EPOCH="+strconv.FormatInt(d.CreationUnix, 10))
	}

	cmd := exec.CommandContext(ctx, c.path, "compile", "--font-path", fontPath, sourcePath, pdfPath)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, nil, fmt.Errorf("compilation typst: %w: %s", err, strings.TrimSpace(string(out)))
	}

	pdf, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, nil, fmt.Errorf("lecture du pdf: %w", err)
	}
	if len(pdf) == 0 {
		return nil, nil, fmt.Errorf("compilation typst: pdf vide")
	}
	if !withZones {
		return pdf, nil, nil
	}

	zones, err := c.queryZones(ctx, sourcePath, fontPath, env)
	if err != nil {
		return nil, nil, err
	}
	return pdf, zones, nil
}

// queryZones relance Typst en mode `query` pour lire les métadonnées de zones.
//
// Le même --font-path que la compilation est obligatoire : une résolution de
// police différente décalerait la mise en page, donc les positions extraites.
func (c *BinaryCompiler) queryZones(ctx context.Context, sourcePath, fontPath string, env []string) ([]SignatureZone, error) {
	cmd := exec.CommandContext(ctx, c.path,
		"query", "--font-path", fontPath, sourcePath, SigZoneLabel,
		"--field", "value", "--format", "json")
	cmd.Env = env

	// Stdout seul : Typst écrit ses avertissements (polices, dépréciations)
	// sur stderr, et les mélanger au flux casse le décodage JSON.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("typst query des zones: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var metas []zoneMeta
	if err := json.Unmarshal(out, &metas); err != nil {
		return nil, fmt.Errorf("décodage des zones: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	zones := make([]SignatureZone, 0, len(metas))
	for _, m := range metas {
		zone := SignatureZone{
			Role:   SignatureZoneRole(m.Role),
			Kind:   SignatureZoneKind(m.Kind),
			Page:   m.Page,
			X:      m.X,
			Y:      m.Y,
			Width:  m.W,
			Height: m.H,
		}
		if err := zone.Validate(); err != nil {
			return nil, err
		}
		zones = append(zones, zone)
	}
	return zones, nil
}

func writeAssets(dir string, assets map[string][]byte) error {
	for name, content := range assets {
		if len(content) == 0 {
			continue
		}
		path := filepath.Join(dir, filepath.Clean(name))
		if !strings.HasPrefix(path, dir+string(os.PathSeparator)) {
			return fmt.Errorf("la ressource %q sort du répertoire temporaire", name)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("répertoire de la ressource %s: %w", name, err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			return fmt.Errorf("écriture de la ressource %s: %w", name, err)
		}
	}
	return nil
}
