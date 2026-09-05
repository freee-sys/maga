package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"netcluster/internal/domain"
)

type RuleRepo struct {
	db *pgxpool.Pool
}

func NewRuleRepo(db *pgxpool.Pool) *RuleRepo {
	return &RuleRepo{db: db}
}

func (r *RuleRepo) Create(ctx context.Context, rule domain.Rule) (domain.Rule, error) {
	transforms, err := json.Marshal(rule.CaptureTransforms)
	if err != nil {
		return domain.Rule{}, fmt.Errorf("marshal capture transforms: %w", err)
	}

	var id uuid.UUID
	err = r.db.QueryRow(ctx, `
		INSERT INTO rules (name, description, pattern, capture_transforms, cluster_key_template, priority, is_enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, rule.Name, rule.Description, rule.Pattern, transforms, rule.ClusterKeyTemplate, rule.Priority, rule.IsEnabled).Scan(&id)
	if err != nil {
		return domain.Rule{}, fmt.Errorf("create rule %q: %w", rule.Name, err)
	}

	rule.ID = id
	return rule, nil
}

func (r *RuleRepo) Get(ctx context.Context, id uuid.UUID) (domain.Rule, error) {
	return scanRule(r.db.QueryRow(ctx, `
		SELECT id, name, description, pattern, capture_transforms, cluster_key_template, priority, is_enabled
		FROM rules WHERE id = $1
	`, id))
}

func (r *RuleRepo) Update(ctx context.Context, rule domain.Rule) (domain.Rule, error) {
	transforms, err := json.Marshal(rule.CaptureTransforms)
	if err != nil {
		return domain.Rule{}, fmt.Errorf("marshal capture transforms: %w", err)
	}

	return scanRule(r.db.QueryRow(ctx, `
		UPDATE rules
		SET name = $2, description = $3, pattern = $4, capture_transforms = $5,
		    cluster_key_template = $6, priority = $7, is_enabled = $8, updated_at = now()
		WHERE id = $1
		RETURNING id, name, description, pattern, capture_transforms, cluster_key_template, priority, is_enabled
	`, rule.ID, rule.Name, rule.Description, rule.Pattern, transforms, rule.ClusterKeyTemplate, rule.Priority, rule.IsEnabled))
}

func (r *RuleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete rule %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ListEnabledOrdered returns enabled rules in priority-ascending order,
// ready to hand straight to ruleengine.Evaluate.
func (r *RuleRepo) ListEnabledOrdered(ctx context.Context) ([]domain.Rule, error) {
	return queryRules(ctx, r.db, `
		SELECT id, name, description, pattern, capture_transforms, cluster_key_template, priority, is_enabled
		FROM rules WHERE is_enabled = true ORDER BY priority ASC
	`)
}

// ListAllOrdered returns every rule (enabled or not) in priority-ascending
// order, for the rules management UI.
func (r *RuleRepo) ListAllOrdered(ctx context.Context) ([]domain.Rule, error) {
	return queryRules(ctx, r.db, `
		SELECT id, name, description, pattern, capture_transforms, cluster_key_template, priority, is_enabled
		FROM rules ORDER BY priority ASC
	`)
}

// Reorder rewrites priorities to match the given ID order, spaced by 10
// so a future insert-between doesn't require renumbering everything.
// priority is UNIQUE, so a direct swap (e.g. two rules exchanging
// positions) can collide mid-update; the reorder is done in two passes,
// first moving every row to a negative placeholder no existing row can
// hold, then to its final value.
func (r *RuleRepo) Reorder(ctx context.Context, orderedIDs []uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reorder transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for i, id := range orderedIDs {
		placeholder := -(i + 1)
		if _, err := tx.Exec(ctx, `UPDATE rules SET priority = $2 WHERE id = $1`, id, placeholder); err != nil {
			return fmt.Errorf("reorder rule %s (placeholder pass): %w", id, err)
		}
	}
	for i, id := range orderedIDs {
		priority := (i + 1) * 10
		if _, err := tx.Exec(ctx, `UPDATE rules SET priority = $2, updated_at = now() WHERE id = $1`, id, priority); err != nil {
			return fmt.Errorf("reorder rule %s: %w", id, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reorder transaction: %w", err)
	}
	return nil
}

func queryRules(ctx context.Context, db *pgxpool.Pool, query string) ([]domain.Rule, error) {
	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()

	var out []domain.Rule
	for rows.Next() {
		rule, err := scanRuleRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows (Query),
// so scanning logic is written once and shared by both paths.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanRule(row rowScanner) (domain.Rule, error) {
	return scanRuleRow(row)
}

func scanRuleRow(row rowScanner) (domain.Rule, error) {
	var rule domain.Rule
	var transforms []byte
	if err := row.Scan(&rule.ID, &rule.Name, &rule.Description, &rule.Pattern,
		&transforms, &rule.ClusterKeyTemplate, &rule.Priority, &rule.IsEnabled); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Rule{}, domain.ErrNotFound
		}
		return domain.Rule{}, fmt.Errorf("scan rule row: %w", err)
	}
	if len(transforms) > 0 {
		if err := json.Unmarshal(transforms, &rule.CaptureTransforms); err != nil {
			return domain.Rule{}, fmt.Errorf("unmarshal capture transforms: %w", err)
		}
	}
	return rule, nil
}
