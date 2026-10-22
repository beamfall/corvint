//go:build darwin || linux

package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// leaseProgressFields is the closed member list of the PENDING progress
// object (PSR-V0-112, PSR-V0-152) in canonical member order.
var leaseProgressFields = []string{"auditMillis", "completeMillis", "completeWalks", "completeWalksMoved", "prefixMillis", "prefixWalks", "prefixWalksMoved", "preparationRounds", "resumeDeclines", "resumedMillis", "resumedRounds", "resumedWalks", "updatedAt"}

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
	// PSR-V0-152 (V1-1626): the walk split, every member a decimal string;
	// the first round of a fresh store is one settled complete walk.
	if got := p.Obj.Keys; !slices.Equal(got, leaseProgressFields) {
		t.Fatalf("progress members %v, want %v", got, leaseProgressFields)
	}
	for _, key := range leaseProgressFields[:len(leaseProgressFields)-1] {
		if _, err := wire.ParseSize(key, field(p, key).Str); err != nil {
			t.Fatalf("progress %s: %v (%s)", key, err, wire.Encode(p))
		}
	}
	if field(p, "completeWalks").Str != "1" || field(p, "completeWalksMoved").Str != "0" || field(p, "prefixWalks").Str != "0" || field(p, "resumedWalks").Str != "0" || field(p, "resumeDeclines").Str != "0" {
		t.Fatalf("walk split: %s", wire.Encode(p))
	}
	if _, err := time.Parse(time.RFC3339, field(p, "updatedAt").Str); err != nil {
		t.Fatalf("updatedAt: %v %s", err, x.stdout)
	}
	// PSR-V0-152: progress.json keeps the closed shape the base revision's
	// reader decodes strictly, so an older --attach still reports its
	// counters; the walk split lives only in progress-walks.json.
	dir := cli.LeaseDetachDir(repo, "claim-progress")
	raw, err := os.ReadFile(filepath.Join(dir, "progress.json"))
	if err != nil {
		t.Fatal(err)
	}
	var older struct {
		Profile           string `json:"profile"`
		PreparationRounds int    `json:"preparationRounds"`
		ResumedRounds     int    `json:"resumedRounds"`
		AuditMillis       int64  `json:"auditMillis"`
		UpdatedAt         string `json:"updatedAt"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&older); err != nil || older.PreparationRounds != 1 || older.Profile != "taskman-detached-lease/0" || older.UpdatedAt != field(p, "updatedAt").Str {
		t.Fatalf("older reader over a new progress.json: %+v %v (%s)", older, err, raw)
	}
	// A progress.json an older child wrote, with no sibling, reports its
	// counters with the walk split as 0.
	if err := os.Remove(filepath.Join(dir, "progress-walks.json")); err != nil {
		t.Fatal(err)
	}
	x = cliRun(t, root, "claim", "--attach", "--request-id", "claim-progress", "--wait", "0")
	p = field(x.res.Items[0], "progress")
	if x.code != 75 || p.Obj == nil || !slices.Equal(p.Obj.Keys, leaseProgressFields) || field(p, "preparationRounds").Str != "1" || field(p, "completeWalks").Str != "0" {
		t.Fatalf("attach over an older child's progress.json: code %d %s", x.code, x.stdout)
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

// TestPSRV0111_CeilingRelaunchesOnSameRequestID: a detached child that ends
// at its ceiling keeps a retryable LOCK_TIMEOUT (ATR-V0-053). Once no child
// runs the record, repeating the same --detach starts a new child under that
// request id, which completes (ATR-V0-066), and --attach then returns the
// new kept result (PSR-V0-111, V1-1617; ATR-V0-021..023).
func TestPSRV0111_CeilingRelaunchesOnSameRequestID(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	t.Setenv("CORVINT_TEST_LEASE_DETACH_CEILING_MS", "1500")
	held := holdPreparation(t, repo)
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-ceiling", "--detach", "--wait", "60"}
	first := cliRun(t, root, claim...)
	if first.res.Outcome == wire.OutcomeOK || first.code == 75 || !slices.Contains(first.res.Codes, wire.CodeLockTimeout) {
		t.Fatalf("ceiling: code %d %s", first.code, first.stdout)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	again := cliRun(t, root, claim...)
	if again.code != 0 || again.res.Outcome != wire.OutcomeOK {
		t.Fatalf("repeated --detach after the ceiling: code %d %s", again.code, again.stdout)
	}
	if attach := cliRun(t, root, "claim", "--attach", "--request-id", "claim-ceiling", "--wait", "60"); !bytes.Equal(attach.stdout, again.stdout) {
		t.Fatalf("--attach after the relaunch: %s, want %s", attach.stdout, again.stdout)
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

// TestPSRV0185_CompactHelpNamesFastRoute: receipt compact help names the
// checkpoint routes that restore no covered elided receipt, and says
// compaction drops both checkpoints so the next write and read pay one
// complete audit (PSR-V0-185, V1-1785, GitHub #1063 ask 4).
func TestPSRV0185_CompactHelpNamesFastRoute(t *testing.T) {
	var out bytes.Buffer
	if code := cli.Run(cli.Env{Cwd: t.TempDir(), Args: []string{"receipt", "compact", "--help", "--verbose"}, Stdout: &out}); code != 0 {
		t.Fatalf("help: code %d %s", code, out.String())
	}
	for _, want := range []string{"CAL-V0-116 writer-checkpoint route", "resumes at the read checkpoint", "restore no elided receipt that checkpoint covers", "the next write and the next read each run one complete audit", "PSR-V0-185"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("receipt compact help lacks %q: %s", want, out.String())
		}
	}
}
