package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// stallSecret is a token the shared secret screen matches, assembled so
// the source holds no literal credential.
var stallSecret = "ghp_" + strings.Repeat("a1B2", 9)

// PSR-V0-273: a --detail-file under a symlinked ancestor is refused
// ERROR UNSUPPORTED_FILESYSTEM and records nothing; the same file named by its
// resolved path is observed. The tests here name detail files under
// resolvedTempDir because macOS links /var to /private/var (V1-2711).
func TestPSRV0273_PoolObserveDetailFileUnderSymlinkedAncestor(t *testing.T) {
	r, _, _ := poolStatusRepo(t)
	base := resolvedTempDir(t)
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "health.log"), []byte("probe timed out\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	x := observeFailed(t, r.Root, "b", "health", filepath.Join(link, "health.log"), "via-link")
	if x.res.Outcome != wire.OutcomeError || !hasCode(x.res, wire.CodeUnsupportedFilesystem) {
		t.Fatalf("symlinked ancestor: %s", x.stdout)
	}
	if b := poolStatusMembers(t, r.Root)["db/b"]; field(b, "lastHealth").Str != "NOT_OBSERVED" {
		t.Fatalf("refused observe recorded health: %s", wire.Encode(b))
	}
	if x := observeFailed(t, r.Root, "b", "health", filepath.Join(real, "health.log"), "via-real"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("resolved path: %s", x.stdout)
	}
	if b := poolStatusMembers(t, r.Root)["db/b"]; field(field(b, "lastHealth"), "consecutive").Str != "1" {
		t.Fatalf("resolved observe not recorded: %s", wire.Encode(b))
	}
}

func observeFailed(t *testing.T, root, member, kind, file, request string) run {
	t.Helper()
	return atm(t, root, nil, "pool", "observe", "--member", member, "--kind", kind, "--result", "FAILED", "--detail-file", file, "--request-id", request)
}

// PSR-V0-260..264: `pool observe` records a bounded, redacted failed cleanup
// of a QUARANTINED member; three identical failures stall it, pool status
// shows the typed condition with the redacted excerpt, a replay keeps the
// count, and a claim that can only reach the stalled member is refused
// POOL_MEMBER_RECOVERY_STALLED instead of a bare RESOURCE_COLLISION.
func TestPSRV0260_PoolObserveStallsQuarantinedMember(t *testing.T) {
	r, _, _ := poolStatusRepo(t)
	b := poolStatusMembers(t, r.Root)["db/b"]
	if _, err := time.Parse(time.RFC3339, field(field(b, "quarantine"), "since").Str); err != nil {
		t.Fatalf("quarantine since is not a recorded time: %s", wire.Encode(b))
	}
	if !obligation.MatchesSecretScreen(stallSecret) {
		t.Fatal("fixture secret is not screened")
	}
	detail := filepath.Join(resolvedTempDir(t), "cleanup.log")
	if err := os.WriteFile(detail, []byte("drop schema failed: connection refused\nauth "+stallSecret+"\nexit 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for i, req := range []string{"obs-1", "obs-2", "obs-3"} {
		if x := observeFailed(t, r.Root, "b", "cleanup", detail, req); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("observe %d: %s", i, x.stdout)
		}
	}
	if x := observeFailed(t, r.Root, "b", "cleanup", detail, "obs-3"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("replay: %s", x.stdout)
	}
	b = poolStatusMembers(t, r.Root)["db/b"]
	cleanup, stall := field(b, "lastCleanup"), field(b, "recoveryStalled")
	if field(cleanup, "result").Str != "FAILED" || field(cleanup, "passed").Bool || field(cleanup, "consecutive").Str != "3" || field(cleanup, "detailSha256").Str == "" {
		t.Fatalf("lastCleanup %s", wire.Encode(b))
	}
	excerpt := field(stall, "excerpt").Str
	if field(stall, "code").Str != wire.CodePoolMemberRecoveryStalled || field(stall, "trigger").Str != "CONSECUTIVE_FAILURES" || field(stall, "consecutive").Str != "3" || field(stall, "detailSha256").Str != field(cleanup, "detailSha256").Str || field(stall, "since").Str != field(field(b, "quarantine"), "since").Str {
		t.Fatalf("recoveryStalled %s", wire.Encode(b))
	}
	if strings.Contains(excerpt, stallSecret) || !strings.Contains(excerpt, "[REDACTED]") || !strings.Contains(excerpt, "connection refused") {
		t.Fatalf("excerpt %q", excerpt)
	}
	status := atm(t, r.Root, nil, "pool", "status")
	if strings.Contains(string(status.stdout), stallSecret) {
		t.Fatal("pool status leaks the secret")
	}
	for _, p := range field(status.res.Items[0], "pools").Arr {
		if field(p, "recoveryStalled").Kind != wire.KindNull && field(p, "recoveryStalled").Kind != 0 {
			t.Fatalf("a pool with an unstalled member shows a pool summary %s", wire.Encode(p))
		}
	}
	next := planTicket(t, r.Root, "next", "P2", `["next"]`)
	x := atm(t, r.Root, nil, "claim", next, "--holder", "builder", "--request-id", "claim-next", "--pool", "db", "--stage", "implement", "--exclude-member", "a", "--exclude-member", "review")
	if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodePoolMemberRecoveryStalled) || !strings.Contains(string(x.stdout), "pool status --pool db") {
		t.Fatalf("claim of a stalled pool %s", x.stdout)
	}
}

// PSR-V0-260: the observe flags are required together and only on pool
// observe, --evidence is not taken, and a member that is not QUARANTINED is
// refused QUIESCENCE_UNPROVED.
func TestPSRV0260_PoolObserveFlagsAndState(t *testing.T) {
	r, _, _ := poolStatusRepo(t)
	detail := filepath.Join(resolvedTempDir(t), "health.log")
	if err := os.WriteFile(detail, []byte("probe timed out\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"pool", "observe", "--member", "b", "--result", "FAILED", "--detail-file", detail, "--request-id", "m1"},
		{"pool", "observe", "--member", "b", "--kind", "health", "--detail-file", detail, "--request-id", "m2"},
		{"pool", "observe", "--member", "b", "--kind", "health", "--result", "FAILED", "--request-id", "m3"},
		{"pool", "observe", "--member", "b", "--kind", "health", "--result", "PASSED", "--detail-file", detail, "--request-id", "m4"},
		{"pool", "observe", "--member", "b", "--kind", "other", "--result", "FAILED", "--detail-file", detail, "--request-id", "m5"},
		{"pool", "observe", "--member", "b", "--kind", "health", "--result", "FAILED", "--detail-file", detail, "--evidence", "local:x", "--request-id", "m6"},
		{"pool", "confirm-safe", "--member", "b", "--kind", "health", "--evidence", "local:x", "--reason", "r", "--request-id", "m7"},
	} {
		if x := atm(t, r.Root, nil, args...); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMalformed) {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	if x := observeFailed(t, r.Root, "a", "health", detail, "obs-a"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeQuiescenceUnproved) {
		t.Fatalf("observe of an ALLOCATED member %s", x.stdout)
	}
	if x := observeFailed(t, r.Root, "b", "health", filepath.Join(t.TempDir(), "missing"), "obs-missing"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("observe of a missing detail file %s", x.stdout)
	}
	if x := observeFailed(t, r.Root, "b", "health", detail, "obs-b"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("observe b %s", x.stdout)
	}
	b := poolStatusMembers(t, r.Root)["db/b"]
	if field(field(b, "lastHealth"), "consecutive").Str != "1" || field(b, "lastCleanup").Str != "NOT_OBSERVED" || field(b, "recoveryStalled").Kind == wire.KindObject {
		t.Fatalf("one failed health %s", wire.Encode(b))
	}
}

// PSR-V0-263, PSR-V0-264: once every member of a pool is QUARANTINED and
// stalled, pool status adds the pool-level summary naming them and a claim
// on the pool is refused POOL_MEMBER_RECOVERY_STALLED.
func TestPSRV0263_PoolSummaryWhenEveryMemberStalled(t *testing.T) {
	r, held, _ := poolStatusRepo(t)
	if x := atm(t, r.Root, nil, "release", "--attempt", held, "--generation", "1", "--request-id", "release-a"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release a %s", x.stdout)
	}
	review := planTicket(t, r.Root, "review", "P2", `["review"]`)
	c := atm(t, r.Root, nil, "claim", review, "--holder", "reviewer", "--request-id", "claim-review", "--pool", "db", "--stage", "review")
	if c.res.Outcome != wire.OutcomeOK || field(field(c.res.Items[0], "poolAllocation"), "memberId").Str != "review" {
		t.Fatalf("claim review %s", c.stdout)
	}
	if x := atm(t, r.Root, nil, "release", "--attempt", field(c.res.Items[0], "attemptId").Str, "--generation", field(c.res.Items[0], "generation").Str, "--request-id", "release-review"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release review %s", x.stdout)
	}
	detail := filepath.Join(resolvedTempDir(t), "health.log")
	if err := os.WriteFile(detail, []byte("probe timed out\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"a", "b", "review"} {
		for i := 1; i <= 3; i++ {
			if x := observeFailed(t, r.Root, m, "health", detail, m+"-obs-"+string(rune('0'+i))); x.res.Outcome != wire.OutcomeOK {
				t.Fatalf("observe %s %d: %s", m, i, x.stdout)
			}
		}
	}
	status := atm(t, r.Root, nil, "pool", "status", "--pool", "db")
	p := field(status.res.Items[0], "pools").Arr[0]
	summary := field(p, "recoveryStalled")
	if field(summary, "code").Str != wire.CodePoolMemberRecoveryStalled || len(field(summary, "members").Arr) != 3 {
		t.Fatalf("pool summary %s", wire.Encode(p))
	}
	next := planTicket(t, r.Root, "next", "P2", `["next"]`)
	x := atm(t, r.Root, nil, "claim", next, "--holder", "builder", "--request-id", "claim-next", "--pool", "db", "--stage", "implement")
	if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodePoolMemberRecoveryStalled) {
		t.Fatalf("claim of a stalled pool %s", x.stdout)
	}
}
