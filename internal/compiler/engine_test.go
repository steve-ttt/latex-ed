package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func requirePdflatex(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("pdflatex"); err != nil {
		t.Skip("pdflatex binary not found on PATH, skipping integration test")
	}
}

func TestPdflatexEngine_CompileValidDocument(t *testing.T) {
	requirePdflatex(t)

	tmpDir := t.TempDir()
	texContent := `\documentclass{article}
\begin{document}
Hello from TDD LaTeX Editor!
\end{document}
`
	mainFile := filepath.Join(tmpDir, "main.tex")
	if err := os.WriteFile(mainFile, []byte(texContent), 0644); err != nil {
		t.Fatalf("failed to write tex file: %v", err)
	}

	engine := NewPdflatexEngine()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := engine.Compile(ctx, CompileRequest{
		RootDir:  tmpDir,
		MainFile: "main.tex",
	})

	if err != nil {
		t.Fatalf("Compile returned unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected compilation success, got failure. Log: %s", result.RawLog)
	}

	if result.PdfFile == "" {
		t.Fatalf("expected non-empty PdfFile path")
	}

	pdfData, err := os.ReadFile(result.PdfFile)
	if err != nil {
		t.Fatalf("failed to read generated PDF file: %v", err)
	}

	if len(pdfData) < 4 || string(pdfData[:4]) != "%PDF" {
		t.Fatalf("generated file is not a valid PDF header: %q", string(pdfData[:10]))
	}

	// Verify build artifact is in the cache directory, not polluting root
	expectedCache := filepath.Join(tmpDir, ".latex-cache")
	if filepath.Dir(result.PdfFile) != expectedCache {
		t.Errorf("expected PDF in %s, got %s", expectedCache, filepath.Dir(result.PdfFile))
	}

	if result.SynctexFile == "" {
		t.Errorf("expected non-empty SynctexFile path")
	} else if _, err := os.Stat(result.SynctexFile); err != nil {
		t.Errorf("synctex file does not exist on disk: %v", err)
	}
}

func TestPdflatexEngine_CompileSyntaxError(t *testing.T) {
	requirePdflatex(t)

	tmpDir := t.TempDir()
	texContent := `\documentclass{article}
\begin{document}
\undefinedmacroxyz{boom}
\end{document}
`
	mainFile := filepath.Join(tmpDir, "broken.tex")
	if err := os.WriteFile(mainFile, []byte(texContent), 0644); err != nil {
		t.Fatalf("failed to write broken tex file: %v", err)
	}

	engine := NewPdflatexEngine()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := engine.Compile(ctx, CompileRequest{
		RootDir:  tmpDir,
		MainFile: "broken.tex",
	})

	if err != nil {
		t.Fatalf("Compile returned unexpected Go error: %v", err)
	}

	if result.Success {
		t.Fatalf("expected compilation failure for broken document, but reported success")
	}

	hasErrorDiag := false
	for _, d := range result.Diagnostics {
		if d.Severity == SeverityError {
			hasErrorDiag = true
			break
		}
	}

	if !hasErrorDiag {
		t.Fatalf("expected at least one error diagnostic, got: %+v", result.Diagnostics)
	}
}

func TestPdflatexEngine_ContextCancellation(t *testing.T) {
	requirePdflatex(t)

	tmpDir := t.TempDir()
	texContent := `\documentclass{article}
\begin{document}
Test cancellation
\end{document}
`
	mainFile := filepath.Join(tmpDir, "main.tex")
	if err := os.WriteFile(mainFile, []byte(texContent), 0644); err != nil {
		t.Fatalf("failed to write tex file: %v", err)
	}

	engine := NewPdflatexEngine()
	// Pre-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := engine.Compile(ctx, CompileRequest{
		RootDir:  tmpDir,
		MainFile: "main.tex",
	})

	if err == nil {
		t.Fatalf("expected error on pre-cancelled context, got nil")
	}
}

func TestPdflatexEngine_CompileMultiFileDocument(t *testing.T) {
	requirePdflatex(t)

	tmpDir := t.TempDir()
	mainTex := `\documentclass{article}
\begin{document}
Root content.
\input{chapters/chapter1.tex}
\end{document}`

	subTex := `\section{Chapter One}
Multi-file sub-content successfully loaded.`

	if err := os.WriteFile(filepath.Join(tmpDir, "main.tex"), []byte(mainTex), 0644); err != nil {
		t.Fatalf("failed to write main.tex: %v", err)
	}

	subDir := filepath.Join(tmpDir, "chapters")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create sub dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "chapter1.tex"), []byte(subTex), 0644); err != nil {
		t.Fatalf("failed to write chapter1.tex: %v", err)
	}

	engine := NewPdflatexEngine()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := engine.Compile(ctx, CompileRequest{
		RootDir:  tmpDir,
		MainFile: "main.tex",
	})

	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected multi-file compile success, got failure. Log: %s", result.RawLog)
	}
	if result.PdfFile == "" {
		t.Fatalf("expected non-empty PdfFile")
	}
}
