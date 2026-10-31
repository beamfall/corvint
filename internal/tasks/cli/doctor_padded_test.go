//go:build darwin || linux

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// paddedWalkStore claims attempts live attempts through the command line,
// records one real heartbeat for each, and pads the journal with heartbeats
// more synthetic liveness receipts, elided as liveness compaction elides them
// (CAL-V0-360). It returns the store root and what the builder wrote.
func paddedWalkStore(t *testing.T, attempts, heartbeats int) (string, fixture.PadReport) {
	t.Helper()
	n := strconv.Itoa(attempts)
	root, _ := leaseCLIStoreWith(t, 0, time.Now().UTC().Add(-time.Minute), func(p *wire.Object) {
		p.Set("capacity", wire.ObjectValue(wire.NewObject().Set("classes", wire.Array()).Set("maxActiveAttempts", wire.String(n)).Set("maxWorkersTotal", wire.String(n))))
	}, "")
	for i := 0; i < attempts; i++ {
		k := strconv.Itoa(i)
		id := planTicket(t, root, "pad-"+k, "P1", `["src/`+k+`/"]`)
		c := handoffCLI(t, root, "claim", id, "--holder", "w"+k, "--stage", "implement", "--request-id", "pad-claim-"+k)
		if c.res.Outcome != wire.OutcomeOK {
			t.Fatalf("claim %d: %s", i, c.stdout)
		}
		attempt, generation := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
		if b := handoffCLI(t, root, "attempt", "heartbeat", "--attempt", attempt, "--generation", generation, "--request-id", "pad-beat-"+k); b.res.Outcome != wire.OutcomeOK {
			t.Fatalf("heartbeat %d: %s", i, b.stdout)
		}
	}
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, fixture.PadLiveness(t, repo.StateDir, fixture.PadOptions{Heartbeats: heartbeats, Elide: true})
}

// CAL-V0-360, V1-2137: the padded-journal builder writes a store that the
// complete audit and the doctor accept as one built by real heartbeats, with
// elided liveness receipts that keep a whole record of each attempt fewer
// than snapshot.MaxElidedRun receipts before every one of them, so the
// doctor's bounded backward walk (TQD-V0-004) always finds it. The second
// case interleaves enough attempts that an elision run counted in each
// attempt's own receipts, not in journal sequence, would put the whole
// record past the walk bound and stop the scan. The audit still refuses a
// padded receipt that was changed afterwards.
func TestCALV0360_PaddedLivenessStoreAuditsAndWalks(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		attempts, heartbeats int
		capped               bool // the journal is longer than the scan bound
	}{
		{"within-the-scan-bound", 3, 2400, false},
		{"past-the-scan-bound", 6, 6000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, rep := paddedWalkStore(t, tc.attempts, tc.heartbeats)
			if rep.Appended != tc.heartbeats || rep.Attempts != tc.attempts || rep.Elided+rep.Whole != tc.heartbeats {
				t.Fatalf("pad report %+v", rep)
			}
			// Each attempt keeps its last receipt whole and one more for every
			// snapshot.MaxElidedRun receipts of journal it spans.
			if rep.Whole < tc.attempts || rep.Whole > tc.attempts*(2+tc.heartbeats/snapshot.MaxElidedRun) || rep.Elided < tc.heartbeats/2 {
				t.Fatalf("elision %+v", rep)
			}
			audit := atm(t, root, nil, "receipt", "audit")
			if audit.code != 0 || field(audit.res.Items[0], "headSeq").Str != strconv.FormatUint(rep.LastSeq, 10) {
				t.Fatalf("audit of the padded store: code %d %s", audit.code, audit.stdout)
			}
			frozen := time.Now().UTC().Add(time.Hour)
			defer cli.SetDoctorClock(func() time.Time { return frozen })()
			reads := map[string]int{}
			restore := cli.CountDoctorReceiptReads(reads)
			x, item := doctorOK(t, root)
			restore()
			scan := field(item, "scan")
			for _, w := range x.res.Warnings {
				if strings.Contains(w, "doctor scan stopped") {
					t.Fatalf("the scan stopped on a receipt of the padded store: %s", w)
				}
			}
			// The scan covers the whole journal, or stops at its 4,096-receipt
			// bound (TQD-V0-004), and in both cases not on a padded receipt.
			if got, want := field(scan, "receipts").Str, field(scan, "headSeq").Str; tc.capped && (got != "4096" || !field(scan, "truncated").Bool) || !tc.capped && (got != want || field(scan, "truncated").Bool) {
				t.Fatalf("scan %s", wire.Encode(scan))
			}
			most := 0
			for _, n := range reads {
				most = maxInt(most, n)
			}
			if len(reads) == 0 || most > 2 {
				t.Fatalf("doctor read %d receipts, one of them %d times", len(reads), most)
			}

			// A padded receipt rewritten after the fact is not a valid chain.
			repo, err := intent.Resolve(root)
			if err != nil {
				t.Fatal(err)
			}
			name, _ := snapshot.ReceiptName(rep.LastSeq / 2)
			path := filepath.Join(repo.StateDir, "receipts", name)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			v, err := wire.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			v.Obj.Set("recordedAt", wire.String("2030-01-01T00:00:00Z"))
			if err := os.WriteFile(path, wire.EncodeFile(v), 0o644); err != nil {
				t.Fatal(err)
			}
			if bad := atm(t, root, nil, "receipt", "audit"); bad.code == 0 {
				t.Fatalf("the audit accepted a rewritten padded receipt: %s", bad.stdout)
			}
		})
	}
}

// TestCALV0360_PaddedDoctorWalkCost is the opt-in V1-1193 / V1-2137
// measurement: it builds CORVINT_DOCTOR_WALK_COST_PADDED receipts through
// the padded-journal builder (CORVINT_DOCTOR_WALK_ATTEMPTS live attempts,
// default 24) and reports the best of five interleaved doctor runs per walk
// mode, as TestTQDV0045_WalkCostMeasurement does for a store of real writes.
func TestCALV0360_PaddedDoctorWalkCost(t *testing.T) {
	n, err := strconv.Atoi(os.Getenv("CORVINT_DOCTOR_WALK_COST_PADDED"))
	if err != nil || n < 1 {
		t.Skip("set CORVINT_DOCTOR_WALK_COST_PADDED=RECEIPTS to measure")
	}
	attempts := 24
	if a, err := strconv.Atoi(os.Getenv("CORVINT_DOCTOR_WALK_ATTEMPTS")); err == nil && a > 0 {
		attempts = a
	}
	start := time.Now()
	root, rep := paddedWalkStore(t, attempts, n)
	t.Logf("built %d attempts, %d padded receipts (%d elided, %d whole), head %d: padding took %v, the store %v", attempts, rep.Appended, rep.Elided, rep.Whole, rep.LastSeq, rep.Elapsed.Round(time.Millisecond), time.Since(start).Round(time.Millisecond))
	audit := time.Now()
	if x := atm(t, root, nil, "receipt", "audit"); x.code != 0 {
		t.Fatalf("audit: %s", x.stdout)
	}
	t.Logf("receipt audit of the padded store: %v", time.Since(audit))
	now := time.Now().UTC().Add(time.Duration(n/100+3600) * time.Second)
	// The builder runs no audit, so no writer retained a read checkpoint and
	// every doctor run walks the whole journal (journalAudit FULL). A store
	// of real writes keeps one; retain it as a writer would and measure the
	// CHECKPOINT_PLUS_TAIL doctor too (TQD-V0-820, V1-1193).
	t.Log("without a read checkpoint:")
	measureDoctorWalk(t, root, now)
	retainPaddedCheckpoint(t, root)
	t.Log("with a writer-retained read checkpoint:")
	measureDoctorWalk(t, root, now)
}

// retainPaddedCheckpoint writes the read checkpoint of a complete audit, as a
// writer's commit retains it (CAL-V0-059).
func retainPaddedCheckpoint(t *testing.T, root string) {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".taskman", "queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	var queue struct {
		QueueID string `json:"queueId"`
	}
	if err := json.Unmarshal(raw, &queue); err != nil {
		t.Fatal(err)
	}
	id, err := wire.ParseQueueID("queueId", queue.QueueID)
	if err != nil {
		t.Fatal(err)
	}
	reader := journal.Reader{QueueID: id, Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.IntentRoot()}, PrimaryWorktree: repo.PrimaryWorktree, SelectState: true}
	res, err := reader.Audit()
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := res.Checkpoint()
	if checkpoint == nil {
		t.Fatalf("the complete audit derived no read checkpoint (mode %s, %s)", res.Mode, res.StructuralConsistency)
	}
	if err := os.WriteFile(journal.CheckpointPath(repo.StateDir), checkpoint.Encode(), 0o644); err != nil {
		t.Fatal(err)
	}
}
