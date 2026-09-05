package ruleengine

import (
	"fmt"
	"regexp"

	"netcluster/internal/dsl"
)

// FieldError names the specific request field a validation problem came
// from, so API handlers can surface it as {"field": ..., "message": ...}.
type FieldError struct {
	Field   string
	Message string
}

func (e FieldError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateRule checks a candidate rule definition before it is persisted:
// the pattern must compile, and every capture-transform key and every
// {token} in the cluster-key template must refer to a named group the
// pattern actually defines. All problems are reported together rather
// than stopping at the first one, so a UI can show every fix needed at
// once instead of a slow one-at-a-time round trip.
func ValidateRule(pattern string, captureTransforms map[string]dsl.Pipeline, clusterKeyTemplate string) []FieldError {
	var errs []FieldError

	re, err := regexp.Compile(pattern)
	if err != nil {
		return []FieldError{{Field: "pattern", Message: err.Error()}}
	}

	groups := map[string]bool{}
	for _, name := range re.SubexpNames() {
		if name != "" {
			groups[name] = true
		}
	}

	for name := range captureTransforms {
		if !groups[name] {
			errs = append(errs, FieldError{
				Field:   "capture_transforms." + name,
				Message: fmt.Sprintf("references undefined capture group %q", name),
			})
		}
	}

	for _, token := range extractTemplateTokens(clusterKeyTemplate) {
		if !groups[token] {
			errs = append(errs, FieldError{
				Field:   "cluster_key_template",
				Message: fmt.Sprintf("references undefined capture group %q", token),
			})
		}
	}

	return errs
}
