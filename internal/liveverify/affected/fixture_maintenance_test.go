package affected_test

import (
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// autoMaintenanceSubcommands are the Git subcommands that end by launching
// automatic maintenance (`git maintenance run --auto`, detached by default).
var autoMaintenanceSubcommands = map[string]bool{
	"am": true, "cherry-pick": true, "commit": true, "fetch": true,
	"merge": true, "pull": true, "rebase": true, "revert": true,
}

// unguardedFixture reports why a Go test source's Git fixture can launch a
// detached `git maintenance` that races its TempDir cleanup: it hides the
// host's global Git config (where CI disables maintenance), runs a subcommand
// that triggers auto maintenance, and does not pass both command-local
// safeguards. Only string literals count, so a comment cannot satisfy it.
func unguardedFixture(source []byte) (string, error) {
	files := token.NewFileSet()
	var lexer scanner.Scanner
	var failure error
	lexer.Init(files.AddFile("", files.Base(), len(source)), source, func(position token.Position, message string) {
		if failure == nil {
			failure = &scanner.Error{Pos: position, Msg: message}
		}
	}, 0)
	isolates, mutating, maintenance, gc := false, "", false, false
	for {
		_, kind, literal := lexer.Scan()
		if kind == token.EOF {
			break
		}
		if kind != token.STRING {
			continue
		}
		value, err := strconv.Unquote(literal)
		if err != nil {
			return "", err
		}
		isolates = isolates || strings.Contains(value, "GIT_CONFIG_GLOBAL") || strings.HasPrefix(value, "HOME=")
		if mutating == "" && autoMaintenanceSubcommands[value] {
			mutating = value
		}
		maintenance = maintenance || value == "maintenance.auto=false"
		gc = gc || value == "gc.auto=0"
	}
	if failure != nil {
		return "", failure
	}
	if !isolates || mutating == "" || (maintenance && gc) {
		return "", nil
	}
	return "hides the global Git config and runs git " + mutating + " without -c maintenance.auto=false -c gc.auto=0", nil
}

// TestUnguardedFixtureDetectsAMissingSafeguard proves the guard below fails a
// fixture that omits either safeguard, so its pass is not vacuous.
func TestUnguardedFixtureDetectsAMissingSafeguard(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		unguarded    bool
	}{
		{"isolated commit without safeguard", `package p; var e, a = "GIT_CONFIG_GLOBAL=/dev/null", []string{"commit", "-m", "x"}`, true},
		{"isolated commit with only maintenance.auto", `package p; var e, a = "GIT_CONFIG_GLOBAL=/dev/null", []string{"-c", "maintenance.auto=false", "commit"}`, true},
		{"home override fetch", `package p; var e, a = "HOME=/tmp/x", []string{"fetch"}`, true},
		{"safeguard only in a comment", "package p\n// \"maintenance.auto=false\" \"gc.auto=0\"\nvar e, a = \"GIT_CONFIG_GLOBAL=/dev/null\", []string{\"commit\"}", true},
		{"isolated commit with both safeguards", `package p; var e, a = "GIT_CONFIG_GLOBAL=/dev/null", []string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "commit"}`, false},
		{"commit under the host config", `package p; var a = []string{"commit", "-m", "x"}`, false},
		{"isolated read only", `package p; var e, a = "GIT_CONFIG_GLOBAL=/dev/null", []string{"status"}`, false},
	} {
		reason, err := unguardedFixture([]byte(tc.source))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if (reason != "") != tc.unguarded {
			t.Fatalf("%s: reason=%q want unguarded=%v", tc.name, reason, tc.unguarded)
		}
	}
}

// TestLiveVerifyGitFixturesDisableDetachedMaintenance is V1-0662: every
// internal/liveverify test fixture that isolates Git from the host's global
// config must disable automatic maintenance command-locally, because CI's
// global maintenance.auto=false/gc.auto=0 no longer reaches it and a detached
// `git maintenance` can outlive the test and fail its TempDir cleanup.
func TestLiveVerifyGitFixturesDisableDetachedMaintenance(t *testing.T) {
	self, err := filepath.Abs("fixture_maintenance_test.go")
	if err != nil {
		t.Fatal(err)
	}
	isolating := 0
	err = filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if absolute, err := filepath.Abs(path); err != nil || absolute == self {
			return err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(source), "GIT_CONFIG_GLOBAL") {
			isolating++
		}
		reason, err := unguardedFixture(source)
		if err != nil {
			return err
		}
		if reason != "" {
			t.Errorf("%s %s", path, reason)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// observation, golang, typescript (Mocha) and pymutate fixtures isolate Git
	// today; fewer means the walk no longer reaches them and the guard is vacuous.
	if isolating < 4 {
		t.Fatalf("scanned %d config-isolating fixture files, want at least 4", isolating)
	}
}
