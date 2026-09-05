//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"netcluster/internal/assignment"
	"netcluster/internal/domain"
	"netcluster/internal/storage/postgres"
)

func TestAssignment_HostnameToAssignedClusterAndHistory(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	elements := postgres.NewElementRepo(pool)
	clusterRepo := postgres.NewClusterRepo(pool)
	history := postgres.NewHistoryRepo(pool)
	svc := assignment.NewService(elements, clusterRepo, history)

	el, err := elements.Create(ctx, "10.0.0.5", "sw-dc1-core-01")
	if err != nil {
		t.Fatalf("failed to create fixture element: %v", err)
	}

	// RuleRepo doesn't exist yet (that's milestone 7); network_elements.
	// last_rule_id has a real FK to rules, so insert the fixture directly.
	rule := domain.Rule{
		ID: uuid.New(), IsEnabled: true,
		Pattern:            `^sw-(?P<site>[a-z0-9]+)-core-\d+$`,
		ClusterKeyTemplate: "{site}-core",
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO rules (id, name, pattern, cluster_key_template, priority)
		VALUES ($1, 'test-rule', $2, $3, 1)
	`, rule.ID, rule.Pattern, rule.ClusterKeyTemplate)
	if err != nil {
		t.Fatalf("failed to insert fixture rule: %v", err)
	}

	result, err := svc.Assign(ctx, el.ID, el.SysName, []domain.Rule{rule}, domain.TriggerSourceDiscovery)
	if err != nil {
		t.Fatalf("Assign failed: %v", err)
	}
	if result.RenderedClusterKey != "dc1-core" {
		t.Fatalf("expected cluster key dc1-core, got %q", result.RenderedClusterKey)
	}

	got, err := elements.Get(ctx, el.ID)
	if err != nil {
		t.Fatalf("failed to reload element: %v", err)
	}
	if got.AssignmentStatus != domain.AssignmentStatusAssigned {
		t.Errorf("expected assignment_status assigned, got %q", got.AssignmentStatus)
	}
	if got.ClusterID == nil {
		t.Fatal("expected element to have a cluster_id set")
	}

	cluster, err := clusterRepo.Get(ctx, *got.ClusterID)
	if err != nil {
		t.Fatalf("failed to load cluster: %v", err)
	}
	if cluster.LookupKey != "dc1-core" {
		t.Errorf("expected lookup_key dc1-core, got %q", cluster.LookupKey)
	}

	rows, err := history.ListByElement(ctx, el.ID)
	if err != nil {
		t.Fatalf("failed to list history: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly one history row, got %d", len(rows))
	}
	if rows[0].RenderedClusterKey != "dc1-core" || rows[0].TriggerSource != domain.TriggerSourceDiscovery {
		t.Errorf("unexpected history row: %+v", rows[0])
	}
}

// TestClusterRepo_GetOrCreateByLookupKey_ConcurrentSameKeyRace verifies the
// upsert is race-safe: many goroutines independently computing the same
// brand-new key must converge on exactly one cluster row. This needs a
// real database, not a fake — it is specifically testing Postgres's
// ON CONFLICT behavior under concurrent writers, not application logic.
func TestClusterRepo_GetOrCreateByLookupKey_ConcurrentSameKeyRace(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := postgres.NewClusterRepo(pool)

	const workers = 25
	ids := make([]uuid.UUID, workers)
	errs := make([]error, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			c, err := repo.GetOrCreateByLookupKey(ctx, "race-key")
			ids[i] = c.ID
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: unexpected error: %v", i, err)
		}
	}
	first := ids[0]
	for i, id := range ids {
		if id != first {
			t.Errorf("worker %d got cluster id %v, expected %v (all workers must agree)", i, id, first)
		}
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM clusters WHERE lookup_key = $1`, "race-key").Scan(&count); err != nil {
		t.Fatalf("failed to count clusters: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 cluster row for the race key, found %d", count)
	}
}
