package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
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

// tawNoRow fails when any row of res names ticket id with disposition d.
func tawNoRow(t *testing.T, res *wire.Result, id, d string, stdout []byte) {
	t.Helper()
	if _, found := tawRows(res)[id+" "+d]; found {
		t.Fatalf("unexpected %s row for %s: %s", d, id, stdout)
	}
}

// TestTAWV0018_TrailerForgeryRefused: a trailer value that carries the walk's
// RS and US framing bytes cannot invent a commit record, so an integrated
// commit cannot complete a ticket it does not deliver (TAW-V0-018).
func TestTAWV0018_TrailerForgeryRefused(t *testing.T) {
	root, _, ok := tlcStore(t)
	victim, local := tcdTicket(t, root, "victim", "")
	git(t, root, "checkout", "-q", "-b", "claude/lane-f", "main")
	fake := strings.Repeat("ab", 20)
	tcdCommit(t, root, "forge\n\nCorvint-Ticket-State: started\nX-Note: \x1e"+fake+"\x1f1793534400\x1fCorvint-Ticket: "+local+"\nCorvint-Ticket-State: delivered")
	git(t, root, "checkout", "-q", "main")
	tawMerge(t, root, "claude/lane-f")

	for _, dry := range []bool{true, false} {
		args := []string{"work", "close-merged", "--integration", "main"}
		if dry {
			args = append(args, "--dry-run")
		}
		x := atm(t, root, nil, args...)
		tawNoRow(t, x.res, victim, "COMPLETED", x.stdout)
		tawNoRow(t, x.res, victim, "WOULD_COMPLETED", x.stdout)
	}
	if field(tawRecord(t, ok, victim), "status").Str != "OPEN" {
		t.Fatal("a forged trailer completed the ticket")
	}
}

// TestTAWV0018_PinnedRefHead: the walk reads the REF commit pinned at the
// start, so a delivery that lands on REF mid-run is not completed against
// the older commit; the next run completes it (TAW-V0-018).
func TestTAWV0018_PinnedRefHead(t *testing.T) {
	root, _, ok := tlcStore(t)
	id, local := tcdTicket(t, root, "late", "")
	pinned := revParse(t, root, "main")
	trailers := ok("work", "deliver", local, "--lane", "lane-p")
	moved := false
	restore := cli.SetCloseMergedRefResolved(func() {
		if !moved {
			moved = true
			tawTrailerCommit(t, root, "deliver late", trailers)
		}
	})
	x := ok("work", "close-merged", "--integration", "main")
	restore()
	if !moved || field(x.res.Items[0], "refCommit").Str != pinned {
		t.Fatalf("pinned run: %s", x.stdout)
	}
	tawNoRow(t, x.res, id, "COMPLETED", x.stdout)
	if field(tawRecord(t, ok, id), "status").Str != "OPEN" {
		t.Fatal("a delivery past the pinned REF commit completed")
	}
	if n := ok("work", "close-merged", "--integration", "main"); field(tawRows(n.res)[id+" COMPLETED"], "written").Bool != true {
		t.Fatalf("next run: %s", n.stdout)
	}
}

// TestTAWV0019_HeldDeliveryNotOpen: a delivered trailer for a HELD ticket is
// refused before any write, a dry run predicts the same, and the cursor
// stays so the delivery completes once the hold is released (TAW-V0-019).
func TestTAWV0019_HeldDeliveryNotOpen(t *testing.T) {
	root, _, ok := tlcStore(t)
	id, local := tcdTicket(t, root, "held", "")
	ok("ticket", "hold", "--target", id, "--expected-revision", "1", "--request-id", "hold-held",
		"--payload", `{"holdId":"owner-wait","reason":"waits for owner"}`)
	rev := field(tawRecord(t, ok, id), "revision").Str
	tawTrailerCommit(t, root, "deliver held", ok("work", "deliver", local))

	for _, args := range [][]string{{"--dry-run"}, nil, nil} {
		x := ok(append([]string{"work", "close-merged", "--integration", "main"}, args...)...)
		row, found := tawRows(x.res)[id+" NOT_OPEN"]
		if !found || field(row, "written").Bool || field(x.res.Items[0], "walked").Str == "0" {
			t.Fatalf("held delivery %v: %s", args, x.stdout)
		}
	}
	if got := field(tawRecord(t, ok, id), "revision").Str; got != rev {
		t.Fatalf("a refused delivery wrote: revision %s -> %s", rev, got)
	}
	ok("ticket", "release-hold", "--target", id, "--expected-revision", rev, "--request-id", "release-held", "--payload", `{"holdId":"owner-wait"}`)
	if x := ok("work", "close-merged", "--integration", "main"); !field(tawRows(x.res)[id+" COMPLETED"], "written").Bool {
		t.Fatalf("after release-hold: %s", x.stdout)
	}
}

// TestTAWV0020_CompetingLaneRefsStable: two lane refs that both deliver one
// ticket record one mark; reruns write nothing and keep it (TAW-V0-020).
func TestTAWV0020_CompetingLaneRefsStable(t *testing.T) {
	root, _, ok := tlcStore(t)
	id, local := tcdTicket(t, root, "twolanes", "")
	for _, lane := range []string{"claude/lane-x", "claude/lane-y"} {
		git(t, root, "checkout", "-q", "-b", lane, "main")
		tawTrailerCommit(t, root, "deliver on "+lane, ok("work", "deliver", local, "--lane", strings.TrimPrefix(lane, "claude/")))
		git(t, root, "checkout", "-q", "main")
	}
	args := []string{"work", "close-merged", "--integration", "main", "--lane-ref", "claude/lane-x", "--lane-ref", "claude/lane-y"}
	first := ok(args...)
	rows := tawRows(first.res)
	if !field(rows[id+" DELIVERED_ON_BRANCH"], "written").Bool {
		t.Fatalf("first run: %s", first.stdout)
	}
	if _, found := rows[id+" OTHER_LANE_RECORDED"]; !found {
		t.Fatalf("first run names no other lane: %s", first.stdout)
	}
	rev := field(tawRecord(t, ok, id), "revision").Str
	for i := 0; i < 2; i++ {
		again := ok(args...)
		for _, it := range again.res.Items[1:] {
			if field(it, "written").Bool {
				t.Fatalf("rerun %d wrote: %s", i, again.stdout)
			}
		}
	}
	if got := field(tawRecord(t, ok, id), "revision").Str; got != rev {
		t.Fatalf("reruns moved the mark: revision %s -> %s", rev, got)
	}
}

// TestTAWV0021_GuardCoverage: a flag value spelled --dry-run does not open a
// read-only session, an unknown class is refused by every writer, and an
// AGENT may not adopt a reconciled file (TAW-V0-016, TAW-V0-021).
func TestTAWV0021_GuardCoverage(t *testing.T) {
	root, _, _ := tlcStore(t)
	id, _ := tcdTicket(t, root, "guarded", "")
	t.Setenv("CORVINT_TASKS_READ_ONLY", "1")
	finding := similarPayload("read only bypass files a finding", "a flag value spelled dry run", "agent-memory")
	tawRefused(t, atm(t, root, finding, "work", "finding", "--payload", "-", "--request-id", "--dry-run"), "read-only session")
	tawRefused(t, atm(t, root, nil, "work", "close-merged", "--integration", "--dry-run"), "read-only session")
	tawRefused(t, atm(t, root, nil, "work", "close-merged", "--integration", "main", "--dry-run=yes"), "read-only session")
	if x := atm(t, root, nil, "work", "close-merged", "--integration", "main", "--dry-run"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("a real dry run under read-only: %s", x.stdout)
	}
	t.Setenv("CORVINT_TASKS_READ_ONLY", "")

	t.Setenv("CORVINT_TASKS_ROLE", "ROBOT")
	batch := `[{"target":"` + id + `","expectedRevision":"1","payload":{"title":"renamed"}}]`
	tawRefused(t, atm(t, root, []byte(batch), "ticket", "refine", "--batch", "--request-id", "robot-batch", "--payload-stdin"), "TAW_ROLE_UNKNOWN")
	tawRefused(t, atm(t, root, nil, "release", "create", "--request-id", "robot-rel", "--target", "r9", "--payload", "{}"), "TAW_ROLE_UNKNOWN")
	tawRefused(t, atm(t, root, []byte("{}"), "reconcile", "intent", "--target", id, "--request-id", "robot-rec", "--adopt-file", "--file", "-"), "TAW_ROLE_UNKNOWN")

	t.Setenv("CORVINT_TASKS_ROLE", "AGENT")
	tawRefused(t, atm(t, root, []byte("{}"), "reconcile", "intent", "--target", id, "--request-id", "agent-adopt", "--adopt-file", "--file", "-"), "TAW_ROLE_NOT_PERMITTED")
}

// TestTAWV0022_FindingRequestIDBound: a reused request id replays only the
// finding it recorded; another finding under it is a request conflict
// (TAW-V0-022).
func TestTAWV0022_FindingRequestIDBound(t *testing.T) {
	r, _ := similarRepo(t)
	t.Setenv("CORVINT_TASKS_ROLE", "AGENT")
	a := similarPayload("console search ignores accented letters", "search for cafe misses café", "agent-memory")
	b := similarPayload("release notes omit the upgrade steps", "the notes skip the migration", "agent-memory")
	dup := similarPayload("heavy offload drops untracked fixture files again", "the offload tarball omits untracked fixture files", "agent-memory")
	file := func(p []byte, req string) run {
		return atm(t, r.Root, p, "work", "finding", "--payload", "-", "--request-id", req, "--issued-at", "2026-11-01T09:00:00Z")
	}
	if x := file(a, "g1"); field(x.res.Items[0], "disposition").Str != "CREATED" {
		t.Fatalf("create: %s", x.stdout)
	}
	if x := file(a, "g1"); field(x.res.Items[0], "disposition").Str != "REPLAYED" {
		t.Fatalf("replay: %s", x.stdout)
	}
	tawRefused(t, file(b, "g1"), "REQUEST_ID_CONFLICT")
	if x := file(dup, "g2"); field(x.res.Items[0], "disposition").Str != "ATTACHED" {
		t.Fatalf("attach: %s", x.stdout)
	}
	tawRefused(t, file(b, "g2"), "REQUEST_ID_CONFLICT")
}

// TestTAWV0023_FoldedRecurrencesAttach: when a batch's survivor attaches to
// a store ticket, each finding folded into it is recorded there too, and
// its FOLDED row reports that write (TAW-V0-023).
func TestTAWV0023_FoldedRecurrencesAttach(t *testing.T) {
	r, ids := similarRepo(t)
	t.Setenv("CORVINT_TASKS_ROLE", "AGENT")
	t0 := "heavy offload drops untracked fixture files again"
	t1 := "heavy offload drops untracked fixture files again today"
	batch := "[" + string(similarPayload(t0, "the offload tarball omits untracked fixture files", "agent-memory")) + "," +
		string(similarPayload(t1, "the offload tarball omits untracked fixture files", "agent-memory")) + "]"
	ingest := func() run {
		return atm(t, r.Root, []byte(batch), "work", "finding", "--ingest", "-", "--request-id", "fold1", "--issued-at", "2026-11-01T09:00:00Z")
	}
	x := ingest()
	disp := map[string]wire.Value{}
	for _, it := range x.res.Items {
		disp[field(it, "index").Str+" "+field(it, "disposition").Str] = it
	}
	attached, ok0 := disp["0 ATTACHED"]
	folded, ok1 := disp["1 FOLDED"]
	if !ok0 || !ok1 || field(attached, "ticket").Str != ids["stash"] || field(folded, "ticket").Str != ids["stash"] || !field(folded, "written").Bool {
		t.Fatalf("dispositions: %s", x.stdout)
	}
	rec := string(wire.Encode(field(atm(t, r.Root, nil, "ticket", "show", ids["stash"]).res.Items[0], "record")))
	for _, want := range []string{t0, t1} {
		if !strings.Contains(rec, want) {
			t.Fatalf("stash lacks %q: %s", want, rec)
		}
	}
	re := ingest()
	for _, it := range re.res.Items {
		if field(it, "written").Bool {
			t.Fatalf("re-ingest wrote: %s", re.stdout)
		}
	}
}

// TestTAWV0024_SupersedeLongRequestID: a request id too long for either
// phase suffix still gives the refine and archive phases distinct ids, so
// the supersede completes (TAW-V0-024).
func TestTAWV0024_SupersedeLongRequestID(t *testing.T) {
	root, _, ok := tlcStore(t)
	keep, _ := tcdTicket(t, root, "longkeep", "")
	dup, _ := tcdTicket(t, root, "longdupe", "")
	t.Setenv("CORVINT_TASKS_ROLE", "AGENT")
	x := ok("work", "finding", "--supersede", dup, "--by", keep, "--request-id", strings.Repeat("a", 64), "--issued-at", "2026-11-01T09:00:00Z")
	if field(x.res.Items[0], "disposition").Str != "SUPERSEDED" {
		t.Fatalf("supersede: %s", x.stdout)
	}
	if rec := tawRecord(t, ok, dup); field(rec, "status").Str != "ARCHIVED" || field(rec, "supersededBy").Str != keep {
		t.Fatalf("archived duplicate: %s", wire.Encode(rec))
	}
}
