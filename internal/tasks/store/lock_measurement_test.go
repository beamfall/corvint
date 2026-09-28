package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0026_LockHoldMeasurement is an opt-in measurement, not a timing
// assertion in the unit suite. Its raw samples preserve failures of the bound.
func TestCALV0026_LockHoldMeasurement(t *testing.T) {
	output := os.Getenv("CORVINT_TASKS_LOCK_MEASUREMENT")
	if output == "" {
		t.Skip("set CORVINT_TASKS_LOCK_MEASUREMENT to a report path")
	}
	count, samples := 3000, 20
	if testing.Short() {
		count, samples = 3, 1
	}
	repo := importStore(t, "ROADMAP")
	items := make([]string, count)
	for i := range items {
		id := fmt.Sprintf("PERF-%04d", i)
		items[i] = importItem(id, "synthetic lease measurement", func(o *wire.Object) {
			o.Set("effects", obj("coverage", str("QUALIFIED"), "externalUnbounded", wire.Bool(false), "resources", wire.Array(), "touchPaths", wire.Strings([]string{fmt.Sprintf("src/%04d/", i)})))
		})
	}
	setup := time.Now()
	runImport(t, repo, importExport(items...))
	cutover, err := store.Cutover(context.Background(), repo, operator(), fixture.QueueID, "measure-cutover", now(t))
	if err != nil || cutover.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("cutover: %+v %v", cutover, err)
	}
	s := leaseStoreOn(t, repo)
	setupSeconds := time.Since(setup).Seconds()
	type sample struct {
		Verb              string
		HoldMS, CommandMS float64
	}
	rows := []sample{}
	coldCommands := map[string]float64{}
	defer func() {
		p95 := map[string]float64{}
		for _, verb := range []string{"claim", "renew"} {
			times := []float64{}
			for _, s := range rows {
				if s.Verb == verb {
					times = append(times, s.HoldMS)
				}
			}
			sort.Float64s(times)
			if len(times) > 0 {
				p95[verb] = times[(95*len(times)+99)/100-1]
			}
		}
		report := map[string]any{"requirement": "CAL-V0-026", "smokeOnly": testing.Short(), "testFailed": t.Failed(), "tickets": count, "samplesPerVerb": samples, "cpuCount": runtime.NumCPU(), "setupSeconds": setupSeconds, "samples": rows, "p95HoldMS": p95, "coldCLICommandMS": coldCommands, "nonGrowingWork": "verified process-local audit cache; inventory, full audit, model and monitor teardown run outside the lock", "hostLoad": "NOT_OBSERVED: capture host load externally alongside this run"}
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(output, append(raw, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
		t.Logf("CAL-V0-026 p95 hold ms: %v; cold CLI ms: %v", p95, coldCommands)
	}()
	call := func(verb string, lease transaction.LeaseRequest, n int) *store.Report {
		held := time.Duration(0)
		calls := 0
		ctx := authority.WithLockObserver(context.Background(), func(d time.Duration) { held += d; calls++ })
		started := time.Now()
		report, err := store.Lease(ctx, s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: fmt.Sprintf("measure-%s-%d", verb, n), Root: s.root, Lease: lease}, s.at(t, 0))
		elapsed := time.Since(started)
		if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("%s: %+v %v", verb, report, err)
		}
		if calls != 1 {
			t.Fatalf("%s measured %d locks", verb, calls)
		}
		rows = append(rows, sample{verb, float64(held) / float64(time.Millisecond), float64(elapsed) / float64(time.Millisecond)})
		return report
	}
	for i := 0; i < samples; i++ {
		claim := call("claim", claimOf(fixture.TicketID(fmt.Sprintf("PERF-%04d", i))), i)
		call("renew", transaction.LeaseRequest{Verb: transaction.LeaseRenew, AttemptID: claim.AttemptID, Generation: claim.Generation, LeaseMinutes: "60"}, i)
		call("release", releaseOf(claim), i)
	}
	if os.Getenv("CORVINT_TASKS_MEASUREMENT_BINARY") != "" {
		// Each invocation is a fresh native CLI process with an empty cache.
		gitRun(t, repo.PrimaryWorktree, "init", "-q", "-b", "main")
		gitRun(t, repo.PrimaryWorktree, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "measurement base")
		callCLI := func(binary, label, verb string, args ...string) (string, string) {
			argv := append([]string{verb, "--request-id", "cold-" + label + "-" + verb}, args...)
			cmd := exec.Command(binary, argv...)
			cmd.Dir = repo.PrimaryWorktree
			started := time.Now()
			raw, err := cmd.CombinedOutput()
			coldCommands[label+"."+verb] = float64(time.Since(started)) / float64(time.Millisecond)
			if err != nil {
				t.Fatalf("cold %s: %v: %s", verb, err, raw)
			}
			var result struct {
				Outcome string
				Items   []struct {
					AttemptID  string
					Generation string
				}
			}
			if err := json.Unmarshal(raw, &result); err != nil || result.Outcome != "OK" || len(result.Items) != 1 {
				t.Fatalf("cold %s: %v %s", verb, err, raw)
			}
			return result.Items[0].AttemptID, result.Items[0].Generation
		}
		for _, label := range []string{"baseline", "candidate"} {
			key := "CORVINT_TASKS_MEASUREMENT_BINARY"
			if label == "baseline" {
				key = "CORVINT_TASKS_BASELINE_BINARY"
			}
			binary := os.Getenv(key)
			if binary == "" {
				continue
			}
			id, generation := callCLI(binary, label, "claim", "PERF-0000", "--holder", "cold-agent", "--lease-minutes", "60")
			callCLI(binary, label, "renew", "--attempt", id, "--generation", generation, "--lease-minutes", "60")
			callCLI(binary, label, "release", "--attempt", id, "--generation", generation)
		}
	}

	auditOK(t, repo)
}
