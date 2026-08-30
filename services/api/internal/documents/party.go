package documents

import (
	"strings"

	"github.com/lemlearn/api/internal/identity"
)

// PartyFromOrg projette l'organisation dans la partie contractante.
//
// La conversion vit ici, et une seule fois, parce qu'elle était faite à trois
// endroits qui ne recopiaient pas les mêmes champs : la convention portait
// l'identité complète tandis que l'attestation et les relevés — produits par
// l'export — s'arrêtaient au SIRET. Deux pièces du même dossier affichaient
// donc deux pieds de page différents, et celle que lit un financeur était la
// moins complète.
//
// Un champ ajouté à l'identité juridique atteint désormais tous les documents,
// et pas seulement celui auquel on pensait ce jour-là.
func PartyFromOrg(org identity.Org) Party {
	return Party{
		Name: org.Name, Address: org.Address,
		PostalCode: org.PostalCode, City: org.City, SIRET: org.SIRET,
		LegalForm: org.LegalForm, Capital: org.Capital, RCS: org.RCS,
		VATNumber: org.VATNumber, VATExempt: org.VATExempt,
		NDA: org.NDA, NDARegion: org.NDARegion,
		Represented: org.RepName, Role: org.RepRole,
	}
}

// LogoAsset est le nom sous lequel le logo est déposé dans les ressources d'un
// document. L'extension suit celle de la clé : Typst choisit son décodeur
// d'après elle, et un SVG nommé .png ne s'affiche pas.
func LogoAsset(key string) string {
	if key == "" {
		return ""
	}
	ext := key[strings.LastIndex(key, "."):]
	return "logo" + ext
}
