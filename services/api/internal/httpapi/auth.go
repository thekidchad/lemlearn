package httpapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/lemlearn/api/internal/config"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/audit"
)

// Noms du cookie de session.
//
// Le préfixe __Host- fait respecter par le navigateur trois garanties : servi
// uniquement en HTTPS, sans attribut Domain, avec Path=/. Un sous-domaine
// compromis ne peut donc pas poser ce cookie à notre place.
//
// Mais ce préfixe *impose* l'attribut Secure : un cookie __Host- sans Secure
// est rejeté, y compris par curl. En développement local, servi en clair sur
// localhost, on retombe donc sur un nom ordinaire — la garantie qu'apporte le
// préfixe n'existe de toute façon pas sans HTTPS.
const (
	SessionCookie      = "__Host-lemlearn_session"
	SessionCookieLocal = "lemlearn_session"
)

// sessionCookieName choisit le nom selon que la connexion est chiffrée.
func sessionCookieName(cfg config.Config) string {
	if cfg.Env == config.EnvLocal {
		return SessionCookieLocal
	}
	return SessionCookie
}

type contextKey string

const sessionContextKey contextKey = "session"

// requireAuth refuse les requêtes sans session valide et dépose la session
// dans le contexte.
func requireAuth(deps Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName(deps.Config))
			if err != nil {
				writeError(w, http.StatusUnauthorized, "session requise")
				return
			}

			session, err := deps.Identity.Authenticate(r.Context(), cookie.Value)
			if err != nil {
				if errors.Is(err, identity.ErrInvalidCredentials) {
					// Le cookie est effacé : sans cela, un navigateur avec une
					// session périmée boucle indéfiniment sur la page de
					// connexion.
					clearSessionCookie(w, deps.Config)
					writeError(w, http.StatusUnauthorized, "session expirée")
					return
				}
				deps.Log.Error("authentification", "err", err)
				writeError(w, http.StatusInternalServerError, "erreur interne")
				return
			}

			ctx := context.WithValue(r.Context(), sessionContextKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// requireRole n'autorise que certains rôles.
func requireRole(check func(identity.Role) bool, message string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := sessionFrom(r)
			if !ok || !check(session.Role) {
				writeError(w, http.StatusForbidden, message)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// sessionFrom lit la session déposée par requireAuth.
func sessionFrom(r *http.Request) (identity.Session, bool) {
	session, ok := r.Context().Value(sessionContextKey).(identity.Session)
	return session, ok
}

// actorFrom compose l'acteur d'audit à partir de la requête.
//
// Toute action passe par ici : l'adresse IP, le navigateur et, le cas échéant,
// le super-administrateur qui agit à la place du client sont systématiquement
// consignés — on ne peut pas les oublier au cas par cas.
func actorFrom(r *http.Request, label string) audit.Actor {
	session, _ := sessionFrom(r)
	return audit.Actor{
		Type:       audit.ActorUser,
		ID:         session.UserID,
		Label:      label,
		IP:         clientIP(r),
		UserAgent:  truncateUA(r.UserAgent()),
		OnBehalfOf: session.ImpersonatedBy,
	}
}

func clientIP(r *http.Request) string {
	// middleware.RealIP a déjà résolu X-Forwarded-For en RemoteAddr.
	//
	// SplitHostPort et non un découpage au premier deux-points : une adresse
	// IPv6 s'écrit « [::1]:54321 », et couper naïvement journalisait « [ »
	// comme adresse du signataire. Une IP fausse dans un dossier de preuve
	// est pire qu'une IP absente.
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func truncateUA(ua string) string {
	const max = 256
	if len(ua) > max {
		return ua[:max]
	}
	return ua
}

func setSessionCookie(w http.ResponseWriter, cfg config.Config, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName(cfg),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		// Le jeton ne doit jamais être lisible en JavaScript ni voyager en
		// clair : une faille XSS ne doit pas donner accès aux dossiers.
		Secure:   cfg.Env != config.EnvLocal,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(identity.SessionTTL.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter, cfg config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName(cfg),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.Env != config.EnvLocal,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// handleRegister crée une organisation et son compte propriétaire.
func handleRegister(deps Deps) http.HandlerFunc {
	type request struct {
		OrgName   string `json:"orgName"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		org, user, err := deps.Identity.Register(r.Context(), identity.RegisterInput{
			OrgName: body.OrgName, Email: body.Email, Password: body.Password,
			FirstName: body.FirstName, LastName: body.LastName,
			IP: clientIP(r), UserAgent: truncateUA(r.UserAgent()),
		})
		if err != nil {
			if errors.Is(err, identity.ErrEmailTaken) {
				writeError(w, http.StatusConflict, "cette adresse e-mail est déjà utilisée")
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// L'inscription ne connecte pas automatiquement : l'utilisateur passe
		// par /login, ce qui garantit que le chemin de connexion fonctionne
		// avant qu'il ne dépende de nous pour son activité.
		writeJSON(w, http.StatusCreated, map[string]any{
			"org":  map[string]string{"id": org.ID, "name": org.Name},
			"user": user.Public(),
		})
	}
}

// handleLogin ouvre une session.
func handleLogin(deps Deps) http.HandlerFunc {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var body request
		if !decodeJSON(w, r, &body) {
			return
		}

		user, token, err := deps.Identity.Login(r.Context(), body.Email, body.Password,
			clientIP(r), truncateUA(r.UserAgent()))
		if err != nil {
			switch {
			case errors.Is(err, identity.ErrInvalidCredentials):
				writeError(w, http.StatusUnauthorized, "identifiants invalides")
			case errors.Is(err, identity.ErrDisabled):
				writeError(w, http.StatusForbidden, "ce compte est désactivé")
			default:
				deps.Log.Error("connexion", "err", err, "request_id", middleware.GetReqID(r.Context()))
				writeError(w, http.StatusInternalServerError, "erreur interne")
			}
			return
		}

		setSessionCookie(w, deps.Config, token)
		writeJSON(w, http.StatusOK, map[string]any{"user": user.Public()})
	}
}

// handleLogout révoque la session courante.
func handleLogout(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(sessionCookieName(deps.Config)); err == nil {
			if err := deps.Identity.Logout(r.Context(), cookie.Value); err != nil {
				deps.Log.Error("déconnexion", "err", err)
			}
		}
		clearSessionCookie(w, deps.Config)
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleMe renvoie l'utilisateur et l'organisation de la session.
func handleMe(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionFrom(r)

		user, err := deps.Identity.LoadUser(r.Context(), session)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session invalide")
			return
		}
		org, err := deps.Identity.LoadOrg(r.Context(), session.OrgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "organisation introuvable")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"user": user.Public(),
			"org": map[string]any{
				"id": org.ID, "name": org.Name, "plan": org.Plan,
				"qualiopiCertified": org.QualiopiCertified,
			},
			"impersonatedBy": session.ImpersonatedBy,
		})
	}
}
