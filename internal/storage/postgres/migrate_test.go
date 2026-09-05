//go:build integration

package postgres_test

import (
	"context"
	"testing"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"netcluster/internal/storage/postgres"
)

func TestRunMigrations_CreatesExpectedTablesAndConstraints(t *testing.T) {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("netcluster_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	if err := postgres.RunMigrations(dsn); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	db, err := postgres.OpenSQLDB(dsn)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	expectedTables := []string{
		"clusters", "rules", "network_elements",
		"discovery_jobs", "discovery_job_results", "assignment_history",
	}
	for _, table := range expectedTables {
		var exists bool
		err := db.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`,
			table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("failed to check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %q to exist after migration", table)
		}
	}

	// Running migrations a second time must be a safe no-op (idempotent startup).
	if err := postgres.RunMigrations(dsn); err != nil {
		t.Fatalf("RunMigrations should be idempotent, got error on second run: %v", err)
	}
}
