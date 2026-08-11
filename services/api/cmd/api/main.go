// Commande api : le service HTTP de lemlearn.
//
// Le même binaire sert l'API en local (net/http) et en production (Lambda
// derrière API Gateway HTTP v2). La détection est faite au démarrage : rien
// dans le code métier ne sait où il tourne.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/lemlearn/api/internal/config"
	"github.com/lemlearn/api/internal/httpapi"
	"github.com/lemlearn/api/internal/platform/doc"
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
