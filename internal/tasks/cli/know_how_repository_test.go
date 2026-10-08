package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// repositoryCheckout is a second Git repository beside the store checkout,
// standing in for one repository of a multi-repository program.
func repositoryCheckout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-b", "main")
	for p, body := range map[string]string{"src/a.go": "package a\n", "src/b.go": "package b\n"} {
		fixture.Write(t, filepath.Join(root, p), []byte(body))
	}
	git(t, root, "add", "src")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "files")
	return root
}

// TestKHNV0024_RepositoryKnowHowThroughTheCLI: add --repo pins anchors in
// another repository's top level and stores them qualified with the alias;
// list and claim resolve the note at that repository's HEAD only when the
// alias is mapped, and otherwise report it UNKNOWN with a warning; reconfirm
// needs the note's own alias; a WORKER add is scoped by the qualified
// touchPaths; ambiguous or unusable repositories are refused before any
// write, and a note without --repo is stored exactly as before.
func TestKHNV0024_RepositoryKnowHowThroughTheCLI(t *testing.T) {
	r := exclusionCLIRepo(t)
	policyPath := filepath.Join(r.IntentDir, "policy.json")
	raw, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	pv, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	pv.Obj.Set("knowHow", wire.ObjectValue(wire.NewObject().Set("workerAdd", wire.Bool(true))))
	fixture.Write(t, policyPath, wire.EncodeFile(pv))
	fixture.Write(t, filepath.Join(r.Root, "src/a.go"), []byte("package a\n"))
	git(t, r.Root, "add", "src")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "files")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	e2e := repositoryCheckout(t)
	e2eRepo := "e2e=" + e2e
	t.Setenv("CORVINT_TASKS_ACTOR", "agent")
	home := planTicket(t, r.Root, "home", "P1", `["e2e/src/"]`)
	other := planTicket(t, r.Root, "other", "P2", `["src/a.go"]`)
	add := func(target, req, rev string, extra ...string) []string {
		return append([]string{"ticket", "know-how", "add", target, "--request-id", req, "--expected-revision", rev,
			"--issued-at", "2026-10-08T12:00:00Z", "--text", "start the fixture server before the e2e suite"}, extra...)
	}

	// Refusals before anything is written.
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	for name, args := range map[string][]string{
		"subdirectory root": add(home, "r-1", "1", "--repo", "e2e="+filepath.Join(e2e, "src"), "--anchor", "a.go"),
		"not a repository":  add(home, "r-2", "1", "--repo", "e2e="+t.TempDir(), "--anchor", "src/a.go"),
		"alias not a token": add(home, "r-3", "1", "--repo", "_e2e="+e2e, "--anchor", "src/a.go"),
		"repo twice":        add(home, "r-4", "1", "--repo", e2eRepo, "--repo", e2eRepo, "--anchor", "src/a.go"),
		"missing in repo":   add(home, "r-5", "1", "--repo", e2eRepo, "--anchor", "src/gone.go"),
		"store commit":      add(home, "r-6", "1", "--repo", e2eRepo, "--anchor", "src/a.go", "--commit", strings.TrimSpace(gitOutput(t, r.Root, "rev-parse", "HEAD"))),
		"duplicate list":    {"ticket", "know-how", "list", "--repo", e2eRepo, "--repo", "e2e=" + r.Root},
		"claim bad root":    {"claim", other, "--holder", "agent", "--request-id", "claim-bad", "--repo", "e2e=" + filepath.Join(e2e, "src")},
		"claim twice":       {"claim", other, "--holder", "agent", "--request-id", "claim-dup", "--repo", e2eRepo, "--repo", e2eRepo},
		"repo on release":   {"release", "--attempt", "x", "--generation", "1", "--request-id", "rel", "--repo", e2eRepo},
	} {
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("%s accepted: %s", name, x.stdout)
		}
		want := map[string]string{"repo twice": "may be given once", "missing in repo": "KNOWHOW_UNRESOLVED",
			"store commit": "does not name a commit", "repo on release": "--repo belongs to claim"}[name]
		if want == "" {
			want = "KNOWHOW_REPOSITORY"
		}
		if !strings.Contains(string(x.stdout), want) {
			t.Fatalf("%s: refusal lacks %q: %s", name, want, x.stdout)
		}
	}
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("a refused repository command wrote state")
	}

	x := atm(t, r.Root, nil, add(home, "kh-1", "1", "--repo", e2eRepo, "--anchor", "src/a.go")...)
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != mutation.OutcomeCompleted {
		t.Fatalf("repository add: %s", x.stdout)
	}
	if again := atm(t, r.Root, nil, add(home, "kh-1", "1", "--repo", e2eRepo, "--anchor", "src/a.go")...); !field(again.res.Items[0], "replayed").Bool {
		t.Fatalf("identical retry did not replay: %s", again.stdout)
	}
	if x := atm(t, r.Root, nil, add(other, "kh-2", "1", "--anchor", "src/a.go")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("plain add: %s", x.stdout)
	}
	show := string(atm(t, r.Root, nil, "ticket", "show", home).stdout)
	if !strings.Contains(show, `"path":"e2e/src/a.go"`) || !strings.Contains(show, `"repository":"e2e"`) {
		t.Fatalf("ticket show lacks the qualified note: %s", show)
	}
	if plain := string(atm(t, r.Root, nil, "ticket", "show", other).stdout); strings.Contains(plain, `"repository"`) {
		t.Fatalf("a note without --repo grew a repository key: %s", plain)
	}

	list := func(args ...string) ([]wire.Value, *wire.Result) {
		t.Helper()
		x := atm(t, r.Root, nil, append([]string{"ticket", "know-how", "list", "--ticket", home}, args...)...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("list: %s", x.stdout)
		}
		return knowHowNotes(t, x.res.Items[0]), x.res
	}
	repoOf := func(n wire.Value) (string, wire.Value) {
		rv := field(n, "repository")
		return field(rv, "alias").Str, field(rv, "head")
	}
	e2eHead := strings.TrimSpace(gitOutput(t, e2e, "rev-parse", "HEAD"))
	notes, res := list()
	if alias, head := repoOf(notes[0]); len(notes) != 1 || field(notes[0], "freshness").Str != "UNKNOWN" || alias != "e2e" || head.Kind != wire.KindNull ||
		len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "repository e2e is not mapped") {
		t.Fatalf("unmapped list: %s %v", wire.Encode(wire.Array(notes...)), res.Warnings)
	}
	notes, res = list("--repo", e2eRepo, "--path", "e2e/")
	if _, head := repoOf(notes[0]); len(notes) != 1 || field(notes[0], "freshness").Str != "CURRENT" || head.Str != e2eHead || len(res.Warnings) != 0 {
		t.Fatalf("mapped list: %s %v", wire.Encode(wire.Array(notes...)), res.Warnings)
	}
	fixture.Write(t, filepath.Join(e2e, "src/a.go"), []byte("package a // changed\n"))
	git(t, e2e, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-am", "change a")
	if notes, _ := list("--repo", e2eRepo); field(notes[0], "freshness").Str != "STALE" {
		t.Fatalf("changed blob: %s", wire.Encode(notes[0]))
	}
	reconfirm := func(req, rev string, extra ...string) []string {
		return append([]string{"ticket", "know-how", "reconfirm", home, "--request-id", req, "--expected-revision", rev,
			"--issued-at", "2026-10-08T13:00:00Z", "--note", "1"}, extra...)
	}
	for name, args := range map[string][]string{
		"no repo":     reconfirm("rc-x1", "2"),
		"other alias": reconfirm("rc-x2", "2", "--repo", "work="+e2e),
	} {
		if x := atm(t, r.Root, nil, args...); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "KNOWHOW_REPOSITORY") {
			t.Fatalf("reconfirm %s: %s", name, x.stdout)
		}
	}
	if x := atm(t, r.Root, nil, "ticket", "know-how", "reconfirm", other, "--request-id", "rc-x3", "--expected-revision", "2",
		"--note", "1", "--repo", e2eRepo); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "KNOWHOW_REPOSITORY") {
		t.Fatalf("reconfirm of a plain note with --repo: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, reconfirm("rc-1", "2", "--repo", e2eRepo)...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("reconfirm: %s", x.stdout)
	}
	e2eHead = strings.TrimSpace(gitOutput(t, e2e, "rev-parse", "HEAD"))
	if notes, _ := list("--repo", e2eRepo); field(notes[0], "freshness").Str != "CURRENT" {
		t.Fatalf("after reconfirm: %s", wire.Encode(notes[0]))
	}

	// Claim delivers the note through the qualified touchPath; the WORKER
	// add is scoped by it.
	c := atm(t, r.Root, nil, "claim", home, "--holder", "agent", "--request-id", "claim-home", "--repo", e2eRepo)
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", c.stdout)
	}
	delivered := knowHowNotes(t, field(c.res.Items[0], "knowHow"))
	if _, head := repoOf(delivered[0]); len(delivered) != 1 || field(delivered[0], "freshness").Str != "CURRENT" || head.Str != e2eHead {
		t.Fatalf("claim delivery: %s", c.stdout)
	}
	if replay := atm(t, r.Root, nil, "claim", home, "--holder", "agent", "--request-id", "claim-home"); replay.res.Outcome != wire.OutcomeOK ||
		field(knowHowNotes(t, field(replay.res.Items[0], "knowHow"))[0], "freshness").Str != "UNKNOWN" || len(replay.res.Warnings) == 0 {
		t.Fatalf("claim replay without --repo: %s", replay.stdout)
	}
	attempt, gen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	worker := func(req string, extra ...string) []string {
		return append(add(home, req, "3", "--attempt", attempt, "--generation", gen, "--role", "WORKER"), extra...)
	}
	if x := atm(t, r.Root, nil, worker("w-bare", "--anchor", "src/a.go")...); x.res.Outcome == wire.OutcomeOK ||
		!strings.Contains(string(x.stdout), mutation.KnowHowWorkerAnchorScope) {
		t.Fatalf("WORKER add of a bare path: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, worker("w-repo", "--repo", e2eRepo, "--anchor", "src/b.go")...); x.res.Outcome != wire.OutcomeOK ||
		field(x.res.Items[0], "outcome").Str != mutation.OutcomeCompleted {
		t.Fatalf("WORKER repository add: %s", x.stdout)
	}
}
