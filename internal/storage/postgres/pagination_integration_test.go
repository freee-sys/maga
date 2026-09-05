//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"netcluster/internal/domain"
	"netcluster/internal/storage/postgres"
)

func elementFilter(limit, offset int) domain.ElementFilter {
	return domain.ElementFilter{Limit: limit, Offset: offset}
}

func TestElementRepo_List_RespectsLimitAndOffset(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	repo := postgres.NewElementRepo(pool)

	for i := 0; i < 5; i++ {
		if _, err := repo.Create(ctx, fmt.Sprintf("10.0.0.%d", i+1), fmt.Sprintf("sw-%d", i+1)); err != nil {
			t.Fatalf("failed to seed element %d: %v", i, err)
		}
	}

	page, err := repo.List(ctx, elementFilter(2, 0))
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("expected 2 elements on first page, got %d", len(page))
	}

	next, err := repo.List(ctx, elementFilter(2, 2))
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(next) != 2 {
		t.Fatalf("expected 2 elements on second page, got %d", len(next))
	}
	if page[0].ID == next[0].ID {
		t.Error("expected different elements on different pages")
	}

	all, err := repo.List(ctx, elementFilter(0, 0))
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("expected all 5 elements when limit=0, got %d", len(all))
	}
}
