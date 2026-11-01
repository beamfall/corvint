// Command py-signal-lint fails when a Python signal handler raises InterruptedError
// (docs/specs/corvint-self-development-v0.md, ASD-V0-574; V1-2178). InterruptedError is an OSError
// that the code around an interrupted call can catch and retry, so a handler that raises it can
// leave a qualification script running with its children; the repository's handlers raise
// SystemExit or KeyboardInterrupt and kill their process groups (V1-0610). For each tracked *.py
// file, a handler is a function named as the second argument of signal.signal(...), or a lambda
// written there. Exit 1 with one line per finding, 2 when the file list cannot be read.
//
// usage: go run ./tools/py-signal-lint
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

func main() {
	out, err := exec.Command("git", "ls-files", "-z", "--", "*.py").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "py-signal-lint: git ls-files: %v\n", err)
		os.Exit(2)
	}
	files := map[string][]byte{}
	for _, path := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "py-signal-lint: %v\n", err)
			os.Exit(2)
		}
		files[path] = data
	}
	os.Exit(run(files, os.Stdout, os.Stderr))
}

var (
	register     = regexp.MustCompile(`signal\.signal\(\s*[^,()]+(?:\([^()]*\))?\s*,\s*([A-Za-z_][A-Za-z0-9_.]*)\s*\)`)
	registerLamb = regexp.MustCompile(`signal\.signal\([^,]+,\s*lambda\b.*\bInterruptedError\b`)
	raiseIE      = regexp.MustCompile(`\braise\s+InterruptedError\b`)
	defLine      = regexp.MustCompile(`^(\s*)(?:async\s+)?def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
)

func run(files map[string][]byte, stdout, stderr io.Writer) int {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var findings []string
	for _, path := range paths {
		findings = append(findings, lint(path, files[path])...)
	}
	for _, f := range findings {
		fmt.Fprintln(stderr, f)
	}
	if len(findings) > 0 {
		return 1
	}
	fmt.Fprintf(stdout, "py-signal-lint: ok (%d Python files)\n", len(paths))
	return 0
}

func lint(path string, data []byte) []string {
	lines := strings.Split(string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))), "\n")
	handlers := map[string]bool{}
	var findings []string
	for i, line := range lines {
		code := stripComment(line)
		for _, m := range register.FindAllStringSubmatch(code, -1) {
			name := m[1][strings.LastIndex(m[1], ".")+1:]
			handlers[name] = true
		}
		if registerLamb.MatchString(code) {
			findings = append(findings, fmt.Sprintf("%s:%d: a signal handler lambda raises InterruptedError; raise SystemExit or KeyboardInterrupt instead (ASD-V0-574)", path, i+1))
		}
	}
	for i := 0; i < len(lines); i++ {
		m := defLine.FindStringSubmatch(lines[i])
		if m == nil || !handlers[m[2]] {
			continue
		}
		indent := len(m[1])
		for j := i + 1; j < len(lines); j++ {
			body := lines[j]
			if strings.TrimSpace(body) == "" {
				continue
			}
			if len(body)-len(strings.TrimLeft(body, " \t")) <= indent {
				break
			}
			if raiseIE.MatchString(stripComment(body)) {
				findings = append(findings, fmt.Sprintf("%s:%d: signal handler %s raises InterruptedError; raise SystemExit or KeyboardInterrupt instead (ASD-V0-574)", path, j+1, m[2]))
			}
		}
	}
	return findings
}

// stripComment drops a trailing # comment outside a string literal.
func stripComment(line string) string {
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0 && c == '\\':
			i++
		case quote != 0 && c == quote:
			quote = 0
		case quote == 0 && (c == '\'' || c == '"'):
			quote = c
		case quote == 0 && c == '#':
			return line[:i]
		}
	}
	return line
}
