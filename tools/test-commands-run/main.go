// Command test-commands-run runs the Go full suite exactly as the project's
// declaration .github/cishards/test-commands.json (corvint-test-commands/0)
// states it: `go test <go.flags> ./...` with go.env applied over the inherited
// environment, in the repository root, with the test run's output and exit
// status passed through. The Makefile's go-test rule is this one fixed
// command, so `make go-test` runs the declaration rather than a recipe the
// declaration would have to mirror (STR-V0-006,
// docs/specs/session-test-runner-v0.md). It reads the declaration through
// the same strict reader `corvint test` uses and refuses an unreadable one.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the declared Go command from the current directory, which
// must be the repository root (the directory holding the declaration), and
// returns go's exit status, or 2 when the declaration or the directory
// refuses the run before go starts.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintf(stderr, "test-commands-run: takes no arguments, got %q; the packages and flags come from %s\n", args, sessiontest.DeclarationPath)
		return 2
	}
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "test-commands-run: "+err.Error())
		return 2
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		fmt.Fprintf(stderr, "test-commands-run: %s holds no go.mod: run it from the repository root\n", root)
		return 2
	}
	declared := sessiontest.ReadDeclared(root)
	if declared.Unreadable != "" {
		fmt.Fprintln(stderr, "test-commands-run: "+declared.Unreadable)
		return 2
	}
	argv, env := invocation(declared, os.Environ())
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = root, env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, stdout, stderr
	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit) && exit.ExitCode() > 0:
		return exit.ExitCode()
	}
	fmt.Fprintf(stderr, "test-commands-run: %s: %v\n", strings.Join(argv, " "), err)
	return 1
}

// invocation is the declared command: argv `go test <go.flags> ./...` and
// the environment with each go.env entry replacing any inherited value of
// the same name.
func invocation(declared sessiontest.Declared, environ []string) ([]string, []string) {
	argv := append(append([]string{"go", "test"}, declared.GoFlags...), "./...")
	env := make([]string, 0, len(environ)+len(declared.GoEnv))
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if !declaredName(declared, name) {
			env = append(env, entry)
		}
	}
	return argv, append(env, declared.GoEnv...)
}

func declaredName(declared sessiontest.Declared, name string) bool {
	for _, entry := range declared.GoEnv {
		if declaredName, _, _ := strings.Cut(entry, "="); declaredName == name {
			return true
		}
	}
	return false
}
