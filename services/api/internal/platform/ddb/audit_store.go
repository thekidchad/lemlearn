package ddb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/lemlearn/api/internal/platform/audit"
)

// Write décrit une écriture métier à inclure dans une transaction.
type Write struct {
	// Item est l'entité à écrire, incorporant Record.
	Item any
	// Condition est une expression DynamoDB optionnelle, par exemple
	// "attribute_not_exists(PK)" pour une création, ou
	// "#s <> :signed" pour un verrouillage sur l'état courant.
	Condition string
	// Values complète Condition si elle référence des valeurs.
	Values map[string]types.AttributeValue
	// Names substitue les noms d'attributs réservés par DynamoDB — `status`
	// en est un, et une condition qui l'utilise directement échoue.
	Names map[string]string
}

// maxAuditAttempts borne les reprises en cas de course sur le rang d'un sujet.
const maxAuditAttempts = 4

// WriteWithAudit écrit les éléments métier **et** l'événement d'audit qui les
// décrit dans une seule TransactWriteItems.
//
// C'est l'invariant central du produit : il ne doit pas exister d'état dans la
// base dont la provenance ne soit pas journalisée. Deux écritures séparées, si
// bien intentionnées soient-elles, laissent une fenêtre où l'une réussit et
// l'autre échoue — et c'est toujours le journal qui manque.
//
// La construction de l'événement est confiée à `build`, qui reçoit le dernier
// événement connu du sujet : en cas de course sur le rang, l'opération est
// rejouée avec un rang à jour plutôt que d'écraser silencieusement l'existant.
func (c *Client) WriteWithAudit(
	ctx context.Context,
	subject string,
	writes []Write,
	build func(prev audit.Event) (audit.Event, error),
) (audit.Event, error) {
	events, err := c.WriteWithAuditChain(ctx, subject, writes,
		func(prev audit.Event) ([]audit.Event, error) {
			event, err := build(prev)
			if err != nil {
				return nil, err
			}
			return []audit.Event{event}, nil
		})
	if err != nil {
		return audit.Event{}, err
	}
	return events[0], nil
}

// WriteWithAuditChain écrit plusieurs événements liés dans la même
// transaction.
//
// Une opération produit parfois deux faits distincts qu'un auditeur doit voir
// séparément — la soumission d'un contrôle, puis la validation du module
// qu'elle débloque. Les écrire en deux transactions laisserait une fenêtre où
// l'état dit « module validé » sans que le journal ne le dise.
func (c *Client) WriteWithAuditChain(
	ctx context.Context,
	subject string,
	writes []Write,
	build func(prev audit.Event) ([]audit.Event, error),
) ([]audit.Event, error) {
	var lastErr error

	for attempt := 0; attempt < maxAuditAttempts; attempt++ {
		prev, err := c.LastAuditEvent(ctx, subject)
		if err != nil {
			return nil, err
		}

		events, err := build(prev)
		if err != nil {
			return nil, err
		}
		if len(events) == 0 {
			return nil, fmt.Errorf("ddb: écriture sans événement d'audit")
		}

		items := make([]types.TransactWriteItem, 0, len(writes)+len(events))
		for _, write := range writes {
			av, err := attributevalue.MarshalMap(write.Item)
			if err != nil {
				return nil, fmt.Errorf("ddb: encodage: %w", err)
			}
			put := &types.Put{TableName: aws.String(c.table), Item: av}
			if write.Condition != "" {
				put.ConditionExpression = aws.String(write.Condition)
				put.ExpressionAttributeValues = write.Values
				put.ExpressionAttributeNames = write.Names
			}
			items = append(items, types.TransactWriteItem{Put: put})
		}

		for _, event := range events {
			auditAV, err := attributevalue.MarshalMap(auditItem{
				PK:    AuditPK(subject),
				SK:    AuditSK(event.Seq),
				Event: event,
			})
			if err != nil {
				return nil, fmt.Errorf("ddb: encodage de l'audit: %w", err)
			}
			items = append(items, types.TransactWriteItem{Put: &types.Put{
				TableName: aws.String(c.auditTable),
				Item:      auditAV,
				// Le rang d'un sujet ne peut être occupé qu'une fois : c'est
				// ce qui empêche deux écritures concurrentes de se recouvrir
				// et de faire disparaître un événement de la chaîne.
				ConditionExpression: aws.String("attribute_not_exists(SK)"),
			}})
		}

		_, err = c.api.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
			TransactItems: items,
		})
		if err == nil {
			return events, nil
		}

		lastErr = wrapErr(err)
		if !errors.Is(lastErr, ErrConflict) {
			return nil, lastErr
		}
		// Un conflit peut venir du rang d'audit (course, on rejoue) comme
		// d'une condition métier (on abandonne). On ne peut pas les
		// distinguer sans coût : on rejoue, et la condition métier échouera
		// de nouveau, cette fois définitivement.
	}

	return nil, fmt.Errorf("ddb: écriture abandonnée après %d tentatives: %w", maxAuditAttempts, lastErr)
}

// LastAuditEvent renvoie le dernier événement d'un sujet, ou le zéro d'Event si
// la chaîne n'existe pas encore.
func (c *Client) LastAuditEvent(ctx context.Context, subject string) (audit.Event, error) {
	res, err := c.api.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(c.auditTable),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: AuditPK(subject)},
		},
		ScanIndexForward: aws.Bool(false),
		Limit:            aws.Int32(1),
		ConsistentRead:   aws.Bool(true),
	})
	if err != nil {
		return audit.Event{}, wrapErr(err)
	}
	if len(res.Items) == 0 {
		return audit.Event{}, nil
	}

	var item auditItem
	if err := attributevalue.UnmarshalMap(res.Items[0], &item); err != nil {
		return audit.Event{}, fmt.Errorf("ddb: décodage de l'audit: %w", err)
	}
	return item.Event, nil
}

// AuditChain renvoie la chaîne complète d'un sujet, dans l'ordre, **vérifiée**.
//
// La vérification est faite ici et non par l'appelant : il ne doit pas être
// possible d'afficher ou d'exporter un journal sans que son intégrité ait été
// contrôlée. Une chaîne rompue est une erreur, pas un avertissement.
func (c *Client) AuditChain(ctx context.Context, subject string) ([]audit.Event, error) {
	var events []audit.Event

	paginator := dynamodb.NewQueryPaginator(c.api, &dynamodb.QueryInput{
		TableName:              aws.String(c.auditTable),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: AuditPK(subject)},
		},
		ScanIndexForward: aws.Bool(true),
		ConsistentRead:   aws.Bool(true),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, wrapErr(err)
		}
		var batch []auditItem
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &batch); err != nil {
			return nil, fmt.Errorf("ddb: décodage de l'audit: %w", err)
		}
		for _, item := range batch {
			events = append(events, item.Event)
		}
	}

	if err := audit.Verify(events); err != nil {
		return nil, fmt.Errorf("journal de %s: %w", subject, err)
	}
	return events, nil
}

// TTL convertit une échéance en attribut de TTL DynamoDB.
func TTL(at time.Time) int64 { return at.Unix() }

// StringValues construit les valeurs d'expression d'une condition d'écriture.
//
// Aide volontairement limitée aux chaînes : les conditions dont dépend la
// cohérence du produit (étape d'un dossier, statut d'un jeton, horodatage de
// dernière modification) portent toutes sur des chaînes.
func StringValues(values map[string]string) map[string]types.AttributeValue {
	out := make(map[string]types.AttributeValue, len(values))
	for name, value := range values {
		out[name] = &types.AttributeValueMemberS{Value: value}
	}
	return out
}

// Write écrit plusieurs articles en une transaction, sans journal.
//
// Réservé à ce qui n'appartient à aucune chaîne de preuve : l'ouverture d'un
// compte, la réservation d'une adresse. Tout ce qui touche un dossier passe
// par WriteWithAudit — un fait probatoire écrit sans son événement serait
// invisible à l'audit.
func (c *Client) Write(ctx context.Context, writes []Write) error {
	if len(writes) == 0 {
		return nil
	}

	items := make([]types.TransactWriteItem, 0, len(writes))
	for _, write := range writes {
		av, err := attributevalue.MarshalMap(write.Item)
		if err != nil {
			return fmt.Errorf("ddb: encodage: %w", err)
		}
		put := &types.Put{TableName: aws.String(c.table), Item: av}
		if write.Condition != "" {
			put.ConditionExpression = aws.String(write.Condition)
			put.ExpressionAttributeValues = write.Values
			put.ExpressionAttributeNames = write.Names
		}
		items = append(items, types.TransactWriteItem{Put: put})
	}

	if _, err := c.api.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: items,
	}); err != nil {
		return wrapErr(err)
	}
	return nil
}
