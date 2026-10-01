package compiler

import (
	"testing"
)

func TestParseLatexLog_CleanLog(t *testing.T) {
	log := `This is pdfTeX, Version 3.141592653-2.6-1.40.24 (TeX Live 2022) (preloaded format=pdflatex)
 restricted \write18 enabled.
entering extended mode
(./main.tex
LaTeX2e <2022-11-01> patch level 1
(/usr/share/texmf-dist/tex/latex/base/article.cls)
No file main.aux.
[1{/var/lib/texmf/fonts/map/pdftex/updmap/pdftex.map}] (./main.aux) )
Output written on main.pdf (1 page, 12345 bytes).
Transcript written on main.log.`

	diagnostics := ParseLatexLog(log)
	if len(diagnostics) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %+v", len(diagnostics), diagnostics)
	}
}

func TestParseLatexLog_FileLineError_UndefinedControlSequence(t *testing.T) {
	log := `(./main.tex
./main.tex:12: Undefined control sequence.
l.12 \badcommand
                {test}
The control sequence at the end of the top line
of your error message was never \def'ed.`

	diagnostics := ParseLatexLog(log)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diagnostics), diagnostics)
	}

	d := diagnostics[0]
	if d.Severity != SeverityError {
		t.Errorf("expected SeverityError, got %v", d.Severity)
	}
	if d.File != "main.tex" && d.File != "./main.tex" {
		t.Errorf("expected File 'main.tex', got %q", d.File)
	}
	if d.Line != 12 {
		t.Errorf("expected Line 12, got %d", d.Line)
	}
	if d.Message != "Undefined control sequence." {
		t.Errorf("expected Message 'Undefined control sequence.', got %q", d.Message)
	}
	if d.Context == "" {
		t.Errorf("expected non-empty Context, got %q", d.Context)
	}
}

func TestParseLatexLog_EnvironmentMismatch(t *testing.T) {
	log := `(./document.tex
./document.tex:25: LaTeX Error: \begin{equation} on input line 20 ended by \end{document}.

See the LaTeX manual or LaTeX Companion for explanation.
Type  H <return>  for immediate help.
 ...                                              
                                                  
l.25 \end{document}
                   `

	diagnostics := ParseLatexLog(log)
	if len(diagnostics) == 0 {
		t.Fatalf("expected at least 1 diagnostic, got 0")
	}

	d := diagnostics[0]
	if d.Severity != SeverityError {
		t.Errorf("expected SeverityError, got %v", d.Severity)
	}
	if d.Line != 25 {
		t.Errorf("expected Line 25, got %d", d.Line)
	}
	expectedMsg := `\begin{equation} on input line 20 ended by \end{document}.`
	if d.Message != expectedMsg && d.Message != "LaTeX Error: "+expectedMsg {
		t.Errorf("unexpected message: %q", d.Message)
	}
}

func TestParseLatexLog_MissingFileOrPackage(t *testing.T) {
	log := `(./main.tex
! LaTeX Error: File 'nonexistent.sty' not found.

Type X to quit or <RETURN> to proceed,
or enter new name. (Default extension: sty)

Enter file name: 
l.4 \usepackage
               {nonexistent}`

	diagnostics := ParseLatexLog(log)
	if len(diagnostics) == 0 {
		t.Fatalf("expected at least 1 diagnostic, got 0")
	}

	d := diagnostics[0]
	if d.Severity != SeverityError {
		t.Errorf("expected SeverityError, got %v", d.Severity)
	}
	if d.Line != 4 {
		t.Errorf("expected Line 4, got %d", d.Line)
	}
}

func TestParseLatexLog_Warnings(t *testing.T) {
	log := `(./main.tex
LaTeX Warning: Reference 'fig:arch' on page 1 undefined on input line 42.
LaTeX Warning: Citation 'smith2020' on page 2 undefined on input line 55.
Overfull \hbox (15.2pt too wide) in paragraph at lines 60--65
)`

	diagnostics := ParseLatexLog(log)
	if len(diagnostics) < 2 {
		t.Fatalf("expected at least 2 warnings, got %d: %+v", len(diagnostics), diagnostics)
	}

	refWarn := diagnostics[0]
	if refWarn.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning, got %v", refWarn.Severity)
	}
	if refWarn.Line != 42 {
		t.Errorf("expected Line 42, got %d", refWarn.Line)
	}

	citeWarn := diagnostics[1]
	if citeWarn.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning, got %v", citeWarn.Severity)
	}
	if citeWarn.Line != 55 {
		t.Errorf("expected Line 55, got %d", citeWarn.Line)
	}
}

func TestParseLatexLog_EmptyAndPackageWarning(t *testing.T) {
	// Empty string
	if d := ParseLatexLog(""); len(d) != 0 {
		t.Errorf("expected 0 diagnostics for empty string, got %d", len(d))
	}

	// Package warning with input line
	log := `(./main.tex
Package hyperref Warning: Token not allowed in a PDF string (PDFDocEncoding):
(hyperref)                removing '\textbf' on input line 78.
)`
	d := ParseLatexLog(log)
	if len(d) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(d), d)
	}
	if d[0].Severity != SeverityWarning {
		t.Errorf("expected warning, got %v", d[0].Severity)
	}
	if d[0].Line != 78 {
		t.Errorf("expected line 78, got %d", d[0].Line)
	}
}
