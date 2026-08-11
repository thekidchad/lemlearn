// Commande devsetup : monte les tables DynamoDB de développement local.
//
// Les tables sont créées avec exactement la même définition qu'en test et
// qu'en CDK. Elle est sans effet si les tables existent déjà, donc réexécutable
// sans précaution.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/lemlearn/api/internal/config"
	"github.com/lemlearn/api/internal/platform/ddb"
)

func main() {
	if os.Getenv("DDB_ENDPOINT") == "" {
		fmt.Fprintln(os.Stderr, "DDB_ENDPOINT absent. Démarrez DynamoDB Local :")
		fmt.Fprintln(os.Stderr, "  docker run -d --name lemlearn-ddb -p 8100:8000 amazon/dynamodb-local \\")
		fmt.Fprintln(os.Stderr, "    -jar DynamoDBLocal.jar -inMemory -sharedDb")
		fmt.Fprintln(os.Stderr, "  export DDB_ENDPOINT=http://localhost:8100")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration : %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	client, err := ddb.New(ctx, cfg.Table, cfg.AuditTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connexion : %v\n", err)
		os.Exit(1)
	}

	if err := create(ctx, cfg.Table, ddb.CreateMainTable(ctx, client.API(), cfg.Table)); err != nil {
		os.Exit(1)
	}
	if err := create(ctx, cfg.AuditTable, ddb.CreateAuditTable(ctx, client.API(), cfg.AuditTable)); err != nil {
		os.Exit(1)
	}
}

func create(ctx context.Context, name string, err error) error {
	var exists *types.ResourceInUseException
	switch {
	case err == nil:
		fmt.Printf("table %s créée\n", name)
	case errors.As(err, &exists):
		fmt.Printf("table %s déjà présente\n", name)
	default:
		fmt.Fprintf(os.Stderr, "table %s : %v\n", name, err)
		return err
	}
	return nil
}
