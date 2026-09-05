package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter_HealthzRouteIsWired(t *testing.T) {
	router := NewRouter(
		NewRulesHandler(nil, nil),
		NewClustersHandler(nil),
		NewDiscoveryHandler(nil, nil, nil),
		NewElementsHandler(nil, nil, nil, nil),
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
