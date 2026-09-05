package dsl_test

import (
	"testing"

	"netcluster/internal/dsl"
)

func TestParse_NoArgOp(t *testing.T) {
	p, err := dsl.Parse(`trim()`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p) != 1 || p[0].Name != "trim" {
		t.Fatalf("expected single trim op, got %+v", p)
	}
}

func TestParse_StringArgs(t *testing.T) {
	p, err := dsl.Parse(`replace("-old-", "-new-")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p) != 1 || p[0].Name != "replace" {
		t.Fatalf("expected single replace op, got %+v", p)
	}
	if len(p[0].Args) != 2 || *p[0].Args[0].S != "-old-" || *p[0].Args[1].S != "-new-" {
		t.Fatalf("unexpected args: %+v", p[0].Args)
	}
}

func TestParse_IntArgs(t *testing.T) {
	p, err := dsl.Parse(`deleteRange(2, 5)`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p[0].Args) != 2 || *p[0].Args[0].I != 2 || *p[0].Args[1].I != 5 {
		t.Fatalf("unexpected args: %+v", p[0].Args)
	}
}

func TestParse_NegativeIntArg(t *testing.T) {
	p, err := dsl.Parse(`insertAt(-1, "x")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *p[0].Args[0].I != -1 {
		t.Fatalf("expected -1, got %d", *p[0].Args[0].I)
	}
}

func TestParse_ChainedPipeline(t *testing.T) {
	p, err := dsl.Parse(`trim() | replace("-old-", "-new-") | upper()`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p) != 3 {
		t.Fatalf("expected 3 ops, got %d: %+v", len(p), p)
	}
	if p[0].Name != "trim" || p[1].Name != "replace" || p[2].Name != "upper" {
		t.Fatalf("unexpected op order: %+v", p)
	}
}

func TestParse_SingleQuotedString(t *testing.T) {
	p, err := dsl.Parse(`prefix('pre-')`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *p[0].Args[0].S != "pre-" {
		t.Fatalf("expected \"pre-\", got %q", *p[0].Args[0].S)
	}
}

func TestParse_EscapedQuoteInString(t *testing.T) {
	p, err := dsl.Parse(`prefix("a\"b")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *p[0].Args[0].S != `a"b` {
		t.Fatalf("expected %q, got %q", `a"b`, *p[0].Args[0].S)
	}
}

func TestParse_EmptyPipelineIsValid(t *testing.T) {
	p, err := dsl.Parse(``)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p) != 0 {
		t.Fatalf("expected empty pipeline, got %+v", p)
	}
}

func TestParse_SyntaxErrors(t *testing.T) {
	tests := []string{
		`trim(`,
		`trim)`,
		`trim() |`,
		`| trim()`,
		`replace("unterminated)`,
		`123invalid()`,
		`trim(,)`,
	}
	for _, src := range tests {
		t.Run(src, func(t *testing.T) {
			_, err := dsl.Parse(src)
			if err == nil {
				t.Fatalf("expected parse error for %q, got nil", src)
			}
		})
	}
}
