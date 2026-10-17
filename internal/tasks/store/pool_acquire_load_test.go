//go:build darwin || linux

package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// acquireLoadWorker is one process's assignment in the V1-1560 harness.
type acquireLoadWorker struct {
	Repo, Root, Role, Out     string
	Tickets                   []string
	Index, PeriodMS           int
	Start, Deadline, HardStop int64
}

// acquireLoadSample is one call as a worker process observed it.
type acquireLoadSample struct {
	Worker          int
	Verb, Code      string
	StartMS, WallMS float64
	SnapshotReadMS  float64
	AdmissionMS     float64
	ValidationMS    float64
	LockWaitMS      float64
	LockHoldMS      float64
	Rounds, Resumed int
	Unfinished      bool
	Detail          string
	Declines        []string
}

// TestCALV0335_PoolAcquireLoad is the opt-in V1-1560 / GH1010 starvation
// harness, not a timing assertion; a plain `go test` skips it. It builds a
// settled historyStoreAt journal of CORVINT_TASKS_POOL_ACQUIRE_LOAD_RECEIPTS
// receipts (default 45000) with a pool "db" of one member per acquirer, then
// runs one dispatcher process that commits a ticket create every
// CORVINT_TASKS_POOL_ACQUIRE_LOAD_WRITER_MS (default 3000) and
// CORVINT_TASKS_POOL_ACQUIRE_LOAD_ACQUIRERS (default 8) acquirer processes,
// each of which repeats claim, pool acquire, pool release, pool confirm-safe and
// release on its own ticket, for CORVINT_TASKS_POOL_ACQUIRE_LOAD_SECONDS (default 600). Every
// process re-executes this test binary and clears its in-process audit cache
// before each call, as a fresh CLI process would. A call still running
// CORVINT_TASKS_POOL_ACQUIRE_LOAD_GRACE_SECONDS (default 300) after the
// deadline is cancelled and recorded as unfinished. The report at
// CORVINT_TASKS_POOL_ACQUIRE_LOAD keeps raw samples and per-verb percentiles.
func TestCALV0335_PoolAcquireLoad(t *testing.T) {
	output := os.Getenv("CORVINT_TASKS_POOL_ACQUIRE_LOAD")
	if output == "" {
		t.Skip("set CORVINT_TASKS_POOL_ACQUIRE_LOAD to a report path")
	}
	envInt := func(name string, def int) int {
		if raw := os.Getenv(name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 {
				t.Fatalf("%s=%q", name, raw)
			}
			return n
		}
		return def
	}
	receipts, acquirers := envInt("CORVINT_TASKS_POOL_ACQUIRE_LOAD_RECEIPTS", 45000), envInt("CORVINT_TASKS_POOL_ACQUIRE_LOAD_ACQUIRERS", 8)
	seconds, period := envInt("CORVINT_TASKS_POOL_ACQUIRE_LOAD_SECONDS", 600), envInt("CORVINT_TASKS_POOL_ACQUIRE_LOAD_WRITER_MS", 3000)
	grace := envInt("CORVINT_TASKS_POOL_ACQUIRE_LOAD_GRACE_SECONDS", 300)
	// Each claim spends one admission of the ticket's retry budget, at most
	// wire.MaxAdmissionsPerRev beyond the first; a worker moves to its next
	// ticket when one is spent. A cycle takes at least about 3 s here.
	perWorker := seconds/(3*(wire.MaxAdmissionsPerRev+1)) + 2
	tickets := acquirers*perWorker + 4
	setup := time.Now()
	base, err := os.MkdirTemp("", "atm-acquire-load-")
	if err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(real) })
	repo := historyStoreAt(t, filepath.Join(real, "repo"), tickets, receipts)
	members := make([]string, acquirers)
	for i := range members {
		members[i] = fmt.Sprintf("m%d", i)
	}
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", wire.String("2"))
	n := wire.String(strconv.Itoa(acquirers + 8))
	v.Obj.Set("capacity", historyObject("maxActiveAttempts", n, "maxWorkersTotal", n, "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	retries, _ := v.Obj.Get("retries")
	retries.Obj.Set("admissionsPerRevision", wire.String(strconv.Itoa(wire.MaxAdmissionsPerRev)))
	v.Obj.Set("pools", wire.Array(historyObject("id", wire.String("db"), "members", wire.Strings(members))))
	if report, err := PolicyUpdate(context.Background(), repo, historyActor, PolicyRequest{QueueID: fixture.QueueID, RequestID: "acquire-load-policy", ExpectedPolicyVersion: "1", Policy: wire.EncodeFile(v)}, WallClock()); err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", report, err)
	}
	root := filepath.Join(real, "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
	if err != nil || len(entries) < acquirers*perWorker {
		t.Fatalf("tickets %d %v", len(entries), err)
	}
	setupMS := ms(time.Since(setup))

	loadBefore := hostLoad()
	start := time.Now()
	deadline := start.Add(time.Duration(seconds) * time.Second)
	hardStop := deadline.Add(time.Duration(grace) * time.Second)
	procs := make([]*exec.Cmd, acquirers+1)
	outs := make([]string, len(procs))
	for i := range procs {
		role, own := "acquire", []string(nil)
		if i == acquirers {
			role = "dispatcher"
		} else {
			for k := 0; k < perWorker; k++ {
				own = append(own, strings.TrimSuffix(entries[i+k*acquirers].Name(), ".json"))
			}
		}
		outs[i] = filepath.Join(real, fmt.Sprintf("worker-%d.jsonl", i))
		spec, _ := json.Marshal(acquireLoadWorker{Repo: repo.PrimaryWorktree, Root: root, Role: role, Tickets: own, Out: outs[i], Index: i, PeriodMS: period, Start: start.UnixMilli(), Deadline: deadline.UnixMilli(), HardStop: hardStop.UnixMilli()})
		cmd := exec.Command(os.Args[0], "-test.run=^TestCALV0335_PoolAcquireLoadWorker$", "-test.count=1", "-test.timeout=0")
		cmd.Env = append(os.Environ(), "CORVINT_TASKS_POOL_ACQUIRE_LOAD=", "CORVINT_TASKS_POOL_ACQUIRE_LOAD_WORKER="+string(spec))
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		procs[i] = cmd
	}
	failed := 0
	for i, cmd := range procs {
		if err := cmd.Wait(); err != nil {
			failed++
			t.Errorf("worker %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	loadAfter := hostLoad()

	var samples []acquireLoadSample
	for _, path := range outs {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var s acquireLoadSample
			if err := json.Unmarshal(sc.Bytes(), &s); err != nil {
				t.Fatal(err)
			}
			samples = append(samples, s)
		}
		f.Close()
	}
	type summary struct {
		Calls, Completed, Unfinished, Resumed int
		Codes                                 map[string]int
		WallP50, WallP99, WallMax             float64
		RoundsMax                             int
	}
	pct := func(v []float64, p float64) float64 {
		if len(v) == 0 {
			return 0
		}
		sort.Float64s(v)
		return v[min(len(v)-1, int(p*float64(len(v))))]
	}
	verbs := map[string]*summary{}
	wall := map[string][]float64{}
	for _, s := range samples {
		v := verbs[s.Verb]
		if v == nil {
			v = &summary{Codes: map[string]int{}}
			verbs[s.Verb] = v
		}
		v.Calls++
		v.Resumed += s.Resumed
		v.RoundsMax = max(v.RoundsMax, s.Rounds)
		switch {
		case s.Unfinished:
			v.Unfinished++
		case s.Code == "":
			v.Completed++
		default:
			v.Codes[s.Code]++
		}
		wall[s.Verb] = append(wall[s.Verb], s.WallMS)
	}
	for key, v := range verbs {
		v.WallP50, v.WallP99, v.WallMax = pct(wall[key], 0.5), pct(wall[key], 0.99), pct(wall[key], 1)
	}
	head, err := writerGuards(repo, "MUTATE")
	if err != nil {
		t.Fatal(err)
	}
	report := map[string]any{
		"ticket": "V1-1560", "issue": "beamfall/corvint#1010", "requirement": "CAL-V0-335",
		"fixture":   fmt.Sprintf("historyStoreAt with %d tickets and %d receipts; pool db of %d members", tickets, receipts, acquirers),
		"workers":   fmt.Sprintf("1 dispatcher creating a ticket every %d ms; %d acquirers, each repeating claim, pool acquire, pool release, pool confirm-safe and release on its own tickets, moving to the next when a ticket's retry budget is spent; in-process lease audit cache cleared before every call", period, acquirers),
		"statistic": "per-verb nearest-rank percentiles over all calls; WallMS is the whole Lease call; an unfinished call was cancelled at the hard stop and its WallMS is a lower bound",
		"receipts":  receipts, "acquirers": acquirers, "seconds": seconds, "graceSeconds": grace, "writerMS": period,
		"elapsedMS": ms(elapsed), "setupMS": setupMS, "finalLastSeq": head.LastSeq.Uint64(), "workerFailures": failed,
		"cpuCount": runtime.NumCPU(), "goVersion": runtime.Version(), "goos": runtime.GOOS,
		"hostLoad": map[string]string{"before": loadBefore, "after": loadAfter},
		"summary":  verbs, "samples": samples,
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(output, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("receipts=%d acquirers=%d elapsed=%s lastSeq=%s load %s -> %s", receipts, acquirers, elapsed, head.LastSeq, loadBefore, loadAfter)
	for _, verb := range []string{"claim", "acquire", "pool-release", "confirm-safe", "release", "dispatch"} {
		if v := verbs[verb]; v != nil {
			t.Logf("%-12s calls=%d completed=%d unfinished=%d resumed=%d codes=%v rounds max=%d wall p50/p99/max=%.0f/%.0f/%.0f ms", verb, v.Calls, v.Completed, v.Unfinished, v.Resumed, v.Codes, v.RoundsMax, v.WallP50, v.WallP99, v.WallMax)
		}
	}
}

// TestCALV0335_PoolAcquireLoadWorker is one process of
// TestCALV0335_PoolAcquireLoad; it runs only when that harness starts it.
func TestCALV0335_PoolAcquireLoadWorker(t *testing.T) {
	raw := os.Getenv("CORVINT_TASKS_POOL_ACQUIRE_LOAD_WORKER")
	if raw == "" {
		t.Skip("started only by TestCALV0335_PoolAcquireLoad")
	}
	var w acquireLoadWorker
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(w.Repo)
	if err != nil {
		t.Fatal(err)
	}
	epoch, deadline, hardStop := time.UnixMilli(w.Start), time.UnixMilli(w.Deadline), time.UnixMilli(w.HardStop)
	var samples []acquireLoadSample
	calls := 0
	call := func(verb string, run func(ctx context.Context, id string) (*Report, error)) *Report {
		calls++
		s := acquireLoadSample{Worker: w.Index, Verb: verb}
		ctx, cancel := context.WithDeadline(WithClock(context.Background(), WallClock), hardStop)
		defer cancel()
		var timing LeaseTiming
		ctx = WithLeaseTiming(ctx, &timing)
		ctx = context.WithValue(ctx, mutationStageKey{}, func(stage string) {
			if strings.HasPrefix(stage, "fast.declined") {
				s.Declines = append(s.Declines, stage)
			}
		})
		leaseAudits.Lock()
		leaseAudits.proof = nil
		leaseAudits.Unlock()
		started := time.Now()
		report, err := run(ctx, fmt.Sprintf("acquire-load-%d-%d-%d", w.Index, calls, started.UnixNano()))
		s.WallMS, s.StartMS = ms(time.Since(started)), ms(started.Sub(epoch))
		s.SnapshotReadMS, s.Rounds, s.Resumed = ms(timing.SnapshotRead), timing.Rounds, timing.Resumed
		s.AdmissionMS, s.ValidationMS, s.LockWaitMS, s.LockHoldMS = ms(timing.AdmissionWait), ms(timing.Validation), ms(timing.LockWait), ms(timing.LockHold)
		switch {
		case ctx.Err() != nil && (err != nil || report == nil || report.Outcome.Outcome != mutation.OutcomeCompleted):
			s.Unfinished, s.Code = true, "UNFINISHED"
		case err != nil:
			s.Code = string(wire.CodeOf(err))
			if s.Code == "" {
				s.Code = "ERROR"
			}
		case report.Outcome.Outcome != mutation.OutcomeCompleted:
			s.Code = "OUTCOME_" + string(report.Outcome.Outcome)
			s.Detail = fmt.Sprintf("%v %.200s", report.Outcome.Codes, report.Detail)
		}
		samples = append(samples, s)
		if s.Code != "" {
			return nil
		}
		return report
	}
	lease := func(verb string, req transaction.LeaseRequest) *Report {
		return call(verb, func(ctx context.Context, id string) (*Report, error) {
			return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: w.Root, Lease: req}, WallClock())
		})
	}
	if w.Role == "dispatcher" {
		for time.Now().Before(deadline) {
			call("dispatch", func(ctx context.Context, id string) (*Report, error) {
				return Mutate(ctx, repo, historyActor, historyCreate(id, "dispatch "+id), WallClock())
			})
			time.Sleep(time.Duration(w.PeriodMS) * time.Millisecond)
		}
	} else {
		// One allocation per attempt generation: each cycle claims,
		// acquires, returns the allocation and releases the attempt.
		ticket := 0
		for time.Now().Before(deadline) && ticket < len(w.Tickets) {
			claim := lease("claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: fixture.TicketID(w.Tickets[ticket]), Holder: fmt.Sprintf("agent-%d", w.Index), LeaseMinutes: "600", Scope: []string{fmt.Sprintf("w%d/", w.Index)}})
			if claim == nil {
				if last := samples[len(samples)-1]; strings.Contains(last.Detail, string(wire.CodeRetryExhausted)) {
					ticket++
				} else {
					time.Sleep(time.Second)
				}
				continue
			}
			generation := claim.Generation
			if acquired := lease("acquire", transaction.LeaseRequest{Verb: transaction.LeasePoolAcquire, AttemptID: claim.AttemptID, Generation: generation, Pool: "db"}); acquired != nil && acquired.PoolAllocation != nil {
				generation = acquired.Generation
				alloc := acquired.PoolAllocation
				if released := lease("pool-release", transaction.LeaseRequest{Verb: transaction.LeasePoolRelease, AttemptID: claim.AttemptID, Generation: generation, Allocation: string(alloc.AllocationID)}); released != nil {
					generation = released.Generation
					// A returned member stays quarantined until confirmed safe.
					lease("confirm-safe", transaction.LeaseRequest{Verb: transaction.LeasePoolSafe, Member: alloc.MemberID, Allocation: string(alloc.AllocationID), Evidence: "load-harness-reset", Reason: "load harness member reset"})
				}
			}
			lease("release", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: claim.AttemptID, Generation: generation})
		}
	}
	out := make([]byte, 0, 256*len(samples))
	for _, s := range samples {
		line, _ := json.Marshal(s)
		out = append(append(out, line...), '\n')
	}
	if err := os.WriteFile(w.Out, out, 0o600); err != nil {
		t.Fatal(err)
	}
}
