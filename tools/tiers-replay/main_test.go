package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AFP-V0-991: a tiers.sh document becomes NARROWED with tier sizes, FULL with
// its reasons, or INCOMPLETE when the tiers were not measured.
func TestAFPV0991DecideReadsTheTierDocument(t *testing.T) {
	for raw, want := range map[string]row{
		`{"tiers":{"changed":["m/a"],"hop1":["m/a","m/b"],"hop2":["m/a","m/b","m/c"],"full":[],"universe":9},"tierIncomplete":[],"millis":12}`:                {Decision: "NARROWED", Changed: 1, Hop1: 2, Hop2: 3, Universe: 9},
		`{"tiers":{"changed":"FULL","hop1":"FULL","hop2":"FULL","full":["a non-Go file changed: README.md"],"universe":9},"tierIncomplete":[],"millis":null}`: {Decision: "FULL", Full: []string{"a non-Go file changed: README.md"}, Universe: 9},
		`{"tiers":null,"tierIncomplete":["go list ./... failed: x"],"millis":null}`:                                                                           {Decision: "INCOMPLETE", TierIncomplete: []string{"go list ./... failed: x"}},
	} {
		got, err := decide("m", "b", "s", 1, []byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		got.Merge, got.Base, got.Subject, got.ChangedFiles, got.Millis = "", "", "", 0, nil
		if g, _ := json.Marshal(got); string(g) != mustJSON(t, want) {
			t.Errorf("decide(%s) = %s, want %s", raw, g, mustJSON(t, want))
		}
	}
}

// AFP-V0-991: the replay runs this checkout's tiers.sh over each merge's own
// tree: a merge that adds a package importing another is measured against the
// go list of that merge, not of the ref's tip, and nothing in the repository
// changes.
func TestAFPV0991ReplayUsesEachMergesOwnGoList(t *testing.T) {
	for _, tool := range []string{"git", "go", "jq", "tar"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " is not installed")
		}
	}
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", repo}, args...)...)
		command.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-10-31T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-31T00:00:00Z")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script, err := os.ReadFile(filepath.Join("..", "..", ".github", "forward-rows", "tiers.sh"))
	if err != nil {
		t.Fatal(err)
	}
	jq, err := os.ReadFile(filepath.Join("..", "..", ".github", "forward-rows", "tiers.jq"))
	if err != nil {
		t.Fatal(err)
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@example.test")
	run("config", "user.name", "T")
	write(".github/forward-rows/tiers.sh", string(script))
	write(".github/forward-rows/tiers.jq", string(jq))
	write("go.mod", "module example.com/m\n\ngo 1.21\n")
	write("a/a.go", "package a\n\nfunc A() int { return 1 }\n")
	run("add", ".")
	run("commit", "-qm", "base")
	merge := func(branch, file, body string) {
		run("checkout", "-qb", branch)
		write(file, body)
		run("add", ".")
		run("commit", "-qm", branch)
		run("checkout", "-q", "main")
		run("merge", "-q", "--no-ff", "-m", "merge "+branch, branch)
	}
	merge("add-b", "b/b.go", "package b\n\nimport \"example.com/m/a\"\n\nfunc B() int { return a.A() }\n")
	merge("edit-a", "a/a.go", "package a\n\nfunc A() int { return 2 }\n")
	merge("docs", "README.md", "readme\n")
	head := run("rev-parse", "HEAD")
	t.Setenv("TMPDIR", t.TempDir())
	var out bytes.Buffer
	if err := replay(repo, "main", 3, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("want 3 rows and a summary:\n%s", out.String())
	}
	var rows []row
	for _, line := range lines[:3] {
		var r row
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, r)
	}
	if rows[0].Subject != "merge docs" || rows[0].Decision != "FULL" || len(rows[0].Full) != 1 {
		t.Errorf("docs merge: %+v", rows[0])
	}
	// edit-a reaches b through the import b's own merge added: hop1 holds a and b.
	if rows[1].Subject != "merge edit-a" || rows[1].Decision != "NARROWED" || rows[1].Changed != 1 || rows[1].Hop1 != 2 || rows[1].Universe != 2 {
		t.Errorf("edit-a merge: %+v", rows[1])
	}
	if rows[2].Subject != "merge add-b" || rows[2].Decision != "NARROWED" || rows[2].Changed != 1 || rows[2].Hop1 != 1 {
		t.Errorf("add-b merge: %+v", rows[2])
	}
	if !strings.Contains(lines[3], `"decisions":{"FULL":1,"NARROWED":2}`) || !strings.Contains(lines[3], `"merges":3`) {
		t.Errorf("summary: %s", lines[3])
	}
	if run("rev-parse", "HEAD") != head || run("status", "--porcelain") != "" {
		t.Error("the replay changed the repository")
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
