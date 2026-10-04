package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"latex-editor/internal/compiler"
	"latex-editor/internal/workspace"
	"latex-editor/web"
)

func TestEndToEnd_WebAndCompilation(t *testing.T) {
	if _, err := exec.LookPath("pdflatex"); err != nil {
		t.Skip("pdflatex not installed, skipping e2e test")
	}

	tmpDir := t.TempDir()
	ws, err := workspace.New(tmpDir)
	if err != nil {
		t.Fatalf("failed to init workspace: %v", err)
	}

	// Create a valid starter main.tex
	starterTex := `\documentclass{article}
\begin{document}
End-to-End Test Document
\end{document}`
	if err := ws.WriteFile("main.tex", []byte(starterTex)); err != nil {
		t.Fatalf("failed to write main.tex: %v", err)
	}

	engine := compiler.NewPdflatexEngine()
	srv := New(ws, engine)

	assetsFS, err := web.GetAssetsFS()
	if err != nil {
		t.Fatalf("failed to load web assets: %v", err)
	}
	srv.SetWebHandler(NewSPAHandler(assetsFS))

	handler := srv.Routes()

	// 1. Verify Root Web UI is served
	rootReq := httptest.NewRequest(http.MethodGet, "/", nil)
	rootRec := httptest.NewRecorder()
	handler.ServeHTTP(rootRec, rootReq)

	if rootRec.Code != http.StatusOK {
		t.Errorf("expected 200 for root UI, got %d", rootRec.Code)
	}
	if !bytes.Contains(rootRec.Body.Bytes(), []byte("LaTeX-Ed")) {
		t.Errorf("expected body to contain 'LaTeX-Ed'")
	}

	// 2. Trigger compilation via API
	compileReq := httptest.NewRequest(http.MethodPost, "/api/compile", bytes.NewBufferString(`{"main_file":"main.tex"}`))
	compileReq.Header.Set("Content-Type", "application/json")
	compileRec := httptest.NewRecorder()
	handler.ServeHTTP(compileRec, compileReq)

	if compileRec.Code != http.StatusOK {
		t.Fatalf("compile failed: %d - %s", compileRec.Code, compileRec.Body.String())
	}
	if !bytes.Contains(compileRec.Body.Bytes(), []byte(`"success":true`)) {
		t.Errorf("expected compile result success:true, got: %s", compileRec.Body.String())
	}

	// 3. Fetch compiled PDF
	pdfReq := httptest.NewRequest(http.MethodGet, "/api/pdf", nil)
	pdfRec := httptest.NewRecorder()
	handler.ServeHTTP(pdfRec, pdfReq)

	if pdfRec.Code != http.StatusOK {
		t.Fatalf("failed to fetch PDF: %d", pdfRec.Code)
	}
	if pdfRec.Header().Get("Content-Type") != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %s", pdfRec.Header().Get("Content-Type"))
	}
	if !bytes.HasPrefix(pdfRec.Body.Bytes(), []byte("%PDF-")) {
		t.Errorf("expected valid PDF header, got %q", pdfRec.Body.Bytes()[:10])
	}
}

func TestEndToEnd_BuiltinTemplateCompilation(t *testing.T) {
	if _, err := exec.LookPath("pdflatex"); err != nil {
		t.Skip("pdflatex not installed, skipping e2e test")
	}

	tmpDir := t.TempDir()
	ws, err := workspace.New(tmpDir)
	if err != nil {
		t.Fatalf("failed to init workspace: %v", err)
	}

	engine := compiler.NewPdflatexEngine()
	srv := New(ws, engine)
	handler := srv.Routes()

	// 1. Create a new document with template=true
	createReq := httptest.NewRequest(http.MethodPost, "/api/files/content?path=howto.tex&template=true", bytes.NewReader(nil))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create file failed: %d - %s", createRec.Code, createRec.Body.String())
	}

	// 2. Compile the template
	compileReq := httptest.NewRequest(http.MethodPost, "/api/compile", bytes.NewBufferString(`{"main_file":"howto.tex"}`))
	compileReq.Header.Set("Content-Type", "application/json")
	compileRec := httptest.NewRecorder()
	handler.ServeHTTP(compileRec, compileReq)

	if compileRec.Code != http.StatusOK {
		t.Fatalf("compile failed: %d - %s", compileRec.Code, compileRec.Body.String())
	}
	if !bytes.Contains(compileRec.Body.Bytes(), []byte(`"success":true`)) {
		t.Fatalf("expected compile success, got: %s", compileRec.Body.String())
	}

	// 3. Fetch PDF and ensure it's generated
	pdfReq := httptest.NewRequest(http.MethodGet, "/api/pdf?path=howto.pdf", nil)
	pdfRec := httptest.NewRecorder()
	handler.ServeHTTP(pdfRec, pdfReq)

	if pdfRec.Code != http.StatusOK {
		t.Fatalf("failed to fetch PDF: %d", pdfRec.Code)
	}
	if !bytes.HasPrefix(pdfRec.Body.Bytes(), []byte("%PDF-")) {
		t.Errorf("expected valid PDF header, got %q", pdfRec.Body.Bytes()[:10])
	}
}

