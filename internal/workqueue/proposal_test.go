package workqueue

import (
	"math/bits"
	"math/rand"
	"sort"
	"testing"
)

func TestProposalDecisionRows(t *testing.T) {
	tests := []struct {
		name       string
		prepare    func(*Snapshot, *CapacityEnvelope, *CollisionClosure)
		wantState  string
		wantReason string
		wantGroups []string
	}{
		{
			name: "unknown authority",
			prepare: func(snapshot *Snapshot, _ *CapacityEnvelope, _ *CollisionClosure) {
				snapshot.Tickets[0].Authority = "UNKNOWN"
			},
			wantState: "ABSTAINED", wantReason: reasonUnknownAuthority,
		},
		{
			name: "contradictory ready",
			prepare: func(snapshot *Snapshot, _ *CapacityEnvelope, _ *CollisionClosure) {
				snapshot.Tickets[0].SelectionFacts.Holds = "HELD"
			},
			wantState: "ABSTAINED", wantReason: reasonContradictoryReady,
		},
		{
			name: "WQO-V0-022 route unavailable",
			prepare: func(snapshot *Snapshot, _ *CapacityEnvelope, _ *CollisionClosure) {
				snapshot.Tickets[0].RouteAlternatives[0].Requires = []string{"capability:corvint:worklist:missing"}
			},
			wantState: "EXCLUDED", wantReason: reasonRouteUnavailable,
		},
		{
			name: "capacity exhausted",
			prepare: func(snapshot *Snapshot, envelope *CapacityEnvelope, _ *CollisionClosure) {
				class := CapacityClass{ID: "capacity:corvint:worklist:cpu", AvailableUnits: 0}
				snapshot.CapacityClasses = []CapacityClass{class}
				snapshot.Tickets[0].CapacityUses = []CapacityUse{{ClassID: class.ID, Units: 1}}
				envelope.Available = []CapacityClass{class}
				refreshEnvelopeID(envelope)
			},
			wantState: "EXCLUDED", wantReason: reasonCapacityExhausted,
		},
		{
			name: "active collision",
			prepare: func(snapshot *Snapshot, _ *CapacityEnvelope, closure *CollisionClosure) {
				group := "collision:corvint:worklist:active"
				snapshot.Tickets[0].CollisionGroupIDs = []string{group}
				snapshot.Leases = []LeaseSummary{{
					BlocksSelection: true, CollisionGroupIDs: []string{group},
					HolderID: "holder:corvint:worklist:h", LeaseID: "lease:corvint:worklist:l",
					QueueAuthorityID: testQueue, RepositoryAuthorityID: testRepository,
					TicketID: snapshot.Tickets[0].TicketID,
				}}
				closure.Groups = []CollisionGroup{{ID: group, MemberTicketIDs: []string{snapshot.Tickets[0].TicketID}, Source: "ADAPTER"}}
			},
			wantState: "EXCLUDED", wantReason: reasonActiveCollision,
			wantGroups: []string{"collision:corvint:worklist:active"},
		},
		{
			name:      "selected",
			prepare:   func(*Snapshot, *CapacityEnvelope, *CollisionClosure) {},
			wantState: "SELECTED", wantReason: reasonEligibleAtCheckpoint,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := testSnapshot(testTicket("one", 1))
			envelope := testEnvelope(snapshot)
			closure := CollisionClosure{Complete: true, State: StateValidated, TicketGroupIDs: map[string][]string{}}
			test.prepare(snapshot, envelope, &closure)
			proposal, err := ProposeWave(snapshot, envelope, closure, 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(proposal.Entries) != 1 {
				t.Fatalf("entries = %v", proposal.Entries)
			}
			entry := proposal.Entries[0]
			if entry.State != test.wantState || entry.Reason != test.wantReason || !equalStrings(entry.CollisionGroupIDs, test.wantGroups) {
				t.Fatalf("entry = %#v", entry)
			}
		})
	}
}

func TestProposalSelectedCollisionAndWaveLimitRows(t *testing.T) {
	first := testTicket("one", 1)
	second := testTicket("two", 2)
	group := "collision:corvint:worklist:shared"
	first.CollisionGroupIDs = []string{group}
	second.CollisionGroupIDs = []string{group}
	snapshot := testSnapshot(first, second)
	envelope := testEnvelope(snapshot)
	closure := CollisionClosure{
		Complete: true,
		Groups:   []CollisionGroup{{ID: group, MemberTicketIDs: []string{first.TicketID, second.TicketID}, Source: "ADAPTER"}},
	}
	proposal, err := ProposeWave(snapshot, envelope, closure, 2)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Entries[0].Reason != reasonEligibleAtCheckpoint || proposal.Entries[1].Reason != reasonSelectedCollision {
		t.Fatalf("entries = %#v", proposal.Entries)
	}
	if !equalStrings(proposal.Entries[1].CollisionGroupIDs, []string{group}) {
		t.Fatalf("selected collision groups = %v", proposal.Entries[1].CollisionGroupIDs)
	}

	first.CollisionGroupIDs = []string{}
	second.CollisionGroupIDs = []string{}
	snapshot = testSnapshot(first, second)
	proposal, err = ProposeWave(snapshot, testEnvelope(snapshot), CollisionClosure{Complete: true}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Entries[1].Reason != reasonWaveLimit {
		t.Fatalf("second reason = %s, want %s", proposal.Entries[1].Reason, reasonWaveLimit)
	}
}

func TestProposalCapacityExhaustsEitherPoolAtomically(t *testing.T) {
	for _, limits := range []struct {
		repository Count
		caller     Count
	}{{2, 1}, {1, 2}} {
		first := testTicket("one", 1)
		second := testTicket("two", 2)
		classID := "capacity:corvint:worklist:cpu"
		first.CapacityUses = []CapacityUse{{ClassID: classID, Units: 1}}
		second.CapacityUses = []CapacityUse{{ClassID: classID, Units: 1}}
		snapshot := testSnapshot(first, second)
		snapshot.CapacityClasses = []CapacityClass{{ID: classID, AvailableUnits: limits.repository}}
		envelope := testEnvelope(snapshot, CapacityClass{ID: classID, AvailableUnits: limits.caller})
		proposal, err := ProposeWave(snapshot, envelope, CollisionClosure{Complete: true}, 2)
		if err != nil {
			t.Fatal(err)
		}
		if proposal.Entries[0].Reason != reasonEligibleAtCheckpoint || proposal.Entries[1].Reason != reasonCapacityExhausted {
			t.Fatalf("limits=%v entries=%#v", limits, proposal.Entries)
		}
	}
}

func TestProposalMaximumTieBreakAndBruteForceOracle(t *testing.T) {
	tickets := make([]TicketSummary, 10)
	for index := range tickets {
		tickets[index] = testTicket(string(rune('a'+index)), Rank(index+1))
	}
	edges := [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 4}, {0, 4}, {5, 6}, {6, 7}, {7, 8}, {8, 9}}
	groups := make([]CollisionGroup, 0, len(edges))
	for index, edge := range edges {
		id := "collision:corvint:worklist:g" + string(rune('a'+index))
		groups = append(groups, CollisionGroup{ID: id, MemberTicketIDs: []string{tickets[edge[0]].TicketID, tickets[edge[1]].TicketID}, Source: "ADAPTER"})
	}
	snapshot := testSnapshot(tickets...)
	proposal, err := ProposeWave(snapshot, testEnvelope(snapshot), CollisionClosure{Complete: true, Groups: groups}, 128)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.WaveOptimality != "MAXIMUM" {
		t.Fatalf("optimality = %s", proposal.WaveOptimality)
	}
	got := selectedRanksFromProposal(proposal, snapshot)
	want := bruteForceRanks(tickets, groups)
	if len(got) != len(want) {
		t.Fatalf("selected %d ranks %v, oracle selects %d %v", len(got), got, len(want), want)
	}
	if !equalRanks(got, selectedRanksFromProposal(mustPropose(t, snapshot, groups), snapshot)) {
		t.Fatalf("selection is not deterministic: %v", got)
	}
}

func mustPropose(t *testing.T, snapshot *Snapshot, groups []CollisionGroup) *Proposal {
	t.Helper()
	proposal, err := ProposeWave(snapshot, testEnvelope(snapshot), CollisionClosure{Complete: true, Groups: groups}, 128)
	if err != nil {
		t.Fatal(err)
	}
	return proposal
}

// TestProposalMaximumDisjointPairsStaysBounded covers the WQO-V0-044 node
// budget: 32 disjoint clash pairs among 64 candidates have 2^32 equal-size
// maxima, and the search must not enumerate them.
func TestProposalMaximumDisjointPairsStaysBounded(t *testing.T) {
	tickets := make([]TicketSummary, 64)
	for index := range tickets {
		tickets[index] = testTicket(rankLocal(index), Rank(index+1))
	}
	groups := make([]CollisionGroup, 0, 32)
	for pair := 0; pair < 32; pair++ {
		id := "collision:corvint:worklist:pair" + rankLocal(pair)
		groups = append(groups, CollisionGroup{ID: id, MemberTicketIDs: []string{tickets[2*pair].TicketID, tickets[2*pair+1].TicketID}, Source: "ADAPTER"})
	}
	snapshot := testSnapshot(tickets...)
	proposal := mustPropose(t, snapshot, groups)
	if proposal.WaveOptimality != "MAXIMUM" {
		t.Fatalf("optimality = %s, want MAXIMUM", proposal.WaveOptimality)
	}
	if got := selectedRanksFromProposal(proposal, snapshot); len(got) != 32 {
		t.Fatalf("selected %d of 64, want 32: %v", len(got), got)
	}
}

func TestProposalGreedyAbove64Candidates(t *testing.T) {
	tickets := make([]TicketSummary, 65)
	for index := range tickets {
		tickets[index] = testTicket(rankLocal(index), Rank(index+1))
	}
	snapshot := testSnapshot(tickets...)
	proposal, err := ProposeWave(snapshot, testEnvelope(snapshot), CollisionClosure{Complete: true}, 128)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.WaveOptimality != "GREEDY" || len(selectedRanksFromProposal(proposal, snapshot)) != 65 {
		t.Fatalf("optimality=%s selected=%d", proposal.WaveOptimality, len(selectedRanksFromProposal(proposal, snapshot)))
	}
}

func TestProposalMaximumAt64Candidates(t *testing.T) {
	tickets := make([]TicketSummary, 64)
	for index := range tickets {
		tickets[index] = testTicket(rankLocal(index), Rank(index+1))
	}
	snapshot := testSnapshot(tickets...)
	proposal, err := ProposeWave(snapshot, testEnvelope(snapshot), CollisionClosure{Complete: true}, 128)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.WaveOptimality != "MAXIMUM" {
		t.Fatalf("optimality = %s, want MAXIMUM", proposal.WaveOptimality)
	}
}

func TestProposalWaveLimitAndStale(t *testing.T) {
	snapshot := testSnapshot(testTicket("one", 1))
	envelope := testEnvelope(snapshot)
	if _, err := ProposeWave(snapshot, envelope, CollisionClosure{Complete: true}, 0); err == nil {
		t.Fatal("zero wave limit was accepted")
	}
	if _, err := ProposeWave(snapshot, envelope, CollisionClosure{Complete: true}, 129); err == nil {
		t.Fatal("wave limit 129 was accepted")
	}
	proposal, err := ProposeWave(snapshot, envelope, CollisionClosure{Complete: true}, 1)
	if err != nil {
		t.Fatal(err)
	}
	proposal.MarkStale()
	if proposal.State != "STALE" || len(proposal.Entries) != 0 || !equalStrings(proposal.Unknowns, []string{UnknownCheckpointChanged}) {
		t.Fatalf("stale proposal = %#v", proposal)
	}
}

// WQO-V0-018: missing observation state is missing evidence, never an
// implicit VALIDATED_AT observation that can select work.
func TestProposalMissingObservationStateIsEmpty(t *testing.T) {
	snapshot := testSnapshot(testTicket("one", 1))
	snapshot.ObservationState = ""

	proposal, err := ProposeWave(snapshot, testEnvelope(snapshot), CollisionClosure{Complete: true}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.State != "EMPTY" || len(proposal.Entries) != 0 {
		t.Fatalf("proposal with missing observation state = %#v", proposal)
	}
}

func TestProposalIdentityAndDeterminism(t *testing.T) {
	ticket := testTicket("one", 1)
	ticket.RouteAlternatives = []RouteAlternative{
		{ID: "route:corvint:worklist:z", Requires: []string{"capability:corvint:worklist:b", "capability:corvint:worklist:a"}},
		{ID: "route:corvint:worklist:a", Requires: []string{}},
	}
	snapshot := testSnapshot(ticket)
	envelope := testEnvelope(snapshot)
	envelope.Capabilities = []string{"capability:corvint:worklist:b", "capability:corvint:worklist:a"}
	refreshEnvelopeID(envelope)
	closure := CollisionClosure{Complete: true, Groups: []CollisionGroup{
		{ID: "collision:corvint:worklist:z", MemberTicketIDs: []string{ticket.TicketID}, Source: "ADAPTER"},
		{ID: "collision:corvint:worklist:a", MemberTicketIDs: []string{ticket.TicketID}, Source: "ADAPTER"},
	}}
	baseline, err := ProposeWave(snapshot, envelope, closure, 1)
	if err != nil {
		t.Fatal(err)
	}
	wantID := testIdentity("work-wave-proposal", ProposalProfile, proposalValue(baseline, false))
	if baseline.ID != wantID {
		t.Fatalf("proposal ID = %s, want %s", baseline.ID, wantID)
	}
	want := string(baseline.Canonical())
	random := rand.New(rand.NewSource(7))
	for iteration := 0; iteration < 100; iteration++ {
		random.Shuffle(len(snapshot.Tickets[0].RouteAlternatives), func(left, right int) {
			snapshot.Tickets[0].RouteAlternatives[left], snapshot.Tickets[0].RouteAlternatives[right] = snapshot.Tickets[0].RouteAlternatives[right], snapshot.Tickets[0].RouteAlternatives[left]
		})
		random.Shuffle(len(envelope.Capabilities), func(left, right int) {
			envelope.Capabilities[left], envelope.Capabilities[right] = envelope.Capabilities[right], envelope.Capabilities[left]
		})
		random.Shuffle(len(closure.Groups), func(left, right int) {
			closure.Groups[left], closure.Groups[right] = closure.Groups[right], closure.Groups[left]
		})
		refreshEnvelopeID(envelope)
		got, err := ProposeWave(snapshot, envelope, closure, 1)
		if err != nil {
			t.Fatal(err)
		}
		if string(got.Canonical()) != want {
			t.Fatalf("iteration %d produced different bytes", iteration)
		}
	}
}

func refreshEnvelopeID(envelope *CapacityEnvelope) {
	envelope.ID = testIdentity("work-capacity-envelope", EnvelopeProfile, envelopeValue(envelope, false))
}

func rankLocal(index int) string {
	return "t" + string(rune('a'+index/26)) + string(rune('a'+index%26))
}

func selectedRanksFromProposal(proposal *Proposal, snapshot *Snapshot) []Rank {
	ranks := map[string]Rank{}
	for _, ticket := range snapshot.Tickets {
		ranks[ticket.TicketID] = ticket.Rank
	}
	result := []Rank{}
	for _, entry := range proposal.Entries {
		if entry.State == "SELECTED" {
			result = append(result, ranks[entry.TicketID])
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result
}

func bruteForceRanks(tickets []TicketSummary, groups []CollisionGroup) []Rank {
	index := map[string]int{}
	for ticketIndex, ticket := range tickets {
		index[ticket.TicketID] = ticketIndex
	}
	edges := map[[2]int]struct{}{}
	for _, group := range groups {
		for left := 0; left < len(group.MemberTicketIDs); left++ {
			for right := left + 1; right < len(group.MemberTicketIDs); right++ {
				a := index[group.MemberTicketIDs[left]]
				b := index[group.MemberTicketIDs[right]]
				if a > b {
					a, b = b, a
				}
				edges[[2]int{a, b}] = struct{}{}
			}
		}
	}
	best := []Rank{}
	for selection := 0; selection < 1<<len(tickets); selection++ {
		if bits.OnesCount(uint(selection)) < len(best) || clashes(selection, edges) {
			continue
		}
		current := []Rank{}
		for index, ticket := range tickets {
			if selection&(1<<index) != 0 {
				current = append(current, ticket.Rank)
			}
		}
		if len(current) > len(best) || lexRanksLess(current, best) {
			best = current
		}
	}
	return best
}

func clashes(selection int, edges map[[2]int]struct{}) bool {
	for edge := range edges {
		if selection&(1<<edge[0]) != 0 && selection&(1<<edge[1]) != 0 {
			return true
		}
	}
	return false
}

func lexRanksLess(left, right []Rank) bool {
	if len(right) == 0 {
		return len(left) > 0
	}
	for index := range left {
		if left[index] != right[index] {
			return left[index] < right[index]
		}
	}
	return false
}

func equalRanks(left, right []Rank) bool {
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
