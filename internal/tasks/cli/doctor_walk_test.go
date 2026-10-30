//go:build darwin || linux

package cli_test

import (
	"context"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// walkStore claims attempts live attempts and heartbeats each of them rounds
// times, so the doctor's scan holds runs of elided liveness receipts whose
// retained records lie far behind them (CAL-V0-300).
func walkStore(t *testing.T, attempts, rounds int) string {
	t.Helper()
	n := strconv.Itoa(attempts)
	root, _ := leaseCLIStoreWith(t, 0, time.Now().UTC().Add(-time.Minute), func(p *wire.Object) {
		p.Set("capacity", wire.ObjectValue(wire.NewObject().Set("classes", wire.Array()).Set("maxActiveAttempts", wire.String(n)).Set("maxWorkersTotal", wire.String(n))))
	}, "")
	type live struct{ attempt, generation string }
	var held []live
	for i := 0; i < attempts; i++ {
		k := strconv.Itoa(i)
		id := planTicket(t, root, "walk-"+k, "P1", `["src/`+k+`/"]`)
		c := handoffCLI(t, root, "claim", id, "--holder", "w"+k, "--stage", "implement", "--request-id", "walk-claim-"+k)
		if c.res.Outcome != wire.OutcomeOK {
			t.Fatalf("claim %d: %s", i, c.stdout)
		}
		held = append(held, live{field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str})
	}
	for r := 0; r < rounds; r++ {
		for i, h := range held {
			id := "walk-beat-" + strconv.Itoa(r) + "-" + strconv.Itoa(i)
			if b := handoffCLI(t, root, "attempt", "heartbeat", "--attempt", h.attempt, "--generation", h.generation, "--request-id", id); b.res.Outcome != wire.OutcomeOK {
				t.Fatalf("heartbeat %s: %s", id, b.stdout)
			}
		}
	}
	// Liveness compaction (CAL-V0-302) rewrites every superseded heartbeat
	// as an elided receipt whose attempt record the doctor restores by a
	// backward walk.
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	if rep, err := store.CompactLiveness(context.Background(), repo, false); err != nil || rep.Elided == 0 {
		t.Fatalf("compaction: %+v %v", rep, err)
	}
	return root
}

// V1-1193 (TQD-V0-045): elided-record walks for different attempts share one
// read and decode of each receipt, so a scan reads every receipt at most
// twice (once in the scan, once in a walk) where unshared walks re-read the
// same receipts per attempt, and the complete encoded findings and scan
// values are byte-identical either way.
func TestTQDV0045_ElidedWalksShareReceipts(t *testing.T) {
	root := walkStore(t, 6, 12)
	frozen := time.Now().UTC()
	restoreClock := cli.SetDoctorClock(func() time.Time { return frozen })
	defer restoreClock()
	run := func(shared bool) (wire.Value, map[string]int) {
		reads := map[string]int{}
		restore := cli.CountDoctorReceiptReads(reads)
		defer restore()
		if !shared {
			defer cli.SetDoctorWalkUnshared()()
		}
		_, item := doctorOK(t, root)
		return item, reads
	}
	before, unshared := run(false)
	after, shared := run(true)
	assertDoctorModesEqual(t, before, after)
	if field(field(after, "scan"), "truncated").Bool {
		t.Fatalf("scan truncated: %s", wire.Encode(field(after, "scan")))
	}
	most := func(reads map[string]int) (max, total int) {
		for _, n := range reads {
			max, total = maxInt(max, n), total+n
		}
		return max, total
	}
	sharedMax, sharedTotal := most(shared)
	unsharedMax, unsharedTotal := most(unshared)
	if sharedMax > 2 {
		t.Fatalf("a receipt was read %d times with shared walks (want <= 2)", sharedMax)
	}
	// The fixture must exercise repeated walks, or the bound above proves
	// nothing.
	if unsharedMax <= 2 || unsharedTotal <= sharedTotal {
		t.Fatalf("fixture has no repeated walks: unshared max %d total %d, shared total %d", unsharedMax, unsharedTotal, sharedTotal)
	}
	t.Logf("receipt reads: unshared %d (max %d per receipt), shared %d (max %d)", unsharedTotal, unsharedMax, sharedTotal, sharedMax)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// TestTQDV0045_WalkCostMeasurement builds CORVINT_DOCTOR_WALK_COST receipts
// of heartbeats over CORVINT_DOCTOR_WALK_ATTEMPTS live attempts (default 24)
// and reports the best of five interleaved doctor runs per walk mode.
func TestTQDV0045_WalkCostMeasurement(t *testing.T) {
	n, err := strconv.Atoi(os.Getenv("CORVINT_DOCTOR_WALK_COST"))
	if err != nil || n < 1 {
		t.Skip("set CORVINT_DOCTOR_WALK_COST=RECEIPTS to measure")
	}
	attempts := 24
	if a, err := strconv.Atoi(os.Getenv("CORVINT_DOCTOR_WALK_ATTEMPTS")); err == nil && a > 0 {
		attempts = a
	}
	built := time.Now()
	root := walkStore(t, attempts, max(1, n/attempts))
	t.Logf("built %d attempts x %d heartbeats in %v", attempts, max(1, n/attempts), time.Since(built))
	measureDoctorWalk(t, root, built)
}

// measureDoctorWalk runs ten interleaved doctor scans of root, five per walk
// mode, with the doctor clock frozen at now, and logs the best wall and CPU
// time of each mode and the scan.
func measureDoctorWalk(t *testing.T, root string, now time.Time) {
	t.Helper()
	cpu := func() time.Duration {
		var ru syscall.Rusage
		_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
		return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
	}
	restoreClock := cli.SetDoctorClock(func() time.Time { return now })
	defer restoreClock()
	var wall, used [2]time.Duration
	var items [2]wire.Value
	for i := 0; i < 10; i++ {
		mode := i % 2
		restore := func() {}
		if mode == 0 {
			restore = cli.SetDoctorWalkUnshared()
		}
		start, c0 := time.Now(), cpu()
		_, items[mode] = doctorOK(t, root)
		w, c := time.Since(start), cpu()-c0
		restore()
		if i < 2 || w < wall[mode] {
			wall[mode] = w
		}
		if i < 2 || c < used[mode] {
			used[mode] = c
		}
	}
	t.Logf("doctor best of 5 (wall / cpu): unshared walks %v / %v, shared walks %v / %v; scan %s", wall[0], used[0], wall[1], used[1], wire.Encode(field(items[1], "scan")))
	assertDoctorModesEqual(t, items[0], items[1])
}
