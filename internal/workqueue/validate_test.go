package workqueue

import (
	"sort"
	"testing"
)

func TestValidateSnapshotInvariants(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*Snapshot)
		state    string
		unknowns []string
	}{
		{
			name: "ticket count",
			mutate: func(snapshot *Snapshot) {
				snapshot.Scope.TicketCount++
				refreshSnapshotID(snapshot)
			},
			state: StateConflicted,
		},
		{
			name: "WQO-V0-010 duplicate stable version and rank",
			mutate: func(snapshot *Snapshot) {
				snapshot.Tickets = append(snapshot.Tickets, snapshot.Tickets[0])
				snapshot.Scope.TicketCount++
				refreshSnapshotID(snapshot)
			},
			state: StateConflicted,
		},
		{
			name: "unknown dependency",
			mutate: func(snapshot *Snapshot) {
				snapshot.Tickets[0].DependencyTicketIDs = []string{"ticket:corvint:worklist:missing"}
				refreshSnapshotID(snapshot)
			},
			state: StateUnknown, unknowns: []string{UnknownReference},
		},
		{
			name: "foreign authority",
			mutate: func(snapshot *Snapshot) {
				snapshot.Tickets[0].TicketID = "ticket:foreign:worklist:one"
				snapshot.Tickets[0].TicketVersionID = testIdentity("ticket-version", "ticket-version/0", ticketVersionBody(snapshot.Tickets[0]))
				refreshSnapshotID(snapshot)
			},
			state: StateUnknown, unknowns: []string{UnknownMultiRepoUnsupported},
		},
		{
			name: "missing atomic repository",
			mutate: func(snapshot *Snapshot) {
				snapshot.Tickets[0].AtomicRepositoryAuthorityIDs = []string{}
				refreshSnapshotID(snapshot)
			},
			state: StateConflicted,
		},
		{
			name: "foreign atomic repository",
			mutate: func(snapshot *Snapshot) {
				snapshot.Tickets[0].AtomicRepositoryAuthorityIDs = []string{testRepository, "repo:foreign"}
				refreshSnapshotID(snapshot)
			},
			state: StateUnknown, unknowns: []string{UnknownMultiRepoUnsupported},
		},
		{
			name: "WQO-V0-010 ticket version",
			mutate: func(snapshot *Snapshot) {
				snapshot.Tickets[0].TicketVersionID = "ticket-version:sha256:" + testZeroDigest
				refreshSnapshotID(snapshot)
			},
			state: StateConflicted,
		},
		{
			name: "snapshot identity",
			mutate: func(snapshot *Snapshot) {
				snapshot.ID = "work-queue-snapshot:sha256:" + testZeroDigest
			},
			state: StateConflicted,
		},
		{
			name: "source identity",
			mutate: func(snapshot *Snapshot) {
				snapshot.RepositorySource.ID = "repository-source:sha256:" + testZeroDigest
				refreshSnapshotID(snapshot)
			},
			state: StateConflicted,
		},
		{
			name: "duplicate capacity class",
			mutate: func(snapshot *Snapshot) {
				class := CapacityClass{ID: "capacity:corvint:worklist:cpu", AvailableUnits: 1}
				snapshot.CapacityClasses = []CapacityClass{class, class}
				refreshSnapshotID(snapshot)
			},
			state: StateConflicted,
		},
		{
			name: "unknown capacity class",
			mutate: func(snapshot *Snapshot) {
				snapshot.Tickets[0].CapacityUses = []CapacityUse{{ClassID: "capacity:corvint:worklist:cpu", Units: 1}}
				refreshSnapshotID(snapshot)
			},
			state: StateConflicted,
		},
		{
			name: "incomplete scope",
			mutate: func(snapshot *Snapshot) {
				snapshot.Scope.Complete = false
				refreshSnapshotID(snapshot)
			},
			state: StatePartial,
		},
		{
			name: "contradictory ready",
			mutate: func(snapshot *Snapshot) {
				snapshot.Tickets[0].SelectionFacts.Holds = "HELD"
				refreshSnapshotID(snapshot)
			},
			state: StateConflicted,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := testSnapshot(testTicket("one", 1))
			test.mutate(snapshot)
			result := ValidateSnapshot(snapshot)
			if result.State != test.state {
				t.Fatalf("state = %s, want %s; unknowns=%v", result.State, test.state, result.Unknowns)
			}
			if !equalStrings(result.Unknowns, test.unknowns) {
				t.Fatalf("unknowns = %v, want %v", result.Unknowns, test.unknowns)
			}
		})
	}
}

func TestValidateSnapshotLeaseIdentityAndReference(t *testing.T) {
	t.Run("WQO-V0-011", func(t *testing.T) {
		ticket := testTicket("one", 1)
		snapshot := testSnapshot(ticket)
		lease := LeaseSummary{
			BlocksSelection: true, CapacityUses: []CapacityUse{}, CollisionGroupIDs: []string{},
			HolderID: "holder:corvint:worklist:h", LeaseID: "lease:corvint:worklist:l", Lifecycle: "ACTIVE",
			QueueAuthorityID: testQueue, RepositoryAuthorityID: testRepository,
			TicketID: ticket.TicketID, TicketVersionID: ticket.TicketVersionID,
		}
		lease.LeaseVersionID = testIdentity("lease-version", "lease-version/0", leaseValue(lease, false))
		snapshot.Leases = []LeaseSummary{lease}
		refreshSnapshotID(snapshot)
		if result := ValidateSnapshot(snapshot); result.State != StateValidated {
			t.Fatalf("valid lease state = %s, unknowns=%v", result.State, result.Unknowns)
		}

		snapshot.Leases[0].LeaseVersionID = "lease-version:sha256:" + testZeroDigest
		refreshSnapshotID(snapshot)
		if result := ValidateSnapshot(snapshot); result.State != StateConflicted {
			t.Fatalf("fabricated lease version state = %s", result.State)
		}
	})
}

func TestValidationPrecedenceAndCanonicalUnknowns(t *testing.T) {
	snapshot := testSnapshot(testTicket("one", 1))
	snapshot.Scope.TicketCount = 2
	snapshot.Tickets[0].DependencyTicketIDs = []string{
		"ticket:corvint:worklist:missing-a",
		"ticket:corvint:worklist:missing-b",
	}
	snapshot.Tickets[0].AtomicRepositoryAuthorityIDs = []string{"repo:foreign"}
	refreshSnapshotID(snapshot)
	result := ValidateSnapshot(snapshot)
	if result.State != StateConflicted {
		t.Fatalf("state = %s, want CONFLICTED", result.State)
	}
	t.Run("WQO-V0-015", func(t *testing.T) {
		want := []string{UnknownMultiRepoUnsupported, UnknownReference}
		if !equalStrings(result.Unknowns, want) {
			t.Fatalf("unknowns = %v, want %v", result.Unknowns, want)
		}
	})
}

// TestUnknownCodesAreExactlyTheSpecSet pins WQO-V0-015: adding, renaming or
// misspelling a code in types.go must fail this test even though every
// individual code is exercised elsewhere by its own name.
func TestUnknownCodesAreExactlyTheSpecSet(t *testing.T) {
	want := []string{
		"ACCESS_INCOMPLETE", "ADAPTER_INVALID", "CHECKPOINT_CHANGED",
		"COLLISION_CLOSURE_INCOMPLETE", "CONTAINMENT_UNQUALIFIED", "DETAIL_MISSING",
		"EXECUTABLE_IDENTITY_UNQUALIFIED", "HOSTILE_INPUT", "INPUT_LIMIT",
		"MULTI_REPO_UNSUPPORTED", "MUTATION_DETECTED", "MUTATION_ENFORCEMENT_UNQUALIFIED",
		"NETWORK_UNOBSERVED", "PROCESS_RESIDUE", "REPOSITORY_DIRTY", "ROUTE_UNKNOWN",
		"SOURCE_UNQUALIFIED", "UNKNOWN_REFERENCE",
	}
	got := make([]string, 0, len(unknownCodeSet))
	for code := range unknownCodeSet {
		got = append(got, code)
	}
	sort.Strings(got)
	if !equalStrings(got, want) {
		t.Fatalf("unknownCodeSet = %v, want exactly %v", got, want)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
