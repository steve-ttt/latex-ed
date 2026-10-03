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
	rootDir       string
	canonicalRoot string
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

	canonicalRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		canonicalRoot = absRoot
	}

	return &Workspace{rootDir: absRoot, canonicalRoot: canonicalRoot}, nil
}

// RootDir returns the absolute root directory path.
func (w *Workspace) RootDir() string {
	return w.rootDir
}

// CanonicalRootDir returns the resolved symlink-free root directory path.
func (w *Workspace) CanonicalRootDir() string {
	return w.canonicalRoot
}

// SafePath checks if relPath is safely within the workspace root, returning the absolute path.
// It guards against lexical traversal (..) and verifies symlink containment.
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

	// Verify relative path from rootDir does not escape lexically
	rel, err := filepath.Rel(w.rootDir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", ErrPathTraversal
	}

	// Canonical symlink containment verification
	if _, err := os.Lstat(fullPath); err == nil {
		// Target exists: resolve full canonical path
		canonicalTarget, err := filepath.EvalSymlinks(fullPath)
		if err != nil {
			return "", ErrPathTraversal
		}
		relCanonical, err := filepath.Rel(w.canonicalRoot, canonicalTarget)
		if err != nil || strings.HasPrefix(relCanonical, "..") || relCanonical == ".." {
			return "", ErrPathTraversal
		}
	} else if os.IsNotExist(err) {
		// Target does not exist yet: verify nearest existing parent directory remains in workspace
		parent := filepath.Dir(fullPath)
		for {
			if _, pErr := os.Stat(parent); pErr == nil {
				canonicalParent, cErr := filepath.EvalSymlinks(parent)
				if cErr != nil {
					return "", ErrPathTraversal
				}
				relCanonical, relErr := filepath.Rel(w.canonicalRoot, canonicalParent)
				if relErr != nil || strings.HasPrefix(relCanonical, "..") || relCanonical == ".." {
					return "", ErrPathTraversal
				}
				break
			}
			nextParent := filepath.Dir(parent)
			if nextParent == parent || nextParent == "." || nextParent == "/" {
				break
			}
			parent = nextParent
		}
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
// If the target is a symlink, it removes the link without traversing it.
func (w *Workspace) DeleteFile(relPath string) error {
	target, err := w.SafePath(relPath)
	if err != nil {
		return err
	}
	if target == w.rootDir || target == w.canonicalRoot {
		return ErrPathTraversal
	}

	fi, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		return os.Remove(target)
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
