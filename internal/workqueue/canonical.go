package workqueue

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func object(fields map[string]wire.Value) wire.Value {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	return wire.Value{Kind: wire.KindObject, Obj: &wire.Object{Keys: keys, Values: fields}}
}

func stringValue(value string) wire.Value { return wire.Value{Kind: wire.KindString, Str: value} }
func boolValue(value bool) wire.Value     { return wire.Value{Kind: wire.KindBool, Bool: value} }
func nullValue() wire.Value               { return wire.Value{Kind: wire.KindNull} }

func stringsValue(values []string) wire.Value {
	ordered := append([]string(nil), values...)
	sort.Slice(ordered, func(left, right int) bool {
		return bytes.Compare(wire.CanonicalValue(stringValue(ordered[left])), wire.CanonicalValue(stringValue(ordered[right]))) < 0
	})
	items := make([]wire.Value, 0, len(ordered))
	for _, value := range ordered {
		items = append(items, stringValue(value))
	}
	return wire.Value{Kind: wire.KindArray, Arr: items}
}

func arrayValue[T any](values []T, project func(T) wire.Value) wire.Value {
	items := make([]wire.Value, 0, len(values))
	for _, value := range values {
		items = append(items, project(value))
	}
	return wire.Value{Kind: wire.KindArray, Arr: items}
}

func sortedArrayValue[T any](values []T, project func(T) wire.Value) wire.Value {
	items := arrayValue(values, project).Arr
	sort.Slice(items, func(left, right int) bool {
		return bytes.Compare(wire.CanonicalValue(items[left]), wire.CanonicalValue(items[right])) < 0
	})
	return wire.Value{Kind: wire.KindArray, Arr: items}
}

func optionalStringValue(value *string) wire.Value {
	if value == nil {
		return nullValue()
	}
	return stringValue(*value)
}

func repositorySourceValue(source RepositorySource, includeID bool) wire.Value {
	fields := map[string]wire.Value{
		"commit":                stringValue(source.Commit),
		"materializationSha256": stringValue(source.MaterializationSHA256),
		"objectFormat":          stringValue(source.ObjectFormat),
		"statusSha256":          stringValue(source.StatusSHA256),
		"tree":                  stringValue(source.Tree),
	}
	if includeID {
		fields["id"] = stringValue(source.ID)
	}
	return object(fields)
}

func checkpointValue(checkpoint Checkpoint) wire.Value {
	return object(map[string]wire.Value{
		"id":      stringValue(checkpoint.ID),
		"version": stringValue(checkpoint.Version),
	})
}

func scopeValue(scope Scope) wire.Value {
	return object(map[string]wire.Value{
		"complete":    boolValue(scope.Complete),
		"id":          stringValue(scope.ID),
		"ticketCount": stringValue(scope.TicketCount.String()),
	})
}

func capacityClassValue(class CapacityClass) wire.Value {
	return object(map[string]wire.Value{
		"availableUnits": stringValue(class.AvailableUnits.String()),
		"id":             stringValue(class.ID),
	})
}

func capacityUseValue(use CapacityUse) wire.Value {
	return object(map[string]wire.Value{
		"classId": stringValue(use.ClassID),
		"units":   stringValue(use.Units.String()),
	})
}

func routeAlternativeValue(route RouteAlternative) wire.Value {
	return object(map[string]wire.Value{
		"id":       stringValue(route.ID),
		"requires": stringsValue(route.Requires),
	})
}

func selectionFactsValue(facts SelectionFacts) wire.Value {
	return object(map[string]wire.Value{
		"approvals":    stringValue(facts.Approvals),
		"dependencies": stringValue(facts.Dependencies),
		"holds":        stringValue(facts.Holds),
		"lease":        stringValue(facts.Lease),
	})
}

func ticketValue(ticket TicketSummary) wire.Value {
	return object(map[string]wire.Value{
		"atomicRepositoryAuthorityIds": stringsValue(ticket.AtomicRepositoryAuthorityIDs),
		"authority":                    stringValue(ticket.Authority),
		"capacityUses":                 sortedArrayValue(ticket.CapacityUses, capacityUseValue),
		"collisionGroupIds":            stringsValue(ticket.CollisionGroupIDs),
		"declaredVersion":              stringValue(ticket.DeclaredVersion),
		"dependencyTicketIds":          stringsValue(ticket.DependencyTicketIDs),
		"detailPayloadSha256":          optionalStringValue(ticket.DetailPayloadSHA256),
		"lifecycle":                    stringValue(ticket.Lifecycle),
		"queueAuthorityId":             stringValue(ticket.QueueAuthorityID),
		"rank":                         stringValue(ticket.Rank.String()),
		"repositoryAuthorityId":        stringValue(ticket.RepositoryAuthorityID),
		"routeAlternatives":            sortedArrayValue(ticket.RouteAlternatives, routeAlternativeValue),
		"selectionFacts":               selectionFactsValue(ticket.SelectionFacts),
		"ticketContentSha256":          stringValue(ticket.TicketContentSHA256),
		"ticketId":                     stringValue(ticket.TicketID),
		"ticketVersionId":              stringValue(ticket.TicketVersionID),
		"touchPaths":                   stringsValue(ticket.TouchPaths),
	})
}

func ticketVersionBody(ticket TicketSummary) wire.Value {
	return object(map[string]wire.Value{
		"declaredVersion":       stringValue(ticket.DeclaredVersion),
		"queueAuthorityId":      stringValue(ticket.QueueAuthorityID),
		"repositoryAuthorityId": stringValue(ticket.RepositoryAuthorityID),
		"ticketContentSha256":   stringValue(ticket.TicketContentSHA256),
		"ticketId":              stringValue(ticket.TicketID),
	})
}

func leaseValue(lease LeaseSummary, includeVersion bool) wire.Value {
	fields := map[string]wire.Value{
		"blocksSelection":       boolValue(lease.BlocksSelection),
		"capacityUses":          sortedArrayValue(lease.CapacityUses, capacityUseValue),
		"collisionGroupIds":     stringsValue(lease.CollisionGroupIDs),
		"holderId":              stringValue(lease.HolderID),
		"leaseId":               stringValue(lease.LeaseID),
		"lifecycle":             stringValue(lease.Lifecycle),
		"queueAuthorityId":      stringValue(lease.QueueAuthorityID),
		"repositoryAuthorityId": stringValue(lease.RepositoryAuthorityID),
		"ticketId":              stringValue(lease.TicketID),
		"ticketVersionId":       stringValue(lease.TicketVersionID),
	}
	if includeVersion {
		fields["leaseVersionId"] = stringValue(lease.LeaseVersionID)
	}
	return object(fields)
}

func collisionGroupValue(group CollisionGroup) wire.Value {
	return object(map[string]wire.Value{
		"id":              stringValue(group.ID),
		"memberTicketIds": stringsValue(group.MemberTicketIDs),
		"path":            optionalStringValue(group.Path),
		"source":          stringValue(group.Source),
	})
}

func proposalEntryValue(entry ProposalEntry) wire.Value {
	return object(map[string]wire.Value{
		"collisionGroupIds":  stringsValue(entry.CollisionGroupIDs),
		"reason":             stringValue(entry.Reason),
		"routeAlternativeId": optionalStringValue(entry.RouteAlternativeID),
		"state":              stringValue(entry.State),
		"ticketId":           stringValue(entry.TicketID),
		"ticketVersionId":    stringValue(entry.TicketVersionID),
	})
}

func snapshotValue(snapshot *Snapshot, includeID bool) wire.Value {
	fields := map[string]wire.Value{
		"accessContextId":               stringValue(snapshot.AccessContextID),
		"capacityClasses":               sortedArrayValue(snapshot.CapacityClasses, capacityClassValue),
		"checkpoint":                    checkpointValue(snapshot.Checkpoint),
		"detailRequestTicketVersionIds": stringsValue(snapshot.DetailRequestTicketVersionIDs),
		"leases":                        sortedArrayValue(snapshot.Leases, func(value LeaseSummary) wire.Value { return leaseValue(value, true) }),
		"policyId":                      stringValue(snapshot.PolicyID),
		"profile":                       stringValue(snapshot.Profile),
		"queueAuthorityId":              stringValue(snapshot.QueueAuthorityID),
		"repositoryAuthorityId":         stringValue(snapshot.RepositoryAuthorityID),
		"repositorySource":              repositorySourceValue(snapshot.RepositorySource, true),
		"scope":                         scopeValue(snapshot.Scope),
		"tickets":                       arrayValue(snapshot.Tickets, ticketValue),
	}
	if includeID {
		fields["id"] = stringValue(snapshot.ID)
	}
	return object(fields)
}

func envelopeValue(envelope *CapacityEnvelope, includeID bool) wire.Value {
	fields := map[string]wire.Value{
		"available":             sortedArrayValue(envelope.Available, capacityClassValue),
		"capabilities":          stringsValue(envelope.Capabilities),
		"profile":               stringValue(envelope.Profile),
		"repositoryAuthorityId": stringValue(envelope.RepositoryAuthorityID),
	}
	if includeID {
		fields["id"] = stringValue(envelope.ID)
	}
	return object(fields)
}

func proposalValue(proposal *Proposal, includeID bool) wire.Value {
	fields := map[string]wire.Value{
		"capacityEnvelopeId": stringValue(proposal.CapacityEnvelopeID),
		"collisionClosure":   sortedArrayValue(proposal.CollisionClosure, collisionGroupValue),
		"entries":            arrayValue(proposal.Entries, proposalEntryValue),
		"mutationAuthority":  boolValue(proposal.MutationAuthority),
		"observationId":      stringValue(proposal.ObservationID),
		"profile":            stringValue(proposal.Profile),
		"queueSourceId":      stringValue(proposal.QueueSourceID),
		"state":              stringValue(proposal.State),
		"unknowns":           stringsValue(proposal.Unknowns),
		"waveLimit":          stringValue(proposal.WaveLimit.String()),
		"waveOptimality":     stringValue(proposal.WaveOptimality),
	}
	if includeID {
		fields["id"] = stringValue(proposal.ID)
	}
	return object(fields)
}

func contentIdentity(kind, profile string, body wire.Value) string {
	hash := sha256.New()
	hash.Write([]byte(kind))
	hash.Write([]byte{0})
	hash.Write([]byte(profile))
	hash.Write([]byte{0})
	hash.Write(wire.CanonicalValue(body))
	return kind + ":sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func canonicalDocument(value wire.Value) []byte {
	result := wire.CanonicalValue(value)
	return append(result, '\n')
}

func (snapshot *Snapshot) Canonical() []byte {
	return canonicalDocument(snapshotValue(snapshot, true))
}

func (envelope *CapacityEnvelope) Canonical() []byte {
	return canonicalDocument(envelopeValue(envelope, true))
}

func (proposal *Proposal) Canonical() []byte {
	return canonicalDocument(proposalValue(proposal, true))
}

func (proposal *Proposal) refreshIdentity() {
	proposal.ID = contentIdentity("work-wave-proposal", ProposalProfile, proposalValue(proposal, false))
}

func sortedUnique(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// MarkStale turns a completed analysis into the empty drift result required by
// WQO-V0-025 and refreshes its content identity.
func (proposal *Proposal) MarkStale() {
	proposal.State = "STALE"
	proposal.Entries = []ProposalEntry{}
	proposal.WaveOptimality = "NONE"
	proposal.Unknowns = sortedUnique(append(proposal.Unknowns, UnknownCheckpointChanged))
	proposal.refreshIdentity()
}
