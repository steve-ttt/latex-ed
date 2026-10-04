package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinHowToTemplate_Valid(t *testing.T) {
	if len(BuiltinHowToTemplate) == 0 {
		t.Fatal("expected BuiltinHowToTemplate to be non-empty")
	}
	for _, expected := range []string{
		"\\documentclass",
		"\\begin{document}",
		"\\end{document}",
		"amsmath",
		"listings",
		"Quick-Start Guide",
	} {
		if !strings.Contains(BuiltinHowToTemplate, expected) {
			t.Errorf("expected BuiltinHowToTemplate to contain %q", expected)
		}
	}
}

func TestResolver_FallbackToBuiltin(t *testing.T) {
	emptyDir := t.TempDir()
	r := NewResolver(emptyDir)

	content := r.ResolveTemplate("chapter1.tex")
	if content != BuiltinHowToTemplate {
		t.Errorf("expected built-in template, got:\n%s", content)
	}

	// Empty filename should also default to LaTeX template
	contentEmpty := r.ResolveTemplate("")
	if contentEmpty != BuiltinHowToTemplate {
		t.Errorf("expected built-in template for empty filename, got:\n%s", contentEmpty)
	}
}

func TestResolver_UserDefaultTexFound(t *testing.T) {
	customDir := t.TempDir()
	customTemplate := "\\documentclass{article}\n% Custom User Template\n\\begin{document}\nHello World\n\\end{document}\n"
	err := os.WriteFile(filepath.Join(customDir, "default.tex"), []byte(customTemplate), 0644)
	if err != nil {
		t.Fatalf("failed to write custom default.tex: %v", err)
	}

	r := NewResolver(customDir)
	content := r.ResolveTemplate("my_new_doc.tex")
	if content != customTemplate {
		t.Errorf("expected custom template, got:\n%s", content)
	}
}

func TestResolver_UserCapitalizedDefaultTexFound(t *testing.T) {
	customDir := t.TempDir()
	customTemplate := "\\documentclass{book}\n% Capitalized Default\n\\begin{document}\n\\end{document}\n"
	err := os.WriteFile(filepath.Join(customDir, "Default.tex"), []byte(customTemplate), 0644)
	if err != nil {
		t.Fatalf("failed to write custom Default.tex: %v", err)
	}

	r := NewResolver(customDir)
	content := r.ResolveTemplate("book.tex")
	if content != customTemplate {
		t.Errorf("expected capitalized Default.tex content, got:\n%s", content)
	}
}

func TestResolver_EmptyDefaultTexFallsBack(t *testing.T) {
	customDir := t.TempDir()
	err := os.WriteFile(filepath.Join(customDir, "default.tex"), []byte("   \n\t  "), 0644)
	if err != nil {
		t.Fatalf("failed to write empty default.tex: %v", err)
	}

	r := NewResolver(customDir)
	content := r.ResolveTemplate("doc.tex")
	if content != BuiltinHowToTemplate {
		t.Errorf("expected fallback to built-in when default.tex is whitespace-only, got:\n%s", content)
	}
}

func TestResolver_NonTexFiles(t *testing.T) {
	customDir := t.TempDir()
	err := os.WriteFile(filepath.Join(customDir, "default.tex"), []byte("LaTeX template"), 0644)
	if err != nil {
		t.Fatalf("failed to write default.tex: %v", err)
	}

	r := NewResolver(customDir)
	for _, nonTex := range []string{"references.bib", "notes.txt", "script.py", "Makefile", "image.png"} {
		content := r.ResolveTemplate(nonTex)
		if content != "" {
			t.Errorf("expected empty string for non-tex file %q, got %q", nonTex, content)
		}
	}
}

func TestParseXDGUserDirs(t *testing.T) {
	tempDir := t.TempDir()
	mockHome := filepath.Join(tempDir, "mockhome")
	_ = os.MkdirAll(mockHome, 0755)

	configPath := filepath.Join(tempDir, "user-dirs.dirs")
	content := `# User directories
XDG_DESKTOP_DIR="$HOME/Desktop"
XDG_TEMPLATES_DIR="$HOME/MyTemplates"
XDG_DOCUMENTS_DIR="${HOME}/Documents"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write mock user-dirs.dirs: %v", err)
	}

	res := parseXDGUserDirs(configPath, mockHome)
	expected := filepath.Join(mockHome, "MyTemplates")
	if res != expected {
		t.Errorf("expected %q, got %q", expected, res)
	}

	// Missing file should return empty string
	if missing := parseXDGUserDirs("/nonexistent/path/user-dirs.dirs", mockHome); missing != "" {
		t.Errorf("expected empty string for missing config, got %q", missing)
	}
}

func TestCandidateTemplateDirs(t *testing.T) {
	dirs := CandidateTemplateDirs()
	// Should at least return a non-empty list of candidate directories
	if len(dirs) == 0 {
		t.Fatal("expected at least one candidate template directory")
	}
	for _, d := range dirs {
		if d == "" || d == "." {
			t.Errorf("invalid candidate directory: %q", d)
		}
	}
}
