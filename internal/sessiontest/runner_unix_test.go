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
// fails, or starts a grandchild and sleeps.
const fakeGo = `#!/bin/sh
case "$1" in
env) [ "$FAKE_MODE" = noversion ] && exit 1; echo go1.27.1; exit 0;;
list) echo m/a; [ "$FAKE_MODE" = extra ] && echo m/x; exit 0;;
esac
echo "$@" >> "$FAKE_DIR/argv"
case "$FAKE_MODE" in
sleep)
  sleep 60 & echo $! > "$FAKE_DIR/grandchild"
  touch "$FAKE_DIR/started"
  sleep 60;;
fail) echo '{"Action":"fail","Package":"m/a"}'; exit 1;;
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
			// loaded host; 300ms was racy.
			runner.Budget = 2 * time.Second
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
			want = "an adapter invocation exceeded the 2s total budget; its process group was killed"
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
