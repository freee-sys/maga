// Package ruleengine implements the pure hostname -> cluster-key
// evaluation shared by discovery, manual redistribution, and the rule
// tester endpoint. It performs no I/O: callers are responsible for
// loading rules (in priority order) and for persisting the result.
package ruleengine

import (
	"fmt"
	"regexp"

	"github.com/google/uuid"

	"netcluster/internal/domain"
)

// MatchResult is the outcome of evaluating one hostname against a set of
// rules. RuleID is nil when no enabled rule's pattern matched the
// hostname at all. A matched rule whose template rendered to an empty
// string is still recorded here (RuleID set, RenderedClusterKey empty,
// with a warning) — the caller decides how to persist that outcome.
type MatchResult struct {
	RuleID              *uuid.UUID
	MatchedPattern      string
	RawCaptures         map[string]string
	TransformedCaptures map[string]string
	RenderedClusterKey  string
	Warnings            []string
}

// Evaluate matches hostname against rules in the given order and returns
// the first successful match. Disabled rules and rules with an invalid
// regex pattern are skipped rather than treated as fatal, so one bad rule
// never blocks evaluation against the rest.
func Evaluate(hostname string, rules []domain.Rule) (*MatchResult, error) {
	for _, rule := range rules {
		if !rule.IsEnabled {
			continue
		}

		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}

		match := re.FindStringSubmatch(hostname)
		if match == nil {
			continue
		}

		return evaluateMatch(rule, re, match), nil
	}

	return &MatchResult{
		RawCaptures:         map[string]string{},
		TransformedCaptures: map[string]string{},
	}, nil
}

func evaluateMatch(rule domain.Rule, re *regexp.Regexp, match []string) *MatchResult {
	ruleID := rule.ID
	result := &MatchResult{
		RuleID:              &ruleID,
		MatchedPattern:      rule.Pattern,
		RawCaptures:         map[string]string{},
		TransformedCaptures: map[string]string{},
	}

	for i, name := range re.SubexpNames() {
		if i == 0 || name == "" {
			continue
		}
		result.RawCaptures[name] = match[i]
	}

	for name, raw := range result.RawCaptures {
		value := raw
		if pipeline, ok := rule.CaptureTransforms[name]; ok {
			transformed, err := pipeline.Apply(raw)
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("capture %q: dsl transform failed, using raw value: %v", name, err))
			} else {
				value = transformed
			}
		}
		result.TransformedCaptures[name] = value
	}

	result.RenderedClusterKey = renderTemplate(rule.ClusterKeyTemplate, result.TransformedCaptures)
	if result.RenderedClusterKey == "" {
		result.Warnings = append(result.Warnings, "cluster key template rendered to an empty string")
	}

	return result
}
