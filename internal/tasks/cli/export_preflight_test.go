package cli

import (
	"time"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// SetImplementationDigest replaces the preflight cache's check
// implementation digest for a test and returns the restore function.
func SetImplementationDigest(f func() (string, error)) func() {
	was := implementationDigest
	implementationDigest = f
	return func() { implementationDigest = was }
}

// SetLocalNow replaces the local cache and ledger clock for a test and
// returns the restore function.
func SetLocalNow(f func() time.Time) func() {
	was := localNow
	localNow = f
	return func() { localNow = was }
}

// PruneCacheDir is pruneCacheDir for a test.
func PruneCacheDir(dir string, maxEntries int, maxAge time.Duration) {
	pruneCacheDir(dir, maxEntries, maxAge)
}

// PreflightCacheBounds are the cache's entry count and age bounds.
const (
	PreflightCacheMaxEntries = preflightCacheMaxEntries
	PreflightCacheMaxAge     = preflightCacheMaxAge
)

// NormalizeFailure is the TOL-V0-051 failure-line normalization.
func NormalizeFailure(s string) string { return normalizeFailure(s) }

// RunBatchInputDigest is the TOL-V0-051 run-batch input digest of a fixed
// job with the given pool and passed environment names.
func RunBatchInputDigest(pool string, env []string) (string, string) {
	j := &runJob{cfg: &runConfig{Env: env}, commit: "0000000000000000000000000000000000000000", pool: pool}
	d, reason := runBatchInputDigest(j, "T-1", "", "", []string{"e2e/a.spec.ts"}, &ticket.ObligationLedger{}, nil, "OWNER")
	return string(d), reason
}

// FailureGuardRun runs one guarded preflight-shaped job against the
// ledger: admit, then record failed with lines. It returns the recorded
// status and count, or the admit refusal.
func FailureGuardRun(input string, threshold int, remediation string, failed bool, lines []string) (status, count string, err error) {
	g := startFailureGuard("preflight", "T-1", wire.Digest(input), "", threshold, remediation)
	if err := g.admit(); err != nil {
		return "", "", err
	}
	m := g.record(failed, lines)
	st, _ := m.Obj.Get("status")
	n, _ := m.Obj.Get("count")
	return st.Str, n.Str, nil
}
