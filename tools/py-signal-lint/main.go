// Command py-signal-lint fails when a Python signal handler raises InterruptedError
// (docs/specs/lane-check-tooling-v0.md, ASD-V0-574; V1-2178). InterruptedError is an OSError
// that the code around an interrupted call can catch and retry, so a handler that raises it can
// leave a qualification script running with its children; the repository's handlers raise
// SystemExit or KeyboardInterrupt and kill their process groups (V1-0610). For each tracked *.py
// file, a handler is a function named as the second argument of signal.signal(...), or a lambda
// written there; each call is read as one logical call through its closing parenthesis, so a
// registration split across lines counts. String literals (triple-quoted docstrings among them)
// and comments are blanked before any of this is read, so a handler that only names
// InterruptedError in a string passes. Exit 1 with one line per finding, 2 when the file list
// cannot be read.
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
	signalCall   = regexp.MustCompile(`signal\.signal\(`)
	register     = regexp.MustCompile(`^signal\.signal\(\s*[^,()]+(?:\([^()]*\))?\s*,\s*([A-Za-z_][A-Za-z0-9_.]*)\s*,?\s*\)`)
	registerLamb = regexp.MustCompile(`^signal\.signal\([^,]+,\s*lambda\b.*\bInterruptedError\b`)
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
	// The registrations, handler definitions and raise statements are read from code only: string
	// contents and comments are blanked, with every newline kept, so offsets and line numbers hold.
	text := blankCode(string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))))
	lines := strings.Split(text, "\n")
	handlers := map[string]bool{}
	var findings []string
	for _, loc := range signalCall.FindAllStringIndex(text, -1) {
		call := logicalCall(text, loc[0], loc[1])
		if m := register.FindStringSubmatch(call); m != nil {
			handlers[m[1][strings.LastIndex(m[1], ".")+1:]] = true
		}
		if registerLamb.MatchString(call) {
			findings = append(findings, fmt.Sprintf("%s:%d: a signal handler lambda raises InterruptedError; raise SystemExit or KeyboardInterrupt instead (ASD-V0-574)", path, strings.Count(text[:loc[0]], "\n")+1))
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
			if raiseIE.MatchString(body) {
				findings = append(findings, fmt.Sprintf("%s:%d: signal handler %s raises InterruptedError; raise SystemExit or KeyboardInterrupt instead (ASD-V0-574)", path, j+1, m[2]))
			}
		}
	}
	return findings
}

// logicalCall returns the call that starts at text[start] and whose argument list opens at
// text[open], through its matching closing parenthesis, with newlines flattened to spaces, so a
// registration split across lines matches like one written on a single line. text comes from
// blankCode, so a parenthesis in a string or comment is already gone; an unclosed call runs to the
// end of the file, so it is still read.
func logicalCall(text string, start, open int) string {
	depth, end := 1, len(text)
	for i := open; i < len(text) && end == len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				end = i + 1
			}
		}
	}
	return strings.ReplaceAll(text[start:end], "\n", " ")
}

// blankCode returns src with every string literal, its quotes included, and every comment replaced
// by spaces, keeping each newline, so the result has src's length, offsets and line numbers. A
// literal opens at ' or " outside a comment (a prefix such as r, b or f stays as code) and is
// triple-quoted when the quote is tripled; a backslash keeps the next byte inside the literal, as
// Python's tokenizer reads raw literals too. A single-quoted literal left open ends at its line.
func blankCode(src string) string {
	out := []byte(src)
	blank := func(from, to int) {
		for k := from; k < to; k++ {
			if out[k] != '\n' {
				out[k] = ' '
			}
		}
	}
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '#':
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				end = len(src) - i
			}
			blank(i, i+end)
			i += end
		case c == '\'' || c == '"':
			delimiter := src[i : i+1]
			if strings.HasPrefix(src[i:], strings.Repeat(delimiter, 3)) {
				delimiter = strings.Repeat(delimiter, 3)
			}
			j := i + len(delimiter)
			for j < len(src) {
				if src[j] == '\\' {
					j += 2
					continue
				}
				if strings.HasPrefix(src[j:], delimiter) {
					j += len(delimiter)
					break
				}
				if src[j] == '\n' && len(delimiter) == 1 {
					break
				}
				j++
			}
			j = min(j, len(src))
			blank(i, j)
			i = j
		default:
			i++
		}
	}
	return string(out)
}
