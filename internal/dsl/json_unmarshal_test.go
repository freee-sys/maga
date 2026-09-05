package dsl_test

import (
	"encoding/json"
	"testing"

	"netcluster/internal/dsl"
)

func TestPipeline_UnmarshalJSON_WorksInsideAMap(t *testing.T) {
	raw := []byte(`{"site": [{"op":"upper","args":{}}], "role": [{"op":"trim","args":{}}]}`)

	var m map[string]dsl.Pipeline
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := m["site"].Apply("dc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "DC1" {
		t.Fatalf("expected DC1, got %q", got)
	}
}

func TestPipeline_UnmarshalJSON_InvalidOpReturnsError(t *testing.T) {
	var p dsl.Pipeline
	err := json.Unmarshal([]byte(`[{"op":"doesNotExist","args":{}}]`), &p)
	if err == nil {
		t.Fatal("expected error for unknown op, got nil")
	}
}
