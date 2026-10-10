//go:build darwin || linux

package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// doctorOK runs `doctor` with args and returns its single item.
func doctorOK(t *testing.T, root string, args ...string) (run, wire.Value) {
	t.Helper()
	x := atm(t, root, nil, append([]string{"doctor"}, args...)...)
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("doctor %v: %s", args, x.stdout)
	}
	item := x.res.Items[0]
	if field(item, "profile").Str != "taskman-doctor/0" || field(item, "mutationAuthority").Kind != wire.KindBool || field(item, "mutationAuthority").Bool {
		t.Fatalf("doctor item: %s", x.stdout)
	}
	return x, item
}

// findingsOf returns the item's findings of kind.
func findingsOf(item wire.Value, kind string) []wire.Value {
	var out []wire.Value
	for _, f := range field(item, "findings").Arr {
		if field(f, "kind").Str == kind {
			out = append(out, f)
		}
	}
	return out
}

func doctorCacheDir(t *testing.T, root string) string {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(repo.CommonDir, "taskman-doctor")
}

// TQD-V0-001: plain doctor is a pure read: state dir, intent tree and the
// cache directory are byte-identical afterwards, and bad argv refuses.
func TestTQDV0001_DoctorIsPureRead(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "pure", "P1", `["src/"]`)
	loopCLIHandoffs(t, root, id, 1)
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	state, intents := fixture.TreeSnapshot(t, repo.StateDir), fixture.TreeSnapshot(t, repo.IntentRoot())
	_, item := doctorOK(t, root)
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, repo.StateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, repo.IntentRoot())) {
		t.Fatal("doctor wrote store state")
	}
	if _, err := os.Stat(doctorCacheDir(t, root)); !os.IsNotExist(err) {
		t.Fatalf("plain doctor created the cache: %v", err)
	}
	if len(field(item, "findings").Arr) != 0 || field(field(item, "summary"), "runningSessions").Str != "0" {
		t.Fatalf("clean store findings: %s", wire.Encode(item))
	}
	for _, args := range [][]string{{"--bogus"}, {"--plugins"}, {"--refresh", "--refresh"}, {"--line", "--refresh"}, {"extra"}} {
		if x := atm(t, root, nil, append([]string{"doctor"}, args...)...); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMalformed) {
			t.Fatalf("doctor %v accepted: %s", args, x.stdout)
		}
	}
}

// TQD-V0-003..005: three empty hand-offs are reported from the journal alone,
// with no loopDetection policy, with first-seen and evidence receipts.
func TestTQDV0005_NoProgressHandoff(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "spinning", "P1", `["src/"]`)
	loopCLIHandoffs(t, root, id, 2)
	_, item := doctorOK(t, root)
	if got := findingsOf(item, "NO_PROGRESS_HANDOFF"); len(got) != 0 {
		t.Fatalf("two hand-offs reported: %s", wire.Encode(item))
	}
	c := handoffCLI(t, root, "claim", id, "--holder", "w9", "--stage", "implement", "--request-id", "claim-9")
	a, g := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	if r := handoffCLI(t, root, "release", "--attempt", a, "--generation", g, "--reason", wire.CodeHandoff, "--evidence", "no-change", "--request-id", "release-9"); r.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release: %s", r.stdout)
	}
	_, item = doctorOK(t, root)
	got := findingsOf(item, "NO_PROGRESS_HANDOFF")
	if len(got) != 1 {
		t.Fatalf("no-progress: %s", wire.Encode(item))
	}
	f := got[0]
	if field(f, "who").Str != id || field(f, "source").Str != "builtin" || field(f, "remedy").Str == "" || len(field(f, "evidenceSeqs").Arr) != 3 {
		t.Fatalf("finding: %s", wire.Encode(f))
	}
	if _, err := time.Parse(time.RFC3339, field(f, "firstSeen").Str); err != nil {
		t.Fatalf("firstSeen: %s", wire.Encode(f))
	}
	if age, err := strconv.Atoi(field(f, "ageSeconds").Str); err != nil || age < 0 {
		t.Fatalf("ageSeconds: %s", wire.Encode(f))
	}
	scan := field(item, "scan")
	if field(scan, "truncated").Bool || field(scan, "receipts").Str == "0" || field(scan, "windowSeconds").Str != "604800" {
		t.Fatalf("scan: %s", wire.Encode(scan))
	}
	if field(field(item, "summary"), "alerts").Str != "1" {
		t.Fatalf("alerts: %s", wire.Encode(item))
	}
}

// TQD-V0-006: the reviewer returns the same candidate tree three times.
func TestTQDV0006_RepeatRefusal(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "refused", "P1", `["src/"]`)
	treeRaw, err := exec.Command("git", "-C", root, "rev-parse", "HEAD^{tree}").Output()
	if err != nil {
		t.Fatal(err)
	}
	tree := strings.TrimSpace(string(treeRaw))
	runOK := func(args ...string) run {
		t.Helper()
		r := handoffCLI(t, root, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	for i := 0; i < 6; i++ {
		stage, reason := "implement", wire.CodeHandoff
		if i%2 == 1 {
			stage, reason = "review", wire.CodeReviewReturned
		}
		r := runOK("claim", id, "--holder", fmt.Sprintf("worker-%d", i), "--stage", stage, "--request-id", fmt.Sprintf("claim-%d", i))
		a, g := field(r.res.Items[0], "attemptId").Str, field(r.res.Items[0], "generation").Str
		runOK("submit", "--attempt", a, "--generation", g, "--tree", tree, "--request-id", fmt.Sprintf("submit-%d", i))
		runOK("release", "--attempt", a, "--generation", g, "--reason", reason, "--request-id", fmt.Sprintf("release-%d", i))
		_, item := doctorOK(t, root)
		got := findingsOf(item, "REPEAT_REFUSAL")
		if want := (i + 1) / 2; want < 3 && len(got) != 0 {
			t.Fatalf("reported after %d returns: %s", want, wire.Encode(item))
		}
		if i == 5 && (len(got) != 1 || field(got[0], "who").Str != id || !strings.Contains(field(got[0], "detail").Str, tree)) {
			t.Fatalf("repeat refusal: %s", wire.Encode(item))
		}
		if len(findingsOf(item, "NO_PROGRESS_HANDOFF")) != 0 {
			t.Fatalf("submitted work reported as no progress: %s", wire.Encode(item))
		}
	}
}

// TQD-V0-007: a member whose cleanup fails three times is a slow recovery.
func TestTQDV0007_SlowLaneRecovery(t *testing.T) {
	cleanup := wire.ObjectValue(wire.NewObject().Set("argv", wire.Strings([]string{"/usr/bin/false"})).Set("cwd", wire.String("REPOSITORY")).Set("env", wire.Array()).Set("timeoutSeconds", wire.String("3")))
	root, _ := leaseCLIStoreWith(t, 0, time.Now().UTC().Add(-time.Minute), func(p *wire.Object) {
		pool := wire.NewObject().Set("id", wire.String("lanes")).Set("members", wire.Strings([]string{"m1", "m2"}))
		pool.Set("memberConfig", wire.ObjectValue(wire.NewObject().Set("m1", wire.ObjectValue(wire.NewObject().Set("cleanup", cleanup))).Set("m2", wire.ObjectValue(wire.NewObject().Set("cleanup", cleanup)))))
		p.Set("pools", wire.Array(wire.ObjectValue(pool)))
	}, "")
	id := planTicket(t, root, "lane", "P1", `["src/"]`)
	c := handoffCLI(t, root, "claim", id, "--holder", "w", "--stage", "implement", "--pool", "lanes", "--request-id", "claim")
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", c.stdout)
	}
	item := c.res.Items[0]
	member, allocation := field(field(item, "poolAllocation"), "memberId").Str, field(field(item, "poolAllocation"), "allocationId").Str
	if r := handoffCLI(t, root, "release", "--attempt", field(item, "attemptId").Str, "--generation", field(item, "generation").Str, "--request-id", "release"); r.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release: %s", r.stdout)
	}
	_, d := doctorOK(t, root)
	if lanes := field(field(d, "summary"), "lanes"); field(lanes, "free").Str != "1" || field(lanes, "total").Str != "2" {
		t.Fatalf("lanes: %s", wire.Encode(d))
	}
	for i := 0; i < 3; i++ {
		if len(findingsOf(d, "SLOW_LANE_RECOVERY")) != 0 {
			t.Fatalf("reported after %d tries: %s", i, wire.Encode(d))
		}
		handoffCLI(t, root, "pool", "cleanup", "--member", member, "--allocation", allocation, "--request-id", fmt.Sprintf("cleanup-%d", i))
		_, d = doctorOK(t, root)
	}
	got := findingsOf(d, "SLOW_LANE_RECOVERY")
	if len(got) != 1 || field(got[0], "who").Str != "lanes/"+member || len(field(got[0], "evidenceSeqs").Arr) != 3 {
		t.Fatalf("slow recovery: %s", wire.Encode(d))
	}
}

// TQD-V0-008: a completion whose ledger witnessed no core obligation.
func TestTQDV0008_SetupOnlyProof(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	if x := atm(t, r.Root, nil, declareArgs(id, "w-setup", obligationRevision(t, r.Root, id), head, "AC-3")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("witness: %s", x.stdout)
	}
	_, item := doctorOK(t, r.Root)
	if len(findingsOf(item, "SETUP_ONLY_PROOF")) != 0 {
		t.Fatalf("open ticket reported: %s", wire.Encode(item))
	}
	if x := atm(t, r.Root, nil, "ticket", "complete-manual", "--request-id", "complete", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
		"--payload", `{"evidence":[],"reason":"manual"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("complete: %s", x.stdout)
	}
	_, item = doctorOK(t, r.Root)
	got := findingsOf(item, "SETUP_ONLY_PROOF")
	if len(got) != 1 || field(got[0], "who").Str != id || !strings.Contains(field(got[0], "detail").Str, "0 of 2 core") {
		t.Fatalf("setup-only: %s", wire.Encode(item))
	}
	if field(field(item, "summary"), "completions24h").Str != "1" {
		t.Fatalf("completions: %s", wire.Encode(item))
	}
}

// TQD-V0-009: a stale holder whose detached run's process tree is busy.
func TestTQDV0009_FalseIdle(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "busy", "P1", `["src/"]`)
	c := handoffCLI(t, root, "claim", id, "--holder", "w", "--stage", "implement", "--request-id", "claim")
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", c.stdout)
	}
	a, g := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	proc := exec.Command("/bin/sh", "-c", "/bin/sleep 30 & wait")
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proc.Process.Kill(); _ = proc.Wait() })
	var identity string
	for deadline := time.Now().Add(5 * time.Second); ; {
		id, err := supervisor.ProcessIdentity(proc.Process.Pid)
		if err == nil && id != "" {
			identity = id
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no identity: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := cli.WriteDoctorTestRun(root, a, g, proc.Process.Pid, identity); err != nil {
		t.Fatal(err)
	}
	// A fresh holder is not idle at all.
	_, item := doctorOK(t, root)
	if len(findingsOf(item, "FALSE_IDLE")) != 0 || field(field(item, "summary"), "runningSessions").Str != "1" {
		t.Fatalf("fresh holder: %s", wire.Encode(item))
	}
	restore := cli.SetDoctorClock(func() time.Time { return time.Now().Add(48 * time.Hour) })
	defer restore()
	var got []wire.Value
	for deadline := time.Now().Add(5 * time.Second); len(got) == 0 && time.Now().Before(deadline); {
		_, item = doctorOK(t, root)
		got = findingsOf(item, "FALSE_IDLE")
		if len(got) == 0 {
			time.Sleep(50 * time.Millisecond) // the shell may not have forked its child yet
		}
	}
	if len(got) != 1 || field(got[0], "who").Str != a {
		t.Fatalf("false idle: %s", wire.Encode(item))
	}
	_ = proc.Process.Kill()
	_ = proc.Wait()
	if _, item = doctorOK(t, root); len(findingsOf(item, "FALSE_IDLE")) != 0 {
		t.Fatalf("dead supervisor reported: %s", wire.Encode(item))
	}
}

// TQD-V0-010: plugin findings appear beside the built-ins; a failing plugin
// is a PLUGIN_FAILED finding, not a command failure.
func TestTQDV0010_PluginFindings(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "plugged", "P1", `["src/"]`)
	loopCLIHandoffs(t, root, id, 3)
	dir := t.TempDir()
	write := func(name, body string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("a-good", "#!/bin/sh\nprintf '%s' '[{\"kind\":\"STALE_DOCS\",\"who\":\"docs/x.md\",\"detail\":\"older than its code\"}]'\n", 0o755)
	write("b-exit", "#!/bin/sh\nexit 3\n", 0o755)
	write("c-junk", "#!/bin/sh\necho not json\n", 0o755)
	write("d-unknown-key", "#!/bin/sh\necho '[{\"kind\":\"X\",\"who\":\"w\",\"detail\":\"d\",\"extra\":1}]'\n", 0o755)
	write("e-slow", "#!/bin/sh\nexec /bin/sleep 20\n", 0o755)
	write("notes.txt", "not executable", 0o644)
	restore := cli.SetDoctorPluginTimeout(500 * time.Millisecond)
	defer restore()
	x, item := doctorOK(t, root, "--plugins", dir)
	if !x.res.Untrusted {
		t.Fatalf("plugin output not marked untrusted: %s", x.stdout)
	}
	good := findingsOf(item, "STALE_DOCS")
	if len(good) != 1 || field(good[0], "source").Str != "plugin:a-good" || field(good[0], "who").Str != "docs/x.md" {
		t.Fatalf("plugin finding: %s", wire.Encode(item))
	}
	failed := map[string]bool{}
	for _, f := range findingsOf(item, "PLUGIN_FAILED") {
		failed[field(f, "who").Str] = true
	}
	for _, name := range []string{"b-exit", "c-junk", "d-unknown-key", "e-slow"} {
		if !failed[name] {
			t.Fatalf("%s not reported failed: %s", name, wire.Encode(item))
		}
	}
	if len(failed) != 4 || len(findingsOf(item, "NO_PROGRESS_HANDOFF")) != 1 {
		t.Fatalf("findings: %s", wire.Encode(item))
	}
	if x := atm(t, root, nil, "doctor", "--plugins", filepath.Join(dir, "missing")); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("missing plugin dir accepted: %s", x.stdout)
	}
}

// lineRun runs `doctor --line` in-process and returns stdout, exit and time.
func lineRun(root string) (string, int, time.Duration) {
	var out, errb bytes.Buffer
	start := time.Now()
	code := cli.Run(cli.Env{Cwd: root, Args: []string{"doctor", "--line"}, Stdout: &out, Stderr: &errb})
	return out.String(), code, time.Since(start)
}

// TQD-V0-002, TQD-V0-011, TQD-V0-012: --refresh writes the cache atomically,
// --line prints the status-bar summary from it alone, fast and lock-free,
// and first-seen survives refreshes.
func TestTQDV0012_LineReadsCache(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "lined", "P1", `["src/"]`)
	if out, code, _ := lineRun(root); code == 0 || !strings.Contains(out, "doctor cache unavailable") {
		t.Fatalf("missing cache: %d %q", code, out)
	}
	loopCLIHandoffs(t, root, id, 3)
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	state := fixture.TreeSnapshot(t, repo.StateDir)
	_, refreshed := doctorOK(t, root, "--refresh")
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, repo.StateDir)) {
		t.Fatal("refresh wrote store state")
	}
	cache := filepath.Join(doctorCacheDir(t, root), "summary.json")
	st, err := os.Stat(cache)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("cache file: %v %v", st, err)
	}
	if dst, err := os.Stat(filepath.Dir(cache)); err != nil || dst.Mode().Perm() != 0o700 {
		t.Fatalf("cache dir: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(cache))
	if len(entries) != 2 || entries[0].Name() != ".lock" || entries[1].Name() != "summary.json" {
		t.Fatalf("leftover temporary files: %v", entries)
	}
	raw, err := os.ReadFile(cache)
	if err != nil || !bytes.Contains(raw, []byte(`"profile":"taskman-doctor-cache/0"`)) {
		t.Fatalf("cache: %s %v", raw, err)
	}
	firstSeen := field(findingsOf(refreshed, "NO_PROGRESS_HANDOFF")[0], "firstSeen").Str
	// The cached line does not read the store: it survives a removed state dir.
	moved := repo.StateDir + ".moved"
	if err := os.Rename(repo.StateDir, moved); err != nil {
		t.Fatal(err)
	}
	best := time.Hour
	var line string
	for i := 0; i < 5; i++ {
		out, code, took := lineRun(root)
		if code != 0 {
			t.Fatalf("line: %d %q", code, out)
		}
		line = out
		if took < best {
			best = took
		}
	}
	if err := os.Rename(moved, repo.StateDir); err != nil {
		t.Fatal(err)
	}
	if line != "lanes 0/0 free | sessions 0 | 24h 0 done | alerts 1\n" {
		t.Fatalf("line %q", line)
	}
	// Best of five excludes one-off scheduler stalls on a loaded host.
	if best >= 50*time.Millisecond {
		t.Fatalf("--line took %v", best)
	}
	restore := cli.SetDoctorClock(func() time.Time { return time.Now().Add(20 * time.Minute) })
	if out, _, _ := lineRun(root); !strings.HasSuffix(out, " | stale 20m\n") {
		restore()
		t.Fatalf("stale line %q", out)
	}
	_, again := doctorOK(t, root, "--refresh")
	restore()
	if got := field(findingsOf(again, "NO_PROGRESS_HANDOFF")[0], "firstSeen").Str; got != firstSeen {
		t.Fatalf("firstSeen moved: %s -> %s", firstSeen, got)
	}
	// A corrupt cache is unavailable, not a partial line.
	if err := os.WriteFile(cache, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, code, _ := lineRun(root); code == 0 || !strings.Contains(out, "doctor cache unavailable") {
		t.Fatalf("corrupt cache: %d %q", code, out)
	}
}

// TQD-V0-006: a gate result counts once, at the receipt that added it to its
// attempt. Results a pre-existing attempt already carried when the scan
// first sees it (recorded before the window) are not recent refusals and do
// not set firstSeen.
func TestTQDV0006_OldGateResultsAreNotRecentRefusals(t *testing.T) {
	at := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	ev := func(seq uint64, attempt string, fresh bool, gates ...string) cli.DoctorTestEvent {
		return cli.DoctorTestEvent{Seq: seq, At: at.Add(time.Duration(seq) * time.Minute), Attempts: []cli.DoctorTestAttempt{{Ticket: "T", Attempt: attempt, Gates: gates, Fresh: fresh}}}
	}
	if seqs, _ := cli.DoctorRepeatRefusalSeqs([]cli.DoctorTestEvent{ev(10, "a1", false, "g1", "g2", "g3"), ev(11, "a1", false, "g1", "g2", "g3")}, "tree"); len(seqs) != 0 {
		t.Fatalf("old gate results counted: %v", seqs)
	}
	seqs, first := cli.DoctorRepeatRefusalSeqs([]cli.DoctorTestEvent{
		ev(20, "a2", true), ev(21, "a2", false, "g4"), ev(22, "a2", false, "g4", "g5"), ev(23, "a2", false, "g4", "g5"), ev(24, "a2", false, "g4", "g5", "g6"),
	}, "tree")
	if len(seqs) != 1 || fmt.Sprint(seqs[0]) != "[21 22 24]" || !first[0].Equal(at.Add(21*time.Minute)) {
		t.Fatalf("in-window results: %v %v", seqs, first)
	}
	seqs, first = cli.DoctorRepeatRefusalSeqs([]cli.DoctorTestEvent{
		ev(30, "a3", false, "g1", "g2"), ev(31, "a3", false, "g1", "g2", "g7"), ev(32, "a3", false, "g1", "g2", "g7", "g8"), ev(33, "a3", false, "g1", "g2", "g7", "g8", "g9"),
	}, "tree")
	if len(seqs) != 1 || fmt.Sprint(seqs[0]) != "[31 32 33]" || !first[0].Equal(at.Add(31*time.Minute)) {
		t.Fatalf("results added to a pre-existing attempt: %v %v", seqs, first)
	}
}

// TQD-V0-010: a plugin's descendants die with it, whether they detach from
// its stdout or keep it open.
func TestTQDV0010_PluginDescendantsAreKilled(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	dir, pids := t.TempDir(), t.TempDir()
	plugins := map[string]string{
		"detached":  "/bin/sleep 300 >/dev/null 2>&1 &",
		"inherited": "/bin/sleep 300 &",
	}
	for name, spawn := range plugins {
		body := "#!/bin/sh\n" + spawn + "\necho $! > '" + filepath.Join(pids, name) + "'\nprintf '[]'\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	doctorOK(t, root, "--plugins", dir)
	for name := range plugins {
		raw, err := os.ReadFile(filepath.Join(pids, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil || pid <= 1 {
			t.Fatalf("%s pid %q", name, raw)
		}
		t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
		deadline := time.Now().Add(3 * time.Second)
		for syscall.Kill(pid, 0) != syscall.ESRCH {
			if time.Now().After(deadline) {
				t.Fatalf("%s: plugin descendant %d survived the doctor", name, pid)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

// TQD-V0-010: discovery reads at most 4096 directory entries; a larger
// plugin directory runs no plugin and reports one PLUGIN_FAILED.
func TestTQDV0010_PluginDiscoveryIsBounded(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	dir := t.TempDir()
	good := "#!/bin/sh\nprintf '%s' '[{\"kind\":\"STALE_DOCS\",\"who\":\"docs/x.md\",\"detail\":\"old\"}]'\n"
	if err := os.WriteFile(filepath.Join(dir, "a-good"), []byte(good), 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4096; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("n%04d", i)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, item := doctorOK(t, root, "--plugins", dir)
	failed := findingsOf(item, "PLUGIN_FAILED")
	if len(findingsOf(item, "STALE_DOCS")) != 0 || len(failed) != 1 || !strings.Contains(field(failed[0], "detail").Str, "4096") {
		t.Fatalf("oversized plugin directory: %s", wire.Encode(item))
	}
	if err := os.Remove(filepath.Join(dir, "n0000")); err != nil {
		t.Fatal(err)
	}
	if _, item = doctorOK(t, root, "--plugins", dir); len(findingsOf(item, "STALE_DOCS")) != 1 || len(findingsOf(item, "PLUGIN_FAILED")) != 0 {
		t.Fatalf("4096-entry plugin directory: %s", wire.Encode(item))
	}
}

// TQD-V0-011: --refresh refuses a cache directory or file that is a symlink,
// and writes and chmods nothing through it.
func TestTQDV0011_RefreshRefusesSymlinkedCache(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := doctorCacheDir(t, root)
	refused := func(why string) {
		t.Helper()
		if x := atm(t, root, nil, "doctor", "--refresh"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeUnsupportedFilesystem) {
			t.Fatalf("%s: refresh not refused: %s", why, x.stdout)
		}
	}
	state := fixture.TreeSnapshot(t, repo.StateDir)
	if err := os.Symlink(repo.StateDir, dir); err != nil {
		t.Fatal(err)
	}
	stateMode, _ := os.Stat(repo.StateDir)
	refused("directory linked to the state dir")
	if after, _ := os.Stat(repo.StateDir); after.Mode() != stateMode.Mode() || !fixture.SameTree(state, fixture.TreeSnapshot(t, repo.StateDir)) {
		t.Fatal("refresh wrote or chmodded the state dir through the link")
	}
	elsewhere := t.TempDir()
	if err := os.Chmod(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, dir); err != nil {
		t.Fatal(err)
	}
	refused("directory linked elsewhere")
	if st, _ := os.Stat(elsewhere); st.Mode().Perm() != 0o755 {
		t.Fatalf("link target chmodded to %v", st.Mode().Perm())
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Fatalf("refresh wrote through the link: %v", entries)
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(elsewhere, "victim.json")
	if err := os.WriteFile(victim, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(dir, "summary.json")); err != nil {
		t.Fatal(err)
	}
	refused("cache file linked elsewhere")
	if raw, _ := os.ReadFile(victim); string(raw) != "keep\n" {
		t.Fatalf("victim rewritten: %q", raw)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leftover temporary file: %v", entries)
		}
	}
}

// TQD-V0-011: --refresh refuses to replace a cache written in a later
// format and leaves it byte-identical; plain doctor still answers.
func TestTQDV0011_RefreshKeepsNewerCache(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	dir := doctorCacheDir(t, root)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	newer := []byte(`{"profile":"taskman-doctor-cache/1"}` + "\n")
	cache := filepath.Join(dir, "summary.json")
	if err := os.WriteFile(cache, newer, 0o600); err != nil {
		t.Fatal(err)
	}
	if x := atm(t, root, nil, "doctor", "--refresh"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeUnsupportedVersion) {
		t.Fatalf("newer cache overwritten: %s", x.stdout)
	}
	if raw, _ := os.ReadFile(cache); !bytes.Equal(raw, newer) {
		t.Fatalf("newer cache changed: %s", raw)
	}
	doctorOK(t, root)
	// Any other undecodable cache is replaced.
	if err := os.WriteFile(cache, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	doctorOK(t, root, "--refresh")
	if out, code, _ := lineRun(root); code != 0 {
		t.Fatalf("replaced cache: %d %q", code, out)
	}
}

// TQD-V0-011, TQD-V0-012: the largest output plugins may produce still
// refreshes into a cache --line can read; findings are cut by bytes in
// order and the cut is reported.
func TestTQDV0011_MaximalPluginOutputStaysReadable(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	dir, data := t.TempDir(), t.TempDir()
	pad := func(p string) string { return p + strings.Repeat("x", 1024-len(p)) }
	for p := 0; p < 32; p++ {
		items := make([]string, 0, 20)
		for i := 0; i < 20; i++ {
			items = append(items, fmt.Sprintf(`{"kind":"X","who":%q,"detail":%q,"remedy":%q}`, pad(fmt.Sprintf("p%02d-w%02d-", p, i)), pad("d"), pad("r")))
		}
		out := "[" + strings.Join(items, ",") + "]"
		if len(out) > 64<<10 {
			t.Fatalf("plugin output %d bytes exceeds the plugin bound", len(out))
		}
		file := filepath.Join(data, fmt.Sprintf("p%02d.json", p))
		if err := os.WriteFile(file, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("p%02d", p)), []byte("#!/bin/sh\nexec /bin/cat '"+file+"'\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_, item := doctorOK(t, root, "--refresh", "--plugins", dir)
	if len(findingsOf(item, "PLUGIN_FAILED")) != 0 {
		t.Fatalf("plugin failed: %s", wire.Encode(findingsOf(item, "PLUGIN_FAILED")[0]))
	}
	raw, err := os.ReadFile(filepath.Join(doctorCacheDir(t, root), "summary.json"))
	if err != nil || len(raw) > 1<<20 {
		t.Fatalf("cache %d bytes: %v", len(raw), err)
	}
	cached, err := wire.Parse(raw)
	if err != nil || !field(field(cached, "scan"), "findingsTruncated").Bool || len(field(cached, "findings").Arr) == 0 {
		t.Fatalf("cache not cut by bytes: %v", err)
	}
	if out, code, _ := lineRun(root); code != 0 || out != "lanes 0/0 free | sessions 0 | 24h 0 done | alerts 640\n" {
		t.Fatalf("line after maximal refresh: %d %q", code, out)
	}
}

// TQD-V0-011: a cache directory swapped for a relative link between the
// refresh's check and its open is refused; nothing is chmodded or written
// through the link into the state directory.
func TestTQDV0011_RefreshRefusesSwappedCacheDir(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	doctorOK(t, root, "--refresh")
	dir := doctorCacheDir(t, root)
	if err := os.Chmod(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	cache := fixture.TreeSnapshot(t, dir)
	state := fixture.TreeSnapshot(t, repo.StateDir)
	moved := filepath.Join(repo.StateDir, "moved-cache")
	defer cli.SetDoctorCacheHookForTest(func(stage string) {
		if stage != "open" {
			return
		}
		if err := os.Rename(dir, moved); err != nil {
			t.Error(err)
		}
		if err := os.Symlink(filepath.Join("taskman", "moved-cache"), dir); err != nil {
			t.Error(err)
		}
	})()
	if x := atm(t, root, nil, "doctor", "--refresh"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeUnsupportedFilesystem) {
		t.Fatalf("swapped cache directory not refused: %s", x.stdout)
	}
	if st, err := os.Stat(moved); err != nil || st.Mode().Perm() != 0o750 {
		t.Fatalf("moved cache directory chmodded: %v %v", st, err)
	}
	if !fixture.SameTree(cache, fixture.TreeSnapshot(t, moved)) {
		t.Fatal("refresh wrote into the state directory through the swapped link")
	}
	if err := os.RemoveAll(moved); err != nil {
		t.Fatal(err)
	}
	// The state directory's own entry is dropped: the test's rename and
	// removal, not the refresh, change its modification time.
	if !fixture.SameTree(state[1:], fixture.TreeSnapshot(t, repo.StateDir)[1:]) {
		t.Fatal("refresh changed the state directory")
	}
}

// TQD-V0-011: the version check is repeated under the cache lock, so a
// newer cache installed after the command's first check is still refused
// and left byte-identical.
func TestTQDV0011_RefreshRechecksVersionUnderLock(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	doctorOK(t, root, "--refresh")
	cache := filepath.Join(doctorCacheDir(t, root), "summary.json")
	newer := []byte(`{"profile":"taskman-doctor-cache/1"}` + "\n")
	defer cli.SetDoctorCacheHookForTest(func(stage string) {
		if stage == "lock" {
			if err := os.WriteFile(cache+".new", newer, 0o600); err != nil {
				t.Error(err)
			}
			if err := os.Rename(cache+".new", cache); err != nil {
				t.Error(err)
			}
		}
	})()
	if x := atm(t, root, nil, "doctor", "--refresh"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeUnsupportedVersion) {
		t.Fatalf("newer cache installed concurrently was replaced: %s", x.stdout)
	}
	if raw, _ := os.ReadFile(cache); !bytes.Equal(raw, newer) {
		t.Fatalf("newer cache changed: %s", raw)
	}
}

// TQD-V0-001: valid tracked intent without a local journal refuses
// MISSING_EVIDENCE, with or without --refresh, and writes nothing.
func TestTQDV0001_JournalAbsentIsMissingEvidence(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteIntent(t, r, fixture.Ticket("A"))
	before := fixture.TreeSnapshot(t, r.Root)
	for _, args := range [][]string{{"doctor"}, {"doctor", "--refresh"}} {
		if x := atm(t, r.Root, nil, args...); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMissingEvidence) {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.Root)) {
		t.Fatal("journal-absent doctor wrote")
	}
}

// TQD-V0-001, TQD-V0-011: a cache directory renamed into the state
// directory after the refresh opened it is refused; the moved directory is
// left as it was, with no cache or temporary file written into state.
func TestTQDV0011_RefreshRefusesCacheDirMovedAfterOpen(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	doctorOK(t, root, "--refresh")
	dir := doctorCacheDir(t, root)
	cache := fixture.TreeSnapshot(t, dir)
	moved := filepath.Join(repo.StateDir, "moved-cache")
	defer cli.SetDoctorCacheHookForTest(func(stage string) {
		if stage == "lock" {
			if err := os.Rename(dir, moved); err != nil {
				t.Error(err)
			}
		}
	})()
	if x := atm(t, root, nil, "doctor", "--refresh"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeUnsupportedFilesystem) {
		t.Fatalf("cache directory moved after open not refused: %s", x.stdout)
	}
	if !fixture.SameTree(cache[1:], fixture.TreeSnapshot(t, moved)[1:]) {
		t.Fatal("refresh wrote into the cache directory moved into the state directory")
	}
}

// TQD-V0-011: a directory where the lock file belongs is refused
// UNSUPPORTED_FILESYSTEM, not reported as malformed.
func TestTQDV0011_RefreshRefusesDirectoryLock(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	dir := doctorCacheDir(t, root)
	if err := os.MkdirAll(filepath.Join(dir, ".lock"), 0o700); err != nil {
		t.Fatal(err)
	}
	if x := atm(t, root, nil, "doctor", "--refresh"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeUnsupportedFilesystem) {
		t.Fatalf("directory lock not refused UNSUPPORTED_FILESYSTEM: %s", x.stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "summary.json")); !os.IsNotExist(err) {
		t.Fatalf("cache written beside a directory lock: %v", err)
	}
}

// TQD-V0-009: a FIFO in place of a stale attempt's run directory is
// skipped; FALSE_IDLE discovery never blocks opening it.
func TestTQDV0009_FalseIdleSkipsRunsFIFO(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "busy", "P1", `["src/"]`)
	c := handoffCLI(t, root, "claim", id, "--holder", "w", "--stage", "implement", "--request-id", "claim")
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", c.stdout)
	}
	base, err := cli.RunDirForTest(root, field(c.res.Items[0], "attemptId").Str)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(base), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(base, 0o600); err != nil {
		t.Fatal(err)
	}
	restore := cli.SetDoctorClock(func() time.Time { return time.Now().Add(48 * time.Hour) })
	defer restore()
	done := make(chan []byte, 1)
	go func() { // the test goroutine must stay free to report a hang
		var out, errb bytes.Buffer
		cli.Run(cli.Env{Cwd: root, Args: []string{"doctor"}, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &errb})
		done <- out.Bytes()
	}()
	select {
	case out := <-done:
		res, err := wire.DecodeResult(out)
		if err != nil || res.Outcome != wire.OutcomeOK || len(res.Items) != 1 || len(findingsOf(res.Items[0], "FALSE_IDLE")) != 0 {
			t.Fatalf("doctor beside a FIFO run directory: %v %s", err, out)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("doctor blocked opening a FIFO in place of the run directory")
	}
}
