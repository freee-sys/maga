package dsl_test

import (
	"testing"

	"netcluster/internal/dsl"
)

func FuzzParse(f *testing.F) {
	seeds := []string{
		"",
		"trim()",
		`replace("-old-", "-new-")`,
		`trim() | upper() | replace("a", "b")`,
		`insertAt(-1, "x")`,
		`prefix('a\'b')`,
		"trim(",
		"| trim()",
		`replace("unterminated)`,
		"123invalid()",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		// Parse must never panic on arbitrary input; a returned error is a
		// perfectly valid outcome for malformed text.
		_, _ = dsl.Parse(src)
	})
}
