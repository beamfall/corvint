package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// guardingTestsMap is a prepared map with one hunk of each kind the
// guarding-test search tells apart: an uncited Go source hunk it would
// search, a test file, a mechanical hunk, a Go source hunk already citing a
// _test.go span as test-claim, and a non-Go file.
const guardingTestsMap = `{"evidence": [{"id": "evidence:t", "path": "p/p_test.go"}, {"id": "evidence:s", "path": "p/spec.md"}],
 "hunks": [
  {"disposition": "unknown", "id": "hunk:1", "path": "p/p.go", "basis": []},
  {"disposition": "unknown", "id": "hunk:2", "path": "p/p_test.go", "basis": []},
  {"disposition": "mechanical", "id": "hunk:3", "path": "p/gen.go", "basis": []},
  {"disposition": "cited", "id": "hunk:4", "path": "p/q.go", "basis": [{"evidenceId": "evidence:t", "relation": "test-claim"}]},
  {"disposition": "cited", "id": "hunk:5", "path": "p/r.go", "basis": [{"evidenceId": "evidence:s", "relation": "test-claim"}]},
  {"disposition": "unknown", "id": "hunk:6", "path": "a.txt", "basis": []}
 ]}
`

// TestChangeDCWV0069RecordsGuardingTestsNotRun pins that the bind path keeps
// the guarding-test search visible: a map with a hunk the search would select
// reports the row guarding-tests NOT_PRODUCED guarding-tests-not-run, sends it
// to the observation ledger, prints a NOTE naming the search command and the
// hunk count, and never lists the row as a failure; a map with no such hunk
// reports no row.
func TestChangeDCWV0069RecordsGuardingTestsNotRun(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	prepared := guardingTestsMap
	var observed []string
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o777); err != nil {
				return 1
			}
			if err := os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte(prepared), 0o666); err != nil {
				return 1
			}
			return 0
		case len(args) > 2 && args[0] == "dogfood-observe":
			observed = append(observed, strings.Join(args[1:], " "))
			return 0
		}
		return 1
	}}
	run := func() (string, string) {
		t.Helper()
		observed = nil
		var stderr bytes.Buffer
		if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps}, &stderr); err != nil {
			t.Fatal(err)
		}
		report, err := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
		if err != nil {
			t.Fatal(err)
		}
		return string(report), stderr.String()
	}
	row := `{"name": "guarding-tests", "status": "NOT_PRODUCED", "reason": "guarding-tests-not-run"}`
	report, stderr := run()
	if !strings.Contains(report, `{"name": "cem-cite", "status": "NOT_PRODUCED", "reason": "citation-plan-not-provided"},`+"\n    "+row+",") {
		t.Fatalf("report lacks the guarding-tests row after cem-cite:\n%s", report)
	}
	note := "dogfood-change: NOTE guarding-tests NOT_PRODUCED guarding-tests-not-run hunks=2: to cite an observed guarding test, run corvint cem verify --map .corvint/change.cem.json --expected-base " + base + " --target "
	if !strings.Contains(stderr, note) || !strings.Contains(stderr, " --guarding-tests and add its guardingTests.plan rows to the citation plan;") {
		t.Fatalf("stderr lacks the note %q:\n%s", note, stderr)
	}
	if strings.Contains(stderr, "\n  guarding-tests:") {
		t.Fatalf("the guarding-tests row was listed as a failure:\n%s", stderr)
	}
	if !containsString(observed, "--step guarding-tests --status NOT_PRODUCED --reason guarding-tests-not-run") {
		t.Fatalf("the row was not observed: %q", observed)
	}
	if failing(step{"guarding-tests", "NOT_PRODUCED", "guarding-tests-not-run"}) || !failing(step{"guarding-tests", "NOT_PRODUCED", "exit-1"}) {
		t.Fatal("only the not-run row is exempt from completion")
	}

	// Once every Go source hunk cites a _test.go test-claim, nothing is
	// searched and no row is reported.
	prepared = strings.Replace(strings.Replace(guardingTestsMap, `"path": "p/p.go", "basis": []`, `"path": "p/p.go", "basis": [{"evidenceId": "evidence:t", "relation": "test-claim"}]`, 1),
		`"path": "p/r.go", "basis": [{"evidenceId": "evidence:s"`, `"path": "p/r.go", "basis": [{"evidenceId": "evidence:t"`, 1)
	if err := os.Remove(filepath.Join(root, ".corvint/change.cem.json")); err != nil {
		t.Fatal(err)
	}
	report, stderr = run()
	if strings.Contains(report, "guarding-tests") || strings.Contains(stderr, "guarding-tests") {
		t.Fatalf("a map with no searched hunk reported the step:\n%s\n%s", report, stderr)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// TestShellWordQuotesOnlyWhenNeeded pins that the DCW-V0-069 NOTE command
// stays one shell word per argument for any repository path.
func TestShellWordQuotesOnlyWhenNeeded(t *testing.T) {
	for value, want := range map[string]string{
		"/work/repo":         "/work/repo",
		"/work/my project":   "'/work/my project'",
		"/work/it's":         `'/work/it'\''s'`,
		"":                   "''",
		"/work/$HOME;rm -rf": "'/work/$HOME;rm -rf'",
	} {
		if got := shellWord(value); got != want {
			t.Fatalf("shellWord(%q) = %q, want %q", value, got, want)
		}
	}
}
