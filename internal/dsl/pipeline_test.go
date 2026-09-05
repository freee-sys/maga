package dsl_test

import (
	"testing"

	"netcluster/internal/dsl"
)

func TestPipeline_Apply_TrimStripsWhitespace(t *testing.T) {
	p := dsl.Pipeline{{Name: "trim"}}

	got, err := p.Apply("  hello  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestPipeline_Apply_UnknownOpReturnsError(t *testing.T) {
	p := dsl.Pipeline{{Name: "doesNotExist"}}

	_, err := p.Apply("hello")
	if err == nil {
		t.Fatal("expected error for unknown op, got nil")
	}
}
