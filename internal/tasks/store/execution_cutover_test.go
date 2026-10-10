package store_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// suiteRun is `go test -json` output in which every CAL-V0-019 test passes,
// with extra events appended.
func suiteRun(extra ...string) []byte {
	var b bytes.Buffer
	b.WriteString(`{"Action":"start","Package":"` + transaction.QualificationPackage + `"}` + "\n")
	for _, name := range transaction.QualificationSuite {
		b.WriteString(`{"Action":"run","Package":"` + transaction.QualificationPackage + `","Test":"` + name + `"}` + "\n")
		b.WriteString(`{"Action":"pass","Package":"` + transaction.QualificationPackage + `","Test":"` + name + `","Elapsed":1}` + "\n")
	}
	b.WriteString(`{"Action":"pass","Package":"` + transaction.QualificationPackage + `"}` + "\n")
	for _, line := range extra {
		b.WriteString(line + "\n")
	}
	return b.Bytes()
}

func (s *leaseStore) executionCutover(t *testing.T, actor mutation.Binding, decision string, run []byte) *store.Report {
	t.Helper()
	report, err := store.ExecutionCutover(context.Background(), s.repo, actor, fixture.QueueID, decision, run, s.at(t, 0))
	if err != nil {
		t.Fatalf("execution cutover: %v", err)
	}
	return report
}

// TestCALV0020_ExecutionCutoverAdmitsClaims: an OWNER records the passing
// CAL-V0-019 run under one QUALIFICATION receipt, which sets executionCutover
// with the decision and the run's digest and posts the run as evidence; the
// non-fixture queue then admits a claim, and the same request replays.
func TestCALV0020_ExecutionCutoverAdmitsClaims(t *testing.T) {
	t.Parallel()
	s := leaseStoreOn(t, nonFixture(t, "NATIVE"))
	id := s.planned(t, "one", "P1", "src/")
	refusedWith(t, s.lease(t, "claim-0", claimOf(id), 0, nil), mutation.OutcomeBlocked, wire.CodeCutoverMissing)
	run := suiteRun()
	report := s.executionCutover(t, operator(), "decision-exec", run)
	if report.Outcome.Outcome != mutation.OutcomeCompleted || report.Kind != "Transaction" {
		t.Fatalf("cutover: %+v", report)
	}
	st, err := intent.Load(s.repo.PrimaryWorktree)
	if err != nil {
		t.Fatal(err)
	}
	ec := st.Queue.ExecutionCutover
	if ec == nil || ec.EnabledBy != operator().ID || ec.DecisionRef != "decision-exec" || len(ec.GateEvidence) != 1 || ec.GateEvidence[0] != wire.Sum(run) {
		t.Fatalf("executionCutover: %+v", ec)
	}
	posted, err := os.ReadFile(filepath.Join(s.repo.StateDir, "evidence", string(wire.Sum(run))))
	if err != nil || !bytes.Equal(posted, run) {
		t.Fatalf("run evidence: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "receipts", report.Receipt))
	if err != nil {
		t.Fatal(err)
	}
	if rc, err := snapshot.DecodeReceipt(raw); err != nil || rc.Kind != "QUALIFICATION" {
		t.Fatalf("receipt: %+v %v", rc, err)
	}
	if again := s.executionCutover(t, operator(), "decision-exec", run); again.Kind != "Replay" {
		t.Fatalf("retry: %+v", again)
	}
	if claimed := s.lease(t, "claim-1", claimOf(id), 0, nil); claimed.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("claim after cutover: %+v", claimed)
	}
}

// TestCALV0020_ExecutionCutoverRefusals: the cutover refuses a non-OWNER
// binding, a fixture queue, a queue not yet natively written, a queue already
// cut over, and a run that is not go test -json, lacks a suite test or has a
// failing test; every refusal leaves the store unchanged.
func TestCALV0020_ExecutionCutoverRefusals(t *testing.T) {
	t.Parallel()
	failing := `{"Action":"fail","Package":"` + transaction.QualificationPackage + `","Test":"TestCALV0019_KilledWriterRecovers"}`
	cases := []struct {
		name, writer    string
		actor           mutation.Binding
		run             []byte
		outcome, code   string
		fixture, before bool
	}{
		{name: "operator", writer: "NATIVE", actor: mutation.Binding{ID: "tester", Role: "OPERATOR"}, run: suiteRun(), outcome: mutation.OutcomeUnauthorized},
		{name: "fixture", actor: operator(), run: suiteRun(), outcome: mutation.OutcomeBlocked, fixture: true},
		{name: "roadmap", writer: "ROADMAP", actor: operator(), run: suiteRun(), outcome: mutation.OutcomeBlocked, code: wire.CodeCutoverMissing},
		{name: "recorded", writer: "NATIVE", actor: operator(), run: suiteRun(), outcome: mutation.OutcomeBlocked, before: true},
		{name: "failing", writer: "NATIVE", actor: operator(), run: suiteRun(failing), outcome: mutation.OutcomeBlocked, code: wire.CodeGateFailed},
		{name: "partial", writer: "NATIVE", actor: operator(), run: bytes.Join(bytes.SplitAfter(suiteRun(), []byte("\n"))[:4], nil), outcome: mutation.OutcomeBlocked, code: wire.CodeMissingEvidence},
		{name: "empty", writer: "NATIVE", actor: operator(), run: []byte{}, outcome: mutation.OutcomeBlocked, code: wire.CodeMissingEvidence},
		{name: "text", writer: "NATIVE", actor: operator(), run: []byte("ok  \tstore\t1.0s\n"), outcome: mutation.OutcomeValidationFailed, code: wire.CodeMalformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var s *leaseStore
			if c.fixture {
				s = newLeaseStore(t)
			} else {
				s = leaseStoreOn(t, nonFixture(t, c.writer))
			}
			if c.before {
				s.executionCutover(t, operator(), "decision-first", suiteRun())
			}
			before := storeDigest(t, s.repo)
			report := s.executionCutover(t, c.actor, "decision-exec", c.run)
			if report.Outcome.Outcome != c.outcome || (c.code != "" && !has(report.Outcome.Codes, c.code)) {
				t.Fatalf("want %s %s, got %+v", c.outcome, c.code, report)
			}
			if storeDigest(t, s.repo) != before {
				t.Fatal("refused cutover wrote")
			}
		})
	}
}
