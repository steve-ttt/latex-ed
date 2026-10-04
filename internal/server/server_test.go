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
	"latex-editor/internal/templates"
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

func TestServer_Pdf_Security(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	handler := srv.Routes()

	// 1. Non-pdf extension rejected
	req1 := httptest.NewRequest(http.MethodGet, "/api/pdf?file=../../etc/passwd", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-pdf file, got %d", rec1.Code)
	}

	// 2. Traversal with .pdf extension rejected
	req2 := httptest.NewRequest(http.MethodGet, "/api/pdf?file=../../etc/secret.pdf", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusForbidden {
		t.Errorf("expected 403 for path traversal, got %d", rec2.Code)
	}

	// 3. Missing pdf returns 404
	req3 := httptest.NewRequest(http.MethodGet, "/api/pdf?file=nonexistent.pdf", nil)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing pdf, got %d", rec3.Code)
	}
}

func TestServer_Compile_Validation(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	handler := srv.Routes()

	// 1. Malformed JSON rejected with 400
	req1 := httptest.NewRequest(http.MethodPost, "/api/compile", bytes.NewBufferString(`{invalid-json`))
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed compile JSON, got %d", rec1.Code)
	}

	// 2. Non-tex file rejected with 400
	req2 := httptest.NewRequest(http.MethodPost, "/api/compile", bytes.NewBufferString(`{"main_file":"script.sh"}`))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-tex main file, got %d", rec2.Code)
	}

	// 3. Out-of-workspace traversal rejected with 403
	req3 := httptest.NewRequest(http.MethodPost, "/api/compile", bytes.NewBufferString(`{"main_file":"../../evil.tex"}`))
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusForbidden {
		t.Errorf("expected 403 for traversal main file, got %d", rec3.Code)
	}

	// 4. Missing tex file returns 404
	req4 := httptest.NewRequest(http.MethodPost, "/api/compile", bytes.NewBufferString(`{"main_file":"missing.tex"}`))
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent main file, got %d", rec4.Code)
	}
}

func TestServer_PayloadLimit(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	handler := srv.Routes()

	// Create oversized payload (> 20MB)
	oversized := make([]byte, 21<<20)
	req := httptest.NewRequest(http.MethodPost, "/api/files/content?path=huge.tex", bytes.NewReader(oversized))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 for oversized file upload, got %d", rec.Code)
	}
}

func TestServer_CORS_Policy(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	handler := srv.Routes()

	// 1. External untrusted origin should not receive CORS headers
	reqExternal := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	reqExternal.Header.Set("Origin", "http://evil-site.com")
	recExternal := httptest.NewRecorder()
	handler.ServeHTTP(recExternal, reqExternal)

	if allowOrigin := recExternal.Header().Get("Access-Control-Allow-Origin"); allowOrigin != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for untrusted origin, got %q", allowOrigin)
	}

	// 2. Localhost origin should be allowed
	reqLocal := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	reqLocal.Header.Set("Origin", "http://localhost:3000")
	recLocal := httptest.NewRecorder()
	handler.ServeHTTP(recLocal, reqLocal)

	if allowOrigin := recLocal.Header().Get("Access-Control-Allow-Origin"); allowOrigin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin: http://localhost:3000, got %q", allowOrigin)
	}

	// 3. 127.0.0.1 origin should be allowed
	reqLoopback := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	reqLoopback.Header.Set("Origin", "http://127.0.0.1:8080")
	recLoopback := httptest.NewRecorder()
	handler.ServeHTTP(recLoopback, reqLoopback)

	if allowOrigin := recLoopback.Header().Get("Access-Control-Allow-Origin"); allowOrigin != "http://127.0.0.1:8080" {
		t.Errorf("expected Access-Control-Allow-Origin: http://127.0.0.1:8080, got %q", allowOrigin)
	}
}

func TestServer_CompileCopiesPdfToSourceDir(t *testing.T) {
	if _, err := exec.LookPath("pdflatex"); err != nil {
		t.Skip("pdflatex not found on PATH, skipping integration test")
	}

	tmpDir := t.TempDir()
	ws, err := workspace.New(tmpDir)
	if err != nil {
		t.Fatalf("failed to init workspace: %v", err)
	}

	texContent := `\documentclass{article}
\begin{document}
Document alongside PDF test
\end{document}`
	if err := ws.WriteFile("anealing.tex", []byte(texContent)); err != nil {
		t.Fatalf("failed to write tex: %v", err)
	}

	engine := compiler.NewPdflatexEngine()
	srv := New(ws, engine)
	handler := srv.Routes()

	// Compile anealing.tex
	compileReq := httptest.NewRequest(http.MethodPost, "/api/compile", bytes.NewBufferString(`{"main_file":"anealing.tex"}`))
	compileReq.Header.Set("Content-Type", "application/json")
	compileRec := httptest.NewRecorder()
	handler.ServeHTTP(compileRec, compileReq)

	if compileRec.Code != http.StatusOK {
		t.Fatalf("compile failed: %d - %s", compileRec.Code, compileRec.Body.String())
	}

	// 1. Verify anealing.pdf exists in the same directory as anealing.tex
	pdfInSource := filepath.Join(tmpDir, "anealing.pdf")
	if fi, err := os.Stat(pdfInSource); err != nil || fi.Size() == 0 {
		t.Fatalf("expected anealing.pdf in source dir, err: %v", err)
	}

	// 2. Verify .latex-cache artifacts remain
	pdfInCache := filepath.Join(tmpDir, ".latex-cache", "anealing.pdf")
	if fi, err := os.Stat(pdfInCache); err != nil || fi.Size() == 0 {
		t.Fatalf("expected anealing.pdf in .latex-cache, err: %v", err)
	}

	// 3. Verify GET /api/files lists anealing.pdf in workspace
	listReq := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)

	var files []workspace.FileInfo
	if err := json.Unmarshal(listRec.Body.Bytes(), &files); err != nil {
		t.Fatalf("unmarshal files failed: %v", err)
	}

	foundPdf := false
	for _, f := range files {
		if f.Path == "anealing.pdf" {
			foundPdf = true
			break
		}
	}
	if !foundPdf {
		t.Errorf("expected anealing.pdf in workspace file list, got: %+v", files)
	}
}

func TestServer_DirectoriesAPI(t *testing.T) {
	srv, _, tmpDir := setupTestServer(t)
	handler := srv.Routes()

	// 1. Create directory via query parameter
	req1 := httptest.NewRequest(http.MethodPost, "/api/directories?path=module3", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected 201 for creating directory, got %d: %s", rec1.Code, rec1.Body.String())
	}
	info, err := os.Stat(filepath.Join(tmpDir, "module3"))
	if err != nil || !info.IsDir() {
		t.Fatalf("expected module3 directory on disk, got err: %v", err)
	}

	// 2. Create directory via JSON body
	req2 := httptest.NewRequest(http.MethodPost, "/api/directories", bytes.NewBufferString(`{"path":"module4/sub"}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected 201 for JSON directory creation, got %d: %s", rec2.Code, rec2.Body.String())
	}
	info, err = os.Stat(filepath.Join(tmpDir, "module4", "sub"))
	if err != nil || !info.IsDir() {
		t.Fatalf("expected module4/sub directory on disk, got err: %v", err)
	}

	// 3. Reject path traversal with 403
	req3 := httptest.NewRequest(http.MethodPost, "/api/directories?path=../../escaped", nil)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusForbidden {
		t.Errorf("expected 403 for directory traversal, got %d", rec3.Code)
	}

	// 4. Reject empty path with 400
	req4 := httptest.NewRequest(http.MethodPost, "/api/directories", nil)
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty path, got %d", rec4.Code)
	}
}

func TestServer_CreateFileInSubdirectoryAPI(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	handler := srv.Routes()

	// 1. Create file in subdirectory
	texContent := "\\section{Sub}\\input{test}"
	writeReq := httptest.NewRequest(http.MethodPost, "/api/files/content?path=module5/chapter1.tex", bytes.NewBufferString(texContent))
	writeRec := httptest.NewRecorder()
	handler.ServeHTTP(writeRec, writeReq)
	if writeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 writing file in subdirectory, got %d: %s", writeRec.Code, writeRec.Body.String())
	}

	// 2. Read back
	readReq := httptest.NewRequest(http.MethodGet, "/api/files/content?path=module5/chapter1.tex", nil)
	readRec := httptest.NewRecorder()
	handler.ServeHTTP(readRec, readReq)
	if readRec.Code != http.StatusOK {
		t.Fatalf("expected 200 reading file in subdirectory, got %d", readRec.Code)
	}
	if readRec.Body.String() != texContent {
		t.Errorf("unexpected content: got %q, want %q", readRec.Body.String(), texContent)
	}

	// 3. Verify in ListFiles
	listReq := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	var files []workspace.FileInfo
	_ = json.Unmarshal(listRec.Body.Bytes(), &files)

	foundDir := false
	foundFile := false
	for _, f := range files {
		if f.Path == "module5" && f.IsDir {
			foundDir = true
		}
		if f.Path == "module5/chapter1.tex" && !f.IsDir {
			foundFile = true
		}
	}
	if !foundDir {
		t.Errorf("expected module5 directory in ListFiles")
	}
	if !foundFile {
		t.Errorf("expected module5/chapter1.tex in ListFiles")
	}
}

func TestServer_TemplateAPI(t *testing.T) {
	wsDir := t.TempDir()
	ws, err := workspace.New(wsDir)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	srv := New(ws, &mockEngine{})
	emptyTplDir := t.TempDir()
	srv.SetTemplateResolver(templates.NewResolver(emptyTplDir))
	handler := srv.Routes()

	// 1. GET /api/template without path (defaults to LaTeX template)
	req1 := httptest.NewRequest(http.MethodGet, "/api/template", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec1.Code)
	}
	if rec1.Body.String() != templates.BuiltinHowToTemplate {
		t.Errorf("expected BuiltinHowToTemplate for /api/template")
	}

	// 2. GET /api/template?path=notes.tex
	req2 := httptest.NewRequest(http.MethodGet, "/api/template?path=notes.tex", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
	if rec2.Body.String() != templates.BuiltinHowToTemplate {
		t.Errorf("expected BuiltinHowToTemplate for /api/template?path=notes.tex")
	}

	// 3. GET /api/template?path=references.bib (non-tex file returns empty)
	req3 := httptest.NewRequest(http.MethodGet, "/api/template?path=references.bib", nil)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec3.Code)
	}
	if rec3.Body.String() != "" {
		t.Errorf("expected empty string for non-tex file, got %q", rec3.Body.String())
	}
}

func TestServer_CreateFileWithTemplate(t *testing.T) {
	wsDir := t.TempDir()
	ws, err := workspace.New(wsDir)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	srv := New(ws, &mockEngine{})
	emptyTplDir := t.TempDir()
	srv.SetTemplateResolver(templates.NewResolver(emptyTplDir))
	handler := srv.Routes()

	// Create file with template=true and empty body
	req := httptest.NewRequest(http.MethodPost, "/api/files/content?path=intro.tex&template=true", bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	data, err := ws.ReadFile("intro.tex")
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	if string(data) != templates.BuiltinHowToTemplate {
		t.Errorf("expected intro.tex to contain built-in template")
	}
}

func TestServer_CreateFileWithCustomTemplate(t *testing.T) {
	wsDir := t.TempDir()
	ws, err := workspace.New(wsDir)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	customDir := t.TempDir()
	customTemplate := "\\documentclass{article}\n% Custom User Paper\n\\begin{document}\nWelcome\n\\end{document}\n"
	err = os.WriteFile(filepath.Join(customDir, "default.tex"), []byte(customTemplate), 0644)
	if err != nil {
		t.Fatalf("failed to write custom default.tex: %v", err)
	}

	srv := New(ws, &mockEngine{})
	srv.SetTemplateResolver(templates.NewResolver(customDir))
	handler := srv.Routes()

	// 1. Check GET /api/template returns custom template
	tplReq := httptest.NewRequest(http.MethodGet, "/api/template?path=paper.tex", nil)
	tplRec := httptest.NewRecorder()
	handler.ServeHTTP(tplRec, tplReq)
	if tplRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", tplRec.Code)
	}
	if tplRec.Body.String() != customTemplate {
		t.Errorf("expected custom template, got %q", tplRec.Body.String())
	}

	// 2. Create file with template=true and empty body
	req := httptest.NewRequest(http.MethodPost, "/api/files/content?path=paper.tex&template=true", bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	data, err := ws.ReadFile("paper.tex")
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	if string(data) != customTemplate {
		t.Errorf("expected paper.tex to contain custom template, got %q", string(data))
	}
}



