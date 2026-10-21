//go:build darwin || linux

package sessiontest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// fakeGo is a `go` on PATH whose behaviour FAKE_MODE selects: it answers the
// version and universe probes, and for `test` records its argv, then passes,
// fails, or blocks on the gateFake FIFO: "slow" passes once released, "sleep"
// first starts a grandchild, and "slowenv" blocks in the version probe.
const fakeGo = `#!/bin/sh
case "$1" in env|list) echo "$1" >> "$FAKE_DIR/probes";; esac
case "$1" in
env) [ "$FAKE_MODE" = noversion ] && exit 1; [ "$FAKE_MODE" = slowenv ] && { read gate < "$FAKE_DIR/gate.$(wc -l < "$FAKE_DIR/probes" | tr -d ' ')"; exit 1; }; echo go1.27.1; exit 0;;
list) echo m/a; [ "$FAKE_MODE" = extra ] && echo m/x; exit 0;;
esac
echo "$@" >> "$FAKE_DIR/argv"
gate="$FAKE_DIR/gate.$(wc -l < "$FAKE_DIR/argv" | tr -d ' ')"
case "$FAKE_MODE" in
sleep)
  sleep 60 & echo $! > "$FAKE_DIR/grandchild"
  read line < "$gate";;
fail) echo '{"Action":"fail","Package":"m/a"}'; exit 1;;
slow) read line < "$gate" || exit 1; echo '{"Action":"pass","Package":"m/a"}';;
*) echo '{"Action":"pass","Package":"m/a"}';;
esac
`

// shardClock is a virtual shard clock: Runner.now reads it and
// Runner.deadline binds contexts to it, so a shard budget expires when the
// test advances the clock, never because a loaded host ran slowly (V1-1445).
type shardClock struct {
	mu      sync.Mutex
	now     time.Time
	pending []virtualDeadline
}

type virtualDeadline struct {
	at     time.Time
	cancel context.CancelFunc
}

func useShardClock(r *Runner) *shardClock {
	c := &shardClock{now: time.Unix(1_700_000_000, 0)}
	r.now, r.deadline = c.Now, c.WithDeadline
	return c
}

func (c *shardClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *shardClock) WithDeadline(ctx context.Context, at time.Time) (context.Context, context.CancelFunc) {
	child, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.now.Before(at) {
		c.pending = append(c.pending, virtualDeadline{at: at, cancel: cancel})
	} else {
		cancel()
	}
	return child, cancel
}

// Advance moves the clock and expires every context whose deadline it reached.
func (c *shardClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	kept := c.pending[:0]
	for _, p := range c.pending {
		if c.now.Before(p.at) {
			kept = append(kept, p)
		} else {
			p.cancel()
		}
	}
	c.pending = kept
}

// gateFake makes FAKE_DIR/gate.N, the FIFO the Nth gated fake go run (or
// probe) blocks on, and runs step N once that fake has opened it. Opening the
// writer returns only once the fake has opened the reader, so each step runs
// at a known point of its own fake's run; the rendezvous is the sync point,
// not elapsed time. One FIFO per run keeps a still-open earlier reader from
// satisfying a later step. A step returning true releases the fake; false
// holds the writer open, so the fake stays blocked until the runner kills its
// process group.
func gateFake(t *testing.T, dir string, steps ...func() bool) {
	t.Helper()
	gates := make([]string, len(steps))
	for i := range steps {
		gates[i] = filepath.Join(dir, "gate."+strconv.Itoa(i+1))
		if err := syscall.Mkfifo(gates[i], 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stop, finished := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		for i, step := range steps {
			w, err := os.OpenFile(gates[i], os.O_WRONLY, 0)
			if err != nil {
				return
			}
			select {
			case <-stop:
				_ = w.Close()
				return
			default:
			}
			if step() {
				_, _ = w.WriteString("go\n")
			} else {
				<-stop
			}
			_ = w.Close()
		}
	}()
	t.Cleanup(func() {
		close(stop)
		// An open reader on each gate lets a writer still blocked in open
		// return; it then sees stop and ends.
		var readers []*os.File
		for _, gate := range gates {
			if r, err := os.OpenFile(gate, os.O_RDONLY|syscall.O_NONBLOCK, 0); err == nil {
				readers = append(readers, r)
			}
		}
		<-finished
		for _, r := range readers {
			_ = r.Close()
		}
	})
}

func runnerFixture(t *testing.T, mode string) (*Runner, Manifest, string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(fakeGo), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin"+string(os.PathListSeparator)+"/usr/bin")
	source := SourceIdentity{Head: "h", Paths: []PathDigest{}, Tree: "t"}
	m := Manifest{
		Checks: []string{}, Incomplete: []string{}, Profile: ManifestProfile, RunnerProfile: ProfileLocal,
		Scope: ScopeNarrowed, ShardCount: 1, Source: source,
		Units: []Unit{{Adapter: AdapterGo, Shard: 0, Slice: SliceWhole, Unit: "go:m/a"}},
	}
	digest, _ := Digest(m)
	runner := &Runner{
		Declared: parsedDeclared(), Environ: []string{"PATH=" + os.Getenv("PATH"), "FAKE_MODE=" + mode, "FAKE_DIR=" + dir},
		Errors: io.Discard, Output: io.Discard, Root: dir, Tool: "corvint test", Universe: []string{"m/a"},
		Recheck: func(context.Context) (SourceIdentity, error) { return source, nil },
	}
	return runner, m, digest, dir
}

func TestSTRV0014RunnerPassAndFail(t *testing.T) {
	runner, m, digest, dir := runnerFixture(t, "pass")
	receipt := runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result != ResultPass || len(receipt.Invocations) != 1 || receipt.Invocations[0].Version != "go1.27.1" ||
		receipt.Invocations[0].Runner != filepath.Join(dir, "go") || receipt.Invocations[0].Outcomes[0].Outcome != OutcomePass {
		t.Fatalf("pass receipt %+v", receipt)
	}
	argv, _ := os.ReadFile(filepath.Join(dir, "argv"))
	if string(argv) != "test -json -p 1 -timeout 30m -count=1 m/a\n" {
		t.Fatalf("argv %q", argv)
	}
	runner, m, digest, _ = runnerFixture(t, "fail")
	if failed := runner.Execute(context.Background(), m, digest, 0); failed.Result != ResultFail || failed.Invocations[0].Exit != 1 {
		t.Fatalf("fail receipt %s %+v", failed.Result, failed.Invocations)
	}
}

// STR-V0-004: an unreadable version or a universe mismatch launches nothing.
func TestSTRV0004RunnerDiscoveryRefusals(t *testing.T) {
	for mode, reason := range map[string]string{
		"noversion": "go: runner version cannot be read",
		"extra":     "go: go list ./... differs from the planned Go universe: 1 unplanned [m/x], 0 unlisted []",
	} {
		runner, m, digest, dir := runnerFixture(t, mode)
		receipt := runner.Execute(context.Background(), m, digest, 0)
		if _, err := os.Stat(filepath.Join(dir, "argv")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s: go test launched", mode)
		}
		if receipt.Result != ResultIncomplete || receipt.Incomplete[0] != reason {
			t.Fatalf("%s: %s %v", mode, receipt.Result, receipt.Incomplete)
		}
	}
}

// STR-V0-017: drift before a launch stops it; drift after the last launch
// makes passing outcomes INCOMPLETE.
func TestSTRV0017RunnerDrift(t *testing.T) {
	runner, m, digest, dir := runnerFixture(t, "pass")
	runner.Recheck = func(context.Context) (SourceIdentity, error) {
		return SourceIdentity{Head: "h", Paths: []PathDigest{{Digest: "deleted", Path: "x"}}, Tree: "t"}, nil
	}
	before := runner.Execute(context.Background(), m, digest, 0)
	if _, err := os.Stat(filepath.Join(dir, "argv")); !errors.Is(err, os.ErrNotExist) || before.Result != ResultIncomplete ||
		before.Incomplete[0] != "source drift before an adapter launch; nothing further launched" {
		t.Fatalf("drift before launch: %s %v", before.Result, before.Incomplete)
	}
	runner, m, digest, _ = runnerFixture(t, "pass")
	calls := 0
	runner.Recheck = func(context.Context) (SourceIdentity, error) {
		calls++
		if calls > 1 {
			return SourceIdentity{}, errors.New("index locked")
		}
		return m.Source, nil
	}
	after := runner.Execute(context.Background(), m, digest, 0)
	if after.Result != ResultIncomplete || after.Invocations[0].Outcomes[0].Outcome != OutcomePass ||
		after.Incomplete[0] != "source identity unavailable (index locked) after the last adapter launch" {
		t.Fatalf("drift during run: %s %v", after.Result, after.Incomplete)
	}
	// A shard with no units still rechecks before it finishes.
	runner, m, _, _ = runnerFixture(t, "pass")
	m.Scope, m.Units = ScopeNothing, []Unit{}
	digest, _ = Digest(m)
	runner.Recheck = func(context.Context) (SourceIdentity, error) {
		return SourceIdentity{Head: "h2", Paths: []PathDigest{}, Tree: "t"}, nil
	}
	empty := runner.Execute(context.Background(), m, digest, 0)
	if empty.Result != ResultIncomplete || len(empty.Invocations) != 0 ||
		!reflect.DeepEqual(empty.Incomplete, []string{"source drift before the shard finished; nothing launched"}) {
		t.Fatalf("empty shard over drift: %s %v", empty.Result, empty.Incomplete)
	}
}

// STR-V0-014, STR-V0-015: cancellation and the budget kill the owned group,
// grandchildren included, and launch nothing further. The fake go has started
// its grandchild and is blocked on the gate when the test cancels or moves the
// virtual shard clock past the budget, so host load cannot reorder the two
// (V1-1439, V1-1445).
func TestSTRV0014RunnerCancelAndBudgetKillGroup(t *testing.T) {
	for _, budget := range []bool{false, true} {
		runner, m, digest, dir := runnerFixture(t, "sleep")
		m.Units = append(m.Units, Unit{Adapter: AdapterGo, Flag: "-run", Pattern: "^T$", Shard: 0, Slice: "0/2", Unit: "go:m/a"})
		digest, _ = Digest(m)
		ctx, cancel := context.WithCancel(context.Background())
		clock := useShardClock(runner)
		runner.Budget = time.Hour
		gateFake(t, dir, func() bool {
			if budget {
				clock.Advance(2 * runner.Budget)
			} else {
				cancel()
			}
			return false
		})
		receipt := runner.Execute(ctx, m, digest, 0)
		cancel()
		want := "cancelled while an adapter ran; its process group was killed"
		if budget {
			want = "the shard exceeded its 1h0m0s total budget while an adapter ran; its process group was killed"
		}
		if receipt.Result != ResultIncomplete || len(receipt.Invocations) != 1 || receipt.Incomplete[0] != want {
			t.Fatalf("budget=%v: %s %d %v", budget, receipt.Result, len(receipt.Invocations), receipt.Incomplete)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "grandchild"))
		if err != nil {
			t.Fatal(err)
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
		for deadline := time.Now().Add(5 * time.Second); syscall.Kill(pid, 0) == nil; time.Sleep(20 * time.Millisecond) {
			if time.Now().After(deadline) {
				_ = syscall.Kill(pid, syscall.SIGKILL)
				t.Fatalf("budget=%v: grandchild %d survived", budget, pid)
			}
		}
	}
}

// STR-V0-015: the budget bounds the whole shard, not each invocation. On the
// virtual shard clock each invocation spends 40 minutes of a one-hour budget:
// both pass a one-hour per-invocation bound, but the second crosses the shard
// deadline and is killed, and once the deadline has passed nothing further
// launches. The gate releases the second fake after the clock moves, so a
// per-invocation deadline (virtual or real) lets it pass and fails this test;
// no real sleep or wall-clock margin decides the outcome (V1-1439, V1-1445).
func TestSTRV0015BudgetBoundsTheShard(t *testing.T) {
	runner, m, _, dir := runnerFixture(t, "slow")
	m.Units = append(m.Units, Unit{Adapter: AdapterGo, Flag: "-run", Pattern: "^T$", Shard: 0, Slice: "0/2", Unit: "go:m/a"})
	digest, _ := Digest(m)
	clock := useShardClock(runner)
	runner.Budget = time.Hour
	spend := func() bool { clock.Advance(40 * time.Minute); return true }
	gateFake(t, dir, spend, spend)
	receipt := runner.Execute(context.Background(), m, digest, 0)
	want := "the shard exceeded its 1h0m0s total budget while an adapter ran; its process group was killed"
	if receipt.Result != ResultIncomplete || len(receipt.Invocations) != 2 || receipt.Invocations[0].Outcomes[0].Outcome != OutcomePass ||
		receipt.Incomplete[0] != want {
		t.Fatalf("shard budget: %s %d %v", receipt.Result, len(receipt.Invocations), receipt.Incomplete)
	}
	// A clock that moves a whole budget per reading has spent the budget
	// before the first probe.
	runner, m, _, _ = runnerFixture(t, "pass")
	digest, _ = Digest(m)
	runner.Budget = time.Hour
	stepping := time.Unix(1_700_000_000, 0)
	runner.now = func() time.Time { stepping = stepping.Add(runner.Budget); return stepping }
	spent := runner.Execute(context.Background(), m, digest, 0)
	if spent.Result != ResultIncomplete || len(spent.Invocations) != 0 ||
		spent.Incomplete[0] != "the shard's 1h0m0s total budget ran out during runner discovery; nothing launched" {
		t.Fatalf("spent budget: %s %d %v", spent.Result, len(spent.Invocations), spent.Incomplete)
	}
	// Discovery spends the same budget: a version probe blocked on the gate is
	// killed when the virtual clock passes the shard deadline, not after its
	// own two-minute bound (discoverDeadline). That bound is the only real
	// time left here, and the passing path never comes near it.
	runner, m, _, dir = runnerFixture(t, "slowenv")
	digest, _ = Digest(m)
	clock = useShardClock(runner)
	runner.Budget = time.Hour
	gateFake(t, dir, func() bool { clock.Advance(2 * runner.Budget); return false })
	started := time.Now()
	hung := runner.Execute(context.Background(), m, digest, 0)
	wantHung := "go: the shard's 1h0m0s total budget ran out during runner discovery; nothing launched"
	if elapsed := time.Since(started); elapsed >= discoverDeadline || hung.Result != ResultIncomplete ||
		len(hung.Invocations) != 0 || len(hung.Incomplete) == 0 || hung.Incomplete[0] != wantHung {
		t.Fatalf("hung discovery: %s %s %d %v", elapsed, hung.Result, len(hung.Invocations), hung.Incomplete)
	}
	if got := (&Runner{}).ShardBudget(Manifest{RunnerProfile: ProfileCI}); got != CIBudget {
		t.Fatalf("ci default budget %s", got)
	}
	if got := (&Runner{}).ShardBudget(Manifest{RunnerProfile: ProfileLocal}); got != DefaultBudget {
		t.Fatalf("local default budget %s", got)
	}
}

// V1-1311 (STR-V0-015): discovery probes spend the remaining shard budget.
// A fixed clock expires the budget as soon as the first probe runs: the Go
// universe probe and the declared runner's version probe never start, nothing
// launches, and the receipt is INCOMPLETE.
func TestSTRV0015DiscoveryStopsAtShardDeadline_V11311(t *testing.T) {
	runner, m, _, dir := runnerFixture(t, "pass")
	script := filepath.Join(dir, "tools", "fake-tests")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho declared >> \"$FAKE_DIR/probes\"\necho fake-tests 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner.Declared.Languages = []LanguageCommand{{Language: "python", Argv: []string{"./tools/fake-tests"}}}
	m.Units = append(m.Units, Unit{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:python"})
	digest, _ := Digest(m)
	probes := filepath.Join(dir, "probes")
	start := time.Now()
	runner.Budget = time.Hour
	runner.now = func() time.Time {
		if _, err := os.Stat(probes); err == nil {
			return start.Add(2 * runner.Budget)
		}
		return start
	}
	receipt := runner.Execute(context.Background(), m, digest, 0)
	ran, _ := os.ReadFile(probes)
	if string(ran) != "env\n" {
		t.Fatalf("probes after the shard deadline: %q; want only the first version probe", ran)
	}
	want := "go: the shard's 1h0m0s total budget ran out during runner discovery; nothing launched"
	if receipt.Result != ResultIncomplete || len(receipt.Invocations) != 0 || len(receipt.Incomplete) == 0 || receipt.Incomplete[0] != want {
		t.Fatalf("receipt %s %d %q", receipt.Result, len(receipt.Invocations), receipt.Incomplete)
	}
	if _, err := os.Stat(filepath.Join(dir, "argv")); err == nil {
		t.Fatal("an adapter launched after the shard deadline")
	}
}

// STR-V0-003: a declared argv[0] with a path separator is resolved against
// the repository root, not the caller's working directory, so `corvint
// --root R test` run from elsewhere finds R/tools/fake-tests.
func TestSTRV0003DeclaredRelativeCommandResolvesAgainstRoot(t *testing.T) {
	runner, m, _, dir := runnerFixture(t, "pass")
	script := filepath.Join(dir, "tools", "fake-tests")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\n[ \"$1\" = --version ] && echo fake-tests 1\npwd > \"$FAKE_DIR/cwd\"\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner.Declared.Languages = []LanguageCommand{{Language: "python", Argv: []string{"./tools/fake-tests"}}}
	m.Scope, m.Units = ScopeFull, []Unit{{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:python"}}
	digest, _ := Digest(m)
	elsewhere := t.TempDir()
	if err := os.WriteFile(filepath.Join(elsewhere, "fake-tests"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(elsewhere)
	receipt := runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result != ResultPass || len(receipt.Invocations) != 1 || receipt.Invocations[0].Runner != script || receipt.Invocations[0].Version != "fake-tests 1" {
		t.Fatalf("receipt %s %+v", receipt.Result, receipt.Invocations)
	}
	resolvedRoot, _ := filepath.EvalSymlinks(dir)
	if cwd, _ := os.ReadFile(filepath.Join(dir, "cwd")); strings.TrimSpace(string(cwd)) != resolvedRoot && strings.TrimSpace(string(cwd)) != dir {
		t.Fatalf("declared command ran in %q, want the root %q", cwd, dir)
	}
	// An absent repository-relative command is a missing runner, not a
	// lookup of the caller's directory.
	runner.Declared.Languages[0].Argv = []string{"./tools/absent"}
	if receipt := runner.Execute(context.Background(), m, digest, 0); receipt.Result != ResultIncomplete || !strings.Contains(strings.Join(receipt.Incomplete, " "), "runner ./tools/absent not found") {
		t.Fatalf("absent receipt %s %+v", receipt.Result, receipt.Incomplete)
	}
}

// STR-V0-004: Execute discovers and launches only an adapter whose capability
// manifest is registered and current; another unit's adapter still runs.
func TestSTRV0004RunnerRefusesAdapterWithoutManifest(t *testing.T) {
	missing := map[string]AdapterManifest{AdapterDeclared: AdapterManifests()[AdapterDeclared]}
	stale := AdapterManifests()
	stale[AdapterGo] = AdapterManifest{Adapter: AdapterGo, Operations: stale[AdapterGo].Operations, Profile: "corvint-test-adapter/1"}
	for name, test := range map[string]struct {
		manifests map[string]AdapterManifest
		reason    string
	}{
		"absent":     {missing, "go: no registered capability manifest; nothing launched for it"},
		"mismatched": {stale, `go: capability manifest "corvint-test-adapter/1" is not corvint-test-adapter/0; nothing launched for it`},
	} {
		runner, m, digest, dir := runnerFixture(t, "pass")
		runner.adapters = test.manifests
		receipt := runner.Execute(context.Background(), m, digest, 0)
		for _, file := range []string{"probes", "argv"} {
			if _, err := os.Stat(filepath.Join(dir, file)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s: the go adapter was discovered or launched (%s exists)", name, file)
			}
		}
		if receipt.Result != ResultIncomplete || len(receipt.Invocations) != 0 || len(receipt.Incomplete) == 0 || receipt.Incomplete[0] != test.reason {
			t.Fatalf("%s: %s %v %+v", name, receipt.Result, receipt.Incomplete, receipt.Invocations)
		}
	}
	// A unit of an adapter nobody registered is refused by name, and the
	// registered Go adapter in the same shard still runs.
	runner, m, _, _ := runnerFixture(t, "pass")
	m.Units = append(m.Units, Unit{Adapter: "rust", Shard: 0, Slice: SliceWhole, Unit: "rust:crate"})
	digest, _ := Digest(m)
	receipt := runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result != ResultIncomplete || len(receipt.Incomplete) == 0 || receipt.Incomplete[0] != "rust: no registered capability manifest; nothing launched for it" ||
		len(receipt.Invocations) != 1 || receipt.Invocations[0].Outcomes[0].Outcome != OutcomePass {
		t.Fatalf("unregistered adapter: %s %v %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
	}
	// The declared adapter is held to its own manifest: without it, the
	// declared command is neither probed nor run.
	runner, m, _, dir := runnerFixture(t, "pass")
	script := filepath.Join(dir, "tools", "fake-tests")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch \"$FAKE_DIR/declared\"\necho fake-tests 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner.Declared.Languages = []LanguageCommand{{Language: "python", Argv: []string{"./tools/fake-tests"}}}
	runner.adapters = map[string]AdapterManifest{AdapterGo: AdapterManifests()[AdapterGo]}
	m.Scope, m.Units = ScopeFull, []Unit{{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:python"}}
	digest, _ = Digest(m)
	receipt = runner.Execute(context.Background(), m, digest, 0)
	if _, err := os.Stat(filepath.Join(dir, "declared")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the declared adapter was discovered or launched without its manifest")
	}
	if receipt.Result != ResultIncomplete || len(receipt.Invocations) != 0 || len(receipt.Incomplete) == 0 || receipt.Incomplete[0] != "declared: no registered capability manifest; nothing launched for it" {
		t.Fatalf("declared without manifest: %s %v %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
	}
}

// STR-V0-059: each receipt invocation cites the capability manifest version
// its adapter ran under, the strict reader keeps it, and verify-shards counts
// an invocation only under the version this binary registers.
func TestSTRV0059ReceiptBindsAdapterManifestVersion(t *testing.T) {
	runner, m, _, dir := runnerFixture(t, "pass")
	sourceDigest, err := Digest(m.Source)
	if err != nil {
		t.Fatal(err)
	}
	m.SourceDigest = sourceDigest
	digest, err := Digest(m)
	if err != nil {
		t.Fatal(err)
	}
	receipt := runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result != ResultPass || len(receipt.Invocations) != 1 || receipt.Invocations[0].AdapterManifest != "corvint-test-adapter/0" {
		t.Fatalf("receipt %s %+v", receipt.Result, receipt.Invocations)
	}
	encoded, err := Encode(receipt)
	if err != nil {
		t.Fatal(err)
	}
	cited := []byte(`"adapter":"go","adapterManifest":"corvint-test-adapter/0",`)
	if !bytes.Contains(encoded, cited) {
		t.Fatalf("the encoded receipt does not cite the manifest version:\n%s", encoded)
	}
	name := filepath.Join(dir, "test-run.json")
	if err := WriteFile(name, encoded); err != nil {
		t.Fatal(err)
	}
	read, err := ReadReceipt(name)
	if err != nil || read.Invocations[0].AdapterManifest != AdapterManifestProfile {
		t.Fatalf("read back %+v %v", read.Invocations, err)
	}
	if result, reasons := Verify(m, digest, []Receipt{read}, nil); result != ResultPass {
		t.Fatalf("verify %s %v", result, reasons)
	}
	for _, test := range []struct{ adapter, version string }{
		{AdapterGo, ""},
		{AdapterGo, "corvint-test-adapter/1"},
		{"rust", AdapterManifestProfile},
	} {
		forged := read
		forged.Invocations = append([]Invocation{}, read.Invocations...)
		forged.Invocations[0].Adapter, forged.Invocations[0].AdapterManifest = test.adapter, test.version
		want := fmt.Sprintf("receipt 0 (shard 0): %s ran under adapter manifest %q, not the registered one", test.adapter, test.version)
		if result, reasons := Verify(m, digest, []Receipt{forged}, nil); result != ResultIncomplete || !slices.Contains(reasons, want) {
			t.Fatalf("%+v: verify %s %v, want INCOMPLETE naming %q", test, result, reasons, want)
		}
	}
	// A receipt that does not cite the version is not canonical and is refused.
	if err := WriteFile(name, bytes.Replace(encoded, cited, []byte(`"adapter":"go",`), 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadReceipt(name); err == nil || !strings.Contains(err.Error(), "is not canonical corvint-test-run/0 JSON") {
		t.Fatalf("receipt without the version read as %v", err)
	}
}
