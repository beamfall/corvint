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

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// gateLoadWorker is one writer process's assignment in the CAL-V0-263 load
// harness.
type gateLoadWorker struct {
	Repo, Root, Role, Ticket, Out string
	Index, ThinkMS                int
	Start, Deadline               int64
}

// gateLoadSample is one lease call as a worker process observed it.
type gateLoadSample struct {
	Worker                       int
	Verb, Code                   string
	StartMS, WallMS              float64
	GateEvents                   int
	GateWaitMS, GateHoldMS       float64
	LockHoldMS, AdmissionWaitMS  float64
	SnapshotReadMS, ValidationMS float64
	LockWaitMS, JournalWriteMS   float64
	FsyncMS                      float64
	Rounds, Transactions         int
}

// TestCALV0263_LeaseGateLoad is the opt-in V1-1494 / GH967 load harness, not
// a timing assertion; a plain `go test` skips it. It builds a settled
// historyStoreAt journal of CORVINT_TASKS_LEASE_GATE_LOAD_RECEIPTS receipts
// (default 40000), then runs CORVINT_TASKS_LEASE_GATE_LOAD_WRITERS (default
// 15) concurrent writer processes, each a re-execution of this test binary
// with its own in-process audit cache cleared before every call, as a fresh
// CLI process would be, for CORVINT_TASKS_LEASE_GATE_LOAD_SECONDS. Writer 0
// reaps, writers 1 and 2 widen, and the rest claim and then alternate
// heartbeat and renew; each releases at the end. Every call records its
// preparation-gate wait and hold (WithPreparationObserver), writer-lock hold
// and error code. The report at CORVINT_TASKS_LEASE_GATE_LOAD keeps raw
// samples and per-verb percentiles, including the LOCK_TIMEOUT count.
func TestCALV0263_LeaseGateLoad(t *testing.T) {
	output := os.Getenv("CORVINT_TASKS_LEASE_GATE_LOAD")
	if output == "" {
		t.Skip("set CORVINT_TASKS_LEASE_GATE_LOAD to a report path")
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
	receipts, writers := envInt("CORVINT_TASKS_LEASE_GATE_LOAD_RECEIPTS", 40000), envInt("CORVINT_TASKS_LEASE_GATE_LOAD_WRITERS", 15)
	seconds, think := envInt("CORVINT_TASKS_LEASE_GATE_LOAD_SECONDS", 120), envInt("CORVINT_TASKS_LEASE_GATE_LOAD_THINK_MS", 10000)
	tickets := writers + 16
	setup := time.Now()
	base, err := os.MkdirTemp("", "atm-gate-load-")
	if err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(real) })
	repo := historyStoreAt(t, filepath.Join(real, "repo"), tickets, receipts)
	gateLoadPolicy(t, repo, writers+8)
	root := filepath.Join(real, "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
	if err != nil || len(entries) < writers+1 {
		t.Fatalf("tickets %d %v", len(entries), err)
	}
	// One warm-up claim and release, as a store in use has already had.
	warm := &gateLoadRun{repo: repo, root: root, worker: -1}
	attempt, generation := warm.claim(strings.TrimSuffix(entries[writers].Name(), ".json"), []string{"warm/"})
	if attempt == "" {
		t.Fatalf("warm-up claim: %+v", warm.samples)
	}
	warm.lease("release", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: attempt, Generation: generation})
	setupMS := ms(time.Since(setup))

	loadBefore := hostLoad()
	start := time.Now()
	deadline := start.Add(time.Duration(seconds) * time.Second)
	procs := make([]*exec.Cmd, writers)
	outs := make([]string, writers)
	for i := range procs {
		role := "liveness"
		switch {
		case i == 0:
			role = "reap"
		case i <= 2:
			role = "widen"
		}
		outs[i] = filepath.Join(real, fmt.Sprintf("worker-%d.jsonl", i))
		spec, _ := json.Marshal(gateLoadWorker{Repo: repo.PrimaryWorktree, Root: root, Role: role, Ticket: strings.TrimSuffix(entries[i].Name(), ".json"), Out: outs[i], Index: i, ThinkMS: think, Start: start.UnixMilli(), Deadline: deadline.UnixMilli()})
		cmd := exec.Command(os.Args[0], "-test.run=^TestCALV0263_LeaseGateLoadWorker$", "-test.count=1", "-test.timeout=0")
		cmd.Env = append(os.Environ(), "CORVINT_TASKS_LEASE_GATE_LOAD=", "CORVINT_TASKS_LEASE_GATE_LOAD_WORKER="+string(spec))
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

	var samples []gateLoadSample
	for _, path := range outs {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var s gateLoadSample
			if err := json.Unmarshal(sc.Bytes(), &s); err != nil {
				t.Fatal(err)
			}
			samples = append(samples, s)
		}
		f.Close()
	}
	type summary struct {
		Calls, Gated, LockTimeouts int
		Codes                      map[string]int
		WallP50, WallP99, WallMax  float64
		GateWaitP50, GateWaitP99   float64
		GateHoldP50, GateHoldP99   float64
		GateHoldMax, LockHoldP99   float64
	}
	pct := func(v []float64, p float64) float64 {
		if len(v) == 0 {
			return 0
		}
		sort.Float64s(v)
		return v[min(len(v)-1, int(p*float64(len(v))))]
	}
	verbs := map[string]*summary{}
	wall, gateWait, gateHold, lockHold := map[string][]float64{}, map[string][]float64{}, map[string][]float64{}, map[string][]float64{}
	for _, s := range samples {
		for _, key := range []string{s.Verb, "all"} {
			v := verbs[key]
			if v == nil {
				v = &summary{Codes: map[string]int{}}
				verbs[key] = v
			}
			v.Calls++
			if s.Code != "" {
				v.Codes[s.Code]++
			}
			if s.Code == string(wire.CodeLockTimeout) {
				v.LockTimeouts++
			}
			if s.GateEvents > 0 {
				v.Gated++
				gateWait[key] = append(gateWait[key], s.GateWaitMS)
				gateHold[key] = append(gateHold[key], s.GateHoldMS)
			}
			wall[key] = append(wall[key], s.WallMS)
			lockHold[key] = append(lockHold[key], s.LockHoldMS)
		}
	}
	for key, v := range verbs {
		v.WallP50, v.WallP99, v.WallMax = pct(wall[key], 0.5), pct(wall[key], 0.99), pct(wall[key], 1)
		v.GateWaitP50, v.GateWaitP99 = pct(gateWait[key], 0.5), pct(gateWait[key], 0.99)
		v.GateHoldP50, v.GateHoldP99, v.GateHoldMax = pct(gateHold[key], 0.5), pct(gateHold[key], 0.99), pct(gateHold[key], 1)
		v.LockHoldP99 = pct(lockHold[key], 0.99)
	}
	all := verbs["all"]
	if all == nil {
		t.Fatal("no samples")
	}
	head, err := writerGuards(repo, "MUTATE")
	if err != nil {
		t.Fatal(err)
	}
	report := map[string]any{
		"ticket": "V1-1494", "issue": "beamfall/corvint#967", "requirement": "CAL-V0-263",
		"fixture":   fmt.Sprintf("historyStoreAt with %d tickets and %d receipts (one request plus one ~%d-byte ticket afterimage per padding receipt); policy capacity %d; one warm-up claim and release", tickets, receipts, historyBodySize, writers+8),
		"workers":   fmt.Sprintf("%d processes re-executing this test binary: worker 0 reaps, each other worker claims a disjoint scope, workers 1-2 then widen it and the rest alternate heartbeat and renew; %d ms think time; each releases at the deadline; in-process lease audit cache cleared before every call", writers, think),
		"statistic": "per-verb nearest-rank percentiles over all calls; gate percentiles only over calls that entered the preparation gate; Code is wire.CodeOf of a failed call; WallMS is the whole Lease call",
		"receipts":  receipts, "writers": writers, "seconds": seconds, "elapsedMS": ms(elapsed), "setupMS": setupMS,
		"finalLastSeq": head.LastSeq.Uint64(), "workerFailures": failed,
		"lockTimeoutsPerHour": float64(all.LockTimeouts) / elapsed.Hours(),
		"cpuCount":            runtime.NumCPU(), "goVersion": runtime.Version(), "goos": runtime.GOOS,
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
	t.Logf("receipts=%d writers=%d elapsed=%s lastSeq=%s LOCK_TIMEOUT=%d", receipts, writers, elapsed, head.LastSeq, all.LockTimeouts)
	for _, verb := range []string{"claim", "heartbeat", "renew", "widen", "reap", "release", "all"} {
		if v := verbs[verb]; v != nil {
			t.Logf("%-9s calls=%d gated=%d timeouts=%d codes=%v wall p50/p99=%.0f/%.0f ms gate wait p99=%.0f hold p50/p99/max=%.0f/%.0f/%.0f ms", verb, v.Calls, v.Gated, v.LockTimeouts, v.Codes, v.WallP50, v.WallP99, v.GateWaitP99, v.GateHoldP50, v.GateHoldP99, v.GateHoldMax)
		}
	}
}

// TestCALV0263_LeaseGateLoadWorker is one writer process of
// TestCALV0263_LeaseGateLoad; it runs only when that harness starts it.
func TestCALV0263_LeaseGateLoadWorker(t *testing.T) {
	raw := os.Getenv("CORVINT_TASKS_LEASE_GATE_LOAD_WORKER")
	if raw == "" {
		t.Skip("started only by TestCALV0263_LeaseGateLoad")
	}
	var w gateLoadWorker
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(w.Repo)
	if err != nil {
		t.Fatal(err)
	}
	run := &gateLoadRun{repo: repo, root: w.Root, worker: w.Index, epoch: time.UnixMilli(w.Start)}
	deadline := time.UnixMilli(w.Deadline)
	pause := func() {
		time.Sleep(time.Duration(w.ThinkMS/2+(w.Index*37)%max(1, w.ThinkMS)) * time.Millisecond)
	}
	attempt, generation, widened := "", wire.Size(""), 0
	for time.Now().Before(deadline) {
		switch {
		case w.Role == "reap":
			run.lease("reap", transaction.LeaseRequest{Verb: transaction.LeaseReap})
		case attempt == "":
			attempt, generation = run.claim(w.Ticket, []string{fmt.Sprintf("w%d/0/", w.Index)})
		case w.Role == "widen":
			widened++
			if g := run.lease("widen", transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: attempt, Generation: generation, Scope: []string{fmt.Sprintf("w%d/%d/", w.Index, widened)}}); g != "" {
				generation = g
			}
		case run.calls%2 == 0:
			run.lease("heartbeat", transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: attempt, Generation: generation})
		default:
			run.lease("renew", transaction.LeaseRequest{Verb: transaction.LeaseRenew, AttemptID: attempt, Generation: generation, LeaseMinutes: "60"})
		}
		pause()
	}
	if attempt != "" {
		run.lease("release", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: attempt, Generation: generation})
	}
	out := make([]byte, 0, 256*len(run.samples))
	for _, s := range run.samples {
		line, _ := json.Marshal(s)
		out = append(append(out, line...), '\n')
	}
	if err := os.WriteFile(w.Out, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// gateLoadRun issues one worker's lease calls and keeps their samples.
type gateLoadRun struct {
	repo    *intent.Repository
	root    string
	worker  int
	calls   int
	epoch   time.Time
	samples []gateLoadSample
}

func (r *gateLoadRun) claim(local string, scope []string) (attempt string, generation wire.Size) {
	var report *Report
	report = r.call("claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: fixture.TicketID(local), Holder: fmt.Sprintf("agent-%d", r.worker), LeaseMinutes: "60", Scope: scope})
	if report == nil {
		return "", ""
	}
	return report.AttemptID, report.Generation
}

// lease issues one call and returns the attempt's new generation, if any.
func (r *gateLoadRun) lease(verb string, req transaction.LeaseRequest) wire.Size {
	if report := r.call(verb, req); report != nil {
		return report.Generation
	}
	return ""
}

func (r *gateLoadRun) call(verb string, req transaction.LeaseRequest) *Report {
	r.calls++
	s := gateLoadSample{Worker: r.worker, Verb: verb}
	// The live clock the command line gives every writer (cli.writerContext), so a
	// write that prepared across commits resamples recordedAt as the CLI does.
	ctx := authority.WithLockObserver(WithClock(context.Background(), WallClock), func(d time.Duration) { s.LockHoldMS += ms(d) })
	ctx = authority.WithPreparationObserver(ctx, func(o authority.PreparationObservation) {
		s.GateEvents++
		s.GateWaitMS += ms(o.Wait)
		s.GateHoldMS += ms(o.Hold)
	})
	var timing LeaseTiming
	ctx = WithLeaseTiming(ctx, &timing)
	forgetLeaseAudits()
	started := time.Now()
	report, err := Lease(ctx, r.repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: fmt.Sprintf("gate-load-%d-%d-%d", r.worker, r.calls, started.UnixNano()), Root: r.root, Lease: req}, WallClock())
	s.WallMS = ms(time.Since(started))
	if !r.epoch.IsZero() {
		s.StartMS = ms(started.Sub(r.epoch))
	}
	s.AdmissionWaitMS, s.SnapshotReadMS, s.ValidationMS = ms(timing.AdmissionWait), ms(timing.SnapshotRead), ms(timing.Validation)
	s.LockWaitMS, s.JournalWriteMS, s.FsyncMS = ms(timing.LockWait), ms(timing.JournalWrite), ms(timing.Fsync)
	s.Rounds, s.Transactions = timing.Rounds, timing.Transactions
	switch {
	case err != nil:
		s.Code = string(wire.CodeOf(err))
		if s.Code == "" {
			s.Code = "ERROR"
		}
	case report.Outcome.Outcome != mutation.OutcomeCompleted:
		s.Code = "OUTCOME_" + string(report.Outcome.Outcome)
	}
	r.samples = append(r.samples, s)
	if s.Code != "" {
		return nil
	}
	return report
}

// gateLoadPolicy widens the fixture policy to capacity attempts with no
// enforced budget fields, as holdLeasePolicy does for four.
func gateLoadPolicy(tb testing.TB, repo *intent.Repository, capacity int) {
	tb.Helper()
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", wire.String("2"))
	n := wire.String(strconv.Itoa(capacity))
	v.Obj.Set("capacity", historyObject("maxActiveAttempts", n, "maxWorkersTotal", n, "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	report, err := PolicyUpdate(context.Background(), repo, historyActor, PolicyRequest{QueueID: fixture.QueueID, RequestID: "gate-load-policy", ExpectedPolicyVersion: "1", Policy: wire.EncodeFile(v)}, WallClock())
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		tb.Fatalf("policy: %+v %v", report, err)
	}
}
