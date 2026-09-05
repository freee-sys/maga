package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"netcluster/internal/domain"
)

type ClusterRepo struct {
	db *pgxpool.Pool
}

func NewClusterRepo(db *pgxpool.Pool) *ClusterRepo {
	return &ClusterRepo{db: db}
}

// GetOrCreateByLookupKey is a race-safe upsert: concurrent callers that
// independently compute the same brand-new key must end up agreeing on
// exactly one cluster row. The no-op DO UPDATE is the standard Postgres
// trick to force RETURNING on a conflict.
func (r *ClusterRepo) GetOrCreateByLookupKey(ctx context.Context, lookupKey string) (domain.Cluster, error) {
	var c domain.Cluster
	err := r.db.QueryRow(ctx, `
		INSERT INTO clusters (lookup_key, display_name)
		VALUES ($1, $1)
		ON CONFLICT (lookup_key) DO UPDATE SET lookup_key = EXCLUDED.lookup_key
		RETURNING id, lookup_key, display_name, COALESCE(description, '')
	`, lookupKey).Scan(&c.ID, &c.LookupKey, &c.DisplayName, &c.Description)
	if err != nil {
		return domain.Cluster{}, fmt.Errorf("get or create cluster %q: %w", lookupKey, err)
	}
	return c, nil
}

// ExistsClusterByLookupKey is a read-only check used by the rule tester
// to report whether a candidate rule would create a brand-new cluster,
// without ever writing one.
func (r *ClusterRepo) ExistsClusterByLookupKey(ctx context.Context, lookupKey string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clusters WHERE lookup_key = $1)`, lookupKey).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check cluster exists %q: %w", lookupKey, err)
	}
	return exists, nil
}

func (r *ClusterRepo) Get(ctx context.Context, id uuid.UUID) (domain.Cluster, error) {
	var c domain.Cluster
	err := r.db.QueryRow(ctx, `
		SELECT id, lookup_key, display_name, COALESCE(description, '')
		FROM clusters WHERE id = $1
	`, id).Scan(&c.ID, &c.LookupKey, &c.DisplayName, &c.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Cluster{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Cluster{}, fmt.Errorf("get cluster %s: %w", id, err)
	}
	return c, nil
}

func (r *ClusterRepo) List(ctx context.Context) ([]domain.ClusterWithCount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.lookup_key, c.display_name, COALESCE(c.description, ''),
		       COUNT(e.id) FILTER (WHERE e.deleted_at IS NULL)
		FROM clusters c
		LEFT JOIN network_elements e ON e.cluster_id = c.id
		GROUP BY c.id
		ORDER BY c.display_name
	`)
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}
	defer rows.Close()

	var out []domain.ClusterWithCount
	for rows.Next() {
		var cwc domain.ClusterWithCount
		if err := rows.Scan(&cwc.ID, &cwc.LookupKey, &cwc.DisplayName, &cwc.Description, &cwc.MemberCount); err != nil {
			return nil, fmt.Errorf("scan cluster row: %w", err)
		}
		out = append(out, cwc)
	}
	return out, rows.Err()
}

func (r *ClusterRepo) Rename(ctx context.Context, id uuid.UUID, displayName string, description *string) (domain.Cluster, error) {
	var c domain.Cluster
	err := r.db.QueryRow(ctx, `
		UPDATE clusters
		SET display_name = $2,
		    description = COALESCE($3, description),
		    updated_at = now()
		WHERE id = $1
		RETURNING id, lookup_key, display_name, COALESCE(description, '')
	`, id, displayName, description).Scan(&c.ID, &c.LookupKey, &c.DisplayName, &c.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Cluster{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Cluster{}, fmt.Errorf("rename cluster %s: %w", id, err)
	}
	return c, nil
}

func (r *ClusterRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM clusters WHERE id = $1`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return domain.ErrClusterNotEmpty
		}
		return fmt.Errorf("delete cluster %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
