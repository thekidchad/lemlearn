// Package httpapi assemble le routeur HTTP du service.
//
// Une seule Lambda sert toute l'API, routée par chi : un seul démarrage à
// froid, un déploiement atomique, et `go run ./cmd/api` sert exactement la
// même API en local qu'en production.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/lemlearn/api/internal/config"
	"github.com/lemlearn/api/internal/platform/doc"
)

// Deps regroupe les dépendances injectées au routeur. Aucun handler ne
// construit ses propres clients : c'est ce qui rend les tests possibles.
type Deps struct {
	Config   config.Config
	Log      *slog.Logger
	Compiler doc.Compiler
}

// NewRouter construit le routeur complet.
func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(deps.Log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(securityHeaders)

	r.Get("/health", handleHealth(deps))

	r.Route("/v1", func(r chi.Router) {
		r.Route("/documents", func(r chi.Router) {
			// Prévisualisation d'un gabarit avec un jeu de données de
			// démonstration. Utile pour itérer sur la mise en page sans
			// créer de dossier réel ; jamais exposée hors développement.
			r.Get("/preview/{template}", handleDocumentPreview(deps))
		})
	})

	return r
}

func handleHealth(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"env":      deps.Config.Env,
			"typst":    deps.Compiler != nil,
			"checked":  time.Now().UTC().Format(time.RFC3339),
			"service":  "lemlearn-api",
			"revision": revision(),
		})
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			log.Info("requête",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
