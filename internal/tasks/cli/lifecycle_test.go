package cli_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// tlcStore is a lease store on main whose tickets the lifecycle tests drive.
func tlcStore(t *testing.T) (root, tree string, ok func(args ...string) run) {
	t.Helper()
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree = ergStore(t)
	ok = func(args ...string) run {
		t.Helper()
		x := atm(t, root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, x.stdout)
		}
		return x
	}
	return root, tree, ok
}

// tlcDeliver records a DELIVERED_ON_BRANCH progress mark for id at branch
// and head (TLF-V0-880), as a lane reports it.
func tlcDeliver(t *testing.T, ok func(args ...string) run, id, revision, branch, head string) {
	t.Helper()
	ok("ticket", "progress", "--target", id, "--expected-revision", revision, "--request-id", "deliver-"+id[strings.LastIndex(id, ":")+1:]+"-"+revision,
		"--issued-at", "2026-10-31T12:00:00Z", "--payload", progressPayload("DELIVERED_ON_BRANCH", branch, head, "", ""))
}

// tlcLaneHead commits on a new branch from main and returns to main.
func tlcLaneHead(t *testing.T, root, branch, msg string) string {
	t.Helper()
	git(t, root, "checkout", "-q", "-b", branch, "main")
	tcdCommit(t, root, msg)
	head := revParse(t, root, "HEAD")
	git(t, root, "checkout", "-q", "main")
	return head
}

func tlcShow(t *testing.T, ok func(args ...string) run, id string) (state, reason string, v wire.Value) {
	t.Helper()
	v = field(ok("ticket", "show", id).res.Items[0], "lifecycle")
	if v.Kind != wire.KindObject {
		t.Fatalf("show %s has no lifecycle: %+v", id, v)
	}
	if r := field(v, "reason"); r.Kind == wire.KindString {
		reason = r.Str
	}
	return field(v, "state").Str, reason, v
}

func tlcWant(t *testing.T, ok func(args ...string) run, id, state, reason string) wire.Value {
	t.Helper()
	got, why, v := tlcShow(t, ok, id)
	if got != state || why != reason {
		t.Fatalf("%s: lifecycle %s/%q, want %s/%q: %+v", id, got, why, state, reason, v)
	}
	return v
}

// TestTLCV0008_ShowLifecycleTransitions drives one ticket per path through
// the derived states that ticket show reports with their evidence.
func TestTLCV0008_ShowLifecycleTransitions(t *testing.T) {
	root, tree, ok := tlcStore(t)
	worked, _ := tcdTicket(t, root, "worked", "")
	held, _ := tcdTicket(t, root, "held", "")
	delivered, _ := tcdTicket(t, root, "delivered", "")
	deleted, _ := tcdTicket(t, root, "deleted", "")
	rewritten, _ := tcdTicket(t, root, "rewritten", "")
	missing, _ := tcdTicket(t, root, "missing", "")

	v := tlcWant(t, ok, worked, "OPEN", "")
	if field(v, "attempt").Kind != wire.KindNull || field(v, "derivedFrom").Str != "none" {
		t.Fatalf("open evidence: %+v", v)
	}
	c := ok("claim", worked, "--holder", "tester", "--stage", "implement", "--request-id", "claim-worked")
	attempt, gen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	v = tlcWant(t, ok, worked, "IN_PROGRESS", "")
	if field(field(v, "attempt"), "attemptId").Str != attempt || field(v, "derivedFrom").Str != "attempt" {
		t.Fatalf("in-progress evidence: %+v", v)
	}
	ok("submit", "--attempt", attempt, "--generation", gen, "--tree", tree, "--request-id", "submit-worked")
	v = tlcWant(t, ok, worked, "SUBMITTED", "")
	if field(field(v, "attempt"), "candidateTreeOid").Str != tree {
		t.Fatalf("submitted evidence: %+v", v)
	}

	ok("ticket", "hold", "--target", held, "--expected-revision", "1", "--request-id", "hold-held",
		"--payload", `{"holdId":"owner-wait","reason":"waits for owner"}`)
	v = tlcWant(t, ok, held, "HELD", "")
	if ids := field(v, "holdIds").Arr; len(ids) != 1 || ids[0].Str != "owner-wait" {
		t.Fatalf("held evidence: %+v", v)
	}

	lane := tlcLaneHead(t, root, "claude/lane-a", "lane work")
	tlcDeliver(t, ok, delivered, "1", "claude/lane-a", lane)
	v = tlcWant(t, ok, delivered, "AWAITING_INTEGRATION", "")
	g := field(v, "git")
	if field(g, "ref").Str != "refs/heads/claude/lane-a" || field(g, "refCommit").Str != lane || field(g, "integrationRef").Str != "refs/heads/main" ||
		field(g, "integrated").Bool || !field(g, "headOnRef").Bool || field(field(v, "progress"), "head").Str != lane {
		t.Fatalf("awaiting-integration evidence: %+v", v)
	}
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q", "--no-ff", "-m", "merge lane a", "claude/lane-a")
	v = tlcWant(t, ok, delivered, "AWAITING_CLOSEOUT", "")
	if !field(field(v, "git"), "integrated").Bool || field(field(v, "git"), "integrationCommit").Str != revParse(t, root, "main") {
		t.Fatalf("awaiting-closeout evidence: %+v", v)
	}

	gone := tlcLaneHead(t, root, "claude/lane-gone", "gone work")
	tlcDeliver(t, ok, deleted, "1", "claude/lane-gone", gone)
	tlcWant(t, ok, deleted, "AWAITING_INTEGRATION", "")
	git(t, root, "branch", "-q", "-D", "claude/lane-gone")
	tlcWant(t, ok, deleted, "OPEN", "REF_DELETED")

	old := tlcLaneHead(t, root, "claude/lane-rw", "first try")
	tlcDeliver(t, ok, rewritten, "1", "claude/lane-rw", old)
	git(t, root, "branch", "-q", "-f", "claude/lane-rw", "main")
	tlcWant(t, ok, rewritten, "OPEN", "HEAD_NOT_ON_REF")

	tlcDeliver(t, ok, missing, "1", "claude/lane-rw", "abababababababababababababababababababab")
	tlcWant(t, ok, missing, "OPEN", "COMMIT_MISSING")
}

// TestTLCV0007_LifecycleReadsAreReadOnly pins that every lifecycle read
// leaves the intent tree, the state tree and the refs unchanged.
func TestTLCV0007_LifecycleReadsAreReadOnly(t *testing.T) {
	root, _, ok := tlcStore(t)
	id, _ := tcdTicket(t, root, "readonly", "")
	tlcDeliver(t, ok, id, "1", "claude/lane-r", tlcLaneHead(t, root, "claude/lane-r", "lane work"))
	ok("release", "create", "--request-id", "rel", "--target", "r1", "--issued-at", "2026-10-31T12:01:00Z",
		"--payload", fmt.Sprintf(`{"acceptanceCriteria":["c"],"predecessorReleaseIds":[],"requiredGates":[],"ticketIds":[%q],"title":"R1","version":"1"}`, id))
	r, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	intentDir := filepath.Join(r.PrimaryWorktree, ".taskman")
	refs := func() string {
		out, err := exec.Command("git", "-C", root, "for-each-ref", "--format=%(objectname) %(refname)").Output()
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}
	beforeIntent, beforeState, beforeRefs := fixture.TreeSnapshot(t, intentDir), fixture.TreeSnapshot(t, r.StateDir), refs()
	ok("ticket", "show", id)
	ok("ticket", "search", "--lifecycle", "AWAITING_INTEGRATION", "--facets")
	ok("roadmap")
	ok("release", "readiness", "r1", "--lifecycle", "AWAITING_INTEGRATION")
	if !fixture.SameTree(beforeIntent, fixture.TreeSnapshot(t, intentDir)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, r.StateDir)) || beforeRefs != refs() {
		t.Fatal("a lifecycle read changed the intent tree, the state tree or the refs")
	}
}

func tlcIDs(items []wire.Value) []string {
	var ids []string
	for _, it := range items {
		ids = append(ids, field(it, "ticketId").Str)
	}
	slices.Sort(ids)
	return ids
}

// TestTLCV0009_SearchLifecycleFilterAndFacet pins the --lifecycle filter, the
// byLifecycle facet over every state and the per-item lifecycle.
func TestTLCV0009_SearchLifecycleFilterAndFacet(t *testing.T) {
	root, _, ok := tlcStore(t)
	a, _ := tcdTicket(t, root, "alpha", "")
	b, _ := tcdTicket(t, root, "beta", "")
	tcdTicket(t, root, "gamma", "")
	tlcDeliver(t, ok, a, "1", "claude/lane-s", tlcLaneHead(t, root, "claude/lane-s", "lane work"))
	ok("claim", b, "--holder", "tester", "--stage", "implement", "--request-id", "claim-b")

	x := ok("ticket", "search", "--lifecycle", "AWAITING_INTEGRATION")
	if got := tlcIDs(x.res.Items); !slices.Equal(got, []string{a}) || field(field(x.res.Items[0], "lifecycle"), "state").Str != "AWAITING_INTEGRATION" {
		t.Fatalf("filtered search: %s", x.stdout)
	}
	all := ok("ticket", "search", "--status", "OPEN", "--facets")
	if all.res.Facets == nil {
		t.Fatalf("no facets: %s", all.stdout)
	}
	by := field(*all.res.Facets, "byLifecycle")
	if got := by.Obj.SortedKeys(); len(got) != 11 {
		t.Fatalf("byLifecycle keys %v", got)
	}
	for state, n := range map[string]string{"AWAITING_INTEGRATION": "1", "IN_PROGRESS": "1", "OPEN": "1", "UNKNOWN": "0", "COMPLETED": "0"} {
		if field(by, state).Str != n {
			t.Fatalf("byLifecycle %s = %q, want %s: %s", state, field(by, state).Str, n, all.stdout)
		}
	}
	for _, it := range all.res.Items {
		if field(it, "lifecycle").Kind != wire.KindObject {
			t.Fatalf("item without lifecycle: %+v", it)
		}
	}
	if x := atm(t, root, nil, "ticket", "search", "--lifecycle", "DELIVERED"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("unknown lifecycle state accepted: %s", x.stdout)
	}
}

// TestTLCV0010_RoadmapLifecycle pins the per-row lifecycle of the roadmap.
func TestTLCV0010_RoadmapLifecycle(t *testing.T) {
	root, _, ok := tlcStore(t)
	a, _ := tcdTicket(t, root, "alpha", "")
	tcdTicket(t, root, "beta", "")
	tlcDeliver(t, ok, a, "1", "claude/lane-m", tlcLaneHead(t, root, "claude/lane-m", "lane work"))
	x := ok("roadmap")
	states := map[string]string{}
	for _, it := range x.res.Items {
		l := field(it, "lifecycle")
		if l.Kind != wire.KindObject {
			t.Fatalf("roadmap row without lifecycle: %s", x.stdout)
		}
		states[field(it, "ticketId").Str] = field(l, "state").Str
	}
	if states[a] != "AWAITING_INTEGRATION" || len(states) != 2 {
		t.Fatalf("roadmap lifecycles %v", states)
	}
}

// TestTLCV0011_ReleaseReadinessLifecycleCounts pins memberCounts.byLifecycle
// and the --lifecycle member list.
func TestTLCV0011_ReleaseReadinessLifecycleCounts(t *testing.T) {
	root, _, ok := tlcStore(t)
	a, _ := tcdTicket(t, root, "alpha", "")
	b, _ := tcdTicket(t, root, "beta", "")
	tlcDeliver(t, ok, a, "1", "claude/lane-q", tlcLaneHead(t, root, "claude/lane-q", "lane work"))
	ok("release", "create", "--request-id", "rel", "--target", "r1", "--issued-at", "2026-10-31T12:01:00Z",
		"--payload", fmt.Sprintf(`{"acceptanceCriteria":["c"],"predecessorReleaseIds":[],"requiredGates":[],"ticketIds":[%q,%q],"title":"R1","version":"1"}`, a, b))
	item := ok("release", "readiness", "r1").res.Items[0]
	by := field(field(item, "memberCounts"), "byLifecycle")
	if len(by.Obj.SortedKeys()) != 11 || field(by, "AWAITING_INTEGRATION").Str != "1" || field(by, "OPEN").Str != "1" {
		t.Fatalf("byLifecycle: %+v", by)
	}
	if _, has := item.Obj.Get("lifecycleMembers"); has {
		t.Fatal("lifecycleMembers without --lifecycle")
	}
	item = ok("release", "readiness", "r1", "--lifecycle", "AWAITING_INTEGRATION").res.Items[0]
	m := field(item, "lifecycleMembers")
	if field(m, "count").Str != "1" || len(field(m, "ticketIds").Arr) != 1 || field(m, "ticketIds").Arr[0].Str != a {
		t.Fatalf("lifecycleMembers: %+v", m)
	}
	if x := atm(t, root, nil, "release", "readiness", "r1", "--lifecycle", "nope"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("unknown lifecycle state accepted: %s", x.stdout)
	}
}

// TestTLCV0012_DeliveredThenIntegratedIsCloseoutDebt pins the reuse path: a
// lane records DELIVERED_ON_BRANCH, its head merges, and closeout-debt lists
// the ticket from the progress head while show says AWAITING_CLOSEOUT.
func TestTLCV0012_DeliveredThenIntegratedIsCloseoutDebt(t *testing.T) {
	root, _, ok := tlcStore(t)
	id, _ := tcdTicket(t, root, "landed", "")
	head := tlcLaneHead(t, root, "claude/lane-c", "lane work without a ticket citation")
	tlcDeliver(t, ok, id, "1", "claude/lane-c", head)
	if code, res := tcdRun(t, root); code != 0 || len(field(res.Items[0], "candidates").Arr) != 0 {
		t.Fatalf("closeout-debt before integration: %d %+v", code, res)
	}
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q", "--no-ff", "-m", "merge lane c", "claude/lane-c")
	tlcWant(t, ok, id, "AWAITING_CLOSEOUT", "")
	_, rows := tcdRows(t, func() *wire.Result { _, r := tcdRun(t, root); return r }())
	found := false
	for _, r := range rows {
		if r.ticket == id && r.citation == "progress-head" {
			found = true
		}
	}
	if !found {
		t.Fatalf("closeout-debt rows %+v lack the progress-head row for %s", rows, id)
	}
}
