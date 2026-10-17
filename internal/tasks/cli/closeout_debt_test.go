package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// tcdRun runs `ticket closeout-debt` and returns its exit status and envelope.
// atm is not used: the verb exits 1 on an OK result with candidates.
func tcdRun(t *testing.T, root string, args ...string) (int, *wire.Result) {
	t.Helper()
	var out bytes.Buffer
	code := cli.Run(cli.Env{Cwd: root, Args: append([]string{"ticket", "closeout-debt"}, args...), Stdout: &out})
	res, err := wire.DecodeResult(out.Bytes())
	if err != nil {
		t.Fatalf("envelope does not decode: %v\n%s", err, out.Bytes())
	}
	return code, res
}

// tcdCommit records an empty commit with message on the checked-out branch.
func tcdCommit(t *testing.T, root, message string) {
	t.Helper()
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-q", "-m", message)
}

// tcdTicket creates a ticket with title, body (empty for null) and touch
// path src/<title>/.
func tcdTicket(t *testing.T, root, title, body string) (id, local string) {
	t.Helper()
	payload := strings.NewReplacer(`"title":"Console ticket"`, `"title":"`+title+`"`, `"touchPaths":[]`, `"touchPaths":["src/`+title+`/"]`).Replace(createPayloadJSON)
	if body != "" {
		payload = strings.Replace(payload, `"body":null`, `"body":"`+body+`"`, 1)
	}
	x := atm(t, root, nil, "ticket", "create", "--request-id", "create-"+title, "--issued-at", "2026-10-17T12:00:00Z", "--payload", payload)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create %s: %s", title, x.stdout)
	}
	id = field(x.res.Items[0], "ticketId").Str
	return id, id[strings.LastIndex(id, ":")+1:]
}

type tcdRow struct{ ticket, citation, issue, step, gate, subject string }

func tcdRows(t *testing.T, res *wire.Result) (wire.Value, []tcdRow) {
	t.Helper()
	if res.Outcome != wire.OutcomeOK || len(res.Items) != 1 {
		t.Fatalf("closeout-debt: %+v", res)
	}
	item := res.Items[0]
	if field(item, "profile").Str != "taskman-closeout-debt/0" || field(item, "mutationAuthority").Bool || !res.Untrusted {
		t.Fatalf("profile, authority or trust label: %+v", item)
	}
	var rows []tcdRow
	str := func(v wire.Value) string {
		if v.Kind == wire.KindNull {
			return ""
		}
		return v.Str
	}
	for _, c := range field(item, "candidates").Arr {
		if field(c, "label").Str != "candidate" {
			t.Fatalf("label is not candidate: %+v", c)
		}
		rows = append(rows, tcdRow{field(c, "ticketId").Str, field(c, "citation").Str, str(field(c, "issue")), field(c, "closeoutStep").Str, str(field(c, "gate")), field(c, "subject").Str})
	}
	return item, rows
}

// TestTCDV0005_CandidatesThroughTheCLI covers V1-1615 acceptance (a)-(e)
// and (g): a cited ticket whose required gate failed, a cited ticket with no
// attempt reached through a merge's second parent, an umbrella commit, an
// issue-link citation, an uncited OPEN ticket left out, and byte-identical
// store and journal trees (TCD-V0-004..009).
func TestTCDV0005_CandidatesThroughTheCLI(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	verify := commandGate("verify", "/bin/sh", "-c", "exit 1")
	verify.Obj.Set("required", wire.Bool(true))
	ergPolicyUpdateRoles(t, root, "3", []string{"implement"}, []string{"review"}, []string{"OPERATOR", "OWNER"}, false, verify)

	gated, gatedLocal := tcdTicket(t, root, "gated", "")
	fresh, freshLocal := tcdTicket(t, root, "fresh", "")
	claimed, claimedLocal := tcdTicket(t, root, "claimed", "")
	linked, _ := tcdTicket(t, root, "linked", "Tracks GitHub issue #77 for the parser.")
	uncited, _ := tcdTicket(t, root, "uncited", "")

	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	c := runOK("claim", gated, "--holder", "tester", "--stage", "implement", "--request-id", "claim-gated")
	attempt, gen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", gen, "--tree", tree, "--request-id", "submit-gated")
	dir, err := os.MkdirTemp("", "tcd")
	if err == nil {
		t.Cleanup(func() { os.RemoveAll(dir) })
		dir, err = filepath.EvalSymlinks(dir)
	}
	if err != nil {
		t.Fatal(err)
	}
	clean := filepath.Join(dir, "clean")
	git(t, root, "worktree", "add", "--detach", clean, "HEAD")
	atm(t, root, nil, "gate", "run", "--attempt", attempt, "--generation", gen, "--gate", "verify", "--request-id", "gate-gated", "--worktree", clean)
	runOK("claim", claimed, "--holder", "tester", "--stage", "implement", "--request-id", "claim-claimed")

	tcdCommit(t, root, "feat: gated work\n\nRefs "+gatedLocal)
	git(t, root, "checkout", "-q", "-b", "side")
	tcdCommit(t, root, "fix: fresh work ("+freshLocal+")")
	git(t, root, "checkout", "-q", "main")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q", "--no-ff", "-m", "Merge pull request #5 from x/side", "side")
	tcdCommit(t, root, "batch: "+gatedLocal+" and "+claimedLocal)
	tcdCommit(t, root, "fix parser\n\nCloses #77")
	tcdCommit(t, root, "docs: mention "+gatedLocal+"5 and X"+freshLocal+" only")

	intentDir, stateDir := filepath.Join(root, ".taskman"), filepath.Join(root, ".git", "taskman")
	beforeIntent, beforeState := fixture.TreeSnapshot(t, intentDir), fixture.TreeSnapshot(t, stateDir)
	code, res := tcdRun(t, root)
	if !fixture.SameTree(beforeIntent, fixture.TreeSnapshot(t, intentDir)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, stateDir)) {
		t.Fatal("closeout-debt changed the intent or state tree")
	}
	if code != 1 {
		t.Fatalf("exit %d with candidates, want 1", code)
	}
	item, rows := tcdRows(t, res)
	if field(item, "ref").Str != "refs/heads/main" || field(item, "scanTruncated").Bool || field(item, "commitsScanned").Str != "7" {
		t.Fatalf("scan header: %+v", item)
	}
	byTicket := map[string][]tcdRow{}
	for i, r := range rows {
		byTicket[r.ticket] = append(byTicket[r.ticket], r)
		if i > 0 && rows[i-1].ticket > r.ticket {
			t.Fatalf("rows are not sorted by ticket ID: %+v", rows)
		}
	}
	if _, ok := byTicket[uncited]; ok {
		t.Fatalf("uncited OPEN ticket listed: %+v", rows)
	}
	want := map[string][]tcdRow{
		gated:   {{gated, "direct-id", "", "GATE_FAILED", "verify", "feat: gated work"}, {gated, "umbrella", "", "GATE_FAILED", "verify", "batch: " + gatedLocal + " and " + claimedLocal}},
		fresh:   {{fresh, "direct-id", "", "NO_ATTEMPT", "", "fix: fresh work (" + freshLocal + ")"}},
		claimed: {{claimed, "umbrella", "", "CLAIMED_NOT_SUBMITTED", "", "batch: " + gatedLocal + " and " + claimedLocal}},
		linked:  {{linked, "issue-link", "77", "NO_ATTEMPT", "", "fix parser"}},
	}
	if len(byTicket) != len(want) {
		t.Fatalf("candidates %+v, want %+v", byTicket, want)
	}
	for id, w := range want {
		got := byTicket[id]
		if len(got) != len(w) {
			t.Fatalf("%s rows %+v, want %+v", id, got, w)
		}
		for _, r := range w {
			found := false
			for _, g := range got {
				found = found || g == r
			}
			if !found {
				t.Fatalf("%s rows %+v lack %+v", id, got, r)
			}
		}
	}
	// A second run reads the same bytes.
	if _, again := tcdRun(t, root); !bytes.Equal(wire.EncodeFile(again.Items[0]), wire.EncodeFile(res.Items[0])) {
		t.Fatal("closeout-debt output is not deterministic")
	}
}

// TestTCDV0003_ClosedTicketsAndTruncation covers V1-1615 acceptance (f) and
// (h): a COMPLETED cited ticket is never listed (exit 0), and --limit 1
// truncates the scan with a warning; a commit citing the OPEN and the
// COMPLETED ticket is umbrella for the OPEN row (TCD-V0-003, TCD-V0-004,
// TCD-V0-005, TCD-V0-009).
func TestTCDV0003_ClosedTicketsAndTruncation(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	done, doneLocal := tcdTicket(t, root, "done", "")
	completeReleaseTicket(t, root, "complete-done", done)
	_, openLocal := tcdTicket(t, root, "open", "")
	tcdCommit(t, root, "Refs "+openLocal)
	tcdCommit(t, root, "batch\n\nRefs "+openLocal+" and "+doneLocal)
	tcdCommit(t, root, "feat: finished\n\nRefs "+doneLocal)

	code, res := tcdRun(t, root, "--limit", "1")
	if code != 0 {
		t.Fatalf("exit %d with no candidates, want 0", code)
	}
	item, rows := tcdRows(t, res)
	if len(rows) != 0 || !field(item, "scanTruncated").Bool || field(item, "commitsScanned").Str != "1" || field(item, "scanLimit").Str != "1" {
		t.Fatalf("truncated scan: %+v", item)
	}
	if len(res.Warnings) == 0 || !strings.Contains(strings.Join(res.Warnings, " "), "--limit 1") {
		t.Fatalf("no truncation warning: %v", res.Warnings)
	}
	code, res = tcdRun(t, root)
	if _, rows = tcdRows(t, res); code != 1 || len(rows) != 2 {
		t.Fatalf("exit %d rows %+v: want only the OPEN ticket's two commits", code, rows)
	}
	for _, r := range rows {
		want := map[string]string{"Refs " + openLocal: "direct-id", "batch": "umbrella"}[r.subject]
		if want == "" || r.citation != want || r.step != "NO_ATTEMPT" || !strings.HasSuffix(r.ticket, ":"+openLocal) {
			t.Errorf("row %+v: want the OPEN ticket, citation %q", r, want)
		}
	}
}

// TestTCDV0002_GitFailureIsUnsupported: a ref whose commit object is corrupt
// refuses UNSUPPORTED with exit 2, given as --ref or found by the default
// origin probe, while a ref that names nothing stays MALFORMED (TCD-V0-002,
// TCD-V0-009).
func TestTCDV0002_GitFailureIsUnsupported(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	tcdCommit(t, root, "base")
	out, err := exec.Command("git", "-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit-tree", "-p", "HEAD", "-m", "side", "HEAD^{tree}").Output()
	if err != nil {
		t.Fatal(err)
	}
	side := strings.TrimSpace(string(out))
	git(t, root, "update-ref", "refs/heads/side", side)
	git(t, root, "update-ref", "refs/remotes/origin/main", side)
	object := filepath.Join(root, ".git", "objects", side[:2], side[2:])
	if err := os.Chmod(object, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(object, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--ref", "side"}, {}, {"--ref", "main", "--since", "side"}} {
		code, res := tcdRun(t, root, args...)
		if code != 2 || res.Outcome == wire.OutcomeOK || !hasCode(res, wire.CodeUnsupported) {
			t.Errorf("%v: exit %d %+v, want 2 UNSUPPORTED", args, code, res)
		}
	}
	code, res := tcdRun(t, root, "--ref", "no-such-ref")
	if code != 2 || !hasCode(res, wire.CodeMalformed) {
		t.Errorf("no-such-ref: exit %d %+v, want 2 MALFORMED", code, res)
	}
}

// TestTCDV0001_ArgumentsAndRefs: --ref and --since select the range, and
// bad arguments refuse MALFORMED with exit 2 (TCD-V0-001, TCD-V0-009).
func TestTCDV0001_ArgumentsAndRefs(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	_, local := tcdTicket(t, root, "cited", "")
	tcdCommit(t, root, "Refs "+local)
	git(t, root, "branch", "before-cite", "HEAD~1")
	git(t, root, "update-ref", "refs/remotes/origin/main", "HEAD~1")

	// The default ref is origin/main when it resolves.
	code, res := tcdRun(t, root)
	if item, rows := tcdRows(t, res); code != 0 || len(rows) != 0 || field(item, "ref").Str != "refs/remotes/origin/main" {
		t.Fatalf("default ref: exit %d %+v", code, item)
	}
	if code, res = tcdRun(t, root, "--ref", "main"); code != 1 {
		t.Fatalf("--ref main: exit %d %+v", code, res)
	}
	code, res = tcdRun(t, root, "--ref", "main", "--since", "main")
	if item, rows := tcdRows(t, res); code != 0 || len(rows) != 0 || field(item, "commitsScanned").Str != "0" || field(item, "since").Kind == wire.KindNull {
		t.Fatalf("--since main: exit %d %+v", code, item)
	}
	for _, args := range [][]string{
		{"extra"}, {"--bogus", "x"}, {"--limit"}, {"--limit", "0"}, {"--limit", "10001"}, {"--limit", "x"},
		{"--ref", "--all"}, {"--ref", "-x"}, {"--ref", "no-such-ref"}, {"--since", "main\n"}, {"--ref", ""},
	} {
		code, res := tcdRun(t, root, args...)
		if code != 2 || res.Outcome == wire.OutcomeOK || !hasCode(res, wire.CodeMalformed) {
			t.Errorf("%v: exit %d %+v, want 2 MALFORMED", args, code, res)
		}
	}
}
