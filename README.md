# LaTeX-Ed

A fast, local-first, zero-cloud LaTeX workbench running as a local web service (like Jupyter Notebook / Overleaf). Built in Go with zero external runtime dependencies, embedding the web interface directly into a single standalone binary.

---

## Features

### 🖥️ Overleaf-Style Multi-Pane Interface
- **Collapsible File Explorer:** Hierarchical directory tree with subfolders collapsed by default, click-to-expand folders, and active file tracking.
- **Subdirectory & File Management:** Create directories (`+ Folder`), create files anywhere (`+ File` or quick folder `+`), and delete files or folders directly from the sidebar.
- **Dynamic Pane Resizers:** Draggable resize handles between all panes (Project Files ↔ Editor ↔ PDF Preview) with double-click reset and `localStorage` layout persistence.
- **Source-Adjacent PDF Output:** Output PDFs are placed directly in the same folder as their source `.tex` file for easy access, while keeping intermediate build artifacts isolated.
- **Direct PDF Viewing:** Click any `.pdf` file in the project tree to view it immediately in the preview pane.

### ✍️ Code Editor & LaTeX Syntax Highlighting
- **IDE Syntax Highlighting:** Lightweight, zero-dependency token highlighter with a modern dark theme:
  - Commands & macros (`\section`, `\textbf`, `\sum`) in vibrant blue/cyan
  - Environments (`\begin{equation}`, `\begin{lstlisting}`) in mint teal
  - Braces & delimiters (`{}`, `[]`) in gold
  - Comments (`% ...`) in italic forest green
  - Inline and display math (`$...$`, `$$...$$`) in warm amber
  - Numeric constants and units in sage green
- **Editor Ergonomics:** Line numbering with wrapped-line gutter alignment, toggleable word wrap, Find (`Ctrl+F`), and Find & Replace (`Ctrl+H`).

### ➕ Insert LaTeX Tag & Symbol Palette (`Alt+I`)
- **Instant Floating Palette:** Press `Alt+I` (or `Cmd+Shift+I`) or click **➕ Insert ▾** to open a searchable palette of LaTeX snippets and math symbols.
- **Categorized Sections:**
  - **Environments:** `itemize`, `enumerate`, `description`, `equation`, `align*`, `figure`, `table`, `lstlisting` (code blocks with syntax highlighting), `quote`, `abstract`.
  - **Math & Calculus:** Fractions (`\frac`), integrals (`\int`), summations (`\sum`), square roots (`\sqrt`), limits (`\lim`), partial derivatives (`\partial`), products (`\prod`), binomial coefficients (`\binom`).
  - **Greek Letters:** Complete uppercase and lowercase Greek alphabet with visual glyph badges (α, β, γ, δ, ε, θ, λ, μ, π, σ, ω, Δ, Γ, Σ, Ω, etc.).
  - **Relational & Logic:** $\le$, $\ge$, $\ne$, $\approx$, $\pm$, $\times$, $\div$, $\in$, $\notin$, $\subset$, $\subseteq$, $\cup$, $\cap$, $\implies$, $\iff$, $\forall$, $\exists$.
  - **Text Formatting:** Bold (`\textbf`), italic (`\textit`), monospace (`\texttt`), inline code (`\lstinline||`), underline (`\underline`), emphasize (`\emph`).
  - **Matrices & Arrays:** `pmatrix`, `bmatrix`, `vmatrix`, `matrix`, `cases`.
- **Smart Insertion Logic:** Automatically wraps selected text or positions the caret inside template placeholders (e.g. `[language=...]` or `$0`).

### 📄 Smart Document Templates
- **Cross-Platform User Templates:** When creating a new document, LaTeX-Ed automatically checks your user template folder for a `default.tex` file:
  - **Linux / Unix:** `~/Templates/default.tex`, `$XDG_TEMPLATES_DIR`, or `~/.config/user-dirs.dirs`
  - **macOS:** `~/Templates/default.tex`, `~/Library/Application Support/Templates/default.tex`
  - **Windows:** `%APPDATA%\Microsoft\Windows\Templates\default.tex` or `%USERPROFILE%\Templates\default.tex`
- **Built-in HOW-TO Guide:** If no custom `default.tex` exists, new documents automatically initialize with a comprehensive, clean 2-page Getting Started guide covering document structure, formatting, math typesetting, listings, and workbench shortcuts.
- **Extension-Aware:** Non-TeX files (e.g. `.bib`, `.txt`, `Makefile`) are safely initialized without injecting LaTeX boilerplate.

### ⚡ Live Compilation & SyncTeX
- **Clean Build Cache:** All intermediate build artifacts (`.aux`, `.log`, `.synctex.gz`, `.out`) are isolated inside `.latex-cache/` so your workspace stays clean.
- **Source-Adjacent Output:** Automatically copies the final PDF back into the source document's directory alongside the `.tex` file.
- **Bidirectional SyncTeX:** Synchronize between source code and PDF preview via `Ctrl+J` ("Jump to PDF"). SyncTeX automatically searches both the source directory and `.latex-cache/`.
- **Live Diagnostics Drawer:** Formats TeX error logs into structured issues; clicking an error in the drawer immediately scrolls and highlights the offending line in the editor.
- **Real-Time SSE Feedback:** Server-Sent Events (SSE) push compilation status, success/failure events, and live build progress to the browser.

### 🔒 Security-Hardened & Reliable
- **Loopback Binding by Default:** Binds strictly to `127.0.0.1` by default to prevent unauthorized network access.
- **Local CORS Protection:** Restricts API requests strictly to local origins (`localhost` and `127.0.0.1`).
- **Canonical Root Containment (`SafePath`):** Resolves canonical symlink paths to prevent directory traversal and symlink escape attacks (`../../`).
- **Strict PDF Endpoints:** Restricts PDF serving strictly to valid `.pdf` files residing within the workspace bounds.
- **Compiler Serialization & Deadlines:** Enforces mutex serialization and a 45-second execution deadline per workspace to prevent resource starvation or compiler hang.
- **Payload Limits:** Restricts file writes to 20MB and JSON payloads to 1MB.

---

## Quick Start

### Build & Run
```bash
# Build the standalone binary
make build

# Run on default port 8081 (bound to 127.0.0.1)
./bin/latex-editor
```
Then navigate to:
**[http://localhost:8081](http://localhost:8081)**

### CLI Flags
```bash
./bin/latex-editor -h
  -dir string
        Working directory containing LaTeX files (default ".")
  -engine string
        LaTeX engine binary (e.g. pdflatex, xelatex, lualatex) (default "pdflatex")
  -host string
        Host address to bind to (default "127.0.0.1" for local security)
  -open
        Automatically open web browser on startup
  -port int
        Port for the LaTeX editor web service (default 8081)
```

Example serving a specific paper directory and opening your browser:
```bash
./bin/latex-editor -port 8081 -dir ~/papers/quantum-sim -open
```

---

## Keyboard Shortcuts

| Shortcut | Action |
| :--- | :--- |
| `Alt+I` / `Cmd+Shift+I` | Open Insert LaTeX Tag / Symbol palette |
| `Ctrl+S` / `Cmd+S` | Save active file & trigger compilation |
| `Ctrl+Enter` / `Cmd+Enter` | Recompile project |
| `Ctrl+F` / `Cmd+F` | Open Find bar |
| `Ctrl+H` / `Cmd+H` | Open Find & Replace bar |
| `Ctrl+J` / `Cmd+J` | SyncTeX jump to corresponding PDF location |
| `Escape` | Close Insert palette / Find bar / Diagnostics drawer |
| `Tab` | Indent with 2 spaces |

---

## Templates

### Using a Custom Template
To automatically use your own document template whenever you create a new file:
1. Create a `default.tex` file in your system's template folder:
   - **Linux:** `~/Templates/default.tex`
   - **macOS:** `~/Templates/default.tex` or `~/Library/Application Support/Templates/default.tex`
   - **Windows:** `%APPDATA%\Microsoft\Windows\Templates\default.tex`
2. Next time you click **+ File** in LaTeX-Ed, your newly created `.tex` document will be pre-populated with your custom template!

### Built-in Getting Started Guide
If no `default.tex` is found, new `.tex` documents will automatically load a clean 2-page introductory guide demonstrating:
- Preamble setup and document structure
- Text formatting and lists
- Inline and display mathematical formulas (`amsmath`, `align*`)
- Code listings with `listings`
- LaTeX-Ed keyboard shortcuts

---

## REST API Reference

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/files` | List all workspace files and directories |
| `DELETE` | `/api/files?path=<path>` | Delete a file or directory |
| `GET` | `/api/files/content?path=<path>` | Read content of a file |
| `POST` | `/api/files/content?path=<path>[&template=true]` | Write file content (or initialize with template if empty) |
| `POST` | `/api/directories?path=<path>` | Create a new subdirectory |
| `GET` | `/api/template[?path=<filename>]` | Resolve the template for a file (user `default.tex` or built-in HOW-TO) |
| `POST` | `/api/compile` | Trigger compilation (`{"main_file": "main.tex"}`) |
| `GET` | `/api/pdf[?path=<filename>]` | Serve the compiled PDF file |
| `GET` | `/api/events` | Server-Sent Events (SSE) stream for live compilation updates |
| `GET` | `/api/synctex/forward` | Resolve PDF coordinates from source line (`file`, `line`, `col`) |
| `GET` | `/api/synctex/inverse` | Resolve source line from PDF coordinates (`page`, `x`, `y`) |

---

## Architecture & Codebase Structure

```
latex-editor/
├── cmd/
│   └── latex-editor/
│       └── main.go          # CLI entry point, flag parsing, server startup
├── internal/
│   ├── compiler/            # LaTeX engine execution, 45s timeouts, log parsing
│   ├── server/              # HTTP router, REST endpoints, CORS, SSE, SPA handler
│   ├── synctex/             # Bidirectional SyncTeX CLI resolver
│   ├── templates/           # OS-aware template resolution & built-in HOW-TO template
│   └── workspace/           # Canonical path containment, symlink escape checks, file CRUD
└── web/
    ├── dist/                # Embedded static assets (app.js, style.css, index.html)
    └── embed.go             # Go 1.16+ //go:embed integration
```

---

## Testing & Quality Assurance

The codebase is built following strict Agile Test-Driven Development (TDD) with full race detection enabled:

```bash
# Run all unit, integration, and end-to-end tests with race detection
make test
```

### Test Coverage Highlights:
- **`internal/compiler`:** Log parsing for standard errors, undefined control sequences, missing packages, environment mismatches, timeout cancellation, and multi-file projects.
- **`internal/workspace`:** Lexical traversal attacks (`../`), symlink escapes, subfolder creation, directory deletion, and hidden cache isolation.
- **`internal/templates`:** Candidate directory resolution across Linux/macOS/Windows, user `default.tex` loading, fallback to built-in HOW-TO guide, and non-TeX filtering.
- **`internal/server`:** Files API CRUD, directory API, template API, body limits (20MB writes, 1MB JSON), CORS policies, traversal prevention, SSE streaming, SPA routing, and source-adjacent PDF verification.
- **`internal/synctex`:** Forward and inverse coordinate resolution.
- **End-to-End (`e2e_test.go`):** Full-stack integration tests serving web assets, compiling documents with `pdflatex`, and verifying valid PDF headers.

---

## License

MIT License. Designed and crafted for private, local-first LaTeX writing.
