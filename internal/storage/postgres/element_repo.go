package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"netcluster/internal/domain"
	"netcluster/internal/snmp"
)

type ElementRepo struct {
	db *pgxpool.Pool
}

func NewElementRepo(db *pgxpool.Pool) *ElementRepo {
	return &ElementRepo{db: db}
}

// Create inserts a bare network element. Real discovery uses
// UpsertDiscovered below; this exists to seed fixtures in tests that
// predate a real SNMP response.
func (r *ElementRepo) Create(ctx context.Context, ip, sysName string) (domain.NetworkElement, error) {
	var e domain.NetworkElement
	err := r.db.QueryRow(ctx, `
		INSERT INTO network_elements (ip_address, sys_name)
		VALUES ($1, $2)
		RETURNING id, host(ip_address), sys_name, assignment_status
	`, ip, sysName).Scan(&e.ID, &e.IPAddress, &e.SysName, &e.AssignmentStatus)
	if err != nil {
		return domain.NetworkElement{}, fmt.Errorf("create element %s: %w", ip, err)
	}
	return e, nil
}

// UpsertDiscovered creates or refreshes an element from a successful SNMP
// response, keyed by ip_address (UNIQUE).
func (r *ElementRepo) UpsertDiscovered(ctx context.Context, ip string, identity snmp.Identity) (domain.NetworkElement, error) {
	var e domain.NetworkElement
	err := r.db.QueryRow(ctx, `
		INSERT INTO network_elements (ip_address, sys_name, sys_descr, sys_object_id, sys_uptime_ticks, last_seen_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, now(), now())
		ON CONFLICT (ip_address) DO UPDATE SET
			sys_name = EXCLUDED.sys_name,
			sys_descr = EXCLUDED.sys_descr,
			sys_object_id = EXCLUDED.sys_object_id,
			sys_uptime_ticks = EXCLUDED.sys_uptime_ticks,
			last_seen_at = now(),
			updated_at = now()
		RETURNING id, host(ip_address), sys_name, assignment_status, cluster_id, last_rule_id
	`, ip, identity.SysName, identity.SysDescr, identity.SysObjectID, identity.SysUpTime).
		Scan(&e.ID, &e.IPAddress, &e.SysName, &e.AssignmentStatus, &e.ClusterID, &e.LastRuleID)
	if err != nil {
		return domain.NetworkElement{}, fmt.Errorf("upsert discovered element %s: %w", ip, err)
	}
	return e, nil
}

func (r *ElementRepo) Get(ctx context.Context, id uuid.UUID) (domain.NetworkElement, error) {
	var e domain.NetworkElement
	err := r.db.QueryRow(ctx, `
		SELECT id, host(ip_address), sys_name, assignment_status, cluster_id, last_rule_id,
		       first_seen_at, last_seen_at
		FROM network_elements WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&e.ID, &e.IPAddress, &e.SysName, &e.AssignmentStatus, &e.ClusterID, &e.LastRuleID,
		&e.FirstSeenAt, &e.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NetworkElement{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.NetworkElement{}, fmt.Errorf("get element %s: %w", id, err)
	}
	return e, nil
}

// List returns non-deleted elements matching filter, newest first.
func (r *ElementRepo) List(ctx context.Context, filter domain.ElementFilter) ([]domain.NetworkElement, error) {
	query := `
		SELECT id, host(ip_address), sys_name, assignment_status, cluster_id, last_rule_id,
		       first_seen_at, last_seen_at
		FROM network_elements WHERE deleted_at IS NULL`
	var args []any
	if filter.ClusterID != nil {
		args = append(args, *filter.ClusterID)
		query += fmt.Sprintf(" AND cluster_id = $%d", len(args))
	}
	if filter.AssignmentStatus != "" {
		args = append(args, filter.AssignmentStatus)
		query += fmt.Sprintf(" AND assignment_status = $%d", len(args))
	}
	if filter.Query != "" {
		args = append(args, "%"+strings.ToLower(filter.Query)+"%")
		query += fmt.Sprintf(" AND (LOWER(sys_name) LIKE $%d OR host(ip_address) LIKE $%d)", len(args), len(args))
	}
	// id as a tiebreaker keeps pagination stable when timestamps collide
	// (e.g. several elements inserted within the same clock tick).
	query += " ORDER BY last_seen_at DESC, id"

	// Limit == 0 means "no limit" (e.g. bulk redistribute needs every
	// matching element); the HTTP handler always sets a positive default
	// via parsePagination for actual listing requests.
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list elements: %w", err)
	}
	defer rows.Close()

	var out []domain.NetworkElement
	for rows.Next() {
		var e domain.NetworkElement
		if err := rows.Scan(&e.ID, &e.IPAddress, &e.SysName, &e.AssignmentStatus, &e.ClusterID, &e.LastRuleID,
			&e.FirstSeenAt, &e.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan element row: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *ElementRepo) UpdateAssignment(ctx context.Context, elementID uuid.UUID, clusterID *uuid.UUID, ruleID *uuid.UUID, status string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE network_elements
		SET cluster_id = $2, last_rule_id = $3, assignment_status = $4, updated_at = now()
		WHERE id = $1
	`, elementID, clusterID, ruleID, status)
	if err != nil {
		return fmt.Errorf("update assignment for element %s: %w", elementID, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SoftDelete marks an element deleted without removing its row, so
// assignment_history foreign keys stay valid.
func (r *ElementRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE network_elements SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return fmt.Errorf("soft-delete element %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
