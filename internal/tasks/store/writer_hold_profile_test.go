package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0118_WriterHoldProfile is the opt-in V1-0645 / V1-0887 before/after
// measurement, not a timing assertion. On a settled historyStoreAt fixture it
// times store.Mutate (CREATE) and lease claim/renew/heartbeat/release end to
// end, with the writer-lock hold from WithLockObserver, the lease preparation
// hold from WithPreparationObserver, the lease phases from WithLeaseTiming and
// the Mutate phases from the mutationStage hook. The in-process lease audit
// cache is cleared before every lease call, so each call pays what a fresh
// CLI process pays. Raw samples are kept in order: sample 0 is the first
// writer on the fixture. CORVINT_TASKS_WRITER_HOLD_PPROF names a directory
// for one extra, unmeasured CPU profile per verb at the largest size.
func TestCALV0118_WriterHoldProfile(t *testing.T) {
	output := os.Getenv("CORVINT_TASKS_WRITER_HOLD_PROFILE")
	if output == "" {
		t.Skip("set CORVINT_TASKS_WRITER_HOLD_PROFILE to a report path")
	}
	envInt := func(name string, def int) int {
		if raw := os.Getenv(name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n <= 0 {
				t.Fatalf("%s=%q", name, raw)
			}
			return n
		}
		return def
	}
	sizes := []int{2000, 3000}
	if raw := os.Getenv("CORVINT_TASKS_HISTORY_SIZES"); raw != "" {
		sizes = nil
		for _, f := range strings.Split(raw, ",") {
			n, err := strconv.Atoi(f)
			if err != nil {
				t.Fatalf("size %q", f)
			}
			sizes = append(sizes, n)
		}
	}
	tickets, reps := envInt("CORVINT_TASKS_HISTORY_TICKETS", 880), envInt("CORVINT_TASKS_WRITER_HOLD_REPS", 5)
	type sample struct {
		Verb                     string
		WallMS, CPUMS, HoldMS    float64
		PreparationHoldMS        float64
		Locks, PreparationEvents int
		Phases                   map[string]float64
	}
	type row struct {
		Receipts, Tickets     int
		SetupMS               float64
		LoadBefore, LoadAfter string
		Samples               []sample
		Median                map[string]sample
	}
	profileDir := os.Getenv("CORVINT_TASKS_WRITER_HOLD_PPROF")
	var rows []row
	for _, n := range sizes {
		if n <= tickets+1 {
			t.Fatalf("size %d must exceed tickets %d", n, tickets)
		}
		setup := time.Now()
		base, err := os.MkdirTemp("", "atm-hold-")
		if err != nil {
			t.Fatal(err)
		}
		real, err := filepath.EvalSymlinks(base)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(real) })
		repo := historyStoreAt(t, filepath.Join(real, "repo"), tickets, n)
		holdLeasePolicy(t, repo)
		root := filepath.Join(real, "worktree")
		holdGit(t, root, "init", "-q", "-b", "main")
		holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
		entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
		if err != nil || len(entries) <= reps {
			t.Fatalf("tickets %d %v", len(entries), err)
		}
		r := row{Receipts: n, Tickets: tickets, SetupMS: ms(time.Since(setup)), LoadBefore: hostLoad()}
		profile := ""
		measure := func(verb string, f func(ctx context.Context) error) {
			s := sample{Verb: verb, Phases: map[string]float64{}}
			ctx := authority.WithLockObserver(context.Background(), func(d time.Duration) { s.HoldMS += ms(d); s.Locks++ })
			ctx = authority.WithPreparationObserver(ctx, func(o authority.PreparationObservation) {
				s.PreparationHoldMS += ms(o.Hold)
				s.PreparationEvents++
			})
			var timing LeaseTiming
			ctx = WithLeaseTiming(ctx, &timing)
			var last time.Time
			ctx = context.WithValue(ctx, mutationStageKey{}, func(stage string) {
				now := time.Now()
				if name, raw, ok := strings.Cut(stage, "="); ok {
					if d, err := time.ParseDuration(raw); err == nil {
						s.Phases["mutate."+name] += ms(d)
						return
					}
				}
				if i := strings.IndexByte(stage, ':'); i > 0 {
					stage = stage[:i]
				}
				s.Phases["mutate.to."+stage] += ms(now.Sub(last))
				last = now
			})
			leaseAudits.Lock()
			leaseAudits.proof = nil
			leaseAudits.Unlock()
			if profile != "" {
				out, err := os.Create(filepath.Join(profile, fmt.Sprintf("%d-%s.pprof", n, verb)))
				if err != nil {
					t.Fatal(err)
				}
				defer out.Close()
				if err = pprof.StartCPUProfile(out); err != nil {
					t.Fatal(err)
				}
				defer pprof.StopCPUProfile()
			}
			cpu, cpuOK := processCPU()
			start := time.Now()
			last = start
			if err := f(ctx); err != nil {
				t.Fatalf("%d %s: %v", n, verb, err)
			}
			end := time.Now()
			s.WallMS = ms(end.Sub(start))
			if verb == "mutate" {
				s.Phases["mutate.to.end"] += ms(end.Sub(last))
			}
			if endCPU, ok := processCPU(); cpuOK && ok {
				s.CPUMS = ms(endCPU - cpu)
			}
			if timing.Transactions > 0 {
				for k, v := range map[string]time.Duration{"admissionWait": timing.AdmissionWait, "guards": timing.Guards, "snapshotRead": timing.SnapshotRead, "validation": timing.Validation, "monitorClose": timing.MonitorClose, "lockWait": timing.LockWait, "lockHold": timing.LockHold, "journalWrite": timing.JournalWrite, "fsync": timing.Fsync} {
					s.Phases["lease."+k] = ms(v)
				}
				s.Phases["lease.rounds"], s.Phases["lease.transactions"] = float64(timing.Rounds), float64(timing.Transactions)
			}
			if profile == "" {
				r.Samples = append(r.Samples, s)
			}
		}
		runs := reps
		if profileDir != "" && n == sizes[len(sizes)-1] {
			runs++
		}
		for rep := 0; rep < runs; rep++ {
			if rep == reps {
				profile = profileDir
			}
			measure("mutate", func(ctx context.Context) error {
				id := fmt.Sprintf("hold-create-%d-%d", n, rep)
				out, err := Mutate(ctx, repo, historyActor, historyCreate(id, "hold "+id), WallClock())
				if err == nil && out.Outcome.Outcome != mutation.OutcomeCompleted {
					err = fmt.Errorf("outcome %+v", out.Outcome)
				}
				return err
			})
			local := strings.TrimSuffix(entries[rep].Name(), ".json")
			var claim *Report
			measure("claim", func(ctx context.Context) error {
				claim, err = Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: fmt.Sprintf("hold-claim-%d-%d", n, rep), Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: fixture.TicketID(local), Holder: "agent-1", LeaseMinutes: "60"}}, WallClock())
				if err == nil && claim.Outcome.Outcome != mutation.OutcomeCompleted {
					err = fmt.Errorf("outcome %+v", claim.Outcome)
				}
				return err
			})
			measure("renew", func(ctx context.Context) error {
				rn, err := Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: fmt.Sprintf("hold-renew-%d-%d", n, rep), Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseRenew, AttemptID: claim.AttemptID, Generation: claim.Generation, LeaseMinutes: "60"}}, WallClock())
				if err == nil && rn.Outcome.Outcome != mutation.OutcomeCompleted {
					err = fmt.Errorf("outcome %+v", rn.Outcome)
				}
				return err
			})
			measure("heartbeat", func(ctx context.Context) error {
				hb, err := Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: fmt.Sprintf("hold-heartbeat-%d-%d", n, rep), Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: claim.AttemptID, Generation: claim.Generation}}, WallClock())
				if err == nil && hb.Outcome.Outcome != mutation.OutcomeCompleted {
					err = fmt.Errorf("outcome %+v", hb.Outcome)
				}
				return err
			})
			measure("release", func(ctx context.Context) error {
				rel, err := Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: fmt.Sprintf("hold-release-%d-%d", n, rep), Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: claim.AttemptID, Generation: claim.Generation}}, WallClock())
				if err == nil && rel.Outcome.Outcome != mutation.OutcomeCompleted {
					err = fmt.Errorf("outcome %+v", rel.Outcome)
				}
				return err
			})
		}
		r.LoadAfter = hostLoad()
		r.Median = map[string]sample{}
		for _, verb := range []string{"mutate", "claim", "renew", "heartbeat", "release"} {
			var wall, cpu, hold, prep []float64
			phases := map[string][]float64{}
			for _, s := range r.Samples {
				if s.Verb == verb {
					wall, cpu, hold, prep = append(wall, s.WallMS), append(cpu, s.CPUMS), append(hold, s.HoldMS), append(prep, s.PreparationHoldMS)
					for k, v := range s.Phases {
						phases[k] = append(phases[k], v)
					}
				}
			}
			med := func(v []float64) float64 { sort.Float64s(v); return v[len(v)/2] }
			m := sample{Verb: verb, WallMS: med(wall), CPUMS: med(cpu), HoldMS: med(hold), PreparationHoldMS: med(prep), Phases: map[string]float64{}}
			for k, v := range phases {
				if len(v) == len(wall) {
					m.Phases[k] = med(v)
				}
			}
			r.Median[verb] = m
		}
		head, err := writerGuards(repo, "MUTATE")
		if err != nil {
			t.Fatal(err)
		}
		if proof, err := journalReader(repo, head).Audit(); err != nil || proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "AGREES" {
			t.Fatalf("final audit: %+v %v", proof, err)
		}
		rows = append(rows, r)
		t.Logf("receipts=%d median %+v", n, r.Median)
	}
	report := map[string]any{"ticket": "V1-0645", "requirement": "CAL-V0-118", "fixture": fmt.Sprintf("historyStoreAt: tickets as listed, one request plus one ~%d-byte inline ticket afterimage per padding receipt; lease policy widened to four attempts; no evidence blobs", historyBodySize), "repetitions": reps, "statistic": "raw samples in order and per-verb medians (median of each phase separately); HoldMS is the writer-lock hold, PreparationHoldMS the lease preparation-lock hold; lease.* phases are LeaseTiming, mutate.to.<stage> the time since the previous mutationStage, mutate.<name> a duration a stage reported", "cpuCount": runtime.NumCPU(), "goVersion": runtime.Version(), "goos": runtime.GOOS, "rows": rows, "hostLoad": "per row: LoadBefore/LoadAfter are the 1, 5 and 15 minute load averages around the measured repetitions"}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(output, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

// holdLeasePolicy widens the fixture policy to four attempts with no enforced
// budget fields, as the lease tests do, so claims reach the lease checks.
func holdLeasePolicy(tb testing.TB, repo *intent.Repository) {
	tb.Helper()
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", wire.String("2"))
	v.Obj.Set("capacity", historyObject("maxActiveAttempts", wire.String("4"), "maxWorkersTotal", wire.String("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	report, err := PolicyUpdate(context.Background(), repo, historyActor, PolicyRequest{QueueID: fixture.QueueID, RequestID: "hold-policy", ExpectedPolicyVersion: "1", Policy: wire.EncodeFile(v)}, WallClock())
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		tb.Fatalf("policy: %+v %v", report, err)
	}
}

// hostLoad reports the host's load averages, or why they were not observed.
func hostLoad() string {
	switch runtime.GOOS {
	case "linux":
		if raw, err := os.ReadFile("/proc/loadavg"); err == nil {
			return strings.Join(strings.Fields(string(raw))[:3], " ")
		}
	case "darwin", "freebsd":
		if raw, err := exec.Command("sysctl", "-n", "vm.loadavg").Output(); err == nil {
			return strings.Trim(strings.TrimSpace(string(raw)), "{} ")
		}
	}
	return "NOT_OBSERVED"
}

func holdGit(tb testing.TB, dir string, args ...string) {
	tb.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		tb.Fatal(err)
	}
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := c.CombinedOutput(); err != nil {
		tb.Fatalf("git %v: %v %s", args, err, out)
	}
}
