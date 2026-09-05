package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"netcluster/internal/domain"
)

type DiscoveryResultRepo struct {
	db *pgxpool.Pool
}

func NewDiscoveryResultRepo(db *pgxpool.Pool) *DiscoveryResultRepo {
	return &DiscoveryResultRepo{db: db}
}

func (r *DiscoveryResultRepo) InsertResult(ctx context.Context, result domain.DiscoveryJobResult) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO discovery_job_results
			(id, job_id, ip_address, status, sys_name, sys_descr, sys_object_id, error_message, element_id, responded_at)
		VALUES ($1, $2, $3::inet, $4, $5, $6, $7, NULLIF($8, ''), $9, $10)
	`, result.ID, result.JobID, result.IPAddress, result.Status, result.SysName, result.SysDescr,
		result.SysObjectID, result.ErrorMessage, result.ElementID, result.RespondedAt)
	if err != nil {
		return fmt.Errorf("insert discovery result for %s: %w", result.IPAddress, err)
	}
	return nil
}

// ListByJob returns a job's per-IP results ordered by IP. limit == 0
// means no limit (used by tests and any future non-paginated caller);
// the HTTP handler always passes a positive default via parsePagination.
func (r *DiscoveryResultRepo) ListByJob(ctx context.Context, jobID uuid.UUID, limit, offset int) ([]domain.DiscoveryJobResult, error) {
	query := `
		SELECT id, job_id, host(ip_address), status, sys_name, sys_descr, sys_object_id,
		       COALESCE(error_message, ''), element_id, responded_at
		FROM discovery_job_results WHERE job_id = $1 ORDER BY ip_address`
	args := []any{jobID}
	if limit > 0 {
		args = append(args, limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if offset > 0 {
		args = append(args, offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list discovery results for job %s: %w", jobID, err)
	}
	defer rows.Close()

	var out []domain.DiscoveryJobResult
	for rows.Next() {
		var r domain.DiscoveryJobResult
		if err := rows.Scan(&r.ID, &r.JobID, &r.IPAddress, &r.Status, &r.SysName, &r.SysDescr,
			&r.SysObjectID, &r.ErrorMessage, &r.ElementID, &r.RespondedAt); err != nil {
			return nil, fmt.Errorf("scan discovery result row: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
