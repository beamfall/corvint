package workqueue

import (
	"math"
	"sort"
)

type validation struct {
	conflicted bool
	partial    bool
	unable     bool
	unknowns   map[string]struct{}
}

func (result *validation) unknown(code string) {
	result.unknowns[code] = struct{}{}
	result.unable = true
}

func (result validation) finish() ValidationResult {
	unknowns := make([]string, 0, len(result.unknowns))
	for code := range result.unknowns {
		unknowns = append(unknowns, code)
	}
	sort.Strings(unknowns)
	state := ResolveState(StateFacts{IdentityContradiction: result.conflicted, PositivePartial: result.partial, Unable: result.unable})
	return ValidationResult{State: state, Unknowns: unknowns}
}

func ValidateSnapshot(snapshot *Snapshot) ValidationResult {
	result := validation{unknowns: map[string]struct{}{}}
	if snapshot == nil {
		result.conflicted = true
		return result.finish()
	}
	authority, queue, rootQualified := authorityTokens(snapshot.RepositoryAuthorityID, snapshot.QueueAuthorityID)
	if !rootQualified {
		result.unknown(UnknownMultiRepoUnsupported)
	}
	validateSnapshotAuthorities(snapshot, authority, queue, &result)
	validateSnapshotIdentities(snapshot, &result)
	validateSnapshotReferences(snapshot, authority, queue, &result)
	validateSnapshotCapacity(snapshot, authority, queue, &result)
	if snapshot.Scope.TicketCount != Count(len(snapshot.Tickets)) {
		result.conflicted = true
	}
	if !snapshot.Scope.Complete {
		result.partial = true
	}
	return result.finish()
}

func validateSnapshotAuthorities(snapshot *Snapshot, authority, queue string, result *validation) {
	accessAuthority, _, _, accessValid := splitQualifiedID(snapshot.AccessContextID, "access")
	if !accessValid || accessAuthority != authority {
		result.unknown(UnknownMultiRepoUnsupported)
	}
	qualified := []struct {
		value string
		kind  string
	}{
		{snapshot.Checkpoint.ID, "checkpoint"},
		{snapshot.Scope.ID, "scope"},
	}
	for _, item := range qualified {
		foreign, malformed := validateQualifiedAuthority(item.value, item.kind, authority, queue)
		if foreign || malformed {
			result.unknown(UnknownMultiRepoUnsupported)
		}
	}
	for _, class := range snapshot.CapacityClasses {
		markForeign(class.ID, "capacity", authority, queue, result)
	}
	for _, ticket := range snapshot.Tickets {
		if ticket.RepositoryAuthorityID != snapshot.RepositoryAuthorityID || ticket.QueueAuthorityID != snapshot.QueueAuthorityID {
			result.unknown(UnknownMultiRepoUnsupported)
		}
		markForeign(ticket.TicketID, "ticket", authority, queue, result)
		for _, dependency := range ticket.DependencyTicketIDs {
			markForeign(dependency, "ticket", authority, queue, result)
		}
		for _, group := range ticket.CollisionGroupIDs {
			markForeign(group, "collision", authority, queue, result)
		}
		for _, use := range ticket.CapacityUses {
			markForeign(use.ClassID, "capacity", authority, queue, result)
		}
		for _, route := range ticket.RouteAlternatives {
			markForeign(route.ID, "route", authority, queue, result)
			for _, capability := range route.Requires {
				markForeign(capability, "capability", authority, queue, result)
			}
		}
		validateAtomicAuthorities(ticket, snapshot.RepositoryAuthorityID, result)
	}
	for _, lease := range snapshot.Leases {
		if lease.RepositoryAuthorityID != snapshot.RepositoryAuthorityID || lease.QueueAuthorityID != snapshot.QueueAuthorityID {
			result.unknown(UnknownMultiRepoUnsupported)
		}
		markForeign(lease.HolderID, "holder", authority, queue, result)
		markForeign(lease.LeaseID, "lease", authority, queue, result)
		markForeign(lease.TicketID, "ticket", authority, queue, result)
		for _, group := range lease.CollisionGroupIDs {
			markForeign(group, "collision", authority, queue, result)
		}
		for _, use := range lease.CapacityUses {
			markForeign(use.ClassID, "capacity", authority, queue, result)
		}
	}
}

func markForeign(value, kind, authority, queue string, result *validation) {
	foreign, malformed := validateQualifiedAuthority(value, kind, authority, queue)
	if foreign || malformed {
		result.unknown(UnknownMultiRepoUnsupported)
	}
}

func validateAtomicAuthorities(ticket TicketSummary, repositoryID string, result *validation) {
	found := 0
	for _, repository := range ticket.AtomicRepositoryAuthorityIDs {
		if repository == repositoryID {
			found++
			continue
		}
		result.unknown(UnknownMultiRepoUnsupported)
	}
	if found != 1 {
		result.conflicted = true
	}
}

func validateSnapshotIdentities(snapshot *Snapshot, result *validation) {
	wantSource := contentIdentity("repository-source", "repository-source/0", repositorySourceValue(snapshot.RepositorySource, false))
	if snapshot.RepositorySource.ID != wantSource {
		result.conflicted = true
	}
	wantSnapshot := contentIdentity("work-queue-snapshot", SnapshotProfile, snapshotValue(snapshot, false))
	if snapshot.ID != wantSnapshot {
		result.conflicted = true
	}
	ticketIDs := map[string]struct{}{}
	ticketVersions := map[string]struct{}{}
	ranks := map[Rank]struct{}{}
	for _, ticket := range snapshot.Tickets {
		if duplicate(ticketIDs, ticket.TicketID) || duplicate(ticketVersions, ticket.TicketVersionID) || duplicate(ranks, ticket.Rank) {
			result.conflicted = true
		}
		want := contentIdentity("ticket-version", "ticket-version/0", ticketVersionBody(ticket))
		if ticket.TicketVersionID != want {
			result.conflicted = true
		}
		if ticket.Lifecycle == "READY" && contradictoryReady(ticket) {
			result.conflicted = true
		}
	}
	leaseIDs := map[string]struct{}{}
	leaseVersions := map[string]struct{}{}
	for _, lease := range snapshot.Leases {
		if duplicate(leaseIDs, lease.LeaseID) || duplicate(leaseVersions, lease.LeaseVersionID) {
			result.conflicted = true
		}
		want := contentIdentity("lease-version", "lease-version/0", leaseValue(lease, false))
		if lease.LeaseVersionID != want {
			result.conflicted = true
		}
	}
}

func duplicate[T comparable](seen map[T]struct{}, value T) bool {
	if _, found := seen[value]; found {
		return true
	}
	seen[value] = struct{}{}
	return false
}

func contradictoryReady(ticket TicketSummary) bool {
	facts := ticket.SelectionFacts
	return facts.Approvals == "BLOCKED" || facts.Dependencies == "BLOCKED" || facts.Holds == "HELD" || facts.Lease == "PRESENT"
}

func validateSnapshotReferences(snapshot *Snapshot, authority, queue string, result *validation) {
	ticketVersions := make(map[string]string, len(snapshot.Tickets))
	tickets := make(map[string]struct{}, len(snapshot.Tickets))
	collisionGroups := map[string]struct{}{}
	for _, ticket := range snapshot.Tickets {
		tickets[ticket.TicketID] = struct{}{}
		ticketVersions[ticket.TicketID] = ticket.TicketVersionID
		for _, group := range ticket.CollisionGroupIDs {
			collisionGroups[group] = struct{}{}
		}
	}
	for _, ticket := range snapshot.Tickets {
		for _, dependency := range ticket.DependencyTicketIDs {
			if _, found := tickets[dependency]; !found {
				result.unknown(UnknownReference)
			}
		}
	}
	for _, requested := range snapshot.DetailRequestTicketVersionIDs {
		found := false
		for _, version := range ticketVersions {
			found = found || version == requested
		}
		if !found {
			result.unknown(UnknownReference)
		}
	}
	for _, lease := range snapshot.Leases {
		version, found := ticketVersions[lease.TicketID]
		if !found || version != lease.TicketVersionID {
			result.unknown(UnknownReference)
		}
		for _, group := range lease.CollisionGroupIDs {
			if _, found := collisionGroups[group]; !found {
				result.unknown(UnknownReference)
			}
		}
	}
	_ = authority
	_ = queue
}

func validateSnapshotCapacity(snapshot *Snapshot, authority, queue string, result *validation) {
	classes := make(map[string]struct{}, len(snapshot.CapacityClasses))
	for _, class := range snapshot.CapacityClasses {
		if duplicate(classes, class.ID) {
			result.conflicted = true
		}
	}
	for _, ticket := range snapshot.Tickets {
		validateCapacityUses(ticket.CapacityUses, classes, result)
	}
	for _, lease := range snapshot.Leases {
		validateCapacityUses(lease.CapacityUses, classes, result)
	}
	_ = authority
	_ = queue
}

func validateCapacityUses(uses []CapacityUse, classes map[string]struct{}, result *validation) {
	seen := map[string]struct{}{}
	var total uint64
	for _, use := range uses {
		if _, found := classes[use.ClassID]; !found {
			result.conflicted = true
		}
		if duplicate(seen, use.ClassID) || use.Units == 0 {
			result.conflicted = true
		}
		total += uint64(use.Units)
		if total > math.MaxInt32 {
			result.conflicted = true
		}
	}
}
