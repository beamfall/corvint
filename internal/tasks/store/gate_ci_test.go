package store_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const ciRepository = "beamfall/corvint"

// newCIStore is a lease store whose policy declares the required,
// non-reusable command gate verify, the optional gate lint, and a ciEvidence
// entry admitting CI runs of ciRepository for verify only when the go-test
// and vet checks passed (TCD-V0-154).
func newCIStore(t *testing.T) *leaseStore {
	t.Helper()
	s := newLeaseStore(t, reuseGate("verify", "0", "30", false, true), reuseGate("lint", "0", "30", false, false))
	store.SetDescentRootForTest(s.repo, s.root)
	v := fixture.PolicyValue()
	v.Obj.Set("gates", wire.Array(reuseGate("verify", "0", "30", false, true), reuseGate("lint", "0", "30", false, false)))
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	v.Obj.Set("ciEvidence", obj("gates", wire.Array(obj("gateId", str("verify"), "repository", str(ciRepository), "requiredChecks", wire.Strings([]string{"go-test", "vet"})))))
	report, err := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("policy-ci", "2", wire.EncodeFile(v)), now(t))
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", report, err)
	}
	return s
}

// ciRun is a canonical CI run record of repo at head.
func ciRun(t *testing.T, repo, head, conclusion string, checks ...string) []byte {
	t.Helper()
	raw, err := (&snapshot.CIRun{Repository: repo, RunID: "18822330011", HeadSha: head, Conclusion: conclusion, PassedChecks: checks, CompletedAt: "2026-10-21T12:00:00Z"}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// ciGate records gate l of an attempt from the CI run record raw.
func (s *leaseStore) ciGate(t *testing.T, requestID string, l transaction.LeaseRequest, raw []byte, minutes int) (*store.Report, error) {
	t.Helper()
	at, err := time.Parse("2006-01-02T15:04:05Z", string(s.at(t, minutes)))
	if err != nil {
		t.Fatal(err)
	}
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: requestID, Root: s.root, Lease: l}
	return store.GateRunCI(context.Background(), s.repo, operator(), choice, raw, func() time.Time { return at })
}

// TestTCDV0155_GreenCIRunClosesMergedTicketsWithoutLocalGates: three
// tickets are delivered one after another on the protected intent branch and
// one green CI run at its tip satisfies every attempt's required gate: no
// gate executes, complete accepts each attempt, the journal audit and the
// receipt binding fold accept the history, and once the branch is rewound
// behind the CI run's head commit the fold refuses it (TCD-V0-157).
func TestTCDV0155_GreenCIRunClosesMergedTicketsWithoutLocalGates(t *testing.T) {
	t.Parallel()
	s := newCIStore(t)
	type delivered struct {
		claim  *store.Report
		commit string
	}
	var all []delivered
	dirs := []string{"src", "lib", "cmd"}
	tickets := map[string]string{}
	for _, dir := range dirs {
		tickets[dir] = s.ticket(t, dir)
	}
	for i, dir := range dirs {
		c := s.claim(t, "claim-"+dir, tickets[dir], 2*i, dir+"/")
		commit, tree := s.commit(t, dir+"/a.go")
		if r := s.lease(t, "submit-"+dir, submitOf(c, tree), 2*i+1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("submit %d: %+v", i, r)
		}
		all = append(all, delivered{c, commit})
	}
	first := all[0].commit
	head := all[len(all)-1].commit
	raw := ciRun(t, ciRepository, head, "success", "go-test", "vet", "windows-vet")
	for i, d := range all {
		report, err := s.ciGate(t, fmt.Sprintf("ci-%d", i), gateOf(d.claim, "verify"), raw, 6)
		if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("CI gate %d: %+v %v", i, report, err)
		}
		if r := report.GateReusedFrom; r == nil || r.CIRunID != "18822330011" || r.Result != wire.Sum(raw) {
			t.Fatalf("report names %+v", r)
		}
		g := s.results(t, d.claim.AttemptID)["verify"]
		if g == nil || g.State != "PASSED" || g.ReusedFrom == nil || *g.ReusedFrom != wire.Sum(raw) || string(s.evidence(t, string(wire.Sum(raw)))) != string(raw) {
			t.Fatalf("CI-backed result: %+v", g)
		}
	}
	if s.runs(t, "verify") != 0 {
		t.Fatalf("a CI-satisfied gate executed: %d runs", s.runs(t, "verify"))
	}
	var last uint64
	for i, d := range all {
		done := s.lease(t, fmt.Sprintf("complete-%d", i), completeOf(d.claim, d.commit), 7+i, nil)
		if done.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("complete %d: %+v", i, done)
		}
		last = done.Outcome.ReceiptSeq.Uint64()
	}
	if s.runs(t, "verify") != 0 {
		t.Fatalf("closing executed the gate: %d runs", s.runs(t, "verify"))
	}
	auditOK(t, s.repo)
	if err := store.FoldReceiptBindings(s.repo, last, nil); err != nil {
		t.Fatalf("binding fold: %v", err)
	}
	gitRun(t, s.root, "checkout", "-q", "--detach", first)
	gitRun(t, s.root, "branch", "-f", "main", first)
	err := store.FoldReceiptBindings(s.repo, last, nil)
	if err == nil || !strings.Contains(err.Error(), "is not a commit carrying tree") {
		t.Fatalf("binding fold after rewind: %v", err)
	}
}

// TestTCDV0156_CIRunRefusals: a CI run that failed, lacks a required check,
// is of another repository, names a gate the policy does not admit, has its
// head commit off the protected branch or not descending from the candidate,
// or names a commit absent locally, never satisfies the gate and writes
// nothing; nothing executes.
func TestTCDV0156_CIRunRefusals(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, gate, outcome, code, detail string
		build                             func(t *testing.T, s *leaseStore, head string) []byte
	}{
		{"failed-run", "verify", mutation.OutcomeBlocked, wire.CodeGateFailed, "concluded failure", func(t *testing.T, s *leaseStore, head string) []byte {
			return ciRun(t, ciRepository, head, "failure", "go-test", "vet")
		}},
		{"missing-check", "verify", mutation.OutcomeBlocked, wire.CodeGateFailed, "required check vet did not pass", func(t *testing.T, s *leaseStore, head string) []byte {
			return ciRun(t, ciRepository, head, "success", "go-test")
		}},
		{"other-repository", "verify", mutation.OutcomeBlocked, wire.CodeGateStale, "not " + ciRepository, func(t *testing.T, s *leaseStore, head string) []byte {
			return ciRun(t, "someone/fork", head, "success", "go-test", "vet")
		}},
		{"gate-not-admitted", "lint", mutation.OutcomeBlocked, wire.CodeUnsupported, "does not admit CI evidence for gate lint", func(t *testing.T, s *leaseStore, head string) []byte {
			return ciRun(t, ciRepository, head, "success", "go-test", "vet")
		}},
		{"off-protected-branch", "verify", mutation.OutcomeBlocked, wire.CodeStaleTree, "is not a commit carrying tree", func(t *testing.T, s *leaseStore, head string) []byte {
			gitRun(t, s.root, "checkout", "-q", "-b", "side", head)
			side, _ := s.commit(t, "doc/side.md")
			gitRun(t, s.root, "checkout", "-q", "main")
			return ciRun(t, ciRepository, side, "success", "go-test", "vet")
		}},
		{"not-a-descendant", "verify", mutation.OutcomeBlocked, wire.CodeStaleTree, "is not an ancestor of the CI run's head commit", func(t *testing.T, s *leaseStore, head string) []byte {
			gitRun(t, s.root, "checkout", "-q", "--detach", head+"^")
			gitRun(t, s.root, "branch", "-f", "main", head+"^")
			other, _ := s.commit(t, "doc/other.md")
			gitRun(t, s.root, "branch", "-f", "main", other)
			gitRun(t, s.root, "checkout", "-q", "main")
			return ciRun(t, ciRepository, other, "success", "go-test", "vet")
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := newCIStore(t)
			claim := s.claim(t, "claim", s.ticket(t, "src"), 0, "src/")
			head, tree := s.commit(t, "src/a.go")
			if r := s.lease(t, "submit", submitOf(claim, tree), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("submit: %+v", r)
			}
			raw := c.build(t, s, head)
			before := storeDigest(t, s.repo)
			report, err := s.ciGate(t, "ci", gateOf(claim, c.gate), raw, 2)
			if err != nil {
				t.Fatal(err)
			}
			refusedWith(t, report, c.outcome, c.code)
			if !strings.Contains(fmt.Sprintf("%+v", report), c.detail) {
				t.Fatalf("refusal does not say %q: %+v", c.detail, report)
			}
			if storeDigest(t, s.repo) != before || s.runs(t, c.gate) != 0 {
				t.Fatal("a refused CI run wrote or executed")
			}
		})
	}
	t.Run("absent-head-commit", func(t *testing.T) {
		t.Parallel()
		s := newCIStore(t)
		claim := s.claim(t, "claim", s.ticket(t, "src"), 0, "src/")
		_, tree := s.commit(t, "src/a.go")
		if r := s.lease(t, "submit", submitOf(claim, tree), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("submit: %+v", r)
		}
		before := storeDigest(t, s.repo)
		_, err := s.ciGate(t, "ci", gateOf(claim, "verify"), ciRun(t, ciRepository, strings.Repeat("ab", 20), "success", "go-test", "vet"), 2)
		if err == nil || !strings.Contains(err.Error(), "not in the local repository") || storeDigest(t, s.repo) != before {
			t.Fatalf("absent head commit: %v", err)
		}
	})
}

// TestTCDV0155_ModelRefusesAForgedCIResult: a CI-backed record that claims
// another tree than the head commit's, handed to the model directly, is
// refused as malformed without writing.
func TestTCDV0155_ModelRefusesAForgedCIResult(t *testing.T) {
	t.Parallel()
	s := newCIStore(t)
	claim := s.claim(t, "claim", s.ticket(t, "src"), 0, "src/")
	_, tree := s.commit(t, "src/a.go")
	if r := s.lease(t, "submit", submitOf(claim, tree), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("submit: %+v", r)
	}
	head, _ := s.commit(t, "lib/a.go")
	raw := ciRun(t, ciRepository, head, "success", "go-test", "vet")
	ci, err := snapshot.DecodeCIRun(raw)
	if err != nil {
		t.Fatal(err)
	}
	rawPolicy, err := os.ReadFile(filepath.Join(s.repo.PrimaryWorktree, intent.Dir, "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	policy, err := intent.DecodePolicy(rawPolicy)
	if err != nil {
		t.Fatal(err)
	}
	forged := transaction.CIGateResult(policy, "verify", ci, raw, s.attempt(t, claim.AttemptID), tree)
	record, err := forged.Encode()
	if err != nil {
		t.Fatal(err)
	}
	before := storeDigest(t, s.repo)
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "forge", Root: s.root, Lease: gateOf(claim, "verify")}
	report, err := store.GateReuseForTest(context.Background(), s.repo, operator(), choice, record, raw, s.at(t, 2))
	if err == nil && report.Outcome.Outcome == mutation.OutcomeCompleted {
		t.Fatalf("forged CI result accepted: %+v", report)
	}
	if storeDigest(t, s.repo) != before {
		t.Fatal("a forged CI result wrote")
	}
}
