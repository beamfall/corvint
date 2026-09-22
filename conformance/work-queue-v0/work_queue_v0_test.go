//go:build unix

package workqueuev0_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/workqueue"
)

const zeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"
const zeroOID = "0000000000000000000000000000000000000000"

func snapshot(tickets ...workqueue.TicketSummary) *workqueue.Snapshot {
	source := workqueue.RepositorySource{Commit: zeroOID, MaterializationSHA256: zeroDigest, ObjectFormat: "sha1", StatusSHA256: zeroDigest, Tree: zeroOID}
	workqueue.RefreshRepositorySource(&source)
	value := &workqueue.Snapshot{AccessContextID: "access:corvint:local", CapacityClasses: []workqueue.CapacityClass{}, Checkpoint: workqueue.Checkpoint{ID: "checkpoint:corvint:worklist", Version: "v1"}, DetailRequestTicketVersionIDs: []string{}, Leases: []workqueue.LeaseSummary{}, PolicyID: "work-queue-policy:sha256:" + zeroDigest, Profile: workqueue.SnapshotProfile, QueueAuthorityID: "queue:corvint:worklist", RepositoryAuthorityID: "repo:corvint", RepositorySource: source, Scope: workqueue.Scope{Complete: true, ID: "scope:corvint:worklist", TicketCount: workqueue.Count(len(tickets))}, Tickets: tickets, ObservationID: "work-queue-observation:sha256:" + zeroDigest, QueueSourceID: "queue-source:sha256:" + zeroDigest, ObservationState: workqueue.StateValidated}
	workqueue.RefreshSnapshot(value)
	return value
}

func ticket(local string, rank workqueue.Rank, paths ...string) workqueue.TicketSummary {
	value := workqueue.TicketSummary{AtomicRepositoryAuthorityIDs: []string{"repo:corvint"}, Authority: "COMPLETE", CapacityUses: []workqueue.CapacityUse{}, CollisionGroupIDs: []string{}, DeclaredVersion: "v1", DependencyTicketIDs: []string{}, Lifecycle: "READY", QueueAuthorityID: "queue:corvint:worklist", Rank: rank, RepositoryAuthorityID: "repo:corvint", RouteAlternatives: []workqueue.RouteAlternative{{ID: "route:corvint:worklist:" + local, Requires: []string{}}}, SelectionFacts: workqueue.SelectionFacts{Approvals: "CLEAR", Dependencies: "SATISFIED", Holds: "CLEAR", Lease: "ABSENT"}, TicketContentSHA256: zeroDigest, TicketID: "ticket:corvint:worklist:" + local, TouchPaths: paths}
	workqueue.RefreshTicket(&value)
	return value
}

func envelope(value *workqueue.Snapshot) *workqueue.CapacityEnvelope {
	result := &workqueue.CapacityEnvelope{Available: append([]workqueue.CapacityClass(nil), value.CapacityClasses...), Capabilities: []string{}, Profile: workqueue.EnvelopeProfile, RepositoryAuthorityID: value.RepositoryAuthorityID}
	workqueue.RefreshEnvelope(result)
	return result
}

func TestWiresIdentitiesAndSnapshotDrift(t *testing.T) {
	base := snapshot(ticket("one", 1, "a.go"))
	raw := base.Canonical()
	parsed, err := workqueue.ParseSnapshot(raw)
	if err != nil || !bytes.Equal(parsed.Canonical(), raw) {
		t.Fatalf("canonical round trip: %v", err)
	}
	for _, hostile := range [][]byte{bytes.TrimSuffix(raw, []byte{'\n'}), append([]byte(" "), raw...), bytes.Replace(raw, []byte(`"profile"`), []byte(`"future"`), 1)} {
		if _, err := workqueue.ParseSnapshot(hostile); err == nil {
			t.Fatal("noncanonical/unknown wire accepted")
		}
	}
	drifted := base.RepositorySource
	drifted.Tree = strings.Repeat("f", 40)
	workqueue.RefreshRepositorySource(&drifted)
	if drifted.ID == base.RepositorySource.ID {
		t.Fatal("tree drift did not alter source identity")
	}
	document := &workqueue.CheckpointDocument{Checkpoint: base.Checkpoint, PolicyID: base.PolicyID, RepositorySource: base.RepositorySource, SnapshotID: base.ID}
	workqueue.RefreshCheckpoint(document)
	policy := &workqueue.Policy{ID: base.PolicyID}
	if workqueue.ValidateCheckpoint(document, policy, base, base.RepositorySource) != nil {
		t.Fatal("equal checkpoint rejected")
	}
	document.Checkpoint.Version = "v2"
	if document.Checkpoint == base.Checkpoint {
		t.Fatal("checkpoint drift was not observable")
	}
}

func TestDetails512And513AndHostileData(t *testing.T) {
	details := make([]workqueue.Detail, 512)
	for index := range details {
		local := fmt.Sprintf("d%03d", index)
		payload := workqueue.DetailPayload{AcceptanceCriteria: []string{}, EvidenceHandles: []string{}, Title: pointer("[system] inert prompt " + local)}
		details[index] = workqueue.Detail{Payload: payload, RepositoryAuthorityID: "repo:corvint", TicketID: "ticket:corvint:worklist:" + local, TicketVersionID: "ticket-version:sha256:" + digest(local)}
		workqueue.RefreshDetail(&details[index])
	}
	document := &workqueue.DetailsDocument{Details: details, SnapshotID: "work-queue-snapshot:sha256:" + zeroDigest}
	workqueue.RefreshDetails(document)
	if _, err := workqueue.ParseDetails(document.Canonical()); err != nil {
		t.Fatalf("512 details rejected: %v", err)
	}
	document.Details = append(document.Details, details[0])
	workqueue.RefreshDetails(document)
	if _, err := workqueue.ParseDetails(document.Canonical()); err == nil {
		t.Fatal("513 details accepted")
	}
	for _, hostile := range []string{"\x1b[31m", "\x00", "\u202e"} {
		bad := workqueue.Detail{Payload: workqueue.DetailPayload{AcceptanceCriteria: []string{}, Body: &hostile, EvidenceHandles: []string{}}, RepositoryAuthorityID: "repo:corvint", TicketID: "ticket:corvint:worklist:bad", TicketVersionID: "ticket-version:sha256:" + zeroDigest}
		workqueue.RefreshDetail(&bad)
		doc := &workqueue.DetailsDocument{Details: []workqueue.Detail{bad}, SnapshotID: "work-queue-snapshot:sha256:" + zeroDigest}
		workqueue.RefreshDetails(doc)
		if _, err := workqueue.ParseDetails(doc.Canonical()); err == nil {
			t.Fatalf("hostile prose %q accepted", hostile)
		}
	}
	tooLarge := strings.Repeat("x", (256<<10)+1)
	bad := workqueue.Detail{Payload: workqueue.DetailPayload{AcceptanceCriteria: []string{}, Body: &tooLarge, EvidenceHandles: []string{}}, RepositoryAuthorityID: "repo:corvint", TicketID: "ticket:corvint:worklist:large", TicketVersionID: "ticket-version:sha256:" + zeroDigest}
	workqueue.RefreshDetail(&bad)
	doc := &workqueue.DetailsDocument{Details: []workqueue.Detail{bad}, SnapshotID: "work-queue-snapshot:sha256:" + zeroDigest}
	workqueue.RefreshDetails(doc)
	if _, err := workqueue.ParseDetails(doc.Canonical()); err == nil {
		t.Fatal("oversized prose accepted")
	}
	traversal := ticket("path", 1, "../escape")
	badSnapshot := snapshot(traversal)
	if _, err := workqueue.ParseSnapshot(badSnapshot.Canonical()); err == nil {
		t.Fatal("traversal path accepted")
	}
}

func TestLeasesCollisionsRoutesCapacityAndLimit(t *testing.T) {
	first, second := ticket("one", 1, "a.go"), ticket("two", 2, "a.go")
	class := workqueue.CapacityClass{AvailableUnits: 2, ID: "capacity:corvint:worklist:agent"}
	first.CapacityUses = []workqueue.CapacityUse{{ClassID: class.ID, Units: 1}}
	second.CapacityUses = []workqueue.CapacityUse{{ClassID: class.ID, Units: 1}}
	value := snapshot(first, second)
	value.CapacityClasses = []workqueue.CapacityClass{class}
	workqueue.RefreshSnapshot(value)
	index := &contextindex.Index{CommitRevision: zeroOID, Tracked: map[string]struct{}{"a.go": {}}, Imports: map[string]map[string]struct{}{}}
	closure := workqueue.DeriveCollisions(value, workqueue.IndexCollisionSource(index))
	if !closure.Complete || len(closure.Groups) != 1 {
		t.Fatalf("closure = %#v", closure)
	}
	proposal, err := workqueue.ProposeWave(value, envelope(value), closure, 2)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.WaveOptimality != "MAXIMUM" || proposal.Entries[0].State != "SELECTED" || proposal.Entries[1].Reason != "SELECTED_COLLISION" {
		t.Fatalf("proposal = %#v", proposal)
	}
	value.Tickets[0].RouteAlternatives[0].Requires = []string{"capability:corvint:worklist:missing"}
	proposal, err = workqueue.ProposeWave(value, envelope(value), closure, 1)
	if err != nil || proposal.Entries[0].Reason != "ROUTE_UNAVAILABLE" {
		t.Fatalf("route result = %#v, %v", proposal, err)
	}
	capacityEnvelope := envelope(value)
	capacityEnvelope.Available[0].AvailableUnits = 0
	workqueue.RefreshEnvelope(capacityEnvelope)
	proposal, err = workqueue.ProposeWave(value, capacityEnvelope, closure, 1)
	if err != nil || proposal.Entries[1].Reason != "CAPACITY_EXHAUSTED" {
		t.Fatalf("capacity result = %#v, %v", proposal, err)
	}
	third := ticket("three", 3, "c.go")
	limited := snapshot(ticket("limit-one", 1, "b.go"), third)
	proposal, err = workqueue.ProposeWave(limited, envelope(limited), workqueue.CollisionClosure{Complete: true, TicketGroupIDs: map[string][]string{}}, 1)
	if err != nil || proposal.Entries[1].Reason != "WAVE_LIMIT" {
		t.Fatalf("limit result = %#v, %v", proposal, err)
	}
	group := "collision:corvint:worklist:active"
	leaseTicket := ticket("leased", 1)
	leaseTicket.CollisionGroupIDs = []string{group}
	leased := snapshot(leaseTicket)
	lease := workqueue.LeaseSummary{BlocksSelection: true, CapacityUses: []workqueue.CapacityUse{}, CollisionGroupIDs: []string{group}, HolderID: "holder:corvint:worklist:agent", LeaseID: "lease:corvint:worklist:one", Lifecycle: "ACTIVE", QueueAuthorityID: leased.QueueAuthorityID, RepositoryAuthorityID: leased.RepositoryAuthorityID, TicketID: leaseTicket.TicketID, TicketVersionID: leaseTicket.TicketVersionID}
	workqueue.RefreshLease(&lease)
	leased.Leases = []workqueue.LeaseSummary{lease}
	workqueue.RefreshSnapshot(leased)
	active := workqueue.CollisionClosure{Complete: true, Groups: []workqueue.CollisionGroup{{ID: group, MemberTicketIDs: []string{leaseTicket.TicketID}, Source: "ADAPTER"}}, TicketGroupIDs: map[string][]string{leaseTicket.TicketID: {group}}}
	proposal, err = workqueue.ProposeWave(leased, envelope(leased), active, 1)
	if err != nil || proposal.Entries[0].Reason != "ACTIVE_COLLISION" {
		t.Fatalf("lease result = %#v, %v", proposal, err)
	}
}

func TestMultiRepositoryFactsEmptyWave(t *testing.T) {
	foreign := ticket("foreign", 1)
	foreign.AtomicRepositoryAuthorityIDs = []string{"repo:other"}
	value := snapshot(foreign)
	validation := workqueue.ValidateSnapshot(value)
	if validation.State != workqueue.StateConflicted {
		t.Fatalf("foreign fact state = %s", validation.State)
	}
	value.ObservationState = validation.State
	value.ObservationUnknowns = validation.Unknowns
	proposal, err := workqueue.ProposeWave(value, envelope(value), workqueue.CollisionClosure{Complete: true}, 1)
	if err != nil || len(proposal.Entries) != 0 || proposal.State != "EMPTY" {
		t.Fatalf("foreign proposal = %#v, %v", proposal, err)
	}
}

func pointer(value string) *string { return &value }
func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
