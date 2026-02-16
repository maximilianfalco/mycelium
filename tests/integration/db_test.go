package integration

import (
	"context"
	"os"
	"testing"

	"github.com/maximilianfalco/mycelium/internal/db"
)

func TestDatabaseConnection(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://mycelium:mycelium@localhost:5433/mycelium"
	}

	ctx := context.Background()

	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	if err := db.HealthCheck(ctx, pool); err != nil {
		t.Fatalf("health check failed: %v", err)
	}
}

func TestTablesExist(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://mycelium:mycelium@localhost:5433/mycelium"
	}

	ctx := context.Background()

	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	expected := []string{"projects", "project_sources", "workspaces", "packages", "nodes", "edges", "unresolved_refs"}
	for _, table := range expected {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)",
			table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("failed to check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s does not exist", table)
		}
	}
}
