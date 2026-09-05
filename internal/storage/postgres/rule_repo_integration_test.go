//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"netcluster/internal/dsl"
	"netcluster/internal/domain"
	"netcluster/internal/storage/postgres"
)

func TestRuleRepo_CreateGetUpdateDelete(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := postgres.NewRuleRepo(pool)

	created, err := repo.Create(ctx, domain.Rule{
		Name:               "core switches",
		Pattern:            `^sw-(?P<site>[a-z0-9]+)-core-\d+$`,
		ClusterKeyTemplate: "{site}-core",
		CaptureTransforms: map[string]dsl.Pipeline{
			"site": mustParsePipeline(t, "upper()"),
		},
		Priority:  10,
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID.String() == "" {
		t.Fatal("expected a generated ID")
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "core switches" || got.Priority != 10 || !got.IsEnabled {
		t.Errorf("unexpected rule after create: %+v", got)
	}
	if result, err := got.CaptureTransforms["site"].Apply("dc1"); err != nil || result != "DC1" {
		t.Errorf("expected persisted transform to upper-case, got %q err=%v", result, err)
	}

	got.Description = "updated description"
	got.IsEnabled = false
	updated, err := repo.Update(ctx, got)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Description != "updated description" || updated.IsEnabled {
		t.Errorf("unexpected rule after update: %+v", updated)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := repo.Get(ctx, created.ID); err == nil {
		t.Fatal("expected error getting a deleted rule, got nil")
	}
}

func TestRuleRepo_ListEnabledOrdered_ExcludesDisabledAndSortsByPriority(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := postgres.NewRuleRepo(pool)

	low, _ := repo.Create(ctx, domain.Rule{Name: "low", Pattern: "a", ClusterKeyTemplate: "x", Priority: 20, IsEnabled: true})
	high, _ := repo.Create(ctx, domain.Rule{Name: "high", Pattern: "b", ClusterKeyTemplate: "x", Priority: 10, IsEnabled: true})
	_, _ = repo.Create(ctx, domain.Rule{Name: "disabled", Pattern: "c", ClusterKeyTemplate: "x", Priority: 5, IsEnabled: false})

	rules, err := repo.ListEnabledOrdered(ctx)
	if err != nil {
		t.Fatalf("ListEnabledOrdered failed: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 enabled rules, got %d", len(rules))
	}
	if rules[0].ID != high.ID || rules[1].ID != low.ID {
		t.Fatalf("expected priority-ascending order [high, low], got %+v", rules)
	}
}

func TestRuleRepo_ListAllOrdered_IncludesDisabled(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := postgres.NewRuleRepo(pool)

	_, _ = repo.Create(ctx, domain.Rule{Name: "enabled", Pattern: "a", ClusterKeyTemplate: "x", Priority: 10, IsEnabled: true})
	_, _ = repo.Create(ctx, domain.Rule{Name: "disabled", Pattern: "b", ClusterKeyTemplate: "x", Priority: 20, IsEnabled: false})

	rules, err := repo.ListAllOrdered(ctx)
	if err != nil {
		t.Fatalf("ListAllOrdered failed: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules total, got %d", len(rules))
	}
}

func TestRuleRepo_Reorder_RewritesPriorities(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := postgres.NewRuleRepo(pool)

	a, _ := repo.Create(ctx, domain.Rule{Name: "a", Pattern: "a", ClusterKeyTemplate: "x", Priority: 10, IsEnabled: true})
	b, _ := repo.Create(ctx, domain.Rule{Name: "b", Pattern: "b", ClusterKeyTemplate: "x", Priority: 20, IsEnabled: true})

	// Reverse the order: b should now sort before a.
	if err := repo.Reorder(ctx, []uuid.UUID{b.ID, a.ID}); err != nil {
		t.Fatalf("Reorder failed: %v", err)
	}

	rules, err := repo.ListEnabledOrdered(ctx)
	if err != nil {
		t.Fatalf("ListEnabledOrdered failed: %v", err)
	}
	if len(rules) != 2 || rules[0].ID != b.ID || rules[1].ID != a.ID {
		t.Fatalf("expected reordered [b, a], got %+v", rules)
	}
}

func mustParsePipeline(t *testing.T, src string) dsl.Pipeline {
	t.Helper()
	p, err := dsl.Parse(src)
	if err != nil {
		t.Fatalf("failed to parse pipeline %q: %v", src, err)
	}
	return p
}
