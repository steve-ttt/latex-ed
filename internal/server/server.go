package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"latex-editor/internal/compiler"
	"latex-editor/internal/synctex"
	"latex-editor/internal/workspace"
)

// Server coordinates the HTTP REST API, SSE streaming, and compiler backend.
type Server struct {
	ws      *workspace.Workspace
	engine  compiler.Engine
	synctex *synctex.SyncTex

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
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

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
		body, err := io.ReadAll(r.Body)
		if err != nil {
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

	var reqBody compilePayload
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
	}
	if reqBody.MainFile == "" {
		if files, err := s.ws.ListFiles(); err == nil {
			for _, f := range files {
				if !f.IsDir && strings.HasSuffix(f.Name, ".tex") {
					reqBody.MainFile = f.Path
					break
				}
			}
		}
		if reqBody.MainFile == "" {
			reqBody.MainFile = "main.tex"
		}
	}

	s.BroadcastEvent("compile_start", fmt.Sprintf(`{"main_file":%q}`, reqBody.MainFile))

	res, err := s.engine.Compile(r.Context(), compiler.CompileRequest{
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
		// Look in cache directory
		cacheCandidate := filepath.Join(s.ws.RootDir(), ".latex-cache", filepath.Base(requestedFile))
		if _, err := os.Stat(cacheCandidate); err == nil {
			pdfPath = cacheCandidate
		} else {
			// Look in root directory
			rootCandidate := filepath.Join(s.ws.RootDir(), filepath.Clean(requestedFile))
			if _, err := os.Stat(rootCandidate); err == nil {
				pdfPath = rootCandidate
			}
		}
	}

	if pdfPath == "" {
		if res != nil && res.PdfFile != "" {
			if _, err := os.Stat(res.PdfFile); err == nil {
				pdfPath = res.PdfFile
			}
		}
	}

	if pdfPath == "" {
		// Fallback: look for any .pdf in cache directory
		cacheDir := filepath.Join(s.ws.RootDir(), ".latex-cache")
		if entries, err := os.ReadDir(cacheDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".pdf") {
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
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".pdf") {
					pdfPath = filepath.Join(s.ws.RootDir(), e.Name())
					break
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
