package httpapi

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lemlearn/api/internal/brand"
)

// Presigner signe un dépôt direct vers un compartiment.
//
// L'interface est déclarée ici plutôt qu'importée du paquet de stockage : le
// routeur n'a besoin que de cette méthode, et la déclarer au point d'usage
// permet de la remplacer en test sans compte AWS.
type Presigner interface {
	PresignedPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
}

// Formats acceptés pour un logo.
//
// Trois seulement, et pour des raisons distinctes : SVG parce que c'est ce
// qu'un organisme a sous la main et que ça reste net partout ; PNG parce que
// la transparence est indispensable sur les deux thèmes ; JPEG parce qu'un
// logo photographique existe aussi. Pas de SVG dans les courriels en
// revanche — les messageries ne le rendent pas — mais la contrainte est côté
// gabarit, pas côté dépôt.
var logoTypes = map[string]string{
	"image/svg+xml": ".svg",
	"image/png":     ".png",
	"image/jpeg":    ".jpg",
}

// handleGetBrand renvoie la marque de l'organisation de la session, sous ses
// deux formes : brute pour le formulaire, résolue pour l'affichage.
func handleGetBrand(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		writeBrand(w, deps, r, session.OrgID)
	}
}

// handleSaveBrand écrit la marque de l'organisation de la session.
func handleSaveBrand(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}
		saveBrand(w, r, deps, session.OrgID, user.Email)
	}
}

// handleAdminGetBrand et handleAdminSaveBrand donnent la même main à l'équipe
// lemlearn, sur n'importe quel organisme. C'est ce qui permet d'ouvrir un
// client sans lui demander de se connecter : on l'habille pour lui, il trouve
// son enseigne à sa première visite.
func handleAdminGetBrand(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeBrand(w, deps, r, chi.URLParam(r, "orgID"))
	}
}

func handleAdminSaveBrand(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}
		saveBrand(w, r, deps, chi.URLParam(r, "orgID"), user.Email)
	}
}

// handlePrepareLogo signe un dépôt direct vers le compartiment public.
//
// Le fichier ne transite pas par l'API : un logo est petit, mais le faire
// passer par la Lambda imposerait d'y encoder l'image en base64 pour rien.
func handlePrepareLogo(deps Deps) http.HandlerFunc {
	type request struct {
		ContentType string `json:"contentType"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)
		orgID := chi.URLParam(r, "orgID")
		if orgID == "" {
			orgID = session.OrgID
		}

		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		ext, ok := logoTypes[strings.ToLower(strings.TrimSpace(body.ContentType))]
		if !ok {
			writeError(w, http.StatusBadRequest, "format accepté : SVG, PNG ou JPEG")
			return
		}
		if deps.Assets == nil {
			writeError(w, http.StatusServiceUnavailable, "dépôt de logo indisponible")
			return
		}

		key := brand.LogoKey(orgID, ext)
		url, err := deps.Assets.PresignedPut(r.Context(), key, body.ContentType, brand.UploadTTL)
		if err != nil {
			deps.Log.Error("signature du dépôt de logo", "err", err)
			writeError(w, http.StatusInternalServerError, "erreur interne")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"uploadUrl":        url,
			"key":              key,
			"expiresInSeconds": int(brand.UploadTTL.Seconds()),
		})
	}
}

// publicBrand résout la marque pour une page qu'un candidat voit sans compte :
// signature, invitation, questionnaire de satisfaction. Ces pages sont le cœur
// de la marque blanche — ce sont les seules que voit quelqu'un qui ne connaît
// pas l'organisme, et y afficher notre nom trahirait tout le reste.
//
// L'échec est silencieux et rend une identité par défaut : un logo manquant ne
// doit jamais empêcher une signature.
func publicBrand(r *http.Request, deps Deps, orgID string) brand.Public {
	if deps.Brand == nil || orgID == "" {
		return brand.Brand{}.Resolve("", "")
	}
	org, err := deps.Identity.LoadOrg(r.Context(), orgID)
	if err != nil {
		deps.Log.Warn("marque publique : organisation illisible", "err", err, "org", orgID)
		return brand.Brand{}.Resolve("", "")
	}
	resolved, err := deps.Brand.Resolve(r.Context(), orgID, org.Name)
	if err != nil {
		deps.Log.Warn("marque publique : lecture", "err", err, "org", orgID)
		return brand.Brand{}.Resolve(org.Name, "")
	}
	return resolved
}

func writeBrand(w http.ResponseWriter, deps Deps, r *http.Request, orgID string) {
	raw, err := deps.Brand.Get(r.Context(), orgID)
	if err != nil {
		deps.Log.Error("lecture de la marque", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	org, err := deps.Identity.LoadOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "organisation introuvable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"brand":    raw,
		"resolved": raw.Resolve(org.Name, deps.Brand.AssetsURL()),
		"orgName":  org.Name,
	})
}

func saveBrand(w http.ResponseWriter, r *http.Request, deps Deps, orgID, by string) {
	var body brand.Brand
	if !decodeJSON(w, r, &body) {
		return
	}
	// La clé du logo n'est pas prise telle quelle : un client qui l'inventerait
	// ferait afficher, sous l'enseigne de son organisme, une image déposée par
	// un autre. Elle est recomposée à partir de la seule extension.
	if body.LogoKey != "" {
		ext := strings.ToLower(filepath.Ext(body.LogoKey))
		valide := false
		for _, connue := range logoTypes {
			if ext == connue {
				valide = true
			}
		}
		if !valide {
			writeError(w, http.StatusBadRequest, "logo dans un format inattendu")
			return
		}
		body.LogoKey = brand.LogoKey(orgID, ext)
	}

	saved, err := deps.Brand.Save(r.Context(), orgID, body, by)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	org, err := deps.Identity.LoadOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "organisation introuvable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"brand":    saved,
		"resolved": saved.Resolve(org.Name, deps.Brand.AssetsURL()),
		"orgName":  org.Name,
	})
}
