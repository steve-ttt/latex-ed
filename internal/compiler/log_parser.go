package compiler

import (
	"bufio"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	// fileLineRegex matches "-file-line-error" formatted messages:
	// ./main.tex:12: Undefined control sequence.
	fileLineRegex = regexp.MustCompile(`^(?:\./)?([^:\n\r]+):(\d+):\s*(.+)$`)

	// classicErrorRegex matches "! LaTeX Error: ..." or "! Undefined control sequence."
	classicErrorRegex = regexp.MustCompile(`^!\s*(.+)$`)

	// lineIndicatorRegex matches "l.12 \badcommand"
	lineIndicatorRegex = regexp.MustCompile(`^l\.(\d+)\s*(.*)$`)

	// warningLineRegex matches "LaTeX Warning: ... on input line 42."
	warningLineRegex = regexp.MustCompile(`^(?:LaTeX|Package\s+[\w.\-]+)\s+Warning:\s*(.+?)(?: on input line (\d+))?\.?$`)

	// overfullRegex matches "Overfull \hbox (...) in paragraph at lines 60--65"
	overfullRegex = regexp.MustCompile(`^(?:Overfull|Underfull)\s+\\[hv]box\s+.*?(?:at lines?\s+(\d+)(?:--\d+)?)?`)

	// packageContinuationRegex matches "(hyperref) removing '\textbf' on input line 78."
	packageContinuationRegex = regexp.MustCompile(`^\(([a-zA-Z0-9_.-]+)\)\s*(.+)$`)
	inputLineRegex           = regexp.MustCompile(`on input line (\d+)`)
)

// ParseLatexLog parses raw stdout/stderr/log from a LaTeX run into structured Diagnostics.
func ParseLatexLog(rawLog string) []Diagnostic {
	var diagnostics []Diagnostic

	scanner := bufio.NewScanner(strings.NewReader(rawLog))

	// Track file stack to approximate current file context if not in error prefix
	var fileStack []string
	currentFile := ""

	var pending *Diagnostic

	flushPending := func() {
		if pending != nil {
			if pending.File == "" && currentFile != "" {
				pending.File = currentFile
			}
			diagnostics = append(diagnostics, *pending)
			pending = nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check for file-line error: ./main.tex:12: Undefined control sequence.
		if m := fileLineRegex.FindStringSubmatch(trimmed); m != nil {
			flushPending()
			lineNum, _ := strconv.Atoi(m[2])
			msg := strings.TrimPrefix(m[3], "LaTeX Error: ")
			pending = &Diagnostic{
				Severity: SeverityError,
				File:     filepath.Clean(m[1]),
				Line:     lineNum,
				Message:  msg,
			}
			continue
		}

		// Check for classic TeX error starting with "! "
		if m := classicErrorRegex.FindStringSubmatch(trimmed); m != nil {
			flushPending()
			msg := strings.TrimPrefix(m[1], "LaTeX Error: ")
			pending = &Diagnostic{
				Severity: SeverityError,
				File:     currentFile,
				Message:  msg,
			}
			continue
		}

		// Check for line indicator "l.12 \badcommand"
		if m := lineIndicatorRegex.FindStringSubmatch(trimmed); m != nil {
			if pending != nil {
				lineNum, _ := strconv.Atoi(m[1])
				if pending.Line == 0 {
					pending.Line = lineNum
				}
				if pending.Context == "" {
					pending.Context = m[2]
				}
			}
			continue
		}

		// If pending diagnostic, additional context lines might follow until empty line or new section
		if pending != nil && pending.Context == "" && trimmed != "" && !strings.HasPrefix(trimmed, "Type ") && !strings.HasPrefix(trimmed, "See the ") {
			// accumulate context or description
		}

		// Check for warnings
		if m := warningLineRegex.FindStringSubmatch(trimmed); m != nil {
			flushPending()
			lineNum := 0
			if len(m) > 2 && m[2] != "" {
				lineNum, _ = strconv.Atoi(m[2])
			}
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityWarning,
				File:     currentFile,
				Line:     lineNum,
				Message:  m[1],
			})
			continue
		}

		// Check for overfull/underfull box warnings
		if m := overfullRegex.FindStringSubmatch(trimmed); m != nil {
			flushPending()
			lineNum := 0
			if len(m) > 1 && m[1] != "" {
				lineNum, _ = strconv.Atoi(m[1])
			}
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityWarning,
				File:     currentFile,
				Line:     lineNum,
				Message:  trimmed,
			})
			continue
		}

		// Check for package warning continuation lines: (hyperref) ... on input line 78.
		if len(diagnostics) > 0 && diagnostics[len(diagnostics)-1].Severity == SeverityWarning && diagnostics[len(diagnostics)-1].Line == 0 {
			if m := packageContinuationRegex.FindStringSubmatch(trimmed); m != nil {
				if lineM := inputLineRegex.FindStringSubmatch(m[2]); lineM != nil {
					lineNum, _ := strconv.Atoi(lineM[1])
					diagnostics[len(diagnostics)-1].Line = lineNum
					diagnostics[len(diagnostics)-1].Message += " " + m[2]
				}
				continue
			}
		}

		// Simple heuristic for tracking source files: (./main.tex or (/usr/...
		parseFileTransitions(line, &fileStack, &currentFile)
	}

	flushPending()

	return diagnostics
}

func parseFileTransitions(line string, stack *[]string, current *string) {
	for i := 0; i < len(line); i++ {
		if line[i] == '(' {
			// Find token after '('
			rest := strings.TrimSpace(line[i+1:])
			if len(rest) > 0 {
				fields := strings.Fields(rest)
				if len(fields) > 0 {
					candidate := fields[0]
					if strings.HasSuffix(candidate, ".tex") || strings.HasSuffix(candidate, ".sty") || strings.HasSuffix(candidate, ".cls") {
						cleaned := filepath.Clean(candidate)
						*stack = append(*stack, cleaned)
						*current = cleaned
					}
				}
			}
		} else if line[i] == ')' {
			if len(*stack) > 0 {
				*stack = (*stack)[:len(*stack)-1]
				if len(*stack) > 0 {
					*current = (*stack)[len(*stack)-1]
				} else {
					*current = ""
				}
			}
		}
	}
}
