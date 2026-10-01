# Local LaTeX Editor - Project Architecture & Agile TDD Roadmap

A local-first, zero-cloud LaTeX editor running as a local web service (like Jupyter Notebook / Overleaf).

---

## 1. Project Architecture & Directory Structure

```
.
├── cmd/
│   └── latex-editor/
│       └── main.go              # CLI flags (-port 8081, -dir), graceful shutdown
├── internal/
│   ├── compiler/
│   │   ├── types.go             # CompileRequest, CompileResult, Diagnostic models
│   │   ├── engine.go            # Engine interface & pdflatex process executor
│   │   ├── log_parser.go        # TeX error log parser -> structured diagnostics
│   │   └── compiler_test.go     # TDD suite (log parsing & process execution)
│   ├── synctex/
│   │   ├── types.go             # SyncQuery, SyncResult (page, x, y <-> file, line)
│   │   ├── synctex.go           # Forward/inverse coordinate resolution via `synctex` CLI
│   │   └── synctex_test.go      # TDD suite for SyncTeX coordinate resolution
│   ├── workspace/
│   │   ├── fs.go                # Project file operations & path traversal guards
│   │   └── fs_test.go           # Security & file system unit tests
│   └── server/
│       ├── server.go            # HTTP routes, SSE/WebSocket compilation stream
│       ├── handlers.go          # REST API endpoints (/api/compile, /api/files, /api/synctex)
│       ├── embedded.go          # go:embed web/dist binding
│       └── server_test.go       # HTTP integration tests
├── web/                         # Frontend UI (built & embedded into Go binary)
│   ├── package.json
│   ├── vite.config.ts
│   ├── index.html
│   └── src/
│       ├── components/
│       │   ├── Editor.tsx       # CodeMirror 6 (LaTeX syntax, error gutters, line numbers)
│       │   ├── Viewer.tsx       # PDF.js canvas viewer & SyncTeX click layer
│       │   ├── FileTree.tsx     # Project file navigation
│       │   └── LogPanel.tsx     # Structured diagnostics & raw log viewer
│       ├── App.tsx              # Split-pane layout & state coordination
│       └── main.tsx
├── Makefile                     # Standard unix targets: test, build, run
├── TODO.md                      # This roadmap & progress tracker
└── go.mod
```

---

## 2. Core Architectural Decisions

- **Daemon Runtime:** Go (`go-lang`) for concurrency, single-binary distribution, and fast subprocess execution.
- **Port:** Default `8081` (configurable via `-port` flag).
- **Toolchain Philosophy:** Unix philosophy — execute host tools (`pdflatex`, `synctex`, `bibtex`) with standard I/O pipes.
- **Build Isolation:** Output artifacts (`.aux`, `.log`, `.out`, `.pdf`) are directed to an isolated `.latex-cache/` directory to keep source directories clean.
- **Frontend Stack:** Lightweight Vite + TypeScript + CodeMirror 6 + PDF.js.
- **Packaging:** `go:embed` compiles `web/dist/` directly into the final standalone Go binary.

---

## 3. Agile & TDD Roadmap

### Sprint 1: Walking Skeleton - Compilation Engine Core (TDD)
> **Goal:** Compile `.tex` files using host `pdflatex`, isolate build artifacts, and reliably parse error logs into structured diagnostics.

- [x] **Step 1.1: Project Setup**
  - [x] Initialize Go module (`go mod init`).
  - [x] Create directory structure (`cmd/`, `internal/compiler/`, `internal/workspace/`, `internal/server/`).
  - [x] Create `Makefile` with `test`, `build`, and `run` targets.
- [x] **Step 1.2: Diagnostic Log Parser (TDD)**
  - [x] `[TEST]` Write unit tests with real LaTeX log outputs:
    - [x] Undefined control sequence (e.g. `\unknowncmd`).
    - [x] Unmatched environments (e.g. missing `\end{document}` or `\end{equation}`).
    - [x] Missing package or missing input files.
    - [x] Multi-line TeX errors and warning logs.
  - [x] `[CODE]` Implement `ParseLatexLog(rawLog string) []Diagnostic`.
  - [x] `[REFACTOR]` Clean regex and state tracking for line mapping.
- [x] **Step 1.3: Process Execution & Engine (TDD)**
  - [x] `[TEST]` Write integration tests for `PdflatexEngine`:
    - [x] Compile minimal valid LaTeX string -> generates `.pdf` in isolated cache directory.
    - [x] Compile document with syntax errors -> returns parsed diagnostics without crashing.
    - [x] Handle process timeout / runaway compilation (protection against hanging on input prompts).
  - [x] `[CODE]` Implement `PdflatexEngine` with `-interaction=nonstopmode -synctex=1 -output-directory=.latex-cache`.
  - [x] `[REFACTOR]` Extract execution options and ensure temporary directory cleanup.

---

### Sprint 2: Local HTTP/WebSocket Service & File Workspace
> **Goal:** Run local web daemon on port `8081` serving a REST API for file operations and compilation.

- [x] **Step 2.1: Workspace File Security & Operations (TDD)**
  - [x] `[TEST]` Path traversal protection tests (`../../etc/passwd` blocked).
  - [x] `[TEST]` List files, read file, write file unit tests.
  - [x] `[CODE]` Implement `internal/workspace/fs.go`.
- [x] **Step 2.2: HTTP API & Compilation Endpoints (TDD)**
  - [x] `[TEST]` `POST /api/compile` triggering compilation and returning JSON diagnostics + PDF status.
  - [x] `[TEST]` `GET /api/pdf` streaming compiled PDF file.
  - [x] `[TEST]` `GET /api/files` and `POST /api/files` for file CRUD.
  - [x] `[CODE]` Implement HTTP routes using Go standard library / router.
- [x] **Step 2.3: Live Compilation Feedback**
  - [x] SSE endpoint (`/api/events`) for real-time compilation progress and status updates.

---

### Sprint 3: Frontend Web Interface (Overleaf Experience)
> **Goal:** Browser interface with dual-pane layout: editor on left, PDF viewer on right, embedded in single binary.

- [x] **Step 3.1: Zero-Dependency Frontend Architecture**
  - [x] Self-contained, zero-npm web interface avoiding sandbox network isolation issues.
  - [x] Modern Overleaf-style UI with dual pane, file tree, and log drawer.
- [x] **Step 3.2: Editor Component**
  - [x] Full code editing with synchronized line numbers gutter.
  - [x] Tab indentation (2 spaces) and shortcut bindings (Ctrl+S, Ctrl+Enter).
  - [x] Gutter error indicators (red dots & numbers) mapped directly from compiler diagnostics.
- [x] **Step 3.3: PDF Viewer Component**
  - [x] Responsive browser PDF rendering with cache-busting auto-refresh.
  - [x] Download PDF and popout window controls.
- [x] **Step 3.4: Binary Embedding**
  - [x] Configured `go:embed dist/*` in `web/embed.go` with SPA router fallback.
  - [x] Verified full end-to-end integration test (`TestEndToEnd_WebAndCompilation`).

---

### Sprint 4: Bidirectional SyncTeX Navigation
> **Goal:** Click in editor -> jumps to PDF location (forward); click in PDF -> jumps to editor line (inverse).

- [x] **Step 4.1: SyncTeX CLI Wrapper (TDD)**
  - [x] `[TEST]` Forward search: (source file + line + col) -> (page, x, y coordinates).
  - [x] `[TEST]` Inverse search: (page, x, y coordinates) -> (source file + line).
  - [x] `[CODE]` Implement `internal/synctex/synctex.go` wrapping `synctex view` / `synctex edit`.
- [x] **Step 4.2: Frontend Integration**
  - [x] Endpoints `POST /api/synctex/forward` and `POST /api/synctex/inverse`.
  - [x] SyncTeX button and `Ctrl+J` shortcut in editor to synchronize with PDF preview.

---

### Sprint 5: Polish & Developer Experience
- [x] Multi-file support (`\input{}`, `\include{}`) with verified integration test.
- [x] Auto-recompile & instant save (`Ctrl+S`, `Ctrl+Enter`, `Ctrl+J` hotkeys).
- [x] Clean CLI options (`-port 8081`, `-dir`, `-engine pdflatex`, `-open`).
- [x] End-to-end integration and concurrency race testing passed.
