// Command test-confine is the `go test -exec` wrapper that enforces declared
// test read scopes (AFP-V0-023). Run as `test-confine BINARY ARGS...` from the
// package directory go test chooses, with CORVINT_CONFINE_ROOT naming the
// repository root: an undeclared package's test binary runs as is; a declared
// one runs under Landlock, reading below the root only its own directory and
// its declared entries. Any doubt (no root, a directory outside it, an invalid
// declaration, no Landlock for a declared package) exits 2 without running the
// test, so a shard can fail but never pass unconfined.
//
// `test-confine -probe` prints the Landlock ABI version and exits 1 if it is
// below 2, which confinement needs.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/.github/testconfine"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-probe" {
		abi := testconfine.ABI()
		fmt.Printf("landlock abi %d\n", abi)
		if abi < 2 {
			os.Exit(1)
		}
		return
	}
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "test-confine:", err)
		os.Exit(2)
	}
}

func run(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("usage: test-confine BINARY ARGS... | test-confine -probe")
	}
	root, err := resolved(os.Getenv("CORVINT_CONFINE_ROOT"))
	if err != nil {
		return fmt.Errorf("CORVINT_CONFINE_ROOT: %w", err)
	}
	working, err := os.Getwd()
	if err != nil {
		return err
	}
	if working, err = resolved(working); err != nil {
		return err
	}
	relative, err := filepath.Rel(root, working)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("working directory %s is outside %s", working, root)
	}
	scopes, err := testconfine.Load(root)
	if err != nil {
		return err
	}
	binary, err := exec.LookPath(args[0])
	if err != nil {
		return err
	}
	if binary, err = filepath.Abs(binary); err != nil {
		return err
	}
	entries, declared := scopes[filepath.ToSlash(relative)]
	if !declared {
		return execUnconfined(binary, args)
	}
	rules, err := testconfine.Rules(root, filepath.ToSlash(relative), entries)
	if err != nil {
		return err
	}
	return testconfine.ExecConfined(rules, binary, args, os.Environ())
}

func resolved(name string) (string, error) {
	if name == "" || !filepath.IsAbs(name) {
		return "", fmt.Errorf("%q is not an absolute path", name)
	}
	return filepath.EvalSymlinks(name)
}
