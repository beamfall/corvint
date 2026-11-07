package testsupport

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// recordingTB observes whether RequireLiveTier skipped.
type recordingTB struct {
	testing.TB
	skipped string
}

func (r *recordingTB) Helper()          {}
func (r *recordingTB) Skip(args ...any) { r.skipped = args[0].(string) }

// TestRequireLiveTierRunsUnlessThePRShardSkips_AFPV02700 pins the default: a live
// test runs unless CORVINT_SKIP_LIVE_TIER is exactly 1 in an untagged binary, and
// then skips with the fixed reason; -tags live always runs it.
func TestRequireLiveTierRunsUnlessThePRShardSkips_AFPV02700(t *testing.T) {
	for _, c := range []struct {
		tagged bool
		env    string
		want   bool
	}{{false, "", false}, {false, "0", false}, {false, "true", false}, {false, "1", true}, {true, "", false}, {true, "1", false}} {
		if got := liveTierSkipped(c.tagged, c.env); got != c.want {
			t.Errorf("liveTierSkipped(%v, %q) = %v, want %v", c.tagged, c.env, got, c.want)
		}
	}
	t.Setenv(LiveTierSkipEnv, "")
	r := &recordingTB{TB: t}
	RequireLiveTier(r)
	if r.skipped != "" {
		t.Fatalf("a run without %s skipped: %q", LiveTierSkipEnv, r.skipped)
	}
	t.Setenv(LiveTierSkipEnv, "1")
	r = &recordingTB{TB: t}
	RequireLiveTier(r)
	if liveTier && r.skipped != "" {
		t.Fatalf("live build skipped: %q", r.skipped)
	}
	if !liveTier && r.skipped != LiveTierSkipReason {
		t.Fatalf("skip reason = %q, want %q", r.skipped, LiveTierSkipReason)
	}
}

const liveFaultRole = "CORVINT_TESTSUPPORT_LIVE_FAULT"

// TestLiveTierFaultInjectionHelper is the injected broken live test of
// TestBrokenLiveTestFailsAnOrdinaryRun_AFPV02700; it does nothing in an ordinary run.
func TestLiveTierFaultInjectionHelper(t *testing.T) {
	if os.Getenv(liveFaultRole) != "1" {
		t.Skip("helper of TestBrokenLiveTestFailsAnOrdinaryRun_AFPV02700")
	}
	RequireLiveTier(t)
	t.Fatal("injected live-test failure")
}

// TestBrokenLiveTestFailsAnOrdinaryRun_AFPV02700 (V1-4923) injects a failing live
// test and runs it the way local go test, corvint test and test-commands.json
// consumers do, with no tier variable: the run fails. Only the PR-shard variable
// in an untagged binary turns the same failure into a skip.
func TestBrokenLiveTestFailsAnOrdinaryRun_AFPV02700(t *testing.T) {
	run := func(extra ...string) (bool, string) {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestLiveTierFaultInjectionHelper$", "-test.v")
		for _, kv := range os.Environ() {
			if !strings.HasPrefix(kv, LiveTierSkipEnv+"=") {
				cmd.Env = append(cmd.Env, kv)
			}
		}
		cmd.Env = append(append(cmd.Env, liveFaultRole+"=1"), extra...)
		out, err := cmd.CombinedOutput()
		return err == nil, string(out)
	}
	if ok, out := run(); ok || !strings.Contains(out, "injected live-test failure") {
		t.Fatalf("an ordinary run of a broken live test passed (ok=%v):\n%s", ok, out)
	}
	ok, out := run(LiveTierSkipEnv + "=1")
	if liveTier {
		if ok || !strings.Contains(out, "injected live-test failure") {
			t.Fatalf("a -tags live run skipped a broken live test (ok=%v):\n%s", ok, out)
		}
		return
	}
	if !ok || !strings.Contains(out, "--- SKIP: TestLiveTierFaultInjectionHelper") {
		t.Fatalf("the PR-shard variable did not skip the live test (ok=%v):\n%s", ok, out)
	}
}
