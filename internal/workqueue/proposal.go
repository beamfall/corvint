package workqueue

import (
	"math/bits"
	"sort"
)

const maxProposalBytes = 16 << 20

const (
	reasonUnknownAuthority     = "UNKNOWN_AUTHORITY"
	reasonContradictoryReady   = "CONTRADICTORY_READY"
	reasonRouteUnavailable     = "ROUTE_UNAVAILABLE"
	reasonCapacityExhausted    = "CAPACITY_EXHAUSTED"
	reasonActiveCollision      = "ACTIVE_COLLISION"
	reasonSelectedCollision    = "SELECTED_COLLISION"
	reasonWaveLimit            = "WAVE_LIMIT"
	reasonEligibleAtCheckpoint = "ELIGIBLE_AT_CHECKPOINT"
)

type candidate struct {
	ticket TicketSummary
	route  *string
	groups []string
}

type capacityPools struct {
	repository map[string]Count
	caller     map[string]Count
}

func ProposeWave(snapshot *Snapshot, envelope *CapacityEnvelope, closure CollisionClosure, limit Count) (*Proposal, error) {
	if limit < 1 || limit > 128 {
		return nil, fail(CodeMalformedInput, "wave limit must be in 1..128")
	}
	if snapshot == nil || envelope == nil {
		return nil, fail(CodeMalformedInput, "snapshot and capacity envelope are required")
	}
	if !validContentID(snapshot.ObservationID, "work-queue-observation") || !validContentID(snapshot.QueueSourceID, "queue-source") {
		return nil, fail(UnknownSourceUnqualified, "proposal bindings are required")
	}
	proposal := newProposal(snapshot, envelope, closure, limit)
	for _, code := range snapshot.ObservationUnknowns {
		if _, valid := unknownCodeSet[code]; !valid {
			return nil, fail(CodeMalformedInput, "proposal contains an unknown code")
		}
	}
	proposal.Unknowns = sortedUnique(snapshot.ObservationUnknowns)
	if snapshot.ObservationState != StateValidated {
		return finishProposal(proposal)
	}
	if !closure.Complete {
		proposal.Unknowns = sortedUnique(append(proposal.Unknowns, UnknownCollisionClosureIncomplete))
		proposal.CollisionClosure = []CollisionGroup{}
		return finishProposal(proposal)
	}
	if !validateClosureForSnapshot(snapshot, closure) {
		proposal.Unknowns = sortedUnique(append(proposal.Unknowns, UnknownCollisionClosureIncomplete))
		proposal.CollisionClosure = []CollisionGroup{}
		return finishProposal(proposal)
	}
	if !proposalAuthoritiesValid(snapshot) {
		return finishProposal(proposal)
	}
	pools, validEnvelope, err := validateEnvelopeForSnapshot(snapshot, envelope)
	if err != nil {
		return nil, err
	}
	if !validEnvelope {
		return finishProposal(proposal)
	}
	if !proposalCapacityValid(snapshot) {
		return nil, fail(CodeConflicted, "capacity uses are inconsistent")
	}
	groups := ticketGroups(snapshot, closure)
	blockingGroups := blockingLeaseGroups(snapshot)
	decisions := map[string]ProposalEntry{}
	candidates := collectCandidates(snapshot, envelope, pools, groups, blockingGroups, decisions)
	selection, optimality := collisionSelection(candidates)
	proposal.WaveOptimality = optimality
	selectedGroups := map[string]struct{}{}
	selectedCount := Count(0)
	for _, index := range selectionOrder(candidates, selection) {
		current := candidates[index]
		entry := proposalEntry(current.ticket, "", "", nil)
		switch {
		case !fitsCapacity(current.ticket.CapacityUses, pools):
			entry = proposalEntry(current.ticket, "EXCLUDED", reasonCapacityExhausted, nil)
		case len(intersection(current.groups, blockingGroups)) > 0:
			entry = proposalEntry(current.ticket, "EXCLUDED", reasonActiveCollision, intersection(current.groups, blockingGroups))
		case len(intersection(current.groups, selectedGroups)) > 0:
			entry = proposalEntry(current.ticket, "EXCLUDED", reasonSelectedCollision, intersection(current.groups, selectedGroups))
		case selectedCount >= limit:
			entry = proposalEntry(current.ticket, "EXCLUDED", reasonWaveLimit, nil)
		default:
			subtractCapacity(current.ticket.CapacityUses, pools)
			for _, group := range current.groups {
				selectedGroups[group] = struct{}{}
			}
			selectedCount++
			entry = proposalEntry(current.ticket, "SELECTED", reasonEligibleAtCheckpoint, nil)
			entry.RouteAlternativeID = current.route
		}
		decisions[current.ticket.TicketID] = entry
	}
	for _, ticket := range snapshot.Tickets {
		if ticket.Lifecycle != "READY" {
			continue
		}
		proposal.Entries = append(proposal.Entries, decisions[ticket.TicketID])
	}
	if selectedCount > 0 {
		proposal.State = "ELIGIBLE_AT"
	}
	return finishProposal(proposal)
}

func newProposal(snapshot *Snapshot, envelope *CapacityEnvelope, closure CollisionClosure, limit Count) *Proposal {
	groups := normalizeCollisionGroups(closure.Groups)
	return &Proposal{
		CapacityEnvelopeID: envelope.ID,
		CollisionClosure:   groups,
		Entries:            []ProposalEntry{},
		MutationAuthority:  false,
		ObservationID:      snapshot.ObservationID,
		Profile:            ProposalProfile,
		QueueSourceID:      snapshot.QueueSourceID,
		State:              "EMPTY",
		Unknowns:           []string{},
		WaveLimit:          limit,
		WaveOptimality:     "NONE",
	}
}

func finishProposal(proposal *Proposal) (*Proposal, error) {
	proposal.refreshIdentity()
	if len(proposal.Canonical()) > maxProposalBytes {
		return nil, fail(CodeInputLimit, "proposal exceeds %d bytes", maxProposalBytes)
	}
	return proposal, nil
}

func validateEnvelopeForSnapshot(snapshot *Snapshot, envelope *CapacityEnvelope) (capacityPools, bool, error) {
	pools := capacityPools{repository: map[string]Count{}, caller: map[string]Count{}}
	if envelope.Profile != EnvelopeProfile || envelope.RepositoryAuthorityID != snapshot.RepositoryAuthorityID {
		return pools, false, nil
	}
	if envelope.ID != contentIdentity("work-capacity-envelope", EnvelopeProfile, envelopeValue(envelope, false)) {
		return pools, false, nil
	}
	authority, queue, ok := authorityTokens(snapshot.RepositoryAuthorityID, snapshot.QueueAuthorityID)
	if !ok {
		return pools, false, nil
	}
	for _, capability := range envelope.Capabilities {
		foreign, malformed := validateQualifiedAuthority(capability, "capability", authority, queue)
		if foreign || malformed {
			return pools, false, nil
		}
	}
	if len(sortedUnique(envelope.Capabilities)) != len(envelope.Capabilities) {
		return pools, false, nil
	}
	// Foreign capacity remains ordinary abstention under 018/024. Check every
	// authority before classifying a missing or unknown local class under 019.
	for _, class := range envelope.Available {
		if !qualifiedForQueue(class.ID, "capacity", authority, queue) {
			return pools, false, nil
		}
	}
	for _, class := range snapshot.CapacityClasses {
		if _, duplicate := pools.repository[class.ID]; duplicate {
			return pools, false, fail(CodeConflicted, "duplicate repository capacity class")
		}
		if uint64(class.AvailableUnits) > maxCount {
			return pools, false, fail(CodeConflicted, "repository capacity exceeds Count bound")
		}
		pools.repository[class.ID] = class.AvailableUnits
	}
	for _, class := range envelope.Available {
		if _, duplicate := pools.caller[class.ID]; duplicate {
			return pools, false, fail(CodeConflicted, "duplicate caller capacity class")
		}
		if _, declared := pools.repository[class.ID]; !declared {
			return pools, false, fail(CodeConflicted, "unknown caller capacity class")
		}
		if uint64(class.AvailableUnits) > maxCount {
			return pools, false, fail(CodeConflicted, "caller capacity exceeds Count bound")
		}
		pools.caller[class.ID] = class.AvailableUnits
	}
	if len(pools.repository) != len(pools.caller) {
		return pools, false, fail(CodeConflicted, "missing caller capacity class")
	}
	return pools, true, nil
}

func ticketGroups(snapshot *Snapshot, closure CollisionClosure) map[string][]string {
	result := map[string][]string{}
	for _, ticket := range snapshot.Tickets {
		result[ticket.TicketID] = append([]string(nil), ticket.CollisionGroupIDs...)
	}
	for ticketID, groupIDs := range closure.TicketGroupIDs {
		result[ticketID] = append(result[ticketID], groupIDs...)
	}
	for _, group := range closure.Groups {
		for _, ticketID := range group.MemberTicketIDs {
			result[ticketID] = append(result[ticketID], group.ID)
		}
	}
	for ticketID, groupIDs := range result {
		result[ticketID] = sortedUnique(groupIDs)
	}
	return result
}

func validateClosureForSnapshot(snapshot *Snapshot, closure CollisionClosure) bool {
	authority, queue, valid := authorityTokens(snapshot.RepositoryAuthorityID, snapshot.QueueAuthorityID)
	if !valid {
		return false
	}
	ready := map[string]struct{}{}
	requiredAdapterGroups := map[string]struct{}{}
	for _, ticket := range snapshot.Tickets {
		if ticket.Lifecycle != "READY" {
			continue
		}
		ready[ticket.TicketID] = struct{}{}
		for _, groupID := range ticket.CollisionGroupIDs {
			requiredAdapterGroups[groupID] = struct{}{}
		}
	}
	seen := map[string]struct{}{}
	for _, group := range closure.Groups {
		if duplicate(seen, group.ID) || !qualifiedForQueue(group.ID, "collision", authority, queue) {
			return false
		}
		if len(group.MemberTicketIDs) == 0 || len(sortedUnique(group.MemberTicketIDs)) != len(group.MemberTicketIDs) {
			return false
		}
		for _, ticketID := range group.MemberTicketIDs {
			if _, found := ready[ticketID]; !found {
				return false
			}
		}
		switch group.Source {
		case "ADAPTER":
			if group.Path != nil {
				return false
			}
			delete(requiredAdapterGroups, group.ID)
		case "CORVINT_INDEX":
			if group.Path == nil || ValidatePath(*group.Path) != nil || len(group.MemberTicketIDs) < 2 {
				return false
			}
			if group.ID != derivedCollisionID(authority, queue, *group.Path) {
				return false
			}
		default:
			return false
		}
	}
	return len(requiredAdapterGroups) == 0
}

func blockingLeaseGroups(snapshot *Snapshot) map[string]struct{} {
	result := map[string]struct{}{}
	for _, lease := range snapshot.Leases {
		if !lease.BlocksSelection {
			continue
		}
		for _, group := range lease.CollisionGroupIDs {
			result[group] = struct{}{}
		}
	}
	return result
}

func collectCandidates(snapshot *Snapshot, envelope *CapacityEnvelope, pools capacityPools, groups map[string][]string, blockingGroups map[string]struct{}, decisions map[string]ProposalEntry) []candidate {
	capabilities := make(map[string]struct{}, len(envelope.Capabilities))
	for _, capability := range envelope.Capabilities {
		capabilities[capability] = struct{}{}
	}
	result := []candidate{}
	for _, ticket := range snapshot.Tickets {
		if ticket.Lifecycle != "READY" {
			continue
		}
		groupIDs := groups[ticket.TicketID]
		route := satisfyingRoute(ticket.RouteAlternatives, capabilities)
		switch {
		case unknownReady(ticket):
			decisions[ticket.TicketID] = proposalEntry(ticket, "ABSTAINED", reasonUnknownAuthority, nil)
		case contradictoryReady(ticket):
			decisions[ticket.TicketID] = proposalEntry(ticket, "ABSTAINED", reasonContradictoryReady, nil)
		case route == nil:
			decisions[ticket.TicketID] = proposalEntry(ticket, "EXCLUDED", reasonRouteUnavailable, nil)
		case !fitsCapacity(ticket.CapacityUses, pools):
			decisions[ticket.TicketID] = proposalEntry(ticket, "EXCLUDED", reasonCapacityExhausted, nil)
		case len(intersection(groupIDs, blockingGroups)) > 0:
			decisions[ticket.TicketID] = proposalEntry(ticket, "EXCLUDED", reasonActiveCollision, intersection(groupIDs, blockingGroups))
		default:
			result = append(result, candidate{ticket: ticket, route: route, groups: groupIDs})
		}
	}
	return result
}

func unknownReady(ticket TicketSummary) bool {
	if ticket.Authority == "UNKNOWN" {
		return true
	}
	facts := ticket.SelectionFacts
	return facts.Approvals == "UNKNOWN" || facts.Dependencies == "UNKNOWN" || facts.Holds == "UNKNOWN" || facts.Lease == "UNKNOWN"
}

func satisfyingRoute(routes []RouteAlternative, capabilities map[string]struct{}) *string {
	ordered := append([]RouteAlternative(nil), routes...)
	canonicalSort(ordered, routeAlternativeValue)
	for _, route := range ordered {
		satisfied := true
		for _, required := range route.Requires {
			if _, found := capabilities[required]; !found {
				satisfied = false
				break
			}
		}
		if satisfied {
			selected := route.ID
			return &selected
		}
	}
	return nil
}

func fitsCapacity(uses []CapacityUse, pools capacityPools) bool {
	normalized, valid := normalizedCapacityUses(uses)
	if !valid {
		return false
	}
	for _, use := range normalized {
		repository, repositoryFound := pools.repository[use.ClassID]
		caller, callerFound := pools.caller[use.ClassID]
		if !repositoryFound || !callerFound || repository < use.Units || caller < use.Units {
			return false
		}
	}
	return true
}

func subtractCapacity(uses []CapacityUse, pools capacityPools) {
	normalized, _ := normalizedCapacityUses(uses)
	for _, use := range normalized {
		pools.repository[use.ClassID] -= use.Units
		pools.caller[use.ClassID] -= use.Units
	}
}

func normalizedCapacityUses(uses []CapacityUse) ([]CapacityUse, bool) {
	result := append([]CapacityUse(nil), uses...)
	sort.Slice(result, func(left, right int) bool { return result[left].ClassID < result[right].ClassID })
	for index, use := range result {
		if use.Units == 0 || uint64(use.Units) > maxCount {
			return nil, false
		}
		if index > 0 && result[index-1].ClassID == use.ClassID {
			return nil, false
		}
	}
	return result, true
}

func proposalCapacityValid(snapshot *Snapshot) bool {
	classes := map[string]struct{}{}
	for _, class := range snapshot.CapacityClasses {
		if duplicate(classes, class.ID) {
			return false
		}
	}
	for _, ticket := range snapshot.Tickets {
		if !usesReferenceClasses(ticket.CapacityUses, classes) {
			return false
		}
	}
	for _, lease := range snapshot.Leases {
		if !usesReferenceClasses(lease.CapacityUses, classes) {
			return false
		}
	}
	return true
}

func usesReferenceClasses(uses []CapacityUse, classes map[string]struct{}) bool {
	normalized, valid := normalizedCapacityUses(uses)
	if !valid {
		return false
	}
	var total uint64
	for _, use := range normalized {
		if _, found := classes[use.ClassID]; !found {
			return false
		}
		total += uint64(use.Units)
		if total > maxCount {
			return false
		}
	}
	return true
}

func proposalAuthoritiesValid(snapshot *Snapshot) bool {
	authority, queue, valid := authorityTokens(snapshot.RepositoryAuthorityID, snapshot.QueueAuthorityID)
	if !valid {
		return false
	}
	for _, class := range snapshot.CapacityClasses {
		if !qualifiedForQueue(class.ID, "capacity", authority, queue) {
			return false
		}
	}
	for _, ticket := range snapshot.Tickets {
		if ticket.RepositoryAuthorityID != snapshot.RepositoryAuthorityID || ticket.QueueAuthorityID != snapshot.QueueAuthorityID {
			return false
		}
		if !qualifiedForQueue(ticket.TicketID, "ticket", authority, queue) {
			return false
		}
		if len(ticket.AtomicRepositoryAuthorityIDs) != 1 {
			return false
		}
		for _, repository := range ticket.AtomicRepositoryAuthorityIDs {
			if repository != snapshot.RepositoryAuthorityID {
				return false
			}
		}
		for _, dependency := range ticket.DependencyTicketIDs {
			if !qualifiedForQueue(dependency, "ticket", authority, queue) {
				return false
			}
		}
		for _, group := range ticket.CollisionGroupIDs {
			if !qualifiedForQueue(group, "collision", authority, queue) {
				return false
			}
		}
		for _, use := range ticket.CapacityUses {
			if !qualifiedForQueue(use.ClassID, "capacity", authority, queue) {
				return false
			}
		}
		for _, route := range ticket.RouteAlternatives {
			if !qualifiedForQueue(route.ID, "route", authority, queue) {
				return false
			}
			for _, capability := range route.Requires {
				if !qualifiedForQueue(capability, "capability", authority, queue) {
					return false
				}
			}
		}
	}
	for _, lease := range snapshot.Leases {
		if lease.RepositoryAuthorityID != snapshot.RepositoryAuthorityID || lease.QueueAuthorityID != snapshot.QueueAuthorityID {
			return false
		}
		if !qualifiedForQueue(lease.HolderID, "holder", authority, queue) || !qualifiedForQueue(lease.LeaseID, "lease", authority, queue) || !qualifiedForQueue(lease.TicketID, "ticket", authority, queue) {
			return false
		}
		for _, use := range lease.CapacityUses {
			if !qualifiedForQueue(use.ClassID, "capacity", authority, queue) {
				return false
			}
		}
		for _, group := range lease.CollisionGroupIDs {
			if !qualifiedForQueue(group, "collision", authority, queue) {
				return false
			}
		}
	}
	return true
}

func qualifiedForQueue(value, kind, authority, queue string) bool {
	foreign, malformed := validateQualifiedAuthority(value, kind, authority, queue)
	return !foreign && !malformed
}

func intersection(values []string, set map[string]struct{}) []string {
	result := []string{}
	for _, value := range values {
		if _, found := set[value]; found {
			result = append(result, value)
		}
	}
	return sortedUnique(result)
}

func proposalEntry(ticket TicketSummary, state, reason string, groups []string) ProposalEntry {
	if groups == nil {
		groups = []string{}
	}
	return ProposalEntry{
		CollisionGroupIDs: sortedUnique(groups),
		Reason:            reason,
		State:             state,
		TicketID:          ticket.TicketID,
		TicketVersionID:   ticket.TicketVersionID,
	}
}

func collisionSelection(candidates []candidate) (map[int]struct{}, string) {
	if len(candidates) == 0 {
		return map[int]struct{}{}, "NONE"
	}
	adjacency := candidateAdjacency(candidates)
	if len(candidates) <= 64 {
		if selection, ok := maximumIndependentSet(candidates, adjacency); ok {
			return selection, "MAXIMUM"
		}
	}
	return greedyIndependentSet(candidates, adjacency), "GREEDY"
}

func candidateAdjacency(candidates []candidate) [][]int {
	groups := make([]map[string]struct{}, len(candidates))
	for index, candidate := range candidates {
		groups[index] = make(map[string]struct{}, len(candidate.groups))
		for _, group := range candidate.groups {
			groups[index][group] = struct{}{}
		}
	}
	adjacency := make([][]int, len(candidates))
	for left := 0; left < len(candidates); left++ {
		for right := left + 1; right < len(candidates); right++ {
			if len(intersection(candidates[right].groups, groups[left])) == 0 {
				continue
			}
			adjacency[left] = append(adjacency[left], right)
			adjacency[right] = append(adjacency[right], left)
		}
	}
	return adjacency
}

// maximumSearchNodes bounds the exact search (WQO-V0-044). A graph with many
// equal-size maxima, such as 32 disjoint clash pairs, is otherwise exponential.
const maximumSearchNodes = 1_000_000

// maximumIndependentSet returns an exact maximum collision-free set, or false
// when the node budget is exhausted before the search completes. The search
// branches on the candidate with the most clashes inside the remaining set,
// ties by ascending rank, include-branch first; the first maximum reached in
// that order wins, and a branch that cannot strictly beat it is pruned.
func maximumIndependentSet(candidates []candidate, adjacency [][]int) (map[int]struct{}, bool) {
	neighbours := make([]uint64, len(candidates))
	for index, clashes := range adjacency {
		for _, clash := range clashes {
			neighbours[index] |= uint64(1) << clash
		}
	}
	remaining := ^uint64(0)
	if len(candidates) < 64 {
		remaining = (uint64(1) << len(candidates)) - 1
	}
	best := uint64(0)
	bestCount := 0
	nodes := 0
	var search func(uint64, uint64) bool
	search = func(available, chosen uint64) bool {
		nodes++
		if nodes > maximumSearchNodes {
			return false
		}
		if bits.OnesCount64(chosen)+cliqueCoverBound(available, neighbours) <= bestCount {
			return true
		}
		if available == 0 {
			best, bestCount = chosen, bits.OnesCount64(chosen)
			return true
		}
		branch := branchCandidate(available, neighbours, candidates)
		bit := uint64(1) << branch
		if !search(available&^bit&^neighbours[branch], chosen|bit) {
			return false
		}
		return search(available&^bit, chosen)
	}
	if !search(remaining, 0) {
		return nil, false
	}
	result := map[int]struct{}{}
	for index := range candidates {
		if best&(uint64(1)<<index) != 0 {
			result[index] = struct{}{}
		}
	}
	return result, true
}

// cliqueCoverBound is an upper bound on the largest collision-free subset of
// the available candidates: every clique of a clique cover contributes at
// most one member. The cover is greedy, so the bound is cheap and exact on a
// set of disjoint clash pairs, where counting candidates would not prune.
func cliqueCoverBound(available uint64, neighbours []uint64) int {
	cliques := []uint64{}
	for index := range neighbours {
		bit := uint64(1) << index
		if available&bit == 0 {
			continue
		}
		placed := false
		for at, clique := range cliques {
			if neighbours[index]&clique == clique {
				cliques[at] = clique | bit
				placed = true
				break
			}
		}
		if !placed {
			cliques = append(cliques, bit)
		}
	}
	return len(cliques)
}

func branchCandidate(available uint64, neighbours []uint64, candidates []candidate) int {
	selected := -1
	clashes := -1
	for index := range candidates {
		bit := uint64(1) << index
		if available&bit == 0 {
			continue
		}
		count := bits.OnesCount64(neighbours[index] & available)
		if count > clashes || (count == clashes && candidates[index].ticket.Rank < candidates[selected].ticket.Rank) {
			selected = index
			clashes = count
		}
	}
	return selected
}

func greedyIndependentSet(candidates []candidate, adjacency [][]int) map[int]struct{} {
	order := make([]int, len(candidates))
	for index := range candidates {
		order[index] = index
	}
	sort.Slice(order, func(left, right int) bool {
		leftIndex := order[left]
		rightIndex := order[right]
		if len(adjacency[leftIndex]) != len(adjacency[rightIndex]) {
			return len(adjacency[leftIndex]) < len(adjacency[rightIndex])
		}
		return candidates[leftIndex].ticket.Rank < candidates[rightIndex].ticket.Rank
	})
	result := map[int]struct{}{}
	for _, index := range order {
		clashes := false
		for _, neighbour := range adjacency[index] {
			_, clashes = result[neighbour]
			if clashes {
				break
			}
		}
		if !clashes {
			result[index] = struct{}{}
		}
	}
	return result
}

func selectionOrder(candidates []candidate, selection map[int]struct{}) []int {
	selected := []int{}
	remaining := []int{}
	for index := range candidates {
		if _, found := selection[index]; found {
			selected = append(selected, index)
		} else {
			remaining = append(remaining, index)
		}
	}
	byRank := func(values []int) {
		sort.Slice(values, func(left, right int) bool {
			return candidates[values[left]].ticket.Rank < candidates[values[right]].ticket.Rank
		})
	}
	byRank(selected)
	byRank(remaining)
	return append(selected, remaining...)
}

func normalizeCollisionGroups(groups []CollisionGroup) []CollisionGroup {
	result := make([]CollisionGroup, len(groups))
	for index, group := range groups {
		result[index] = group
		result[index].MemberTicketIDs = sortedUnique(group.MemberTicketIDs)
	}
	canonicalSort(result, collisionGroupValue)
	return result
}
