package http

import (
	"net/http/httptest"
	"testing"
)

func TestParsePagination_DefaultsWhenUnset(t *testing.T) {
	req := httptest.NewRequest("GET", "/x", nil)
	limit, offset, err := parsePagination(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != defaultPageLimit || offset != 0 {
		t.Errorf("expected defaults (%d, 0), got (%d, %d)", defaultPageLimit, limit, offset)
	}
}

func TestParsePagination_ReadsExplicitValues(t *testing.T) {
	req := httptest.NewRequest("GET", "/x?limit=25&offset=50", nil)
	limit, offset, err := parsePagination(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 25 || offset != 50 {
		t.Errorf("expected (25, 50), got (%d, %d)", limit, offset)
	}
}

func TestParsePagination_ClampsLimitToMax(t *testing.T) {
	req := httptest.NewRequest("GET", "/x?limit=999999", nil)
	limit, _, err := parsePagination(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != maxPageLimit {
		t.Errorf("expected limit clamped to %d, got %d", maxPageLimit, limit)
	}
}

func TestParsePagination_RejectsNegativeValues(t *testing.T) {
	for _, q := range []string{"?limit=-1", "?offset=-1"} {
		req := httptest.NewRequest("GET", "/x"+q, nil)
		if _, _, err := parsePagination(req); err == nil {
			t.Errorf("expected error for query %q, got nil", q)
		}
	}
}

func TestParsePagination_RejectsNonNumeric(t *testing.T) {
	req := httptest.NewRequest("GET", "/x?limit=abc", nil)
	if _, _, err := parsePagination(req); err == nil {
		t.Error("expected error for non-numeric limit, got nil")
	}
}
