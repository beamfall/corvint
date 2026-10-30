package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// batchGitOut runs Git in root with extra environment and returns stdout.
func batchGitOut(t *testing.T, root string, env []string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	c.Env = append(os.Environ(), env...)
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// batchCandidateTree writes HEAD's tree with path replaced by body.
func batchCandidateTree(t *testing.T, root, path, body string) string {
	t.Helper()
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	batchGitOut(t, root, env, "read-tree", "HEAD")
	c := exec.Command("git", "hash-object", "-w", "--stdin")
	c.Dir, c.Stdin = root, strings.NewReader(body)
	blob, err := c.Output()
	if err != nil {
		t.Fatal(err)
	}
	batchGitOut(t, root, env, "update-index", "--add", "--cacheinfo", "100644,"+strings.TrimSpace(string(blob))+","+path)
	return batchGitOut(t, root, env, "write-tree")
}

func batchGroupIDs(t *testing.T, x run) [][]string {
	t.Helper()
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("plan batches: %s", x.stdout)
	}
	if p := field(x.res.Items[0], "profile").Str; p != "taskman-plan-batches/0" {
		t.Fatalf("profile %q", p)
	}
	out := [][]string{}
	for _, g := range field(x.res.Items[0], "groups").Arr {
		row := []string{}
		for _, id := range field(g, "ticketIds").Arr {
			row = append(row, id.Str)
		}
		out = append(out, row)
	}
	return out
}

// NTP-V0-020 / NTP-V0-025: through the CLI, plan batches groups the
// submitted candidates of a fixture store in plan order, ignores a running
// attempt, reports an unresolvable candidate, refuses bad flags, and takes
// no lock and writes nothing: the state and intent directories, the object
// count and the refs are unchanged.
func TestNTPV0025_PlanBatchesIsAReadOnlyCLIVerb(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	for _, p := range []string{"a/x", "b/y", "c/z"} {
		fixture.Write(t, filepath.Join(root, p), []byte(p+"\n"))
	}
	git(t, root, "add", "a", "b", "c")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "files")
	ids := map[string]string{}
	for _, c := range []struct{ name, priority string }{{"a", "P1"}, {"b", "P0"}, {"c", "P2"}} {
		ids[c.name] = planTicket(t, root, "batch-"+c.name, c.priority, `["`+c.name+`/"]`)
		claim := atm(t, root, nil, "claim", ids[c.name], "--holder", "tester", "--stage", "implement", "--request-id", "claim-"+c.name)
		if claim.res.Outcome != wire.OutcomeOK {
			t.Fatalf("claim %s: %s", c.name, claim.stdout)
		}
		if c.name == "c" {
			continue // RUNNING, never a candidate
		}
		attempt, gen := field(claim.res.Items[0], "attemptId").Str, field(claim.res.Items[0], "generation").Str
		tree := batchCandidateTree(t, root, c.name+"/"+map[string]string{"a": "x", "b": "y"}[c.name], "changed "+c.name+"\n")
		if s := atm(t, root, nil, "submit", "--attempt", attempt, "--generation", gen, "--tree", tree, "--request-id", "submit-"+c.name); s.res.Outcome != wire.OutcomeOK {
			t.Fatalf("submit %s: %s", c.name, s.stdout)
		}
		ids[c.name+"-tree"] = tree
	}
	stateDir, intentDir := filepath.Join(root, ".git", "taskman"), filepath.Join(root, intent.Dir)
	state, intents := fixture.TreeSnapshot(t, stateDir), fixture.TreeSnapshot(t, intentDir)
	objects, refs := batchGitOut(t, root, nil, "count-objects", "-v"), batchGitOut(t, root, nil, "for-each-ref")
	if got, want := batchGroupIDs(t, atm(t, root, nil, "plan", "batches")), [][]string{{ids["b"], ids["a"]}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups %v, want %v", got, want)
	}
	if got, want := batchGroupIDs(t, atm(t, root, nil, "plan", "batches", "--max-group-size", "1")), [][]string{{ids["b"]}, {ids["a"]}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("singleton groups %v, want %v", got, want)
	}
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, stateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, intentDir)) {
		t.Fatal("plan batches changed the store")
	}
	if got := batchGitOut(t, root, nil, "count-objects", "-v"); got != objects {
		t.Fatalf("object count changed:\n%s\nwas\n%s", got, objects)
	}
	if got := batchGitOut(t, root, nil, "for-each-ref"); got != refs {
		t.Fatalf("refs changed:\n%s\nwas\n%s", got, refs)
	}
	// Losing a's candidate tree object leaves it ungrouped, b still grouped.
	tree := ids["a-tree"]
	if err := os.Remove(filepath.Join(root, ".git", "objects", tree[:2], tree[2:])); err != nil {
		t.Fatal(err)
	}
	x := atm(t, root, nil, "plan", "batches")
	if got, want := batchGroupIDs(t, x), [][]string{{ids["b"]}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups after loss %v, want %v", got, want)
	}
	un := field(x.res.Items[0], "ungrouped").Arr
	if len(un) != 1 || field(un[0], "ticketId").Str != ids["a"] || field(un[0], "reason").Str != wire.CodeMissingEvidence {
		t.Fatalf("ungrouped %s", x.stdout)
	}
	for _, bad := range [][]string{
		{"--max-group-size", "0"}, {"--max-group-size", "65"}, {"--max-group-size", "x"}, {"--max-group-size", "02"},
		{"--max-group-size", "2", "--max-group-size", "3"}, {"--max-group-size"}, {"--shared-path", "/abs"},
		{"--shared-path", "../up"}, {"--shared-path"}, {"--bogus"},
	} {
		if r := atm(t, root, nil, append([]string{"plan", "batches"}, bad...)...); r.res.Outcome != wire.OutcomeError {
			t.Fatalf("%v: %s", bad, r.stdout)
		}
	}
	if r := atm(t, root, nil, "plan", "batches", "--help"); r.res.Outcome != wire.OutcomeOK {
		t.Fatalf("help: %s", r.stdout)
	}
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, stateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, intentDir)) {
		t.Fatal("plan batches changed the store")
	}
}
