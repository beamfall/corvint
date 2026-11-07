package testsupport

import (
	"os"
	"testing"
)

// LiveTierTag is the build tag `make live-test` builds with; a tagged test binary
// always runs the live tier (AFP-V0-2700).
const LiveTierTag = "live"

// LiveTierSkipEnv, set to 1, skips the live tier in an untagged test binary
// (AFP-V0-2700). Only the go-product-shard Go test step of ci.yml sets it
// (AFP-V0-2702); every other run (local go test, corvint test, consumers of
// test-commands.json, make gate, batch-precheck, nightly and the merge-queue
// go-live job) runs the tier, so a change that breaks a live test fails before
// the merge queue (V1-4923).
const LiveTierSkipEnv = "CORVINT_SKIP_LIVE_TIER"

// LiveTierSkipReason is the fixed reason a live test skips with in a PR shard.
const LiveTierSkipReason = "live tier (AFP-V0-2700): skipped because CORVINT_SKIP_LIVE_TIER=1, which only the CI go-product-shard test step sets; the merge-queue go-live job runs this test"

// RequireLiveTier marks t as a live test: one that reads the live repository or
// builds release binaries (AFP-V0-2700). It skips t only when CORVINT_SKIP_LIVE_TIER
// is 1 and the binary was built without -tags live; cishards.LiveTests discovers
// the tier from these calls (AFP-V0-2701).
func RequireLiveTier(t testing.TB) {
	t.Helper()
	if liveTierSkipped(liveTier, os.Getenv(LiveTierSkipEnv)) {
		t.Skip(LiveTierSkipReason)
	}
}

func liveTierSkipped(tagged bool, env string) bool { return !tagged && env == "1" }
