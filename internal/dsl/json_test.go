package dsl_test

import (
	"encoding/json"
	"testing"

	"netcluster/internal/dsl"
)

func TestPipeline_JSONRoundTrip_NoArgOp(t *testing.T) {
	p := dsl.Pipeline{{Name: "trim"}}

	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	got, err := dsl.FromJSON(raw)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}
	if len(got) != 1 || got[0].Name != "trim" {
		t.Fatalf("expected single trim op, got %+v", got)
	}
}

func TestPipeline_JSONRoundTrip_StringArgsUseNamedFields(t *testing.T) {
	p := dsl.Pipeline{{Name: "replace", Args: []dsl.Arg{strArg("-old-"), strArg("-new-")}}}

	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded []map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("failed to decode raw JSON: %v", err)
	}
	args := decoded[0]["args"].(map[string]any)
	if args["old"] != "-old-" || args["new"] != "-new-" {
		t.Fatalf("expected named args old/new, got %+v", args)
	}

	got, err := dsl.FromJSON(raw)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}
	result, err := got.Apply("sw-old-dc1")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if result != "sw-new-dc1" {
		t.Fatalf("expected sw-new-dc1, got %q", result)
	}
}

func TestPipeline_JSONRoundTrip_IntArgs(t *testing.T) {
	p := dsl.Pipeline{{Name: "deleteRange", Args: []dsl.Arg{intArg(2), intArg(5)}}}

	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got, err := dsl.FromJSON(raw)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}
	result, err := got.Apply("abcdef")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if result != "abf" {
		t.Fatalf("expected abf, got %q", result)
	}
}

func TestPipeline_JSONRoundTrip_MapArg(t *testing.T) {
	p := dsl.Pipeline{{Name: "mapValues", Args: []dsl.Arg{dsl.MapArg(map[string]string{"dc": "datacenter"}), strArg("unknown")}}}

	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got, err := dsl.FromJSON(raw)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}
	result, err := got.Apply("dc")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if result != "datacenter" {
		t.Fatalf("expected datacenter, got %q", result)
	}
}

func TestPipeline_JSONRoundTrip_MultiStepPipeline(t *testing.T) {
	p := dsl.Pipeline{
		{Name: "trim"},
		{Name: "upper"},
		{Name: "replace", Args: []dsl.Arg{strArg("-OLD-"), strArg("-NEW-")}},
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got, err := dsl.FromJSON(raw)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}
	result, err := got.Apply("  sw-old-dc1  ")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if result != "SW-NEW-DC1" {
		t.Fatalf("expected SW-NEW-DC1, got %q", result)
	}
}

func TestFromJSON_UnknownOpReturnsError(t *testing.T) {
	_, err := dsl.FromJSON([]byte(`[{"op":"doesNotExist","args":{}}]`))
	if err == nil {
		t.Fatal("expected error for unknown op, got nil")
	}
}

func TestFromJSON_MissingRequiredArgReturnsError(t *testing.T) {
	_, err := dsl.FromJSON([]byte(`[{"op":"replace","args":{"old":"a"}}]`))
	if err == nil {
		t.Fatal("expected error for missing required arg, got nil")
	}
}

func TestFromJSON_EmptyArrayIsValidEmptyPipeline(t *testing.T) {
	p, err := dsl.FromJSON([]byte(`[]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p) != 0 {
		t.Fatalf("expected empty pipeline, got %+v", p)
	}
}

func TestFromJSON_MalformedJSONReturnsError(t *testing.T) {
	_, err := dsl.FromJSON([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}
