package testsupport

import (
	"os"
	"testing"
)

// LiveTierTag is the build tag that enables the live test tier (AFP-V0-2700).
const LiveTierTag = "live"

// LiveTierEnv, set to 1, also enables the tier in an untagged test binary, so a
// targeted run (a doc gate, a batch-precheck drift test) reuses the build cache
// of the ordinary test binary instead of compiling a tagged one (AFP-V0-2700).
const LiveTierEnv = "CORVINT_LIVE_TIER"

// LiveTierSkipReason is the fixed reason a live test skips with outside the tier.
const LiveTierSkipReason = "live tier (AFP-V0-2700): reads the live repository or builds release binaries; run with -tags live (make live-test) or CORVINT_LIVE_TIER=1"

// RequireLiveTier skips t unless the test binary was built with -tags live or
// CORVINT_LIVE_TIER is 1 (AFP-V0-2700). A live test calls it first, so a PR
// shard, which sets neither, spends no time on it; `make live-test` runs the
// tier in the merge queue, the nightly release gates and `make gate`
// (AFP-V0-2701, AFP-V0-2702).
func RequireLiveTier(t testing.TB) {
	t.Helper()
	if !liveTierEnabled(liveTier, os.Getenv(LiveTierEnv)) {
		t.Skip(LiveTierSkipReason)
	}
}

func liveTierEnabled(tagged bool, env string) bool { return tagged || env == "1" }
