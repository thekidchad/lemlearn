package httpapi

import (
	"net/http"
	"strconv"
)

// Pagination des listes, par curseur.
//
// DynamoDB ne sait pas sauter à la septième page : il n'existe pas d'offset,
// parce qu'atteindre le millième élément supposerait de lire les neuf cent
// quatre-vingt-dix-neuf premiers. Ce qu'il rend est la clé du dernier élément
// lu, à représenter au tour suivant.
//
// Le contrat est donc le même sur toutes les listes : `?limite=&curseur=`, et
// une réponse qui porte `cursor` quand il reste quelque chose. Un écran ne peut
// pas afficher « page 3 sur 12 » — le total n'est pas connu sans tout compter,
// et l'annoncer coûterait précisément ce qu'on évite.

const (
	// defaultPageSize tient sur un écran sans faire défiler à l'infini.
	defaultPageSize = 25
	// maxPageSize borne ce qu'un appelant peut demander. Sans plafond, un
	// client curieux ramènerait la table entière en une requête.
	maxPageSize = 100
)

// pageParams lit la taille et le curseur d'une requête.
//
// Une limite absente ou aberrante retombe sur la valeur par défaut plutôt que
// d'échouer : une liste qui refuse de s'afficher parce qu'un paramètre est mal
// formé est plus gênante qu'une liste de vingt-cinq éléments.
func pageParams(r *http.Request) (int32, string) {
	limite := int32(defaultPageSize)
	if brut := r.URL.Query().Get("limite"); brut != "" {
		if valeur, err := strconv.Atoi(brut); err == nil && valeur > 0 {
			if valeur > maxPageSize {
				valeur = maxPageSize
			}
			limite = int32(valeur)
		}
	}
	return limite, r.URL.Query().Get("curseur")
}
