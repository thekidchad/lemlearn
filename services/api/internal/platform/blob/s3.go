package blob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3 range les fichiers dans un compartiment Amazon S3.
//
// Les documents scellés vont dans un compartiment sous Object Lock : une fois
// écrits, ils ne peuvent plus être ni modifiés ni supprimés avant l'échéance
// de rétention, pas même par le compte racine. C'est ce qui distingue un
// archivage d'un simple stockage.
type S3 struct {
	api    *s3.Client
	bucket string
}

// NewS3 construit le magasin.
func NewS3(ctx context.Context, bucket string) (*S3, error) {
	if bucket == "" {
		return nil, fmt.Errorf("blob: nom de compartiment vide")
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("blob: configuration aws: %w", err)
	}

	api := s3.NewFromConfig(cfg, func(o *s3.Options) {
		// S3_ENDPOINT vise un émulateur local (MinIO, LocalStack). Le style
		// « chemin » est nécessaire : un émulateur ne résout pas les
		// sous-domaines par compartiment.
		if endpoint := os.Getenv("S3_ENDPOINT"); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
	})

	return &S3{api: api, bucket: bucket}, nil
}

// Put range un contenu.
//
// Aucune condition d'écrasement n'est posée ici : c'est la politique Object
// Lock du compartiment qui refuse la réécriture d'un document scellé, et elle
// le fait de façon opposable — une vérification applicative ne le serait pas.
func (s *S3) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := s.api.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		// Le SDK calcule l'empreinte et S3 la vérifie : une corruption en
		// transit est rejetée plutôt qu'archivée.
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
	})
	if err != nil {
		return fmt.Errorf("blob: écriture de %s: %w", key, err)
	}
	return nil
}

// Get relit un contenu.
func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	res, err := s.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var missing *types.NoSuchKey
		if errors.As(err, &missing) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("blob: lecture de %s: %w", key, err)
	}
	defer func() { _ = res.Body.Close() }()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("blob: lecture de %s: %w", key, err)
	}
	return data, nil
}

// PresignedGet produit une URL de lecture temporaire.
//
// C'est ainsi que sont servies les pièces d'identité : jamais en direct,
// jamais durablement. Soixante secondes suffisent à afficher un document et
// ne laissent pas de lien exploitable dans un historique de navigation.
func (s *S3) PresignedGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	presigner := s3.NewPresignClient(s.api)
	request, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("blob: signature de l'url de %s: %w", key, err)
	}
	return request.URL, nil
}

// PresignedPut produit une URL de téléversement temporaire.
//
// Les vidéos et les pièces d'identité montent directement vers S3 : les faire
// transiter par l'API imposerait de dimensionner la Lambda pour des fichiers
// dont elle n'a rien à faire.
func (s *S3) PresignedPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	presigner := s3.NewPresignClient(s.api)
	request, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("blob: signature de l'url de dépôt de %s: %w", key, err)
	}
	return request.URL, nil
}
