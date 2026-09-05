// Package clusters provides the GetOrCreate/rename/list logic for
// dynamically created clusters, against a Repo interface implemented by
// internal/storage/postgres.
package clusters

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"netcluster/internal/domain"
)

// Repo is the persistence interface this package needs. The Postgres
// implementation's GetOrCreateByLookupKey must be a race-safe upsert:
// concurrent discovery workers can independently compute the same
// brand-new cluster key at the same time.
type Repo interface {
	GetOrCreateByLookupKey(ctx context.Context, lookupKey string) (domain.Cluster, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Cluster, error)
	List(ctx context.Context) ([]domain.ClusterWithCount, error)
	Rename(ctx context.Context, id uuid.UUID, displayName string, description *string) (domain.Cluster, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

// GetOrCreate looks up a cluster by its rendered key, creating it (with
// display_name defaulted to the key) on first use.
func (s *Service) GetOrCreate(ctx context.Context, lookupKey string) (domain.Cluster, error) {
	if lookupKey == "" {
		return domain.Cluster{}, errors.New("lookup key must not be empty")
	}
	return s.repo.GetOrCreateByLookupKey(ctx, lookupKey)
}
