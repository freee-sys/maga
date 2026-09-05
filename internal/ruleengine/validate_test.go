package ruleengine_test

import (
	"testing"

	"netcluster/internal/dsl"
	"netcluster/internal/ruleengine"
)

func TestValidateRule_ValidRuleHasNoErrors(t *testing.T) {
	errs := ruleengine.ValidateRule(
		`^sw-(?P<site>[a-z0-9]+)-(?P<role>[a-z]+)-\d+$`,
		map[string]dsl.Pipeline{"site": mustPipeline(t, "upper()")},
		"{site}-{role}",
	)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %+v", errs)
	}
}

func TestValidateRule_InvalidRegexReportsPatternField(t *testing.T) {
	errs := ruleengine.ValidateRule(`(unterminated`, nil, "x")
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got %+v", errs)
	}
	if errs[0].Field != "pattern" {
		t.Errorf("expected field %q, got %q", "pattern", errs[0].Field)
	}
}

func TestValidateRule_TransformReferencesUndefinedGroup(t *testing.T) {
	errs := ruleengine.ValidateRule(
		`^sw-(?P<site>[a-z0-9]+)-\d+$`,
		map[string]dsl.Pipeline{"role": mustPipeline(t, "upper()")},
		"{site}",
	)
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got %+v", errs)
	}
	if errs[0].Field != "capture_transforms.role" {
		t.Errorf("expected field %q, got %q", "capture_transforms.role", errs[0].Field)
	}
}

func TestValidateRule_TemplateReferencesUndefinedGroup(t *testing.T) {
	errs := ruleengine.ValidateRule(
		`^sw-(?P<site>[a-z0-9]+)-\d+$`,
		nil,
		"{site}-{role}",
	)
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got %+v", errs)
	}
	if errs[0].Field != "cluster_key_template" {
		t.Errorf("expected field %q, got %q", "cluster_key_template", errs[0].Field)
	}
}

func TestValidateRule_ReportsAllErrorsAtOnce(t *testing.T) {
	errs := ruleengine.ValidateRule(
		`^sw-(?P<site>[a-z0-9]+)-\d+$`,
		map[string]dsl.Pipeline{"role": mustPipeline(t, "upper()")},
		"{site}-{env}",
	)
	if len(errs) != 2 {
		t.Fatalf("expected exactly two errors, got %+v", errs)
	}
}
