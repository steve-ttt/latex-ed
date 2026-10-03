package compiler

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Engine defines the compilation interface.
type Engine interface {
	Compile(ctx context.Context, req CompileRequest) (*CompileResult, error)
}

// PdflatexEngine implements Engine using the system pdflatex binary.
type PdflatexEngine struct {
	BinaryPath   string
	CacheDirName string
}

// NewPdflatexEngine initializes a new PdflatexEngine.
func NewPdflatexEngine() *PdflatexEngine {
	return &PdflatexEngine{
		BinaryPath:   "pdflatex",
		CacheDirName: ".latex-cache",
	}
}

// Compile compiles a LaTeX document using pdflatex.
func (e *PdflatexEngine) Compile(ctx context.Context, req CompileRequest) (*CompileResult, error) {
	if req.RootDir == "" {
		return nil, fmt.Errorf("root directory cannot be empty")
	}
	if req.MainFile == "" {
		return nil, fmt.Errorf("main file cannot be empty")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cacheDirName := e.CacheDirName
	if cacheDirName == "" {
		cacheDirName = ".latex-cache"
	}
	cacheDir := filepath.Join(req.RootDir, cacheDirName)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	binary := e.BinaryPath
	if binary == "" {
		binary = "pdflatex"
	}

	args := []string{
		"-interaction=nonstopmode",
		"-file-line-error",
		"-synctex=1",
		"-output-directory=" + cacheDir,
		req.MainFile,
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = req.RootDir

	startTime := time.Now()
	output, execErr := cmd.CombinedOutput()
	durationMs := time.Since(startTime).Milliseconds()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Read log file from cache if present for full details
	baseName := strings.TrimSuffix(filepath.Base(req.MainFile), filepath.Ext(req.MainFile))
	logFile := filepath.Join(cacheDir, baseName+".log")
	rawLog := string(output)
	if logBytes, err := os.ReadFile(logFile); err == nil && len(logBytes) > 0 {
		rawLog = string(logBytes)
	}

	diagnostics := ParseLatexLog(rawLog)

	// Check if PDF exists and has non-zero size
	pdfPath := filepath.Join(cacheDir, baseName+".pdf")
	pdfExists := false
	if fi, err := os.Stat(pdfPath); err == nil && fi.Size() > 0 {
		pdfExists = true
	}

	// ExitError is normal when LaTeX encounters syntax errors
	success := (execErr == nil) && pdfExists
	if execErr != nil {
		if _, ok := execErr.(*exec.ExitError); !ok {
			return nil, fmt.Errorf("pdflatex execution error: %w", execErr)
		}
	}

	res := &CompileResult{
		Success:     success,
		Diagnostics: diagnostics,
		RawLog:      rawLog,
		DurationMs:  durationMs,
	}

	if pdfExists {
		res.PdfFile = pdfPath

		// On successful creation of the PDF, copy the PDF back to the same directory
		// as the source .tex file while keeping artifacts in .latex-cache.
		if success {
			targetDir := filepath.Join(req.RootDir, filepath.Dir(req.MainFile))
			targetPdf := filepath.Join(targetDir, baseName+".pdf")
			if err := copyFile(pdfPath, targetPdf); err == nil {
				res.PdfFile = targetPdf
			}
		}
	}

	synctexGz := filepath.Join(cacheDir, baseName+".synctex.gz")
	if _, err := os.Stat(synctexGz); err == nil {
		res.SynctexFile = synctexGz
	} else {
		synctexPlain := filepath.Join(cacheDir, baseName+".synctex")
		if _, err := os.Stat(synctexPlain); err == nil {
			res.SynctexFile = synctexPlain
		}
	}

	return res, nil
}

func copyFile(src, dst string) error {
	srcClean := filepath.Clean(src)
	dstClean := filepath.Clean(dst)
	if srcClean == dstClean {
		return nil
	}

	in, err := os.Open(srcClean)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dstClean), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dstClean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
