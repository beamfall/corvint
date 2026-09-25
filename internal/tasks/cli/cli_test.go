package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type run struct {
	code   int
	stdout []byte
	stderr []byte
	res    *wire.Result // decoded envelope (stdout, or stderr for archive export)
}

func atm(t *testing.T, cwd string, stdin []byte, args ...string) run {
	t.Helper()
	var out, errb bytes.Buffer
	code := cli.Run(cli.Env{Cwd: cwd, Args: args, Stdin: bytes.NewReader(stdin), Stdout: &out, Stderr: &errb})
	r := run{code: code, stdout: out.Bytes(), stderr: errb.Bytes()}
	env := r.stdout
	if len(args) >= 2 && args[0] == "archive" && args[1] == "export" {
		env = r.stderr
	}
	res, err := wire.DecodeResult(env)
	if err != nil {
		t.Fatalf("%v: envelope does not decode: %v\n%s", args, err, env)
	}
	r.res = res
	if (code == 0) != (res.Outcome == wire.OutcomeOK) {
		t.Errorf("%v: exit %d with outcome %s", args, code, res.Outcome)
	}
	return r
}

func hasCode(res *wire.Result, c string) bool {
	for _, x := range res.Codes {
		if x == c {
			return true
		}
	}
	return false
}

func field(v wire.Value, key string) wire.Value {
	x, _ := v.Obj.Get(key)
	return x
}

// TestTMV0008_AS07_HelpAndVersion: both are envelopes, OK, snapshot null.
func TestTMV0008_AS07_HelpAndVersion(t *testing.T) {
	r := fixture.TempRepo(t)
	for _, args := range [][]string{{}, {"help"}, {"--help"}, {"version"}} {
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK || x.res.Snapshot != nil || len(x.res.Items) != 1 {
			t.Errorf("%v: %+v", args, x.res)
		}
	}
	x := atm(t, r.Root, nil, "help")
	impl := field(x.res.Items[0], "implemented")
	if len(impl.Arr) != len(cli.ReadVerbs) {
		t.Errorf("help does not list the implemented verbs")
	}
	x = atm(t, r.Root, nil, "version")
	if field(x.res.Items[0], "verification").Str != "NOT_RUN" {
		t.Errorf("version must report verification NOT_RUN")
	}
	// Decision 0397: the version label carries the -ldflags build stamp.
	defer func(build string) { cli.Build = build }(cli.Build)
	cli.Build = "7"
	x = atm(t, r.Root, nil, "version")
	if got := field(x.res.Items[0], "version").Str; got != cli.Version+"+build.7" {
		t.Errorf("version %q does not carry the build stamp", got)
	}
	// B3 resolution (§3.3): commands that probe no store emit snapshot null:
	// usage errors before any read, and archive verify of a stream.
	for _, args := range [][]string{{"ticket"}, {"archive"}, {"frobnicate"}, {"ticket", "list", "--limit", "0"}} {
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome != wire.OutcomeError || x.res.Snapshot != nil {
			t.Errorf("%v: usage error must carry a null snapshot: %+v", args, x.res)
		}
	}
	x = atm(t, r.Root, []byte("not a tar stream"), "archive", "verify")
	if x.res.Outcome != wire.OutcomeError || x.res.Snapshot != nil {
		t.Errorf("archive verify probes no store; snapshot must be null: %+v", x.res)
	}
}

// TestTMV0008_AS07_MutationVerbsAreNotRun: every omitted verb answers
// NOT_RUN and writes nothing, even on an initialised store.
func TestTMV0008_AS07_MutationVerbsAreNotRun(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.WriteIntent(t, r, fixture.Ticket("A"))
	stateBefore := fixture.TreeSnapshot(t, r.StateDir)
	intentBefore := fixture.TreeSnapshot(t, r.IntentDir)
	for _, verb := range cli.OmittedVerbs {
		args := strings.Fields(verb)
		args = append(args, "--reason", "x", "A")
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome != wire.OutcomeNotRun || len(x.res.Codes) != 0 || len(x.res.Warnings) != 1 {
			t.Errorf("%s: %+v", verb, x.res)
		}
		fixture.AssertUntouched(t, r, stateBefore, intentBefore, verb)
	}
	x := atm(t, r.Root, nil, "frobnicate")
	if x.res.Outcome != wire.OutcomeError {
		t.Errorf("unknown verb: %+v", x.res)
	}
	x = atm(t, r.Root, nil, "ticket")
	if x.res.Outcome != wire.OutcomeError {
		t.Errorf("bare ticket: %+v", x.res)
	}
	fixture.AssertUntouched(t, r, stateBefore, intentBefore, "usage errors")
}

var readCommands = [][]string{
	{"ticket", "list"},
	{"ticket", "list", "--offset", "1", "--limit", "1"},
	{"ticket", "search", "--status", "OPEN"},
	{"ticket", "show", "A"},
	{"ticket", "blockers", "A"},
	{"ticket", "export"},
	{"queue", "status"},
	{"roadmap"},
	{"gate", "list"},
	{"gate", "show", "verify"},
	{"archive", "export"},
}

// TestTMV0008_AS07_ReadsLeaveStoreByteIdentical: every read on an
// uninitialized, pending-redo, forked, head-ahead and healthy store leaves
// the state dir and the intent tree byte/mode/mtime-identical, creates no
// lock, and reports the state verbatim.
func TestTMV0008_AS07_ReadsLeaveStoreByteIdentical(t *testing.T) {
	type state struct {
		name  string
		setup func(t *testing.T, r *fixture.Repo)
		code  string
		out   string
	}
	states := []state{
		{"no git", func(t *testing.T, r *fixture.Repo) { os.RemoveAll(r.CommonDir) }, wire.CodeUninitialized, wire.OutcomeRefused},
		{"uninitialized state dir", func(t *testing.T, r *fixture.Repo) {}, wire.CodeUninitialized, wire.OutcomeRefused},
		{"no intent store", func(t *testing.T, r *fixture.Repo) { fixture.WriteState(t, r) }, wire.CodeUninitialized, wire.OutcomeRefused},
		{"pending redo", func(t *testing.T, r *fixture.Repo) {
			fixture.WriteState(t, r)
			fixture.WriteIntent(t, r, fixture.Ticket("A"))
			fixture.PlantReceipt(t, r, 2)
		}, wire.CodeRedoPending, wire.OutcomeNotRun},
		{"forked (N4b)", func(t *testing.T, r *fixture.Repo) {
			fixture.WriteState(t, r)
			fixture.WriteIntent(t, r, fixture.Ticket("A"))
			fixture.PlantReceipt(t, r, 3)
		}, wire.CodeJournalForked, wire.OutcomeRefused},
		{"head ahead", func(t *testing.T, r *fixture.Repo) {
			fixture.WriteState(t, r)
			fixture.WriteIntent(t, r, fixture.Ticket("A"))
			fixture.Commit(t, r, "MUTATION")
			os.Remove(filepath.Join(r.StateDir, "receipts", "000000000002.json"))
		}, wire.CodeJournalForked, wire.OutcomeRefused},
		{"unsupported version", func(t *testing.T, r *fixture.Repo) {
			fixture.WriteState(t, r)
			fixture.WriteIntent(t, r, fixture.Ticket("A"))
			fixture.Write(t, filepath.Join(r.StateDir, "VERSION"), []byte("taskman-state/1\n"))
		}, wire.CodeUnsupportedVersion, wire.OutcomeError},
		{"malformed ticket", func(t *testing.T, r *fixture.Repo) {
			fixture.WriteState(t, r)
			fixture.WriteIntent(t, r, fixture.Ticket("A"))
			fixture.Write(t, filepath.Join(r.IntentDir, "tickets", "Z.json"), []byte("{}\n"))
		}, wire.CodeMalformed, wire.OutcomeError},
		{"moved primary", func(t *testing.T, r *fixture.Repo) {
			fixture.WriteState(t, r)
			fixture.WriteIntent(t, r, fixture.Ticket("A"))
			raw, _ := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
			fixture.Write(t, filepath.Join(r.StateDir, "head.json"), []byte(strings.Replace(string(raw), `"primaryWorktree":"`+r.Root+`"`, `"primaryWorktree":"/elsewhere"`, 1)))
		}, wire.CodeUnsupportedFilesystem, wire.OutcomeRefused},
	}
	for _, st := range states {
		t.Run(st.name, func(t *testing.T) {
			r := fixture.TempRepo(t)
			st.setup(t, r)
			stateBefore := fixture.TreeSnapshot(t, r.StateDir)
			intentBefore := fixture.TreeSnapshot(t, r.IntentDir)
			for _, args := range readCommands {
				x := atm(t, r.Root, nil, args...)
				if st.name == "malformed ticket" && args[0] == "archive" {
					// Export is a byte-level snapshot (TM-V0-022); it copies a
					// malformed ticket file rather than validating it.
					if x.res.Outcome != wire.OutcomeOK {
						t.Errorf("%v: %+v %v", args, x.res, x.res.Warnings)
					}
					fixture.AssertUntouched(t, r, stateBefore, intentBefore, st.name+" "+strings.Join(args, " "))
					continue
				}
				if x.res.Outcome != st.out || !hasCode(x.res, st.code) {
					t.Errorf("%v: outcome %s codes %v, want %s/%s (%v)", args, x.res.Outcome, x.res.Codes, st.out, st.code, x.res.Warnings)
				}
				if args[0] == "archive" && len(x.stdout) != 0 {
					t.Errorf("archive export wrote %d bytes to stdout on failure", len(x.stdout))
				}
				if st.code == wire.CodeRedoPending && (x.res.Snapshot == nil || !x.res.Snapshot.PendingRedo || x.res.Snapshot.HeadSeq == nil) {
					t.Errorf("%v: REDO_PENDING must carry the head it saw with pendingRedo true: %+v", args, x.res.Snapshot)
				}
				fixture.AssertUntouched(t, r, stateBefore, intentBefore, st.name+" "+strings.Join(args, " "))
			}
		})
	}
	// Healthy store: every read succeeds and still changes nothing.
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.WriteIntent(t, r, fixture.Ticket("A"), fixture.Ticket("B"))
	stateBefore := fixture.TreeSnapshot(t, r.StateDir)
	intentBefore := fixture.TreeSnapshot(t, r.IntentDir)
	tree, _ := intent.TreeDigest(r.Root)
	for _, args := range readCommands {
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK || x.res.Snapshot == nil || x.res.Snapshot.HeadSeq == nil || *x.res.Snapshot.HeadSeq != "1" {
			t.Errorf("%v: %+v %v", args, x.res, x.res.Warnings)
			continue
		}
		if x.res.Snapshot.IntentTreeSha256 == nil || *x.res.Snapshot.IntentTreeSha256 != tree.Sha256 || x.res.Snapshot.PendingRedo {
			t.Errorf("%v: snapshot does not pin the intent tree: %+v", args, x.res.Snapshot)
		}
		if x.res.Snapshot.PrimaryWorktreeSha256 == nil || *x.res.Snapshot.PrimaryWorktreeSha256 != wire.Sum([]byte(r.Root)) {
			t.Errorf("%v: primary worktree digest", args)
		}
		fixture.AssertUntouched(t, r, stateBefore, intentBefore, "healthy "+strings.Join(args, " "))
	}
	// A read from a subdirectory resolves the same store.
	sub := filepath.Join(r.Root, "src")
	os.MkdirAll(sub, 0o755)
	if x := atm(t, sub, nil, "queue", "status"); x.res.Outcome != wire.OutcomeOK {
		t.Errorf("read from subdir: %+v", x.res)
	}
	// The exported stream verifies through the CLI (stdin and file).
	x := atm(t, r.Root, nil, "archive", "export")
	if len(x.stdout) == 0 {
		t.Fatalf("no stream")
	}
	v := atm(t, r.Root, x.stdout, "archive", "verify")
	if v.res.Outcome != wire.OutcomeOK || field(v.res.Items[0], "exportedAtSeq").Str != "1" {
		t.Errorf("verify from stdin: %+v %v", v.res, v.res.Warnings)
	}
	if v.res.Snapshot != nil {
		t.Errorf("archive verify is store-independent and must emit a null snapshot (§3.3, B3): %+v", v.res.Snapshot)
	}
	outside := fixture.TempDirOutside(t)
	path := filepath.Join(outside, "a.tar")
	fixture.Write(t, path, x.stdout)
	if v = atm(t, r.Root, nil, "archive", "verify", path); v.res.Outcome != wire.OutcomeOK {
		t.Errorf("verify from file: %+v", v.res)
	}
	if v = atm(t, r.Root, x.stdout[:len(x.stdout)-1024], "archive", "verify", "-"); v.res.Outcome != wire.OutcomeError || !hasCode(v.res, wire.CodeMalformed) {
		t.Errorf("truncated verify: %+v", v.res)
	}
	if v = atm(t, r.Root, nil, "archive", "verify", filepath.Join(outside, "missing.tar")); v.res.Outcome != wire.OutcomeRefused {
		t.Errorf("missing file: %+v", v.res)
	}
	// --staging inside the repository is refused with zero stream bytes.
	x = atm(t, r.Root, nil, "archive", "export", "--staging", r.Root)
	if x.res.Outcome != wire.OutcomeRefused || !hasCode(x.res, wire.CodeUnsupportedFilesystem) || len(x.stdout) != 0 {
		t.Errorf("staging in repo: %+v (%d bytes)", x.res, len(x.stdout))
	}
	fixture.AssertUntouched(t, r, stateBefore, intentBefore, "cli export/verify")
}

// TestTMV0008_AS08_Pagination: pages pin (headSeq, intent tree digest),
// report total and truncation, keep the §4.3 order, and bound the limit.
func TestTMV0008_AS08_Pagination(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.Commit(t, r, "MUTATION")
	var recs []*ticket.Record
	for i, p := range []string{"P2", "P0", "P1", "P2", "P3"} {
		x := fixture.Ticket("T" + string(wire.CountOf(int64(i))))
		x.Priority = p
		recs = append(recs, x)
	}
	fixture.WriteIntent(t, r, recs...)
	tree, _ := intent.TreeDigest(r.Root)
	ids := func(x run) []string {
		var out []string
		for _, it := range x.res.Items {
			out = append(out, field(it, "ticketId").Str)
		}
		return out
	}
	x := atm(t, r.Root, nil, "ticket", "list")
	if x.res.Page == nil || x.res.Page.Total == nil || *x.res.Page.Total != "5" || x.res.Page.Truncated || x.res.Page.Limit != "100" || x.res.Page.Offset != "0" {
		t.Errorf("default page: %+v", x.res.Page)
	}
	want := []string{fixture.TicketID("T1"), fixture.TicketID("T2"), fixture.TicketID("T0"), fixture.TicketID("T3"), fixture.TicketID("T4")}
	if strings.Join(ids(x), ",") != strings.Join(want, ",") {
		t.Errorf("order %v", ids(x))
	}
	if !x.res.Untrusted {
		t.Errorf("items carry title prose and must be labelled UNTRUSTED_QUEUE_DATA")
	}
	x = atm(t, r.Root, nil, "ticket", "list", "--offset", "1", "--limit", "2")
	if !x.res.Page.Truncated || len(x.res.Items) != 2 || ids(x)[0] != fixture.TicketID("T2") || *x.res.Page.Total != "5" {
		t.Errorf("page 1/2: %+v %v", x.res.Page, ids(x))
	}
	if *x.res.Snapshot.HeadSeq != "2" || *x.res.Snapshot.IntentTreeSha256 != tree.Sha256 {
		t.Errorf("page does not pin (headSeq, tree digest): %+v", x.res.Snapshot)
	}
	x = atm(t, r.Root, nil, "ticket", "list", "--offset=3", "--limit=2")
	if x.res.Page.Truncated || len(x.res.Items) != 2 {
		t.Errorf("last page: %+v", x.res.Page)
	}
	x = atm(t, r.Root, nil, "ticket", "list", "--offset", "99")
	if len(x.res.Items) != 0 || x.res.Page.Truncated || x.res.Outcome != wire.OutcomeOK {
		t.Errorf("offset past end: %+v", x.res.Page)
	}
	x = atm(t, r.Root, nil, "ticket", "list", "--limit", "1000")
	if x.res.Outcome != wire.OutcomeOK {
		t.Errorf("limit at max: %+v", x.res)
	}
	x = atm(t, r.Root, nil, "ticket", "list", "--limit", "1001")
	if x.res.Outcome != wire.OutcomeError || !hasCode(x.res, wire.CodeLimitExceeded) || x.res.Snapshot != nil {
		t.Errorf("limit over max must fail before any read: %+v", x.res)
	}
	x = atm(t, r.Root, nil, "ticket", "list", "--limit", "0")
	if x.res.Outcome != wire.OutcomeError {
		t.Errorf("limit 0: %+v", x.res)
	}
	x = atm(t, r.Root, nil, "ticket", "list", "--offset", "-1")
	if x.res.Outcome != wire.OutcomeError || !hasCode(x.res, wire.CodeMalformed) {
		t.Errorf("negative offset: %+v", x.res)
	}
	x = atm(t, r.Root, nil, "ticket", "list", "--bogus", "1")
	if x.res.Outcome != wire.OutcomeError {
		t.Errorf("unknown flag: %+v", x.res)
	}
	// After a commit the pinned headSeq moves; the tree digest does not.
	fixture.Commit(t, r, "MUTATION")
	x = atm(t, r.Root, nil, "ticket", "list", "--limit", "1")
	if *x.res.Snapshot.HeadSeq != "3" || *x.res.Snapshot.IntentTreeSha256 != tree.Sha256 {
		t.Errorf("pin after commit: %+v", x.res.Snapshot)
	}
}

// TestTMV0008_AS08_ShowBlockersAndQueueStatus: show embeds the record,
// blockers reports certain blockers and unknowns, queue status counts.
func TestTMV0008_AS08_ShowBlockersAndQueueStatus(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	a := fixture.Ticket("A")
	b := fixture.Ticket("B")
	b.Dependencies = []ticket.Dependency{fixture.Dep("A"), fixture.GateDep("A", "verify")}
	h := fixture.Ticket("H")
	h.Status = "HELD"
	h.Holds = []ticket.Hold{{HoldID: "review", Actor: "op", Reason: "x", PlacedAt: fixture.Timestamp}}
	fixture.WriteIntent(t, r, a, b, h)
	x := atm(t, r.Root, nil, "ticket", "show", "B")
	if x.res.Outcome != wire.OutcomeOK || !x.res.Untrusted {
		t.Fatalf("show: %+v %v", x.res, x.res.Warnings)
	}
	it := x.res.Items[0]
	if field(it, "record").Kind != wire.KindObject || field(it, "eligibility").Str != "BLOCKED" || field(it, "nextAction").Str != "wait-dependency" {
		t.Errorf("show item: %s", wire.Encode(it))
	}
	if string(wire.EncodeFile(field(it, "record"))) != string(b.Encode()) {
		t.Errorf("embedded record differs from the file")
	}
	x = atm(t, r.Root, nil, "ticket", "blockers", fixture.TicketID("B"))
	it = x.res.Items[0]
	if field(it, "record").Kind != wire.KindNull || len(field(it, "blockers").Arr) != 1 || len(field(it, "unknowns").Arr) != 2 {
		t.Errorf("blockers item: %s", wire.Encode(it))
	}
	if field(field(it, "blockers").Arr[0], "code").Str != wire.CodeDependencyUnsatisfied {
		t.Errorf("blocker code: %s", wire.Encode(it))
	}
	x = atm(t, r.Root, nil, "ticket", "show", "NOPE")
	if x.res.Outcome != wire.OutcomeRefused || len(x.res.Items) != 0 || x.res.Snapshot == nil {
		t.Errorf("unknown ticket: %+v", x.res)
	}
	x = atm(t, r.Root, nil, "ticket", "show", "ticket:acme:other:A")
	if x.res.Outcome != wire.OutcomeError || !hasCode(x.res, wire.CodeMalformed) {
		t.Errorf("foreign queue id: %+v", x.res)
	}
	x = atm(t, r.Root, nil, "ticket", "show", "-bad")
	if x.res.Outcome != wire.OutcomeError {
		t.Errorf("bad token: %+v", x.res)
	}
	x = atm(t, r.Root, nil, "ticket", "show")
	if x.res.Outcome != wire.OutcomeError {
		t.Errorf("missing arg: %+v", x.res)
	}
	x = atm(t, r.Root, nil, "queue", "status")
	if x.res.Outcome != wire.OutcomeOK || x.res.Untrusted {
		t.Fatalf("queue status: %+v", x.res)
	}
	it = x.res.Items[0]
	by := field(it, "byStatus")
	if field(it, "tickets").Str != "3" || field(by, "OPEN").Str != "2" || field(by, "HELD").Str != "1" || field(it, "blocked").Str != "2" || field(it, "intentChecksPassed").Str != "1" {
		t.Errorf("counts: %s", wire.Encode(it))
	}
	if field(it, "publication").Str != "NOT_OBSERVED" || field(it, "attempts").Str != "NOT_OBSERVED" || field(it, "headSeq").Str != "1" {
		t.Errorf("status facts: %s", wire.Encode(it))
	}
	// Head queue differing from queue.json is MALFORMED.
	raw, _ := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
	fixture.Write(t, filepath.Join(r.StateDir, "head.json"), []byte(strings.Replace(string(raw), `"queueId":"queue:acme:main"`, `"queueId":"queue:acme:zzzz"`, 1)))
	if x = atm(t, r.Root, nil, "queue", "status"); x.res.Outcome != wire.OutcomeError || !hasCode(x.res, wire.CodeMalformed) {
		t.Errorf("queue mismatch: %+v", x.res)
	}
}

// TestTMV0008_AS08_TicketSearch: the closed filter set is a conjunction over
// inventory fields; items, order, paging and the untrusted label are those
// of `ticket list`; every filter value is validated before any read; an
// unfiltered search is a usage error.
func TestTMV0008_AS08_TicketSearch(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	owner := "russell"
	ms := "m1"
	body := "Contains the needle inside the body"
	a := fixture.Ticket("A")
	a.Priority = "P1"
	a.Owner = &owner
	a.Labels = []string{"api", "urgent"}
	b := fixture.Ticket("B")
	b.Kind = "BUG"
	b.Milestone = &ms
	b.Body = &body
	h := fixture.Ticket("H")
	h.Status = "HELD"
	h.Title = "Fixture NEEDLE title"
	h.Holds = []ticket.Hold{{HoldID: "review", Actor: "op", Reason: "x", PlacedAt: fixture.Timestamp}}
	fixture.WriteIntent(t, r, a, b, h)
	stateBefore := fixture.TreeSnapshot(t, r.StateDir)
	intentBefore := fixture.TreeSnapshot(t, r.IntentDir)
	ids := func(x run) string {
		var out []string
		for _, it := range x.res.Items {
			out = append(out, field(it, "ticketId").Str)
		}
		return strings.Join(out, ",")
	}
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--status", "OPEN"}, fixture.TicketID("A") + "," + fixture.TicketID("B")},
		{[]string{"--status", "HELD"}, fixture.TicketID("H")},
		{[]string{"--kind", "BUG"}, fixture.TicketID("B")},
		{[]string{"--priority", "P1"}, fixture.TicketID("A")},
		{[]string{"--owner", "russell"}, fixture.TicketID("A")},
		{[]string{"--milestone", "m1"}, fixture.TicketID("B")},
		{[]string{"--label", "urgent"}, fixture.TicketID("A")},
		{[]string{"--text", "needle"}, fixture.TicketID("B") + "," + fixture.TicketID("H")},
		{[]string{"--text", "needle", "--status", "HELD"}, fixture.TicketID("H")},
		{[]string{"--kind", "BUG", "--priority", "P1"}, ""},
	}
	for _, c := range cases {
		x := atm(t, r.Root, nil, append([]string{"ticket", "search"}, c.args...)...)
		if x.res.Outcome != wire.OutcomeOK || ids(x) != c.want {
			t.Errorf("search %v: outcome %s items %q, want %q (%v)", c.args, x.res.Outcome, ids(x), c.want, x.res.Warnings)
		}
		if x.res.Page == nil || x.res.Page.Total == nil || *x.res.Page.Total != wire.CountOf(int64(len(x.res.Items))) {
			t.Errorf("search %v: page total must count the matches: %+v", c.args, x.res.Page)
		}
		if x.res.Untrusted != (len(x.res.Items) > 0) {
			t.Errorf("search %v: untrusted label", c.args)
		}
		if x.res.Snapshot == nil || x.res.Snapshot.HeadSeq == nil {
			t.Errorf("search %v: snapshot must be pinned", c.args)
		}
	}
	// Items are the compact list view (record null).
	x := atm(t, r.Root, nil, "ticket", "search", "--status", "OPEN")
	if field(x.res.Items[0], "record").Kind != wire.KindNull || field(x.res.Items[0], "eligibility").Str == "" {
		t.Errorf("search item shape: %s", wire.Encode(x.res.Items[0]))
	}
	// Paging applies to the filtered set.
	x = atm(t, r.Root, nil, "ticket", "search", "--status", "OPEN", "--limit", "1")
	if len(x.res.Items) != 1 || !x.res.Page.Truncated || *x.res.Page.Total != "2" {
		t.Errorf("filtered paging: %+v", x.res.Page)
	}
	// Invalid inputs fail before any read (null snapshot).
	for _, bad := range [][]string{
		{}, {"--status", "open"}, {"--kind", "FEAT"}, {"--priority", "P9"}, {"--owner", ""},
		{"--label", strings.Repeat("x", 65)}, {"--text", ""}, {"--text", strings.Repeat("x", 513)},
		{"--text", "a\tb"}, {"--bogus", "1"}, {"A"},
	} {
		x := atm(t, r.Root, nil, append([]string{"ticket", "search"}, bad...)...)
		if x.res.Outcome != wire.OutcomeError || x.res.Snapshot != nil {
			t.Errorf("search %v must be a usage error before any read: %+v", bad, x.res)
		}
	}
	fixture.AssertUntouched(t, r, stateBefore, intentBefore, "ticket search")
}

// TestTMV0008_AS08_TicketExport: every page item carries the ticket's
// intent path, file digest, byte count and the full record, and the record
// re-encodes to the file bytes; paging is the §4.3 order with explicit
// truncation; the byte bound never cuts a record.
func TestTMV0008_AS08_TicketExport(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	var recs []*ticket.Record
	for i, p := range []string{"P2", "P0", "P1"} {
		x := fixture.Ticket("T" + string(wire.CountOf(int64(i))))
		x.Priority = p
		recs = append(recs, x)
	}
	fixture.WriteIntent(t, r, recs...)
	stateBefore := fixture.TreeSnapshot(t, r.StateDir)
	intentBefore := fixture.TreeSnapshot(t, r.IntentDir)
	x := atm(t, r.Root, nil, "ticket", "export")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 3 || !x.res.Untrusted || x.res.Page == nil || x.res.Page.Truncated {
		t.Fatalf("export: %+v %v", x.res, x.res.Warnings)
	}
	want := []string{fixture.TicketID("T1"), fixture.TicketID("T2"), fixture.TicketID("T0")}
	for i, it := range x.res.Items {
		if field(it, "ticketId").Str != want[i] {
			t.Errorf("export order at %d: %s", i, field(it, "ticketId").Str)
		}
		path := field(it, "path").Str
		raw, err := os.ReadFile(filepath.Join(r.IntentDir, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("export path %q: %v", path, err)
		}
		if string(wire.EncodeFile(field(it, "record"))) != string(raw) {
			t.Errorf("%s: record does not re-encode to the file bytes", path)
		}
		if field(it, "sha256").Str != string(wire.Sum(raw)) || field(it, "bytes").Str != string(wire.SizeOf(uint64(len(raw)))) {
			t.Errorf("%s: digest or size differ from disk", path)
		}
	}
	x = atm(t, r.Root, nil, "ticket", "export", "--offset", "1", "--limit", "1")
	if len(x.res.Items) != 1 || field(x.res.Items[0], "ticketId").Str != fixture.TicketID("T2") || !x.res.Page.Truncated || *x.res.Page.Total != "3" {
		t.Errorf("export page: %+v", x.res.Page)
	}
	x = atm(t, r.Root, nil, "ticket", "export", "--offset", "2", "--limit", "1")
	if len(x.res.Items) != 1 || x.res.Page.Truncated {
		t.Errorf("export last page: %+v", x.res.Page)
	}
	if x = atm(t, r.Root, nil, "ticket", "export", "T1"); x.res.Outcome != wire.OutcomeError || x.res.Snapshot != nil {
		t.Errorf("export takes no positional: %+v", x.res)
	}
	fixture.AssertUntouched(t, r, stateBefore, intentBefore, "ticket export")
}

// TestTMV0008_AS08_RoadmapAndGates: roadmap rows are grouped by milestone
// (none last) in §4.3 order with gate state NOT_OBSERVED; gate list renders
// every policy definition; gate show adds requiredBy and results
// NOT_OBSERVED; an unknown gate is REFUSED/GATE_UNKNOWN.
func TestTMV0008_AS08_RoadmapAndGates(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	m1, m2 := "m1", "m2"
	a := fixture.Ticket("A")
	a.Milestone = &m2
	b := fixture.Ticket("B")
	b.Milestone = &m1
	b.Priority = "P3"
	c := fixture.Ticket("C")
	c.Milestone = &m1
	c.RequiredGates = []string{"verify"}
	d := fixture.Ticket("D")
	fixture.WriteIntent(t, r, a, b, c, d)
	stateBefore := fixture.TreeSnapshot(t, r.StateDir)
	intentBefore := fixture.TreeSnapshot(t, r.IntentDir)
	x := atm(t, r.Root, nil, "roadmap")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 4 || !x.res.Untrusted || x.res.Page == nil {
		t.Fatalf("roadmap: %+v %v", x.res, x.res.Warnings)
	}
	var got []string
	for _, it := range x.res.Items {
		got = append(got, field(it, "milestone").Str+":"+field(it, "ticketId").Str)
		if field(it, "gateResults").Str != "NOT_OBSERVED" {
			t.Errorf("roadmap gate state must be NOT_OBSERVED: %s", wire.Encode(it))
		}
	}
	want := []string{"m1:" + fixture.TicketID("C"), "m1:" + fixture.TicketID("B"), "m2:" + fixture.TicketID("A"), ":" + fixture.TicketID("D")}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("roadmap order %v, want %v", got, want)
	}
	if field(x.res.Items[3], "milestone").Kind != wire.KindNull {
		t.Errorf("a ticket without milestone renders null")
	}
	if len(field(x.res.Items[0], "requiredGates").Arr) != 1 {
		t.Errorf("roadmap row must list requiredGates: %s", wire.Encode(x.res.Items[0]))
	}
	x = atm(t, r.Root, nil, "roadmap", "--offset", "3", "--limit", "1")
	if len(x.res.Items) != 1 || x.res.Page.Truncated || *x.res.Page.Total != "4" {
		t.Errorf("roadmap page: %+v", x.res.Page)
	}
	if x = atm(t, r.Root, nil, "roadmap", "extra"); x.res.Outcome != wire.OutcomeError {
		t.Errorf("roadmap takes no positional: %+v", x.res)
	}
	x = atm(t, r.Root, nil, "gate", "list")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 || x.res.Untrusted || x.res.Page != nil {
		t.Fatalf("gate list: %+v", x.res)
	}
	g := x.res.Items[0]
	if field(g, "gateId").Str != "verify" || field(g, "kind").Str != "COMMAND" || len(field(g, "argv").Arr) != 2 || field(g, "timeoutSeconds").Str != "1800" || !field(g, "required").Bool {
		t.Errorf("gate definition: %s", wire.Encode(g))
	}
	if _, ok := g.Obj.Get("results"); ok {
		t.Errorf("gate list carries definitions only")
	}
	x = atm(t, r.Root, nil, "gate", "show", "verify")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("gate show: %+v %v", x.res, x.res.Warnings)
	}
	g = x.res.Items[0]
	if field(g, "results").Str != "NOT_OBSERVED" || len(field(g, "requiredBy").Arr) != 1 || field(g, "requiredBy").Arr[0].Str != fixture.TicketID("C") {
		t.Errorf("gate show: %s", wire.Encode(g))
	}
	x = atm(t, r.Root, nil, "gate", "show", "nope")
	if x.res.Outcome != wire.OutcomeRefused || !hasCode(x.res, wire.CodeGateUnknown) || len(x.res.Items) != 0 || x.res.Snapshot == nil {
		t.Errorf("unknown gate: %+v", x.res)
	}
	for _, bad := range [][]string{{"gate"}, {"gate", "show"}, {"gate", "show", "a", "b"}, {"gate", "show", "--x"}, {"gate", "list", "x"}, {"gate", "frob"}} {
		if x := atm(t, r.Root, nil, bad...); x.res.Outcome != wire.OutcomeError || x.res.Snapshot != nil {
			t.Errorf("%v: usage error expected: %+v", bad, x.res)
		}
	}
	fixture.AssertUntouched(t, r, stateBefore, intentBefore, "roadmap and gates")
}
