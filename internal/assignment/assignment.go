// Package assignment orchestrates the rule engine plus persistence: given
// an element and a hostname, it evaluates the rules, looks up-or-creates
// the target cluster, updates the element, and writes an audit history
// row — always, even when nothing matched. It is used identically by
// discovery, manual redistribute, and nothing else (the rule-tester
// endpoint calls ruleengine.Evaluate directly and skips this package
// entirely, since it must never persist anything).
package assignment

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"netcluster/internal/domain"
	"netcluster/internal/ruleengine"
)

// ElementRepo is the persistence interface for updating a device's
// assignment outcome.
type ElementRepo interface {
	UpdateAssignment(ctx context.Context, elementID uuid.UUID, clusterID *uuid.UUID, ruleID *uuid.UUID, status string) error
}

// ClusterRepo is the subset of clusters.Repo this package needs.
type ClusterRepo interface {
	GetOrCreateByLookupKey(ctx context.Context, lookupKey string) (domain.Cluster, error)
}

// HistoryRepo is the persistence interface for the assignment audit trail.
type HistoryRepo interface {
	Insert(ctx context.Context, h domain.AssignmentHistory) error
}

type Service struct {
	elements ElementRepo
	clusters ClusterRepo
	history  HistoryRepo
}

func NewService(elements ElementRepo, clusters ClusterRepo, history HistoryRepo) *Service {
	return &Service{elements: elements, clusters: clusters, history: history}
}

// Assign evaluates hostname against rules and persists the outcome for
// elementID. A rule matching but rendering an empty cluster key is a
// failed assignment, not an empty-key cluster: no cluster is created, the
// element is left/returned to unassigned, and the matched rule is still
// recorded in history.
func (s *Service) Assign(ctx context.Context, elementID uuid.UUID, hostname string, rules []domain.Rule, trigger string) (*ruleengine.MatchResult, error) {
	result, err := ruleengine.Evaluate(hostname, rules)
	if err != nil {
		return nil, fmt.Errorf("evaluate rules: %w", err)
	}

	var clusterID *uuid.UUID
	status := domain.AssignmentStatusUnassigned
	if result.RenderedClusterKey != "" {
		cluster, err := s.clusters.GetOrCreateByLookupKey(ctx, result.RenderedClusterKey)
		if err != nil {
			return nil, fmt.Errorf("get or create cluster: %w", err)
		}
		clusterID = &cluster.ID
		status = domain.AssignmentStatusAssigned
	}

	if err := s.elements.UpdateAssignment(ctx, elementID, clusterID, result.RuleID, status); err != nil {
		return nil, fmt.Errorf("update element assignment: %w", err)
	}

	history := domain.AssignmentHistory{
		ID:                  uuid.New(),
		ElementID:           elementID,
		ClusterID:           clusterID,
		RuleID:              result.RuleID,
		MatchedPattern:      result.MatchedPattern,
		RawCaptures:         result.RawCaptures,
		TransformedCaptures: result.TransformedCaptures,
		RenderedClusterKey:  result.RenderedClusterKey,
		TriggerSource:       trigger,
		Warnings:            strings.Join(result.Warnings, "; "),
	}
	if err := s.history.Insert(ctx, history); err != nil {
		return nil, fmt.Errorf("insert assignment history: %w", err)
	}

	return result, nil
}
