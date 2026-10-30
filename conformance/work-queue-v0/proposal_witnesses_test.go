//go:build unix

package workqueuev0_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/workqueue"
)

// These are standalone proposal witnesses, with trusted, internally bound
// observations. They do not authenticate a caller-supplied observation or prove
// fresh-process CLI isolation. The command-level witnesses own that boundary.
func TestProposalAtomicMultiresourceWitnesses(t *testing.T) {
	t.Run("WQO-V0-020", func(t *testing.T) {
		for _, test := range []struct {
			name       string
			repository [2]workqueue.Count
			caller     [2]workqueue.Count
			uses       [][2]workqueue.Count
			want       []string
		}{
			{"initial-second-resource-failure", [2]workqueue.Count{1, 1}, [2]workqueue.Count{1, 1}, [][2]workqueue.Count{{1, 2}, {1, 0}}, []string{"CAPACITY_EXHAUSTED", "ELIGIBLE_AT_CHECKPOINT"}},
			{"caller-second-resource-failure", [2]workqueue.Count{1, 2}, [2]workqueue.Count{1, 1}, [][2]workqueue.Count{{1, 2}, {1, 0}}, []string{"CAPACITY_EXHAUSTED", "ELIGIBLE_AT_CHECKPOINT"}},
			{"repository-second-resource-failure", [2]workqueue.Count{1, 1}, [2]workqueue.Count{1, 2}, [][2]workqueue.Count{{1, 2}, {1, 0}}, []string{"CAPACITY_EXHAUSTED", "ELIGIBLE_AT_CHECKPOINT"}},
			{"successful-two-resource-consumption", [2]workqueue.Count{2, 1}, [2]workqueue.Count{2, 1}, [][2]workqueue.Count{{1, 1}, {1, 1}, {1, 0}}, []string{"ELIGIBLE_AT_CHECKPOINT", "CAPACITY_EXHAUSTED", "ELIGIBLE_AT_CHECKPOINT"}},
			{"caller-failure-after-earlier-consumption", [2]workqueue.Count{1, 2}, [2]workqueue.Count{1, 1}, [][2]workqueue.Count{{0, 1}, {1, 1}, {1, 0}}, []string{"ELIGIBLE_AT_CHECKPOINT", "CAPACITY_EXHAUSTED", "ELIGIBLE_AT_CHECKPOINT"}},
			{"repository-failure-after-earlier-consumption", [2]workqueue.Count{1, 1}, [2]workqueue.Count{1, 2}, [][2]workqueue.Count{{0, 1}, {1, 1}, {1, 0}}, []string{"ELIGIBLE_AT_CHECKPOINT", "CAPACITY_EXHAUSTED", "ELIGIBLE_AT_CHECKPOINT"}},
		} {
			t.Run(test.name, func(t *testing.T) {
				tickets := make([]workqueue.TicketSummary, len(test.uses))
				for i, uses := range test.uses {
					tickets[i] = ticket(fmt.Sprintf("atomic-%d", i), workqueue.Rank(i+1))
					tickets[i].CapacityUses = witnessUses(uses)
					workqueue.RefreshTicket(&tickets[i])
				}
				value := snapshot(tickets...)
				value.CapacityClasses = witnessClasses(test.repository)
				workqueue.RefreshSnapshot(value)
				capacity := envelope(value)
				capacity.Available = witnessClasses(test.caller)
				workqueue.RefreshEnvelope(capacity)
				before, capacityBefore := value.Canonical(), capacity.Canonical()
				assertWitnessValidated(t, value)
				proposal := proposeWitness(t, value, capacity, workqueue.CollisionClosure{Complete: true})
				assertWitnessReasons(t, proposal, tickets, test.want)
				if !bytes.Equal(before, value.Canonical()) || !bytes.Equal(capacityBefore, capacity.Canonical()) {
					t.Fatal("proposal mutated repository facts or caller capacity")
				}
			})
		}
	})
}

func TestProposalEveryNonReadyLifecycleWitness(t *testing.T) {
	t.Run("WQO-V0-023", func(t *testing.T) {
		states := []string{"BLOCKED", "HELD", "ACTIVE", "REVIEW", "REPAIR", "DONE", "RETIRED", "UNKNOWN"}
		tickets := make([]workqueue.TicketSummary, 0, len(states)+1)
		for i, state := range states {
			value := ticket(fmt.Sprintf("lifecycle-%d", i), workqueue.Rank(i+1))
			value.Lifecycle = state
			workqueue.RefreshTicket(&value)
			tickets = append(tickets, value)
		}
		ready := ticket("ready", 9)
		tickets = append(tickets, ready)
		value := snapshot(tickets...)
		before := value.Canonical()
		assertWitnessValidated(t, value)
		proposal := proposeWitness(t, value, envelope(value), workqueue.CollisionClosure{Complete: true})
		assertWitnessReasons(t, proposal, []workqueue.TicketSummary{ready}, []string{"ELIGIBLE_AT_CHECKPOINT"})
		if proposal.MutationAuthority || !bytes.Equal(before, value.Canonical()) {
			t.Fatal("non-READY lifecycle facts changed or acquired mutation authority")
		}
	})
}

func TestProposalLeaseCapacityAlreadyNetWitness(t *testing.T) {
	t.Run("WQO-V0-019", func(t *testing.T) {
		ready, active := ticket("ready", 1), ticket("active", 2)
		ready.CapacityUses = witnessUses([2]workqueue.Count{1, 1})
		active.Lifecycle = "ACTIVE"
		workqueue.RefreshTicket(&ready)
		workqueue.RefreshTicket(&active)
		value := snapshot(ready, active)
		value.CapacityClasses = witnessClasses([2]workqueue.Count{1, 1})
		lease := workqueue.LeaseSummary{BlocksSelection: true, CapacityUses: witnessUses([2]workqueue.Count{1, 1}), CollisionGroupIDs: []string{}, HolderID: "holder:corvint:worklist:agent", LeaseID: "lease:corvint:worklist:active", Lifecycle: "ACTIVE", QueueAuthorityID: value.QueueAuthorityID, RepositoryAuthorityID: value.RepositoryAuthorityID, TicketID: active.TicketID, TicketVersionID: active.TicketVersionID}
		workqueue.RefreshLease(&lease)
		value.Leases = []workqueue.LeaseSummary{lease}
		workqueue.RefreshSnapshot(value)
		before := value.Canonical()
		assertWitnessValidated(t, value)
		proposal := proposeWitness(t, value, envelope(value), workqueue.CollisionClosure{Complete: true})
		assertWitnessReasons(t, proposal, []workqueue.TicketSummary{ready}, []string{"ELIGIBLE_AT_CHECKPOINT"})
		if !bytes.Equal(before, value.Canonical()) {
			t.Fatal("proposal altered explanatory lease capacity")
		}
	})
}

func TestProposalFirstEqualMaximumWitness(t *testing.T) {
	t.Run("WQO-V0-044", func(t *testing.T) {
		// Triangle 1,2,3 plus edge 3--4. Rank 3 has the most clashes;
		// including it gives {3}. Excluding it branches on rank 1 (tied
		// with 2), first reaching {1,4}. {2,4} is an equal maximum and
		// cannot strictly improve the incumbent. Thus {1,4}, not merely
		// cardinality two, is the independent expected result.
		tickets := []workqueue.TicketSummary{ticket("tie-1", 1), ticket("tie-2", 2), ticket("tie-3", 3), ticket("tie-4", 4)}
		groups := []workqueue.CollisionGroup{}
		for i, edge := range [][2]int{{0, 1}, {0, 2}, {1, 2}, {2, 3}} {
			groups = append(groups, workqueue.CollisionGroup{ID: fmt.Sprintf("collision:corvint:worklist:tie-%d", i), MemberTicketIDs: []string{tickets[edge[0]].TicketID, tickets[edge[1]].TicketID}, Source: "ADAPTER"})
		}
		value := snapshot(tickets...)
		assertWitnessValidated(t, value)
		proposal := proposeWitness(t, value, envelope(value), workqueue.CollisionClosure{Complete: true, Groups: groups})
		assertWitnessReasons(t, proposal, tickets, []string{"ELIGIBLE_AT_CHECKPOINT", "SELECTED_COLLISION", "SELECTED_COLLISION", "ELIGIBLE_AT_CHECKPOINT"})
		if proposal.WaveOptimality != "MAXIMUM" {
			t.Fatalf("optimality = %s", proposal.WaveOptimality)
		}
		for _, test := range []struct {
			entry  int
			groups []string
		}{{1, []string{groups[0].ID}}, {2, []string{groups[1].ID, groups[3].ID}}} {
			if !reflect.DeepEqual(proposal.Entries[test.entry].CollisionGroupIDs, test.groups) {
				t.Fatalf("entry %d collision witnesses = %v, want %v", test.entry, proposal.Entries[test.entry].CollisionGroupIDs, test.groups)
			}
		}
	})
}

func TestProposalLibraryContextIsolationWitness(t *testing.T) {
	t.Run("WQO-V0-016", func(t *testing.T) {
		const secretPath = "private/high-only-sentinel.go"
		visible := ticket("visible", 1, "public.go")
		secret := ticket("high-only-sentinel", 2, "public.go", secretPath)
		secret.TicketContentSHA256 = digest("high-only-content-sentinel")
		workqueue.RefreshTicket(&secret)
		secretPeer := ticket("high-only-peer", 3, secretPath)
		low, high := snapshot(visible), snapshot(visible, secret, secretPeer)
		low.AccessContextID = "access:corvint:low"
		high.AccessContextID = "access:corvint:high"
		workqueue.RefreshSnapshot(low)
		workqueue.RefreshSnapshot(high)
		// Sharing this immutable index is supported; there is no durable queue
		// cache. Only paths reached from this snapshot may enter its closure.
		index := &contextindex.Index{CommitRevision: zeroOID, Tracked: map[string]struct{}{"public.go": {}, secretPath: {}}, Imports: map[string]map[string]struct{}{}}
		run := func(value *workqueue.Snapshot) []byte {
			assertWitnessValidated(t, value)
			closure := workqueue.DeriveCollisions(value, workqueue.IndexCollisionSource(index))
			if !closure.Complete {
				t.Fatal("isolation fixture has incomplete closure")
			}
			proposal := proposeWitness(t, value, envelope(value), closure)
			if proposal.State != "ELIGIBLE_AT" {
				t.Fatalf("populated isolation fixture is not eligible: %#v", proposal)
			}
			return proposal.Canonical()
		}
		baseline := run(low)
		for _, order := range []struct {
			name   string
			values []*workqueue.Snapshot
		}{{"high-then-low", []*workqueue.Snapshot{high, low}}, {"low-then-high", []*workqueue.Snapshot{low, high}}} {
			t.Run(order.name, func(t *testing.T) {
				for _, value := range order.values {
					output := run(value)
					if value == high {
						if !bytes.Contains(output, []byte(secretPath)) || !bytes.Contains(output, []byte(secret.TicketID)) || !bytes.Contains(output, []byte(secret.TicketVersionID)) || !bytes.Contains(output, []byte("SELECTED_COLLISION")) {
							t.Fatal("high fixture did not exercise private identity/collision witnesses")
						}
						continue
					}
					if !bytes.Equal(output, baseline) {
						t.Fatal("low proposal differs from low-only baseline")
					}
					for _, sentinel := range []string{secretPath, secret.TicketID, secret.TicketVersionID, secretPeer.TicketID} {
						if bytes.Contains(output, []byte(sentinel)) {
							t.Fatalf("low proposal leaked %q", sentinel)
						}
					}
				}
			})
		}
	})
}

func TestProposalContextContractionBindingWitness(t *testing.T) {
	t.Run("WQO-V0-040", func(t *testing.T) {
		for _, dimension := range []string{"access", "scope", "scope-complete"} {
			t.Run(dimension, func(t *testing.T) {
				original := snapshot(ticket("bound", 1), ticket("contracted-away", 2))
				policy := &workqueue.Policy{AccessContextID: original.AccessContextID, AdapterPath: "script/fixture-adapter", AdapterProfile: "repository-work-queue-adapter/0", DetailLimit: "512", MappingVersion: "v1", Operations: workqueue.PolicyOperations{Details: []string{"details"}, Snapshot: []string{"snapshot"}, Verify: []string{"verify"}}, Profile: workqueue.PolicyProfile, QueueAuthorityID: original.QueueAuthorityID, RepositoryAuthorityID: original.RepositoryAuthorityID, ScopeID: original.Scope.ID}
				policy.RefreshIdentity()
				if _, err := workqueue.ParsePolicy(policy.Canonical()); err != nil {
					t.Fatalf("invalid policy fixture: %v", err)
				}
				original.PolicyID = policy.ID
				workqueue.RefreshSnapshot(original)
				oldSource := workqueue.QueueSourceIdentity(policy, original)
				checkpoint := &workqueue.CheckpointDocument{Checkpoint: original.Checkpoint, PolicyID: policy.ID, RepositorySource: original.RepositorySource, SnapshotID: original.ID}
				workqueue.RefreshCheckpoint(checkpoint)
				before := checkpoint.Canonical()
				if err := workqueue.ValidateCheckpoint(checkpoint, policy, original, original.RepositorySource); err != nil {
					t.Fatalf("original binding rejected: %v", err)
				}
				changed := *original
				switch dimension {
				case "access":
					policy.AccessContextID, changed.AccessContextID = "access:corvint:contracted", "access:corvint:contracted"
				case "scope":
					changed.Tickets = changed.Tickets[:1]
					changed.Scope.TicketCount = 1
				case "scope-complete":
					changed.Scope.Complete = false
				}
				policy.RefreshIdentity()
				changed.PolicyID = policy.ID
				workqueue.RefreshSnapshot(&changed)
				if changed.ID == original.ID || workqueue.QueueSourceIdentity(policy, &changed) == oldSource {
					t.Fatal("changed context reused snapshot or queue-source identity")
				}
				if err := workqueue.ValidateCheckpoint(checkpoint, policy, &changed, changed.RepositorySource); err == nil {
					t.Fatal("historical checkpoint accepted for changed context")
				}
				if !bytes.Equal(before, checkpoint.Canonical()) {
					t.Fatal("historical checkpoint was rewritten")
				}
				if dimension == "scope-complete" {
					validation := workqueue.ValidateSnapshot(&changed)
					if validation.State != workqueue.StatePartial {
						t.Fatalf("incomplete scope state = %s", validation.State)
					}
					changed.ObservationState, changed.ObservationUnknowns = validation.State, validation.Unknowns
					proposal := proposeWitness(t, &changed, envelope(&changed), workqueue.CollisionClosure{Complete: true})
					if proposal.State != "EMPTY" || len(proposal.Entries) != 0 {
						t.Fatal("incomplete scope permitted proposal entries")
					}
					return
				}
				assertWitnessValidated(t, &changed)
			})
		}
	})
}

func witnessClasses(units [2]workqueue.Count) []workqueue.CapacityClass {
	return []workqueue.CapacityClass{{ID: "capacity:corvint:worklist:a", AvailableUnits: units[0]}, {ID: "capacity:corvint:worklist:b", AvailableUnits: units[1]}}
}

func witnessUses(units [2]workqueue.Count) []workqueue.CapacityUse {
	uses := []workqueue.CapacityUse{}
	for i, class := range witnessClasses(units) {
		if units[i] != 0 {
			uses = append(uses, workqueue.CapacityUse{ClassID: class.ID, Units: units[i]})
		}
	}
	return uses
}

func assertWitnessValidated(t *testing.T, value *workqueue.Snapshot) {
	t.Helper()
	if validation := workqueue.ValidateSnapshot(value); validation.State != workqueue.StateValidated {
		t.Fatalf("fixture is not validated: %#v", validation)
	}
}

func proposeWitness(t *testing.T, value *workqueue.Snapshot, capacity *workqueue.CapacityEnvelope, closure workqueue.CollisionClosure) *workqueue.Proposal {
	t.Helper()
	proposal, err := workqueue.ProposeWave(value, capacity, closure, 128)
	if err != nil {
		t.Fatal(err)
	}
	return proposal
}

func assertWitnessReasons(t *testing.T, proposal *workqueue.Proposal, tickets []workqueue.TicketSummary, reasons []string) {
	t.Helper()
	if len(proposal.Entries) != len(tickets) {
		t.Fatalf("entries = %#v; want %d rows", proposal.Entries, len(tickets))
	}
	for i, entry := range proposal.Entries {
		if entry.TicketID != tickets[i].TicketID || entry.TicketVersionID != tickets[i].TicketVersionID || entry.Reason != reasons[i] {
			t.Fatalf("row %d = %#v; want ticket %s reason %s", i, entry, tickets[i].TicketID, reasons[i])
		}
		if reasons[i] == "ELIGIBLE_AT_CHECKPOINT" {
			if entry.State != "SELECTED" || entry.RouteAlternativeID == nil || *entry.RouteAlternativeID != tickets[i].RouteAlternatives[0].ID {
				t.Fatalf("selected row %d has incorrect state/route: %#v", i, entry)
			}
			continue
		}
		if entry.State != "EXCLUDED" || entry.RouteAlternativeID != nil {
			t.Fatalf("excluded row %d has incorrect state/route: %#v", i, entry)
		}
	}
}
