package main

import (
	"fmt"
	"slices"
	"sort"
	"testing"

	"github.com/Beamfall/corvint/internal/workqueue"
)

// Request selection must use semantic ticket order, which deliberately opposes
// digest order here. Returned records are re-bound to the supplied request set.
func newDetailRequestFixture(limit string, requested []int) captureFixture {
	f := newCaptureFixture()
	f.policy.DetailLimit = limit
	f.policy.RefreshIdentity()
	seed := f.snapshot.Tickets[0]
	f.snapshot.Tickets = nil
	for i := 0; i < 4; i++ {
		ticket := seed
		ticket.TicketID = fmt.Sprintf("ticket:corvint:worklist:request-%d", i)
		workqueue.RefreshTicket(&ticket)
		f.snapshot.Tickets = append(f.snapshot.Tickets, ticket)
	}
	sort.Slice(f.snapshot.Tickets, func(i, j int) bool {
		return f.snapshot.Tickets[i].TicketVersionID > f.snapshot.Tickets[j].TicketVersionID
	})
	for i := range f.snapshot.Tickets {
		f.snapshot.Tickets[i].Rank = workqueue.Rank(i + 1)
	}
	f.snapshot.Tickets[3].Lifecycle = "ACTIVE"
	f.snapshot.PolicyID = f.policy.ID
	f.snapshot.Scope.TicketCount = 4
	f.checkpoint.PolicyID = f.policy.ID
	for i := range f.receipts {
		f.receipts[i].PolicyID = f.policy.ID
		workqueue.RefreshReceipt(&f.receipts[i])
	}
	setDetailRequests(&f, requested)
	return f
}

func setDetailRequests(f *captureFixture, requested []int) {
	f.snapshot.DetailRequestTicketVersionIDs = []string{}
	f.details.Details = []workqueue.Detail{}
	for _, index := range requested {
		ticket := f.snapshot.Tickets[index]
		f.snapshot.DetailRequestTicketVersionIDs = append(f.snapshot.DetailRequestTicketVersionIDs, ticket.TicketVersionID)
		if ticket.DetailPayloadSHA256 == nil {
			continue
		}
		detail := workqueue.Detail{RepositoryAuthorityID: ticket.RepositoryAuthorityID, TicketID: ticket.TicketID, TicketVersionID: ticket.TicketVersionID, Payload: workqueue.DetailPayload{AcceptanceCriteria: []string{}, EvidenceHandles: []string{}}}
		workqueue.RefreshDetail(&detail)
		f.details.Details = append(f.details.Details, detail)
	}
	sort.Strings(f.snapshot.DetailRequestTicketVersionIDs)
	workqueue.RefreshSnapshot(f.snapshot)
	f.details.SnapshotID = f.snapshot.ID
	workqueue.RefreshDetails(f.details)
	f.checkpoint.SnapshotID = f.snapshot.ID
	workqueue.RefreshCheckpoint(f.checkpoint)
}

func TestWorkDetailRequestDerivation(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-001 detail request derivation", func(t *testing.T) {
		for _, c := range []struct {
			name, limit string
			requested   []int
			null        bool
			want        string
		}{
			{"active requested", "4", []int{0, 1, 2, 3}, false, "CONFLICTED"},
			{"null requested", "4", []int{0, 1, 2}, true, "CONFLICTED"},
			{"omitted eligible", "4", []int{0, 2}, false, "CONFLICTED"},
			{"over limit", "1", []int{0, 1}, false, "CONFLICTED"},
			{"later replaces earlier", "1", []int{2}, false, "CONFLICTED"},
			{"zero limit violated", "0", []int{0}, false, "CONFLICTED"},
			{"exact three", "4", []int{0, 1, 2}, false, "VALIDATED_AT"},
			{"rank before digest", "1", []int{0}, false, "VALIDATED_AT"},
			{"zero limit", "0", nil, false, "VALIDATED_AT"},
			{"skip null", "4", []int{0, 2}, true, "VALIDATED_AT"},
		} {
			t.Run(c.name, func(t *testing.T) {
				f := newDetailRequestFixture(c.limit, c.requested)
				if c.null {
					f.snapshot.Tickets[1].DetailPayloadSHA256 = nil
					setDetailRequests(&f, c.requested)
				}
				assertDetailFixtureCoherent(t, f, c.null && c.want == "CONFLICTED")
				got := validateWorkCapture(f.policy, f.opening, f.closing, f.snapshot, f.details, f.checkpoint, f.receipts, f.closure)
				if got.State != c.want {
					t.Fatalf("state = %s; want %s (codes %v)", got.State, c.want, got.Unknowns)
				}
				assertObservationIdentity(t, got)
				if c.want == "CONFLICTED" {
					assertDetailRequestEmptyProposal(t, f, got)
				}
			})
		}
	})
	t.Run("WQO-V0-012 conflict preserves incomplete evidence", func(t *testing.T) {
		f := newDetailRequestFixture("4", []int{0, 1, 2, 3})
		assertDetailFixtureCoherent(t, f, false)
		f.opening.manifest.complete = false
		f.closing.manifest.complete = false
		f.closure.Complete = false
		f.closure.Unknowns = []string{workqueue.UnknownCollisionClosureIncomplete}
		f.checkpoint.Checkpoint.Version = "drift"
		workqueue.RefreshCheckpoint(f.checkpoint)
		got := validateWorkCapture(f.policy, f.opening, f.closing, f.snapshot, f.details, f.checkpoint, f.receipts, f.closure)
		if got.State != "CONFLICTED" {
			t.Fatalf("state = %s; want CONFLICTED", got.State)
		}
		for _, code := range []string{"CHECKPOINT_CHANGED", "COLLISION_CLOSURE_INCOMPLETE", "SOURCE_UNQUALIFIED"} {
			if !slices.Contains(got.Unknowns, code) {
				t.Fatalf("missing %s in %v", code, got.Unknowns)
			}
		}
		assertDetailRequestEmptyProposal(t, f, got)
	})
	t.Run("WQO-V0-031 missing detail coverage", func(t *testing.T) {
		f := newDetailRequestFixture("4", []int{0, 1, 2})
		f.details.Details = f.details.Details[:2]
		workqueue.RefreshDetails(f.details)
		got := validateWorkCapture(f.policy, f.opening, f.closing, f.snapshot, f.details, f.checkpoint, f.receipts, f.closure)
		if got.State != "PARTIAL" || !slices.Contains(got.Unknowns, "DETAIL_MISSING") {
			t.Fatalf("got %s %v", got.State, got.Unknowns)
		}
		assertDetailRequestEmptyProposal(t, f, got)
	})
}

func TestWorkDetailRequestUnusablePolicy(t *testing.T) {
	t.Parallel()
	for _, limit := range []string{"", "01", "-1", "one", "513", "2147483648"} {
		t.Run(limit, func(t *testing.T) {
			f := newDetailRequestFixture(limit, []int{0, 1, 2})
			got := validateWorkCapture(f.policy, f.opening, f.closing, f.snapshot, f.details, f.checkpoint, f.receipts, f.closure)
			if got.State != "UNKNOWN" || !slices.Contains(got.Unknowns, "ADAPTER_INVALID") {
				t.Fatalf("got %s %v", got.State, got.Unknowns)
			}
			assertDetailRequestEmptyProposal(t, f, got)
		})
	}
}

func assertDetailFixtureCoherent(t *testing.T, f captureFixture, missing bool) {
	t.Helper()
	if _, err := workqueue.ParsePolicy(f.policy.Canonical()); err != nil {
		t.Fatal(err)
	}
	if _, err := workqueue.ParseSnapshot(f.snapshot.Canonical()); err != nil {
		t.Fatal(err)
	}
	if _, err := workqueue.ParseDetails(f.details.Canonical()); err != nil {
		t.Fatal(err)
	}
	if _, err := workqueue.ParseCheckpoint(f.checkpoint.Canonical()); err != nil {
		t.Fatal(err)
	}
	if got := workqueue.ValidateSnapshot(f.snapshot); got.State != "VALIDATED_AT" {
		t.Fatalf("unrelated snapshot defect: %+v", got)
	}
	want := "VALIDATED_AT"
	if missing {
		want = "PARTIAL"
	}
	if got := workqueue.ValidateDetailCoverage(f.snapshot, f.details); got.State != want {
		t.Fatalf("unrelated coverage defect: %+v", got)
	}
	if err := workqueue.ValidateCheckpoint(f.checkpoint, f.policy, f.snapshot, f.closing.source); err != nil {
		t.Fatal(err)
	}
}

func assertDetailRequestEmptyProposal(t *testing.T, f captureFixture, observation *workqueue.Observation) {
	t.Helper()
	f.snapshot.ObservationID = observation.ID
	f.snapshot.QueueSourceID = observation.QueueSourceID
	f.snapshot.ObservationState = observation.State
	f.snapshot.ObservationUnknowns = append([]string(nil), observation.Unknowns...)
	envelope := &workqueue.CapacityEnvelope{Available: []workqueue.CapacityClass{}, Capabilities: []string{}, Profile: workqueue.EnvelopeProfile, RepositoryAuthorityID: f.policy.RepositoryAuthorityID}
	workqueue.RefreshEnvelope(envelope)
	proposal, err := workqueue.ProposeWave(f.snapshot, envelope, f.closure, 4)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.State != "EMPTY" || len(proposal.Entries) != 0 {
		t.Fatalf("nonvalidated request produced proposal: %+v", proposal)
	}
}
