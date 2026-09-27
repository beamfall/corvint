package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestEmptyRootsRefuseBeforeExecution(t *testing.T) {
	for _, args := range [][]string{
		{"-source-root="},
		{},
	} {
		scratch := filepath.Join(t.TempDir(), "must-not-exist")
		args = append(args, "-scratch", scratch, "-output-parent", t.TempDir(), "-bundle-name", "fixture", "-npm-cache", t.TempDir())
		var out, err bytes.Buffer
		if code := run(args, &out, &err); code != 2 || !strings.Contains(err.String(), "required") {
			t.Fatalf("%v: %d %s", args, code, err.String())
		}
		if _, e := os.Stat(scratch); !os.IsNotExist(e) {
			t.Fatalf("refusal touched scratch: %v", e)
		}
	}
}

func TestCoreBundleBuildSkipsEditorTools(t *testing.T) {
	t.Run("CRB-V0-019 no npm cache argument required", func(t *testing.T) {
		scratch := filepath.Join(t.TempDir(), "must-not-exist")
		args := []string{"-source-root", t.TempDir(), "-scratch", scratch, "-output-parent", t.TempDir(), "-bundle-name", "core", "-target", "unsupported"}
		var out, err bytes.Buffer
		if code := run(args, &out, &err); code != 1 || !strings.Contains(err.String(), "unsupported") {
			t.Fatalf("missing npm cache rejected before core admission: %d %s", code, err.String())
		}
		if _, e := os.Stat(scratch); !os.IsNotExist(e) {
			t.Fatalf("refusal touched scratch: %v", e)
		}
	})
}

// TestGateScriptsPassOnlyDefinedFlags pins V1-0346: every flag a gate script
// passes to this command is one the command defines, so a wrapper still naming
// a retired flag fails here instead of at release time.
func TestGateScriptsPassOnlyDefinedFlags(t *testing.T) {
	for _, script := range []string{"local-console-release-gate", "corvint-companion-release-gate"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "script", script))
		if err != nil {
			t.Fatal(err)
		}
		flags := invokedFlags(string(raw))
		if !slices.Contains(flags, "-source-root") {
			t.Fatalf("%s: invocation flags %v lack -source-root", script, flags)
		}
		for _, name := range flags {
			var out, stderr bytes.Buffer
			run([]string{name + "=x"}, &out, &stderr)
			if strings.Contains(stderr.String(), "not defined") {
				t.Errorf("%s passes undefined flag %s", script, name)
			}
		}
	}
}

// TestLocalConsoleGateBuildsFromRepositoryRoot pins V1-0353: the gate builds
// the release command in the checkout that holds the script, whatever the
// caller's working directory. A fake go on PATH records where it ran and stops
// the gate before any real build.
func TestLocalConsoleGateBuildsFromRepositoryRoot(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if root, err = filepath.EvalSymlinks(root); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	record := filepath.Join(bin, "go-dir")
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte("#!/bin/sh\npwd -P > '"+record+"'\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(filepath.Join(root, "script", "local-console-release-gate"))
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin", "CORVINT_COMPANION_SCRATCH="+t.TempDir(), "CORVINT_COMPANION_OUTPUT="+t.TempDir())
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("fake go did not stop the gate:\n%s", out)
	}
	dir, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(dir)); got != root {
		t.Fatalf("gate built in %s, want %s", got, root)
	}
}

// invokedFlags returns the flags on the backslash-continued command line that
// invokes corvint-companion-release.
func invokedFlags(script string) []string {
	var flags []string
	inside := false
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if !inside {
			inside = strings.Contains(trimmed, "corvint-companion-release") && strings.HasSuffix(trimmed, `\`)
			continue
		}
		if name, _, _ := strings.Cut(trimmed, " "); strings.HasPrefix(name, "-") {
			flags = append(flags, name)
		}
		if !strings.HasSuffix(trimmed, `\`) {
			break
		}
	}
	return flags
}
