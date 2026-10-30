package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// compileAdviceRepository is a committed Go module with one tested package,
// an untested package, an untested command importing it, and a nested module.
func compileAdviceRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":                 "module example.test/f\n\ngo 1.22\n",
		"go.sum":                 "",
		"tested/tested.go":       "package tested\n",
		"tested/tested_test.go":  "package tested\n",
		"cue/validate.go":        "package cue\n\nfunc Validate() error { return nil }\n",
		"app/main.go":            "package main\n\nimport _ \"example.test/f/cue\"\n\nfunc main() {}\n",
		"nested/go.mod":          "module example.test/nested\n\ngo 1.22\n",
		"nested/nested.go":       "package nested\n",
		"nested/nested_test.go":  "package nested\n",
		"testdata/fake/go.mod":   "module example.test/fake\n",
		"testdata/fake/fake.txt": "fixture\n",
	} {
		cemWrite(t, root, name, body)
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
		{"add", "."}, {"commit", "-qm", "fixture"},
	} {
		affectedGit(t, root, arguments...)
	}
	return root
}

func affectedCheckCommands(t *testing.T, receipt map[string]any) []string {
	t.Helper()
	var commands []string
	for _, check := range affectedChecks(t, receipt) {
		commands = append(commands, check["command"].(string))
	}
	return commands
}

// AFP-V0-468 (V1-2759): a dirty Go package with no test still gets a runnable
// compile-only check over it and the untested packages it reaches, so an
// empty test selection is never a silent empty advice.
func TestAffectedUntestedPackageGetsCompileCheck_AFP_V0_468(t *testing.T) {
	t.Parallel()
	root := compileAdviceRepository(t)
	// The edit changes an existing body, so additive narrowing (ADN-V0-003)
	// keeps the importing command reached.
	cemWrite(t, root, "cue/validate.go", "package cue\n\nfunc Validate() error { return Extra() }\n\nfunc Extra() error { return nil }\n")
	receipt, _, stderr, code := runAffectedCLI(t, root, "--full")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if state := receipt["provider"].(map[string]any)["go"].(map[string]any)["state"]; state != providerStateEmpty {
		t.Fatalf("provider.go.state = %v, want %s", state, providerStateEmpty)
	}
	want := "GOTOOLCHAIN=local go test -count=1 -run '^$' 'example.test/f/app' 'example.test/f/cue'"
	if commands := affectedCheckCommands(t, receipt); !slices.Equal(commands, []string{want}) {
		t.Fatalf("checks = %q, want only %q", commands, want)
	}
	unknown := affectedAdviceUnknown(t, receipt)
	if !slices.Contains(unknown, "NO_SELECTABLE_TEST: go:example.test/f/cue") {
		t.Fatalf("the untested package must stay a named behaviour gap: %v", unknown)
	}
	for _, line := range unknown {
		if strings.HasPrefix(line, "NO_ADVISORY_GO_COMMAND") {
			t.Fatalf("a compile check covers the empty selection, yet advice says %q", line)
		}
	}
}

// AFP-V0-469 (V1-2759): a dirty go.mod or go.sum yields one module-level
// compile check per module directory, a nested module from its own directory;
// a go.mod under testdata is a fixture and adds nothing; a dirty go.mod that no
// longer declares a module is a named unknown.
func TestAffectedGoModuleChangeGetsModuleCheck_AFP_V0_469(t *testing.T) {
	t.Parallel()
	t.Run("changed requirements", func(t *testing.T) {
		t.Parallel()
		root := compileAdviceRepository(t)
		cemWrite(t, root, "go.mod", "module example.test/f\n\ngo 1.23\n")
		cemWrite(t, root, "go.sum", "example.test/dep v1.0.0 h1:x=\n")
		cemWrite(t, root, "nested/go.mod", "module example.test/nested\n\ngo 1.23\n")
		cemWrite(t, root, "testdata/fake/go.mod", "module example.test/fake\n\ngo 1.23\n")
		receipt, _, stderr, code := runAffectedCLI(t, root, "--full")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		want := []string{"GOTOOLCHAIN=local go vet ./...", "(cd 'nested' && GOTOOLCHAIN=local go vet ./...)"}
		if commands := affectedCheckCommands(t, receipt); !slices.Equal(commands, want) {
			t.Fatalf("checks = %q, want %q", commands, want)
		}
		for _, line := range affectedAdviceUnknown(t, receipt) {
			if strings.HasPrefix(line, "NO_ADVISORY_GO_COMMAND") || strings.HasPrefix(line, "NO_ADVISORY_GO_MODULE_COMMAND") {
				t.Fatalf("module checks cover the change, yet advice says %q", line)
			}
		}
		if plan := receipt["plan"].(map[string]any); plan["scope"] != "UNKNOWN" {
			t.Fatalf("a changed go.mod must keep the plan scope UNKNOWN: %v", plan["scope"])
		}
	})
	t.Run("removed module", func(t *testing.T) {
		t.Parallel()
		root := compileAdviceRepository(t)
		if err := os.Remove(filepath.Join(root, "nested", "go.mod")); err != nil {
			t.Fatal(err)
		}
		receipt, _, stderr, code := runAffectedCLI(t, root, "--full")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		unknown := affectedAdviceUnknown(t, receipt)
		if !slices.Contains(unknown, "NO_ADVISORY_GO_MODULE_COMMAND: nested/go.mod changed but nested/go.mod declares no readable module") {
			t.Fatalf("a removed go.mod must be a named unknown: %v", unknown)
		}
	})
}

// AFP-V0-469 (V1-2759): the governing go.work decides how a module is vetted.
// A module it uses is vetted in workspace mode, even below testdata; a module
// it does not use is vetted with GOWORK=off, since the go tool refuses ./...
// there; a go.work that does not parse is a named unknown.
func TestAffectedGoModuleCheckFollowsWorkspace_AFP_V0_469(t *testing.T) {
	t.Parallel()
	workspaceRepository := func(t *testing.T, work string) string {
		root := compileAdviceRepository(t)
		cemWrite(t, root, "go.work", work)
		affectedGit(t, root, "add", "go.work")
		affectedGit(t, root, "commit", "-qm", "workspace")
		cemWrite(t, root, "go.mod", "module example.test/f\n\ngo 1.23\n")
		cemWrite(t, root, "nested/go.mod", "module example.test/nested\n\ngo 1.23\n")
		cemWrite(t, root, "testdata/fake/go.mod", "module example.test/fake\n\ngo 1.23\n")
		return root
	}
	t.Run("uses and non-members", func(t *testing.T) {
		t.Parallel()
		root := workspaceRepository(t, "go 1.22\n\nuse (\n\t.\n\t./testdata/fake\n)\n")
		receipt, _, stderr, code := runAffectedCLI(t, root, "--full")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		want := []string{
			"GOTOOLCHAIN=local go vet ./...",
			"(cd 'nested' && GOWORK=off GOTOOLCHAIN=local go vet ./...)",
			"(cd 'testdata/fake' && GOTOOLCHAIN=local go vet ./...)",
		}
		if commands := affectedCheckCommands(t, receipt); !slices.Equal(commands, want) {
			t.Fatalf("checks = %q, want %q", commands, want)
		}
	})
	t.Run("unparsable workspace", func(t *testing.T) {
		t.Parallel()
		root := workspaceRepository(t, "go 1.22\n\nuse (\n")
		receipt, _, stderr, code := runAffectedCLI(t, root, "--full")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		if commands := affectedCheckCommands(t, receipt); len(commands) != 0 {
			t.Fatalf("an unparsable go.work must not yield a module check: %q", commands)
		}
		named := 0
		for _, line := range affectedAdviceUnknown(t, receipt) {
			if strings.HasPrefix(line, "NO_ADVISORY_GO_MODULE_COMMAND: ") && strings.Contains(line, "changed but its governing go.work is unparsable") {
				named++
			}
		}
		if named != 2 {
			t.Fatalf("want the root and nested module changes named, got %d: %v", named, affectedAdviceUnknown(t, receipt))
		}
	})
}
