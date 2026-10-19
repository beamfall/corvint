package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sccCLIGit runs git in root with a fixed identity and extra env.
func sccCLIGit(t *testing.T, root string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(append(os.Environ(), "GIT_AUTHOR_NAME=scc", "GIT_AUTHOR_EMAIL=scc@example.invalid",
		"GIT_COMMITTER_NAME=scc", "GIT_COMMITTER_EMAIL=scc@example.invalid"), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// sccCLICommit writes a commit on parent that sets path to content, through
// a private index, so the checkout and its index are untouched.
func sccCLICommit(t *testing.T, root, parent, path, content, subject string) string {
	t.Helper()
	index := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	sccCLIGit(t, root, index, "read-tree", parent)
	cmd := exec.Command("git", "hash-object", "-w", "--stdin")
	cmd.Dir, cmd.Stdin = root, strings.NewReader(content)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	sccCLIGit(t, root, index, "update-index", "--add", "--cacheinfo", "100644,"+strings.TrimSpace(string(out))+","+path)
	tree := sccCLIGit(t, root, index, "write-tree")
	return sccCLIGit(t, root, nil, "commit-tree", tree, "-p", parent, "-m", subject)
}

// sccCLIItem returns the item whose commit argument is c.
func sccCLIItem(t *testing.T, items []wire.Value, c string) wire.Value {
	t.Helper()
	for _, it := range items {
		if field(it, "commit").Str == c {
			return it
		}
	}
	t.Fatalf("no item for %s", c)
	return wire.Value{}
}

// SCC-V0-005: `source contains` answers per commit, distinguishes a
// conflict-resolved rebase (PATCH_DIFFERS), abstains on a missing commit,
// bounds its argv and writes nothing.
func TestSCCV0005_SourceContainsVerb(t *testing.T) {
	root := t.TempDir()
	sccCLIGit(t, root, nil, "init", "-q", "-b", "main")
	sccCLIGit(t, root, nil, "commit", "-q", "--allow-empty", "-m", "base")
	base := sccCLIGit(t, root, nil, "rev-parse", "HEAD")
	x := sccCLICommit(t, root, base, "f.txt", "x\n", "add f")
	other := sccCLICommit(t, root, base, "g.txt", "g\n", "other work")
	rebased := sccCLICommit(t, root, other, "f.txt", "x\n", "add f")
	conflicted := sccCLICommit(t, root, base, "f.txt", "conflict\n", "add f")
	sccCLIGit(t, root, nil, "update-ref", "refs/heads/scc-deploy", rebased)
	missing := strings.Repeat("e", 40)
	before := fixture.TreeSnapshot(t, root)
	x1 := atm(t, root, nil, "source", "contains", "refs/heads/scc-deploy", x, other, base, conflicted, missing)
	if x1.res.Outcome != wire.OutcomeOK || len(x1.res.Items) != 5 {
		t.Fatalf("source contains: %s", x1.stdout)
	}
	for c, want := range map[string][3]string{
		x:          {"EQUIVALENT", rebased, ""},
		other:      {"ANCESTOR", "", ""},
		base:       {"ANCESTOR", "", ""},
		conflicted: {"ABSENT", "", "PATCH_DIFFERS"},
		missing:    {"UNKNOWN", "", "MISSING_COMMIT"},
	} {
		it := sccCLIItem(t, x1.res.Items, c)
		if got := [3]string{field(it, "state").Str, field(it, "equivalent").Str, field(it, "reason").Str}; got != want {
			t.Errorf("%s: got %v, want %v: %s", c, got, want, wire.Encode(it))
		}
	}
	if got := field(sccCLIItem(t, x1.res.Items, conflicted), "patchDiffers").Str; got != rebased {
		t.Errorf("patchDiffers = %q, want %s", got, rebased)
	}
	if after := fixture.TreeSnapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("source contains changed the repository")
	}
	if x := atm(t, root, nil, "source", "contains", "refs/heads/nope", x); field(x.res.Items[0], "reason").Str != "MISSING_SOURCE" {
		t.Fatalf("missing source: %s", x.stdout)
	}
	if x := atm(t, root, nil, "source", "contains", "HEAD"); x.res.Outcome != wire.OutcomeError {
		t.Fatalf("no commit: %s", x.stdout)
	}
	if x := atm(t, root, nil, "source", "contains", "HEAD", "--all"); x.res.Outcome != wire.OutcomeError {
		t.Fatalf("flag argument: %s", x.stdout)
	}
	many := append([]string{"source", "contains", "HEAD"}, strings.Fields(strings.Repeat(base+" ", 65))...)
	if x := atm(t, root, nil, many...); x.res.Outcome != wire.OutcomeRefused || !hasCode(x.res, wire.CodeLimitExceeded) {
		t.Fatalf("65 commits: %s", x.stdout)
	}
}

// sccHold places hold payload on ticket id.
func sccHold(t *testing.T, root, id, payload string) {
	t.Helper()
	args := []string{"ticket", "hold", "--target", id, "--expected-revision", obligationRevision(t, root, id), "--request-id", "hold-" + id + "-" + obligationRevision(t, root, id), "--payload", payload}
	if x := atm(t, root, nil, args...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("%v: %s", args, x.stdout)
	}
}

// sccStatusConditions returns queue status's holdConditions keyed by holdId.
func sccStatusConditions(t *testing.T, root string) (map[string]wire.Value, bool) {
	t.Helper()
	x := atm(t, root, nil, "queue", "status")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("queue status: %s", x.stdout)
	}
	v, ok := x.res.Items[0].Obj.Get("holdConditions")
	out := map[string]wire.Value{}
	for _, e := range v.Arr {
		out[field(e, "holdId").Str] = e
	}
	return out, ok
}
