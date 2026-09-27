package store_test

import (
	"context"
	"os"
	"os/exec"
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

// leaseStore is an initialized store whose policy requires no enforced
// budget field and admits four live attempts, plus a real Git checkout with
// one commit for claims to resolve their base against. Given gates replace
// the policy's.
type leaseStore struct {
	repo *intent.Repository
	root string
	t0   wire.Timestamp
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func newLeaseStore(t *testing.T, gates ...wire.Value) *leaseStore {
	t.Helper()
	repo, _ := initialized(t)
	v := fixture.PolicyValue()
	if len(gates) > 0 {
		v.Obj.Set("gates", wire.Array(gates...))
	}
	v.Obj.Set("policyVersion", str("2"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	report, err := store.PolicyUpdate(context.Background(), repo, operator(), policyRequest("policy-lease", "1", wire.EncodeFile(v)), now(t))
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", report, err)
	}
	return &leaseStore{repo: repo, root: gitRoot(t), t0: now(t)}
}

// gitRoot is a fresh Git checkout with one empty commit on main.
func gitRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitRun(t, root, "init", "-q", "-b", "main")
	gitRun(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	return root
}

func (s *leaseStore) ticket(t *testing.T, title string) string {
	t.Helper()
	report := mutate(t, s.repo, envelope("create-"+title, "CREATE", "", "", createPayload(title)))
	if report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create %s: %+v", title, report)
	}
	s.t0 = now(t)
	return report.Ticket
}

// at is t0, the time of the last ticket creation, plus the given minutes.
func (s *leaseStore) at(t *testing.T, minutes int) wire.Timestamp {
	t.Helper()
	base, err := time.Parse("2006-01-02T15:04:05Z", string(s.t0))
	if err != nil {
		t.Fatal(err)
	}
	ts, err := wire.ParseTimestamp("at", base.Add(time.Duration(minutes)*time.Minute).Format("2006-01-02T15:04:05Z"))
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func (s *leaseStore) lease(t *testing.T, requestID string, l transaction.LeaseRequest, minutes int, derive store.ScopeDeriver) *store.Report {
	t.Helper()
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: requestID, Root: s.root, Lease: l, Derive: derive}
	report, err := store.Lease(context.Background(), s.repo, operator(), choice, s.at(t, minutes))
	if err != nil {
		t.Fatalf("lease %s: %v", requestID, err)
	}
	return report
}

func claimOf(id string, scope ...string) transaction.LeaseRequest {
	l := transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: id, Holder: "agent-1", LeaseMinutes: "60"}
	if len(scope) > 0 {
		l.Scope = scope
	}
	return l
}

func (s *leaseStore) claim(t *testing.T, requestID, id string, minutes int, scope ...string) *store.Report {
	t.Helper()
	report := s.lease(t, requestID, claimOf(id, scope...), minutes, nil)
	if report.Outcome.Outcome != mutation.OutcomeCompleted || report.AttemptID == "" || report.Generation == "" {
		t.Fatalf("claim %s: %+v", requestID, report)
	}
	return report
}

func (s *leaseStore) attempt(t *testing.T, id string) *snapshot.Attempt {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := snapshot.DecodeAttempt(raw)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func (s *leaseStore) entries(t *testing.T) []snapshot.ReservationEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "reservations.json"))
	if err != nil {
		t.Fatal(err)
	}
	set, err := snapshot.DecodeReservations(raw)
	if err != nil {
		t.Fatal(err)
	}
	return set.Entries
}

func refusedWith(t *testing.T, report *store.Report, outcome, code string) {
	t.Helper()
	if report.Outcome.Outcome != outcome || !has(report.Outcome.Codes, code) {
		t.Fatalf("want %s %s, got %+v", outcome, code, report)
	}
}

func renewOf(a *store.Report) transaction.LeaseRequest {
	return transaction.LeaseRequest{Verb: transaction.LeaseRenew, AttemptID: a.AttemptID, Generation: a.Generation, LeaseMinutes: "60"}
}

func releaseOf(a *store.Report) transaction.LeaseRequest {
	return transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: a.AttemptID, Generation: a.Generation}
}

// TestCALV0007_ClaimAdmitsOneRunningAttempt: a claim writes one RUNNING
// external-agent attempt with a lease and one ACTIVE reservation entry in a
// single ADMIT receipt, and its retry replays the same attempt.
func TestCALV0007_ClaimAdmitsOneRunningAttempt(t *testing.T) {
	s := newLeaseStore(t)
	id := s.ticket(t, "one")
	report := s.claim(t, "claim-1", id, 0, "src/a.go")
	a := s.attempt(t, report.AttemptID)
	if a.Phase != "RUNNING" || a.RuntimeID != snapshot.RuntimeExternalAgent || a.Supervisor != nil || a.Lane != nil || a.Lease == nil || a.Lease.Holder != "agent-1" || a.Lease.ExpiresAt != s.at(t, 60) || a.Generation != report.Generation {
		t.Fatalf("attempt: %+v", a)
	}
	if a.Scope.Source != "REQUESTED" || len(a.Scope.Resources) != 1 || a.Scope.Resources[0].Key != "src/a.go" {
		t.Fatalf("scope: %+v", a.Scope)
	}
	entries := s.entries(t)
	if len(entries) != 1 || entries[0].AttemptID != report.AttemptID || entries[0].State != "ACTIVE" {
		t.Fatalf("entries: %+v", entries)
	}
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "receipts", report.Receipt))
	if err != nil {
		t.Fatal(err)
	}
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil || rc.Kind != "ADMIT" || rc.AttemptID == nil || *rc.AttemptID != report.AttemptID {
		t.Fatalf("receipt: %+v %v", rc, err)
	}
	again := s.lease(t, "claim-1", claimOf(id, "src/a.go"), 0, nil)
	if again.Kind != "Replay" || again.AttemptID != report.AttemptID || again.Generation != report.Generation {
		t.Fatalf("replay: %+v", again)
	}
	auditOK(t, s.repo)
}

// TestCALV0007_ClaimRefusesBudgetUnknown: the fixture policy requires
// enforced budget fields, which an external agent never reports.
func TestCALV0007_ClaimRefusesBudgetUnknown(t *testing.T) {
	repo, _ := initialized(t)
	id := mutate(t, repo, envelope("create-b", "CREATE", "", "", createPayload("b"))).Ticket
	before := storeDigest(t, repo)
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "claim-b", Root: gitRoot(t), Lease: claimOf(id, "src/a.go")}
	report, err := store.Lease(context.Background(), repo, operator(), choice, now(t))
	if err != nil {
		t.Fatal(err)
	}
	refusedWith(t, report, mutation.OutcomeBlocked, wire.CodeBudgetUnknown)
	if storeDigest(t, repo) != before {
		t.Fatal("refused claim wrote")
	}
}

// TestCALV0023_CollidingClaimsAdmitOne: the second claim over the same
// path refuses RESOURCE_COLLISION naming the first attempt and writes nothing.
func TestCALV0023_CollidingClaimsAdmitOne(t *testing.T) {
	s := newLeaseStore(t)
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	first := s.claim(t, "claim-1", one, 0, "src/")
	before := storeDigest(t, s.repo)
	second := s.lease(t, "claim-2", claimOf(two, "src/b.go"), 0, nil)
	refusedWith(t, second, mutation.OutcomeBlocked, wire.CodeResourceCollision)
	if !strings.Contains(second.Detail, first.AttemptID) {
		t.Fatalf("collision does not name %s: %q", first.AttemptID, second.Detail)
	}
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused claim wrote")
	}
}

// TestCALV0023_DisjointPathScopesAreBothAdmitted.
func TestCALV0023_DisjointPathScopesAreBothAdmitted(t *testing.T) {
	s := newLeaseStore(t)
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	s.claim(t, "claim-1", one, 0, "src/a")
	s.claim(t, "claim-2", two, 0, "src/b")
	if len(s.entries(t)) != 2 {
		t.Fatal("both claims should hold an entry")
	}
	auditOK(t, s.repo)
}

// TestCALV0021_DeclaredNonPathResourcesJoinTheScope: two claims on disjoint
// paths still collide when both tickets declare the same database.
func TestCALV0021_DeclaredNonPathResourcesJoinTheScope(t *testing.T) {
	s := newLeaseStore(t)
	ids := []string{}
	for _, title := range []string{"one", "two"} {
		payload := createPayload(title)
		payload.Obj.Set("effects", obj("coverage", str("QUALIFIED"), "externalUnbounded", wire.Bool(false), "resources", wire.Array(obj("class", str("DATABASE"), "key", str("dev"))), "touchPaths", wire.Strings(nil)))
		report := mutate(t, s.repo, envelope("create-"+title, "CREATE", "", "", payload))
		if report.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("create %s: %+v", title, report)
		}
		ids = append(ids, report.Ticket)
	}
	s.t0 = now(t)
	first := s.claim(t, "claim-1", ids[0], 0, "src/a")
	if a := s.attempt(t, first.AttemptID); len(a.Scope.Resources) != 2 {
		t.Fatalf("scope: %+v", a.Scope)
	}
	refusedWith(t, s.lease(t, "claim-2", claimOf(ids[1], "src/b"), 0, nil), mutation.OutcomeBlocked, wire.CodeResourceCollision)
}

// TestCALV0021_WholeRepositoryBlocksEverything: a ticket that declares no
// paths, with no --scope and an abstaining deriver, holds WHOLE_REPOSITORY,
// which collides with any other claim.
func TestCALV0021_WholeRepositoryBlocksEverything(t *testing.T) {
	s := newLeaseStore(t)
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	whole := s.claim(t, "claim-1", one, 0)
	if a := s.attempt(t, whole.AttemptID); a.Scope.Source != "WHOLE_REPOSITORY" || a.Scope.Resources[0].Class != "WHOLE_REPOSITORY" {
		t.Fatalf("scope: %+v", a.Scope)
	}
	refusedWith(t, s.lease(t, "claim-2", claimOf(two, "docs/x.md"), 0, nil), mutation.OutcomeBlocked, wire.CodeResourceCollision)
}

// TestCALV0022_DerivedScopeWhenTheTicketDeclaresNone: the injected deriver
// supplies the scope and its input digest; an invalid result abstains.
func TestCALV0022_DerivedScopeWhenTheTicketDeclaresNone(t *testing.T) {
	s := newLeaseStore(t)
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	digest := string(wire.Sum([]byte("index")))
	derive := func(_ context.Context, root, tree, title, _ string) ([]string, string, bool) {
		if root != s.root || len(tree) != 40 || title != "one" {
			return []string{"../escape"}, digest, true
		}
		return []string{"src/z.go", "src/a.go", "src/a.go"}, digest, true
	}
	report := s.lease(t, "claim-1", claimOf(one), 0, derive)
	a := s.attempt(t, report.AttemptID)
	if a.Scope.Source != "DERIVED" || len(a.Scope.Resources) != 2 || a.Scope.DerivationSha256 == nil || string(*a.Scope.DerivationSha256) != digest {
		t.Fatalf("derived scope: %+v", a.Scope)
	}
	abstained := s.lease(t, "claim-2", claimOf(two), 0, derive)
	refusedWith(t, abstained, mutation.OutcomeBlocked, wire.CodeResourceCollision)
}

// TestCALV0011_ExpiredLeaseIsReapedByACollidingClaim: the expired attempt
// is reaped in its own receipt, then the claim is admitted.
func TestCALV0011_ExpiredLeaseIsReapedByACollidingClaim(t *testing.T) {
	s := newLeaseStore(t)
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	l := claimOf(one, "src/")
	l.LeaseMinutes = "5"
	old := s.lease(t, "claim-1", l, 0, nil)
	next := s.lease(t, "claim-2", claimOf(two, "src/a.go"), 10, nil)
	if next.Outcome.Outcome != mutation.OutcomeCompleted || len(next.Reaped) != 1 || next.Reaped[0].AttemptID != old.AttemptID {
		t.Fatalf("claim after expiry: %+v", next)
	}
	a := s.attempt(t, old.AttemptID)
	if a.Phase != "FAILED" || a.Cause == nil || *a.Cause != snapshot.CauseLeaseExpired || a.Quiescence != "FENCED" {
		t.Fatalf("reaped attempt: %+v", a)
	}
	if entries := s.entries(t); len(entries) != 1 || entries[0].AttemptID != next.AttemptID {
		t.Fatalf("entries: %+v", entries)
	}
	auditOK(t, s.repo)
}

// TestCALV0011_ReapAndRelease: reap without an attempt reaps every expired
// lease; release cancels and frees the entry.
func TestCALV0011_ReapAndRelease(t *testing.T) {
	s := newLeaseStore(t)
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	l := claimOf(one, "src/a")
	l.LeaseMinutes = "5"
	expired := s.lease(t, "claim-1", l, 0, nil)
	live := s.claim(t, "claim-2", two, 0, "src/b")
	reap := s.lease(t, "reap-all", transaction.LeaseRequest{Verb: transaction.LeaseReap}, 30, nil)
	if len(reap.Reaped) != 1 || reap.Reaped[0].AttemptID != expired.AttemptID {
		t.Fatalf("reap: %+v", reap)
	}
	release := s.lease(t, "release-2", releaseOf(live), 31, nil)
	if release.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release: %+v", release)
	}
	if a := s.attempt(t, live.AttemptID); a.Phase != "CANCELLED" || a.Quiescence != "FENCED" {
		t.Fatalf("released attempt: %+v", a)
	}
	if len(s.entries(t)) != 0 {
		t.Fatal("entries remain")
	}
	auditOK(t, s.repo)
}

// TestCALV0009_StaleGenerationIsFencedAndRecorded.
func TestCALV0009_StaleGenerationIsFencedAndRecorded(t *testing.T) {
	s := newLeaseStore(t)
	claim := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "src")
	stale := renewOf(claim)
	stale.Generation = wire.SizeOf(claim.Generation.Uint64() + 1)
	report := s.lease(t, "renew-stale", stale, 1, nil)
	refusedWith(t, report, mutation.OutcomeRevisionConflict, wire.CodeFenced)
	if report.Receipt == "" {
		t.Fatal("fenced refusal was not recorded")
	}
	released := s.lease(t, "release-1", releaseOf(claim), 2, nil)
	if released.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release: %+v", released)
	}
	refusedWith(t, s.lease(t, "renew-terminal", renewOf(claim), 3, nil), mutation.OutcomeRevisionConflict, wire.CodeFenced)
	auditOK(t, s.repo)
}

// TestCALV0010_RenewExtendsAndIsFencedAfterExpiry.
func TestCALV0010_RenewExtendsAndIsFencedAfterExpiry(t *testing.T) {
	s := newLeaseStore(t)
	claim := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "src")
	if r := s.lease(t, "renew-1", renewOf(claim), 30, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("renew: %+v", r)
	}
	if a := s.attempt(t, claim.AttemptID); a.Lease.ExpiresAt != s.at(t, 90) {
		t.Fatalf("lease: %+v", a.Lease)
	}
	refusedWith(t, s.lease(t, "renew-2", renewOf(claim), 91, nil), mutation.OutcomeRevisionConflict, wire.CodeFenced)
}

// TestCALV0012_LeaseBoundsAndBackwardClock.
func TestCALV0012_LeaseBoundsAndBackwardClock(t *testing.T) {
	s := newLeaseStore(t)
	id := s.ticket(t, "one")
	for _, minutes := range []wire.Size{"4", "1441"} {
		l := claimOf(id, "src")
		l.LeaseMinutes = minutes
		choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "claim-" + string(minutes), Root: s.root, Lease: l}
		report, err := store.Lease(context.Background(), s.repo, operator(), choice, s.at(t, 0))
		if wire.CodeOf(err) != wire.CodeMalformed && report.Outcome.Outcome == mutation.OutcomeCompleted {
			t.Fatalf("%s minutes admitted: %+v %v", minutes, report, err)
		}
	}
	claim := s.claim(t, "claim-1", id, 10, "src")
	if back := s.lease(t, "renew-back", renewOf(claim), 5, nil); back.Outcome.Outcome != mutation.OutcomeStorageFailed || back.Receipt != "" {
		t.Fatalf("backward clock: %+v", back)
	}
}

// TestCALV0013_RetryAsNextGenerationUpToThree: a cancelled ticket is
// claimable again as the same attempt's next generation three times, then
// refuses RETRY_EXHAUSTED.
func TestCALV0013_RetryAsNextGenerationUpToThree(t *testing.T) {
	s := newLeaseStore(t)
	id := s.ticket(t, "one")
	first := s.claim(t, "claim-0", id, 0, "src")
	prev := first
	for i := 1; i <= transaction.MaxRetries; i++ {
		s.lease(t, "release-"+string(wire.SizeOf(uint64(i))), releaseOf(prev), i, nil)
		next := s.claim(t, "claim-"+string(wire.SizeOf(uint64(i))), id, i, "src")
		a := s.attempt(t, next.AttemptID)
		if next.AttemptID != first.AttemptID || next.Generation.Uint64() <= prev.Generation.Uint64() || a.RetryCount != wire.CountOf(int64(i)) || len(a.PriorGenerations) != i {
			t.Fatalf("retry %d: %+v %+v", i, next, a)
		}
		prev = next
	}
	s.lease(t, "release-last", releaseOf(prev), 10, nil)
	refusedWith(t, s.lease(t, "claim-last", claimOf(id, "src"), 10, nil), mutation.OutcomeBlocked, wire.CodeRetryExhausted)
	auditOK(t, s.repo)
}

// TestCALV0025_WidenAddsPathsAndRefusesCollision.
func TestCALV0025_WidenAddsPathsAndRefusesCollision(t *testing.T) {
	s := newLeaseStore(t)
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	a := s.claim(t, "claim-1", one, 0, "src/a")
	s.claim(t, "claim-2", two, 0, "src/b/")
	widen := transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: a.AttemptID, Generation: a.Generation, Scope: []string{"docs"}}
	if r := s.lease(t, "widen-1", widen, 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("widen: %+v", r)
	}
	if got := s.attempt(t, a.AttemptID); len(got.Scope.Resources) != 2 || got.Scope.Source != "REQUESTED" {
		t.Fatalf("widened scope: %+v", got.Scope)
	}
	before := storeDigest(t, s.repo)
	widen.Scope = []string{"src/b/c.go"}
	refusedWith(t, s.lease(t, "widen-2", widen, 2, nil), mutation.OutcomeBlocked, wire.CodeResourceCollision)
	widen.Scope, widen.WholeRepository = nil, true
	refusedWith(t, s.lease(t, "widen-3", widen, 2, nil), mutation.OutcomeBlocked, wire.CodeResourceCollision)
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused widen wrote")
	}
	auditOK(t, s.repo)
}

// TestCALV0012_BackwardClockRefusesEveryWriter: a ticket mutation recorded
// earlier than the head receipt refuses STORAGE_FAILED without a receipt, so
// a lease a later write has outlived cannot be renewed.
func TestCALV0012_BackwardClockRefusesEveryWriter(t *testing.T) {
	s := newLeaseStore(t)
	claim := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "src")
	later, err := store.Mutate(context.Background(), s.repo, operator(), envelope("create-later", "CREATE", "", "", createPayload("later")), s.at(t, 70))
	if err != nil || later.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("later write: %+v %v", later, err)
	}
	back, err := store.Mutate(context.Background(), s.repo, operator(), envelope("create-back", "CREATE", "", "", createPayload("back")), s.at(t, 1))
	if err != nil || back.Outcome.Outcome != mutation.OutcomeStorageFailed || back.Receipt != "" {
		t.Fatalf("backward write: %+v %v", back, err)
	}
	if r := s.lease(t, "renew-back", renewOf(claim), 2, nil); r.Outcome.Outcome != mutation.OutcomeStorageFailed || r.Receipt != "" {
		t.Fatalf("backward renew: %+v", r)
	}
	refusedWith(t, s.lease(t, "renew-1", renewOf(claim), 71, nil), mutation.OutcomeRevisionConflict, wire.CodeFenced)
	auditOK(t, s.repo)
}

// TestCALV0025_WidenRefusedUnderAdmissionBarrier: pause refuses
// scope-expand (TCP-00 §3.4) without writing, while renew proceeds.
func TestCALV0025_WidenRefusedUnderAdmissionBarrier(t *testing.T) {
	s := newLeaseStore(t)
	a := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "src/a")
	pause, err := store.Barrier(context.Background(), s.repo, operator(), barrierRequest(transaction.Pause, "pause-1"), s.at(t, 1))
	if err != nil || pause.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("pause: %+v %v", pause, err)
	}
	before := storeDigest(t, s.repo)
	widen := transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: a.AttemptID, Generation: a.Generation, Scope: []string{"docs"}}
	refusedWith(t, s.lease(t, "widen-1", widen, 2, nil), mutation.OutcomeBlocked, wire.CodePaused)
	widen.Scope, widen.WholeRepository = nil, true
	refusedWith(t, s.lease(t, "widen-2", widen, 2, nil), mutation.OutcomeBlocked, wire.CodePaused)
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused widen wrote")
	}
	if r := s.lease(t, "renew-1", renewOf(a), 3, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("renew under pause: %+v", r)
	}
	auditOK(t, s.repo)
}

// TestCALV0011_ReleaseAndReapPassAnAllBarrier: an ALL barrier lets release
// and reap through, as it does cancel (TCP-00 §3.4), and still refuses
// claim and renew.
func TestCALV0011_ReleaseAndReapPassAnAllBarrier(t *testing.T) {
	s := newLeaseStore(t)
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	a := s.claim(t, "claim-1", one, 0, "src/a")
	b := s.claim(t, "claim-2", two, 0, "src/b")
	reconciliationBarrier(t, s.repo, "ALL")
	refusedWith(t, s.lease(t, "renew-1", renewOf(a), 1, nil), mutation.OutcomeBlocked, wire.CodePaused)
	if r := s.lease(t, "release-1", releaseOf(a), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release under ALL: %+v", r)
	}
	reap := transaction.LeaseRequest{Verb: transaction.LeaseReap}
	if r := s.lease(t, "reap-1", reap, 61, nil); len(r.Reaped) != 1 || r.Reaped[0].AttemptID != b.AttemptID {
		t.Fatalf("reap under ALL: %+v", r)
	}
	auditOK(t, s.repo)
}

// planned creates a ticket at the given priority that declares the given
// paths, or none.
func (s *leaseStore) planned(t *testing.T, title, priority string, paths ...string) string {
	t.Helper()
	payload := createPayload(title)
	payload.Obj.Set("priority", str(priority))
	payload.Obj.Set("effects", obj("coverage", str("QUALIFIED"), "externalUnbounded", wire.Bool(false), "resources", wire.Array(), "touchPaths", wire.Strings(paths)))
	report := mutate(t, s.repo, envelope("create-"+title, "CREATE", "", "", payload))
	if report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create %s: %+v", title, report)
	}
	s.t0 = now(t)
	return report.Ticket
}

var claimNext = transaction.LeaseRequest{Verb: transaction.LeaseClaimNext, Holder: "agent-1", LeaseMinutes: "60"}

// TestCALV0008_ClaimNextTakesThePlanInPriorityOrder: claim --next claims
// the first SELECTED ticket of the priority-first plan, skips one whose
// declared paths collide with a live reservation, and refuses BLOCKED once
// nothing is SELECTED.
func TestCALV0008_ClaimNextTakesThePlanInPriorityOrder(t *testing.T) {
	s := newLeaseStore(t)
	low := s.planned(t, "low", "P3", "docs/")
	high := s.planned(t, "high", "P1", "src/")
	clash := s.planned(t, "clash", "P2", "src/b.go")
	first := s.lease(t, "next-1", claimNext, 0, nil)
	if first.Outcome.Outcome != mutation.OutcomeCompleted || first.Ticket != high || s.attempt(t, first.AttemptID).TicketID.Raw != high {
		t.Fatalf("first: %+v", first)
	}
	if a := s.attempt(t, first.AttemptID); a.Scope.Source != "DECLARED" {
		t.Fatalf("scope: %+v", a.Scope)
	}
	second := s.lease(t, "next-2", claimNext, 1, nil)
	if second.Outcome.Outcome != mutation.OutcomeCompleted || s.attempt(t, second.AttemptID).TicketID.Raw != low {
		t.Fatalf("second skips %s: %+v", clash, second)
	}
	if replay := s.lease(t, "next-2", claimNext, 1, nil); replay.Outcome.Outcome != mutation.OutcomeCompleted || !replay.Outcome.Replayed || replay.AttemptID != second.AttemptID || replay.Ticket != low {
		t.Fatalf("replay: %+v", replay)
	}
	before := storeDigest(t, s.repo)
	refusedWith(t, s.lease(t, "next-3", claimNext, 2, nil), mutation.OutcomeBlocked, wire.CodeAttemptLive)
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused claim --next wrote")
	}
	auditOK(t, s.repo)
}

// TestCALV0008_ClaimNextRefusesWithoutACandidate: an empty queue refuses
// TICKET_STATE; spent capacity refuses LIMIT_EXCEEDED.
func TestCALV0008_ClaimNextRefusesWithoutACandidate(t *testing.T) {
	s := newLeaseStore(t)
	refusedWith(t, s.lease(t, "next-empty", claimNext, 0, nil), mutation.OutcomeBlocked, wire.CodeTicketState)
	for _, dir := range []string{"a", "b", "c", "d"} {
		s.claim(t, "claim-"+dir, s.planned(t, dir, "P1", dir+"/"), 0)
	}
	s.planned(t, "e", "P0", "e/")
	refusedWith(t, s.lease(t, "next-full", claimNext, 0, nil), mutation.OutcomeBlocked, wire.CodeLimitExceeded)
}

// TestCALV0008_ClaimNextReapsEveryExpiredLeaseFirst: an expired lease
// anywhere is reaped before the plan, so the reaped ticket can be retried.
func TestCALV0008_ClaimNextReapsEveryExpiredLeaseFirst(t *testing.T) {
	s := newLeaseStore(t)
	id := s.planned(t, "one", "P1", "src/")
	l := claimOf(id)
	l.LeaseMinutes = "5"
	old := s.lease(t, "claim-1", l, 0, nil)
	next := s.lease(t, "next-1", claimNext, 10, nil)
	if next.Outcome.Outcome != mutation.OutcomeCompleted || len(next.Reaped) != 1 || next.Reaped[0].AttemptID != old.AttemptID {
		t.Fatalf("claim --next after expiry: %+v", next)
	}
	if a := s.attempt(t, next.AttemptID); a.TicketID.Raw != id || a.RetryCount != wire.CountOf(1) {
		t.Fatalf("retry: %+v", a)
	}
	auditOK(t, s.repo)
}
