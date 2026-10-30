package transaction

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// reportSecret is a token the shared secret screen matches, assembled so
// the source holds no literal credential.
var reportSecret = "ghp_" + strings.Repeat("a1B2", 9)

func reportState(t *testing.T, entries ...snapshot.PoolEntry) *snapshot.PoolState {
	t.Helper()
	q, err := wire.ParseQueueID("", fixture.QueueID)
	if err != nil {
		t.Fatal(err)
	}
	return &snapshot.PoolState{QueueID: q, Entries: entries}
}

func reportContext(st *snapshot.PoolState, member, at string, seq wire.Size, doc []byte) leaseContext {
	return leaseContext{r: Request{RequestID: "report-" + string(seq)}, l: &LeaseRequest{Verb: LeasePoolReport, Member: member, Evidence: string(wire.Sum(doc))}, seq: seq, in: Input{RecordedAt: wire.Timestamp(at), LeaseFacts: LeaseFacts{Pool: PoolFacts{Detail: doc}}}, st: inputState{pools: st}}
}

func plannedPools(t *testing.T, out leaseOutcome) *snapshot.PoolState {
	t.Helper()
	if out.result != nil {
		t.Fatalf("refused %+v", *out.result)
	}
	st, err := snapshot.DecodePools(out.posts["pools.json"])
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func refusedCode(t *testing.T, out leaseOutcome, code string) {
	t.Helper()
	if out.result == nil || len(out.result.Outcome.Codes) == 0 || out.result.Outcome.Codes[0] != code {
		t.Fatalf("want %s, got %+v", code, out.result)
	}
}

// PSR-V0-260: the detail document is deterministic, bounded to the tail of
// the input, replaces invalid UTF-8 and control characters, and redacts
// every line the secret screen touches; its decoder refuses a document
// that is not canonical or still carries a secret.
func TestPSRV0260_PoolRecoveryDetailBoundedAndRedacted(t *testing.T) {
	raw := []byte("connect db:5432 refused\x07\xff\ntoken " + reportSecret + " leaked\nretry 3 failed\n")
	doc, err := NewPoolRecoveryDetail("cleanup", "FAILED", raw)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := NewPoolRecoveryDetail("cleanup", "FAILED", raw)
	if string(doc) != string(again) {
		t.Fatal("detail document is not deterministic")
	}
	d, err := DecodePoolRecoveryDetail(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(doc), reportSecret) || !strings.Contains(d.Detail, PoolRecoveryRedacted+"\n") || d.RedactedLines != "1" || d.Truncated || d.Bytes != wire.SizeOf(uint64(len(raw))) || !strings.HasPrefix(d.Detail, "connect db:5432 refused??\n") || !strings.HasSuffix(d.Detail, "retry 3 failed\n") {
		t.Fatalf("detail %+v", d)
	}
	long := []byte(strings.Repeat("é", 3000) + "\nlast line")
	doc, err = NewPoolRecoveryDetail("health", "FAILED", long)
	if err != nil {
		t.Fatal(err)
	}
	if d, err = DecodePoolRecoveryDetail(doc); err != nil || !d.Truncated || len(d.Detail) > MaxPoolRecoveryDetailBytes || !strings.HasSuffix(d.Detail, "last line") {
		t.Fatalf("truncated %+v %v", d, err)
	}
	for _, bad := range []struct {
		kind, result string
		raw          []byte
		code         string
	}{{"other", "FAILED", nil, wire.CodeMalformed}, {"health", "PASSED", nil, wire.CodeMalformed}, {"health", "FAILED", make([]byte, MaxPoolReportInputBytes+1), wire.CodeLimitExceeded}} {
		if _, err := NewPoolRecoveryDetail(bad.kind, bad.result, bad.raw); wire.CodeOf(err) != bad.code {
			t.Fatalf("%s/%s: %v, want %s", bad.kind, bad.result, err, bad.code)
		}
	}
	leaky := PoolRecoveryDetail{Kind: "health", Result: "FAILED", Detail: reportSecret, Bytes: "1", RedactedLines: "0"}
	if !obligation.MatchesSecretScreen(reportSecret) {
		t.Fatal("fixture secret is not screened")
	}
	if _, err := DecodePoolRecoveryDetail(leaky.encode()); wire.CodeOf(err) != wire.CodeSecretDetected {
		t.Fatalf("leaky detail: %v", err)
	}
	if _, err := DecodePoolRecoveryDetail(append(doc, ' ')); err == nil {
		t.Fatal("noncanonical detail accepted")
	}
}

// PSR-V0-262: identical failures (same kind and digest) count up from the
// first one's time; a different failure restarts at one, and a member that
// is not QUARANTINED, or a detail that differs from its digest, is refused.
func TestPSRV0262_PoolReportCountsConsecutiveIdenticalFailures(t *testing.T) {
	quarantined := validPoolEntry(t, "b", "QUARANTINED", func(e *snapshot.PoolEntry) { e.QuarantinedAt = "2026-10-25T08:00:00Z" })
	st := reportState(t, quarantined)
	doc, _ := NewPoolRecoveryDetail("cleanup", "FAILED", []byte("disk full\n"))
	at := []string{"2026-10-25T09:00:00Z", "2026-10-25T09:05:00Z", "2026-10-25T09:10:00Z"}
	for i, ts := range at {
		out := planPoolReport(reportContext(st, "b", ts, wire.SizeOf(uint64(i+2)), doc))
		if string(out.posts["evidence/"+string(wire.Sum(doc))]) != string(doc) {
			t.Fatalf("detail not retained under evidence/")
		}
		st = plannedPools(t, out)
		x := st.Entries[0].Recovery
		if x == nil || x.Consecutive != wire.CountOf(int64(i+1)) || x.FirstObservedAt != wire.Timestamp(at[0]) || x.ObservedAt != wire.Timestamp(ts) || x.DetailSha256 != wire.Sum(doc) || x.Kind != "cleanup" || st.Entries[0].QuarantinedAt != "2026-10-25T08:00:00Z" {
			t.Fatalf("report %d: %+v", i, x)
		}
	}
	now, _ := time.Parse(timestampLayout, at[2])
	if trigger, stalled := snapshot.PoolRecoveryStall(st.Entries[0], now); !stalled || trigger != snapshot.PoolStallConsecutiveFailures {
		t.Fatalf("3 identical failures not stalled: %q", trigger)
	}
	other, _ := NewPoolRecoveryDetail("cleanup", "FAILED", []byte("disk full again\n"))
	st = plannedPools(t, planPoolReport(reportContext(st, "b", "2026-10-25T09:15:00Z", "9", other)))
	if x := st.Entries[0].Recovery; x.Consecutive != "1" || x.FirstObservedAt != "2026-10-25T09:15:00Z" {
		t.Fatalf("different failure did not restart: %+v", x)
	}
	allocated := reportState(t, validPoolEntry(t, "b", "ALLOCATED"))
	refusedCode(t, planPoolReport(reportContext(allocated, "b", at[0], "2", doc)), wire.CodeQuiescenceUnproved)
	refusedCode(t, planPoolReport(reportContext(st, "missing", at[0], "2", doc)), wire.CodeFenced)
	c := reportContext(st, "b", at[0], "2", doc)
	c.l.Evidence = string(wire.Sum([]byte("other")))
	refusedCode(t, planPoolReport(c), wire.CodeMalformed)
}

// PSR-V0-261: every planned pools.json stamps quarantinedAt = recordedAt on
// an entry that enters QUARANTINED, keeps it across a cleanup cycle of the
// same allocation, never invents one for a legacy quarantine, and clears
// it (and any recovery record) once the member leaves quarantine.
func TestPSRV0261_StampQuarantineTimestamp(t *testing.T) {
	const at = "2026-10-25T10:00:00Z"
	stamp := func(prior *snapshot.PoolState, planned snapshot.PoolEntry) snapshot.PoolEntry {
		t.Helper()
		raw, err := reportState(t, planned).Encode()
		if err != nil {
			t.Fatal(err)
		}
		c := leaseContext{in: Input{RecordedAt: at}, st: inputState{pools: prior}}
		return plannedPools(t, c.stampQuarantine(leaseOutcome{posts: map[string][]byte{"pools.json": raw}})).Entries[0]
	}
	allocated := validPoolEntry(t, "b", "ALLOCATED")
	if en := stamp(reportState(t, allocated), validPoolEntry(t, "b", "QUARANTINED")); en.QuarantinedAt != at || en.Recovery != nil {
		t.Fatalf("entering quarantine: %+v", en)
	}
	legacy := validPoolEntry(t, "b", "QUARANTINED")
	if en := stamp(reportState(t, legacy), legacy); en.QuarantinedAt != "" {
		t.Fatalf("legacy quarantine invented a time: %q", en.QuarantinedAt)
	}
	rec := &snapshot.PoolRecovery{Kind: "cleanup", Result: "FAILED", DetailSha256: wire.Sum([]byte("d")), Consecutive: "2", FirstObservedAt: "2026-10-25T08:00:00Z", ObservedAt: "2026-10-25T09:00:00Z", ObservedSeq: "4"}
	since := validPoolEntry(t, "b", "QUARANTINED", func(e *snapshot.PoolEntry) { e.QuarantinedAt, e.Recovery = "2026-10-25T07:00:00Z", rec })
	cleaning := validPoolEntry(t, "b", "CLEANING", func(e *snapshot.PoolEntry) { e.Recovery = rec })
	en := stamp(reportState(t, since), cleaning)
	if en.QuarantinedAt != since.QuarantinedAt || en.Recovery == nil || en.Recovery.Consecutive != "2" {
		t.Fatalf("cleaning lost the quarantine: %+v", en)
	}
	back := validPoolEntry(t, "b", "QUARANTINED", func(e *snapshot.PoolEntry) { e.Recovery = rec })
	if en := stamp(reportState(t, en), back); en.QuarantinedAt != since.QuarantinedAt || en.Recovery == nil {
		t.Fatalf("failed cleanup restarted the quarantine: %+v", en)
	}
	freshCleaning := validPoolEntry(t, "b", "CLEANING", func(e *snapshot.PoolEntry) { e.QuarantinedAt, e.Recovery = since.QuarantinedAt, rec })
	if en := stamp(reportState(t, allocated), freshCleaning); en.QuarantinedAt != "" || en.Recovery != nil {
		t.Fatalf("cleaning outside a quarantine kept its record: %+v", en)
	}
	untouched, _ := reportState(t, legacy).Encode()
	c := leaseContext{in: Input{RecordedAt: at}, st: inputState{pools: reportState(t, legacy)}}
	if out := c.stampQuarantine(leaseOutcome{posts: map[string][]byte{"pools.json": untouched}}); string(out.posts["pools.json"]) != string(untouched) {
		t.Fatal("an unchanged pools.json was re-encoded")
	}
}

// PSR-V0-264: with every eligible member QUARANTINED and stalled, acquire
// and claim refuse POOL_MEMBER_RECOVERY_STALLED naming the members; one
// member that is not stalled leaves the plain RESOURCE_COLLISION path.
func TestPSRV0264_StalledPoolRefusesTyped(t *testing.T) {
	stalled := func(m string) snapshot.PoolEntry {
		return validPoolEntry(t, m, "QUARANTINED", func(e *snapshot.PoolEntry) {
			e.QuarantinedAt = "2026-10-25T08:00:00Z"
			e.Recovery = &snapshot.PoolRecovery{Kind: "health", Result: "FAILED", DetailSha256: wire.Sum([]byte(m)), Consecutive: "3", FirstObservedAt: "2026-10-25T08:00:00Z", ObservedAt: "2026-10-25T08:10:00Z", ObservedSeq: "5"}
		})
	}
	c := leaseContext{in: Input{RecordedAt: "2026-10-25T09:00:00Z"}, st: inputState{pools: reportState(t, stalled("a"), stalled("b"))}}
	e := c.stalledRefusal("db", []string{"a", "b"})
	if wire.CodeOf(e) != wire.CodePoolMemberRecoveryStalled || !strings.Contains(e.Error(), "a, b") || !strings.Contains(e.Error(), "pool status --pool db") {
		t.Fatalf("stalled refusal %v", e)
	}
	if wire.RetryOf(wire.CodePoolMemberRecoveryStalled).Retryable {
		t.Fatal("a stalled pool is not retryable without an operator")
	}
	young := validPoolEntry(t, "b", "QUARANTINED", func(e *snapshot.PoolEntry) { e.QuarantinedAt = "2026-10-25T08:00:00Z" })
	c.st.pools = reportState(t, stalled("a"), young)
	if e := c.stalledRefusal("db", []string{"a", "b"}); e != nil {
		t.Fatalf("one unstalled member refused typed: %v", e)
	}
	// Twenty-four hours after quarantinedAt the young member ages into a stall.
	c.in.RecordedAt = "2026-10-26T08:00:00Z"
	if e := c.stalledRefusal("db", []string{"a", "b"}); wire.CodeOf(e) != wire.CodePoolMemberRecoveryStalled {
		t.Fatalf("aged quarantine not stalled: %v", e)
	}
}

// PSR-V0-260: redaction marks every line a match overlaps, a match that
// spans lines included, and stays near-linear on a large admitted input
// with many short lines and many matches.
func TestPSRV0260_RedactLinesSpansAndScale(t *testing.T) {
	text := "keep\nauth " + reportSecret[:10] + "\n" + reportSecret[10:] + " tail\nkeep too\n"
	naive := func(text string) string {
		spans := obligation.SecretScreenSpans(text)
		lines := strings.SplitAfter(text, "\n")
		start := 0
		for i, l := range lines {
			end := start + len(l)
			for _, sp := range spans {
				if sp[0] < end && sp[1] > start {
					lines[i] = PoolRecoveryRedacted + l[len(strings.TrimSuffix(l, "\n")):]
					break
				}
			}
			start = end
		}
		return strings.Join(lines, "")
	}
	for _, in := range []string{text, "auth " + reportSecret + "\n", reportSecret, "plain\n\n", ""} {
		if got, _ := redactLines(in); got != naive(in) {
			t.Fatalf("redactLines(%q) = %q, want %q", in, got, naive(in))
		}
	}
	var b strings.Builder
	for b.Len() < MaxPoolReportInputBytes-64 {
		b.WriteString("\n\n\n\n\n\n\n\n\n\n\n\nauth " + reportSecret + "\n")
	}
	started := time.Now()
	doc, err := NewPoolRecoveryDetail("cleanup", "FAILED", []byte(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("redaction of %d bytes took %s", b.Len(), elapsed)
	}
	if d, err := DecodePoolRecoveryDetail(doc); err != nil || !d.Truncated || d.RedactedLines.Int() < 10000 {
		t.Fatalf("large detail %v %+v", err, d)
	}
}

// PSR-V0-262: a sweep cleanup phase observation is a native cleanup
// observation, so it supersedes a recorded cleanup failure whatever its
// outcome; a recorded health failure survives it.
func TestPSRV0262_SweepCleanupSupersedesRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, kind, class string
		cleared           bool
	}{
		{"pass-cleanup", "cleanup", "EXIT_ZERO", true},
		{"fail-cleanup", "cleanup", "EXIT_NONZERO", true},
		{"pass-health", "health", "EXIT_ZERO", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, e := intent.DecodeQueue(fixture.QueueBytes())
			if e != nil {
				t.Fatal(e)
			}
			pv := fixture.PolicyValue()
			pv.Obj.Set("pools", wire.Array(object("id", s("db"), "members", wire.Strings([]string{"a"}))))
			policy, e := intent.DecodePolicy(wire.EncodeFile(pv))
			if e != nil {
				t.Fatal(e)
			}
			policy.Pools[0].MemberConfig["a"] = intent.MemberConfig{Cleanup: &intent.PoolCommand{}, SafeReuse: &intent.SafeReuse{MaxAttempts: "2", ExpectExit: "0"}}
			d := wire.Sum(nil)
			rev, tree := strings.Repeat("a", 40), strings.Repeat("b", 40)
			exit := wire.Count("0")
			if tc.class == "EXIT_NONZERO" {
				exit = "1"
			}
			log := snapshot.EncodePoolSweepLog(nil, nil)
			o := snapshot.PoolSweepObservation{AllocationID: d, DefinitionSha256: policy.MemberDefinition("db", "a"), Owner: d, Log: wire.Sum(log), Environment: d, EnvFile: d, Phase: "cleanup", Attempt: "1", Revision: rev, Tree: tree, Class: tc.class, Exit: &exit, Passed: tc.class == "EXIT_ZERO", GroupClean: true, Stdout: d, Stderr: d, Timing: snapshot.PoolSweepTiming{StartedAt: "1", Deadline: "2", WaitReturnedAt: "2", CleanupEndedAt: "3", ExecutionMillis: "0", CleanupMillis: "0", CleanupAllowanceMillis: "5000"}}
			raw := o.Encode()
			rec := &snapshot.PoolRecovery{Kind: tc.kind, Result: "FAILED", DetailSha256: wire.Sum([]byte("d")), Consecutive: "3", FirstObservedAt: "2026-10-25T08:00:00Z", ObservedAt: "2026-10-25T09:00:00Z", ObservedSeq: "1"}
			en := snapshot.PoolEntry{PoolAllocation: snapshot.PoolAllocation{PoolID: "db", MemberID: "a", AllocationID: d, DefinitionSha256: o.DefinitionSha256, AllocatedSeq: "1"}, State: "CLEANING", Holder: "runner", Stage: "implement", Generation: "0", ChangedSeq: "1", PolicySha256: d, RequestSha256: d, RunnerPID: "1", RunnerStarted: "identity", CommandKind: "sweep", CommandRevision: rev, QuarantinedAt: "2026-10-25T07:00:00Z", Recovery: rec, Sweep: &snapshot.PoolSweepOwner{RequestSha256: d, Phase: "cleanup", Attempt: "1", Tree: tree}}
			c := leaseContext{l: &LeaseRequest{Verb: LeasePoolObserve, Member: "a", Allocation: string(d), Evidence: string(d)}, in: Input{LeaseFacts: LeaseFacts{Pool: PoolFacts{Observation: raw, SweepLog: log}}}, st: inputState{policy: policy, pools: &snapshot.PoolState{QueueID: q.QueueID, Entries: []snapshot.PoolEntry{en}}}, seq: "2"}
			result := planPoolSweepObserve(c, &en)
			if result.effect == nil || result.effect.outcome != mutation.OutcomeCompleted {
				t.Fatalf("observe %+v", result)
			}
			state, e := snapshot.DecodePools(result.posts["pools.json"])
			if e != nil || len(state.Entries) != 1 {
				t.Fatal(state, e)
			}
			if got := state.Entries[0].Recovery; (got == nil) != tc.cleared {
				t.Fatalf("recovery %+v, cleared want %t", got, tc.cleared)
			}
		})
	}
}
