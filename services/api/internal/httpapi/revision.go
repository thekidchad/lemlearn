package httpapi

import "runtime/debug"

// revision renvoie la révision Git incluse par le compilateur dans les
// informations de build. Elle apparaît dans /health pour qu'on sache
// exactement quelle version répond, sans dépendre d'une variable injectée
// à la main au moment du déploiement.
func revision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "inconnue"
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			if len(setting.Value) > 12 {
				return setting.Value[:12]
			}
			return setting.Value
		}
	}
	return "inconnue"
}
