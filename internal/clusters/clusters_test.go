package clusters_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"netcluster/internal/clusters"
	"netcluster/internal/domain"
)

var errClusterNotFound = errors.New("cluster not found")

// fakeRepo is an in-memory stand-in for the Postgres-backed repo, letting
// clusters.Service's own logic (argument validation, delegation) be
// tested without a database. The repo's own concurrency-sensitive upsert
// behavior is covered separately by a testcontainers integration test in
// internal/storage/postgres, since that's the layer that can actually
// race.
type fakeRepo struct {
	byKey     map[string]domain.Cluster
	getOrCreateCalls int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byKey: map[string]domain.Cluster{}}
}

func (f *fakeRepo) GetOrCreateByLookupKey(ctx context.Context, lookupKey string) (domain.Cluster, error) {
	f.getOrCreateCalls++
	if c, ok := f.byKey[lookupKey]; ok {
		return c, nil
	}
	c := domain.Cluster{ID: uuid.New(), LookupKey: lookupKey, DisplayName: lookupKey}
	f.byKey[lookupKey] = c
	return c, nil
}

func (f *fakeRepo) Get(ctx context.Context, id uuid.UUID) (domain.Cluster, error) {
	for _, c := range f.byKey {
		if c.ID == id {
			return c, nil
		}
	}
	return domain.Cluster{}, errClusterNotFound
}

func (f *fakeRepo) List(ctx context.Context) ([]domain.ClusterWithCount, error) {
	var out []domain.ClusterWithCount
	for _, c := range f.byKey {
		out = append(out, domain.ClusterWithCount{Cluster: c})
	}
	return out, nil
}

func (f *fakeRepo) Rename(ctx context.Context, id uuid.UUID, displayName string, description *string) (domain.Cluster, error) {
	for key, c := range f.byKey {
		if c.ID == id {
			c.DisplayName = displayName
			if description != nil {
				c.Description = *description
			}
			f.byKey[key] = c
			return c, nil
		}
	}
	return domain.Cluster{}, errClusterNotFound
}

func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	for key, c := range f.byKey {
		if c.ID == id {
			delete(f.byKey, key)
			return nil
		}
	}
	return errClusterNotFound
}

func TestService_GetOrCreate_RejectsEmptyLookupKey(t *testing.T) {
	svc := clusters.NewService(newFakeRepo())

	_, err := svc.GetOrCreate(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty lookup key, got nil")
	}
}

func TestService_GetOrCreate_DelegatesToRepo(t *testing.T) {
	repo := newFakeRepo()
	svc := clusters.NewService(repo)

	c, err := svc.GetOrCreate(context.Background(), "dc1-core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.LookupKey != "dc1-core" {
		t.Errorf("expected lookup key dc1-core, got %q", c.LookupKey)
	}
	if repo.getOrCreateCalls != 1 {
		t.Errorf("expected repo to be called once, got %d", repo.getOrCreateCalls)
	}
}

func TestService_GetOrCreate_SameKeyReturnsSameCluster(t *testing.T) {
	svc := clusters.NewService(newFakeRepo())
	ctx := context.Background()

	first, err := svc.GetOrCreate(ctx, "dc1-core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := svc.GetOrCreate(ctx, "dc1-core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("expected same cluster ID for repeated key, got %v and %v", first.ID, second.ID)
	}
}
