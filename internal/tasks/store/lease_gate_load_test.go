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
	"runtime/trace"
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
	// TracePrefix names the retained flight-recorder snapshots (V1-2540).
	TracePrefix     string
	Index, ThinkMS  int
	Start, Deadline int64
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
	// Resumed and ResumeDeclines count the call's resumed writes and
	// declined resumes, so a widen or reap shows whether it resumed from the
	// sealed base or paid the complete audit (JSG-V0-780, V1-2592).
	Resumed, ResumeDeclines int `json:",omitempty"`
	// Stages sums the writer-route and complete-route stage durations the
	// call reported (writerStage); Declines lists its writer-route
	// declines in order (V1-2531).
	Stages   map[string]float64 `json:",omitempty"`
	Declines []string           `json:",omitempty"`
	// V1-2540: OutsideGateMS is WallMS less GateWaitMS and GateHoldMS;
	// PreGateMS runs from the call's start to its first gate admission
	// request and PostGateMS from its last gate release to its return.
	// GuardsMS and MonitorCloseMS are the named LeaseTiming stages outside
	// the gate, and Process is what the call added to the process clocks.
	OutsideGateMS, PreGateMS, PostGateMS float64
	GuardsMS, MonitorCloseMS             float64
	Process                              processDelta
	// Trace names the flight-recorder snapshot written for this call;
	// TraceError is why a snapshot this call triggered was not retained.
	Trace      string `json:",omitempty"`
	TraceError string `json:",omitempty"`
}

// TestCALV0263_LeaseGateLoad is the opt-in V1-1494 / GH967 load harness, not
// a timing assertion; a plain `go test` skips it. It builds a settled
// historyStoreAt journal of CORVINT_TASKS_LEASE_GATE_LOAD_RECEIPTS receipts
// (default 40000), then runs CORVINT_TASKS_LEASE_GATE_LOAD_WRITERS (default
// 15) concurrent writer processes over CORVINT_TASKS_LEASE_GATE_LOAD_TICKETS
// tickets (gateLoadTickets: default 880, minimum writers+16; LGL-V0-015),
// each a re-execution of this test binary with its own in-process audit
// cache cleared before every call, as a fresh CLI process would be, for
// CORVINT_TASKS_LEASE_GATE_LOAD_SECONDS. Writer 0
// reaps, writers 1 and 2 widen, and the rest claim and then alternate
// heartbeat and renew; each releases at the end. Every call records its
// preparation-gate wait and hold (WithPreparationObserver), writer-lock hold
// and error code. The report at CORVINT_TASKS_LEASE_GATE_LOAD keeps raw
// samples and per-verb percentiles, including the LOCK_TIMEOUT count.
// gateLoadTicketsDefault is the harness queue size (LGL-V0-015): near the
// live store's ticket count (about 950 on 2026-10-27), so the per-ticket
// decode under the gate (6 ms at 31 tickets, 66 ms at 880) is measured.
const gateLoadTicketsDefault = 880

// gateLoadTickets is the harness ticket count: requested, or the default
// when zero, and never fewer than writers+16, so every writer has a ticket.
func gateLoadTickets(requested, writers int) int {
	if requested == 0 {
		requested = gateLoadTicketsDefault
	}
	return max(requested, writers+16)
}

// gateLoadPublishSyncs applies CORVINT_TASKS_LEASE_GATE_LOAD_SYNC (LGL-V0-017
// proposed; V1-2877): "serial" runs a lease write's publish directory syncs
// one by one as before LGL-V0-016, "concurrent" or unset keeps the overlap.
// Only this harness reads it, in the parent and in every worker process it
// starts; production code has no such knob and the store format is unchanged.
func gateLoadPublishSyncs(t testing.TB) {
	t.Helper()
	switch v := os.Getenv("CORVINT_TASKS_LEASE_GATE_LOAD_SYNC"); v {
	case "", "concurrent":
	case "serial":
		t.Cleanup(SetLeasePublishSyncsOverlapForTest(false))
	default:
		t.Fatalf("CORVINT_TASKS_LEASE_GATE_LOAD_SYNC=%q; want serial or concurrent", v)
	}
}

func gateLoadSyncMode() string {
	if os.Getenv("CORVINT_TASKS_LEASE_GATE_LOAD_SYNC") == "serial" {
		return "serial"
	}
	return "concurrent"
}

func TestCALV0263_LeaseGateLoad(t *testing.T) {
	output := os.Getenv("CORVINT_TASKS_LEASE_GATE_LOAD")
	if output == "" {
		t.Skip("set CORVINT_TASKS_LEASE_GATE_LOAD to a report path")
	}
	gateLoadPublishSyncs(t)
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
	tickets := gateLoadTickets(envInt("CORVINT_TASKS_LEASE_GATE_LOAD_TICKETS", 0), writers)
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
	// A store this long is segmented in use (JSG-V0-008); the direct
	// history appends skip the seal, so seal it before measuring.
	if err := sealSegmentsLocked(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
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
	// The warm-up claim took the complete route on the freshly sealed store;
	// the checkpoint it retained must keep the sealed base for widen and reap
	// to resume (JSG-V0-780). The report records both sequences.
	warmCheckpoint := map[string]uint64{"sealedThrough": segmentThrough(t, repo)}
	if wc := readWriterCheckpoint(repo); wc != nil {
		warmCheckpoint["fullSeq"] = wc.FullSeq
	}

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
		spec, _ := json.Marshal(gateLoadWorker{Repo: repo.PrimaryWorktree, Root: root, Role: role, Ticket: strings.TrimSuffix(entries[i].Name(), ".json"), Out: outs[i], TracePrefix: strings.TrimSuffix(output, ".json") + fmt.Sprintf(".w%d", i), Index: i, ThinkMS: think, Start: start.UnixMilli(), Deadline: deadline.UnixMilli()})
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
		Resumed, ResumeDeclines    int
		Codes                      map[string]int
		WallP50, WallP99, WallMax  float64
		GateWaitP50, GateWaitP99   float64
		GateHoldP50, GateHoldP99   float64
		GateHoldMax, LockHoldP99   float64
		OutsideGateP50             float64
		OutsideGateP99             float64
		OutsideGateMax             float64
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
	outside := map[string][]float64{}
	for _, s := range samples {
		for _, key := range []string{s.Verb, "all"} {
			v := verbs[key]
			if v == nil {
				v = &summary{Codes: map[string]int{}}
				verbs[key] = v
			}
			v.Calls++
			v.Resumed += s.Resumed
			v.ResumeDeclines += s.ResumeDeclines
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
			outside[key] = append(outside[key], s.OutsideGateMS)
			lockHold[key] = append(lockHold[key], s.LockHoldMS)
		}
	}
	for key, v := range verbs {
		v.WallP50, v.WallP99, v.WallMax = pct(wall[key], 0.5), pct(wall[key], 0.99), pct(wall[key], 1)
		v.GateWaitP50, v.GateWaitP99 = pct(gateWait[key], 0.5), pct(gateWait[key], 0.99)
		v.GateHoldP50, v.GateHoldP99, v.GateHoldMax = pct(gateHold[key], 0.5), pct(gateHold[key], 0.99), pct(gateHold[key], 1)
		v.LockHoldP99 = pct(lockHold[key], 0.99)
		v.OutsideGateP50, v.OutsideGateP99, v.OutsideGateMax = pct(outside[key], 0.5), pct(outside[key], 0.99), pct(outside[key], 1)
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
		"fixture":      fmt.Sprintf("historyStoreAt with %d tickets and %d receipts (one request plus one ~%d-byte ticket afterimage per padding receipt); policy capacity %d; one warm-up claim and release", tickets, receipts, historyBodySize, writers+8),
		"workers":      fmt.Sprintf("%d processes re-executing this test binary: worker 0 reaps, each other worker claims a disjoint scope, workers 1-2 then widen it and the rest alternate heartbeat and renew; %d ms think time; each releases at the deadline; in-process lease audit cache cleared before every call", writers, think),
		"statistic":    "per-verb nearest-rank percentiles over all calls; gate percentiles only over calls that entered the preparation gate; Code is wire.CodeOf of a failed call; WallMS is the whole Lease call",
		"publishSyncs": gateLoadSyncMode(), "receipts": receipts, "tickets": tickets, "writers": writers, "seconds": seconds, "elapsedMS": ms(elapsed), "setupMS": setupMS,
		"finalLastSeq": head.LastSeq.Uint64(), "workerFailures": failed, "warmupCheckpoint": warmCheckpoint,
		"lockTimeoutsPerHour": float64(all.LockTimeouts) / elapsed.Hours(),
		"cpuCount":            runtime.NumCPU(), "goVersion": runtime.Version(), "goos": runtime.GOOS,
		"hostLoad": map[string]string{"before": loadBefore, "after": loadAfter},
		"summary":  verbs, "samples": samples,
	}
	// JSG-V0-021: the run is labelled by the loads around it, as B3 is.
	for k, v := range b3Report(loadBefore, loadAfter) {
		report[k] = v
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(output, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("receipts=%d tickets=%d writers=%d elapsed=%s lastSeq=%s LOCK_TIMEOUT=%d", receipts, tickets, writers, elapsed, head.LastSeq, all.LockTimeouts)
	for _, verb := range []string{"claim", "heartbeat", "renew", "widen", "reap", "release", "all"} {
		if v := verbs[verb]; v != nil {
			t.Logf("%-9s calls=%d gated=%d timeouts=%d codes=%v wall p50/p99=%.0f/%.0f ms gate wait p99=%.0f hold p50/p99/max=%.0f/%.0f/%.0f ms outside-gate p50/p99/max=%.0f/%.0f/%.0f ms", verb, v.Calls, v.Gated, v.LockTimeouts, v.Codes, v.WallP50, v.WallP99, v.GateWaitP99, v.GateHoldP50, v.GateHoldP99, v.GateHoldMax, v.OutsideGateP50, v.OutsideGateP99, v.OutsideGateMax)
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
	gateLoadPublishSyncs(t)
	var w gateLoadWorker
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(w.Repo)
	if err != nil {
		t.Fatal(err)
	}
	run := &gateLoadRun{repo: repo, root: w.Root, worker: w.Index, epoch: time.UnixMilli(w.Start)}
	// V1-2540: with CORVINT_TASKS_LEASE_GATE_LOAD_TRACE_MS set, a flight
	// recorder keeps the last seconds of the worker's execution trace and a
	// call whose time outside the gate reaches that many ms writes it out.
	if raw := os.Getenv("CORVINT_TASKS_LEASE_GATE_LOAD_TRACE_MS"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			run.traceMS, run.traceOut = float64(n), w.TracePrefix
			run.recorder = trace.NewFlightRecorder(trace.FlightRecorderConfig{MinAge: 5 * time.Second, MaxBytes: 64 << 20})
			if err := run.recorder.Start(); err != nil {
				t.Fatal(err)
			}
			defer run.recorder.Stop()
		}
	}
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
	// The V1-2540 flight recorder, its threshold and the snapshots written.
	recorder *trace.FlightRecorder
	traceMS  float64
	traceOut string
	traces   int
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
	var started time.Time
	firstAdmission, lastRelease := -1.0, -1.0
	// The live clock the command line gives every writer (cli.writerContext), so a
	// write that prepared across commits resamples recordedAt as the CLI does.
	ctx := authority.WithLockObserver(WithClock(context.Background(), WallClock), func(d time.Duration) { s.LockHoldMS += ms(d) })
	ctx = authority.WithPreparationObserver(ctx, func(o authority.PreparationObservation) {
		s.GateEvents++
		s.GateWaitMS += ms(o.Wait)
		s.GateHoldMS += ms(o.Hold)
		// The observer runs after the gate closes its descriptor and slot,
		// so that close time counts in PreGateMS, not PostGateMS.
		lastRelease = ms(time.Since(started))
		if firstAdmission < 0 {
			firstAdmission = lastRelease - ms(o.Hold) - ms(o.Wait)
		}
	})
	var timing LeaseTiming
	ctx = WithLeaseTiming(ctx, &timing)
	ctx = context.WithValue(ctx, mutationStageKey{}, func(stage string) {
		if r.recorder != nil {
			trace.Log(ctx, "stage", stage)
		}
		if why, ok := strings.CutPrefix(stage, "fast.declined: "); ok {
			s.Declines = append(s.Declines, why)
			return
		}
		if name, raw, ok := strings.Cut(strings.TrimPrefix(stage, "fast."), "="); ok {
			if d, err := time.ParseDuration(raw); err == nil {
				if s.Stages == nil {
					s.Stages = map[string]float64{}
				}
				s.Stages[name] += ms(d)
			}
		}
	})
	forgetLeaseAudits()
	var task *trace.Task
	if r.recorder != nil {
		ctx, task = trace.NewTask(ctx, "lease."+verb)
	}
	// The wall clock brackets Lease alone; the process deltas also span
	// part of the two probes around it.
	before := probeProcess()
	started = time.Now()
	report, err := Lease(ctx, r.repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: fmt.Sprintf("gate-load-%d-%d-%d", r.worker, r.calls, started.UnixNano()), Root: r.root, Lease: req}, WallClock())
	ended := time.Now()
	after := probeProcess()
	if task != nil {
		task.End()
	}
	s.WallMS = ms(ended.Sub(started))
	s.Process = before.to(after)
	s.OutsideGateMS = s.WallMS - s.GateWaitMS - s.GateHoldMS
	if firstAdmission >= 0 {
		s.PreGateMS, s.PostGateMS = firstAdmission, s.WallMS-lastRelease
	}
	s.GuardsMS, s.MonitorCloseMS = ms(timing.Guards), ms(timing.MonitorClose)
	if r.recorder != nil && s.OutsideGateMS >= r.traceMS && r.traces < 4 {
		r.traces++
		name := fmt.Sprintf("%s.c%d.trace", r.traceOut, r.calls)
		if err := writeFlightTrace(r.recorder, name); err != nil {
			s.TraceError = err.Error()
		} else {
			s.Trace = name
		}
	}
	if !r.epoch.IsZero() {
		s.StartMS = ms(started.Sub(r.epoch))
	}
	s.AdmissionWaitMS, s.SnapshotReadMS, s.ValidationMS = ms(timing.AdmissionWait), ms(timing.SnapshotRead), ms(timing.Validation)
	s.LockWaitMS, s.JournalWriteMS, s.FsyncMS = ms(timing.LockWait), ms(timing.JournalWrite), ms(timing.Fsync)
	s.Rounds, s.Transactions = timing.Rounds, timing.Transactions
	s.Resumed, s.ResumeDeclines = timing.Resumed, timing.ResumeDeclines
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
