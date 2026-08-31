package ddb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Pagination par curseur.
//
// DynamoDB ne sait pas sauter à la septième page : il n'existe pas d'offset,
// parce qu'atteindre le millième élément supposerait de lire les neuf cent
// quatre-vingt-dix-neuf premiers — exactement le coût qu'on cherche à éviter.
// Ce qu'il rend, c'est la clé du dernier élément lu, à représenter au tour
// suivant.
//
// Le curseur est donc opaque et se suit vers l'avant. C'est aussi pourquoi
// aucune écran ne peut afficher « page 3 sur 12 » : le nombre total n'est pas
// connu sans tout compter, et l'annoncer serait un mensonge coûteux.

// Page porte une tranche de résultats et de quoi demander la suivante.
type Page[T any] struct {
	Items []T `json:"items"`
	// Cursor est vide quand il n'y a plus rien après : c'est le seul signal
	// fiable de fin de liste. Une tranche plus courte que la limite demandée
	// n'en est pas un — DynamoDB borne aussi par la taille lue.
	Cursor string `json:"cursor,omitempty"`
}

// QueryPage lit une tranche et rend le curseur de la suivante.
func QueryPage[T any](ctx context.Context, c *Client, spec QuerySpec, cursor string) (Page[T], error) {
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

	if cursor != "" {
		start, err := decodeCursor(cursor)
		if err != nil {
			return Page[T]{}, err
		}
		// La partition est réimposée depuis la requête, jamais reprise du
		// curseur. DynamoDB refuserait de toute façon une clé de départ hors
		// du domaine interrogé, mais le garde-fou vaut mieux ici qu'à
		// distance : un curseur est une valeur qui vient du navigateur.
		start[pkName] = &types.AttributeValueMemberS{Value: spec.PK}
		input.ExclusiveStartKey = start
	}

	out, err := c.api.Query(ctx, input)
	if err != nil {
		return Page[T]{}, wrapErr(err)
	}

	var items []T
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &items); err != nil {
		return Page[T]{}, fmt.Errorf("ddb: décodage: %w", err)
	}

	page := Page[T]{Items: items}
	if len(out.LastEvaluatedKey) > 0 {
		page.Cursor, err = encodeCursor(out.LastEvaluatedKey)
		if err != nil {
			return Page[T]{}, err
		}
	}
	return page, nil
}

// encodeCursor sérialise la clé de reprise.
//
// Toutes les clés de ce schéma sont des chaînes : on n'encode donc que des
// chaînes, ce qui évite d'embarquer la représentation des types DynamoDB dans
// un jeton qui traverse le navigateur.
func encodeCursor(key map[string]types.AttributeValue) (string, error) {
	plat := make(map[string]string, len(key))
	for nom, valeur := range key {
		s, ok := valeur.(*types.AttributeValueMemberS)
		if !ok {
			return "", fmt.Errorf("ddb: clé de reprise non textuelle sur %q", nom)
		}
		plat[nom] = s.Value
	}
	encoded, err := json.Marshal(plat)
	if err != nil {
		return "", fmt.Errorf("ddb: encodage du curseur: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeCursor(cursor string) (map[string]types.AttributeValue, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("curseur illisible")
	}
	var plat map[string]string
	if err := json.Unmarshal(raw, &plat); err != nil {
		return nil, fmt.Errorf("curseur illisible")
	}
	// Une clé de reprise ne porte jamais plus de quatre attributs — deux pour
	// la table, deux pour l'index. Au-delà, c'est une valeur fabriquée.
	if len(plat) == 0 || len(plat) > 4 {
		return nil, fmt.Errorf("curseur illisible")
	}

	key := make(map[string]types.AttributeValue, len(plat))
	for nom, valeur := range plat {
		key[nom] = &types.AttributeValueMemberS{Value: valeur}
	}
	return key, nil
}
