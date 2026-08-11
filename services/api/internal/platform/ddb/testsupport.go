package ddb

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// NewTestClient monte deux tables jetables sur DynamoDB Local et renvoie un
// client dessus.
//
// Les tests d'intégration tournent contre le vrai moteur, jamais contre une
// imitation en mémoire : la moitié de ce que ce paquet exploite — conditions
// d'écriture, transactions multi-tables, cohérence forte — n'existe pas dans
// un faux client, et ce sont précisément les mécanismes sur lesquels repose
// l'intégrité du journal.
//
//	docker run -d -p 8100:8000 amazon/dynamodb-local \
//	  -jar DynamoDBLocal.jar -inMemory -sharedDb
//	DDB_ENDPOINT=http://localhost:8100 go test ./...
func NewTestClient(t *testing.T) *Client {
	t.Helper()

	if os.Getenv("DDB_ENDPOINT") == "" {
		t.Skip("DDB_ENDPOINT absent : démarrez DynamoDB Local (voir doc de NewTestClient)")
	}

	ctx := context.Background()
	suffix := fmt.Sprintf("%s-%d", sanitize(t.Name()), os.Getpid())
	table := "test-" + suffix
	auditTable := "test-audit-" + suffix

	client, err := New(ctx, table, auditTable)
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	if err := CreateMainTable(ctx, client.api, table); err != nil {
		t.Fatalf("création de %s: %v", table, err)
	}
	if err := CreateAuditTable(ctx, client.api, auditTable); err != nil {
		t.Fatalf("création de %s: %v", auditTable, err)
	}

	t.Cleanup(func() {
		for _, name := range []string{table, auditTable} {
			_, _ = client.api.DeleteTable(ctx, &dynamodb.DeleteTableInput{TableName: aws.String(name)})
		}
	})

	return client
}

// sanitize réduit un nom de test à ce qu'accepte DynamoDB comme nom de table.
func sanitize(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '-')
		}
	}
	return string(out)
}
