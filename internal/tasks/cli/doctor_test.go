package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	if len(entries) != 1 {
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
