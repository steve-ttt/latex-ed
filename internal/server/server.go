package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"latex-editor/internal/compiler"
	"latex-editor/internal/synctex"
	"latex-editor/internal/workspace"
)

// Server coordinates the HTTP REST API, SSE streaming, and compiler backend.
type Server struct {
	ws      *workspace.Workspace
	engine  compiler.Engine
	synctex *synctex.SyncTex

	compileMu  sync.Mutex // Serializes compilation jobs per workspace (REL-01)
	mu         sync.RWMutex
	lastResult *compiler.CompileResult
	clients    map[chan string]struct{}
	webHandler http.Handler
}

// New initializes a new Server.
func New(ws *workspace.Workspace, engine compiler.Engine) *Server {
	return &Server{
		ws:      ws,
		engine:  engine,
		synctex: synctex.New(),
		clients: make(map[chan string]struct{}),
	}
}

// SetWebHandler attaches a static or embedded web handler for SPA routing.
func (s *Server) SetWebHandler(h http.Handler) {
	s.webHandler = h
}

// Routes constructs the HTTP multiplexer with all API routes.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/files", s.handleFiles)
	mux.HandleFunc("/api/files/content", s.handleFileContent)
	mux.HandleFunc("/api/compile", s.handleCompile)
	mux.HandleFunc("/api/pdf", s.handlePdf)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/synctex/forward", s.handleSyncTexForward)
	mux.HandleFunc("/api/synctex/inverse", s.handleSyncTexInverse)

	if s.webHandler != nil {
		mux.Handle("/", s.webHandler)
	}

	return s.corsMiddleware(mux)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			// Restrict CORS to local/loopback origins only (SEC-02)
			if strings.HasPrefix(origin, "http://localhost:") ||
				strings.HasPrefix(origin, "http://127.0.0.1:") ||
				origin == "http://localhost" ||
				origin == "http://127.0.0.1" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// BroadcastEvent dispatches a server-sent event to all connected listeners.
func (s *Server) BroadcastEvent(event, data string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	payload := fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)
	for ch := range s.clients {
		select {
		case ch <- payload:
		default:
			// Client buffer full or blocked; skip to prevent server stall
		}
	}
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		files, err := s.ws.ListFiles()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if files == nil {
			files = []workspace.FileInfo{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(files)

	case http.MethodDelete:
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "missing path parameter", http.StatusBadRequest)
			return
		}
		if err := s.ws.DeleteFile(path); err != nil {
			if errors.Is(err, workspace.ErrPathTraversal) {
				http.Error(w, "access denied", http.StatusForbidden)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleFileContent(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "missing path parameter", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		data, err := s.ws.ReadFile(path)
		if err != nil {
			if errors.Is(err, workspace.ErrPathTraversal) {
				http.Error(w, "access denied", http.StatusForbidden)
				return
			}
			if os.IsNotExist(err) {
				http.Error(w, "file not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)

	case http.MethodPost:
		// Limit file writes to 20MB (REL-02)
		r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "request payload too large (max 20MB)", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		if err := s.ws.WriteFile(path, body); err != nil {
			if errors.Is(err, workspace.ErrPathTraversal) {
				http.Error(w, "access denied", http.StatusForbidden)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type compilePayload struct {
	MainFile string `json:"main_file"`
}

func (s *Server) handleCompile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit compile payload to 1MB (REL-02)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var reqBody compilePayload
	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&reqBody); err != nil && !errors.Is(err, io.EOF) {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "request payload too large", http.StatusRequestEntityTooLarge)
				return
			}
			// Reject malformed JSON explicitly (REL-03)
			http.Error(w, fmt.Sprintf("invalid json payload: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Validate main file input path (SEC-05)
	if reqBody.MainFile != "" {
		if !strings.HasSuffix(strings.ToLower(reqBody.MainFile), ".tex") {
			http.Error(w, "main file must have .tex extension", http.StatusBadRequest)
			return
		}
		safePath, err := s.ws.SafePath(reqBody.MainFile)
		if err != nil {
			http.Error(w, "access denied: invalid main file path", http.StatusForbidden)
			return
		}
		fi, err := os.Stat(safePath)
		if err != nil || fi.IsDir() {
			http.Error(w, "main file not found", http.StatusNotFound)
			return
		}
		rel, err := filepath.Rel(s.ws.RootDir(), safePath)
		if err != nil {
			http.Error(w, "invalid path resolution", http.StatusInternalServerError)
			return
		}
		reqBody.MainFile = rel
	} else {
		// Discover first .tex file or fall back to main.tex
		if files, err := s.ws.ListFiles(); err == nil {
			for _, f := range files {
				if !f.IsDir && strings.HasSuffix(strings.ToLower(f.Name), ".tex") {
					reqBody.MainFile = f.Path
					break
				}
			}
		}
		if reqBody.MainFile == "" {
			reqBody.MainFile = "main.tex"
		}
	}

	// Serialize compiles per workspace to protect .latex-cache (REL-01)
	s.compileMu.Lock()
	defer s.compileMu.Unlock()

	// Apply server-side compile timeout (REL-01)
	compileCtx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	s.BroadcastEvent("compile_start", fmt.Sprintf(`{"main_file":%q}`, reqBody.MainFile))

	res, err := s.engine.Compile(compileCtx, compiler.CompileRequest{
		RootDir:  s.ws.RootDir(),
		MainFile: reqBody.MainFile,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("compilation failure: %v", err), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.lastResult = res
	s.mu.Unlock()

	resultJson, _ := json.Marshal(res)
	s.BroadcastEvent("compile_done", string(resultJson))

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resultJson)
}

func (s *Server) handlePdf(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requestedFile := r.URL.Query().Get("file")

	s.mu.RLock()
	res := s.lastResult
	s.mu.RUnlock()

	var pdfPath string

	if requestedFile != "" {
		// Only allow PDF files (SEC-01)
		if !strings.HasSuffix(strings.ToLower(requestedFile), ".pdf") {
			http.Error(w, "only PDF files may be requested", http.StatusBadRequest)
			return
		}

		// 1. Look in cache directory by base name only (immune to path traversal)
		cleanBase := filepath.Base(requestedFile)
		cacheCandidate := filepath.Join(s.ws.RootDir(), ".latex-cache", cleanBase)
		if fi, err := os.Stat(cacheCandidate); err == nil && !fi.IsDir() {
			pdfPath = cacheCandidate
		} else {
			// 2. Resolve through SafePath to ensure containment inside workspace
			safePath, err := s.ws.SafePath(requestedFile)
			if err != nil {
				http.Error(w, "access denied", http.StatusForbidden)
				return
			}
			if fi, err := os.Stat(safePath); err == nil && !fi.IsDir() {
				pdfPath = safePath
			}
		}
	}

	if pdfPath == "" {
		if res != nil && res.PdfFile != "" {
			if fi, err := os.Stat(res.PdfFile); err == nil && !fi.IsDir() {
				canonPdf, _ := filepath.EvalSymlinks(res.PdfFile)
				relW, errW := filepath.Rel(s.ws.CanonicalRootDir(), canonPdf)
				if errW == nil && !strings.HasPrefix(relW, "..") {
					pdfPath = res.PdfFile
				}
			}
		}
	}

	if pdfPath == "" {
		// Fallback: look for any .pdf in cache directory
		cacheDir := filepath.Join(s.ws.RootDir(), ".latex-cache")
		if entries, err := os.ReadDir(cacheDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".pdf") {
					pdfPath = filepath.Join(cacheDir, e.Name())
					break
				}
			}
		}
	}

	if pdfPath == "" {
		// Fallback: look for any .pdf in workspace root
		if entries, err := os.ReadDir(s.ws.RootDir()); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".pdf") {
					pdfCandidate := filepath.Join(s.ws.RootDir(), e.Name())
					safePath, err := s.ws.SafePath(e.Name())
					if err == nil && safePath == pdfCandidate {
						pdfPath = pdfCandidate
						break
					}
				}
			}
		}
	}

	if pdfPath == "" {
		http.Error(w, "no compiled PDF found", http.StatusNotFound)
		return
	}

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		http.Error(w, "failed to read PDF", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filepath.Base(pdfPath)))
	_, _ = w.Write(data)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	msgChan := make(chan string, 16)
	s.mu.Lock()
	s.clients[msgChan] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, msgChan)
		close(msgChan)
		s.mu.Unlock()
	}()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg, open := <-msgChan:
			if !open {
				return
			}
			_, _ = fmt.Fprint(w, msg)
			flusher.Flush()
		}
	}
}

func (s *Server) handleSyncTexForward(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var q synctex.ForwardQuery
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if q.File == "" {
		q.File = "main.tex"
	}
	absFile, err := s.ws.SafePath(q.File)
	if err != nil {
		http.Error(w, "invalid file path", http.StatusForbidden)
		return
	}
	q.File = absFile

	s.mu.RLock()
	res := s.lastResult
	s.mu.RUnlock()

	if q.PdfFile == "" {
		if res != nil && res.PdfFile != "" {
			q.PdfFile = res.PdfFile
		} else {
			q.PdfFile = filepath.Join(s.ws.RootDir(), ".latex-cache", "main.pdf")
		}
	}
	if q.Dir == "" {
		q.Dir = filepath.Dir(q.PdfFile)
	}

	fwd, err := s.synctex.ForwardSearch(r.Context(), q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fwd)
}

func (s *Server) handleSyncTexInverse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var q synctex.InverseQuery
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	res := s.lastResult
	s.mu.RUnlock()

	if q.PdfFile == "" {
		if res != nil && res.PdfFile != "" {
			q.PdfFile = res.PdfFile
		} else {
			q.PdfFile = filepath.Join(s.ws.RootDir(), ".latex-cache", "main.pdf")
		}
	}
	if q.Dir == "" {
		q.Dir = filepath.Dir(q.PdfFile)
	}

	inv, err := s.synctex.InverseSearch(r.Context(), q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if rel, err := filepath.Rel(s.ws.RootDir(), inv.File); err == nil && !filepath.IsAbs(rel) {
		inv.File = rel
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inv)
}
