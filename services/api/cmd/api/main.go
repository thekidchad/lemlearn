// Commande api : le service HTTP de lemlearn.
//
// Le même binaire sert l'API en local (net/http) et en production (Lambda
// derrière API Gateway HTTP v2). La détection est faite au démarrage : rien
// dans le code métier ne sait où il tourne.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/lemlearn/api/internal/config"
	"github.com/lemlearn/api/internal/crm"
	"github.com/lemlearn/api/internal/docflow"
	"github.com/lemlearn/api/internal/httpapi"
	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/blob"
	"github.com/lemlearn/api/internal/platform/ddb"
	"github.com/lemlearn/api/internal/platform/doc"
	"github.com/lemlearn/api/internal/platform/mail"
	"github.com/lemlearn/api/internal/platform/seal"
	"github.com/lemlearn/api/internal/platform/tsa"
	"github.com/lemlearn/api/internal/signature"
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

		if compiler != nil {
			// En local, les courriels sont journalisés plutôt qu'envoyés et
			// les fichiers restent en mémoire : le parcours de signature est
			// exerçable de bout en bout sans compte Resend ni compartiment S3.
			var mailer mail.Sender = mail.NewLog(log)
			if cfg.ResendAPIKey != "" {
				mailer = mail.NewResend(cfg.ResendAPIKey, cfg.MailFrom)
			}
			// Le scelleur : certificat de l'organisme en production, certificat
			// auto-signé et explicitement nommé « sans valeur » en local.
			sealer, err := buildSealer(cfg)
			if err != nil {
				log.Error("scellement indisponible", "err", err)
				os.Exit(1)
			}

			deps.Signature = signature.NewService(signature.Deps{
				DB:       db,
				Renderer: docflow.NewRenderer(deps.Identity, deps.CRM, compiler),
				Blobs:    blob.NewMemory(),
				Mailer:   mailer,
				Sealer:   sealer,
				AppURL:   cfg.AppURL,
			})
		}
	}

	handler := httpapi.NewRouter(deps)

	if config.IsLambda() {
		log.Info("démarrage en lambda", "env", cfg.Env)
		adapter := httpadapter.NewV2(handler)
		lambda.Start(adapter.ProxyWithContext)
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
		if cfg.Env != config.EnvLocal {
			return nil, fmt.Errorf(
				"LEMLEARN_SEAL_CERT et LEMLEARN_SEAL_KEY sont requis en %s : "+
					"un document contractuel ne peut pas être scellé avec un certificat de développement", cfg.Env)
		}
		return seal.Development("Organisme de développement", timestamper)
	}

	cert, key, chain, err := seal.LoadKeyPair([]byte(cfg.SealCertPEM), []byte(cfg.SealKeyPEM))
	if err != nil {
		return nil, err
	}
	return seal.New(cert, key, chain, timestamper), nil
}
