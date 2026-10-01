package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathTraversal is returned when a path attempts to escape the workspace root.
var ErrPathTraversal = errors.New("path traversal detected")

// FileInfo contains metadata for a project file.
type FileInfo struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
}

// Workspace manages file operations within a bounded directory.
type Workspace struct {
	rootDir string
}

// New creates and validates a Workspace at rootDir.
func New(rootDir string) (*Workspace, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace path: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("workspace directory error: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace path is not a directory: %s", absRoot)
	}

	return &Workspace{rootDir: absRoot}, nil
}

// RootDir returns the absolute root directory path.
func (w *Workspace) RootDir() string {
	return w.rootDir
}

// SafePath checks if relPath is safely within the workspace root, returning the absolute path.
func (w *Workspace) SafePath(relPath string) (string, error) {
	if strings.ContainsRune(relPath, 0) {
		return "", ErrPathTraversal
	}

	// Clean path
	cleaned := filepath.Clean(relPath)

	// Disallow absolute paths
	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "\\") {
		return "", ErrPathTraversal
	}

	// Disallow explicit parent references
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "/../") || strings.Contains(cleaned, "\\..\\") {
		return "", ErrPathTraversal
	}

	fullPath := filepath.Join(w.rootDir, cleaned)

	// Verify relative path from rootDir does not escape
	rel, err := filepath.Rel(w.rootDir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", ErrPathTraversal
	}

	return fullPath, nil
}

// ReadFile reads the content of a file within the workspace.
func (w *Workspace) ReadFile(relPath string) ([]byte, error) {
	target, err := w.SafePath(relPath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(target)
}

// WriteFile writes data to a file within the workspace, creating any missing parent directories.
func (w *Workspace) WriteFile(relPath string, data []byte) error {
	target, err := w.SafePath(relPath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(target, data, 0644)
}

// DeleteFile removes a file or directory within the workspace.
func (w *Workspace) DeleteFile(relPath string) error {
	target, err := w.SafePath(relPath)
	if err != nil {
		return err
	}
	return os.RemoveAll(target)
}

// ListFiles recursively returns all files in the workspace, ignoring caches and VCS folders.
func (w *Workspace) ListFiles() ([]FileInfo, error) {
	var results []FileInfo

	err := filepath.WalkDir(w.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := d.Name()
		if d.IsDir() {
			// Skip internal cache, hidden folders, version control, virtualenvs, and python caches
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "venv" || name == "env" || name == "__pycache__" {
				return filepath.SkipDir
			}
		} else {
			// Skip hidden files, backup files, and TeX/Python intermediate build artifacts
			if strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") || strings.HasSuffix(name, ".pyc") {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".aux" || ext == ".log" || ext == ".out" || ext == ".toc" || ext == ".fls" || ext == ".fdb_latexmk" || ext == ".synctex" || strings.HasSuffix(name, ".synctex.gz") {
				return nil
			}
		}

		rel, err := filepath.Rel(w.rootDir, path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		results = append(results, FileInfo{
			Path:    rel,
			Name:    name,
			IsDir:   d.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().UnixMilli(),
		})

		return nil
	})

	return results, err
}
