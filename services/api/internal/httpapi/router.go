// Package httpapi assemble le routeur HTTP du service.
//
// Une seule Lambda sert toute l'API, routée par chi : un seul démarrage à
// froid, un déploiement atomique, et `go run ./cmd/api` sert exactement la
// même API en local qu'en production.
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/config"
	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/learning"
	"github.com/lemlearn/api/internal/platform/doc"
	"github.com/lemlearn/api/internal/signature"
)

// Deps regroupe les dépendances injectées au routeur. Aucun handler ne
// construit ses propres clients : c'est ce qui rend les tests possibles.
type Deps struct {
	Config    config.Config
	Log       *slog.Logger
	Compiler  doc.Compiler
	Identity  *identity.Service
	CRM       *crm.Service
	Signature *signature.Service
	Catalog   *catalog.Service
	Learning  *learning.Service
	Clock     func() time.Time
}

// Now renvoie l'heure courante, injectable pour les tests.
func (d Deps) Now() time.Time {
	if d.Clock != nil {
		return d.Clock()
	}
	return time.Now().UTC()
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
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", handleRegister(deps))
			r.Post("/login", handleLogin(deps))
			r.Post("/logout", handleLogout(deps))
		})

		// Tout le reste exige une session.
		r.Group(func(r chi.Router) {
			r.Use(requireAuth(deps))

			r.Get("/me", handleMe(deps))

			// Espace apprenant. Ces routes ne sont pas réservées aux
			// administrateurs : c'est l'apprenant lui-même qui les appelle,
			// et l'identifiant de sa fiche vient de son compte, jamais de
			// l'URL.
			r.Route("/learn", func(r chi.Router) {
				r.Get("/", handleLearnerDashboard(deps))
				r.Route("/{sessionID}/courses/{courseID}", func(r chi.Router) {
					r.Get("/modules/{moduleID}/progress", handleModuleProgress(deps))
					r.Post("/modules/{moduleID}/beat", handleHeartbeat(deps))
					r.Get("/quizzes/{quizID}", handleGetQuiz(deps))
					r.Post("/quizzes/{quizID}/submit", handleSubmitQuiz(deps))
				})
			})

			r.Group(func(r chi.Router) {
				r.Use(requireRole(identity.Role.CanManageCRM, "réservé aux administrateurs"))

				r.Route("/contacts", func(r chi.Router) {
					r.Get("/", handleListContacts(deps))
					r.Post("/", handleCreateContact(deps))
					r.Get("/{contactID}", handleGetContact(deps))
				})

				r.Route("/courses", func(r chi.Router) {
					r.Get("/", handleListCourses(deps))
					r.Post("/", handleCreateCourse(deps))
					r.Get("/{courseID}", handleGetCourse(deps))
					r.Post("/{courseID}/modules", handleAddModule(deps))
				})

				r.Route("/sessions", func(r chi.Router) {
					r.Get("/", handleListSessions(deps))
					r.Post("/", handleCreateSession(deps))
					r.Get("/{sessionID}/enrollments", handleListEnrollments(deps))
					r.Post("/{sessionID}/enrollments", handleEnroll(deps))
				})

				r.Route("/quizzes", func(r chi.Router) {
					r.Post("/", handleSaveQuiz(deps))
					r.Post("/{quizID}/versions/{version}/publish", handlePublishQuiz(deps))
				})

				r.Route("/files", func(r chi.Router) {
					r.Get("/", handlePipeline(deps))
					r.Post("/", handleCreateFile(deps))
					r.Get("/{fileID}", handleGetFile(deps))
					r.Patch("/{fileID}/stage", handleMoveFile(deps))
					r.Get("/{fileID}/timeline", handleFileTimeline(deps))
					r.Get("/{fileID}/signatures", handleListSignatures(deps))
					r.Post("/{fileID}/signatures", handleIssueSignature(deps))
				})
			})
		})

		// Parcours du signataire : délibérément hors de tout groupe
		// authentifié. Le signataire n'a pas de compte ; sa légitimité vient
		// du jeton du lien, puis du code envoyé à son adresse vérifiée.
		r.Route("/sign/{token}", func(r chi.Router) {
			r.Get("/", handleSignOpen(deps))
			r.Get("/document", handleSignDocument(deps))
			r.Post("/otp", handleSignOTP(deps))
			r.Post("/confirm", handleSignConfirm(deps))
			r.Get("/sealed", handleSignSealed(deps))
		})

		r.Route("/documents", func(r chi.Router) {
			// Prévisualisation d'un gabarit avec un jeu de données de
			// démonstration. Utile pour itérer sur la mise en page sans
			// créer de dossier réel ; indisponible en production.
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
			"database": deps.Identity != nil,
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
		// L'API ne sert que du JSON et des PDF : aucune de ses réponses n'a
		// de raison d'être mise en cache par un intermédiaire.
		h.Set("Cache-Control", "no-store")
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

// maxBodyBytes borne les corps de requête. Les téléversements de vidéos et de
// pièces d'identité passent par des URL présignées S3, jamais par l'API : un
// mégaoctet suffit largement au JSON, et refuser au-delà évite qu'une requête
// malformée n'épuise la mémoire de la Lambda.
const maxBodyBytes = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	return decodeJSONLimit(w, r, target, maxBodyBytes)
}

// decodeJSONLimit décode avec une borne explicite, pour les rares corps
// légitimement plus gros — l'image d'un tracé de signature.
func decodeJSONLimit(w http.ResponseWriter, r *http.Request, target any, limit int64) bool {
	decoder := json.NewDecoder(io.LimitReader(r.Body, limit))
	// Un champ inconnu est une erreur, pas un silence : c'est ce qui rattrape
	// les fautes de frappe côté client avant qu'elles ne deviennent des
	// données manquantes en base.
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "corps de requête vide")
			return false
		}
		writeError(w, http.StatusBadRequest, fmt.Sprintf("corps de requête invalide: %v", err))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
