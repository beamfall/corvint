package testsupport

import "testing"

// recordingTB observes whether RequireLiveTier skipped.
type recordingTB struct {
	testing.TB
	skipped string
}

func (r *recordingTB) Helper()          {}
func (r *recordingTB) Skip(args ...any) { r.skipped = args[0].(string) }

// TestRequireLiveTierFollowsTheTagAndEnv_AFPV02700 pins that only the live build
// tag or CORVINT_LIVE_TIER=1 enables the tier, and that outside it a live test
// skips with the fixed reason.
func TestRequireLiveTierFollowsTheTagAndEnv_AFPV02700(t *testing.T) {
	for _, c := range []struct {
		tagged bool
		env    string
		want   bool
	}{{false, "", false}, {false, "0", false}, {false, "true", false}, {false, "1", true}, {true, "", true}, {true, "0", true}} {
		if got := liveTierEnabled(c.tagged, c.env); got != c.want {
			t.Errorf("liveTierEnabled(%v, %q) = %v, want %v", c.tagged, c.env, got, c.want)
		}
	}
	t.Setenv(LiveTierEnv, "")
	r := &recordingTB{TB: t}
	RequireLiveTier(r)
	if liveTier && r.skipped != "" {
		t.Fatalf("live build skipped: %q", r.skipped)
	}
	if !liveTier && r.skipped != LiveTierSkipReason {
		t.Fatalf("default build skip reason = %q, want %q", r.skipped, LiveTierSkipReason)
	}
	t.Setenv(LiveTierEnv, "1")
	r = &recordingTB{TB: t}
	RequireLiveTier(r)
	if r.skipped != "" {
		t.Fatalf("CORVINT_LIVE_TIER=1 skipped: %q", r.skipped)
	}
}
