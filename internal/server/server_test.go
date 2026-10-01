package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"latex-editor/internal/compiler"
	"latex-editor/internal/workspace"
)

// mockEngine implements compiler.Engine for server unit testing.
type mockEngine struct {
	compileFunc func(ctx context.Context, req compiler.CompileRequest) (*compiler.CompileResult, error)
}

func (m *mockEngine) Compile(ctx context.Context, req compiler.CompileRequest) (*compiler.CompileResult, error) {
	if m.compileFunc != nil {
		return m.compileFunc(ctx, req)
	}
	return &compiler.CompileResult{Success: true}, nil
}

func setupTestServer(t *testing.T) (*Server, *workspace.Workspace, string) {
	t.Helper()
	tmpDir := t.TempDir()
	ws, err := workspace.New(tmpDir)
	if err != nil {
		t.Fatalf("failed to init workspace: %v", err)
	}

	srv := New(ws, &mockEngine{})
	return srv, ws, tmpDir
}

func TestServer_FilesAPI_CRUD(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	handler := srv.Routes()

	// 1. Write file
	writeReq := httptest.NewRequest(http.MethodPost, "/api/files/content?path=main.tex", bytes.NewBufferString("\\documentclass{article}"))
	writeRec := httptest.NewRecorder()
	handler.ServeHTTP(writeRec, writeReq)

	if writeRec.Code != http.StatusOK {
		t.Fatalf("expected write status 200, got %d: %s", writeRec.Code, writeRec.Body.String())
	}

	// 2. Read file
	readReq := httptest.NewRequest(http.MethodGet, "/api/files/content?path=main.tex", nil)
	readRec := httptest.NewRecorder()
	handler.ServeHTTP(readRec, readReq)

	if readRec.Code != http.StatusOK {
		t.Fatalf("expected read status 200, got %d: %s", readRec.Code, readRec.Body.String())
	}
	if readRec.Body.String() != "\\documentclass{article}" {
		t.Errorf("unexpected content: %s", readRec.Body.String())
	}

	// 3. List files
	listReq := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d", listRec.Code)
	}

	var files []workspace.FileInfo
	if err := json.Unmarshal(listRec.Body.Bytes(), &files); err != nil {
		t.Fatalf("failed to unmarshal file list: %v", err)
	}
	if len(files) != 1 || files[0].Path != "main.tex" {
		t.Fatalf("expected file list with main.tex, got: %+v", files)
	}

	// 4. Traversal attack blocked
	traversalReq := httptest.NewRequest(http.MethodGet, "/api/files/content?path=../../etc/passwd", nil)
	traversalRec := httptest.NewRecorder()
	handler.ServeHTTP(traversalRec, traversalReq)

	if traversalRec.Code != http.StatusForbidden && traversalRec.Code != http.StatusBadRequest {
		t.Errorf("expected 403 or 400 for path traversal, got %d", traversalRec.Code)
	}

	// 5. Delete file
	delReq := httptest.NewRequest(http.MethodDelete, "/api/files?path=main.tex", nil)
	delRec := httptest.NewRecorder()
	handler.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusOK {
		t.Fatalf("expected delete status 200, got %d", delRec.Code)
	}
}

func TestServer_CompileAndPdf(t *testing.T) {
	srv, ws, tmpDir := setupTestServer(t)

	// Create fake PDF file in cache
	cacheDir := filepath.Join(tmpDir, ".latex-cache")
	_ = os.MkdirAll(cacheDir, 0755)
	fakePdf := filepath.Join(cacheDir, "main.pdf")
	_ = os.WriteFile(fakePdf, []byte("%PDF-1.5 fake data"), 0644)

	// Set engine mock to return the fake PDF
	srv.engine = &mockEngine{
		compileFunc: func(ctx context.Context, req compiler.CompileRequest) (*compiler.CompileResult, error) {
			return &compiler.CompileResult{
				Success: true,
				PdfFile: fakePdf,
				Diagnostics: []compiler.Diagnostic{
					{Severity: compiler.SeverityInfo, Message: "Build successful"},
				},
				DurationMs: 120,
			}, nil
		},
	}

	_ = ws.WriteFile("main.tex", []byte("valid tex"))
	handler := srv.Routes()

	// POST /api/compile
	compileBody := []byte(`{"main_file":"main.tex"}`)
	compileReq := httptest.NewRequest(http.MethodPost, "/api/compile", bytes.NewBuffer(compileBody))
	compileReq.Header.Set("Content-Type", "application/json")
	compileRec := httptest.NewRecorder()
	handler.ServeHTTP(compileRec, compileReq)

	if compileRec.Code != http.StatusOK {
		t.Fatalf("expected compile 200, got %d: %s", compileRec.Code, compileRec.Body.String())
	}

	var res compiler.CompileResult
	if err := json.Unmarshal(compileRec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode compile response: %v", err)
	}
	if !res.Success {
		t.Errorf("expected compile success")
	}

	// GET /api/pdf
	pdfReq := httptest.NewRequest(http.MethodGet, "/api/pdf", nil)
	pdfRec := httptest.NewRecorder()
	handler.ServeHTTP(pdfRec, pdfReq)

	if pdfRec.Code != http.StatusOK {
		t.Fatalf("expected PDF 200, got %d", pdfRec.Code)
	}
	if pdfRec.Header().Get("Content-Type") != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %s", pdfRec.Header().Get("Content-Type"))
	}
	if pdfRec.Body.String() != "%PDF-1.5 fake data" {
		t.Errorf("unexpected PDF content: %q", pdfRec.Body.String())
	}
}

func TestServer_EventsStream(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	handler := srv.Routes()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	eventsReq := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	eventsRec := httptest.NewRecorder()

	done := make(chan bool)
	go func() {
		handler.ServeHTTP(eventsRec, eventsReq)
		done <- true
	}()

	// Wait briefly for connection
	time.Sleep(20 * time.Millisecond)
	srv.BroadcastEvent("compile_done", `{"success":true}`)

	select {
	case <-done:
		// Client disconnected due to timeout
	case <-time.After(500 * time.Millisecond):
		t.Fatal("events handler did not exit after context cancellation")
	}

	if eventsRec.Code != http.StatusOK {
		t.Errorf("expected events status 200, got %d", eventsRec.Code)
	}
	if !bytes.Contains(eventsRec.Body.Bytes(), []byte("event: compile_done")) {
		t.Errorf("expected body to contain event: compile_done, got: %s", eventsRec.Body.String())
	}
}

func TestServer_SyncTex_Endpoints(t *testing.T) {
	if _, err := exec.LookPath("synctex"); err != nil {
		t.Skip("synctex not installed, skipping test")
	}
	if _, err := exec.LookPath("pdflatex"); err != nil {
		t.Skip("pdflatex not installed, skipping test")
	}

	tmpDir := t.TempDir()
	ws, err := workspace.New(tmpDir)
	if err != nil {
		t.Fatalf("failed to init workspace: %v", err)
	}

	texContent := `\documentclass{article}
\begin{document}
SyncTex line 3
SyncTex line 4
\end{document}`
	if err := ws.WriteFile("main.tex", []byte(texContent)); err != nil {
		t.Fatalf("failed to write main.tex: %v", err)
	}

	engine := compiler.NewPdflatexEngine()
	srv := New(ws, engine)
	handler := srv.Routes()

	// 1. Compile
	compileReq := httptest.NewRequest(http.MethodPost, "/api/compile", bytes.NewBufferString(`{"main_file":"main.tex"}`))
	compileRec := httptest.NewRecorder()
	handler.ServeHTTP(compileRec, compileReq)
	if compileRec.Code != http.StatusOK {
		t.Fatalf("compile failed: %d", compileRec.Code)
	}

	// 2. Forward search
	fwdBody := bytes.NewBufferString(`{"file":"main.tex","line":3}`)
	fwdReq := httptest.NewRequest(http.MethodPost, "/api/synctex/forward", fwdBody)
	fwdRec := httptest.NewRecorder()
	handler.ServeHTTP(fwdRec, fwdReq)

	if fwdRec.Code != http.StatusOK {
		t.Fatalf("forward synctex failed: %d - %s", fwdRec.Code, fwdRec.Body.String())
	}

	// 3. Inverse search
	invBody := bytes.NewBufferString(`{"page":1,"x":150,"y":200}`)
	invReq := httptest.NewRequest(http.MethodPost, "/api/synctex/inverse", invBody)
	invRec := httptest.NewRecorder()
	handler.ServeHTTP(invRec, invReq)

	if invRec.Code != http.StatusOK {
		t.Fatalf("inverse synctex failed: %d - %s", invRec.Code, invRec.Body.String())
	}
}
