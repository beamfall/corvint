//go:build darwin || linux

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// JSG-V0-020, V1-2204, V1-2278: the B3 single-writer harness. One writer,
// no peers, calls the lease verbs on a segmented store with the in-process
// audit cache cleared before every call, as a fresh CLI process would, and
// reports nearest-rank p50, p95 and p99 of wall time per verb with the host
// it ran on. A "moved" series also commits one dispatcher write after every
// complete preparation audit (the window of PSR-V0-052 and JSG-V0-011), so
// the resume path is timed apart from the quiet one. Results are
// measurements, never assertions; the maintained tests assert counts.

// b3QualifiedLoad is the highest 1-minute load at which a result qualifies,
// the lower of the ticket's 12 (V1-2278) and the host's core count (decision
// 0549, Targets): a run above it is recorded and labelled unqualified.
const b3QualifiedLoad = 12

var b3Verbs = []string{"heartbeat", "renew", "widen", "reap"}

// b3Stats are nearest-rank percentiles of one verb's wall times in
// milliseconds.
type b3Stats struct {
	Calls, Errors, Audits, Rounds, Resumed int
	MinMS, P50MS, P95MS, P99MS             float64
	MaxMS                                  float64
	// PhaseP50MS are the median of each LeaseTiming phase over the calls.
	PhaseP50MS map[string]float64
	Codes      map[string]int `json:",omitempty"`
}

// b3Percentile is the nearest-rank percentile of an ascending sample: the
// smallest value with at least p of the samples at or below it.
func b3Percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(p * float64(len(sorted))))
	return sorted[min(len(sorted), max(1, rank))-1]
}

func b3Summarize(samples []b3Sample) b3Stats {
	s := b3Stats{Calls: len(samples)}
	wall := make([]float64, 0, len(samples))
	phases := map[string][]float64{}
	for _, x := range samples {
		wall = append(wall, x.WallMS)
		s.Audits += x.Audits
		s.Rounds += x.Rounds
		s.Resumed += x.Resumed
		for name, v := range map[string]float64{"snapshotRead": x.SnapshotReadMS, "validation": x.ValidationMS, "lockHold": x.LockHoldMS, "journalWrite": x.JournalWriteMS, "fsync": x.SyncMS} {
			phases[name] = append(phases[name], v)
		}
		if x.Code != "" {
			s.Errors++
			if s.Codes == nil {
				s.Codes = map[string]int{}
			}
			s.Codes[x.Code]++
		}
	}
	sort.Float64s(wall)
	if len(wall) > 0 {
		s.MinMS, s.MaxMS = wall[0], wall[len(wall)-1]
	}
	s.P50MS, s.P95MS, s.P99MS = b3Percentile(wall, 0.50), b3Percentile(wall, 0.95), b3Percentile(wall, 0.99)
	s.PhaseP50MS = map[string]float64{}
	for name, v := range phases {
		sort.Float64s(v)
		s.PhaseP50MS[name] = b3Percentile(v, 0.50)
	}
	return s
}

// b3Qualification labels a series by the worst 1-minute load observed around
// it. A load that was not observed does not qualify.
func b3Qualification(load1 []float64, cpus int) string {
	limit := float64(min(b3QualifiedLoad, cpus))
	worst := -1.0
	for _, l := range load1 {
		if l < 0 {
			worst = -1
			break
		}
		worst = math.Max(worst, l)
	}
	switch {
	case worst < 0:
		return "UNQUALIFIED: host load NOT_OBSERVED"
	case worst > limit:
		return fmt.Sprintf("UNQUALIFIED: 1-minute load %.2f above %.0f", worst, limit)
	}
	return fmt.Sprintf("QUALIFIED: 1-minute load %.2f at or below %.0f", worst, limit)
}

// b3ParseLoad1 is the 1-minute load in a hostLoad string, or -1.
func b3ParseLoad1(load string) float64 {
	f := strings.Fields(load)
	if len(f) == 0 {
		return -1
	}
	v, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return -1
	}
	return v
}

// b3Report names the host a report was measured on and labels the run by the
// worse of the loads around it (JSG-V0-020, JSG-V0-021).
func b3Report(loadBefore, loadAfter string) map[string]any {
	return map[string]any{"host": b3Host(), "qualification": b3Qualification([]float64{b3ParseLoad1(loadBefore), b3ParseLoad1(loadAfter)}, runtime.NumCPU())}
}

// b3Sample is one lease call.
type b3Sample struct {
	Verb, Code, Detail string
	WallMS             float64
	// Audits counts the complete preparation audits the call ran, Rounds
	// its leaseWrite rounds and Resumed the writes it resumed from its own
	// audit; the Walk counts and the millisecond phases are its LeaseTiming.
	Audits, Rounds, Resumed            int
	CompleteWalks, CompleteMoved       int
	PrefixWalks, ResumeDeclines        int
	SnapshotReadMS, ValidationMS       float64
	LockHoldMS, JournalWriteMS, SyncMS float64
}

// b3Series is one (size, mode) run: its raw samples, per-verb statistics and
// host.
type b3Series struct {
	Receipts      int
	Mode          string
	Iterations    int
	LoadBefore    string
	LoadAfter     string
	Qualification string
	Stats         map[string]b3Stats
	Samples       []b3Sample
}

// b3Run times iterations calls of each verb on repo, a segmented store with
// the pool policy. moved adds a dispatcher write after every complete
// preparation audit. Every call, the dispatcher's included, uses the
// harness's own non-decreasing clock, so receipts stay in order whichever
// route commits; reap runs last, at a clock past the claim's lease.
func b3Run(t *testing.T, repo *intent.Repository, root string, iterations int, moved bool) (samples []b3Sample) {
	t.Helper()
	now := WallClock()
	// step advances the harness clock by d, never behind the wall clock: a
	// call that outlasts the steps (a widen at 40,000 receipts) records its
	// receipts at the wall time, and a reap's lease must expire after that.
	step := func(d time.Duration) wire.Timestamp {
		if wall := WallClock(); wall > now {
			now = wall
		}
		at, err := time.Parse(time.RFC3339, string(now))
		if err != nil {
			t.Fatal(err)
		}
		now = wire.Timestamp(at.Add(d).UTC().Format(time.RFC3339))
		return now
	}
	entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
	if err != nil || len(entries) < iterations+1 {
		t.Fatalf("tickets %d %v", len(entries), err)
	}
	local := func(i int) string { return strings.TrimSuffix(entries[i].Name(), ".json") }
	dispatcher := 0
	// The live clock the command line gives every writer (cli.writerContext):
	// the wall clock, or the harness's clock while that is ahead of it, so a
	// write that waited or prepared across commits resamples as the CLI does.
	live := func() wire.Timestamp {
		if wall := WallClock(); wall > now {
			return wall
		}
		return now
	}
	call := func(verb, id string, req transaction.LeaseRequest) *Report {
		forgetLeaseAudits()
		var timing LeaseTiming
		ctx := WithLeaseTiming(WithClock(context.Background(), live), &timing)
		var audits int
		hooks := hooksForInventory(ctx)
		audit := hooks.audit
		at := now
		hooks.audit = func(r journal.Reader) (*journal.Result, journal.PhysicalObservation, error) {
			audits++
			res, obs, aerr := audit(r)
			if moved {
				dispatcher++
				name := fmt.Sprintf("b3-dispatcher-%d", dispatcher)
				if rep, merr := Mutate(WithClock(context.Background(), live), repo, historyActor, historyCreate(name, "b3 "+name), at); merr != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Errorf("dispatcher write: %+v %v", rep, merr)
				}
			}
			return res, obs, aerr
		}
		ctx = context.WithValue(ctx, inventoryHooksKey{}, hooks)
		started := time.Now()
		rep, lerr := Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: root, Lease: req}, now)
		s := b3Sample{Verb: verb, WallMS: ms(time.Since(started)), Audits: audits, Rounds: timing.Rounds, Resumed: timing.Resumed,
			CompleteWalks: timing.CompleteWalks, CompleteMoved: timing.CompleteMoved, PrefixWalks: timing.PrefixWalks, ResumeDeclines: timing.ResumeDeclines,
			SnapshotReadMS: ms(timing.SnapshotRead), ValidationMS: ms(timing.Validation), LockHoldMS: ms(timing.LockHold), JournalWriteMS: ms(timing.JournalWrite), SyncMS: ms(timing.Fsync)}
		switch {
		case lerr != nil:
			s.Code = string(wire.CodeOf(lerr))
			if s.Code == "" {
				s.Code = "ERROR"
			}
		case rep.Outcome.Outcome != mutation.OutcomeCompleted:
			s.Code, s.Detail = "OUTCOME_"+string(rep.Outcome.Outcome), rep.Detail
		}
		if verb != "claim" {
			samples = append(samples, s)
		}
		if s.Code != "" {
			if verb == "claim" {
				t.Fatalf("%s: %s %v", id, s.Code, lerr)
			}
			return nil
		}
		return rep
	}
	claim := call("claim", "b3-claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: fixture.TicketID(local(0)), Holder: "agent-b3", LeaseMinutes: "60", Scope: []string{"b3/0/"}})
	if claim == nil {
		t.Fatal("claim failed")
	}
	attempt, generation := claim.AttemptID, claim.Generation
	for i := 0; i < iterations; i++ {
		step(time.Second)
		call("heartbeat", fmt.Sprintf("b3-heartbeat-%d", i), transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: attempt, Generation: generation})
	}
	for i := 0; i < iterations; i++ {
		step(time.Second)
		call("renew", fmt.Sprintf("b3-renew-%d", i), transaction.LeaseRequest{Verb: transaction.LeaseRenew, AttemptID: attempt, Generation: generation, LeaseMinutes: "60"})
	}
	for i := 1; i <= iterations; i++ {
		step(time.Second)
		if rep := call("widen", fmt.Sprintf("b3-widen-%d", i), transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: attempt, Generation: generation, Scope: []string{fmt.Sprintf("b3/%d/", i)}}); rep != nil {
			generation = rep.Generation
		}
	}
	for i := 1; i <= iterations; i++ {
		step(time.Second)
		doomed := call("claim", fmt.Sprintf("b3-reap-claim-%d", i), transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: fixture.TicketID(local(i)), Holder: "agent-b3-reap", LeaseMinutes: "60", Scope: []string{fmt.Sprintf("b3/reap/%d/", i)}})
		step(61 * time.Minute)
		call("reap", fmt.Sprintf("b3-reap-%d", i), transaction.LeaseRequest{Verb: transaction.LeaseReap, AttemptID: doomed.AttemptID, Generation: doomed.Generation})
	}
	return samples
}

// b3Fixture builds a sealed store of receipts at base/<receipts>/live once,
// keeps a stamp-preserving copy at base/<receipts>/pristine, and returns a
// function that restores the copy over live (the head binds its primary
// worktree path, so the store is always used where it was built) and
// returns the resolved repository and a worktree root for claims.
func b3Fixture(t *testing.T, base string, receipts, tickets int) func() (*intent.Repository, string) {
	t.Helper()
	dir := filepath.Join(base, strconv.Itoa(receipts))
	pristine, live := filepath.Join(dir, "pristine"), filepath.Join(dir, "live")
	// A finished fixture of this size and ticket count is reused, so a
	// store kept at CORVINT_TASKS_B3_STORE is built once.
	ready := filepath.Join(dir, "ready-"+strconv.Itoa(tickets))
	if _, err := os.Stat(ready); err != nil {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		repo := historyStoreAt(t, filepath.Join(live, "repo"), tickets, receipts)
		if err := sealSegmentsLocked(context.Background(), repo); err != nil {
			t.Fatal(err)
		}
		gateLoadPolicy(t, repo, 8)
		if out, err := exec.Command("cp", "-a", live, pristine).CombinedOutput(); err != nil {
			t.Fatalf("cp -a: %v %s", err, out)
		}
		if err := os.WriteFile(ready, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return func() (*intent.Repository, string) {
		t.Helper()
		if err := os.RemoveAll(live); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("cp", "-a", pristine, live).CombinedOutput(); err != nil {
			t.Fatalf("cp -a: %v %s", err, out)
		}
		restored, err := intent.Resolve(filepath.Join(live, "repo"))
		if err != nil {
			t.Fatal(err)
		}
		root := filepath.Join(live, "worktree")
		holdGit(t, root, "init", "-q", "-b", "main")
		holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
		return restored, root
	}
}

func b3Measure(t *testing.T, repo *intent.Repository, root string, receipts, iterations int, mode string) b3Series {
	t.Helper()
	series := b3Series{Receipts: receipts, Mode: mode, Iterations: iterations, LoadBefore: hostLoad()}
	series.Samples = b3Run(t, repo, root, iterations, mode == "moved")
	series.LoadAfter = hostLoad()
	// The label comes from the recorded loads themselves, never from a
	// further reading that could fall on the other side of the threshold.
	series.Qualification = b3Qualification([]float64{b3ParseLoad1(series.LoadBefore), b3ParseLoad1(series.LoadAfter)}, runtime.NumCPU())
	series.Stats = map[string]b3Stats{}
	for _, verb := range b3Verbs {
		var own []b3Sample
		for _, s := range series.Samples {
			if s.Verb == verb {
				own = append(own, s)
			}
		}
		series.Stats[verb] = b3Summarize(own)
	}
	return series
}

// b3Host names the host a report was measured on.
func b3Host() map[string]any {
	kernel := "NOT_OBSERVED"
	if raw, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		kernel = strings.TrimSpace(string(raw))
	} else if out, err := exec.Command("uname", "-r").Output(); err == nil {
		kernel = strings.TrimSpace(string(out))
	}
	return map[string]any{"goos": runtime.GOOS, "goarch": runtime.GOARCH, "kernel": kernel, "cpuCount": runtime.NumCPU(), "gomaxprocs": runtime.GOMAXPROCS(0), "goVersion": runtime.Version(), "load": hostLoad()}
}

// TestJSGV0020_SingleWriterHarness is the opt-in B3 measurement, not a
// timing assertion; a plain `go test` skips it. CORVINT_TASKS_B3 names the
// JSON report. CORVINT_TASKS_B3_RECEIPTS lists the store sizes (default
// 10000,40000), CORVINT_TASKS_B3_ITERATIONS the calls per verb (default 20)
// and CORVINT_TASKS_B3_MODES the series, quiet and moved (both by default).
// CORVINT_TASKS_B3_STORE, when set, is a directory whose sealed fixtures are
// built once and kept for later runs; otherwise they are temporary.
func TestJSGV0020_SingleWriterHarness(t *testing.T) {
	output := os.Getenv("CORVINT_TASKS_B3")
	if output == "" {
		t.Skip("set CORVINT_TASKS_B3 to a report path")
	}
	var sizes []int
	for _, f := range strings.Split(envOr("CORVINT_TASKS_B3_RECEIPTS", "10000,40000"), ",") {
		n, err := strconv.Atoi(f)
		if err != nil || n < 1100 {
			t.Fatalf("CORVINT_TASKS_B3_RECEIPTS entry %q", f)
		}
		sizes = append(sizes, n)
	}
	iterations, err := strconv.Atoi(envOr("CORVINT_TASKS_B3_ITERATIONS", "20"))
	if err != nil || iterations < 1 {
		t.Fatalf("CORVINT_TASKS_B3_ITERATIONS")
	}
	modes := strings.Split(envOr("CORVINT_TASKS_B3_MODES", "quiet,moved"), ",")
	base := os.Getenv("CORVINT_TASKS_B3_STORE")
	if base == "" {
		tmp, err := os.MkdirTemp("", "atm-b3-")
		if err != nil {
			t.Fatal(err)
		}
		if base, err = filepath.EvalSymlinks(tmp); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(base) })
	}
	report := map[string]any{"tickets": []string{"V1-2204", "V1-2278"}, "requirement": "JSG-V0-020", "host": b3Host(),
		"statistic": "nearest-rank percentiles of wall time per Lease call, in-process audit cache cleared before every call; reap is timed on an expired claim; moved adds one dispatcher write after each complete preparation audit",
		"targets":   "T3 (decision 0549): single-writer widen or reap at 40,000 receipts, at most 2000 ms"}
	var series []b3Series
	for _, receipts := range sizes {
		restore := b3Fixture(t, base, receipts, iterations+2)
		for _, mode := range modes {
			repo, root := restore()
			s := b3Measure(t, repo, root, receipts, iterations, mode)
			series = append(series, s)
			for _, verb := range b3Verbs {
				st := s.Stats[verb]
				t.Logf("receipts=%d %-5s %-9s n=%d err=%d p50=%.0f p95=%.0f p99=%.0f max=%.0f ms audits=%d resumed=%d load=%s -> %s [%s]", receipts, mode, verb, st.Calls, st.Errors, st.P50MS, st.P95MS, st.P99MS, st.MaxMS, st.Audits, st.Resumed, s.LoadBefore+" / "+s.LoadAfter, "", s.Qualification)
			}
		}
	}
	report["series"] = series
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

// JSG-V0-020: nearest-rank percentiles, and a result qualifies only at or
// below the lower of load 12 and the host's core count.
func TestJSGV0020_PercentilesAndQualification(t *testing.T) {
	t.Parallel()
	ten := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	for _, tc := range []struct {
		p    float64
		want float64
	}{{0.50, 5}, {0.95, 10}, {0.99, 10}, {0.10, 1}, {1, 10}} {
		if got := b3Percentile(ten, tc.p); got != tc.want {
			t.Errorf("p%.0f of 1..10 = %v, want %v", tc.p*100, got, tc.want)
		}
	}
	if got := b3Percentile(nil, 0.5); got != 0 {
		t.Errorf("empty sample: %v", got)
	}
	hundred := make([]float64, 100)
	for i := range hundred {
		hundred[i] = float64(i + 1)
	}
	if got := b3Percentile(hundred, 0.99); got != 99 {
		t.Errorf("p99 of 1..100 = %v, want 99", got)
	}
	for _, tc := range []struct {
		load      []float64
		cpus      int
		qualified bool
	}{
		{[]float64{3.9, 4.0}, 4, true},
		{[]float64{3.9, 4.1}, 4, false},
		{[]float64{11, 12}, 64, true},
		{[]float64{12.01, 1}, 64, false},
		{[]float64{-1, 1}, 4, false},
		{[]float64{-1}, 4, false},
		{nil, 4, false},
	} {
		if got := strings.HasPrefix(b3Qualification(tc.load, tc.cpus), "QUALIFIED"); got != tc.qualified {
			t.Errorf("load %v on %d cpus: %s", tc.load, tc.cpus, b3Qualification(tc.load, tc.cpus))
		}
	}
}

// JSG-V0-021: the 15-writer B2 report carries the host tuple and the same
// qualification label as B3, from the loads before and after the run.
func TestJSGV0021_GateLoadReportCarriesHostAndQualification(t *testing.T) {
	t.Parallel()
	quiet := b3Report("0.10 0.10 0.10", "0.20 0.15 0.12")
	if q, _ := quiet["qualification"].(string); !strings.HasPrefix(q, "QUALIFIED") {
		t.Errorf("a quiet host: %v", quiet["qualification"])
	}
	loaded := b3Report("0.10 0.10 0.10", "40.00 20.00 10.00")
	if q, _ := loaded["qualification"].(string); !strings.HasPrefix(q, "UNQUALIFIED") {
		t.Errorf("a loaded host: %v", loaded["qualification"])
	}
	if q, _ := b3Report("NOT_OBSERVED", "0.1 0.1 0.1")["qualification"].(string); !strings.HasPrefix(q, "UNQUALIFIED") {
		t.Errorf("an unobserved load: %v", q)
	}
	host, ok := loaded["host"].(map[string]any)
	if !ok || host["cpuCount"] != runtime.NumCPU() || host["goVersion"] != runtime.Version() || host["kernel"] == "" {
		t.Errorf("host %+v", loaded["host"])
	}
}

// JSG-V0-020: the harness runs end to end on a small sealed store in both
// modes: every verb is timed the requested number of times with no failure,
// and each widen and reap resumes from the sealed base the fixture's
// complete-route writes keep (JSG-V0-780) with no complete audit.
func TestJSGV0020_HarnessRunsBothModes(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	const iterations = 2
	restore := b3Fixture(t, t.TempDir(), segmentHistory, iterations+2)
	for _, mode := range []string{"quiet", "moved"} {
		repo, root := restore()
		s := b3Measure(t, repo, root, segmentHistory, iterations, mode)
		if want := b3Qualification([]float64{b3ParseLoad1(s.LoadBefore), b3ParseLoad1(s.LoadAfter)}, runtime.NumCPU()); s.Qualification != want {
			t.Errorf("%s: qualification %q, but the recorded loads %q and %q give %q", mode, s.Qualification, s.LoadBefore, s.LoadAfter, want)
		}
		for _, verb := range b3Verbs {
			st := s.Stats[verb]
			if st.Calls != iterations || st.Errors != 0 {
				t.Fatalf("%s %s: %+v", mode, verb, st)
			}
			if !(st.MinMS <= st.P50MS && st.P50MS <= st.P95MS && st.P95MS <= st.P99MS && st.P99MS <= st.MaxMS) {
				t.Errorf("%s %s: percentiles out of order: %+v", mode, verb, st)
			}
			if verb != "widen" && verb != "reap" {
				continue
			}
			// The fixture's complete-route writes keep the sealed base
			// (JSG-V0-780), so in both series every widen and reap resumes
			// from it and runs no complete preparation audit.
			if st.Audits != 0 || st.Resumed != iterations {
				t.Errorf("%s %s: %d audits and %d resumed over %d calls, want no audit and one resume per call", mode, verb, st.Audits, st.Resumed, iterations)
			}
		}
	}
}
