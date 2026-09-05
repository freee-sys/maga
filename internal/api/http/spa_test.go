package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func testSPAFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":        {Data: []byte("<html>spa shell</html>")},
		"assets/app.js":     {Data: []byte("console.log('app')")},
	}
}

func TestSPAHandler_ServesStaticFileAsIs(t *testing.T) {
	h := NewSPAHandler(testSPAFS())
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "console.log('app')" {
		t.Errorf("expected static file content, got %q", rec.Body.String())
	}
}

func TestSPAHandler_FallsBackToIndexForUnknownPath(t *testing.T) {
	h := NewSPAHandler(testSPAFS())
	req := httptest.NewRequest(http.MethodGet, "/rules/some-uuid", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "<html>spa shell</html>" {
		t.Errorf("expected index.html fallback, got %q", rec.Body.String())
	}
}

func TestSPAHandler_ServesIndexAtRoot(t *testing.T) {
	h := NewSPAHandler(testSPAFS())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "<html>spa shell</html>" {
		t.Fatalf("expected index.html at root, got %d %q", rec.Code, rec.Body.String())
	}
}
