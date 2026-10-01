package compiler

// Severity represents the diagnostic severity level (error, warning, info).
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Diagnostic represents an error or warning extracted from the LaTeX log.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Message  string   `json:"message"`
	Context  string   `json:"context,omitempty"`
}

// CompileRequest contains parameters for compiling a LaTeX project.
type CompileRequest struct {
	// RootDir is the directory containing the project files.
	RootDir string `json:"root_dir"`
	// MainFile is the relative path to the entry document (e.g. "main.tex").
	MainFile string `json:"main_file"`
	// Engine is the compiler binary to use (default: "pdflatex").
	Engine string `json:"engine,omitempty"`
	// Draft mode enables fast syntax checking if supported.
	Draft bool `json:"draft,omitempty"`
}

// CompileResult represents the output of a LaTeX compilation run.
type CompileResult struct {
	Success     bool         `json:"success"`
	PdfFile     string       `json:"pdf_file,omitempty"`
	SynctexFile string       `json:"synctex_file,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	RawLog      string       `json:"raw_log,omitempty"`
	DurationMs  int64        `json:"duration_ms"`
}
