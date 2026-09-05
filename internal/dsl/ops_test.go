package dsl_test

import (
	"testing"

	"netcluster/internal/dsl"
)

func strArg(s string) dsl.Arg { return dsl.Arg{S: &s} }
func intArg(i int) dsl.Arg    { return dsl.Arg{I: &i} }

func TestPipeline_Apply_SingleOps(t *testing.T) {
	tests := []struct {
		name  string
		op    dsl.Op
		input string
		want  string
	}{
		{"trimLeft", dsl.Op{Name: "trimLeft", Args: []dsl.Arg{strArg("-")}}, "--x", "x"},
		{"trimRight", dsl.Op{Name: "trimRight", Args: []dsl.Arg{strArg("-")}}, "x--", "x"},
		{"upper", dsl.Op{Name: "upper"}, "abc", "ABC"},
		{"lower", dsl.Op{Name: "lower"}, "ABC", "abc"},
		{"replace", dsl.Op{Name: "replace", Args: []dsl.Arg{strArg("-old-"), strArg("-new-")}}, "sw-old-dc1", "sw-new-dc1"},
		{"replaceN_limits_count", dsl.Op{Name: "replaceN", Args: []dsl.Arg{strArg("a"), strArg("b"), intArg(1)}}, "aaa", "baa"},
		{"replaceRegex", dsl.Op{Name: "replaceRegex", Args: []dsl.Arg{strArg(`\d+`), strArg("N")}}, "sw01dc02", "swNdcN"},
		{"deleteSubstring", dsl.Op{Name: "deleteSubstring", Args: []dsl.Arg{strArg("-core")}}, "sw-core-01", "sw-01"},
		{"deleteRange", dsl.Op{Name: "deleteRange", Args: []dsl.Arg{intArg(2), intArg(5)}}, "abcdef", "abf"},
		{"insertAt", dsl.Op{Name: "insertAt", Args: []dsl.Arg{intArg(2), strArg("-X-")}}, "abcd", "ab-X-cd"},
		{"slice", dsl.Op{Name: "slice", Args: []dsl.Arg{intArg(1), intArg(4)}}, "abcdef", "bcd"},
		{"prefix", dsl.Op{Name: "prefix", Args: []dsl.Arg{strArg("pre-")}}, "abc", "pre-abc"},
		{"suffix", dsl.Op{Name: "suffix", Args: []dsl.Arg{strArg("-post")}}, "abc", "abc-post"},
		{"padLeft", dsl.Op{Name: "padLeft", Args: []dsl.Arg{intArg(5), strArg("0")}}, "7", "00007"},
		{"padRight", dsl.Op{Name: "padRight", Args: []dsl.Arg{intArg(5), strArg("0")}}, "7", "70000"},
		{"padLeft_noop_when_already_long_enough", dsl.Op{Name: "padLeft", Args: []dsl.Arg{intArg(2), strArg("0")}}, "777", "777"},
		{"default_used_on_empty", dsl.Op{Name: "default", Args: []dsl.Arg{strArg("fallback")}}, "", "fallback"},
		{"default_ignored_when_present", dsl.Op{Name: "default", Args: []dsl.Arg{strArg("fallback")}}, "value", "value"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := dsl.Pipeline{tc.op}
			got, err := p.Apply(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("op %s: input %q: expected %q, got %q", tc.op.Name, tc.input, tc.want, got)
			}
		})
	}
}

func TestPipeline_Apply_MapValues(t *testing.T) {
	table := map[string]string{"dc": "datacenter", "br": "branch"}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"known key maps to value", "dc", "datacenter"},
		{"unknown key falls back", "zz", "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op := dsl.Op{Name: "mapValues", Args: []dsl.Arg{dsl.MapArg(table), strArg("unknown")}}
			got, err := (dsl.Pipeline{op}).Apply(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestPipeline_Apply_Chaining(t *testing.T) {
	p := dsl.Pipeline{
		{Name: "trim"},
		{Name: "upper"},
		{Name: "replace", Args: []dsl.Arg{strArg("-OLD-"), strArg("-NEW-")}},
	}
	got, err := p.Apply("  sw-old-dc1  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "SW-NEW-DC1" {
		t.Errorf("expected %q, got %q", "SW-NEW-DC1", got)
	}
}

func TestPipeline_Apply_WrongArgCountReturnsError(t *testing.T) {
	p := dsl.Pipeline{{Name: "replace", Args: []dsl.Arg{strArg("only-one")}}}
	_, err := p.Apply("hello")
	if err == nil {
		t.Fatal("expected error for wrong argument count, got nil")
	}
}

func TestPipeline_Apply_DeleteRangeOutOfBoundsReturnsError(t *testing.T) {
	p := dsl.Pipeline{{Name: "deleteRange", Args: []dsl.Arg{intArg(0), intArg(99)}}}
	_, err := p.Apply("short")
	if err == nil {
		t.Fatal("expected error for out-of-range deleteRange, got nil")
	}
}
