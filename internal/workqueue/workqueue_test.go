package workqueue

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

const (
	testRepository = "repo:corvint"
	testQueue      = "queue:corvint:worklist"
	testZeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"
	testZeroOID    = "0000000000000000000000000000000000000000"
)

func testIdentity(kind, profile string, body wire.Value) string {
	preimage := append([]byte(kind), 0)
	preimage = append(preimage, []byte(profile)...)
	preimage = append(preimage, 0)
	preimage = append(preimage, wire.CanonicalValue(body)...)
	digest := sha256.Sum256(preimage)
	return kind + ":sha256:" + hex.EncodeToString(digest[:])
}

func testTicket(local string, rank Rank) TicketSummary {
	ticket := TicketSummary{
		AtomicRepositoryAuthorityIDs: []string{testRepository},
		Authority:                    "COMPLETE",
		CapacityUses:                 []CapacityUse{},
		CollisionGroupIDs:            []string{},
		DeclaredVersion:              "v1",
		DependencyTicketIDs:          []string{},
		Lifecycle:                    "READY",
		QueueAuthorityID:             testQueue,
		Rank:                         rank,
		RepositoryAuthorityID:        testRepository,
		RouteAlternatives: []RouteAlternative{{
			ID:       "route:corvint:worklist:" + local,
			Requires: []string{},
		}},
		SelectionFacts: SelectionFacts{
			Approvals: "CLEAR", Dependencies: "SATISFIED", Holds: "CLEAR", Lease: "ABSENT",
		},
		TicketContentSHA256: testZeroDigest,
		TicketID:            "ticket:corvint:worklist:" + local,
		TouchPaths:          []string{},
	}
	ticket.TicketVersionID = testIdentity("ticket-version", "ticket-version/0", ticketVersionBody(ticket))
	return ticket
}

func testSnapshot(tickets ...TicketSummary) *Snapshot {
	snapshot := &Snapshot{
		AccessContextID:               "access:corvint:local",
		CapacityClasses:               []CapacityClass{},
		Checkpoint:                    Checkpoint{ID: "checkpoint:corvint:worklist", Version: "v1"},
		DetailRequestTicketVersionIDs: []string{},
		Leases:                        []LeaseSummary{},
		PolicyID:                      "work-queue-policy:sha256:" + testZeroDigest,
		Profile:                       SnapshotProfile,
		QueueAuthorityID:              testQueue,
		RepositoryAuthorityID:         testRepository,
		RepositorySource: RepositorySource{
			Commit:                testZeroOID,
			MaterializationSHA256: testZeroDigest,
			ObjectFormat:          "sha1",
			StatusSHA256:          testZeroDigest,
			Tree:                  testZeroOID,
		},
		Scope:            Scope{Complete: true, ID: "scope:corvint:worklist", TicketCount: Count(len(tickets))},
		Tickets:          append([]TicketSummary(nil), tickets...),
		ObservationID:    "work-queue-observation:sha256:" + testZeroDigest,
		QueueSourceID:    "queue-source:sha256:" + testZeroDigest,
		ObservationState: StateValidated,
	}
	snapshot.RepositorySource.ID = testIdentity("repository-source", "repository-source/0", repositorySourceValue(snapshot.RepositorySource, false))
	refreshSnapshotID(snapshot)
	return snapshot
}

func refreshSnapshotID(snapshot *Snapshot) {
	snapshot.ID = testIdentity("work-queue-snapshot", SnapshotProfile, snapshotValue(snapshot, false))
}

func testEnvelope(snapshot *Snapshot, available ...CapacityClass) *CapacityEnvelope {
	envelope := &CapacityEnvelope{
		Available:             append([]CapacityClass(nil), available...),
		Capabilities:          []string{},
		Profile:               EnvelopeProfile,
		RepositoryAuthorityID: snapshot.RepositoryAuthorityID,
	}
	envelope.ID = testIdentity("work-capacity-envelope", EnvelopeProfile, envelopeValue(envelope, false))
	return envelope
}

func TestParseSnapshotMinimalAndPopulatedCanonical(t *testing.T) {
	minimal := testSnapshot()
	parsed, err := ParseSnapshot(minimal.Canonical())
	if err != nil {
		t.Fatal(err)
	}
	if string(parsed.Canonical()) != string(minimal.Canonical()) {
		t.Fatal("minimal snapshot did not round trip byte-identically")
	}

	ticket := testTicket("one", 7)
	ticket.TouchPaths = []string{"internal/a.go", "internal/dir/"}
	ticket.CapacityUses = []CapacityUse{{ClassID: "capacity:corvint:worklist:cpu", Units: 2}}
	ticket.RouteAlternatives[0].Requires = []string{"capability:corvint:worklist:go"}
	ticket.TicketVersionID = testIdentity("ticket-version", "ticket-version/0", ticketVersionBody(ticket))
	populated := testSnapshot(ticket)
	populated.CapacityClasses = []CapacityClass{{ID: "capacity:corvint:worklist:cpu", AvailableUnits: 4}}
	refreshSnapshotID(populated)
	parsed, err = ParseSnapshot(populated.Canonical())
	if err != nil {
		t.Fatal(err)
	}
	if string(parsed.Canonical()) != string(populated.Canonical()) {
		t.Fatal("populated snapshot did not round trip byte-identically")
	}
}

func TestParseSnapshotRejectsClosedObjectAndNoncanonicalBytes(t *testing.T) {
	base := testSnapshot()
	root, err := wire.Parse(base.Canonical()[:len(base.Canonical())-1])
	if err != nil {
		t.Fatal(err)
	}
	root.Obj.Keys = append(root.Obj.Keys, "zzz")
	root.Obj.Values["zzz"] = stringValue("unknown")
	unknown := canonicalDocument(root)
	delete(root.Obj.Values, "zzz")
	root.Obj.Keys = root.Obj.Keys[:len(root.Obj.Keys)-1]
	if _, err := ParseSnapshot(unknown); err == nil {
		t.Fatal("unknown key was accepted")
	}

	delete(root.Obj.Values, "profile")
	missing := canonicalDocument(root)
	if _, err := ParseSnapshot(missing); err == nil {
		t.Fatal("missing key was accepted")
	}

	noncanonical := append([]byte(" "), base.Canonical()...)
	if _, err := ParseSnapshot(noncanonical); err == nil {
		t.Fatal("noncanonical whitespace was accepted")
	}
	wrongOrder := bytes.Replace(
		base.Canonical(),
		[]byte(`{"accessContextId":"access:corvint:local","capacityClasses":[]`),
		[]byte(`{"capacityClasses":[],"accessContextId":"access:corvint:local"`),
		1,
	)
	if _, err := ParseSnapshot(wrongOrder); err == nil {
		t.Fatal("noncanonical key ordering was accepted")
	}
}

func TestParseSnapshotRejectsFourPartShortQualifiedIDs(t *testing.T) {
	base := testSnapshot()
	fourPartCheckpoint := bytes.Replace(
		base.Canonical(),
		[]byte(`"checkpoint":{"id":"checkpoint:corvint:worklist"`),
		[]byte(`"checkpoint":{"id":"checkpoint:corvint:worklist:extra"`),
		1,
	)
	if _, err := ParseSnapshot(fourPartCheckpoint); err == nil {
		t.Fatal("four-part checkpoint ID was accepted; checkpoint is a three-part kind")
	}

	fourPartScope := bytes.Replace(
		base.Canonical(),
		[]byte(`"id":"scope:corvint:worklist"`),
		[]byte(`"id":"scope:corvint:worklist:extra"`),
		1,
	)
	if _, err := ParseSnapshot(fourPartScope); err == nil {
		t.Fatal("four-part scope ID was accepted; scope is a three-part kind")
	}

	fourPartAccess := bytes.Replace(
		base.Canonical(),
		[]byte(`"accessContextId":"access:corvint:local"`),
		[]byte(`"accessContextId":"access:corvint:local:extra"`),
		1,
	)
	if _, err := ParseSnapshot(fourPartAccess); err == nil {
		t.Fatal("four-part access ID was accepted; access is a three-part kind")
	}
}

func TestParseSnapshotNonArrayTicketsIsMalformedNotInputLimit(t *testing.T) {
	base := testSnapshot()
	nonArray := bytes.Replace(base.Canonical(), []byte(`"tickets":[]`), []byte(`"tickets":{}`), 1)
	_, err := ParseSnapshot(nonArray)
	if err == nil {
		t.Fatal("non-array tickets value was accepted")
	}
	typed, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if typed.Code != CodeMalformedInput {
		t.Fatalf("non-array tickets must be MALFORMED_INPUT (a shape error), got %s", typed.Code)
	}
}

func TestParseEnvelopeIdentityAndBounds(t *testing.T) {
	envelope := testEnvelope(testSnapshot())
	parsed, err := ParseEnvelope(envelope.Canonical())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ID != envelope.ID {
		t.Fatalf("ID = %q, want %q", parsed.ID, envelope.ID)
	}
	bad := *envelope
	bad.ID = "work-capacity-envelope:sha256:" + strings.Repeat("f", 64)
	if _, err := ParseEnvelope(bad.Canonical()); err == nil {
		t.Fatal("fabricated envelope identity was accepted")
	}
	tooLarge := append([]byte{'{'}, make([]byte, maxEnvelopeBytes)...)
	if _, err := ParseEnvelope(tooLarge); err == nil {
		t.Fatal("oversize envelope was accepted")
	}
}

func TestCountAndRankGrammar(t *testing.T) {
	valid := []string{"0", "1", "2147483647"}
	for _, value := range valid {
		if _, err := ParseCount(value); err != nil {
			t.Errorf("ParseCount(%q): %v", value, err)
		}
		if _, err := ParseRank(value); err != nil {
			t.Errorf("ParseRank(%q): %v", value, err)
		}
	}
	invalid := []string{"", "00", "01", "-1", "+1", "2147483648", "1.0"}
	for _, value := range invalid {
		if _, err := ParseCount(value); err == nil {
			t.Errorf("ParseCount(%q) succeeded", value)
		}
	}
}

func TestPathGrammar(t *testing.T) {
	t.Run("WQO-V0-041", func(t *testing.T) {
		valid := []string{"a", "a/b.go", "dir/"}
		for _, value := range valid {
			if err := ValidatePath(value); err != nil {
				t.Errorf("ValidatePath(%q): %v", value, err)
			}
		}
		invalid := []string{"", "/a", "a//b", "a/./b", "a/../b", `a\b`, "a\x00b", "a\x1fb"}
		for _, value := range invalid {
			if err := ValidatePath(value); err == nil {
				t.Errorf("ValidatePath(%q) succeeded", value)
			}
		}
	})
}

func TestIdentifierGrammar(t *testing.T) {
	valid := []string{"opaque-version", "café", strings.Repeat("a", 128)}
	for _, value := range valid {
		if err := ValidateIdentifier(value); err != nil {
			t.Errorf("ValidateIdentifier(%q): %v", value, err)
		}
	}
	invalid := []string{"", strings.Repeat("a", 129), "a\x00b", "a\x7fb", "a\ufeffb", "a\u202eb"}
	for _, value := range invalid {
		if err := ValidateIdentifier(value); err == nil {
			t.Errorf("ValidateIdentifier(%q) succeeded", value)
		}
	}
}

func TestSnapshotTicketAndTouchPathBounds(t *testing.T) {
	t.Run("WQO-V0-041", func(t *testing.T) {
		snapshot := testSnapshot(testTicket("one", 1))
		snapshot.Tickets[0].TouchPaths = make([]string, maxTouchPaths+1)
		for index := range snapshot.Tickets[0].TouchPaths {
			snapshot.Tickets[0].TouchPaths[index] = "p/" + rankLocal(index)
		}
		refreshSnapshotID(snapshot)
		if _, err := ParseSnapshot(snapshot.Canonical()); err == nil {
			t.Fatal("257 touch paths were accepted")
		}

	})
	root, err := wire.Parse(testSnapshot().Canonical()[:len(testSnapshot().Canonical())-1])
	if err != nil {
		t.Fatal(err)
	}
	tickets := make([]wire.Value, maxArrayItems+1)
	for index := range tickets {
		tickets[index] = object(map[string]wire.Value{})
	}
	root.Obj.Values["tickets"] = wire.Value{Kind: wire.KindArray, Arr: tickets}
	if _, err := ParseSnapshot(canonicalDocument(root)); err == nil {
		t.Fatal("10,001 tickets were accepted")
	}
}

func TestIDGrammar(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"ticket:corvint:worklist:one", true},
		{"ticket:Corvint-1:q_1:x.1", true},
		{"ticket:corvint:worklist", false},
		{"ticket::worklist:one", false},
		{"ticket:corvint:work/list:one", false},
		{"ticket:corvint:worklist:-one", false},
	}
	for _, test := range tests {
		_, _, _, valid := splitQualifiedID(test.name, "ticket")
		if valid != test.valid {
			t.Errorf("splitQualifiedID(%q) valid = %v", test.name, valid)
		}
	}
	if _, valid := splitRepositoryID("repo:corvint"); !valid {
		t.Fatal("valid repository ID rejected")
	}
	if _, _, valid := splitQueueID("queue:corvint:worklist"); !valid {
		t.Fatal("valid queue ID rejected")
	}
}

func TestContentIdentityPreimages(t *testing.T) {
	ticket := testTicket("identity", 1)
	if ticket.TicketVersionID != testIdentity("ticket-version", "ticket-version/0", ticketVersionBody(ticket)) {
		t.Fatal("ticket version preimage differs from reference")
	}
	lease := LeaseSummary{
		BlocksSelection: false, CapacityUses: []CapacityUse{}, CollisionGroupIDs: []string{},
		HolderID: "holder:corvint:worklist:h", LeaseID: "lease:corvint:worklist:l", Lifecycle: "ACTIVE",
		QueueAuthorityID: testQueue, RepositoryAuthorityID: testRepository,
		TicketID: ticket.TicketID, TicketVersionID: ticket.TicketVersionID,
	}
	lease.LeaseVersionID = testIdentity("lease-version", "lease-version/0", leaseValue(lease, false))
	if lease.LeaseVersionID != contentIdentity("lease-version", "lease-version/0", leaseValue(lease, false)) {
		t.Fatal("lease version preimage differs from reference")
	}
	snapshot := testSnapshot(ticket)
	if snapshot.ID != testIdentity("work-queue-snapshot", SnapshotProfile, snapshotValue(snapshot, false)) {
		t.Fatal("snapshot preimage differs from reference")
	}
	envelope := testEnvelope(snapshot)
	if envelope.ID != testIdentity("work-capacity-envelope", EnvelopeProfile, envelopeValue(envelope, false)) {
		t.Fatal("envelope preimage differs from reference")
	}
}
