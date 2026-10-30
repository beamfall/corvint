//go:build unix

package workqueuev0_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/workqueue"
)

// Expected wires/preimages were frozen by a separate Python stdlib/literal
// oracle before native execution. No production Refresh call builds expectations.
// These are invented complete mutation/receipt inputs for hypothetical historical
// VALIDATED_AT records, not captures from the production UNKNOWN/EMPTY observer.
type localEvidenceScenario struct {
	Name, Policy, Snapshot, Details, Checkpoint, Envelope      string
	Observation, Proposal, ObservationCommand, ProposalCommand string
	Identities                                                 []struct {
		Label, Kind, Profile, Body, Expected string
		Bare                                 bool
	}
	ContentRecords []struct {
		TicketID, Body, SHA256 string
	}
}

type localEvidenceEvaluation struct {
	policy                              *workqueue.Policy
	snapshot                            *workqueue.Snapshot
	details                             *workqueue.DetailsDocument
	checkpoint                          *workqueue.CheckpointDocument
	envelope                            *workqueue.CapacityEnvelope
	observation                         *workqueue.Observation
	proposal                            *workqueue.Proposal
	observationCommand, proposalCommand *workqueue.CommandResult
}

func localEvidenceScenarios(t *testing.T) []localEvidenceScenario {
	t.Helper()
	raw, err := os.ReadFile("testdata/local_evidence_wires.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Oracle, Qualification string
		Scenarios             []localEvidenceScenario
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Scenarios) != 2 || fixture.Scenarios[0].Name != "benign-a" || fixture.Scenarios[1].Name != "adversarial-b" {
		t.Fatal("missing independent A/B fixtures")
	}
	return fixture.Scenarios
}

func localEvidenceSHA(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func localEvidenceExpectedIDs(t *testing.T, fixture localEvidenceScenario) map[string]string {
	t.Helper()
	ids := map[string]string{}
	for _, row := range fixture.Identities {
		if _, exists := ids[row.Label]; exists {
			t.Fatalf("duplicate oracle identity %s", row.Label)
		}
		preimage := []byte(row.Kind + "\x00" + row.Profile + "\x00" + row.Body)
		want := localEvidenceSHA(preimage)
		if !row.Bare {
			want = row.Kind + ":sha256:" + want
		}
		if want != row.Expected {
			t.Fatalf("independent preimage mismatch: %s", row.Label)
		}
		ids[row.Label] = row.Expected
	}
	return ids
}

func localEvidenceExact(t *testing.T, name string, got []byte, want string) {
	t.Helper()
	if !bytes.Equal(got, []byte(want)) {
		t.Fatalf("%s differs from frozen independent wire:\n%s", name, got)
	}
}

func localEvidenceEvaluate(t *testing.T, fixture localEvidenceScenario) *localEvidenceEvaluation {
	t.Helper()
	ids := localEvidenceExpectedIDs(t, fixture)
	policy, err := workqueue.ParsePolicy([]byte(fixture.Policy))
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	snapshot, err := workqueue.ParseSnapshot([]byte(fixture.Snapshot))
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	details, err := workqueue.ParseDetails([]byte(fixture.Details))
	if err != nil {
		t.Fatalf("details: %v", err)
	}
	checkpoint, err := workqueue.ParseCheckpoint([]byte(fixture.Checkpoint))
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	envelope, err := workqueue.ParseEnvelope([]byte(fixture.Envelope))
	if err != nil {
		t.Fatalf("envelope: %v", err)
	}
	for _, check := range []struct {
		name string
		raw  []byte
		want string
	}{
		{"policy", policy.Canonical(), fixture.Policy}, {"snapshot", snapshot.Canonical(), fixture.Snapshot},
		{"details", details.Canonical(), fixture.Details}, {"checkpoint", checkpoint.Canonical(), fixture.Checkpoint},
		{"envelope", envelope.Canonical(), fixture.Envelope},
	} {
		localEvidenceExact(t, check.name, check.raw, check.want)
	}
	if result := workqueue.ValidateSnapshot(snapshot); result.State != workqueue.StateValidated || len(result.Unknowns) != 0 {
		t.Fatalf("synthetic snapshot structure: %+v", result)
	}
	if result := workqueue.ValidateDetailCoverage(snapshot, details); result.State != workqueue.StateValidated || len(result.Unknowns) != 0 {
		t.Fatalf("synthetic detail bindings: %+v", result)
	}
	if err := workqueue.ValidateCheckpoint(checkpoint, policy, snapshot, snapshot.RepositorySource); err != nil {
		t.Fatal(err)
	}
	if checkpoint.Checkpoint != snapshot.Checkpoint || snapshot.PolicyID != policy.ID || snapshot.AccessContextID != policy.AccessContextID || snapshot.Scope.ID != policy.ScopeID || snapshot.RepositoryAuthorityID != policy.RepositoryAuthorityID || snapshot.QueueAuthorityID != policy.QueueAuthorityID {
		t.Fatal("independent policy/checkpoint tuple does not bind the snapshot")
	}
	if len(snapshot.Tickets) != 3 || len(snapshot.Leases) != 1 || len(fixture.ContentRecords) != 3 {
		t.Fatal("fixture lost ticket/content/lease witnesses")
	}
	for index, record := range fixture.ContentRecords {
		ticket := snapshot.Tickets[index]
		if localEvidenceSHA([]byte(record.Body)) != record.SHA256 || ticket.TicketID != record.TicketID || ticket.TicketContentSHA256 != record.SHA256 {
			t.Fatalf("content commitment does not bind source record %s", record.TicketID)
		}
		var item struct {
			ID, Body, Title string
			TouchPaths      []string
		}
		if err := json.Unmarshal([]byte(record.Body), &item); err != nil {
			t.Fatal(err)
		}
		if ticket.TicketID != "ticket:corvint:worklist:"+item.ID || !reflect.DeepEqual(ticket.TouchPaths, item.TouchPaths) {
			t.Fatal("source content record changed its stable identity or paths")
		}
		for _, detail := range details.Details {
			if detail.TicketID == ticket.TicketID && (detail.Payload.Body == nil || detail.Payload.Title == nil || *detail.Payload.Body != item.Body || *detail.Payload.Title != item.Title) {
				t.Fatal("detail prose does not bind the independent source content")
			}
		}
	}
	observation := identityObservation(t, fixture.Observation)
	for index := range observation.AdapterReceipts {
		receipt := &observation.AdapterReceipts[index]
		want := ids[receipt.Operation+"-receipt"]
		receipt.ID = "old self identity must be excluded"
		workqueue.RefreshReceipt(receipt)
		if receipt.ID != want {
			t.Fatalf("synthetic receipt %s identity differs", receipt.Operation)
		}
	}
	observation.ID = "old self identity must be excluded"
	workqueue.RefreshObservation(&observation)
	if observation.ID != ids["observation"] || observation.SnapshotID != snapshot.ID || observation.PolicyID != policy.ID || observation.StartCheckpoint != snapshot.Checkpoint || observation.EndCheckpoint != checkpoint.Checkpoint {
		t.Fatal("historical observation identity/checkpoint binding differs")
	}
	if workqueue.QueueSourceIdentity(policy, snapshot) != ids["queue-source"] || observation.QueueSourceID != ids["queue-source"] {
		t.Fatal("queue-source identity does not independently rederive")
	}
	if observation.State != "VALIDATED_AT" || observation.MutationState != "UNCHANGED_OBSERVED" || observation.NetworkState != "HOST_UNOBSERVED" || observation.ContainmentClass != "DARWIN_PROCESS_GROUP_UNQUALIFIED" {
		t.Fatal("hypothetical complete mutation facts or separate qualifications changed")
	}
	// VALIDATED_AT is a hypothetical fixture input here, never the result of the
	// actual production capture/reducer. The actual capture is tested separately.
	snapshot.ObservationID = observation.ID
	snapshot.QueueSourceID = observation.QueueSourceID
	snapshot.ObservationState = observation.State
	snapshot.ObservationUnknowns = append([]string(nil), observation.Unknowns...)
	proposal, err := workqueue.ProposeWave(snapshot, envelope, workqueue.CollisionClosure{Complete: true}, 2)
	if err != nil {
		t.Fatal(err)
	}
	localEvidenceExact(t, "proposal", proposal.Canonical(), fixture.Proposal)
	if proposal.ID != ids["proposal"] || proposal.ObservationID != observation.ID || proposal.State != "ELIGIBLE_AT" || proposal.MutationAuthority {
		t.Fatal("synthetic proposal lost historical/non-operative binding")
	}
	observationCommand := &workqueue.CommandResult{Observation: &observation, State: "OK"}
	proposalCommand := &workqueue.CommandResult{Proposal: proposal, State: "OK"}
	workqueue.RefreshCommandResult(observationCommand)
	workqueue.RefreshCommandResult(proposalCommand)
	localEvidenceExact(t, "observation command", observationCommand.Canonical(), fixture.ObservationCommand)
	localEvidenceExact(t, "proposal command", proposalCommand.Canonical(), fixture.ProposalCommand)
	if observationCommand.ID != ids["observation-command"] || proposalCommand.ID != ids["proposal-command"] {
		t.Fatal("command identities differ")
	}
	var command struct{ Observation json.RawMessage }
	if err := json.Unmarshal(observationCommand.Canonical(), &command); err != nil {
		t.Fatal(err)
	}
	localEvidenceExact(t, "nested observation", append(command.Observation, '\n'), fixture.Observation)
	return &localEvidenceEvaluation{policy, snapshot, details, checkpoint, envelope, &observation, proposal, observationCommand, proposalCommand}
}

type localEvidenceDecision struct {
	TicketID, State, Reason, Route string
	Groups                         []string
}

func localEvidenceDecisions(proposal *workqueue.Proposal) []localEvidenceDecision {
	result := make([]localEvidenceDecision, len(proposal.Entries))
	for index, entry := range proposal.Entries {
		route := ""
		if entry.RouteAlternativeID != nil {
			route = *entry.RouteAlternativeID
		}
		result[index] = localEvidenceDecision{entry.TicketID, entry.State, entry.Reason, route, entry.CollisionGroupIDs}
	}
	return result
}

func TestLocalProseSelectionInvariance(t *testing.T) {
	t.Run("WQO-V0-026 independently rebound prose preserves nonempty selection", func(t *testing.T) {
		fixtures := localEvidenceScenarios(t)
		benign := localEvidenceEvaluate(t, fixtures[0])
		adversarial := localEvidenceEvaluate(t, fixtures[1])
		want := []localEvidenceDecision{
			{"ticket:corvint:worklist:selected", "SELECTED", "ELIGIBLE_AT_CHECKPOINT", "route:corvint:worklist:go", []string{}},
			{"ticket:corvint:worklist:excluded", "EXCLUDED", "ROUTE_UNAVAILABLE", "", []string{}},
			{"ticket:corvint:worklist:abstained", "ABSTAINED", "UNKNOWN_AUTHORITY", "", []string{}},
		}
		for _, value := range []*localEvidenceEvaluation{benign, adversarial} {
			if got := localEvidenceDecisions(value.proposal); !reflect.DeepEqual(got, want) {
				t.Fatalf("nonempty stable decisions = %#v", got)
			}
		}
		if !reflect.DeepEqual(benign.policy, adversarial.policy) || !reflect.DeepEqual(benign.envelope, adversarial.envelope) || benign.snapshot.Scope != adversarial.snapshot.Scope || !reflect.DeepEqual(benign.snapshot.CapacityClasses, adversarial.snapshot.CapacityClasses) {
			t.Fatal("prose variants changed policy, authority/scope or capacity input")
		}
		for index := range benign.snapshot.Tickets {
			left, right := benign.snapshot.Tickets[index], adversarial.snapshot.Tickets[index]
			// Only these separately verified content commitments may differ.
			left.TicketVersionID, right.TicketVersionID = "", ""
			left.TicketContentSHA256, right.TicketContentSHA256 = "", ""
			left.DetailPayloadSHA256, right.DetailPayloadSHA256 = nil, nil
			if !reflect.DeepEqual(left, right) {
				t.Fatalf("authoritative ticket facts changed at rank %d", index+1)
			}
		}
		leftLease, rightLease := benign.snapshot.Leases[0], adversarial.snapshot.Leases[0]
		if leftLease.LeaseVersionID == rightLease.LeaseVersionID || leftLease.TicketVersionID == rightLease.TicketVersionID {
			t.Fatal("affected lease content was not rebound")
		}
		leftLease.LeaseVersionID, rightLease.LeaseVersionID = "", ""
		leftLease.TicketVersionID, rightLease.TicketVersionID = "", ""
		if !reflect.DeepEqual(leftLease, rightLease) {
			t.Fatal("prose changed lease facts")
		}
		for _, leftDetail := range benign.details.Details {
			for _, rightDetail := range adversarial.details.Details {
				if leftDetail.TicketID != rightDetail.TicketID {
					continue
				}
				left, right := leftDetail.Payload, rightDetail.Payload
				if leftDetail.TicketID == "ticket:corvint:worklist:selected" {
					if *left.Title == *right.Title || *left.Body == *right.Body {
						t.Fatal("finite pair did not change both title and body")
					}
					left.Title, right.Title, left.Body, right.Body = nil, nil, nil, nil
				}
				if !reflect.DeepEqual(left, right) {
					t.Fatal("variant changed a field outside the selected ticket title/body")
				}
			}
		}
		if benign.snapshot.Tickets[0].TicketVersionID == adversarial.snapshot.Tickets[0].TicketVersionID || benign.snapshot.ID == adversarial.snapshot.ID || benign.snapshot.RepositorySource.ID == adversarial.snapshot.RepositorySource.ID || benign.details.ID == adversarial.details.ID || benign.checkpoint.ID == adversarial.checkpoint.ID || benign.observation.ID == adversarial.observation.ID || benign.proposal.ID == adversarial.proposal.ID {
			t.Fatal("changed content reused a bound identity")
		}
		if fixtures[0].ContentRecords[0].Body == fixtures[1].ContentRecords[0].Body || !strings.Contains(fixtures[1].Details, "LOCAL_EVIDENCE_SENTINEL") {
			t.Fatal("adversarial pair lost its finite payload")
		}
		// A changed payload with old commitments must fail before selection.
		stale := bytes.Replace([]byte(fixtures[1].Details), []byte("LOCAL_EVIDENCE_SENTINEL"), []byte("LOCAL_EVIDENCE_CHANGED"), 1)
		if _, err := workqueue.ParseDetails(stale); workqueue.ErrorCode(err) != workqueue.CodeConflicted {
			t.Fatalf("stale payload commitment accepted: %v", err)
		}
		if result := workqueue.ValidateDetailCoverage(benign.snapshot, adversarial.details); result.State != workqueue.StateConflicted {
			t.Fatalf("cross-tuple details accepted: %+v", result)
		}
	})
}

func TestLocalHistoricalSerialization(t *testing.T) {
	t.Run("WQO-V0-028 synthetic historical records retain checkpoint binding", func(t *testing.T) {
		fixtures := localEvidenceScenarios(t)
		earlier := localEvidenceEvaluate(t, fixtures[0])
		oldObservationID, oldProposalID := earlier.observation.ID, earlier.proposal.ID
		oldCheckpoint := earlier.observation.EndCheckpoint
		later := localEvidenceEvaluate(t, fixtures[1])
		if later.observation.EndCheckpoint == oldCheckpoint || later.observation.ID == oldObservationID || later.proposal.ID == oldProposalID {
			t.Fatal("A/B history did not advance its immutable tuple")
		}
		if earlier.proposal.ObservationID != oldObservationID || later.proposal.ObservationID != later.observation.ID {
			t.Fatal("proposal references the wrong historical observation")
		}
		// Presenting B cannot relabel, rebind or rewrite retained A.
		workqueue.RefreshObservation(earlier.observation)
		workqueue.RefreshCommandResult(earlier.observationCommand)
		workqueue.RefreshCommandResult(earlier.proposalCommand)
		localEvidenceExact(t, "retained A observation command", earlier.observationCommand.Canonical(), fixtures[0].ObservationCommand)
		localEvidenceExact(t, "retained A proposal", earlier.proposal.Canonical(), fixtures[0].Proposal)
		localEvidenceExact(t, "retained A proposal command", earlier.proposalCommand.Canonical(), fixtures[0].ProposalCommand)
		if earlier.observation.ID != oldObservationID || earlier.proposal.ID != oldProposalID || earlier.observation.StartCheckpoint != oldCheckpoint || earlier.observation.EndCheckpoint != oldCheckpoint {
			t.Fatal("retained A ceased to be historical")
		}
		if earlier.observation.State != "VALIDATED_AT" || earlier.observation.NetworkState != "HOST_UNOBSERVED" || earlier.observation.MutationState != "UNCHANGED_OBSERVED" || earlier.observation.ContainmentClass != "DARWIN_PROCESS_GROUP_UNQUALIFIED" || earlier.proposal.MutationAuthority {
			t.Fatal("historical state or separate qualification was rewritten")
		}
	})
}
