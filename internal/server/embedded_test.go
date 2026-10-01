package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestNewSPAHandler(t *testing.T) {
	mockFS := fstest.MapFS{
		"index.html":       {Data: []byte("<html><body>Root App</body></html>")},
		"assets/style.css": {Data: []byte("body { background: black; }")},
	}

	handler := NewSPAHandler(mockFS)

	// 1. Root index.html
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /, got %d", rec.Code)
	}
	if rec.Body.String() != "<html><body>Root App</body></html>" {
		t.Errorf("unexpected body for /: %s", rec.Body.String())
	}

	// 2. Static asset exists
	req = httptest.NewRequest(http.MethodGet, "/assets/style.css", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /assets/style.css, got %d", rec.Code)
	}
	if rec.Body.String() != "body { background: black; }" {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}

	// 3. Unknown route falls back to index.html for SPA
	req = httptest.NewRequest(http.MethodGet, "/projects/123", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 fallback, got %d", rec.Code)
	}
	if rec.Body.String() != "<html><body>Root App</body></html>" {
		t.Errorf("expected fallback to index.html, got: %s", rec.Body.String())
	}
}
