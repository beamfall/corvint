package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// tawRows indexes the per-ticket rows of a work verb result by ticket and
// disposition ("ticket disposition"), skipping the summary item.
func tawRows(res *wire.Result) map[string]wire.Value {
	rows := map[string]wire.Value{}
	for _, it := range res.Items {
		if d := field(it, "disposition"); d.Kind == wire.KindString {
			rows[field(it, "ticket").Str+" "+d.Str] = it
		}
	}
	return rows
}

func tawRefused(t *testing.T, x run, tag string) {
	t.Helper()
	if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), tag) {
		t.Fatalf("want a refusal naming %s: %s", tag, x.stdout)
	}
}

// tawRecord is the record of ticket show id.
func tawRecord(t *testing.T, ok func(args ...string) run, id string) wire.Value {
	t.Helper()
	return field(ok("ticket", "show", id).res.Items[0], "record")
}

// tawTrailerCommit commits on the checked-out branch with the trailers that
// work start|deliver|block printed.
func tawTrailerCommit(t *testing.T, root, subject string, x run) string {
	t.Helper()
	var lines []string
	for _, l := range field(x.res.Items[0], "trailers").Arr {
		lines = append(lines, l.Str)
	}
	tcdCommit(t, root, subject+"\n\n"+strings.Join(lines, "\n"))
	return revParse(t, root, "HEAD")
}

func tawMerge(t *testing.T, root, branch string) {
	t.Helper()
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q", "--no-ff", "-m", "merge "+branch, branch)
}

// TestTAWV0001_RoleUnknown: an unknown session class is refused by every
// work verb and by every mutation (TAW-V0-001).
func TestTAWV0001_RoleUnknown(t *testing.T) {
	root, _, _ := tlcStore(t)
	_, local := tcdTicket(t, root, "role", "")
	t.Setenv("CORVINT_TASKS_ROLE", "ROBOT")
	tawRefused(t, atm(t, root, nil, "work", "start", local), "TAW_ROLE_UNKNOWN")
	tawRefused(t, atm(t, root, nil, "work", "close-merged", "--integration", "main"), "TAW_ROLE_UNKNOWN")
	tawRefused(t, atm(t, root, nil, "ticket", "create", "--request-id", "role-create", "--payload", createPayloadJSON), "TAW_ROLE_UNKNOWN")
}

// TestTAWV0003_LaneVerbs: start, deliver and block validate against the
// local store copy, print the trailers and the git arguments, and write
// nothing (TAW-V0-002, TAW-V0-003).
func TestTAWV0003_LaneVerbs(t *testing.T) {
	root, _, ok := tlcStore(t)
	id, local := tcdTicket(t, root, "lane", "")
	t.Setenv("CORVINT_TASKS_ROLE", "AGENT")
	before := field(tawRecord(t, ok, id), "revision").Str

	x := ok("work", "deliver", local, "--lane", "taw-w15")
	item := x.res.Items[0]
	want := []string{"Corvint-Ticket: " + local, "Corvint-Ticket-State: delivered", "Corvint-Ticket-Lane: taw-w15"}
	var got []string
	for _, l := range field(item, "trailers").Arr {
		got = append(got, l.Str)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") || !field(item, "storeChecked").Bool {
		t.Fatalf("deliver trailers %q, want %q: %s", got, want, x.stdout)
	}
	if args := field(item, "gitArgs").Arr; len(args) != 6 || args[0].Str != "--trailer" || args[1].Str != want[0] {
		t.Fatalf("git args: %s", x.stdout)
	}
	ok("work", "start", local)
	tawRefused(t, atm(t, root, nil, "work", "block", local), "TAW_REASON_REQUIRED")
	b := ok("work", "block", local, "--reason", "needs internal/foo owned by lane x")
	if l := field(b.res.Items[0], "trailers").Arr; len(l) != 3 || l[2].Str != "Corvint-Ticket-Reason: needs internal/foo owned by lane x" {
		t.Fatalf("block trailers: %s", b.stdout)
	}
	tawRefused(t, atm(t, root, nil, "work", "deliver", "bad id!"), "TAW_TICKET_ID")
	tawRefused(t, atm(t, root, nil, "work", "deliver", "V1-9999"), "TAW_TICKET_UNKNOWN")
	tawRefused(t, atm(t, root, nil, "work", "block", local, "--reason", "two\nlines"), "TAW_REASON")
	u := ok("work", "deliver", "V1-9999", "--allow-unknown")
	if len(u.res.Warnings) == 0 || !strings.Contains(u.res.Warnings[0], "TAW_TICKET_UNKNOWN") {
		t.Fatalf("allow-unknown warning: %s", u.stdout)
	}
	if after := field(tawRecord(t, ok, id), "revision").Str; after != before {
		t.Fatalf("a lane verb wrote the ticket: revision %s -> %s", before, after)
	}

	// A host with no store copy checks the form only and says so.
	bare := t.TempDir()
	n := atm(t, bare, nil, "work", "start", local)
	if n.res.Outcome != wire.OutcomeOK || field(n.res.Items[0], "storeChecked").Bool || len(n.res.Warnings) == 0 || !strings.HasPrefix(n.res.Warnings[0], "STORE_NOT_PRESENT") {
		t.Fatalf("no store: %s", n.stdout)
	}
}

// TestTAWV0013_AgentGuard: under the AGENT class a bare complete-manual or
// archive is refused, a fixed reason code is accepted and prefixed, the
// status hand-edits are refused, and the store-host verbs are refused; the
// OWNER class keeps today's behavior (TAW-V0-013, TAW-V0-009).
func TestTAWV0013_AgentGuard(t *testing.T) {
	root, _, ok := tlcStore(t)
	done, _ := tcdTicket(t, root, "guarddone", "")
	gone, _ := tcdTicket(t, root, "guardgone", "")
	owner, _ := tcdTicket(t, root, "guardowner", "")
	t.Setenv("CORVINT_TASKS_ROLE", "AGENT")
	cm := func(id, req string, extra ...string) run {
		return atm(t, root, nil, append([]string{"ticket", "complete-manual", "--target", id, "--expected-revision", "1", "--request-id", req,
			"--issued-at", "2026-11-01T10:00:00Z", "--payload", `{"evidence":[],"reason":"lane says done"}`}, extra...)...)
	}
	tawRefused(t, cm(done, "bare"), "TAW_REASON_CODE_REQUIRED")
	tawRefused(t, cm(done, "badcode", "--reason-code", "BECAUSE"), "TAW_REASON_CODE_UNKNOWN")
	coded := cm(done, "coded", "--reason-code", "ALREADY_DELIVERED")
	if coded.res.Outcome != wire.OutcomeOK {
		t.Fatalf("coded complete-manual: %s", coded.stdout)
	}
	if rec := tawRecord(t, ok, done); field(rec, "status").Str != "COMPLETED" {
		t.Fatalf("status: %+v", rec)
	}
	arch := func(req string, extra ...string) run {
		return atm(t, root, nil, append([]string{"ticket", "archive", "--target", gone, "--expected-revision", "1", "--request-id", req,
			"--issued-at", "2026-11-01T10:01:00Z", "--payload", `{"reason":"stale"}`}, extra...)...)
	}
	tawRefused(t, arch("arch-bare"), "TAW_REASON_CODE_REQUIRED")
	if a := arch("arch-coded", "--reason-code", "OBSOLETE"); a.res.Outcome != wire.OutcomeOK {
		t.Fatalf("coded archive: %s", a.stdout)
	}
	tawRefused(t, atm(t, root, nil, "ticket", "restore", "--target", gone, "--expected-revision", "2", "--request-id", "restore",
		"--issued-at", "2026-11-01T10:02:00Z", "--payload", `{"reason":"back"}`), "TAW_ROLE_NOT_PERMITTED")
	tawRefused(t, atm(t, root, nil, "work", "close-merged", "--integration", "main"), "TAW_ROLE_NOT_PERMITTED")
	tawRefused(t, atm(t, root, nil, "work", "release-sync", "--release", "r1", "--milestone", "m1", "--request-id", "rs"), "TAW_ROLE_NOT_PERMITTED")

	t.Setenv("CORVINT_TASKS_ROLE", "OWNER")
	if o := cm(owner, "owner-bare"); o.res.Outcome != wire.OutcomeOK {
		t.Fatalf("owner bare complete-manual: %s", o.stdout)
	}
}

// TestTAWV0005_CloseMergedIdempotent: close-merged completes a merged
// delivery through an evidence-bearing complete-manual, records a block, and
// a rerun (with or without the cursor) writes nothing (TAW-V0-004..008).
func TestTAWV0005_CloseMergedIdempotent(t *testing.T) {
	root, _, ok := tlcStore(t)
	did, dlocal := tcdTicket(t, root, "merged", "")
	bid, blocal := tcdTicket(t, root, "blocked", "")
	base := revParse(t, root, "HEAD")
	git(t, root, "checkout", "-q", "-b", "claude/lane-a", "main")
	dc := tawTrailerCommit(t, root, "deliver merged", ok("work", "deliver", dlocal, "--lane", "lane-a"))
	tawTrailerCommit(t, root, "block blocked", ok("work", "block", blocal, "--reason", "needs a spec decision", "--lane", "lane-a"))
	tcdCommit(t, root, "an ordinary commit\n\nCorvint-Ticket-State: shipped")
	git(t, root, "checkout", "-q", "main")
	tawMerge(t, root, "claude/lane-a")

	dry := ok("work", "close-merged", "--integration", "main", "--dry-run")
	if _, found := tawRows(dry.res)[did+" WOULD_COMPLETED"]; !found {
		t.Fatalf("dry run: %s", dry.stdout)
	}
	if field(tawRecord(t, ok, did), "status").Str != "OPEN" {
		t.Fatal("the dry run wrote")
	}
	x := atm(t, root, nil, "work", "close-merged", "--integration", "main")
	rows := tawRows(x.res)
	if _, found := rows[did+" COMPLETED"]; !found {
		t.Fatalf("close-merged: %s", x.stdout)
	}
	if _, found := rows[bid+" BLOCKED"]; !found {
		t.Fatalf("block row: %s", x.stdout)
	}
	if _, found := rows[" MALFORMED"]; !found || x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("malformed trailer row: %s", x.stdout)
	}
	rec := tawRecord(t, ok, did)
	if field(rec, "status").Str != "COMPLETED" {
		t.Fatalf("delivered ticket: %+v", rec)
	}
	completion := string(wire.Encode(rec))
	if !strings.Contains(completion, "[TAW-REASON:INTEGRATED]") || !strings.Contains(completion, dc) {
		t.Fatalf("completion does not cite the commit: %s", completion)
	}
	brec := tawRecord(t, ok, bid)
	brev := field(brec, "revision").Str
	if !strings.Contains(string(wire.Encode(brec)), "needs a spec decision") {
		t.Fatalf("blocked mark: %s", wire.Encode(brec))
	}

	// Rerun with the cursor: nothing new is walked or written.
	again := ok("work", "close-merged", "--integration", "main")
	if field(again.res.Items[0], "walked").Str != "0" || len(again.res.Items) != 1 {
		t.Fatalf("cursor rerun: %s", again.stdout)
	}
	// Re-ingest from the base past the cursor: every row is a no-op.
	re := ok("work", "close-merged", "--integration", "main", "--since", base)
	rows = tawRows(re.res)
	if _, found := rows[did+" ALREADY_COMPLETED"]; !found {
		t.Fatalf("re-ingest: %s", re.stdout)
	}
	for _, it := range re.res.Items[1:] {
		if field(it, "written").Bool {
			t.Fatalf("re-ingest wrote: %s", re.stdout)
		}
	}
	if field(tawRecord(t, ok, bid), "revision").Str != brev {
		t.Fatal("re-ingest rewrote the block mark")
	}
}

// TestTAWV0007_RebasedAndDropped: a delivery on an unmerged lane ref waits
// with NOT_ON_INTEGRATION_REF; a dropped commit is reported HEAD_NOT_ON_REF;
// the rebased commit completes once it reaches the integration ref
// (TAW-V0-007).
func TestTAWV0007_RebasedAndDropped(t *testing.T) {
	root, _, ok := tlcStore(t)
	id, local := tcdTicket(t, root, "rebased", "")
	git(t, root, "checkout", "-q", "-b", "claude/lane-r", "main")
	old := tawTrailerCommit(t, root, "deliver rebased", ok("work", "deliver", local))
	git(t, root, "checkout", "-q", "main")

	x := ok("work", "close-merged", "--integration", "main", "--lane-ref", "claude/lane-r")
	if row, found := tawRows(x.res)[id+" DELIVERED_ON_BRANCH"]; !found || field(row, "reason").Str != "NOT_ON_INTEGRATION_REF" {
		t.Fatalf("lane delivery: %s", x.stdout)
	}
	if field(tawRecord(t, ok, id), "status").Str != "OPEN" {
		t.Fatal("an unmerged delivery completed")
	}
	if st, _, v := tlcShow(t, ok, id); st != "AWAITING_INTEGRATION" {
		t.Fatalf("lifecycle %s: %+v", st, v)
	}

	// The lane drops the commit (reset); the recorded head is reported.
	git(t, root, "branch", "-f", "claude/lane-r", "main")
	d := ok("work", "close-merged", "--integration", "main", "--lane-ref", "claude/lane-r")
	if row, found := tawRows(d.res)[id+" HEAD_NOT_ON_REF"]; !found || field(row, "commit").Str != old {
		t.Fatalf("dropped head: %s", d.stdout)
	}

	// The rebased commit carries the same trailers and lands on main.
	git(t, root, "checkout", "-q", "claude/lane-r")
	git(t, root, "commit", "--allow-empty", "-q", "-m", "an unrelated base change")
	nc := tawTrailerCommit(t, root, "deliver rebased again", ok("work", "deliver", local))
	git(t, root, "checkout", "-q", "main")
	tawMerge(t, root, "claude/lane-r")
	c := ok("work", "close-merged", "--integration", "main", "--lane-ref", "claude/lane-r")
	if row, found := tawRows(c.res)[id+" COMPLETED"]; !found || field(row, "commit").Str != nc {
		t.Fatalf("rebased delivery: %s", c.stdout)
	}
}

// TestTAWV0010_FindingAttachesOrCreates: a duplicate finding attaches to
// the existing ticket as a recurrence, --new-despite files it with the
// override recorded, a new finding is created, and a rerun replays
// (TAW-V0-010).
func TestTAWV0010_FindingAttachesOrCreates(t *testing.T) {
	r, ids := similarRepo(t)
	t.Setenv("CORVINT_TASKS_ROLE", "AGENT")
	dup := similarPayload("heavy offload drops untracked fixture files again", "the offload tarball omits untracked fixture files", "agent-memory")
	a := atm(t, r.Root, dup, "work", "finding", "--payload", "-", "--request-id", "f1", "--issued-at", "2026-11-01T09:00:00Z")
	if a.res.Outcome != wire.OutcomeOK || field(a.res.Items[0], "disposition").Str != "ATTACHED" || field(a.res.Items[0], "ticket").Str != ids["stash"] {
		t.Fatalf("attach: %s", a.stdout)
	}
	show := atm(t, r.Root, nil, "ticket", "show", ids["stash"])
	if !strings.Contains(string(wire.Encode(field(show.res.Items[0], "record"))), "heavy offload drops untracked fixture files again") {
		t.Fatalf("recurrence not recorded: %s", show.stdout)
	}
	if rp := atm(t, r.Root, dup, "work", "finding", "--payload", "-", "--request-id", "f1", "--issued-at", "2026-11-01T09:00:00Z"); field(rp.res.Items[0], "disposition").Str != "REPLAYED" {
		t.Fatalf("replay: %s", rp.stdout)
	}
	n := atm(t, r.Root, dup, "work", "finding", "--payload", "-", "--request-id", "f2", "--issued-at", "2026-11-01T09:01:00Z", "--new-despite", ids["stash"])
	if n.res.Outcome != wire.OutcomeOK || field(n.res.Items[0], "disposition").Str != "CREATED" || !strings.Contains(field(n.res.Items[0], "reason").Str, "filed despite similar") {
		t.Fatalf("new despite: %s", n.stdout)
	}
	created := atm(t, r.Root, nil, "ticket", "show", field(n.res.Items[0], "ticket").Str)
	if !strings.Contains(string(wire.Encode(field(created.res.Items[0], "record"))), "Filed despite similar") {
		t.Fatalf("override not recorded: %s", created.stdout)
	}
	fresh := similarPayload("console search ignores accented letters", "search for cafe misses café", "agent-memory")
	if f := atm(t, r.Root, fresh, "work", "finding", "--payload", "-", "--request-id", "f3", "--issued-at", "2026-11-01T09:02:00Z"); field(f.res.Items[0], "disposition").Str != "CREATED" {
		t.Fatalf("fresh finding: %s", f.stdout)
	}
}

// TestTAWV0011_IngestDedupesBatch: a lane's batch folds its own duplicates
// into one survivor as recurrence lines and dedupes the rest against the
// store; a rerun replays (TAW-V0-011).
func TestTAWV0011_IngestDedupesBatch(t *testing.T) {
	r, ids := similarRepo(t)
	t.Setenv("CORVINT_TASKS_ROLE", "AGENT")
	batch := "[" + string(similarPayload("dogfood bind refuses a lane branched from another lane", "prechange after change on a stacked lane", "agent-memory")) + "," +
		string(similarPayload("dogfood bind refuses a lane branched from another lane head", "prechange after change on a stacked lane branch", "agent-memory")) + "," +
		string(similarPayload("lease heartbeat expires during long gate runs again", "the claim lease lapses while a gate runs", "agent-memory")) + "]"
	path := filepath.Join(t.TempDir(), "findings.json")
	if err := os.WriteFile(path, []byte(batch), 0o600); err != nil {
		t.Fatal(err)
	}
	x := atm(t, r.Root, nil, "work", "finding", "--ingest", path, "--request-id", "batch1", "--issued-at", "2026-11-01T09:00:00Z")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 3 {
		t.Fatalf("ingest: %s", x.stdout)
	}
	disp := map[string]wire.Value{}
	for _, it := range x.res.Items {
		disp[field(it, "index").Str+" "+field(it, "disposition").Str] = it
	}
	created, ok0 := disp["0 CREATED"]
	folded, ok1 := disp["1 FOLDED"]
	attached, ok2 := disp["2 ATTACHED"]
	if !ok0 || !ok1 || !ok2 || field(folded, "ticket").Str != field(created, "ticket").Str || field(attached, "ticket").Str != ids["lease"] {
		t.Fatalf("dispositions: %s", x.stdout)
	}
	show := atm(t, r.Root, nil, "ticket", "show", field(created, "ticket").Str)
	if !strings.Contains(string(wire.Encode(field(show.res.Items[0], "record"))), "2026-11-01") {
		t.Fatalf("folded recurrence line missing: %s", show.stdout)
	}
	re := atm(t, r.Root, nil, "work", "finding", "--ingest", path, "--request-id", "batch1", "--issued-at", "2026-11-01T09:00:00Z")
	for _, it := range re.res.Items {
		if field(it, "written").Bool {
			t.Fatalf("re-ingest wrote: %s", re.stdout)
		}
	}
}

// TestTAWV0012_SupersedeArchives: a duplicate is refined with supersededBy
// and archived with the DUPLICATE reason code; a rerun is a no-op and a
// self-supersede is refused (TAW-V0-012).
func TestTAWV0012_SupersedeArchives(t *testing.T) {
	root, _, ok := tlcStore(t)
	keep, _ := tcdTicket(t, root, "keeper", "")
	dup, _ := tcdTicket(t, root, "dupe", "")
	t.Setenv("CORVINT_TASKS_ROLE", "AGENT")
	tawRefused(t, atm(t, root, nil, "work", "finding", "--supersede", dup, "--by", dup, "--request-id", "self"), "supersede itself")
	x := ok("work", "finding", "--supersede", dup, "--by", keep, "--request-id", "sup", "--issued-at", "2026-11-01T09:00:00Z")
	if field(x.res.Items[0], "disposition").Str != "SUPERSEDED" {
		t.Fatalf("supersede: %s", x.stdout)
	}
	rec := tawRecord(t, ok, dup)
	if field(rec, "status").Str != "ARCHIVED" || field(rec, "supersededBy").Str != keep {
		t.Fatalf("archived duplicate: %s", wire.Encode(rec))
	}
	if again := ok("work", "finding", "--supersede", dup, "--by", keep, "--request-id", "sup2"); field(again.res.Items[0], "disposition").Str != "ALREADY_SUPERSEDED" {
		t.Fatalf("rerun: %s", again.stdout)
	}
}

// TestTAWV0014_ReleaseSync: release-sync adds the missing OPEN milestone
// members in one membership write, reports members outside the milestone,
// and is a no-op when in sync (TAW-V0-014).
func TestTAWV0014_ReleaseSync(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), releaseQueueBytes(false))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	mk := func(req, milestone string) string {
		p := createPayloadJSON
		if milestone != "" {
			p = strings.Replace(p, `"milestone":null`, `"milestone":"`+milestone+`"`, 1)
		}
		x := atm(t, r.Root, nil, "ticket", "create", "--request-id", req, "--issued-at", "2026-09-20T12:00:00Z", "--payload", p)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("create %s: %s", req, x.stdout)
		}
		return field(x.res.Items[0], "ticketId").Str
	}
	stray := mk("stray", "")
	in1 := mk("in1", "m1")
	in2 := mk("in2", "m1")
	if x := atm(t, r.Root, nil, "release", "create", "--request-id", "rel", "--target", "v1", "--issued-at", "2026-09-20T12:01:00Z", "--payload", releaseCreatePayload("v1", "Version 1", stray, "")); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release create: %s", x.stdout)
	}
	dry := atm(t, r.Root, nil, "work", "release-sync", "--release", "v1", "--milestone", "m1", "--dry-run")
	if dry.res.Outcome != wire.OutcomeOK || field(dry.res.Items[0], "written").Bool || len(tawRows(dry.res)) != 3 {
		t.Fatalf("dry run: %s", dry.stdout)
	}
	x := atm(t, r.Root, nil, "work", "release-sync", "--release", "v1", "--milestone", "m1", "--request-id", "sync1", "--issued-at", "2026-09-20T12:02:00Z")
	rows := tawRows(x.res)
	_, a1 := rows[in1+" ADDED"]
	_, a2 := rows[in2+" ADDED"]
	_, o := rows[stray+" OUTSIDE_MILESTONE"]
	if x.res.Outcome != wire.OutcomeOK || !a1 || !a2 || !o || !field(x.res.Items[0], "written").Bool {
		t.Fatalf("sync: %s", x.stdout)
	}
	again := atm(t, r.Root, nil, "work", "release-sync", "--release", "v1", "--milestone", "m1", "--request-id", "sync2")
	if again.res.Outcome != wire.OutcomeOK || field(again.res.Items[0], "written").Bool || len(field(again.res.Items[0], "added").Arr) != 0 {
		t.Fatalf("rerun: %s", again.stdout)
	}
}
