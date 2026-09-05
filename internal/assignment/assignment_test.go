package assignment_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"netcluster/internal/assignment"
	"netcluster/internal/domain"
)

type fakeClusterRepo struct {
	byKey map[string]domain.Cluster
	calls int
}

func newFakeClusterRepo() *fakeClusterRepo {
	return &fakeClusterRepo{byKey: map[string]domain.Cluster{}}
}

func (f *fakeClusterRepo) GetOrCreateByLookupKey(ctx context.Context, lookupKey string) (domain.Cluster, error) {
	f.calls++
	if c, ok := f.byKey[lookupKey]; ok {
		return c, nil
	}
	c := domain.Cluster{ID: uuid.New(), LookupKey: lookupKey, DisplayName: lookupKey}
	f.byKey[lookupKey] = c
	return c, nil
}

type assignmentCall struct {
	elementID uuid.UUID
	clusterID *uuid.UUID
	ruleID    *uuid.UUID
	status    string
}

type fakeElementRepo struct {
	calls []assignmentCall
}

func (f *fakeElementRepo) UpdateAssignment(ctx context.Context, elementID uuid.UUID, clusterID *uuid.UUID, ruleID *uuid.UUID, status string) error {
	f.calls = append(f.calls, assignmentCall{elementID, clusterID, ruleID, status})
	return nil
}

type fakeHistoryRepo struct {
	inserted []domain.AssignmentHistory
}

func (f *fakeHistoryRepo) Insert(ctx context.Context, h domain.AssignmentHistory) error {
	f.inserted = append(f.inserted, h)
	return nil
}

func matchingRule() domain.Rule {
	return domain.Rule{
		ID: uuid.New(), IsEnabled: true,
		Pattern:            `^sw-(?P<site>[a-z0-9]+)-\d+$`,
		ClusterKeyTemplate: "{site}",
	}
}

func TestService_Assign_MatchedRuleAssignsClusterAndWritesHistory(t *testing.T) {
	clusterRepo := newFakeClusterRepo()
	elementRepo := &fakeElementRepo{}
	historyRepo := &fakeHistoryRepo{}
	svc := assignment.NewService(elementRepo, clusterRepo, historyRepo)

	rule := matchingRule()
	elementID := uuid.New()

	result, err := svc.Assign(context.Background(), elementID, "sw-dc1-01", []domain.Rule{rule}, domain.TriggerSourceDiscovery)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RenderedClusterKey != "dc1" {
		t.Fatalf("expected cluster key dc1, got %q", result.RenderedClusterKey)
	}

	if len(elementRepo.calls) != 1 {
		t.Fatalf("expected exactly one UpdateAssignment call, got %d", len(elementRepo.calls))
	}
	call := elementRepo.calls[0]
	if call.elementID != elementID {
		t.Errorf("expected elementID %v, got %v", elementID, call.elementID)
	}
	if call.clusterID == nil || call.ruleID == nil || *call.ruleID != rule.ID {
		t.Errorf("expected assigned cluster/rule, got cluster=%v rule=%v", call.clusterID, call.ruleID)
	}
	if call.status != domain.AssignmentStatusAssigned {
		t.Errorf("expected status assigned, got %q", call.status)
	}

	if len(historyRepo.inserted) != 1 {
		t.Fatalf("expected exactly one history row, got %d", len(historyRepo.inserted))
	}
	h := historyRepo.inserted[0]
	if h.ElementID != elementID || h.RuleID == nil || *h.RuleID != rule.ID {
		t.Errorf("unexpected history row: %+v", h)
	}
	if h.TriggerSource != domain.TriggerSourceDiscovery {
		t.Errorf("expected trigger source discovery, got %q", h.TriggerSource)
	}
	if h.MatchedPattern != rule.Pattern {
		t.Errorf("expected matched pattern snapshot %q, got %q", rule.Pattern, h.MatchedPattern)
	}
}

func TestService_Assign_NoMatchLeavesElementUnassignedButWritesHistory(t *testing.T) {
	clusterRepo := newFakeClusterRepo()
	elementRepo := &fakeElementRepo{}
	historyRepo := &fakeHistoryRepo{}
	svc := assignment.NewService(elementRepo, clusterRepo, historyRepo)

	elementID := uuid.New()
	result, err := svc.Assign(context.Background(), elementID, "router-9000", []domain.Rule{matchingRule()}, domain.TriggerSourceDiscovery)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RuleID != nil {
		t.Fatalf("expected no matched rule, got %v", result.RuleID)
	}

	if clusterRepo.calls != 0 {
		t.Errorf("expected cluster repo not to be called on no-match, got %d calls", clusterRepo.calls)
	}

	call := elementRepo.calls[0]
	if call.clusterID != nil || call.ruleID != nil {
		t.Errorf("expected nil cluster/rule on no-match, got cluster=%v rule=%v", call.clusterID, call.ruleID)
	}
	if call.status != domain.AssignmentStatusUnassigned {
		t.Errorf("expected status unassigned, got %q", call.status)
	}

	if len(historyRepo.inserted) != 1 {
		t.Fatalf("expected a history row even on no-match, got %d", len(historyRepo.inserted))
	}
	if historyRepo.inserted[0].ClusterID != nil || historyRepo.inserted[0].RuleID != nil {
		t.Errorf("expected nil cluster/rule in no-match history row, got %+v", historyRepo.inserted[0])
	}
}

func TestService_Assign_EmptyRenderedKeyDoesNotCreateCluster(t *testing.T) {
	clusterRepo := newFakeClusterRepo()
	elementRepo := &fakeElementRepo{}
	historyRepo := &fakeHistoryRepo{}
	svc := assignment.NewService(elementRepo, clusterRepo, historyRepo)

	rule := domain.Rule{
		ID: uuid.New(), IsEnabled: true,
		Pattern:            `^sw-(?P<site>[a-z0-9]*)-\d+$`,
		ClusterKeyTemplate: "{site}",
	}
	elementID := uuid.New()

	result, err := svc.Assign(context.Background(), elementID, "sw--01", []domain.Rule{rule}, domain.TriggerSourceManualRedistributeSingle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RenderedClusterKey != "" {
		t.Fatalf("expected empty cluster key, got %q", result.RenderedClusterKey)
	}

	if clusterRepo.calls != 0 {
		t.Errorf("expected cluster repo not to be called for empty rendered key, got %d calls", clusterRepo.calls)
	}
	call := elementRepo.calls[0]
	if call.clusterID != nil {
		t.Errorf("expected nil cluster on empty key, got %v", call.clusterID)
	}
	if call.ruleID == nil || *call.ruleID != rule.ID {
		t.Errorf("expected matched rule still recorded, got %v", call.ruleID)
	}
	if call.status != domain.AssignmentStatusUnassigned {
		t.Errorf("expected status unassigned for empty key, got %q", call.status)
	}
}
