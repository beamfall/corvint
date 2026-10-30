package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// diffScopeRepo is a lease store under a policy with one required command
// gate verify, plus two commits on main: base adds src/keep.txt and
// docs/old.md, head adds src/new.go and renames docs/old.md to docs/new.md.
func diffScopeRepo(t *testing.T) (root, base, head, headTree string) {
	t.Helper()
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, _ = ergStore(t)
	gate := commandGate("verify", "/bin/sh", "-c", "exit 0") // portable: macOS has no /bin/true
	gate.Obj.Set("required", wire.Bool(true))
	ergPolicyUpdateRoles(t, root, "3", []string{"implement"}, []string{"review"}, []string{"OPERATOR", "OWNER"}, false, gate)
	write := func(path, text string) {
		t.Helper()
		fixture.Write(t, filepath.Join(root, path), []byte(text))
	}
	commit := func(msg string) string {
		t.Helper()
		git(t, root, "add", "-A", "--", "src", "docs")
		git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", msg)
		return revParse(t, root, "HEAD")
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("src/keep.txt", "keep\n")
	write("docs/old.md", "moved\n")
	base = commit("base")
	write("src/new.go", "package src\n")
	git(t, root, "mv", "docs/old.md", "docs/new.md")
	head = commit("change")
	return root, base, head, revParse(t, root, head+"^{tree}")
}

func revParse(t *testing.T, root, rev string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-parse", "--verify", rev).Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", rev, err)
	}
	return strings.TrimSpace(string(out))
}

// TestTCDV0174_DiffDerivedScopeClosesAnIncompleteTicketNatively (V1-2807):
// a ticket whose effects coverage is INCOMPLETE, with a guessed touchPath,
// is claimed with --scope-from-diff. The claim records a DERIVED scope of
// exactly the rename-free diff, both sides of the rename included, with the
// recomputable taskman-diff-scope/0 digest; submit of the head tree passes
// the scope check, the required gate runs on a clean worktree, complete
// closes the ticket COMPLETED, and the receipt audit stays OK. No
// touchPaths are written by hand and the ticket's effects are unchanged.
func TestTCDV0174_DiffDerivedScopeClosesAnIncompleteTicketNatively(t *testing.T) {
	root, base, head, headTree := diffScopeRepo(t)
	id, _ := effectsTicket(t, root, "incomplete", "P1", `{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":["elsewhere/"]}`)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim", "--base", base, "--scope-from-diff", head)
	item := c.res.Items[0]
	attempt, generation := field(item, "attemptId").Str, field(item, "generation").Str
	sc := field(item, "scope")
	want := []string{"docs/new.md", "docs/old.md", "src/new.go"}
	var got []string
	for _, r := range field(sc, "resources").Arr {
		if field(r, "class").Str != "PATH" {
			t.Fatalf("non-PATH resource: %s", c.stdout)
		}
		got = append(got, field(r, "key").Str)
	}
	if field(sc, "source").Str != "DERIVED" || strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("claim scope: want DERIVED %v, got %s", want, c.stdout)
	}
	digest := string(store.DiffScopeDigest(base, headTree, want))
	if !strings.Contains(string(c.stdout), digest) {
		attemptState := runOK("attempt", "show", attempt)
		if !strings.Contains(string(attemptState.stdout), digest) {
			t.Fatalf("derivationSha256 %s not recorded: claim %s attempt %s", digest, c.stdout, attemptState.stdout)
		}
	}
	runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", headTree, "--request-id", "submit")
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	clean := filepath.Join(dir, "clean")
	git(t, root, "worktree", "add", "--detach", clean, head)
	runOK("gate", "run", "--attempt", attempt, "--generation", generation, "--gate", "verify", "--request-id", "gate", "--worktree", clean)
	runOK("complete", "--attempt", attempt, "--generation", generation, "--request-id", "complete", "--commit", head)
	show := runOK("ticket", "show", id)
	if st := field(show.res.Items[0], "status").Str; st != "COMPLETED" {
		t.Fatalf("status %s after native complete: %s", st, show.stdout)
	}
	effects := field(field(show.res.Items[0], "record"), "effects")
	if field(effects, "coverage").Str != "INCOMPLETE" || !strings.Contains(string(wire.Encode(effects)), "elsewhere/") {
		t.Fatalf("the claim changed the ticket's effects: %s", wire.Encode(effects))
	}
	if x := atm(t, root, nil, "receipt", "audit"); x.code != 0 || x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("receipt audit: %s", x.stdout)
	}
}

// TestTCDV0175_DiffScopeNeverWidensAndFailsClosed (V1-2807): with a declared
// QUALIFIED scope the diff must fit inside it, so a changed path outside
// refuses OUT_OF_SCOPE naming it and writes nothing, while a covered diff
// keeps the DECLARED scope. An empty diff, an unresolvable revision and every
// flag combination that would mix scope sources refuse; none falls back to
// WHOLE_REPOSITORY.
func TestTCDV0175_DiffScopeNeverWidensAndFailsClosed(t *testing.T) {
	root, base, head, _ := diffScopeRepo(t)
	narrow, _ := effectsTicket(t, root, "narrow", "P1", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src/"]}`)
	before := fixture.TreeSnapshot(t, filepath.Join(root, ".git", "taskman"))
	r := atm(t, root, nil, "claim", narrow, "--holder", "tester", "--request-id", "narrow", "--base", base, "--scope-from-diff", head)
	if r.res.Outcome == wire.OutcomeOK || !hasCode(r.res, wire.CodeOutOfScope) || !strings.Contains(string(r.stdout), "docs/new.md docs/old.md") || !strings.Contains(string(r.stdout), "TCD-V0-175") {
		t.Fatalf("declared scope widened by the diff: %s", r.stdout)
	}
	if after := fixture.TreeSnapshot(t, filepath.Join(root, ".git", "taskman")); !fixture.SameTree(before, after) {
		t.Fatal("refused diff claim changed the store")
	}

	wide, _ := effectsTicket(t, root, "wide", "P1", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["docs/","src/"]}`)
	c := atm(t, root, nil, "claim", wide, "--holder", "tester", "--request-id", "wide", "--base", base, "--scope-from-diff", head)
	if c.res.Outcome != wire.OutcomeOK || field(field(c.res.Items[0], "scope"), "source").Str != "DECLARED" {
		t.Fatalf("covered diff should keep the DECLARED scope: %s", c.stdout)
	}

	open, _ := effectsTicket(t, root, "open", "P2", `{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":[]}`)
	for _, tc := range []struct {
		args []string
		code string
		text string
	}{
		{[]string{"claim", open, "--base", head, "--scope-from-diff", head}, wire.CodeMalformed, "changes no path"},
		{[]string{"claim", open, "--base", base, "--scope-from-diff", "no-such-rev"}, wire.CodeMalformed, "does not name a tree-ish"},
		{[]string{"claim", open, "--scope-from-diff", head, "--scope", "src/"}, wire.CodeMalformed, "excludes --scope"},
		{[]string{"claim", open, "--scope-from-diff", head, "--whole-repository"}, wire.CodeMalformed, "excludes --scope"},
		{[]string{"claim", "--next", "--scope-from-diff", head}, wire.CodeMalformed, "excludes --scope"},
		{[]string{"claim", open, "--scope-from-diff="}, wire.CodeMalformed, ""},
		{[]string{"renew", "--attempt", "a", "--generation", "1", "--scope-from-diff", head}, wire.CodeMalformed, "belongs to claim"},
	} {
		args := append(tc.args, "--holder", "tester", "--request-id", "refused")
		x := atm(t, root, nil, args...)
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, tc.code) || !strings.Contains(string(x.stdout), tc.text) {
			t.Fatalf("%v: want %s %q, got %s", tc.args, tc.code, tc.text, x.stdout)
		}
	}
	if h := atm(t, root, nil, "claim", "--help", "--verbose"); !strings.Contains(string(h.stdout), "--scope-from-diff") || !strings.Contains(string(h.stdout), "TCD-V0-174") {
		t.Fatalf("claim help omits --scope-from-diff: %s", h.stdout)
	}
}

// TestTCDV0181_ScopeFromDiffJoinsTheClaimRequest (V1-2832): the lease
// request names the full tree --scope-from-diff resolved to, so a replay
// under the same request id returns the stored claim only when it names the
// same tree. Another tree, including the same symbolic revision after it
// moved, refuses REQUEST_ID_CONFLICT and the stored claim still replays.
func TestTCDV0181_ScopeFromDiffJoinsTheClaimRequest(t *testing.T) {
	root, base, head, _ := diffScopeRepo(t)
	id, _ := effectsTicket(t, root, "incomplete", "P1", `{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":[]}`)
	claim := func(rev string) run {
		t.Helper()
		return atm(t, root, nil, "claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim", "--base", base, "--scope-from-diff", rev)
	}
	first := claim("HEAD")
	if first.res.Outcome != wire.OutcomeOK || field(first.res.Items[0], "replayed").Bool {
		t.Fatalf("first claim: %s", first.stdout)
	}
	attempt := field(first.res.Items[0], "attemptId").Str
	// The same tree under another name is the same request.
	for _, rev := range []string{"HEAD", head, head + "^{tree}"} {
		again := claim(rev)
		if again.res.Outcome != wire.OutcomeOK || !field(again.res.Items[0], "replayed").Bool || field(again.res.Items[0], "attemptId").Str != attempt {
			t.Fatalf("identical replay %s: want the stored claim %s, got %s", rev, attempt, again.stdout)
		}
	}
	fixture.Write(t, filepath.Join(root, "src", "more.go"), []byte("package src\n"))
	git(t, root, "add", "-A", "--", "src")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "more")
	for _, rev := range []string{"HEAD", base + "^{tree}"} {
		x := claim(rev)
		if x.res.Outcome != wire.OutcomeRefused || !hasCode(x.res, wire.CodeRequestIDConflict) {
			t.Fatalf("changed --scope-from-diff %s under the same request id: want REQUEST_ID_CONFLICT, got %s", rev, x.stdout)
		}
	}
	// The refusals changed nothing: the original tree still replays.
	if again := claim(head); !field(again.res.Items[0], "replayed").Bool || field(again.res.Items[0], "attemptId").Str != attempt {
		t.Fatalf("replay after the refusals: %s", again.stdout)
	}
}

// TestTCDV0182_ShowNamesClaimScopeFromDiffBesideSetEffects (V1-2831): an
// OPEN ticket whose only blocker is COVERAGE_UNKNOWN keeps nextAction
// set-effects, the closed CAL-V0-206 value the byNextAction facet counts, and
// names claim --scope-from-diff in alternativeActions, which that claim then
// proves valid. A ticket with qualified effects carries no such member.
func TestTCDV0182_ShowNamesClaimScopeFromDiffBesideSetEffects(t *testing.T) {
	root, base, head, _ := diffScopeRepo(t)
	open, _ := effectsTicket(t, root, "incomplete", "P1", `{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":[]}`)
	qualified, _ := effectsTicket(t, root, "qualified", "P2", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src/"]}`)
	show := func(id string) wire.Value {
		t.Helper()
		x := atm(t, root, nil, "ticket", "show", id)
		if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
			t.Fatalf("ticket show %s: %s", id, x.stdout)
		}
		return x.res.Items[0]
	}
	item := show(open)
	alt := field(item, "alternativeActions")
	if field(item, "nextAction").Str != "set-effects" || len(alt.Arr) != 1 || alt.Arr[0].Str != "claim --scope-from-diff" {
		t.Fatalf("incomplete ticket: want nextAction set-effects and alternativeActions [claim --scope-from-diff], got %s", wire.Encode(item))
	}
	if _, ok := show(qualified).Obj.Get("alternativeActions"); ok {
		t.Fatalf("qualified ticket carries alternativeActions: %s", wire.Encode(show(qualified)))
	}
	// The facet counts the same nextAction show reports, for every ticket.
	list := atm(t, root, nil, "ticket", "list", "--facets")
	want := map[string]int{}
	for _, it := range list.res.Items {
		want[field(show(field(it, "ticketId").Str), "nextAction").Str]++
	}
	next := field(facetValue(t, list.stdout), "byNextAction")
	if len(next.Obj.SortedKeys()) != len(want) {
		t.Fatalf("byNextAction %s, show counts %v", wire.Encode(next), want)
	}
	for action, n := range want {
		if got := field(next, action).Str; got != strconv.Itoa(n) {
			t.Fatalf("byNextAction %s = %q, show counts %d: %s", action, got, n, list.stdout)
		}
	}
	if c := atm(t, root, nil, "claim", open, "--holder", "tester", "--stage", "implement", "--request-id", "claim", "--base", base, "--scope-from-diff", head); c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("the named alternative is refused: %s", c.stdout)
	}
}

// TestTCDV0184_PlanPreviewNamesClaimScopeFromDiff (TCD-V0-184): a `plan
// preview` entry for an OPEN ticket whose only view blocker is
// COVERAGE_UNKNOWN carries the alternativeActions member `ticket show`
// reports for it, and an entry with qualified effects carries none.
func TestTCDV0184_PlanPreviewNamesClaimScopeFromDiff(t *testing.T) {
	root, _, _, _ := diffScopeRepo(t)
	open, _ := effectsTicket(t, root, "incomplete", "P1", `{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":[]}`)
	qualified, _ := effectsTicket(t, root, "qualified", "P2", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src/"]}`)
	x := atm(t, root, nil, "plan", "preview")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("plan preview: %s", x.stdout)
	}
	seen := 0
	for _, e := range field(x.res.Items[0], "entries").Arr {
		alt, ok := e.Obj.Get("alternativeActions")
		switch field(e, "ticketId").Str {
		case open:
			seen++
			show := atm(t, root, nil, "ticket", "show", open)
			if !ok || !bytes.Equal(wire.Encode(alt), wire.Encode(field(show.res.Items[0], "alternativeActions"))) || len(alt.Arr) != 1 || alt.Arr[0].Str != "claim --scope-from-diff" {
				t.Fatalf("incomplete entry: want show's alternativeActions, got %s / %s", wire.Encode(e), show.stdout)
			}
		case qualified:
			seen++
			if ok {
				t.Fatalf("qualified entry carries alternativeActions: %s", wire.Encode(e))
			}
		}
	}
	if seen != 2 {
		t.Fatalf("plan entries %d of 2: %s", seen, x.stdout)
	}
}
