package ruleengine_test

import (
	"testing"

	"github.com/google/uuid"

	"netcluster/internal/dsl"
	"netcluster/internal/domain"
	"netcluster/internal/ruleengine"
)

func mustPipeline(t *testing.T, src string) dsl.Pipeline {
	t.Helper()
	p, err := dsl.Parse(src)
	if err != nil {
		t.Fatalf("failed to parse pipeline %q: %v", src, err)
	}
	return p
}

func TestEvaluate_NoRulesMatch_ReturnsUnmatchedResult(t *testing.T) {
	rules := []domain.Rule{
		{ID: uuid.New(), Pattern: `^sw-(?P<site>[a-z]+)-\d+$`, ClusterKeyTemplate: "{site}", IsEnabled: true},
	}

	result, err := ruleengine.Evaluate("router-9000", rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RuleID != nil {
		t.Errorf("expected no matched rule, got %v", result.RuleID)
	}
	if result.RenderedClusterKey != "" {
		t.Errorf("expected empty cluster key, got %q", result.RenderedClusterKey)
	}
	if len(result.RawCaptures) != 0 {
		t.Errorf("expected no captures, got %v", result.RawCaptures)
	}
}

func TestEvaluate_FirstMatchingRuleByOrderWins(t *testing.T) {
	specific := domain.Rule{
		ID: uuid.New(), Priority: 10, IsEnabled: true,
		Pattern:            `^sw-(?P<site>dc1)-core-(?P<idx>\d+)$`,
		ClusterKeyTemplate: "{site}-core",
	}
	generic := domain.Rule{
		ID: uuid.New(), Priority: 20, IsEnabled: true,
		Pattern:            `^sw-(?P<site>[a-z0-9]+)-.*$`,
		ClusterKeyTemplate: "{site}-generic",
	}

	// Caller is responsible for priority ordering; Evaluate takes the slice as given.
	result, err := ruleengine.Evaluate("sw-dc1-core-01", []domain.Rule{specific, generic})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RuleID == nil || *result.RuleID != specific.ID {
		t.Fatalf("expected specific rule %v to win, got %v", specific.ID, result.RuleID)
	}
	if result.RenderedClusterKey != "dc1-core" {
		t.Errorf("expected cluster key %q, got %q", "dc1-core", result.RenderedClusterKey)
	}
}

func TestEvaluate_ExtractsNamedCaptureGroups(t *testing.T) {
	rules := []domain.Rule{
		{
			ID: uuid.New(), IsEnabled: true,
			Pattern:            `^sw-(?P<site>[a-z0-9]+)-(?P<role>[a-z]+)-(?P<idx>\d+)$`,
			ClusterKeyTemplate: "{site}-{role}",
		},
	}

	result, err := ruleengine.Evaluate("sw-dc1-core-01", rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"site": "dc1", "role": "core", "idx": "01"}
	for k, v := range want {
		if result.RawCaptures[k] != v {
			t.Errorf("capture %q: expected %q, got %q", k, v, result.RawCaptures[k])
		}
	}
}

func TestEvaluate_AppliesDSLTransformToCapturedGroup(t *testing.T) {
	rules := []domain.Rule{
		{
			ID: uuid.New(), IsEnabled: true,
			Pattern: `^sw-(?P<site>[a-z0-9]+)-core-\d+$`,
			CaptureTransforms: map[string]dsl.Pipeline{
				"site": mustPipeline(t, `upper()`),
			},
			ClusterKeyTemplate: "{site}-CORE",
		},
	}

	result, err := ruleengine.Evaluate("sw-dc1-core-01", rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TransformedCaptures["site"] != "DC1" {
		t.Errorf("expected transformed site DC1, got %q", result.TransformedCaptures["site"])
	}
	if result.RenderedClusterKey != "DC1-CORE" {
		t.Errorf("expected cluster key DC1-CORE, got %q", result.RenderedClusterKey)
	}
}

func TestEvaluate_GroupsWithoutTransformPassThroughUnchanged(t *testing.T) {
	rules := []domain.Rule{
		{
			ID: uuid.New(), IsEnabled: true,
			Pattern:            `^sw-(?P<site>[a-z0-9]+)-\d+$`,
			ClusterKeyTemplate: "{site}",
		},
	}

	result, err := ruleengine.Evaluate("sw-dc1-01", rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TransformedCaptures["site"] != "dc1" {
		t.Errorf("expected passthrough dc1, got %q", result.TransformedCaptures["site"])
	}
}

func TestEvaluate_DSLStepErrorFallsBackToRawValueWithWarning(t *testing.T) {
	rules := []domain.Rule{
		{
			ID: uuid.New(), IsEnabled: true,
			Pattern: `^sw-(?P<site>[a-z0-9]+)-\d+$`,
			CaptureTransforms: map[string]dsl.Pipeline{
				// deleteRange out of bounds for a short value: must not abort assignment.
				"site": mustPipeline(t, `deleteRange(0, 99)`),
			},
			ClusterKeyTemplate: "{site}",
		},
	}

	result, err := ruleengine.Evaluate("sw-dc1-01", rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TransformedCaptures["site"] != "dc1" {
		t.Errorf("expected raw fallback dc1, got %q", result.TransformedCaptures["site"])
	}
	if len(result.Warnings) == 0 {
		t.Error("expected a warning to be recorded for the failed DSL step")
	}
}

func TestEvaluate_EmptyRenderedKeyIsWarnedButRuleStillRecorded(t *testing.T) {
	ruleID := uuid.New()
	rules := []domain.Rule{
		{
			ID: ruleID, IsEnabled: true,
			Pattern:            `^sw-(?P<site>[a-z0-9]*)-\d+$`,
			ClusterKeyTemplate: "{site}",
		},
	}

	result, err := ruleengine.Evaluate("sw--01", rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RuleID == nil || *result.RuleID != ruleID {
		t.Fatalf("expected matched rule to still be recorded, got %v", result.RuleID)
	}
	if result.RenderedClusterKey != "" {
		t.Errorf("expected empty cluster key, got %q", result.RenderedClusterKey)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected a warning for empty rendered cluster key")
	}
}

func TestEvaluate_DisabledRulesAreSkipped(t *testing.T) {
	disabled := domain.Rule{
		ID: uuid.New(), IsEnabled: false,
		Pattern: `^sw-(?P<site>[a-z0-9]+)-\d+$`, ClusterKeyTemplate: "{site}",
	}
	enabled := domain.Rule{
		ID: uuid.New(), IsEnabled: true,
		Pattern: `^sw-(?P<site>[a-z0-9]+)-\d+$`, ClusterKeyTemplate: "{site}-fallback",
	}

	result, err := ruleengine.Evaluate("sw-dc1-01", []domain.Rule{disabled, enabled})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RuleID == nil || *result.RuleID != enabled.ID {
		t.Fatalf("expected disabled rule to be skipped, got %v", result.RuleID)
	}
}

func TestEvaluate_InvalidRegexIsSkippedNotFatal(t *testing.T) {
	broken := domain.Rule{
		ID: uuid.New(), IsEnabled: true,
		Pattern: `(unterminated`, ClusterKeyTemplate: "x",
	}
	working := domain.Rule{
		ID: uuid.New(), IsEnabled: true,
		Pattern: `^sw-(?P<site>[a-z0-9]+)-\d+$`, ClusterKeyTemplate: "{site}",
	}

	result, err := ruleengine.Evaluate("sw-dc1-01", []domain.Rule{broken, working})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RuleID == nil || *result.RuleID != working.ID {
		t.Fatalf("expected broken rule to be skipped in favor of working rule, got %v", result.RuleID)
	}
}
