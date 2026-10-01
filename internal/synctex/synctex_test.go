package synctex

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"latex-editor/internal/compiler"
)

func requireSyncTex(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("synctex"); err != nil {
		t.Skip("synctex binary not found on PATH, skipping test")
	}
	if _, err := exec.LookPath("pdflatex"); err != nil {
		t.Skip("pdflatex binary not found on PATH, skipping test")
	}
}

func TestSyncTex_ForwardAndInverseSearch(t *testing.T) {
	requireSyncTex(t)

	tmpDir := t.TempDir()
	texContent := `\documentclass{article}
\begin{document}
First line of document.

SyncTeX Integration Target line to locate.

Third paragraph of document.
\end{document}
`
	mainFile := filepath.Join(tmpDir, "main.tex")
	if err := os.WriteFile(mainFile, []byte(texContent), 0644); err != nil {
		t.Fatalf("failed to write tex: %v", err)
	}

	// 1. Compile with pdflatex
	engine := compiler.NewPdflatexEngine()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := engine.Compile(ctx, compiler.CompileRequest{
		RootDir:  tmpDir,
		MainFile: "main.tex",
	})
	if err != nil || !result.Success {
		t.Fatalf("compilation failed: %v", err)
	}

	syn := New()

	// 2. Forward search: Line 5 ("SyncTeX Integration Target line to locate.")
	fwd, err := syn.ForwardSearch(ctx, ForwardQuery{
		File:    mainFile,
		Line:    5,
		Column:  1,
		PdfFile: result.PdfFile,
		Dir:     filepath.Dir(result.PdfFile),
	})

	if err != nil {
		t.Fatalf("ForwardSearch failed: %v", err)
	}
	if fwd.Page != 1 {
		t.Errorf("expected Page 1, got %d", fwd.Page)
	}
	if fwd.Y <= 0 {
		t.Errorf("expected positive Y coordinate, got %f", fwd.Y)
	}

	// 3. Inverse search using the coordinates from forward search
	inv, err := syn.InverseSearch(ctx, InverseQuery{
		Page:    fwd.Page,
		X:       fwd.X,
		Y:       fwd.Y,
		PdfFile: result.PdfFile,
		Dir:     filepath.Dir(result.PdfFile),
	})

	if err != nil {
		t.Fatalf("InverseSearch failed: %v", err)
	}
	if filepath.Base(inv.File) != "main.tex" {
		t.Errorf("expected file main.tex, got %q", inv.File)
	}
	if inv.Line < 3 || inv.Line > 7 {
		t.Errorf("expected line near 5, got %d", inv.Line)
	}
}
