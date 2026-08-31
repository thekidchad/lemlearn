package ddb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/lemlearn/api/internal/platform/audit"
)

// ErrNotFound est renvoyée quand un élément demandé n'existe pas.
var ErrNotFound = errors.New("ddb: élément introuvable")

// ErrConflict est renvoyée quand une condition d'écriture échoue : e-mail déjà
// pris, dossier modifié entre-temps, jeton déjà consommé.
var ErrConflict = errors.New("ddb: conflit d'écriture")

// Record est incorporé par toutes les entités persistées. Les champs de clé
// sont exportés car l'encodeur DynamoDB en a besoin, mais aucun code métier ne
// doit les renseigner à la main : c'est le rôle des constructeurs de clé.
type Record struct {
	// Les clés ne sortent jamais de l'API : elles décrivent la façon dont on
	// range les données, pas l'entité. Les exposer ferait entrer un détail de
	// stockage dans le contrat du client — et le premier qui s'en servirait
	// pour deviner une autre clé aurait raison d'essayer.
	PK     string `dynamodbav:"PK" json:"-"`
	SK     string `dynamodbav:"SK" json:"-"`
	GSI1PK string `dynamodbav:"GSI1PK,omitempty" json:"-"`
	GSI1SK string `dynamodbav:"GSI1SK,omitempty" json:"-"`
	GSI2PK string `dynamodbav:"GSI2PK,omitempty" json:"-"`
	GSI2SK string `dynamodbav:"GSI2SK,omitempty" json:"-"`

	// Type nomme l'entité. Il ne sert pas au routage — la clé de tri suffit —
	// mais rend les exports et les journaux lisibles sans décodage.
	Type string `dynamodbav:"Type" json:"type,omitempty"`

	CreatedAt time.Time `dynamodbav:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `dynamodbav:"updatedAt" json:"updatedAt"`

	// ExpiresAt alimente le TTL DynamoDB, en secondes Unix. Renseigné
	// uniquement sur les éléments à durée de vie bornée : sessions, codes à
	// usage unique, heartbeats bruts.
	ExpiresAt int64 `dynamodbav:"expiresAt,omitempty" json:"-"`
}

// Client encapsule l'accès aux deux tables.
type Client struct {
	api        *dynamodb.Client
	table      string
	auditTable string
}

// New construit un client à partir de la configuration AWS ambiante.
//
// DDB_ENDPOINT permet de viser DynamoDB Local : les tests d'intégration
// tournent contre le vrai moteur, jamais contre une imitation en mémoire dont
// les conditions d'écriture se comporteraient différemment.
func New(ctx context.Context, table, auditTable string) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if endpoint := os.Getenv("DDB_ENDPOINT"); endpoint != "" {
		opts = append(opts,
			awsconfig.WithRegion(orDefaultRegion()),
			awsconfig.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider("local", "local", "")),
		)
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("ddb: configuration aws: %w", err)
	}

	api := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		if endpoint := os.Getenv("DDB_ENDPOINT"); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	return &Client{api: api, table: table, auditTable: auditTable}, nil
}

// API expose le client brut pour les cas non couverts par les aides.
func (c *Client) API() *dynamodb.Client { return c.api }

// Table renvoie le nom de la table principale.
func (c *Client) Table() string { return c.table }

// AuditTable renvoie le nom de la table d'audit.
func (c *Client) AuditTable() string { return c.auditTable }

func orDefaultRegion() string {
	if region := os.Getenv("AWS_REGION"); region != "" {
		return region
	}
	return "eu-west-3"
}

// Put écrit un élément sans condition.
func Put[T any](ctx context.Context, c *Client, item T) error {
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("ddb: encodage: %w", err)
	}
	_, err = c.api.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.table),
		Item:      av,
	})
	return wrapErr(err)
}

// PutNew écrit un élément à condition qu'il n'existe pas déjà.
//
// C'est la primitive d'unicité : création d'organisation, réservation d'un
// e-mail, consommation d'un jeton à usage unique.
func PutNew[T any](ctx context.Context, c *Client, item T) error {
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("ddb: encodage: %w", err)
	}
	_, err = c.api.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(c.table),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	return wrapErr(err)
}

// Get lit un élément par sa clé complète.
func Get[T any](ctx context.Context, c *Client, pk, sk string) (T, error) {
	var out T
	res, err := c.api.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		// Lecture fortement cohérente : après une écriture, l'utilisateur
		// doit voir son changement. Une lecture éventuellement cohérente
		// produirait des « mon dossier a disparu » impossibles à déboguer.
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return out, wrapErr(err)
	}
	if res.Item == nil {
		return out, ErrNotFound
	}
	if err := attributevalue.UnmarshalMap(res.Item, &out); err != nil {
		return out, fmt.Errorf("ddb: décodage: %w", err)
	}
	return out, nil
}

// GetRaw lit un élément sous forme de dictionnaire.
//
// Utile lorsqu'un paquet doit modifier un champ d'une entité qui appartient à
// un autre domaine — le catalogue rattachant un dossier à sa session — sans
// importer son type et créer un cycle de dépendances. La contrepartie est
// assumée : les noms d'attributs sont écrits en toutes lettres, donc à tenir
// à jour si le schéma change.
func GetRaw(ctx context.Context, c *Client, pk, sk string) (map[string]any, error) {
	res, err := c.api.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, wrapErr(err)
	}
	if res.Item == nil {
		return nil, ErrNotFound
	}

	var out map[string]any
	if err := attributevalue.UnmarshalMap(res.Item, &out); err != nil {
		return nil, fmt.Errorf("ddb: décodage: %w", err)
	}
	return out, nil
}

// QuerySpec décrit une requête sur la table ou l'un de ses index.
type QuerySpec struct {
	// Index vide interroge la table principale.
	Index string
	// PK est la valeur de la clé de partition de l'index visé.
	PK string
	// SKPrefix restreint aux clés de tri commençant par ce préfixe.
	SKPrefix string
	// Descending inverse l'ordre — le plus récent d'abord.
	Descending bool
	Limit      int32
}

// Query lit une collection d'éléments.
func Query[T any](ctx context.Context, c *Client, spec QuerySpec) ([]T, error) {
	pkName, skName := "PK", "SK"
	switch spec.Index {
	case "GSI1":
		pkName, skName = "GSI1PK", "GSI1SK"
	case "GSI2":
		pkName, skName = "GSI2PK", "GSI2SK"
	}

	condition := "#pk = :pk"
	names := map[string]string{"#pk": pkName}
	values := map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: spec.PK}}
	if spec.SKPrefix != "" {
		condition += " AND begins_with(#sk, :skp)"
		names["#sk"] = skName
		values[":skp"] = &types.AttributeValueMemberS{Value: spec.SKPrefix}
	}

	input := &dynamodb.QueryInput{
		TableName:                 aws.String(c.table),
		KeyConditionExpression:    aws.String(condition),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
		ScanIndexForward:          aws.Bool(!spec.Descending),
	}
	if spec.Index != "" {
		input.IndexName = aws.String(spec.Index)
	}
	if spec.Limit > 0 {
		input.Limit = aws.Int32(spec.Limit)
	}

	var out []T
	paginator := dynamodb.NewQueryPaginator(c.api, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, wrapErr(err)
		}
		var batch []T
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &batch); err != nil {
			return nil, fmt.Errorf("ddb: décodage: %w", err)
		}
		out = append(out, batch...)
		if spec.Limit > 0 && int32(len(out)) >= spec.Limit {
			return out[:spec.Limit], nil
		}
	}
	return out, nil
}

// Delete supprime un élément de la table principale.
//
// Il n'existe volontairement aucune aide de suppression pour la table d'audit.
func Delete(ctx context.Context, c *Client, pk, sk string) error {
	_, err := c.api.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(c.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})
	return wrapErr(err)
}

func wrapErr(err error) error {
	if err == nil {
		return nil
	}
	var conditionFailed *types.ConditionalCheckFailedException
	if errors.As(err, &conditionFailed) {
		return ErrConflict
	}
	var transactionCanceled *types.TransactionCanceledException
	if errors.As(err, &transactionCanceled) {
		for _, reason := range transactionCanceled.CancellationReasons {
			if reason.Code != nil && *reason.Code == "ConditionalCheckFailed" {
				return ErrConflict
			}
		}
	}
	return err
}

// auditItem encode un événement d'audit en élément DynamoDB.
type auditItem struct {
	PK string `dynamodbav:"PK"`
	SK string `dynamodbav:"SK"`
	// Les clés de l'index chronologique. Elles ne participent pas au calcul de
	// l'empreinte — celle-ci ne porte que sur l'événement — donc les ajouter
	// ne rompt aucune chaîne existante.
	GSI1PK string `dynamodbav:"GSI1PK,omitempty"`
	GSI1SK string `dynamodbav:"GSI1SK,omitempty"`
	audit.Event
}

// Increment ajoute une valeur à un compteur, en créant l'article au besoin.
//
// C'est la seule écriture non transactionnelle du produit, et elle ne porte
// que des compteurs d'usage : un compteur de facturation doit être exact sous
// concurrence, ce qu'un lire-modifier-écrire ne garantit pas, et ADD est
// atomique côté moteur.
func (c *Client) Increment(ctx context.Context, pk, sk, field string, by int64) (int64, error) {
	out, err := c.api.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("ADD #f :n"),
		ExpressionAttributeNames: map[string]string{
			"#f": field,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":n": &types.AttributeValueMemberN{Value: strconv.FormatInt(by, 10)},
		},
		ReturnValues: types.ReturnValueUpdatedNew,
	})
	if err != nil {
		return 0, wrapErr(err)
	}

	value, ok := out.Attributes[field].(*types.AttributeValueMemberN)
	if !ok {
		return 0, nil
	}
	total, err := strconv.ParseInt(value.Value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ddb: compteur %s illisible: %w", field, err)
	}
	return total, nil
}
