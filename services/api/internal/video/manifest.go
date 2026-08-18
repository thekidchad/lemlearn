package video

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// ManifestContentType est le type MIME d'un manifeste HLS.
const ManifestContentType = "application/vnd.apple.mpegurl"

// renditionName borne ce qu'un client peut demander : un nom de fichier, dans
// le dossier de l'asset, et rien d'autre. Sans cette borne, « ../ » ferait de
// la route un lecteur universel du compartiment.
var renditionName = regexp.MustCompile(`^[A-Za-z0-9._-]+\.m3u8$`)

// Manifest renvoie un manifeste HLS dont toutes les URL sont absolues.
//
// Le manifeste est réécrit plutôt que servi tel quel, parce qu'un flux HLS est
// une arborescence : le manifeste principal renvoie vers trois sous-manifestes,
// qui renvoient chacun vers des dizaines de segments, tous en relatif. Signer
// la seule URL d'entrée donnerait un lecteur qui affiche la liste des qualités
// puis échoue en 403 au premier segment.
//
// Réécrire côté serveur plutôt que faire compléter les URL par le lecteur a une
// raison précise : Safari sur iPhone lit le HLS nativement, sans passer par
// JavaScript, et n'offre aucun moyen d'intercepter ses requêtes. Un produit qui
// ne se lit pas sur iPhone n'est pas un produit.
//
// Les sous-manifestes repassent par childURL — donc par nous — pour que leurs
// segments soient réécrits à leur tour ; les segments, eux, partent directement
// vers le CDN. Seuls quelques kilo-octets de texte transitent par l'API, jamais
// la vidéo.
func (s *Service) Manifest(ctx context.Context, orgID, assetID, rendition string,
	childURL func(name string) string) (string, error) {
	if s.signer == nil {
		return "", fmt.Errorf("la diffusion vidéo n'est pas configurée")
	}
	if s.blobs == nil {
		return "", fmt.Errorf("le dépôt vidéo n'est pas configuré")
	}

	asset, err := s.Get(ctx, orgID, assetID)
	if err != nil {
		return "", err
	}
	if asset.Status != StatusReady || asset.MasterKey == "" {
		return "", fmt.Errorf("cette vidéo n'est pas encore diffusable (%s)", asset.Status)
	}

	folder := path.Dir(asset.MasterKey) + "/"
	key := asset.MasterKey
	if rendition != "" {
		if !renditionName.MatchString(rendition) {
			return "", fmt.Errorf("rendu %q inattendu", rendition)
		}
		key = folder + rendition
	}

	raw, err := s.blobs.Get(ctx, key)
	if err != nil {
		return "", err
	}

	query, err := s.signer.SignPrefix(folder, PlaybackTTL)
	if err != nil {
		return "", err
	}

	rewrite := func(uri string) string {
		// Une URL déjà absolue n'est pas à nous : on n'y touche pas.
		if strings.Contains(uri, "://") {
			return uri
		}
		if strings.HasSuffix(uri, ".m3u8") && childURL != nil {
			return childURL(uri)
		}
		return s.signer.domain + "/" + folder + uri + "?" + query
	}

	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
		case strings.HasPrefix(trimmed, "#"):
			// Les pistes audio et les sous-titres déclarent leur URI dans
			// l'attribut d'une balise, pas sur une ligne à part.
			lines[i] = uriAttribute.ReplaceAllStringFunc(trimmed, func(match string) string {
				inner := match[len(`URI="`) : len(match)-1]
				return `URI="` + rewrite(inner) + `"`
			})
		default:
			lines[i] = rewrite(trimmed)
		}
	}

	return strings.Join(lines, "\n"), nil
}

var uriAttribute = regexp.MustCompile(`URI="[^"]*"`)
