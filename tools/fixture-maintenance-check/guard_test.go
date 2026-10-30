package fixturemaintenancecheck

import (
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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

// mutatingSubcommand returns the auto-maintenance subcommand a literal names,
// either as the whole literal (an argument) or as a word of a shell command
// that runs git ("git -C dir commit -m x").
func mutatingSubcommand(value string) string {
	if autoMaintenanceSubcommands[value] {
		return value
	}
	words := strings.Fields(value)
	for index, word := range words {
		if word != "git" {
			continue
		}
		for _, later := range words[index+1:] {
			if autoMaintenanceSubcommands[later] {
				return later
			}
		}
	}
	return ""
}

// packageFacts are the literal-token facts of one package's Go test files.
type packageFacts struct {
	isolates    bool   // hides the host's global Git config
	mutating    string // first auto-maintenance subcommand literal
	maintenance bool   // maintenance.auto=false is passed
	gc          bool   // gc.auto=0 is passed
}

// add folds one Go test source into the package facts. Only string literals
// count, so a comment cannot satisfy or trip the guard.
func (facts *packageFacts) add(source []byte) error {
	files := token.NewFileSet()
	var lexer scanner.Scanner
	var failure error
	lexer.Init(files.AddFile("", files.Base(), len(source)), source, func(position token.Position, message string) {
		if failure == nil {
			failure = &scanner.Error{Pos: position, Msg: message}
		}
	}, 0)
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
			return err
		}
		facts.isolates = facts.isolates || strings.Contains(value, "GIT_CONFIG_GLOBAL") || strings.HasPrefix(value, "HOME=")
		if facts.mutating == "" {
			facts.mutating = mutatingSubcommand(value)
		}
		// Contains, not equality: a shell script literal carries the flags
		// inside one string, and GIT_CONFIG_PARAMETERS quotes each key.
		facts.maintenance = facts.maintenance || strings.Contains(value, "maintenance.auto=false") || strings.Contains(value, "'maintenance.auto'='false'")
		facts.gc = facts.gc || strings.Contains(value, "gc.auto=0") || strings.Contains(value, "'gc.auto'='0'")
	}
	return failure
}

// unguarded reports why the package's Git fixtures can launch a detached
// `git maintenance` that races TempDir cleanup: they hide the host's global
// Git config (where CI disables maintenance), run a subcommand that triggers
// auto maintenance, and pass no command-local -c maintenance.auto=false and
// -c gc.auto=0. The check is per package so a shared helper in one file
// covers callers in its siblings.
func (facts packageFacts) unguarded() string {
	if !facts.isolates || facts.mutating == "" || (facts.maintenance && facts.gc) {
		return ""
	}
	return "hides the global Git config and runs git " + facts.mutating + " without -c maintenance.auto=false -c gc.auto=0"
}

// TestUnguardedDetectsAMissingSafeguard proves the guard fails a fixture that
// omits either safeguard, so its pass is not vacuous.
func TestUnguardedDetectsAMissingSafeguard(t *testing.T) {
	for _, tc := range []struct {
		name      string
		sources   []string
		unguarded bool
	}{
		{"isolated commit without safeguard", []string{`package p; var e, a = "GIT_CONFIG_GLOBAL=/dev/null", []string{"commit", "-m", "x"}`}, true},
		{"only maintenance.auto", []string{`package p; var e, a = "GIT_CONFIG_GLOBAL=/dev/null", []string{"-c", "maintenance.auto=false", "commit"}`}, true},
		{"only gc.auto", []string{`package p; var e, a = "HOME=/tmp/x", []string{"-c", "gc.auto=0", "fetch"}`}, true},
		{"home override merge", []string{`package p; var e, a = "HOME=/tmp/x", []string{"merge"}`}, true},
		{"safeguard only in a comment", []string{"package p\n// \"maintenance.auto=false\" \"gc.auto=0\"\nvar e, a = \"GIT_CONFIG_GLOBAL=/dev/null\", []string{\"commit\"}"}, true},
		{"commit in one file, safeguard in none of the package", []string{`package p; var e = "GIT_CONFIG_GLOBAL=/dev/null"`, `package p; var a = []string{"commit"}`}, true},
		{"shell-only commit without safeguard", []string{`package p; var e, a = "GIT_CONFIG_GLOBAL=/dev/null", "git -C dir commit -qm x"`}, true},
		{"both safeguards", []string{`package p; var e, a = "GIT_CONFIG_GLOBAL=/dev/null", []string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "commit"}`}, false},
		{"helper in a sibling file", []string{`package p; var e, a = "GIT_CONFIG_GLOBAL=/dev/null", []string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}`, `package p; var a = []string{"commit"}`}, false},
		{"script literal carries both", []string{`package p; var e, a = "GIT_CONFIG_GLOBAL=/dev/null", "git -c maintenance.auto=false -c gc.auto=0 commit"`}, false},
		{"parameters environment", []string{`package p; var e, a = "HOME=/x", "GIT_CONFIG_PARAMETERS='maintenance.auto'='false' 'gc.auto'='0'"; var c = "commit"`}, false},
		{"commit under the host config", []string{`package p; var a = []string{"commit", "-m", "x"}`}, false},
		{"isolated read only", []string{`package p; var e, a = "GIT_CONFIG_GLOBAL=/dev/null", []string{"status"}`}, false},
	} {
		var facts packageFacts
		for _, source := range tc.sources {
			if err := facts.add([]byte(source)); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
		}
		if got := facts.unguarded() != ""; got != tc.unguarded {
			t.Fatalf("%s: unguarded=%v want %v", tc.name, got, tc.unguarded)
		}
	}
}

// TestRepositoryGitFixturesDisableDetachedMaintenance is V1-1030: every Go
// test package whose Git fixtures isolate Git from the host's global config
// must disable automatic maintenance command-locally, because CI's global
// maintenance.auto=false/gc.auto=0 no longer reaches them and a detached
// `git maintenance` can outlive the test and fail its TempDir cleanup.
func TestRepositoryGitFixturesDisableDetachedMaintenance(t *testing.T) {
	factsByDirectory := map[string]*packageFacts{}
	scanned := 0
	for _, top := range []string{"cmd", "conformance", "internal", "interop", "tools"} {
		err := filepath.WalkDir(filepath.Join("..", "..", top), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" || entry.Name() == "node_modules" || entry.Name() == "fixture-maintenance-check" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, "_test.go") {
				return nil
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !strings.Contains(string(source), "git") {
				return nil
			}
			directory := filepath.ToSlash(filepath.Dir(strings.TrimPrefix(filepath.ToSlash(path), "../../")))
			facts := factsByDirectory[directory]
			if facts == nil {
				facts = &packageFacts{}
				factsByDirectory[directory] = facts
			}
			scanned++
			return facts.add(source)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	directories := make([]string, 0, len(factsByDirectory))
	for directory := range factsByDirectory {
		directories = append(directories, directory)
	}
	sort.Strings(directories)
	isolating := 0
	for _, directory := range directories {
		facts := factsByDirectory[directory]
		if facts.isolates {
			isolating++
		}
		reason := facts.unguarded()
		if reason != "" {
			t.Errorf("%s %s", directory, reason)
		}
	}
	// Dozens of packages isolate Git today; far fewer means the walk no longer
	// reaches them and the guard is vacuous.
	if isolating < 40 {
		t.Fatalf("scanned %d Git-isolating packages in %d files, want at least 40", isolating, scanned)
	}
}
