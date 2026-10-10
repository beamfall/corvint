//go:build unix

package store_test

import (
	"bytes"
	"context"
	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// CAL-V0-028, CAL-V0-029, CAL-V0-030 and CAL-V0-034.
func TestPoolAllocationQuarantine(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a", "b", "review"}), "reservedFor", obj("review", str("review")))))
	rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("pools", "2", wire.EncodeFile(v)), now(t))
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	ids := []string{s.ticket(t, "one"), s.ticket(t, "two"), s.ticket(t, "three")}
	c := claimOf(ids[0], "one")
	c.Pool = "db"
	c.Stage = "implement"
	a := s.lease(t, "claim-a", c, 0, nil)
	if a.PoolAllocation == nil {
		t.Fatalf("allocation: %+v", a)
	}
	c.TicketID = ids[1]
	c.Scope = []string{"two"}
	bclaim := s.lease(t, "claim-b", c, 0, nil)
	if bclaim.PoolAllocation == nil || a.PoolAllocation.MemberID == bclaim.PoolAllocation.MemberID {
		t.Fatalf("distinct %+v", bclaim)
	}
	// Occupied membership is immutable; adding an unrelated free member is allowed.
	v.Obj.Set("policyVersion", str("4"))
	pools, _ := v.Obj.Get("pools")
	pools.Arr[0].Obj.Set("members", wire.Strings([]string{"b", "review"}))
	changed, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("remove-occupied", "3", wire.EncodeFile(v)), now(t))
	if e != nil || changed.Outcome.Outcome == mutation.OutcomeCompleted {
		t.Fatalf("occupied removal %+v %v", changed, e)
	}
	pools.Arr[0].Obj.Set("members", wire.Strings([]string{"a", "b", "review"}))
	v.Obj.Set("cemRequired", wire.Bool(true))
	changed, e = store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("unrelated-policy", "3", wire.EncodeFile(v)), now(t))
	if e != nil || changed.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("unrelated policy %+v %v", changed, e)
	}
	s.t0 = now(t)
	c.TicketID = ids[2]
	c.Scope = []string{"three"}
	blocked := s.lease(t, "reserved", c, 0, nil)
	if !blocked.Outcome.HasCode(wire.CodeResourceCollision) {
		t.Fatalf("reserved %+v", blocked)
	}
	rel := s.lease(t, "release-a", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: a.AttemptID, Generation: a.Generation}, 0, nil)
	if rel.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release %+v", rel)
	}
	blocked = s.lease(t, "quarantine", c, 0, nil)
	if !blocked.Outcome.HasCode(wire.CodeResourceCollision) {
		t.Fatalf("quarantine %+v", blocked)
	}
	safe := s.lease(t, "safe", transaction.LeaseRequest{Verb: transaction.LeasePoolSafe, Member: a.PoolAllocation.MemberID, Allocation: string(a.PoolAllocation.AllocationID), Evidence: "operator-reset-record", Reason: "confirmed external reset"}, 0, nil)
	if safe.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("safe %+v", safe)
	}
	reuse := s.lease(t, "reuse", c, 0, nil)
	if reuse.PoolAllocation == nil || reuse.PoolAllocation.MemberID != a.PoolAllocation.MemberID || reuse.PoolAllocation.AllocationID == a.PoolAllocation.AllocationID {
		t.Fatalf("reuse %+v", reuse)
	}
	var exported bytes.Buffer
	result, e := archive.Export(archive.ExportOptions{Repo: s.repo, Stdout: &exported})
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, file := range result.Manifest.Files {
		found = found || file.Path == "pools.json"
	}
	if !found {
		t.Fatal("pool projection omitted from archive")
	}
	if _, e = archive.Verify(bytes.NewReader(exported.Bytes())); e != nil {
		t.Fatal(e)
	}
}

// CAL-V0-029 and CAL-V0-034.
func TestPoolAllocationPrefersMatchingStageReservation(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"integrate", "open", "review"}), "reservedFor", obj("integrate", str("integrate"), "review", str("review")))))
	rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("reserved-priority", "2", wire.EncodeFile(v)), now(t))
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	ids := []string{s.ticket(t, "reserved"), s.ticket(t, "fallback"), s.ticket(t, "blocked")}
	claim := claimOf(ids[0], "reserved")
	claim.Pool, claim.Stage = "db", "review"
	reserved := s.lease(t, "reserved", claim, 0, nil)
	t.Run("CAL-V0-029 matching reservation priority", func(t *testing.T) {
		if reserved.PoolAllocation == nil || reserved.PoolAllocation.MemberID != "review" {
			t.Fatalf("matching reservation not preferred %+v", reserved)
		}
	})
	claim.TicketID, claim.Scope = ids[1], []string{"fallback"}
	fallback := s.lease(t, "fallback", claim, 0, nil)
	if fallback.PoolAllocation == nil || fallback.PoolAllocation.MemberID != "open" {
		t.Fatalf("unreserved fallback not selected %+v", fallback)
	}
	claim.TicketID, claim.Scope, claim.Stage = ids[2], []string{"blocked"}, "implement"
	blocked := s.lease(t, "opposite-reservation", claim, 0, nil)
	if !blocked.Outcome.HasCode(wire.CodeResourceCollision) {
		t.Fatalf("opposite-stage reservation admitted %+v", blocked)
	}
}

// CAL-V0-031 and CAL-V0-032.
func TestPoolHealthSkipsFailedMember(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	command := func(code string) wire.Value {
		return obj("argv", wire.Strings([]string{"/bin/sh", "-c", "exit " + code}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3"))
	}
	v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a", "review"}), "reservedFor", obj("review", str("review")), "memberConfig", obj("a", obj("health", command("0"), "cleanup", command("0")), "review", obj("health", command("1"))))))
	rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("health-policy", "2", wire.EncodeFile(v)), now(t))
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	id := s.ticket(t, "health")
	c := claimOf(id, "one")
	c.Pool, c.Stage = "db", "review"
	a := s.lease(t, "health-claim", c, 0, nil)
	if a.PoolAllocation == nil || a.PoolAllocation.MemberID != "a" {
		t.Fatalf("health claim %+v", a)
	}
	raw, e := os.ReadFile(filepath.Join(s.repo.StateDir, "pools.json"))
	if e != nil {
		t.Fatal(e)
	}
	state, e := snapshot.DecodePools(raw)
	if e != nil {
		t.Fatal(e)
	}
	foundFailedReservation := false
	for _, entry := range state.Entries {
		if entry.MemberID == "review" && entry.State == "QUARANTINED" && entry.ObservationSha256 != nil && strings.HasPrefix(entry.Reason, "EXIT_NONZERO;") {
			foundFailedReservation = true
		}
	}
	if !foundFailedReservation {
		t.Fatalf("failed reserved health probe not retained: %+v", state.Entries)
	}
	replay := s.lease(t, "health-claim", c, 0, nil)
	if replay.Kind != "Replay" || replay.PoolAllocation == nil || replay.PoolAllocation.MemberID != a.PoolAllocation.MemberID || replay.PoolAllocation.AllocationID != a.PoolAllocation.AllocationID {
		t.Fatalf("replay %+v", replay)
	}
}

// CAL-V0-033.
func TestPoolNoHealthConfigReference(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"valid", "missing-revision", "missing-path", "wrong-blob", "symlink"} {
		t.Run(scenario, func(t *testing.T) {
			s := newLeaseStore(t)
			if e := os.WriteFile(filepath.Join(s.root, "config.json"), []byte("{}\n"), 0600); e != nil {
				t.Fatal(e)
			}
			if e := os.Symlink("config.json", filepath.Join(s.root, "link")); e != nil {
				t.Fatal(e)
			}
			gitRun(t, s.root, "add", "config.json", "link")
			gitRun(t, s.root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "config")
			object := func(name string) string {
				c := exec.Command("git", "rev-parse", name)
				c.Dir = s.root
				raw, e := c.Output()
				if e != nil {
					t.Fatal(e)
				}
				return strings.TrimSpace(string(raw))
			}
			rev, blob, path := object("HEAD"), object("HEAD:config.json"), "config.json"
			switch scenario {
			case "missing-revision":
				rev = strings.Repeat("1", 40)
			case "missing-path":
				path = "absent"
			case "wrong-blob":
				blob = strings.Repeat("1", 40)
			case "symlink":
				path = "link"
				blob = object("HEAD:link")
			}
			v := fixture.PolicyValue()
			v.Obj.Set("policyVersion", str("3"))
			budgets, _ := v.Obj.Get("budgets")
			budgets.Obj.Set("requireEnforcedFields", wire.Array())
			v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a"}), "memberConfig", obj("a", obj("configRef", obj("revision", str(rev), "path", str(path), "blob", str(blob)))))))
			rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("config-policy", "2", wire.EncodeFile(v)), now(t))
			if e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("policy %+v %v", rep, e)
			}
			id := s.ticket(t, "config")
			c := claimOf(id, "one")
			c.Pool = "db"
			rep, e = store.Lease(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "config-claim", Root: s.root, Lease: c}, now(t))
			if scenario == "valid" {
				if e != nil || rep.PoolAllocation == nil {
					t.Fatalf("valid %+v %v", rep, e)
				}
			} else if e == nil && rep.PoolAllocation != nil {
				t.Fatalf("invalid config allocated %+v", rep)
			}
		})
	}
}

// CAL-V0-029 and CAL-V0-030.
func TestPoolReplayReturnsOriginalAllocation(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a"}))))
	rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("replay-policy", "2", wire.EncodeFile(v)), now(t))
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	id := s.ticket(t, "replay")
	c := claimOf(id, "one")
	c.Pool = "db"
	a := s.lease(t, "original", c, 0, nil)
	if a.PoolAllocation == nil {
		t.Fatalf("original %+v", a)
	}
	s.lease(t, "release-original", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: a.AttemptID, Generation: a.Generation}, 0, nil)
	safe := transaction.LeaseRequest{Verb: transaction.LeasePoolSafe, Member: "a", Allocation: string(a.PoolAllocation.AllocationID), Evidence: "reset", Reason: "independent reset"}
	s.lease(t, "confirm-original", safe, 0, nil)
	successor := s.lease(t, "successor", c, 0, nil)
	if successor.PoolAllocation == nil || successor.AttemptID != a.AttemptID || successor.Generation == a.Generation {
		t.Fatalf("retry %+v", successor)
	}
	replay := s.lease(t, "original", c, 0, nil)
	if replay.Kind != "Replay" || replay.Generation != a.Generation || replay.PoolAllocation == nil || replay.PoolAllocation.AllocationID != a.PoolAllocation.AllocationID || replay.PoolAllocation.AllocationID == successor.PoolAllocation.AllocationID {
		t.Fatalf("replay borrowed successor %+v", replay)
	}
	stale := s.lease(t, "stale-confirm", safe, 0, nil)
	if !stale.Outcome.HasCode(wire.CodeFenced) {
		t.Fatalf("stale confirmation %+v", stale)
	}
}

// TestCALV0078_PoolCommandReportsExecution: a direct pool health command
// reports Unretryable once the member's program ran, so the CLI marks any coded
// result not retryable; a command refused before running does not.
func TestCALV0078_PoolCommandReportsExecution(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	health := obj("argv", wire.Strings([]string{"/bin/sh", "-c", "exit 0"}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3"))
	v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a"}), "memberConfig", obj("a", obj("health", health)))))
	if rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("execution-policy", "2", wire.EncodeFile(v)), now(t)); e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	choice := func(id, member string) store.LeaseChoice {
		return store.LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: s.root, Lease: transaction.LeaseRequest{Member: member}}
	}
	if rep, e := store.PoolCommand(context.Background(), s.repo, operator(), choice("probe-none", "absent"), "health"); wire.CodeOf(e) != wire.CodeUnsupported || rep == nil || rep.Unretryable {
		t.Fatalf("refused before running: %+v %v", rep, e)
	}
	if rep, e := store.PoolCommand(context.Background(), s.repo, operator(), choice("probe-a", "a"), "health"); e != nil || rep == nil || !rep.Unretryable {
		t.Fatalf("health ran: %+v %v", rep, e)
	}
}

// CAL-V0-078: once health commits its preparation receipt, a later failure is
// not retryable, because a same-request retry replays that receipt and never
// runs the program or records its observation.
func TestCALV0078_PreparedPoolCommandIsNotRetryable(t *testing.T) {
	s := newLeaseStore(t)
	marker := filepath.Join(t.TempDir(), "ran")
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	health := obj("argv", wire.Strings([]string{"/bin/sh", "-c", "printf x >> " + marker}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3"))
	v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a"}), "memberConfig", obj("a", obj("health", health)))))
	if rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("prepared-policy", "2", wire.EncodeFile(v)), now(t)); e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "prep-1", Root: s.root, Lease: transaction.LeaseRequest{Member: "a"}}
	restore := store.SetPoolPreparedFaultForTest(func() error {
		return wire.Errorf(wire.CodeRedoPending, "head", "injected post-preparation read failure")
	})
	rep, e := store.PoolCommand(context.Background(), s.repo, operator(), choice, "health")
	restore()
	if wire.CodeOf(e) != wire.CodeRedoPending || rep == nil || !rep.Unretryable {
		t.Fatalf("committed preparation reported retryable: %+v %v", rep, e)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("program ran before the injected failure: %v", err)
	}
	if entries := psrPools(t, s).Entries; len(entries) != 1 || entries[0].State == "FREE" {
		t.Fatalf("preparation not committed: %+v", entries)
	}
	replay, e := store.PoolCommand(context.Background(), s.repo, operator(), choice, "health")
	if e != nil || replay == nil || replay.Kind != "Replay" {
		t.Fatalf("same-request retry: %+v %v", replay, e)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("same-request retry ran the program: %v", err)
	}
	if entries := psrPools(t, s).Entries; len(entries) != 1 || entries[0].State == "FREE" {
		t.Fatalf("same-request retry finished the preparation: %+v", entries)
	}
}
