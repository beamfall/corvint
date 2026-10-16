//go:build darwin || linux

package sessiontest

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// fakeGo is a `go` on PATH whose behaviour FAKE_MODE selects: it answers the
// version and universe probes, and for `test` records its argv, then passes,
// fails, passes after seven seconds, or starts a grandchild and sleeps.
const fakeGo = `#!/bin/sh
case "$1" in env|list) echo "$1" >> "$FAKE_DIR/probes";; esac
case "$1" in
env) [ "$FAKE_MODE" = noversion ] && exit 1; [ "$FAKE_MODE" = slowenv ] && exec sleep 60; echo go1.27.1; exit 0;;
list) echo m/a; [ "$FAKE_MODE" = extra ] && echo m/x; exit 0;;
esac
echo "$@" >> "$FAKE_DIR/argv"
case "$FAKE_MODE" in
sleep)
  sleep 60 & echo $! > "$FAKE_DIR/grandchild"
  touch "$FAKE_DIR/started"
  sleep 60;;
fail) echo '{"Action":"fail","Package":"m/a"}'; exit 1;;
slow) sleep 7; echo '{"Action":"pass","Package":"m/a"}';;
*) echo '{"Action":"pass","Package":"m/a"}';;
esac
`

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
// grandchildren included, and launch nothing further.
func TestSTRV0014RunnerCancelAndBudgetKillGroup(t *testing.T) {
	for _, budget := range []bool{false, true} {
		runner, m, digest, dir := runnerFixture(t, "sleep")
		m.Units = append(m.Units, Unit{Adapter: AdapterGo, Flag: "-run", Pattern: "^T$", Shard: 0, Slice: "0/2", Unit: "go:m/a"})
		digest, _ = Digest(m)
		ctx, cancel := context.WithCancel(context.Background())
		if budget {
			// Long enough for the fake go to start its grandchild on a
			// loaded host; 300ms was racy, and 2s still was (V1-1439).
			runner.Budget = 10 * time.Second
		} else {
			go func() {
				for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
					if _, err := os.Stat(filepath.Join(dir, "started")); err == nil {
						cancel()
						return
					}
				}
			}()
		}
		started := time.Now()
		receipt := runner.Execute(ctx, m, digest, 0)
		cancel()
		if elapsed := time.Since(started); elapsed > 30*time.Second {
			t.Fatalf("budget=%v: took %s", budget, elapsed)
		}
		want := "cancelled while an adapter ran; its process group was killed"
		if budget {
			want = "the shard exceeded its 10s total budget while an adapter ran; its process group was killed"
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

// STR-V0-015: the budget bounds the whole shard, not each invocation. Two
// invocations of seven seconds each pass a 12s per-invocation bound but not a
// 12s shard bound: the second is killed at the shard deadline, and once the
// deadline has passed nothing further launches. The 5s margin absorbs
// discovery and launch under host load, where 1s runs in a 1.6s budget left
// 0.6s and failed (V1-1439).
func TestSTRV0015BudgetBoundsTheShard(t *testing.T) {
	runner, m, _, _ := runnerFixture(t, "slow")
	m.Units = append(m.Units, Unit{Adapter: AdapterGo, Flag: "-run", Pattern: "^T$", Shard: 0, Slice: "0/2", Unit: "go:m/a"})
	digest, _ := Digest(m)
	runner.Budget = 12 * time.Second
	receipt := runner.Execute(context.Background(), m, digest, 0)
	want := "the shard exceeded its 12s total budget while an adapter ran; its process group was killed"
	if receipt.Result != ResultIncomplete || len(receipt.Invocations) != 2 || receipt.Invocations[0].Outcomes[0].Outcome != OutcomePass ||
		receipt.Incomplete[0] != want {
		t.Fatalf("shard budget: %s %d %v", receipt.Result, len(receipt.Invocations), receipt.Incomplete)
	}
	runner.Budget = time.Nanosecond
	spent := runner.Execute(context.Background(), m, digest, 0)
	if spent.Result != ResultIncomplete || len(spent.Invocations) != 0 ||
		spent.Incomplete[0] != "the shard's 1ns total budget ran out during runner discovery; nothing launched" {
		t.Fatalf("spent budget: %s %d %v", spent.Result, len(spent.Invocations), spent.Incomplete)
	}
	// Discovery spends the same budget: a hung version probe is killed at the
	// shard deadline, not after its own two-minute bound.
	runner, m, _, _ = runnerFixture(t, "slowenv")
	digest, _ = Digest(m)
	runner.Budget = 500 * time.Millisecond
	started := time.Now()
	hung := runner.Execute(context.Background(), m, digest, 0)
	if elapsed := time.Since(started); elapsed > 20*time.Second || hung.Result != ResultIncomplete ||
		len(hung.Invocations) != 0 || len(hung.Incomplete) == 0 || !strings.HasPrefix(hung.Incomplete[0], "go: ") {
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
}
