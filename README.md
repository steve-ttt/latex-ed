# LaTeX-Ed

A fast, local-first, zero-cloud LaTeX workbench running as a local web service (like Jupyter Notebook / Overleaf). Built in Go with zero external dependencies, embedding the web interface directly into a single standalone binary.

## Features

- **Local & Private:** Zero cloud dependencies. Edits files directly on your local disk in your git repository.
- **Clean Build Cache:** Intermediate build files (`.aux`, `.log`, `.synctex.gz`) are isolated to `.latex-cache/` so your source folder stays pristine.
- **Overleaf-style Dual Pane:** Source code editor on the left with line numbers and error indicators; live PDF preview on the right.
- **Diagnostics & Error Parser:** Formats TeX error logs into structured issues; clicking an error in the drawer scrolls and highlights the offending line.
- **Bidirectional SyncTeX:** Synchronize between source code and PDF preview (`Ctrl+J` or "Jump to PDF").
- **Multi-File Projects:** Works seamlessly with `\input{...}` and `\include{...}` across project subfolders.
- **Live Build Feedback:** Server-Sent Events (SSE) push compilation status and live updates to the browser.
- **Zero-Dependency Single Binary:** The web frontend is embedded into the executable with `//go:embed`.

---

## Quick Start

### Build & Run
```bash
# Build the binary
make build

# Run on default port 8081
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
  -open
        Automatically open web browser on startup
  -port int
        Port for the LaTeX editor web service (default 8081)
```

Example serving a specific paper directory:
```bash
./bin/latex-editor -port 8081 -dir ~/papers/my-paper -open
```

---

## Keyboard Shortcuts

| Shortcut | Action |
| :--- | :--- |
| `Alt+I` / `Cmd+Shift+I` | Open Insert LaTeX Tag / Symbol palette |
| `Ctrl+F` / `Cmd+F` | Open Find bar |
| `Ctrl+H` / `Cmd+H` | Open Find & Replace bar |
| `Ctrl+S` / `Cmd+S` | Save file & recompile |
| `Ctrl+Enter` / `Cmd+Enter` | Recompile project |
| `Ctrl+J` / `Cmd+J` | SyncTeX jump to corresponding PDF location |
| `Escape` | Close Insert palette / Find dialog |
| `Tab` | Indent with 2 spaces |

---

## Testing

The project was developed with strict Agile TDD:
```bash
make test
```
Runs unit and integration tests with Go's race detector (`-race`) enabled across:
- `internal/compiler`: TeX error log parsing and process execution
- `internal/workspace`: Path traversal guards and file operations
- `internal/server`: HTTP API, SSE streaming, and SPA routing
- `internal/synctex`: Coordinate resolution via `synctex` CLI
