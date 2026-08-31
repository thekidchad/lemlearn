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

	"github.com/lemlearn/api/internal/attendance"
	"github.com/lemlearn/api/internal/billing"
	"github.com/lemlearn/api/internal/brand"
	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/config"
	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/emailtpl"
	"github.com/lemlearn/api/internal/export"
	"github.com/lemlearn/api/internal/followup"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/learning"
	"github.com/lemlearn/api/internal/library"
	"github.com/lemlearn/api/internal/platform/doc"
	"github.com/lemlearn/api/internal/platform/mail"
	"github.com/lemlearn/api/internal/signature"
	"github.com/lemlearn/api/internal/video"
)

// Deps regroupe les dépendances injectées au routeur. Aucun handler ne
// construit ses propres clients : c'est ce qui rend les tests possibles.
type Deps struct {
	Config     config.Config
	Log        *slog.Logger
	Compiler   doc.Compiler
	Identity   *identity.Service
	CRM        *crm.Service
	Signature  *signature.Service
	Catalog    *catalog.Service
	Learning   *learning.Service
	Export     *export.Service
	Attendance *attendance.Service
	Video      *video.Service
	FollowUp   *followup.Service
	Billing    *billing.Service
	// Mailer sert les courriels qui n'appartiennent à aucun domaine : ceux de
	// la signature partent du service de signature, ceux de la relance à
	// froid du sien.
	Mailer mail.Sender
	// Emails porte les gabarits modifiables par l'équipe lemlearn, et
	// MailJournal la trace de ce qui est parti.
	Emails      *emailtpl.Service
	MailJournal *mail.Journal
	// Brand porte l'identité visible de chaque organisme, et Assets le
	// compartiment public où vivent les logos. La marque blanche tient à ces
	// deux-là : sans le second, on sait quoi afficher mais pas où le prendre.
	Brand  *brand.Service
	Assets Presigner
	Library     *library.Service
	Stripe      *billing.Stripe
	Clock       func() time.Time
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
			// Quitter une impersonation. La route vit ici et non sous la garde
			// super-admin : le temps de l'impersonation, la session porte le
			// rôle du client, et la garde refuserait la sortie.
			r.Post("/impersonation/fin", handleEndImpersonation(deps))
			// L'état de conformité de l'organisation : ce que regarde un
			// dirigeant avant un audit, et la seule vue qui agrège les
			// dossiers plutôt que de les lister.
			r.Get("/qualiopi", handleQualiopiDashboard(deps))
			// Souscrire est un acte du dirigeant, pas du support : ces routes
			// vivent dans l'espace client, pas dans la vue de l'équipe.
			r.Get("/abonnement", handleSubscription(deps))
			r.Post("/abonnement/paiement", handleCheckout(deps))
			// Portabilité : accessible à la personne concernée comme à
			// l'administrateur, jamais à un tiers.
			r.Get("/contacts/{contactID}/donnees", handlePortability(deps))

			// Espace apprenant. Ces routes ne sont pas réservées aux
			// administrateurs : c'est l'apprenant lui-même qui les appelle,
			// et l'identifiant de sa fiche vient de son compte, jamais de
			// l'URL.
			r.Route("/learn", func(r chi.Router) {
				r.Get("/", handleLearnerDashboard(deps))

				// L'émargement par l'apprenant lui-même. Une feuille signée du
				// seul organisme n'atteste que ses propres déclarations : c'est
				// la signature du stagiaire qui en fait une pièce opposable.
				// La session résout sa formation : l'adresse d'un module n'a
				// plus à porter trois identifiants.
				r.Get("/{sessionID}", handleLearnerSession(deps))

				r.Route("/{sessionID}/emargement", func(r chi.Router) {
					r.Get("/", handleLearnerSheet(deps))
					r.Post("/{slotID}", handleLearnerSign(deps))
				})

				r.Route("/{sessionID}/courses/{courseID}", func(r chi.Router) {
					r.Get("/", handleLearnerCourse(deps))
					r.Get("/modules/{moduleID}/progress", handleModuleProgress(deps))
					r.Post("/modules/{moduleID}/beat", handleHeartbeat(deps))
					r.Post("/modules/{moduleID}/playback", handlePlayback(deps))
					r.Get("/modules/{moduleID}/manifest.m3u8", handleManifest(deps))
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
					r.Patch("/{contactID}", handleUpdateContact(deps))
					r.Post("/{contactID}/anonymize", handleAnonymize(deps))
					// L'apprenant ne s'inscrit pas lui-même : c'est
					// l'organisme qui lui ouvre un accès.
					r.Post("/{contactID}/invitation", handleInviteLearner(deps))

					// Pièce d'identité : elle ne transite jamais par l'API,
					// et son lien de lecture vit une minute.
					r.Post("/{contactID}/piece-identite", handlePrepareIdentityDoc(deps))
					r.Put("/{contactID}/piece-identite", handleAttachIdentityDoc(deps))
					r.Get("/{contactID}/piece-identite", handleIdentityDocURL(deps))
					r.Delete("/{contactID}/piece-identite", handleDeleteIdentityDoc(deps))
				})

				// L'identité visible de l'organisme. En lecture pour tous les
				// membres — chaque écran s'en sert pour s'habiller — et en
				// écriture pour ceux qui administrent l'organisme.
				// L'identité juridique de l'organisme : ce qui rend une
				// convention opposable, et que la réglementation impose de
				// faire figurer sur chaque pièce.
				r.Route("/organisme", func(r chi.Router) {
					r.Get("/", handleGetOrgLegal(deps))
					r.Put("/", handleSaveOrgLegal(deps))
					// Le bilan annuel, et le règlement intérieur : deux
					// obligations que le produit peut remplir à partir de ce
					// qu'il détient déjà.
					r.Get("/bilan", handleBPF(deps))
					r.Get("/reglement", handleReglement(deps))
					// Aperçu d'un gabarit, sous l'identité réelle de
					// l'organisme : contenu fictif, mais en-tête, logo et
					// mention légale sont ceux qui partiront. Il exige donc une
					// session, ce qui n'était pas le cas quand il ne montrait
					// qu'un exemplaire de démonstration.
					r.Get("/apercu/{template}", handleDocumentPreview(deps))
				})

				r.Route("/marque", func(r chi.Router) {
					r.Get("/", handleGetBrand(deps))
					r.Group(func(r chi.Router) {
						r.Use(requireRole(identity.Role.CanManageCRM, "réservé aux administrateurs de l'organisme"))
						r.Put("/", handleSaveBrand(deps))
						r.Post("/logo", handlePrepareLogo(deps))
					})
				})

				// La bibliothèque lemlearn : consultable et importable par
				// l'organisme, modifiable par la seule équipe.
				r.Route("/bibliotheque", func(r chi.Router) {
					r.Get("/", handleListLibrary(deps, true))
					r.Get("/{courseID}", handleGetLibraryCourse(deps))
					r.Post("/{courseID}/importer", handleImportLibraryCourse(deps))
				})

				r.Route("/courses", func(r chi.Router) {
					r.Get("/", handleListCourses(deps))
					r.Post("/", handleCreateCourse(deps))
					r.Get("/{courseID}", handleGetCourse(deps))
					r.Post("/{courseID}/modules", handleAddModule(deps))
					r.Patch("/{courseID}/modules/{moduleID}", handleUpdateModule(deps))
				})

				r.Route("/sessions", func(r chi.Router) {
					r.Get("/", handleListSessions(deps))
					r.Post("/", handleCreateSession(deps))
					r.Get("/{sessionID}/enrollments", handleListEnrollments(deps))
					r.Post("/{sessionID}/enrollments", handleEnroll(deps))
					r.Post("/{sessionID}/close", handleCloseSession(deps))
					r.Get("/{sessionID}/attendance", handleGetSheet(deps))
					r.Post("/{sessionID}/attendance", handleSignAttendance(deps))
					r.Post("/{sessionID}/attendance/countersign", handleCountersign(deps))
				})

				r.Route("/videos", func(r chi.Router) {
					r.Get("/", handleListVideos(deps))
					r.Post("/", handleReserveVideo(deps))
					r.Get("/{assetID}", handleGetVideo(deps))
					r.Post("/{assetID}/uploaded", handleVideoUploaded(deps))
				})

				r.Route("/quizzes", func(r chi.Router) {
					r.Get("/", handleListQuizzes(deps))
					r.Post("/", handleSaveQuiz(deps))
					r.Get("/{quizID}", handleGetQuizVersions(deps))
					r.Get("/{quizID}/resultats", handleQuizResults(deps))
					r.Post("/{quizID}/versions/{version}/publish", handlePublishQuiz(deps))
				})

				r.Route("/files", func(r chi.Router) {
					r.Get("/", handlePipeline(deps))
					r.Post("/", handleCreateFile(deps))
					r.Get("/{fileID}", handleGetFile(deps))
					r.Patch("/{fileID}/stage", handleMoveFile(deps))
					r.Patch("/{fileID}/financement", handleSetFunding(deps))
					r.Get("/{fileID}/timeline", handleFileTimeline(deps))
					r.Post("/{fileID}/export", handleExportFile(deps))
					r.Get("/{fileID}/signatures", handleListSignatures(deps))
					r.Post("/{fileID}/signatures", handleIssueSignature(deps))
				})
			})

			// Vue super-admin : l'équipe lemlearn, jamais un client.
			r.Group(func(r chi.Router) {
				r.Use(requireRole(func(role identity.Role) bool {
					return role == identity.RoleSuperAdmin
				}, "réservé à l'équipe lemlearn"))

				r.Route("/admin", func(r chi.Router) {
					r.Get("/tableau", handleAdminDashboard(deps))
					r.Get("/orgs", handleListOrgs(deps))
					// Ouvrir l'espace d'un client et inviter son responsable.
					// C'est le premier geste commercial, et il n'existait pas :
					// une organisation ne pouvait naître que par l'inscription
					// libre.
					r.Post("/orgs", handleOpenOrg(deps))
					r.Get("/orgs/{orgID}", handleOrgDetail(deps))
					r.Post("/orgs/{orgID}/plan", handleSetPlan(deps))
					r.Post("/orgs/{orgID}/impersonate", handleImpersonate(deps))

					// Habiller un client depuis la vue super-admin : c'est ce
					// qui rend l'ouverture d'un organisme immédiate, sans rien
					// lui demander ni rien redéployer.
					// Le contenu d'un client, en lecture. Chaque appel interroge
					// la partition d'un organisme désigné : c'est une lecture
					// ciblée, pas un balayage de la table.
					r.Get("/orgs/{orgID}/repertoire", handleAdminOrgContacts(deps))
					r.Get("/orgs/{orgID}/sessions", handleAdminOrgSessions(deps))
					r.Get("/orgs/{orgID}/formations", handleAdminOrgCourses(deps))
					r.Get("/orgs/{orgID}/dossiers", handleAdminOrgFiles(deps))

					r.Get("/orgs/{orgID}/marque", handleAdminGetBrand(deps))
					r.Put("/orgs/{orgID}/marque", handleAdminSaveBrand(deps))
					r.Post("/orgs/{orgID}/marque/logo", handlePrepareLogo(deps))

					// Traçabilité : ce qui est parti, et à qui.
					r.Get("/emails", handleMailJournal(deps))

					// Gabarits des courriels transactionnels.
					r.Route("/gabarits", func(r chi.Router) {
						r.Get("/", handleListEmailTemplates(deps))
						r.Put("/{key}", handleSaveEmailTemplate(deps))
						r.Delete("/{key}", handleResetEmailTemplate(deps))
						r.Post("/{key}/apercu", handlePreviewEmailTemplate(deps))
					})

					// Retrouver un apprenant à travers les organisations.
					r.Get("/apprenants", handleFindLearner(deps))

					// Recherche unique de la palette : organisations,
					// apprenants, bibliothèque et gabarits en un seul appel.
					r.Get("/recherche", handleAdminSearch(deps))

					// La bibliothèque de formations mise à disposition des
					// organismes.
					r.Route("/bibliotheque", func(r chi.Router) {
						r.Get("/", handleListLibrary(deps, false))
						r.Post("/", handleSaveLibraryCourse(deps))
						r.Get("/{courseID}", handleGetLibraryCourse(deps))
						r.Put("/{courseID}", handleSaveLibraryCourse(deps))
						r.Delete("/{courseID}", handleDeleteLibraryCourse(deps))
						r.Post("/{courseID}/modules", handleSaveLibraryModule(deps))
					})
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

		// Invitation d'un apprenant : il n'a pas encore de compte, sa
		// légitimité vient du jeton reçu par courriel.
		r.Route("/invitation/{token}", func(r chi.Router) {
			r.Get("/", handleInvitationOpen(deps))
			r.Post("/", handleInvitationAccept(deps))
		})

		// Satisfaction à froid : même principe que la signature — la personne
		// n'a pas de compte, sa légitimité vient du jeton reçu par courriel.
		r.Route("/satisfaction/{token}", func(r chi.Router) {
			r.Get("/", handleSurveyOpen(deps))
			r.Post("/", handleSurveySubmit(deps))
		})

		// Stripe n'a pas de session : sa légitimité tient à la signature de
		// l'appel, vérifiée avant d'ouvrir la charge utile.
		r.Post("/stripe/webhook", handleStripeWebhook(deps))

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

// list garantit qu'une liste vide sort en `[]` et non en `null`.
//
// Go rend une tranche nil en `null`, ce qui oblige chaque client à écrire la
// même garde — et à l'oublier une fois. Le premier écran qui plante sur un
// dossier sans document est un bug d'API, pas un bug de front.
func list[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
