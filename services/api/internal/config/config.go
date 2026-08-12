// Package config lit la configuration du service depuis l'environnement.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Env distingue les environnements de déploiement.
type Env string

const (
	EnvLocal   Env = "local"
	EnvDev     Env = "dev"
	EnvStaging Env = "staging"
	EnvProd    Env = "prod"
)

// Config regroupe tout ce que le service lit à l'environnement. Aucun autre
// paquet ne doit appeler os.Getenv : une valeur manquante doit faire échouer
// le démarrage, pas se manifester au premier appel d'un client.
type Config struct {
	Env  Env
	Port string

	// Tables DynamoDB. La table d'audit est séparée pour que sa politique
	// IAM puisse interdire UpdateItem et DeleteItem sans affecter le reste.
	Table      string
	AuditTable string

	// Compartiments S3. Les documents scellés vivent sous Object Lock, les
	// pièces d'identité dans un compartiment chiffré par clé dédiée à durée
	// de conservation courte.
	DocumentsBucket string
	IdentityBucket  string
	VideoBucket     string

	// URL publique de l'application, utilisée pour composer les liens de
	// signature envoyés par e-mail.
	AppURL string

	ResendAPIKey string
	MailFrom     string

	// Autorité d'horodatage RFC 3161. FreeTSA en développement, une autorité
	// qualifiée eIDAS en production.
	TSAURL string

	// Certificat de cachet d'organisation, au format PEM, et sa clé privée.
	// Ils viennent de Secrets Manager : jamais du dépôt, jamais d'un fichier
	// posé à côté du binaire.
	SealCertPEM string
	SealKeyPEM  string
}

// Load lit la configuration et refuse de démarrer si l'essentiel manque.
func Load() (Config, error) {
	cfg := Config{
		Env:             Env(orDefault("LEMLEARN_ENV", string(EnvLocal))),
		Port:            orDefault("PORT", "8787"),
		Table:           orDefault("LEMLEARN_TABLE", "lemlearn-local"),
		AuditTable:      orDefault("LEMLEARN_AUDIT_TABLE", "lemlearn-audit-local"),
		DocumentsBucket: os.Getenv("LEMLEARN_DOCUMENTS_BUCKET"),
		IdentityBucket:  os.Getenv("LEMLEARN_IDENTITY_BUCKET"),
		VideoBucket:     os.Getenv("LEMLEARN_VIDEO_BUCKET"),
		AppURL:          orDefault("LEMLEARN_APP_URL", "http://localhost:3000"),
		ResendAPIKey:    os.Getenv("RESEND_API_KEY"),
		MailFrom:        orDefault("LEMLEARN_MAIL_FROM", "lemlearn <ne-pas-repondre@lemlearn.fr>"),
		TSAURL:          orDefault("LEMLEARN_TSA_URL", "https://freetsa.org/tsr"),
		SealCertPEM:     os.Getenv("LEMLEARN_SEAL_CERT"),
		SealKeyPEM:      os.Getenv("LEMLEARN_SEAL_KEY"),
	}

	switch cfg.Env {
	case EnvLocal, EnvDev, EnvStaging, EnvProd:
	default:
		return Config{}, fmt.Errorf("config: LEMLEARN_ENV=%q inconnu", cfg.Env)
	}

	// En local on tolère l'absence des ressources AWS : la génération de
	// documents et les tests de gabarit doivent tourner sans compte AWS.
	if cfg.Env != EnvLocal {
		missing := []string{}
		for name, value := range map[string]string{
			"LEMLEARN_DOCUMENTS_BUCKET": cfg.DocumentsBucket,
			"LEMLEARN_IDENTITY_BUCKET":  cfg.IdentityBucket,
			"LEMLEARN_VIDEO_BUCKET":     cfg.VideoBucket,
			"RESEND_API_KEY":            cfg.ResendAPIKey,
		} {
			if strings.TrimSpace(value) == "" {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			return Config{}, fmt.Errorf("config: variables manquantes en %s: %s", cfg.Env, strings.Join(missing, ", "))
		}
	}

	return cfg, nil
}

// IsLambda indique si le processus tourne dans un environnement Lambda.
func IsLambda() bool {
	return os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != ""
}

func orDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
