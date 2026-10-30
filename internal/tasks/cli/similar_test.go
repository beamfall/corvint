package cli_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// similarPayload is a CREATE payload with the given title, body and labels.
func similarPayload(title, body string, labels ...string) []byte {
	p := strings.Replace(createPayloadJSON, `"title":"Console ticket"`, `"title":"`+title+`"`, 1)
	p = strings.Replace(p, `"body":null`, `"body":"`+body+`"`, 1)
	if len(labels) > 0 {
		p = strings.Replace(p, `"labels":[]`, `"labels":["`+strings.Join(labels, `","`)+`"]`, 1)
	}
	return []byte(p)
}

func similarRepo(t *testing.T) (*fixture.Repo, map[string]string) {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	ids := map[string]string{}
	for i, tk := range []struct{ key, title, body string }{
		{"stash", "heavy offload drops untracked fixture files", "the offload tarball omits untracked fixture files so remote tests miss them"},
		{"lease", "lease heartbeat expires during long gate runs", "the claim lease lapses while a gate runs for an hour"},
		{"theme", "dark mode colours unreadable in console", "console contrast fails in dark mode"},
	} {
		labels := []string{}
		if tk.key != "theme" {
			labels = []string{"agent-memory"}
		}
		x := atm(t, r.Root, similarPayload(tk.title, tk.body, labels...), "ticket", "create", "--request-id", "seed-"+tk.key, "--issued-at", "2026-10-30T10:00:0"+string(rune('0'+i))+"Z", "--payload-stdin")
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("seed %s: %s", tk.key, x.stdout)
		}
		ids[tk.key] = field(x.res.Items[0], "ticketId").Str
	}
	return r, ids
}

// TestTCLV0001_SimilarRanksWithEvidence: ticket similar ranks OPEN tickets
// and names the matched terms (TCL-V0-001, TCL-V0-002).
func TestTCLV0001_SimilarRanksWithEvidence(t *testing.T) {
	r, ids := similarRepo(t)
	x := atm(t, r.Root, nil, "ticket", "similar", "--text", "offload drops untracked fixture files")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) == 0 || !x.res.Untrusted {
		t.Fatalf("similar: %s", x.stdout)
	}
	top := x.res.Items[0]
	if field(top, "ticketId").Str != ids["stash"] || !field(top, "duplicate").Bool {
		t.Fatalf("top candidate: %s", x.stdout)
	}
	if ev := field(top, "evidence"); len(ev.Arr) < 3 || !strings.Contains(field(top, "reason").Str, "distinct query terms") {
		t.Fatalf("evidence: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "ticket", "similar"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("similar without a query: %s", x.stdout)
	}
}

// TestTCLV0004_CreateGuard: a duplicate CREATE is refused DUPLICATE_ID and
// writes nothing; --not-duplicate-of files it; a replay is never refused.
func TestTCLV0004_CreateGuard(t *testing.T) {
	r, ids := similarRepo(t)
	dup := similarPayload("heavy offload drops untracked fixture files again", "untracked fixture files are missing on the remote", "agent-memory")
	x := atm(t, r.Root, dup, "ticket", "create", "--request-id", "dup-1", "--payload-stdin")
	if x.res.Outcome != wire.OutcomeError || len(x.res.Codes) != 1 || x.res.Codes[0] != wire.CodeDuplicateID || !bytes.Contains(x.stdout, []byte(`"retryable":false`)) {
		t.Fatalf("duplicate create: %s", x.stdout)
	}
	w := strings.Join(x.res.Warnings, " ")
	if !strings.Contains(w, "ticket recur") || !strings.Contains(w, "--not-duplicate-of") || field(x.res.Items[0], "ticketId").Str != ids["stash"] {
		t.Fatalf("refusal names neither remedy nor candidate: %s", x.stdout)
	}
	if listed := atm(t, r.Root, nil, "ticket", "list"); len(listed.res.Items) != 3 {
		t.Fatalf("a refused create wrote: %s", listed.stdout)
	}
	args := []string{"ticket", "create", "--request-id", "dup-2", "--issued-at", "2026-10-30T11:00:00Z", "--payload-stdin", "--not-duplicate-of", ids["stash"]}
	if x := atm(t, r.Root, dup, args...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("--not-duplicate-of create: %s", x.stdout)
	}
	// The replay's own ticket now matches it; the recorded request replays.
	replay := atm(t, r.Root, dup, args[:len(args)-2]...)
	if replay.res.Outcome != wire.OutcomeOK || !field(replay.res.Items[0], "replayed").Bool {
		t.Fatalf("replay: %s", replay.stdout)
	}
	if x := atm(t, r.Root, similarPayload("heavy offload drops untracked fixture files again", "untracked fixture files are missing on the remote"), "ticket", "create", "--request-id", "unlabelled", "--issued-at", "2026-10-30T11:30:00Z", "--payload-stdin"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("a create without the agent-memory label was guarded: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "ticket", "refine", "--request-id", "r", "--not-duplicate-of", "X"); x.res.Outcome == wire.OutcomeOK || !strings.Contains(strings.Join(x.res.Warnings, " "), "only ticket create") {
		t.Fatalf("refine accepted --not-duplicate-of: %s", x.stdout)
	}
}

// TestTCLV0005_DigestAndOfflineMatch: the text digest lists OPEN
// agent-memory tickets in ID order, and a lane outside any repository ranks
// against it (TCL-V0-005, TCL-V0-006).
func TestTCLV0005_DigestAndOfflineMatch(t *testing.T) {
	r, ids := similarRepo(t)
	var out bytes.Buffer
	if code := cli.Run(cli.Env{Cwd: r.Root, Args: []string{"ticket", "digest", "--format", "text"}, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: io.Discard}); code != 0 {
		t.Fatalf("text digest exit %d: %s", code, out.String())
	}
	text := out.Bytes()
	lines := strings.Split(strings.TrimSpace(string(text)), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "# corvint-ticket-digest/0 tree ") || !strings.HasSuffix(lines[0], " tickets 2") {
		t.Fatalf("digest: %q", text)
	}
	local := func(id string) string { return id[strings.LastIndex(id, ":")+1:] }
	if !strings.HasPrefix(lines[1], local(ids["stash"])+"\t") || !strings.HasPrefix(lines[2], local(ids["lease"])+"\t") {
		t.Fatalf("digest order: %q", text)
	}
	if j := atm(t, r.Root, nil, "ticket", "digest"); j.res.Outcome != wire.OutcomeOK || len(j.res.Items) != 2 {
		t.Fatalf("json digest: %s", j.stdout)
	}
	if j := atm(t, r.Root, nil, "ticket", "digest", "--since", "2099-01-01"); j.res.Outcome != wire.OutcomeOK || len(j.res.Items) != 0 {
		t.Fatalf("--since digest: %s", j.stdout)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "digest.txt")
	if err := os.WriteFile(path, text, 0o600); err != nil {
		t.Fatal(err)
	}
	off := atm(t, dir, nil, "ticket", "similar", "--digest", path, "--text", "lease heartbeat expires in gate runs")
	if off.res.Outcome != wire.OutcomeOK || len(off.res.Items) == 0 || field(off.res.Items[0], "ticketId").Str != local(ids["lease"]) {
		t.Fatalf("offline similar: %s", off.stdout)
	}
	if !strings.Contains(strings.Join(off.res.Warnings, " "), "no repository") {
		t.Fatalf("offline warnings: %s", off.stdout)
	}
	if err := os.WriteFile(path, []byte("V1-1\ttitle\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if bad := atm(t, dir, nil, "ticket", "similar", "--digest", path, "--text", "x y z"); bad.res.Outcome != wire.OutcomeError || bad.res.Codes[0] != wire.CodeMalformed {
		t.Fatalf("headerless digest: %s", bad.stdout)
	}
}

// TestTCLV0007_RecurAppendsOneLine: ticket recur appends a dated line once
// per request and refuses bad context and finished tickets (TCL-V0-007).
func TestTCLV0007_RecurAppendsOneLine(t *testing.T) {
	r, ids := similarRepo(t)
	args := []string{"ticket", "recur", ids["stash"], "--context", "lane w4 hit it again", "--request-id", "rec-1", "--issued-at", "2026-10-30T12:00:00Z"}
	if x := atm(t, r.Root, nil, args...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("recur: %s", x.stdout)
	}
	if again := atm(t, r.Root, nil, args...); again.res.Outcome != wire.OutcomeOK || !strings.Contains(strings.Join(again.res.Warnings, " "), "already recorded") {
		t.Fatalf("recur replay: %s", again.stdout)
	}
	show := atm(t, r.Root, nil, "ticket", "show", ids["stash"])
	body := field(field(show.res.Items[0], "record"), "body").Str
	if strings.Count(body, "Recurrence 2026-10-30: lane w4 hit it again") != 1 {
		t.Fatalf("body: %q", body)
	}
	if s := atm(t, r.Root, nil, "ticket", "similar", "--text", "offload drops untracked fixture files"); field(s.res.Items[0], "recurrences").Str != "1" {
		t.Fatalf("recurrence count: %s", s.stdout)
	}
	if x := atm(t, r.Root, nil, "ticket", "recur", ids["stash"], "--context", "a context it never recorded", "--request-id", "seed-stash"); x.res.Outcome != wire.OutcomeError || x.res.Codes[0] != wire.CodeRequestIDConflict {
		t.Fatalf("recur reusing the create's request ID: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "ticket", "recur", ids["lease"], "--context", "lane w4 hit it again", "--request-id", "rec-1"); x.res.Outcome != wire.OutcomeError || x.res.Codes[0] != wire.CodeRequestIDConflict {
		t.Fatalf("recur reusing a request ID on another ticket: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "ticket", "recur", ids["stash"], "--context", "two\nlines", "--request-id", "rec-2"); x.res.Outcome != wire.OutcomeError || x.res.Codes[0] != wire.CodeMalformed {
		t.Fatalf("multi-line context: %s", x.stdout)
	}
	if x := atm(t, r.Root, []byte(`{"reason":"gone"}`), "ticket", "archive", "--target", ids["theme"], "--expected-revision", "1", "--request-id", "arc", "--payload-stdin"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("archive: %s", x.stdout)
	}
	x := atm(t, r.Root, nil, "ticket", "recur", ids["theme"], "--context", "again", "--request-id", "rec-3")
	if x.res.Outcome == wire.OutcomeOK || x.res.Codes[0] != wire.CodeTicketState || !strings.Contains(strings.Join(x.res.Warnings, " "), "ticket reopen") {
		t.Fatalf("recur on archived: %s", x.stdout)
	}
}
