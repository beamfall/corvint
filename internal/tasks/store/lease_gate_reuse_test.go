package store_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// reuseGate is a COMMAND gate that appends one byte to .git/runs-<id> on
// every execution, so a test counts runs while the worktree stays clean,
// then exits with exit. Extra argv entries follow the script as its
// positional parameters.
func reuseGate(id, exit, timeout string, reusable, required bool, extra ...string) wire.Value {
	argv := append([]string{"sh", "-c", "printf x >> .git/runs-" + id + "; printf ok; exit " + exit, id}, extra...)
	return obj("gateId", str(id), "kind", str("COMMAND"), "argv", wire.Strings(argv), "cwd", str("WORKTREE"),
		"env", wire.Strings(nil), "timeoutSeconds", str(timeout), "expected", obj("exitCode", str("0"), "reducer", wire.Null()),
		"evidence", wire.Strings(nil), "inputs", wire.Strings(nil), "sharedResource", wire.Null(), "reusable", wire.Bool(reusable), "required", wire.Bool(required))
}

// runs is how many times gate id has executed in the checkout.
func (s *leaseStore) runs(t *testing.T, id string) int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.root, ".git", "runs-"+id))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(raw)
}

// repolicy replaces the policy's gates, one policy version later, at the
// given minutes after t0.
func (s *leaseStore) repolicy(t *testing.T, version string, minutes int, gates ...wire.Value) {
	t.Helper()
	v := fixture.PolicyValue()
	v.Obj.Set("gates", wire.Array(gates...))
	v.Obj.Set("policyVersion", str(version))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	prev := wire.SizeOf(wire.Size(version).Uint64() - 1)
	report, err := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("policy-"+version, string(prev), wire.EncodeFile(v)), s.at(t, minutes))
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %s: %+v %v", version, report, err)
	}
}

// sharedTree claims one ticket per scope directory and submits one commit
// that changes every directory to each attempt, so all of them stand at
// one candidate tree.
func (s *leaseStore) sharedTree(t *testing.T, dirs ...string) ([]*store.Report, string, string) {
	t.Helper()
	claims := []*store.Report{}
	paths := []string{}
	for _, d := range dirs {
		claims = append(claims, s.claim(t, "claim-"+d, s.ticket(t, d), 0, d+"/"))
		paths = append(paths, d+"/a.go")
	}
	commit, tree := s.commit(t, paths...)
	for i, c := range claims {
		if r := s.lease(t, "submit-"+dirs[i], submitOf(c, tree), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("submit %s: %+v", dirs[i], r)
		}
	}
	return claims, commit, tree
}

// resultOf is the digest of the attempt's result for gate.
func (s *leaseStore) resultOf(t *testing.T, attemptID, gate string) string {
	t.Helper()
	for _, d := range s.attempt(t, attemptID).GateResults {
		if g, err := snapshot.DecodeGateResult(s.evidence(t, d)); err == nil && g.GateID == gate {
			return d
		}
	}
	t.Fatalf("attempt %s has no %s result", attemptID, gate)
	return ""
}

// TestCALV0240_ReusablePassSatisfiesAnotherAttemptAtTheTree: a reusable
// gate's PASS for one attempt satisfies the same gate for another attempt
// of another ticket at the same candidate tree without executing it; the
// reuse is a GATE_RESULT receipt whose result names its source, complete
// accepts it, a retry replays it, and both the journal audit and the
// receipt binding fold accept the history.
func TestCALV0240_ReusablePassSatisfiesAnotherAttemptAtTheTree(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t, reuseGate("verify", "0", "30", true, true))
	claims, commit, _ := s.sharedTree(t, "src", "lib")
	a, b := claims[0], claims[1]
	s.passes(t, "gate-a", gateOf(a, "verify"), 2)
	source := s.resultOf(t, a.AttemptID, "verify")
	report, err := s.gate(t, "gate-b", gateOf(b, "verify"), 3)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.Unretryable {
		t.Fatalf("reuse: %+v %v", report, err)
	}
	if s.runs(t, "verify") != 1 {
		t.Fatalf("the reused gate executed: %d runs", s.runs(t, "verify"))
	}
	if r := report.GateReusedFrom; r == nil || string(r.Result) != source || r.AttemptID != a.AttemptID {
		t.Fatalf("report names %+v, want %s of %s", r, source, a.AttemptID)
	}
	ga, gb := s.results(t, a.AttemptID)["verify"], s.results(t, b.AttemptID)["verify"]
	if gb == nil || gb.ReusedFrom == nil || string(*gb.ReusedFrom) != source || gb.AttemptID != b.AttemptID || gb.Generation != b.Generation || gb.State != "PASSED" || gb.StartedAt != ga.StartedAt || gb.Evidence[0] != ga.Evidence[0] || gb.InputsSha256 != ga.InputsSha256 {
		t.Fatalf("reused result: %+v from %+v", gb, ga)
	}
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "receipts", report.Receipt))
	if err != nil {
		t.Fatal(err)
	}
	if rc, err := snapshot.DecodeReceipt(raw); err != nil || rc.Kind != "GATE_RESULT" {
		t.Fatalf("receipt: %+v %v", rc, err)
	}
	if again, err := s.gate(t, "gate-b", gateOf(b, "verify"), 4); err != nil || again.Kind != "Replay" || s.runs(t, "verify") != 1 || again.GateReusedFrom == nil || *again.GateReusedFrom != *report.GateReusedFrom {
		t.Fatalf("replay: %+v %v", again, err)
	}
	for i, c := range []*store.Report{b, a} {
		done := s.lease(t, "complete-"+c.AttemptID, completeOf(c, commit), 5+i, nil)
		if done.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("complete %s: %+v", c.AttemptID, done)
		}
		if i == 0 {
			m, err := snapshot.DecodeManifest(s.evidence(t, string(*s.attempt(t, b.AttemptID).ManifestSha256)))
			if err != nil || len(m.GateResults) != 1 || m.GateResults[0] != s.attempt(t, b.AttemptID).GateResults[0] {
				t.Fatalf("manifest: %+v %v", m, err)
			}
		}
		report = done
	}
	auditOK(t, s.repo)
	if err := store.FoldReceiptBindings(s.repo, report.Outcome.ReceiptSeq.Uint64(), nil); err != nil {
		t.Fatalf("binding fold: %v", err)
	}
}

// TestCALV0243_ReuseRenamedAfterReturningToTheTree: an attempt that reused
// a result, executed the gate at another tree, then returned to the first
// tree reuses the identical record again; that receipt does not post the
// already retained record, and the receipt audit still accepts the history.
func TestCALV0243_ReuseRenamedAfterReturningToTheTree(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t, reuseGate("verify", "0", "30", true, true))
	claims, _, tree := s.sharedTree(t, "src", "lib")
	a, b := claims[0], claims[1]
	s.passes(t, "gate-a", gateOf(a, "verify"), 2)
	s.passes(t, "gate-b-1", gateOf(b, "verify"), 3)
	reused := s.resultOf(t, b.AttemptID, "verify")
	_, other := s.commit(t, "lib/b.go")
	if r := s.lease(t, "submit-b-2", submitOf(b, other), 4, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("submit at another tree: %+v", r)
	}
	s.passes(t, "gate-b-2", gateOf(b, "verify"), 5)
	gitRun(t, s.root, "rm", "-q", "lib/b.go")
	gitRun(t, s.root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "back")
	if back := gitOut(t, s.root, "rev-parse", "HEAD^{tree}"); back != tree {
		t.Fatalf("tree %s, want %s", back, tree)
	}
	if r := s.lease(t, "submit-b-3", submitOf(b, tree), 6, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("submit back: %+v", r)
	}
	report, err := s.gate(t, "gate-b-3", gateOf(b, "verify"), 7)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.GateReusedFrom == nil {
		t.Fatalf("reuse again: %+v %v", report, err)
	}
	if s.runs(t, "verify") != 2 || s.resultOf(t, b.AttemptID, "verify") != reused {
		t.Fatalf("runs %d, result %s, want 2 and %s", s.runs(t, "verify"), s.resultOf(t, b.AttemptID, "verify"), reused)
	}
	auditOK(t, s.repo)
	if err := store.FoldReceiptBindings(s.repo, report.Outcome.ReceiptSeq.Uint64(), nil); err != nil {
		t.Fatalf("binding fold: %v", err)
	}
}

// TestCALV0243_RedoBindsAPendingReuse: a reuse receipt whose head was not
// yet advanced is redone through the gate reuse binding, and the same
// pending receipt without the reused result's evidence post is refused.
func TestCALV0243_RedoBindsAPendingReuse(t *testing.T) {
	t.Parallel()
	for _, forged := range []bool{false, true} {
		s := newLeaseStore(t, reuseGate("verify", "0", "30", true, true))
		claims, _, _ := s.sharedTree(t, "src", "lib")
		s.passes(t, "gate-a", gateOf(claims[0], "verify"), 2)
		head := filepath.Join(s.repo.StateDir, "head.json")
		headBefore := mustRead(t, head)
		report, err := s.gate(t, "gate-b", gateOf(claims[1], "verify"), 3)
		if err != nil || report.GateReusedFrom == nil {
			t.Fatalf("reuse: %+v %v", report, err)
		}
		reused := s.resultOf(t, claims[1].AttemptID, "verify")
		fixture.Write(t, head, headBefore)
		if forged {
			forgeReceipt(t, s.repo, report.Receipt, func(path string, rec wire.Value) (wire.Value, bool) {
				return rec, path != "evidence/"+reused
			})
		}
		next, err := s.gate(t, "gate-a-again", gateOf(claims[0], "verify"), 4)
		if forged {
			if wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), "gate reuse binding") {
				t.Fatalf("forged pending reuse = %+v, %v; want JOURNAL_FORKED from the gate reuse binding", next, err)
			}
			if !bytes.Equal(mustRead(t, head), headBefore) {
				t.Fatal("refused redo moved head")
			}
			continue
		}
		if err != nil || !next.Redone || next.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("redo = %+v, %v", next, err)
		}
	}
}

// TestCALV0241_GateRunExecutesWithoutAQualifyingSource: another tree, a
// gate not declared reusable, a FAILED source, a changed definition and
// different substituted inputs each leave the gate to execute for the new
// attempt, which records an unreused result.
func TestCALV0241_GateRunExecutesWithoutAQualifyingSource(t *testing.T) {
	t.Parallel()
	executes := func(t *testing.T, s *leaseStore, b *store.Report, gate string, minutes int) {
		t.Helper()
		before := s.runs(t, gate)
		report, err := s.gate(t, "gate-b-"+gate, gateOf(b, gate), minutes)
		if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.GateReusedFrom != nil {
			t.Fatalf("gate %s: %+v %v", gate, report, err)
		}
		if s.runs(t, gate) != before+1 || s.results(t, b.AttemptID)[gate].ReusedFrom != nil {
			t.Fatalf("gate %s was not executed: %d runs", gate, s.runs(t, gate))
		}
	}
	t.Run("another tree", func(t *testing.T) {
		t.Parallel()
		s := newLeaseStore(t, reuseGate("verify", "0", "30", true, true))
		ta, tb := s.ticket(t, "a"), s.ticket(t, "b")
		a, _ := s.submitted(t, ta, "src", 0)
		s.passes(t, "gate-a", gateOf(a, "verify"), 2)
		b, _ := s.submitted(t, tb, "lib", 3)
		executes(t, s, b, "verify", 5)
	})
	t.Run("not reusable", func(t *testing.T) {
		t.Parallel()
		s := newLeaseStore(t, reuseGate("solo", "0", "30", false, true))
		claims, _, _ := s.sharedTree(t, "src", "lib")
		s.passes(t, "gate-a", gateOf(claims[0], "solo"), 2)
		executes(t, s, claims[1], "solo", 3)
	})
	t.Run("failed source", func(t *testing.T) {
		t.Parallel()
		s := newLeaseStore(t, reuseGate("fails", "3", "30", true, false))
		claims, _, _ := s.sharedTree(t, "src", "lib")
		s.passes(t, "gate-a", gateOf(claims[0], "fails"), 2)
		executes(t, s, claims[1], "fails", 3)
		if g := s.results(t, claims[1].AttemptID)["fails"]; g.State != "FAILED" {
			t.Fatalf("fails: %+v", g)
		}
	})
	t.Run("changed definition", func(t *testing.T) {
		t.Parallel()
		s := newLeaseStore(t, reuseGate("verify", "0", "30", true, true))
		claims, _, _ := s.sharedTree(t, "src", "lib")
		s.passes(t, "gate-a", gateOf(claims[0], "verify"), 2)
		s.repolicy(t, "3", 3, reuseGate("verify", "0", "31", true, true))
		executes(t, s, claims[1], "verify", 4)
	})
	t.Run("different inputs", func(t *testing.T) {
		t.Parallel()
		s := newLeaseStore(t, reuseGate("based", "0", "30", true, true, store.GateArgBaseCommit))
		a := s.claim(t, "claim-a", s.ticket(t, "a"), 0, "src/")
		s.commit(t, "docs/n.go")
		b := s.claim(t, "claim-b", s.ticket(t, "b"), 0, "lib/")
		_, tree := s.commit(t, "src/a.go", "lib/a.go")
		for _, c := range []*store.Report{a, b} {
			if r := s.lease(t, "submit-"+c.AttemptID, submitOf(c, tree), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("submit: %+v", r)
			}
		}
		if s.attempt(t, a.AttemptID).BaseCommit == s.attempt(t, b.AttemptID).BaseCommit {
			t.Fatal("the attempts share a base commit")
		}
		s.passes(t, "gate-a", gateOf(a, "based"), 2)
		executes(t, s, b, "based", 3)
	})
}

// TestCALV0242_ModelRefusesAnUnqualifiedReuse hands the model reuses that
// GateRun would never choose: a gate not declared reusable (UNSUPPORTED), a
// FAILED source (GATE_FAILED), a record that is not its source rebound, a
// source absent from evidence/ and a reused source (MALFORMED), a source
// its attempt no longer names and a changed definition (GATE_STALE),
// another tree (STALE_TREE), and a stale generation (FENCED). Only the
// fenced refusal is recorded.
func TestCALV0242_ModelRefusesAnUnqualifiedReuse(t *testing.T) {
	t.Parallel()
	gates := []wire.Value{reuseGate("verify", "0", "30", true, true), reuseGate("solo", "0", "30", false, false), reuseGate("fails", "3", "30", true, false)}
	s := newLeaseStore(t, gates...)
	claims, _, _ := s.sharedTree(t, "src", "lib", "etc")
	a, b, c := claims[0], claims[1], claims[2]
	for i, g := range []string{"verify", "solo", "fails"} {
		s.passes(t, "gate-a-"+g, gateOf(a, g), 2+i)
	}
	rebind := func(from string, to *store.Report) ([]byte, []byte) {
		t.Helper()
		source := s.evidence(t, from)
		src, err := snapshot.DecodeGateResult(source)
		if err != nil {
			t.Fatal(err)
		}
		g := transaction.ReusedGateResult(src, wire.Digest(from), s.attempt(t, to.AttemptID))
		record, err := g.Encode()
		if err != nil {
			t.Fatal(err)
		}
		return record, source
	}
	forge := func(requestID string, to *store.Report, record, source []byte, minutes int) *store.Report {
		t.Helper()
		l := gateOf(to, "verify")
		if g, err := snapshot.DecodeGateResult(record); err == nil {
			l.Gate = g.GateID
		}
		choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: requestID, Root: s.root, Lease: l}
		report, err := store.GateReuseForTest(context.Background(), s.repo, operator(), choice, record, source, s.at(t, minutes))
		if err != nil {
			t.Fatalf("%s: %v", requestID, err)
		}
		return report
	}
	refuses := func(requestID string, to *store.Report, record, source []byte, minutes int, outcome, code string) {
		t.Helper()
		before := storeDigest(t, s.repo)
		refusedWith(t, forge(requestID, to, record, source, minutes), outcome, code)
		if storeDigest(t, s.repo) != before {
			t.Fatalf("%s: a refused reuse wrote", requestID)
		}
	}
	verify := s.resultOf(t, a.AttemptID, "verify")
	record, source := rebind(s.resultOf(t, a.AttemptID, "solo"), b)
	refuses("forge-solo", b, record, source, 5, mutation.OutcomeBlocked, wire.CodeUnsupported)
	record, source = rebind(s.resultOf(t, a.AttemptID, "fails"), b)
	refuses("forge-fails", b, record, source, 5, mutation.OutcomeBlocked, wire.CodeGateFailed)
	record, source = rebind(verify, b)
	g, err := snapshot.DecodeGateResult(record)
	if err != nil {
		t.Fatal(err)
	}
	g.EndedAt = s.at(t, 9)
	tampered, err := g.Encode()
	if err != nil {
		t.Fatal(err)
	}
	refuses("forge-tampered", b, tampered, source, 5, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	src, err := snapshot.DecodeGateResult(source)
	if err != nil {
		t.Fatal(err)
	}
	src.EndedAt = s.at(t, 9)
	unstored, err := src.Encode()
	if err != nil {
		t.Fatal(err)
	}
	moved := transaction.ReusedGateResult(src, wire.Sum(unstored), s.attempt(t, b.AttemptID))
	movedRecord, err := moved.Encode()
	if err != nil {
		t.Fatal(err)
	}
	refuses("forge-unstored", b, movedRecord, unstored, 5, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	stale := gateOf(b, "verify")
	stale.Generation = wire.SizeOf(b.Generation.Uint64() + 1)
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "forge-stale", Root: s.root, Lease: stale}
	fenced, err := store.GateReuseForTest(context.Background(), s.repo, operator(), choice, record, source, s.at(t, 5))
	if err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fenced, mutation.OutcomeRevisionConflict, wire.CodeFenced)
	if len(s.attempt(t, b.AttemptID).GateResults) != 0 {
		t.Fatal("a fenced reuse recorded a result")
	}
	// B reuses A's PASS as GateRun chooses; C may not chain from B's reuse.
	if r, err := s.gate(t, "gate-b", gateOf(b, "verify"), 6); err != nil || r.GateReusedFrom == nil {
		t.Fatalf("reuse: %+v %v", r, err)
	}
	record, source = rebind(s.resultOf(t, b.AttemptID, "verify"), c)
	refuses("forge-chain", c, record, source, 7, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	// A reruns the gate: its earlier result is no longer named by A.
	s.passes(t, "gate-a-again", gateOf(a, "verify"), 8)
	if s.resultOf(t, a.AttemptID, "verify") == verify {
		t.Fatal("the rerun did not replace the result")
	}
	record, source = rebind(verify, c)
	refuses("forge-unnamed", c, record, source, 9, mutation.OutcomeBlocked, wire.CodeGateStale)
	// C moves to another tree: A's PASS is at the old one.
	_, tree := s.commit(t, "etc/b.go")
	if r := s.lease(t, "submit-c-again", submitOf(c, tree), 10, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("resubmit: %+v", r)
	}
	current := s.resultOf(t, a.AttemptID, "verify")
	record, source = rebind(current, c)
	refuses("forge-tree", c, record, source, 11, mutation.OutcomeBlocked, wire.CodeStaleTree)
	// The definition changes: B's tree still matches, A's PASS ran the old one.
	s.repolicy(t, "3", 12, reuseGate("verify", "0", "31", true, true), gates[1], gates[2])
	record, source = rebind(current, b)
	refuses("forge-definition", b, record, source, 13, mutation.OutcomeBlocked, wire.CodeGateStale)
	auditOK(t, s.repo)
}

// TestCALV0243_ReceiptAuditBindsEveryReuse folds a journal holding one
// reuse through the gate reuse binding: unaltered it passes; a reuse under
// another receipt kind, without its own evidence post, with its source
// absent, or under a policy that does not declare the gate reusable is
// JOURNAL_FORKED.
func TestCALV0243_ReceiptAuditBindsEveryReuse(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t, reuseGate("verify", "0", "30", true, true))
	claims, _, _ := s.sharedTree(t, "src", "lib")
	s.passes(t, "gate-a", gateOf(claims[0], "verify"), 2)
	report, err := s.gate(t, "gate-b", gateOf(claims[1], "verify"), 3)
	if err != nil || report.GateReusedFrom == nil {
		t.Fatalf("reuse: %+v %v", report, err)
	}
	last := report.Outcome.ReceiptSeq.Uint64()
	reused := s.resultOf(t, claims[1].AttemptID, "verify")
	receipts := func() []*snapshot.Receipt {
		out := []*snapshot.Receipt{}
		for seq := uint64(1); seq <= last; seq++ {
			name, err := snapshot.ReceiptName(seq)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "receipts", name))
			if err != nil {
				t.Fatal(err)
			}
			rc, err := snapshot.DecodeReceipt(raw)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, rc)
		}
		return out
	}
	blob := store.ExternalReviewBlob(s.repo)
	fold := func(rcs []*snapshot.Receipt, blob transaction.ExternalReviewBlob) error {
		audit := &transaction.GateReuseReceiptAudit{}
		for _, rc := range rcs {
			if err := audit.Step(rc, "", blob); err != nil {
				return err
			}
		}
		return nil
	}
	if err := fold(receipts(), blob); err != nil {
		t.Fatalf("unaltered: %v", err)
	}
	forked := func(name string, rcs []*snapshot.Receipt, blob transaction.ExternalReviewBlob) {
		t.Helper()
		if err := fold(rcs, blob); wire.CodeOf(err) != wire.CodeJournalForked {
			t.Fatalf("%s: %v", name, err)
		}
	}
	rcs := receipts()
	rcs[last-1].Kind = "TRANSITION"
	forked("kind", rcs, blob)
	rcs = receipts()
	posts := []snapshot.PostEntry{}
	for _, p := range rcs[last-1].Post {
		if p.Path != "evidence/"+reused {
			posts = append(posts, p)
		}
	}
	rcs[last-1].Post = posts
	forked("unposted", rcs, blob)
	source := wire.Digest(s.resultOf(t, claims[0].AttemptID, "verify"))
	forked("source absent", receipts(), func(d wire.Digest) ([]byte, bool) {
		if d == source {
			return nil, false
		}
		return blob(d)
	})
	repolicied := func(gate wire.Value) []*snapshot.Receipt {
		t.Helper()
		rcs := receipts()
		policy := fixture.PolicyValue()
		policy.Obj.Set("gates", wire.Array(gate))
		raw := wire.EncodeFile(policy)
		sum := wire.Sum(raw)
		replaced := false
		for _, rc := range rcs {
			for i, p := range rc.Post {
				if p.Path == "intent/policy.json" && p.Sha256 != nil {
					v, err := wire.Parse(raw)
					if err != nil {
						t.Fatal(err)
					}
					rc.Post[i] = snapshot.PostEntry{Path: p.Path, Sha256: &sum, Record: &v}
					replaced = true
				}
			}
		}
		if !replaced {
			t.Fatal("no policy post")
		}
		return rcs
	}
	forked("not reusable", repolicied(reuseGate("verify", "0", "30", false, true)), blob)
	forked("changed definition", repolicied(reuseGate("verify", "0", "31", true, true)), blob)
	// An undecodable result named in the reuse's place is skipped unless it
	// still claims a reusedFrom.
	undecodable := func(reusedFrom wire.Value) ([]*snapshot.Receipt, transaction.ExternalReviewBlob) {
		t.Helper()
		g, err := wire.Parse(s.evidence(t, reused))
		if err != nil {
			t.Fatal(err)
		}
		g.Obj.Set("state", wire.Array())
		g.Obj.Set("reusedFrom", reusedFrom)
		bad := wire.EncodeFile(g)
		if _, err := snapshot.DecodeGateResult(bad); err == nil {
			t.Fatal("the altered result still decodes")
		}
		badSum := wire.Sum(bad)
		rcs := receipts()
		renamed := false
		for i, p := range rcs[last-1].Post {
			if p.Path != "attempts/"+claims[1].AttemptID+".json" || p.Sha256 == nil {
				continue
			}
			raw, ok := []byte(nil), true
			if p.Record != nil {
				raw = wire.EncodeFile(*p.Record)
			} else {
				raw, ok = blob(*p.BlobSha256)
			}
			v, err := wire.Parse(raw)
			if !ok || err != nil {
				t.Fatalf("attempt post: %v", err)
			}
			v.Obj.Set("gateResults", wire.Strings([]string{string(badSum)}))
			sum := wire.Sum(wire.EncodeFile(v))
			rcs[last-1].Post[i] = snapshot.PostEntry{Path: p.Path, Sha256: &sum, Record: &v}
			renamed = true
		}
		if !renamed {
			t.Fatal("no attempt post")
		}
		return rcs, func(d wire.Digest) ([]byte, bool) {
			if d == badSum {
				return bad, true
			}
			return blob(d)
		}
	}
	if err := fold(undecodable(wire.Null())); err != nil {
		t.Fatalf("an undecodable unreused result: %v", err)
	}
	forkedRcs, forkedBlob := undecodable(str(string(source)))
	forked("undecodable reuse", forkedRcs, forkedBlob)
}
