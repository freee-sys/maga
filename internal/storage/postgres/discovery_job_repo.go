package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"netcluster/internal/domain"
)

type DiscoveryJobRepo struct {
	db *pgxpool.Pool
}

func NewDiscoveryJobRepo(db *pgxpool.Pool) *DiscoveryJobRepo {
	return &DiscoveryJobRepo{db: db}
}

func (r *DiscoveryJobRepo) Create(ctx context.Context, job domain.DiscoveryJob) (domain.DiscoveryJob, error) {
	jobType := job.JobType
	if jobType == "" {
		jobType = "snmp_discovery"
	}

	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO discovery_jobs (job_type, input_spec, target_ips, snmp_version, snmp_community, status, total_targets)
		VALUES ($1, $2, $3::inet[], $4, $5, $6, $7)
		RETURNING id
	`, jobType, job.InputSpec, job.TargetIPs, job.SNMPVersion, job.SNMPCommunity,
		domain.DiscoveryJobStatusPending, len(job.TargetIPs)).Scan(&id)
	if err != nil {
		return domain.DiscoveryJob{}, fmt.Errorf("create discovery job: %w", err)
	}

	job.ID = id
	job.JobType = jobType
	job.Status = domain.DiscoveryJobStatusPending
	job.TotalTargets = len(job.TargetIPs)
	return job, nil
}

// UpdateStatus transitions a job's status, stamping started_at/completed_at
// as appropriate. errorMessage is stored only for job-level (not per-host)
// failures; pass "" when there is none.
func (r *DiscoveryJobRepo) UpdateStatus(ctx context.Context, jobID uuid.UUID, status string, errorMessage string) error {
	var query string
	switch status {
	case domain.DiscoveryJobStatusRunning:
		query = `UPDATE discovery_jobs SET status = $2, started_at = now(), error_message = NULLIF($3, '') WHERE id = $1`
	case domain.DiscoveryJobStatusCompleted, domain.DiscoveryJobStatusFailed, domain.DiscoveryJobStatusCancelled:
		query = `UPDATE discovery_jobs SET status = $2, completed_at = now(), error_message = NULLIF($3, '') WHERE id = $1`
	default:
		query = `UPDATE discovery_jobs SET status = $2, error_message = NULLIF($3, '') WHERE id = $1`
	}

	tag, err := r.db.Exec(ctx, query, jobID, status, errorMessage)
	if err != nil {
		return fmt.Errorf("update discovery job %s status: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *DiscoveryJobRepo) Get(ctx context.Context, id uuid.UUID) (domain.DiscoveryJob, error) {
	job, err := scanDiscoveryJob(r.db.QueryRow(ctx, `
		SELECT id, job_type, input_spec, target_ips::text[], snmp_version, snmp_community,
		       status, total_targets, started_at, completed_at, COALESCE(error_message, ''), created_at
		FROM discovery_jobs WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DiscoveryJob{}, domain.ErrNotFound
	}
	return job, err
}

func (r *DiscoveryJobRepo) List(ctx context.Context) ([]domain.DiscoveryJob, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, job_type, input_spec, target_ips::text[], snmp_version, snmp_community,
		       status, total_targets, started_at, completed_at, COALESCE(error_message, ''), created_at
		FROM discovery_jobs ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list discovery jobs: %w", err)
	}
	defer rows.Close()

	var out []domain.DiscoveryJob
	for rows.Next() {
		job, err := scanDiscoveryJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

func scanDiscoveryJob(row rowScanner) (domain.DiscoveryJob, error) {
	var job domain.DiscoveryJob
	if err := row.Scan(&job.ID, &job.JobType, &job.InputSpec, &job.TargetIPs, &job.SNMPVersion, &job.SNMPCommunity,
		&job.Status, &job.TotalTargets, &job.StartedAt, &job.CompletedAt, &job.ErrorMessage, &job.CreatedAt); err != nil {
		return domain.DiscoveryJob{}, fmt.Errorf("scan discovery job row: %w", err)
	}
	return job, nil
}
