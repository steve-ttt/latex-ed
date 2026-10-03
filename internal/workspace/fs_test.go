package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspace_SafePath_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	ws, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	validPaths := []string{
		"main.tex",
		"chapters/ch1.tex",
		"./figures/image.png",
	}

	for _, p := range validPaths {
		resolved, err := ws.SafePath(p)
		if err != nil {
			t.Errorf("expected %q to be valid, got error: %v", p, err)
		}
		rel, err := filepath.Rel(tmpDir, resolved)
		if err != nil || rel == ".." || filepath.IsAbs(rel) {
			t.Errorf("resolved path %q escaped tmpDir %q", resolved, tmpDir)
		}
	}
}

func TestWorkspace_SafePath_TraversalAttack(t *testing.T) {
	tmpDir := t.TempDir()
	ws, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	traversalPaths := []string{
		"../outside.txt",
		"../../etc/passwd",
		"/etc/passwd",
		"foo/../../outside.txt",
		"./../../secret",
	}

	for _, p := range traversalPaths {
		_, err := ws.SafePath(p)
		if err == nil {
			t.Errorf("expected traversal path %q to fail, but succeeded", p)
		}
	}
}

func TestWorkspace_ReadWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	ws, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	content := []byte("\\documentclass{article}\n\\begin{document}\nHello\n\\end{document}\n")
	err = ws.WriteFile("chapters/intro.tex", content)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	readBack, err := ws.ReadFile("chapters/intro.tex")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(readBack) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", string(readBack), string(content))
	}
}

func TestWorkspace_ListFiles_ExcludesCacheAndHidden(t *testing.T) {
	tmpDir := t.TempDir()
	ws, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	// Create test files
	_ = os.WriteFile(filepath.Join(tmpDir, "main.tex"), []byte("test"), 0644)
	_ = os.MkdirAll(filepath.Join(tmpDir, "chapters"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "chapters", "ch1.tex"), []byte("ch1"), 0644)

	// Create files that should be ignored
	_ = os.MkdirAll(filepath.Join(tmpDir, ".latex-cache"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, ".latex-cache", "main.aux"), []byte("aux"), 0644)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, ".git", "config"), []byte("git"), 0644)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".venv"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, ".venv", "pip.py"), []byte("pip"), 0644)
	_ = os.MkdirAll(filepath.Join(tmpDir, "__pycache__"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "__pycache__", "foo.cpython-311.pyc"), []byte("c"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "main.tex~"), []byte("backup"), 0644)

	files, err := ws.ListFiles()
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	paths := make(map[string]bool)
	for _, f := range files {
		paths[f.Path] = true
	}

	if !paths["main.tex"] {
		t.Errorf("expected main.tex to be listed")
	}
	if !paths["chapters/ch1.tex"] {
		t.Errorf("expected chapters/ch1.tex to be listed")
	}
	if paths[".latex-cache/main.aux"] || paths[".latex-cache"] {
		t.Errorf("expected .latex-cache to be excluded from file list")
	}
	if paths[".git/config"] || paths[".git"] {
		t.Errorf("expected .git to be excluded from file list")
	}
	if paths[".venv/pip.py"] || paths[".venv"] {
		t.Errorf("expected .venv to be excluded from file list")
	}
	if paths["__pycache__/foo.cpython-311.pyc"] || paths["main.tex~"] {
		t.Errorf("expected __pycache__ and backup files to be excluded")
	}
}

func TestWorkspace_DeleteFile(t *testing.T) {
	tmpDir := t.TempDir()
	ws, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	if err := ws.WriteFile("temp.tex", []byte("remove me")); err != nil {
		t.Fatalf("failed to write temp.tex: %v", err)
	}

	if err := ws.DeleteFile("temp.tex"); err != nil {
		t.Fatalf("failed to delete temp.tex: %v", err)
	}

	if _, err := ws.ReadFile("temp.tex"); err == nil {
		t.Errorf("expected error reading deleted file, got nil")
	}
}

func TestWorkspace_SafePath_SymlinkEscape(t *testing.T) {
	tmpDir := t.TempDir()
	outsideDir := t.TempDir()

	ws, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	secretFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("super secret data"), 0644); err != nil {
		t.Fatalf("failed to create secret file: %v", err)
	}

	// 1. Directory symlink pointing outside workspace
	symlinkDir := filepath.Join(tmpDir, "external_link")
	if err := os.Symlink(outsideDir, symlinkDir); err != nil {
		t.Fatalf("failed to create directory symlink: %v", err)
	}

	// SafePath on file within outside symlink must be blocked
	if _, err := ws.SafePath("external_link/secret.txt"); err == nil {
		t.Errorf("expected SafePath on outside symlink directory to fail with ErrPathTraversal, got nil")
	}

	// ReadFile through outside symlink must fail
	if _, err := ws.ReadFile("external_link/secret.txt"); err == nil {
		t.Errorf("expected ReadFile through outside symlink to fail, got nil")
	}

	// WriteFile through outside symlink must fail
	if err := ws.WriteFile("external_link/new_hacked.txt", []byte("bad")); err == nil {
		t.Errorf("expected WriteFile through outside symlink to fail, got nil")
	}

	// 2. Direct file symlink pointing outside workspace
	symlinkFile := filepath.Join(tmpDir, "symlink_secret.txt")
	if err := os.Symlink(secretFile, symlinkFile); err != nil {
		t.Fatalf("failed to create file symlink: %v", err)
	}

	if _, err := ws.SafePath("symlink_secret.txt"); err == nil {
		t.Errorf("expected SafePath on direct outside file symlink to fail, got nil")
	}
	if _, err := ws.ReadFile("symlink_secret.txt"); err == nil {
		t.Errorf("expected ReadFile on direct outside file symlink to fail, got nil")
	}
}

func TestWorkspace_CreateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	ws, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	// 1. Create simple directory
	if err := ws.CreateDirectory("module1"); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	info, err := os.Stat(filepath.Join(tmpDir, "module1"))
	if err != nil || !info.IsDir() {
		t.Fatalf("expected directory module1 to exist, got err: %v", err)
	}

	// 2. Create nested directory
	if err := ws.CreateDirectory("chapters/subchapters/deep"); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}
	info, err = os.Stat(filepath.Join(tmpDir, "chapters", "subchapters", "deep"))
	if err != nil || !info.IsDir() {
		t.Fatalf("expected nested directory to exist, got err: %v", err)
	}

	// 3. Creating directory should appear in ListFiles()
	files, err := ws.ListFiles()
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}
	foundModule := false
	for _, f := range files {
		if f.Path == "module1" && f.IsDir {
			foundModule = true
			break
		}
	}
	if !foundModule {
		t.Errorf("expected module1 directory in ListFiles, got: %+v", files)
	}

	// 4. Traversal rejection
	if err := ws.CreateDirectory("../escaped"); err == nil {
		t.Errorf("expected error creating directory with path traversal, got nil")
	}
	if err := ws.CreateDirectory("/tmp/evil"); err == nil {
		t.Errorf("expected error creating absolute directory, got nil")
	}
}

func TestWorkspace_CreateFileInSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	ws, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	texData := []byte("\\section{Subchapter}\nSubchapter content\n")
	// Create file in new subdirectory
	if err := ws.WriteFile("module4/notes.tex", texData); err != nil {
		t.Fatalf("failed to write file in subdirectory: %v", err)
	}

	// Check file on disk
	content, err := ws.ReadFile("module4/notes.tex")
	if err != nil {
		t.Fatalf("failed to read file in subdirectory: %v", err)
	}
	if string(content) != string(texData) {
		t.Errorf("content mismatch: got %q, want %q", string(content), string(texData))
	}

	// Verify both directory and file are in ListFiles
	files, err := ws.ListFiles()
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}
	foundDir := false
	foundFile := false
	for _, f := range files {
		if f.Path == "module4" && f.IsDir {
			foundDir = true
		}
		if f.Path == "module4/notes.tex" && !f.IsDir {
			foundFile = true
		}
	}
	if !foundDir {
		t.Errorf("expected module4 in file list")
	}
	if !foundFile {
		t.Errorf("expected module4/notes.tex in file list")
	}
}


