package templates

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// BuiltinHowToTemplate is the default 2-page introductory LaTeX document
// used when no custom default.tex is found in user template directories.
const BuiltinHowToTemplate = `\documentclass[11pt,a4paper]{article}
\usepackage[utf8]{inputenc}
\usepackage[margin=1in]{geometry}
\usepackage{amsmath, amssymb}
\usepackage{graphicx}
\usepackage{xcolor}
\usepackage{listings}
\usepackage{hyperref}

\hypersetup{
    colorlinks=true,
    linkcolor=blue,
    urlcolor=blue
}

\lstset{
    basicstyle=\ttfamily\small,
    backgroundcolor=\color{gray!10},
    frame=single,
    breaklines=true
}

\title{\textbf{Getting Started with \LaTeX{}: A Quick-Start Guide}}
\author{LaTeX-Ed Workbench}
\date{\today}

\begin{document}

\maketitle

\begin{abstract}
Welcome to \LaTeX{}! This introductory template provides a concise reference for common syntax, mathematical equations, formatting styles, and code blocks. You can edit this file directly or replace it with your own writing.
\end{abstract}

\section{Document Structure}
Every \LaTeX{} document consists of two main parts:
\begin{enumerate}
    \item \textbf{The Preamble:} Everything before \verb|\begin{document}|. Here you configure document classes (e.g., \verb|article|, \verb|report|, \verb|book|) and import packages using \verb|\usepackage{...}|.
    \item \textbf{The Body:} Everything enclosed between \verb|\begin{document}| and \verb|\end{document}|.
\end{enumerate}

Create new paragraphs by leaving a blank line in the source. Lines starting with a percent sign (\verb|%|) are comments and will not appear in the compiled PDF.

\section{Text Styling and Environments}
Standard text formatting commands include:
\begin{itemize}
    \item \textbf{Bold}: \verb|\textbf{bold text}|
    \item \textit{Italic}: \verb|\textit{italic text}|
    \item \underline{Underline}: \verb|\underline{underlined text}|
    \item \texttt{Monospace / Typewriter}: \verb|\texttt{code or path}|
\end{itemize}

\subsection{Bullet and Numbered Lists}
Unordered lists use the \verb|itemize| environment, while ordered lists use \verb|enumerate|:
\begin{itemize}
    \item First bullet point item
    \item Second bullet point item with nested points
\end{itemize}

\section{Mathematical Typesetting}
One of the greatest strengths of \LaTeX{} is typesetting mathematics.
\begin{itemize}
    \item \textbf{Inline math} is enclosed in single dollar signs: e.g., $E = mc^2$ or $\lim_{x \to 0} \frac{\sin x}{x} = 1$.
    \item \textbf{Display math} or numbered equations use the \verb|equation| environment:
\end{itemize}

\begin{equation}
    \int_{-\infty}^{\infty} e^{-x^2} \, dx = \sqrt{\pi}
\end{equation}

Matrices and aligned multiline equations can be written using \verb|amsmath| environments like \verb|align*|:
\begin{align*}
    (x + y)^2 &= (x + y)(x + y) \\
              &= x^2 + 2xy + y^2
\end{align*}

\section{Source Code Listings}
You can present syntax-highlighted source code using the \verb|listings| package:

\begin{lstlisting}[language=Python]
def fibonacci(n):
    a, b = 0, 1
    for _ in range(n):
        yield a
        a, b = b, a + b
\end{lstlisting}

\section{Tips and Keyboard Shortcuts}
\begin{itemize}
    \item \textbf{Save \& Compile:} Press \texttt{Ctrl+S} (or \texttt{Cmd+S}) to immediately save and recompile.
    \item \textbf{Insert Palette:} Press \texttt{Alt+I} to open the snippet and symbol palette.
    \item \textbf{SyncTeX Navigation:} Press \texttt{Ctrl+J} to jump from the cursor line directly to the corresponding location in the PDF preview.
    \item \textbf{Custom Templates:} Place a \texttt{default.tex} file in your \texttt{\textasciitilde/Templates} folder to automatically use your own template whenever you create a new document!
\end{itemize}

\end{document}
`

// Resolver resolves template contents for newly created documents.
type Resolver struct {
	candidateDirs []string
}

// NewResolver returns a Resolver searching the given candidate directories.
// If no directories are provided, CandidateTemplateDirs() for the current OS is used.
func NewResolver(candidateDirs ...string) *Resolver {
	if len(candidateDirs) == 0 {
		return &Resolver{candidateDirs: CandidateTemplateDirs()}
	}
	return &Resolver{candidateDirs: candidateDirs}
}

// CandidateDirs returns the search directories configured for this resolver.
func (r *Resolver) CandidateDirs() []string {
	return r.candidateDirs
}

// ResolveTemplate resolves the template content for the given filename.
// For LaTeX files (.tex extension or empty filename), it searches for default.tex in candidate template directories.
// If found and non-empty, it returns that content.
// Otherwise, it returns BuiltinHowToTemplate.
// For non-LaTeX files (e.g. .bib, .txt, Makefile), it returns an empty string.
func (r *Resolver) ResolveTemplate(filename string) string {
	if filename == "" {
		return r.resolveTexTemplate()
	}

	base := filepath.Base(filename)
	ext := strings.ToLower(filepath.Ext(base))
	if ext != ".tex" {
		return ""
	}

	return r.resolveTexTemplate()
}

func (r *Resolver) resolveTexTemplate() string {
	for _, dir := range r.candidateDirs {
		if dir == "" {
			continue
		}
		for _, name := range []string{"default.tex", "Default.tex"} {
			target := filepath.Join(dir, name)
			data, err := os.ReadFile(target)
			if err == nil && len(bytes.TrimSpace(data)) > 0 {
				return string(data)
			}
		}
	}

	return BuiltinHowToTemplate
}

// CandidateTemplateDirs returns prioritized template directories for the host OS.
func CandidateTemplateDirs() []string {
	var dirs []string
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			dirs = append(dirs, filepath.Join(appData, "Microsoft", "Windows", "Templates"))
		}
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			dirs = append(dirs, filepath.Join(userProfile, "Templates"))
		}
		if home != "" {
			dirs = append(dirs, filepath.Join(home, "Templates"))
		}

	case "darwin":
		if home != "" {
			dirs = append(dirs,
				filepath.Join(home, "Templates"),
				filepath.Join(home, "Library", "Application Support", "Templates"),
				filepath.Join(home, "Library", "Templates"),
			)
		}

	default: // linux, freebsd, openbsd, etc.
		// 1. Check explicit XDG_TEMPLATES_DIR env var
		if xdgTemplates := os.Getenv("XDG_TEMPLATES_DIR"); xdgTemplates != "" {
			dirs = append(dirs, xdgTemplates)
		}
		// 2. Check ~/.config/user-dirs.dirs
		if home != "" {
			if xdgDir := parseXDGUserDirs(filepath.Join(home, ".config", "user-dirs.dirs"), home); xdgDir != "" {
				dirs = append(dirs, xdgDir)
			}
			dirs = append(dirs,
				filepath.Join(home, "Templates"),
				filepath.Join(home, ".templates"),
			)
		}
	}

	return deduplicatePaths(dirs)
}

// parseXDGUserDirs parses a freedesktop user-dirs.dirs file to find XDG_TEMPLATES_DIR.
func parseXDGUserDirs(configPath, home string) string {
	f, err := os.Open(configPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || !strings.HasPrefix(line, "XDG_TEMPLATES_DIR=") {
			continue
		}
		val := strings.TrimPrefix(line, "XDG_TEMPLATES_DIR=")
		val = strings.Trim(val, `"'`)
		val = strings.ReplaceAll(val, "$HOME", home)
		val = strings.ReplaceAll(val, "${HOME}", home)
		return filepath.Clean(val)
	}
	return ""
}

func deduplicatePaths(paths []string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, p := range paths {
		cleaned := filepath.Clean(p)
		if cleaned == "" || cleaned == "." {
			continue
		}
		if _, exists := seen[cleaned]; !exists {
			seen[cleaned] = struct{}{}
			result = append(result, cleaned)
		}
	}
	return result
}
