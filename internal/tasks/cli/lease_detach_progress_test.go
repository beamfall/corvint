//go:build darwin || linux

package cli_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestPSRV0112_PendingReportsProgress: a detached lease command still
// waiting for the store reports, in its PENDING item, the child's
// preparation rounds, resumed rounds and audit time after its first round,
// and progress null before it (PSR-V0-112, V1-1617). The member is advisory:
// the kept result is unchanged.
func TestPSRV0112_PendingReportsProgress(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	held := holdPreparation(t, repo)
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-progress", "--detach", "--wait", "0"}
	x := cliRun(t, root, claim...)
	if x.code != 75 || field(x.res.Items[0], "state").Str != "PENDING" {
		t.Fatalf("detach: code %d %s", x.code, x.stdout)
	}
	if p, ok := x.res.Items[0].Obj.Get("progress"); !ok || (p.Kind != wire.KindNull && p.Obj == nil) {
		t.Fatalf("PENDING item without a progress member: %s", x.stdout)
	}
	// The held preparation gate stops the claim after its first round.
	waitFile(t, filepath.Join(cli.LeaseDetachDir(repo, "claim-progress"), "progress.json"))
	x = cliRun(t, root, "claim", "--attach", "--request-id", "claim-progress", "--wait", "0")
	p := field(x.res.Items[0], "progress")
	if x.code != 75 || p.Obj == nil || field(p, "preparationRounds").Str != "1" || field(p, "resumedRounds").Str != "0" || field(p, "auditMillis").Str == "" {
		t.Fatalf("attach: code %d %s", x.code, x.stdout)
	}
	if _, err := time.Parse(time.RFC3339, field(p, "updatedAt").Str); err != nil {
		t.Fatalf("updatedAt: %v %s", err, x.stdout)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if x = cliRun(t, root, "claim", "--attach", "--request-id", "claim-progress", "--wait", "60"); x.code != 0 {
		t.Fatalf("attach after release: %s", x.stdout)
	}
	if _, ok := x.res.Items[0].Obj.Get("progress"); ok {
		t.Fatalf("kept result carries progress: %s", x.stdout)
	}
}

// TestPSRV0111_CeilingNeedsAFreshRequestID: once a detached child ends at
// its ceiling, its kept result answers that request id, so repeating the
// same --detach returns it and starts nothing; a new request id starts a
// fresh child, which completes (PSR-V0-111, V1-1617; ATR-V0-021..023).
func TestPSRV0111_CeilingNeedsAFreshRequestID(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	t.Setenv("CORVINT_TEST_LEASE_DETACH_CEILING_MS", "1500")
	held := holdPreparation(t, repo)
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-ceiling", "--detach", "--wait", "60"}
	first := cliRun(t, root, claim...)
	if first.res.Outcome == wire.OutcomeOK || first.code == 75 {
		t.Fatalf("ceiling: code %d %s", first.code, first.stdout)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if again := cliRun(t, root, claim...); !bytes.Equal(again.stdout, first.stdout) {
		t.Fatalf("repeated --detach after the ceiling: %s, want the kept %s", again.stdout, first.stdout)
	}
	fresh := cliRun(t, root, "claim", id, "--holder", "agent", "--request-id", "claim-after-ceiling", "--detach", "--wait", "60")
	if fresh.code != 0 || fresh.res.Outcome != wire.OutcomeOK {
		t.Fatalf("fresh request id after the ceiling: code %d %s", fresh.code, fresh.stdout)
	}
}

// TestPSRV0114_CompactHelpNotesWalkCost: receipt compact help says a
// complete audit restores each elided receipt, so compaction does not make
// that walk cheaper (PSR-V0-114, V1-1617, GitHub #1063).
func TestPSRV0114_CompactHelpNotesWalkCost(t *testing.T) {
	var out bytes.Buffer
	if code := cli.Run(cli.Env{Cwd: t.TempDir(), Args: []string{"receipt", "compact", "--help", "--verbose"}, Stdout: &out}); code != 0 {
		t.Fatalf("help: code %d %s", code, out.String())
	}
	if !strings.Contains(out.String(), "restores each elided receipt before validating it") || !strings.Contains(out.String(), "PSR-V0-114") {
		t.Fatalf("receipt compact help without the walk cost note: %s", out.String())
	}
}
