package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// copyIntentDir copies the flat intent store src into dst/.taskman.
func copyIntentDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		to := filepath.Join(dst, ".taskman", rel)
		if fi.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(to, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestTCLV0869_SnapshotValidatesAWorktreeStore: queue snapshot --worktree
// reads that directory's intent tree rather than the primary checkout's,
// so a pruned worktree store's missing dependency is reported through the
// snapshot reads while the primary store still reads clean; receipt audit
// stays refused, a directory without a store is UNINITIALIZED and a
// malformed record writes nothing.
func TestTCLV0869_SnapshotValidatesAWorktreeStore(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	create := func(req, title, dep string) string {
		t.Helper()
		payload := strings.Replace(createPayloadJSON, `"title":"Console ticket"`, `"title":"`+title+`"`, 1)
		if dep != "" {
			payload = strings.Replace(payload, `"dependencies":[]`, `"dependencies":[{"gateId":null,"obligation":"COMPLETED","ticketId":"`+dep+`"}]`, 1)
		}
		x := atm(t, r.Root, nil, "ticket", "create", "--request-id", req, "--issued-at", "2026-10-07T12:00:00Z", "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("create %s: %s", title, x.stdout)
		}
		return field(x.res.Items[0], "ticketId").Str
	}
	dep := create("req-dep", "dependency", "")
	child := create("req-child", "child", dep)
	if x := atm(t, r.Root, nil, "ticket", "blockers", child); x.res.Outcome != wire.OutcomeOK || bytes.Contains(x.stdout, []byte(wire.CodeDependencyMissing)) {
		t.Fatalf("primary blockers: %s", x.stdout)
	}

	file := func(id string) string { return id[strings.LastIndex(id, ":")+1:] + ".json" }
	wt := t.TempDir()
	copyIntentDir(t, r.IntentDir, wt)
	if err := os.Remove(filepath.Join(wt, ".taskman", "tickets", file(dep))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "wt-snapshot.json")
	x := atm(t, r.Root, nil, "queue", "snapshot", "--worktree", wt, "--out", path)
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "lastSeq").Str != "NOT_OBSERVED" {
		t.Fatalf("snapshot --worktree: %s", x.stdout)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var b struct {
		PrimaryWorktree string
		Journal         *json.RawMessage
		Files           []struct{ Path string }
	}
	if err := json.Unmarshal(raw, &b); err != nil || b.PrimaryWorktree != wt || b.Journal != nil || strings.Contains(string(raw), file(dep)) {
		t.Fatalf("worktree snapshot: %v %+v", err, b)
	}

	lane := t.TempDir()
	t.Setenv(cli.SnapshotEnv, path)
	for _, args := range [][]string{{"ticket", "blockers", child}, {"ticket", "show", child}} {
		if x := atm(t, lane, nil, args...); x.res.Outcome != wire.OutcomeOK || !bytes.Contains(x.stdout, []byte(wire.CodeDependencyMissing)) || !strings.Contains(strings.Join(x.res.Warnings, " "), wt) {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	if x := atm(t, lane, nil, "receipt", "audit"); x.res.Outcome == wire.OutcomeOK || len(x.res.Codes) != 1 || x.res.Codes[0] != wire.CodeUnsupported {
		t.Fatalf("receipt audit under a worktree snapshot: %s", x.stdout)
	}
	os.Unsetenv(cli.SnapshotEnv)

	if x := atm(t, r.Root, nil, "queue", "snapshot", "--worktree", t.TempDir()); x.res.Outcome == wire.OutcomeOK || x.res.Codes[0] != wire.CodeUninitialized {
		t.Fatalf("no store: %s", x.stdout)
	}
	bad := t.TempDir()
	copyIntentDir(t, r.IntentDir, bad)
	fixture.Write(t, filepath.Join(bad, ".taskman", "tickets", file(child)), []byte("{\"not\":\"a ticket\"}\n"))
	badOut := filepath.Join(t.TempDir(), "bad.json")
	if x := atm(t, r.Root, nil, "queue", "snapshot", "--worktree", bad, "--out", badOut); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("malformed worktree store: %s", x.stdout)
	}
	if _, err := os.Stat(badOut); !os.IsNotExist(err) {
		t.Fatalf("a refused snapshot wrote %s: %v", badOut, err)
	}
}

// TCL-V0-7102: audit the explicitly selected projection against its owning
// journal. A divergent copy must not be silently replaced by the primary's.
func TestTCLV07102_AuditExplicitWorktreeProjection(t *testing.T) {
	r, local, _ := papercutRepo(t)
	wt := t.TempDir()
	copyIntentDir(t, r.IntentDir, wt)
	canonical, err := filepath.EvalSymlinks(wt)
	if err != nil {
		t.Fatal(err)
	}
	before := fixture.TreeSnapshot(t, r.CommonDir)
	help := atm(t, r.Root, nil, "receipt", "audit", "--help")
	if !strings.Contains(string(help.stdout), "owning journal") || !strings.Contains(string(help.stdout), "receipt audit --worktree ../candidate") {
		t.Fatalf("worktree help: %s", help.stdout)
	}
	good := atm(t, r.Root, nil, "receipt", "audit", "--worktree", wt)
	if good.code != 0 || len(good.res.Items) != 1 || field(good.res.Items[0], "projectionAgreement").Str != "AGREES" || field(good.res.Items[0], "worktree").Str != canonical {
		t.Fatalf("explicit projection: %s", good.stdout)
	}
	file := filepath.Join(wt, ".taskman", "tickets", local+".json")
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.Replace(raw, []byte("Console ticket"), []byte("Changed ticket"), 1)
	fixture.Write(t, file, raw)
	bad := atm(t, r.Root, nil, "receipt", "audit", "--worktree", wt)
	if bad.code == 0 || !hasCode(bad.res, wire.CodeIntentDiverged) {
		t.Fatalf("divergent projection hidden: %s", bad.stdout)
	}
	if text := string(bad.stdout); !strings.Contains(text, "receipt audit --worktree selected projection") || !strings.Contains(text, canonical) || !strings.Contains(text, "select an agreeing projection with --worktree DIR") || strings.Contains(text, "linked intent worktree") || strings.Contains(text, "commit or carry") {
		t.Fatalf("wrong explicit-worktree repair: %s", bad.stdout)
	}
	if x := atm(t, r.Root, nil, "receipt", "audit"); x.code != 0 {
		t.Fatalf("primary audit: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "receipt", "audit", "--through", "1", "--worktree", wt); !hasCode(x.res, wire.CodeMalformed) {
		t.Fatalf("partial projection validation admitted: %s", x.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.CommonDir)) {
		t.Fatal("read changed journal")
	}
}

// TCL-V0-7102: ancestor aliases remain usable, but DIR itself must not be
// a symlink. Resolving the leaf before checking it would hide this refusal.
func TestTCLV07102_RefusesSymlinkedWorktree(t *testing.T) {
	r, _, _ := papercutRepo(t)
	parent := t.TempDir()
	wt := filepath.Join(parent, "candidate")
	copyIntentDir(t, r.IntentDir, wt)
	link := filepath.Join(t.TempDir(), "candidate-link")
	if err := os.Symlink(wt, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	before := fixture.TreeSnapshot(t, r.CommonDir)
	bad := atm(t, r.Root, nil, "receipt", "audit", "--worktree", link)
	if bad.code == 0 || !hasCode(bad.res, wire.CodeUnsupportedFilesystem) {
		t.Fatalf("symlinked DIR admitted: %s", bad.stdout)
	}
	alias := filepath.Join(t.TempDir(), "parent-alias")
	if err := os.Symlink(parent, alias); err != nil {
		t.Fatal(err)
	}
	good := atm(t, r.Root, nil, "receipt", "audit", "--worktree", filepath.Join(alias, "candidate"))
	if good.code != 0 || len(good.res.Items) != 1 || field(good.res.Items[0], "projectionAgreement").Str != "AGREES" {
		t.Fatalf("ancestor alias refused: %s", good.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.CommonDir)) {
		t.Fatal("read changed journal")
	}
}
