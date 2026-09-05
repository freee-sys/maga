package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"netcluster/internal/domain"
)

type HistoryRepo struct {
	db *pgxpool.Pool
}

func NewHistoryRepo(db *pgxpool.Pool) *HistoryRepo {
	return &HistoryRepo{db: db}
}

func (r *HistoryRepo) Insert(ctx context.Context, h domain.AssignmentHistory) error {
	rawCaptures, err := json.Marshal(h.RawCaptures)
	if err != nil {
		return fmt.Errorf("marshal raw captures: %w", err)
	}
	transformedCaptures, err := json.Marshal(h.TransformedCaptures)
	if err != nil {
		return fmt.Errorf("marshal transformed captures: %w", err)
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO assignment_history
			(id, element_id, cluster_id, rule_id, matched_pattern,
			 raw_captures, transformed_captures, rendered_cluster_key,
			 trigger_source, warnings)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, h.ID, h.ElementID, h.ClusterID, h.RuleID, h.MatchedPattern,
		rawCaptures, transformedCaptures, h.RenderedClusterKey,
		h.TriggerSource, h.Warnings)
	if err != nil {
		return fmt.Errorf("insert assignment history for element %s: %w", h.ElementID, err)
	}
	return nil
}

func (r *HistoryRepo) ListByElement(ctx context.Context, elementID uuid.UUID) ([]domain.AssignmentHistory, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, element_id, cluster_id, rule_id, matched_pattern,
		       raw_captures, transformed_captures, rendered_cluster_key,
		       trigger_source, warnings, created_at
		FROM assignment_history
		WHERE element_id = $1
		ORDER BY created_at DESC
	`, elementID)
	if err != nil {
		return nil, fmt.Errorf("list history for element %s: %w", elementID, err)
	}
	defer rows.Close()

	var out []domain.AssignmentHistory
	for rows.Next() {
		var h domain.AssignmentHistory
		var rawCaptures, transformedCaptures []byte
		if err := rows.Scan(&h.ID, &h.ElementID, &h.ClusterID, &h.RuleID, &h.MatchedPattern,
			&rawCaptures, &transformedCaptures, &h.RenderedClusterKey,
			&h.TriggerSource, &h.Warnings, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan history row: %w", err)
		}
		if err := json.Unmarshal(rawCaptures, &h.RawCaptures); err != nil {
			return nil, fmt.Errorf("unmarshal raw captures: %w", err)
		}
		if err := json.Unmarshal(transformedCaptures, &h.TransformedCaptures); err != nil {
			return nil, fmt.Errorf("unmarshal transformed captures: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
