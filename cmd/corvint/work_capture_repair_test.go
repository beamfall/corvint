package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/workqueue"
)

type captureFixture struct {
	policy           *workqueue.Policy
	opening, closing workSource
	snapshot         *workqueue.Snapshot
	details          *workqueue.DetailsDocument
	checkpoint       *workqueue.CheckpointDocument
	receipts         []workqueue.AdapterReceipt
	closure          workqueue.CollisionClosure
}

func newCaptureFixture() captureFixture {
	policy := &workqueue.Policy{AccessContextID: "access:corvint:local", AdapterPath: "script/queue", AdapterProfile: "repository-work-queue-adapter/0", DetailLimit: "512", MappingVersion: "v1", Operations: workqueue.PolicyOperations{Snapshot: []string{"snapshot"}, Details: []string{"details"}, Verify: []string{"verify"}}, Profile: workqueue.PolicyProfile, QueueAuthorityID: "queue:corvint:worklist", RepositoryAuthorityID: "repo:corvint", ScopeID: "scope:corvint:worklist"}
	policy.RefreshIdentity()
	source := workqueue.RepositorySource{Commit: strings.Repeat("0", 40), Tree: strings.Repeat("0", 40), ObjectFormat: "sha1", StatusSHA256: workqueue.SHA256Hex(nil), MaterializationSHA256: workqueue.SHA256Hex(nil)}
	workqueue.RefreshRepositorySource(&source)
	ticket := workqueue.TicketSummary{AtomicRepositoryAuthorityIDs: []string{"repo:corvint"}, Authority: "COMPLETE", CapacityUses: []workqueue.CapacityUse{}, CollisionGroupIDs: []string{}, DeclaredVersion: "v1", DependencyTicketIDs: []string{}, Lifecycle: "READY", QueueAuthorityID: policy.QueueAuthorityID, Rank: 1, RepositoryAuthorityID: policy.RepositoryAuthorityID, RouteAlternatives: []workqueue.RouteAlternative{}, SelectionFacts: workqueue.SelectionFacts{Approvals: "CLEAR", Dependencies: "SATISFIED", Holds: "CLEAR", Lease: "ABSENT"}, TicketContentSHA256: workqueue.SHA256Hex(nil), TicketID: "ticket:corvint:worklist:one", TouchPaths: []string{}}
	workqueue.RefreshTicket(&ticket)
	detail := workqueue.Detail{RepositoryAuthorityID: policy.RepositoryAuthorityID, TicketID: ticket.TicketID, TicketVersionID: ticket.TicketVersionID, Payload: workqueue.DetailPayload{AcceptanceCriteria: []string{}, EvidenceHandles: []string{}}}
	workqueue.RefreshDetail(&detail)
	ticket.DetailPayloadSHA256 = &detail.PayloadSHA256
	snapshot := &workqueue.Snapshot{AccessContextID: policy.AccessContextID, CapacityClasses: []workqueue.CapacityClass{}, Checkpoint: workqueue.Checkpoint{ID: "checkpoint:corvint:worklist", Version: "v1"}, DetailRequestTicketVersionIDs: []string{ticket.TicketVersionID}, Leases: []workqueue.LeaseSummary{}, PolicyID: policy.ID, Profile: workqueue.SnapshotProfile, QueueAuthorityID: policy.QueueAuthorityID, RepositoryAuthorityID: policy.RepositoryAuthorityID, RepositorySource: source, Scope: workqueue.Scope{Complete: true, ID: policy.ScopeID, TicketCount: 1}, Tickets: []workqueue.TicketSummary{ticket}}
	workqueue.RefreshSnapshot(snapshot)
	details := &workqueue.DetailsDocument{SnapshotID: snapshot.ID, Details: []workqueue.Detail{detail}}
	workqueue.RefreshDetails(details)
	checkpoint := &workqueue.CheckpointDocument{Checkpoint: snapshot.Checkpoint, PolicyID: policy.ID, RepositorySource: source, SnapshotID: snapshot.ID}
	workqueue.RefreshCheckpoint(checkpoint)
	receipts := []workqueue.AdapterReceipt{}
	for _, op := range []string{"snapshot", "details", "verify"} {
		zero := workqueue.Count(0)
		r := workqueue.AdapterReceipt{Operation: op, State: "PASSED", PolicyID: policy.ID, ExitCode: &zero, ExecutableQualification: "EXACT"}
		workqueue.RefreshReceipt(&r)
		receipts = append(receipts, r)
	}
	return captureFixture{policy, workSource{source: source, manifest: workManifest{complete: true, monitoredComplete: true}}, workSource{source: source, manifest: workManifest{complete: true, monitoredComplete: true}}, snapshot, details, checkpoint, receipts, workqueue.CollisionClosure{Complete: true}}
}

func TestWorkCaptureStateRepair(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-014", func(t *testing.T) {
		cases := []struct {
			name    string
			changes []string
			state   string
			codes   []string
		}{
			{"qualification only", nil, "VALIDATED_AT", nil},
			{"missing", []string{"missing"}, "PARTIAL", []string{"DETAIL_MISSING"}},
			{"missing closure", []string{"missing", "closure"}, "PARTIAL", []string{"COLLISION_CLOSURE_INCOMPLETE", "DETAIL_MISSING"}},
			{"drift closure", []string{"drift", "closure"}, "STALE", []string{"CHECKPOINT_CHANGED", "COLLISION_CLOSURE_INCOMPLETE"}},
			{"mutation drift missing", []string{"mutation", "drift", "missing"}, "UNKNOWN", []string{"CHECKPOINT_CHANGED", "DETAIL_MISSING", "MUTATION_DETECTED"}},
			{"conflict mutation drift", []string{"conflict", "mutation", "drift"}, "CONFLICTED", []string{"CHECKPOINT_CHANGED", "MUTATION_DETECTED"}},
			{"closure", []string{"closure"}, "UNKNOWN", []string{"COLLISION_CLOSURE_INCOMPLETE"}},
			{"foreign field", []string{"foreign"}, "UNKNOWN", []string{"MULTI_REPO_UNSUPPORTED"}},
			{"foreign tuple conflict", []string{"foreign", "tuple"}, "CONFLICTED", []string{"MULTI_REPO_UNSUPPORTED"}},
			{"checkpoint binding", []string{"checkpoint binding"}, "CONFLICTED", nil},
			{"source identity", []string{"source identity"}, "CONFLICTED", []string{"CHECKPOINT_CHANGED"}},
			{"failed receipt drift", []string{"failed", "drift"}, "UNKNOWN", []string{"ADAPTER_INVALID", "CHECKPOINT_CHANGED"}},
			{"incomplete receipt missing", []string{"incomplete", "missing"}, "UNKNOWN", []string{"ADAPTER_INVALID", "DETAIL_MISSING"}},
			{"missing receipt", []string{"missing receipt"}, "UNKNOWN", []string{"ADAPTER_INVALID"}},
			{"incomplete manifest", []string{"incomplete manifest"}, "UNKNOWN", []string{"SOURCE_UNQUALIFIED"}},
			{"incomplete manifest missing", []string{"incomplete manifest", "missing"}, "PARTIAL", []string{"DETAIL_MISSING", "SOURCE_UNQUALIFIED"}},
			{"incomplete changed manifest", []string{"incomplete manifest", "observed change", "drift"}, "UNKNOWN", []string{"CHECKPOINT_CHANGED", "MUTATION_DETECTED", "SOURCE_UNQUALIFIED"}},
			{"source qualification failure", []string{"qualification failure"}, "UNKNOWN", []string{"SOURCE_UNQUALIFIED"}},
			{"unqualified executable", []string{"unqualified executable"}, "VALIDATED_AT", []string{"EXECUTABLE_IDENTITY_UNQUALIFIED"}},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				f := newCaptureFixture()
				for _, change := range c.changes {
					switch change {
					case "missing":
						f.details.Details = []workqueue.Detail{}
					case "closure":
						f.closure.Complete = false
						f.closure.Unknowns = []string{workqueue.UnknownCollisionClosureIncomplete, workqueue.UnknownCollisionClosureIncomplete}
					case "drift":
						f.checkpoint.Checkpoint.Version = "v2"
					case "mutation":
						f.closing.manifest.digest = "changed"
					case "conflict":
						f.snapshot.PolicyID = "work-queue-policy:sha256:" + strings.Repeat("1", 64)
						workqueue.RefreshSnapshot(f.snapshot)
						f.details.SnapshotID = f.snapshot.ID
						f.checkpoint.SnapshotID = f.snapshot.ID
					case "foreign":
						f.details.Details[0].RepositoryAuthorityID = "repo:foreign"
					case "tuple":
						f.details.Details[0].TicketID = "ticket:foreign:worklist:other"
					case "checkpoint binding":
						f.checkpoint.PolicyID = "work-queue-policy:sha256:" + strings.Repeat("1", 64)
					case "source identity":
						f.checkpoint.RepositorySource.ID = "repository-source:sha256:" + strings.Repeat("1", 64)
					case "failed":
						f.receipts[0].State = "FAILED"
					case "incomplete":
						f.receipts[0].State = "INCOMPLETE"
					case "incomplete manifest":
						f.opening.manifest.complete = false
						f.closing.manifest.complete = false
					case "observed change":
						f.closing.manifest.changed = true
						f.closing.manifest.monitoredComplete = false
					case "qualification failure":
						f.closing.manifest.qualificationFailed = true
					case "unqualified executable":
						f.receipts[0].ExecutableQualification = "UNQUALIFIED"
					case "missing receipt":
						f.receipts = f.receipts[:2]
					}
				}
				for i := range f.details.Details {
					workqueue.RefreshDetail(&f.details.Details[i])
				}
				workqueue.RefreshDetails(f.details)
				workqueue.RefreshCheckpoint(f.checkpoint)
				got := validateWorkCapture(f.policy, f.opening, f.closing, f.snapshot, f.details, f.checkpoint, f.receipts, f.closure)
				codes := append([]string{"CONTAINMENT_UNQUALIFIED", "MUTATION_ENFORCEMENT_UNQUALIFIED", "NETWORK_UNOBSERVED"}, c.codes...)
				sort.Strings(codes)
				if got.State != c.state || !reflect.DeepEqual(got.Unknowns, codes) {
					t.Fatalf("got state=%s codes=%v; want %s %v", got.State, got.Unknowns, c.state, codes)
				}
				wantMutation := "UNCHANGED_OBSERVED"
				if !f.opening.manifest.complete || !f.closing.manifest.complete {
					wantMutation = "UNKNOWN"
				}
				if f.closing.manifest.changed || f.closing.manifest.digest != "" {
					wantMutation = "CHANGED"
				}
				if got.MutationState != wantMutation {
					t.Fatalf("mutation=%s; want %s", got.MutationState, wantMutation)
				}
				assertObservationIdentity(t, got)
			})
		}
	})
}

func TestWorkCommandErrorRepair(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-027", func(t *testing.T) {
		for _, c := range []struct{ input, want string }{{"HOSTILE_INPUT", "HOSTILE_INPUT"}, {"INPUT_LIMIT", "INPUT_LIMIT"}, {"CONFLICTED", "MALFORMED_INPUT"}, {"MALFORMED_INPUT", "MALFORMED_INPUT"}} {
			var output bytes.Buffer
			code := workCommandError(&workqueue.Error{Code: c.input, Message: "hostile-secret\u202e"})
			if exit := emitWorkError(&output, code); exit != 2 {
				t.Fatal(exit)
			}
			var result struct {
				ErrorCode             string
				State                 string
				Observation, Proposal any
			}
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.ErrorCode != c.want || result.State != "ERROR" || result.Observation != nil || result.Proposal != nil || bytes.Contains(output.Bytes(), []byte("hostile-secret")) {
				t.Fatalf("wrong closed error %s", output.Bytes())
			}
		}
	})
}

// WQO-V0-015/032: an expired source command establishes inability, never a
// refusal attributed to source facts that were not acquired.
func TestWorkSourceCommandExpiryIsInputLimit(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-015 source expiry mapping", func(t *testing.T) {
		for _, cause := range []error{context.DeadlineExceeded, exec.ErrWaitDelay} {
			if got := workSourceCommandError(context.Background(), cause, io.Discard); got != "INPUT_LIMIT" {
				t.Fatalf("opening source expiry %v = %s", cause, got)
			}
			if got := workClosingCommandError(workSource{qualificationError: cause}); got != "INPUT_LIMIT" {
				t.Fatalf("closing source expiry %v = %s", cause, got)
			}
		}
	})
}

func assertObservationIdentity(t *testing.T, observation *workqueue.Observation) {
	t.Helper()
	// Re-extract the emitted body and independently hash the literal identity domain.
	command := &workqueue.CommandResult{Observation: observation, State: "OK"}
	var document map[string]any
	if err := json.Unmarshal(command.Canonical(), &document); err != nil {
		t.Fatal(err)
	}
	body := document["observation"].(map[string]any)
	delete(body, "id")
	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(body); err != nil {
		t.Fatal(err)
	}
	preimage := append([]byte("work-queue-observation\x00work-queue-observation/0\x00"), bytes.TrimSuffix(canonical.Bytes(), []byte{'\n'})...)
	digest := sha256.Sum256(preimage)
	want := "work-queue-observation:sha256:" + hex.EncodeToString(digest[:])
	if observation.ID != want {
		t.Fatalf("observation ID=%s; independently want %s", observation.ID, want)
	}
}
