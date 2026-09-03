// Commande api : le service HTTP de lemlearn.
//
// Le même binaire sert l'API en local (net/http) et en production (Lambda
// derrière API Gateway HTTP v2). La détection est faite au démarrage : rien
// dans le code métier ne sait où il tourne.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/lemlearn/api/internal/attendance"
	"github.com/lemlearn/api/internal/billing"
	"github.com/lemlearn/api/internal/brand"
	"github.com/lemlearn/api/internal/catalog"
	"github.com/lemlearn/api/internal/config"
	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/docflow"
	"github.com/lemlearn/api/internal/documents"
	"github.com/lemlearn/api/internal/emailtpl"
	"github.com/lemlearn/api/internal/export"
	"github.com/lemlearn/api/internal/followup"
	"github.com/lemlearn/api/internal/httpapi"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/invoicing"
	"github.com/lemlearn/api/internal/learning"
	"github.com/lemlearn/api/internal/library"
	"github.com/lemlearn/api/internal/platform/blob"
	"github.com/lemlearn/api/internal/platform/ddb"
	"github.com/lemlearn/api/internal/platform/doc"
	"github.com/lemlearn/api/internal/platform/mail"
	"github.com/lemlearn/api/internal/platform/seal"
	"github.com/lemlearn/api/internal/platform/tsa"
	"github.com/lemlearn/api/internal/signature"
	"github.com/lemlearn/api/internal/video"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		log.Error("configuration invalide", "err", err)
		os.Exit(1)
	}

	// Le compilateur Typst est optionnel au démarrage : en local, un
	// développeur qui travaille sur le CRM n'a pas besoin d'avoir installé
	// Typst. Les routes qui en dépendent renvoient alors 503.
	compiler, err := doc.NewBinaryCompiler()
	if err != nil {
		if cfg.Env != config.EnvLocal {
			log.Error("compilateur typst indisponible", "err", err)
			os.Exit(1)
		}
		log.Warn("compilateur typst indisponible, génération de documents désactivée", "err", err)
	}

	deps := httpapi.Deps{Config: cfg, Log: log}
	if compiler != nil {
		deps.Compiler = compiler
	}

	// La base est elle aussi optionnelle en local : `pnpm doc` et la
	// prévisualisation des gabarits doivent fonctionner sans DynamoDB.
	db, err := ddb.New(context.Background(), cfg.Table, cfg.AuditTable)
	if err != nil {
		if cfg.Env != config.EnvLocal {
			log.Error("dynamodb indisponible", "err", err)
			os.Exit(1)
		}
		log.Warn("dynamodb indisponible, routes métier désactivées", "err", err)
	} else {
		deps.Identity = identity.NewService(db, nil)
		deps.CRM = crm.NewService(db, nil)

		// Le compartiment des pièces d'identité est chiffré par une clé KMS
		// dédiée et purgé au bout de quatre-vingt-dix jours. Sans lui, la
		// fiche accepte tout le reste et refuse la pièce avec un motif :
		// mieux vaut un dossier incomplet qu'une carte d'identité rangée dans
		// un compartiment ordinaire.
		if cfg.IdentityBucket != "" {
			if bucket, err := blob.NewS3(context.Background(), cfg.IdentityBucket); err != nil {
				log.Warn("dépôt des pièces d'identité indisponible", "err", err)
			} else {
				deps.CRM = deps.CRM.WithDocs(bucket)
			}
		}
		deps.Catalog = catalog.NewService(db, nil)
		// La facturation de l'organisme à ses clients. À ne pas confondre avec
		// billing, qui est notre propre abonnement.
		deps.Invoicing = invoicing.NewService(db, deps.CRM, deps.Identity, nil)
		deps.Learning = learning.NewService(db, deps.Catalog, nil)
		deps.Attendance = attendance.NewService(db, deps.Catalog, nil)
		deps.Video = buildVideo(context.Background(), cfg, db, log)

		// Sans clé Resend, les courriels sont journalisés plutôt qu'envoyés :
		// le parcours de signature reste exerçable de bout en bout sur un
		// poste de développement comme sur un environnement de recette. Le
		// corps du message y figure — donc le lien de signature, qui est un
		// jeton d'accès — ce qui serait une fuite en production, où l'absence
		// de clé fait échouer le démarrage.
		var mailer mail.Sender = mail.NewLogVerbose(log)
		provider := "journal"
		if cfg.ResendAPIKey != "" {
			mailer = mail.NewResend(cfg.ResendAPIKey, cfg.MailFrom)
			provider = "resend"
		} else if cfg.Env == config.EnvProd {
			log.Error("RESEND_API_KEY est requise en production : " +
				"sans expéditeur réel, aucun lien de signature ne part")
			os.Exit(1)
		}

		// Tout envoi laisse une trace : « ce message est-il parti, quand, à
		// qui » est la première question posée quand un apprenant dit n'avoir
		// rien reçu. Le corps n'y figure pas — il porte des liens de
		// signature et des codes à usage unique.
		mailer = mail.NewJournaled(mailer, db, provider, nil)
		deps.MailJournal = mail.NewJournal(db)
		deps.Emails = emailtpl.NewService(db, nil)

		// L'identité visible de chaque organisme. Le service est monté même
		// sans compartiment : sans lui on ne peut pas déposer de logo, mais le
		// nom, la couleur et le monogramme suffisent déjà à ne plus afficher
		// notre marque chez un client.
		deps.Brand = brand.NewService(db, nil).WithAssets(cfg.AssetsURL)
		if cfg.AssetsBucket != "" {
			if bucket, err := blob.NewS3(context.Background(), cfg.AssetsBucket); err != nil {
				log.Warn("dépôt des logos indisponible", "err", err)
			} else {
				deps.Assets = bucket
			}
		}
		// Un seul résolveur d'identité, partagé par tout ce qui écrit à un
		// apprenant. Il échoue en silence vers une enseigne neutre : un logo
		// illisible ne doit jamais empêcher un courriel de partir.
		branding := func(ctx context.Context, orgID string) map[string]any {
			org, err := deps.Identity.LoadOrg(ctx, orgID)
			if err != nil {
				log.Warn("identité de l'organisme illisible", "err", err, "org", orgID)
				return nil
			}
			resolved, err := deps.Brand.Resolve(ctx, orgID, org.Name)
			if err != nil {
				log.Warn("marque illisible", "err", err, "org", orgID)
				return nil
			}
			return resolved.MailData()
		}

		deps.Library = library.NewService(db, nil)

		// La satisfaction à froid ne dépend ni du compilateur ni du
		// scellement : elle poste un lien de questionnaire, trois mois après.
		deps.FollowUp = followup.NewService(db, mailer, cfg.AppURL, nil).
			WithComposer(deps.Emails).WithBranding(branding)
		deps.Mailer = mailer

		// La vue super-admin non plus : elle doit rester consultable
		// précisément quand une brique manque, puisque c'est ce qu'on
		// regarde quand un client appelle.
		deps.Billing = billing.NewService(billing.Deps{
			DB: db, CRM: deps.CRM, Catalog: deps.Catalog, Video: deps.Video,
		})
		deps.Stripe = billing.NewStripe(cfg.StripeKey, cfg.StripeWebhookSecret, cfg.StripePrices)

		if compiler != nil {
			// Le scelleur : certificat de l'organisme en production, certificat
			// auto-signé et explicitement nommé « sans valeur » en local.
			sealer, err := buildSealer(cfg)
			if err != nil {
				log.Error("scellement indisponible", "err", err)
				os.Exit(1)
			}

			// Les documents scellés vont dans S3 sous Object Lock. En local,
			// faute de compartiment, ils restent en mémoire : le parcours est
			// exerçable, mais un redémarrage les perd — et l'export le dit,
			// plutôt que de laisser croire à un archivage.
			var store signature.BlobStore = blob.NewMemory()
			if cfg.DocumentsBucket != "" {
				bucket, err := blob.NewS3(context.Background(), cfg.DocumentsBucket)
				if err != nil {
					log.Error("archivage S3 indisponible", "err", err)
					os.Exit(1)
				}
				store = bucket
			} else if cfg.Env != config.EnvLocal {
				log.Error("LEMLEARN_DOCUMENTS_BUCKET est requis hors développement : " +
					"un document scellé ne peut pas être archivé en mémoire")
				os.Exit(1)
			}

			// Le logo de l'organisme, incorporé aux documents. Il est composé
			// ici parce que le nom vient de la marque et les octets du
			// compartiment : deux paquets que le composeur de documents n'a
			// pas à connaître.
			logos := func(ctx context.Context, orgID string) (string, []byte) {
				if deps.Assets == nil || deps.Brand == nil {
					return "", nil
				}
				marque, err := deps.Brand.Get(ctx, orgID)
				if err != nil || marque.LogoKey == "" {
					return "", nil
				}
				octets, err := deps.Assets.Get(ctx, marque.LogoKey)
				if err != nil {
					log.Warn("logo illisible", "err", err, "org", orgID)
					return "", nil
				}
				return documents.LogoAsset(marque.LogoKey), octets
			}

			deps.Signature = signature.NewService(signature.Deps{
				DB:       db,
				Renderer: docflow.NewRenderer(deps.Identity, deps.CRM, compiler).WithLogos(logos),
				Blobs:    store,
				Mailer:   mailer,
				Composer: deps.Emails,
				Branding: branding,
				Sealer:   sealer,
				AppURL:   cfg.AppURL,
			})

			// L'export vient après la signature : il relit les documents
			// scellés, donc il lui faut le service qui les archive.
			deps.Export = export.NewService(export.Deps{
				Identity: deps.Identity, CRM: deps.CRM,
				Catalog: deps.Catalog, Learning: deps.Learning,
				Signature: deps.Signature, Compiler: compiler,
			})
		}
	}

	handler := httpapi.NewRouter(deps)

	if config.IsLambda() {
		log.Info("démarrage en lambda", "env", cfg.Env)
		adapter := httpadapter.NewV2(handler)

		// La même fonction sert l'API et les travaux programmés. Une fonction
		// séparée pour la relance à froid n'aurait rien apporté : mêmes
		// dépendances, même code, un artefact de plus à déployer et à garder
		// en phase.
		lambda.Start(func(ctx context.Context, payload json.RawMessage) (any, error) {
			var scheduled struct {
				Task string `json:"task"`
			}
			if err := json.Unmarshal(payload, &scheduled); err == nil && scheduled.Task != "" {
				return runTask(ctx, scheduled.Task, deps, log)
			}

			var request events.APIGatewayV2HTTPRequest
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, fmt.Errorf("événement non reconnu: %w", err)
			}
			return adapter.ProxyWithContext(ctx, request)
		})
		return
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("api à l'écoute", "port", cfg.Port, "env", cfg.Env)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("serveur arrêté", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Error("arrêt forcé", "err", err)
	}
	log.Info("api arrêtée")
}

// buildSealer construit le scelleur PAdES à partir de la configuration.
func buildSealer(cfg config.Config) (*seal.PAdES, error) {
	timestamper := tsa.New(cfg.TSAURL)

	if cfg.SealCertPEM == "" || cfg.SealKeyPEM == "" {
		if cfg.Env == config.EnvProd {
			// Un document contractuel ne peut pas être scellé avec un
			// certificat auto-signé : en production, l'absence de cachet
			// d'organisation est une erreur de déploiement, pas un mode
			// dégradé.
			return nil, fmt.Errorf(
				"LEMLEARN_SEAL_CERT et LEMLEARN_SEAL_KEY sont requis en production")
		}
		// Le certificat porte « sans valeur » dans son nom : un document de
		// recette ne peut pas passer pour un document contractuel.
		return seal.Development("Organisme de recette", timestamper)
	}

	cert, key, chain, err := seal.LoadKeyPair([]byte(cfg.SealCertPEM), []byte(cfg.SealKeyPEM))
	if err != nil {
		return nil, err
	}
	return seal.New(cert, key, chain, timestamper), nil
}

// buildVideo assemble la chaîne vidéo, ou renvoie nil si elle n'est pas
// configurée.
//
// L'absence est un état normal : un organisme qui ne fait que du présentiel
// n'a pas de vidéo à héberger, et le reste du produit ne doit pas en dépendre.
// Les routes concernées répondent alors 503 avec un motif, plutôt que
// d'échouer au démarrage.
func buildVideo(ctx context.Context, cfg config.Config, db *ddb.Client, log *slog.Logger) *video.Service {
	if cfg.VideoBucket == "" {
		return nil
	}

	bucket, err := blob.NewS3(ctx, cfg.VideoBucket)
	if err != nil {
		log.Warn("dépôt vidéo indisponible", "err", err)
		return nil
	}

	deps := video.Deps{DB: db, Uploader: bucket, Objects: bucket, Bucket: cfg.VideoBucket}

	if cfg.CloudFrontDomain != "" && cfg.CloudFrontKeyPairID != "" && cfg.CloudFrontKeyPEM != "" {
		signer, err := video.NewSigner(cfg.CloudFrontDomain, cfg.CloudFrontKeyPairID,
			[]byte(cfg.CloudFrontKeyPEM))
		if err != nil {
			log.Warn("diffusion vidéo indisponible", "err", err)
		} else {
			deps.Signer = signer
		}
	}

	if cfg.MediaConvertRoleARN != "" {
		encoder, err := video.NewMediaConvert(ctx, cfg.MediaConvertEndpoint,
			cfg.MediaConvertRoleARN, cfg.MediaConvertQueueARN)
		if err != nil {
			log.Warn("transcodage indisponible", "err", err)
		} else {
			deps.Encoder = encoder
		}
	}

	log.Info("chaîne vidéo",
		"depot", true, "transcodage", deps.Encoder != nil, "diffusion", deps.Signer != nil)
	return video.NewService(deps)
}

// runTask exécute un travail programmé.
//
// Le résultat est renvoyé et journalisé plutôt que silencieux : une relance
// qui ne part pas doit se voir dans les métriques d'invocation, pas se
// découvrir trois mois plus tard sur un taux de retour à zéro.
func runTask(ctx context.Context, task string, deps httpapi.Deps, log *slog.Logger) (any, error) {
	switch task {
	case "satisfaction-froid":
		if deps.FollowUp == nil {
			return nil, fmt.Errorf("relances non configurées")
		}
		sent, failed, err := deps.FollowUp.Run(ctx, time.Now().UTC())
		if err != nil {
			log.Error("relance de satisfaction à froid", "err", err)
			return nil, err
		}
		log.Info("relance de satisfaction à froid", "envoyees", sent, "echecs", failed)
		if failed > 0 {
			// Échouer l'invocation rend l'incident visible dans les alarmes,
			// et les tâches restées « planned » repartiront au tour suivant.
			return nil, fmt.Errorf("%d relance(s) en échec sur %d", failed, sent+failed)
		}
		return map[string]int{"envoyees": sent}, nil
	default:
		return nil, fmt.Errorf("travail %q inconnu", task)
	}
}
