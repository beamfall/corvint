// Command test-commands-check is the STR-V0-006 declaration gate: the Go
// environment and flags declared in .github/cishards/test-commands.json
// (corvint-test-commands/0) must equal what `make -s -n go-test` would run,
// and every declared language command that names a make target must exist.
// `corvint test` reads the declaration as a plain file and never interprets
// the Makefile; this gate is what keeps the two from drifting
// (docs/specs/session-test-runner-v0.md). It compares the Makefile defaults
// under a scrubbed environment (PATH only): a developer override such as
// GO_TEST_TIMEOUT=1m or CORVINT_GOCACHE is not the declared full suite.
// Every recipe and declared word must be a plain word, because corvint test
// passes the declared words to exec literally while make hands the recipe to
// a shell; a word a shell would rewrite (quotes, $, globs, redirections) is a
// difference even when both spellings agree.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

const makeDeadline = time.Minute

func main() {
	root := flag.String("root", ".", "repository root holding the Makefile")
	declaration := flag.String("declaration", "", "declaration file (default <root>/"+sessiontest.DeclarationPath+")")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: test-commands-check [--root DIR] [--declaration FILE]")
		os.Exit(2)
	}
	if *declaration == "" {
		*declaration = filepath.Join(*root, filepath.FromSlash(sessiontest.DeclarationPath))
	}
	os.Exit(run(*root, *declaration, os.Stdout, os.Stderr))
}

// run returns 0 when the declaration equals the Makefile, 1 on any
// difference or unreadable input, naming every difference on stderr.
func run(root, declaration string, stdout, stderr io.Writer) int {
	declared := sessiontest.ReadDeclaredFile(declaration)
	if declared.Unreadable != "" {
		fmt.Fprintln(stderr, "test-commands-check: "+declared.Unreadable)
		return 1
	}
	problems := []string{}
	recipe, err := dryRun(root, "go-test")
	if err != nil {
		problems = append(problems, "make -s -n go-test: "+err.Error())
	} else {
		problems = append(problems, compareGo(declared, recipe)...)
	}
	for _, entry := range declared.Languages {
		if len(entry.Argv) < 2 || entry.Argv[0] != "make" {
			continue
		}
		if _, err := dryRun(root, entry.Argv[1:]...); err != nil {
			problems = append(problems, fmt.Sprintf("languages.%s names make %s: %v", entry.Language, strings.Join(entry.Argv[1:], " "), err))
		}
	}
	if len(problems) != 0 {
		for _, problem := range problems {
			fmt.Fprintln(stderr, "test-commands-check: "+problem)
		}
		return 1
	}
	fmt.Fprintf(stdout, "test-commands-check: %s equals make go-test (%s go test %s ./...)\n",
		sessiontest.DeclarationPath, strings.Join(declared.GoEnv, " "), strings.Join(declared.GoFlags, " "))
	return 0
}

// compareGo checks that the one go-test recipe line is exactly the declared
// environment (as a set of NAME=VALUE words) followed by `go test`, the
// declared flags in order, and `./...`.
func compareGo(declared sessiontest.Declared, recipe string) []string {
	// Only a space or tab separates recipe words, and only a line of those
	// is blank: any other whitespace or control rune (U+00A0, \r, ...) is part
	// of a shell word, not a separator strings.Fields may hide.
	problems := []string{}
	lines := []string{}
	for _, line := range strings.Split(recipe, "\n") {
		for _, r := range line {
			if (unicode.IsSpace(r) || unicode.IsControl(r)) && r != ' ' && r != '\t' {
				problems = append(problems, fmt.Sprintf("make go-test recipe contains the whitespace or control rune %U, which is not a word separator", r))
			}
		}
		if strings.Trim(line, " \t") != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != 1 {
		return append(problems, fmt.Sprintf("make -s -n go-test prints %d command lines, want exactly one", len(lines)))
	}
	words := strings.FieldsFunc(lines[0], func(r rune) bool { return r == ' ' || r == '\t' })
	for _, word := range append(append(append([]string{}, words...), declared.GoEnv...), declared.GoFlags...) {
		if !plainWord(word) {
			problems = append(problems, fmt.Sprintf("%q is not a plain word: the shell would rewrite it, corvint test passes it literally", word))
		}
	}
	env := []string{}
	for len(words) != 0 && strings.Contains(words[0], "=") && !strings.HasPrefix(words[0], "-") {
		if name, _, _ := strings.Cut(words[0], "="); !sessiontest.EnvName(name) {
			problems = append(problems, fmt.Sprintf("%q is not a shell assignment: the shell would run it as a command", words[0]))
		}
		env, words = append(env, words[0]), words[1:]
	}
	sort.Strings(env)
	want := append(append([]string{"go", "test"}, declared.GoFlags...), "./...")
	if !reflect.DeepEqual(env, declared.GoEnv) {
		problems = append(problems, fmt.Sprintf("go.env is %v, make go-test sets %v", declared.GoEnv, env))
	}
	if !reflect.DeepEqual(words, want) {
		problems = append(problems, fmt.Sprintf("go.flags give %q, make go-test runs %q", strings.Join(want, " "), strings.Join(words, " ")))
	}
	return problems
}

// plainWord reports whether word reaches a program unchanged through both
// `sh -c` and exec: only letters, digits and the characters _ - . / = : , + @ %.
func plainWord(word string) bool {
	if word == "" {
		return false
	}
	for _, r := range word {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("_-./=:,+@%", r):
		default:
			return false
		}
	}
	return true
}

// dryRun prints what make would run for the targets, under an environment
// holding only PATH so no inherited MAKEFLAGS or variable override leaks in.
func dryRun(root string, targets ...string) (string, error) {
	makePath, err := exec.LookPath("make")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), makeDeadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, makePath, append([]string{"-s", "-n"}, targets...)...)
	cmd.Dir, cmd.Env = root, []string{"PATH=" + os.Getenv("PATH")}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
