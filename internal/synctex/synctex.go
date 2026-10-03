package synctex

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// SyncTex provides forward and inverse search using the system `synctex` CLI.
type SyncTex struct {
	BinaryPath string
}

// New initializes a SyncTex runner.
func New() *SyncTex {
	return &SyncTex{
		BinaryPath: "synctex",
	}
}

// ForwardSearch maps source (file, line, column) -> PDF (page, x, y coordinates).
func (s *SyncTex) ForwardSearch(ctx context.Context, q ForwardQuery) (*ForwardResult, error) {
	binary := s.BinaryPath
	if binary == "" {
		binary = "synctex"
	}

	col := q.Column
	if col <= 0 {
		col = 1
	}

	inputSpec := fmt.Sprintf("%d:%d:%s", q.Line, col, q.File)

	synDir := resolveSyncTexDir(q.Dir, q.PdfFile)

	args := []string{"view", "-i", inputSpec, "-o", q.PdfFile}
	if synDir != "" {
		args = append(args, "-d", synDir)
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	if synDir != "" {
		cmd.Dir = synDir
	} else {
		cmd.Dir = filepath.Dir(q.PdfFile)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("synctex view failed (%w): %s", err, string(out))
	}

	res := &ForwardResult{}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	found := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "Page":
			if p, err := strconv.Atoi(val); err == nil {
				res.Page = p
				found = true
			}
		case "x":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				res.X = f
			}
		case "y":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				res.Y = f
			}
		case "h":
			if res.X == 0 {
				if f, err := strconv.ParseFloat(val, 64); err == nil {
					res.X = f
				}
			}
		case "v":
			if res.Y == 0 {
				if f, err := strconv.ParseFloat(val, 64); err == nil {
					res.Y = f
				}
			}
		case "W":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				res.Width = f
			}
		case "H":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				res.Height = f
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("no synctex record found in output: %s", string(out))
	}

	return res, nil
}

// InverseSearch maps PDF (page, x, y) -> source (file, line, column).
func (s *SyncTex) InverseSearch(ctx context.Context, q InverseQuery) (*InverseResult, error) {
	binary := s.BinaryPath
	if binary == "" {
		binary = "synctex"
	}

	outputSpec := fmt.Sprintf("%d:%f:%f:%s", q.Page, q.X, q.Y, q.PdfFile)

	synDir := resolveSyncTexDir(q.Dir, q.PdfFile)

	args := []string{"edit", "-o", outputSpec}
	if synDir != "" {
		args = append(args, "-d", synDir)
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	if synDir != "" {
		cmd.Dir = synDir
	} else {
		cmd.Dir = filepath.Dir(q.PdfFile)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("synctex edit failed (%w): %s", err, string(out))
	}

	res := &InverseResult{}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	found := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "Line":
			if l, err := strconv.Atoi(val); err == nil {
				res.Line = l
				found = true
			}
		case "Column":
			if c, err := strconv.Atoi(val); err == nil {
				res.Column = c
			}
		case "Input":
			res.File = filepath.Clean(val)
		}
	}

	if !found {
		return nil, fmt.Errorf("no inverse synctex record found in output: %s", string(out))
	}

	return res, nil
}

func resolveSyncTexDir(dir, pdfFile string) string {
	pdfDir := filepath.Dir(pdfFile)
	base := strings.TrimSuffix(filepath.Base(pdfFile), filepath.Ext(pdfFile))

	// If explicit dir given, check if synctex file exists there
	if dir != "" {
		if hasSyncTexArtifact(dir, base) {
			return dir
		}
		// Also check dir/.latex-cache
		cacheSub := filepath.Join(dir, ".latex-cache")
		if hasSyncTexArtifact(cacheSub, base) {
			return cacheSub
		}
	}

	// Check pdf directory
	if hasSyncTexArtifact(pdfDir, base) {
		return pdfDir
	}

	// Check pdfDir/.latex-cache
	cacheInPdfDir := filepath.Join(pdfDir, ".latex-cache")
	if hasSyncTexArtifact(cacheInPdfDir, base) {
		return cacheInPdfDir
	}

	if dir != "" {
		return dir
	}
	return pdfDir
}

func hasSyncTexArtifact(dir, baseName string) bool {
	if fi, err := os.Stat(filepath.Join(dir, baseName+".synctex.gz")); err == nil && !fi.IsDir() {
		return true
	}
	if fi, err := os.Stat(filepath.Join(dir, baseName+".synctex")); err == nil && !fi.IsDir() {
		return true
	}
	return false
}
