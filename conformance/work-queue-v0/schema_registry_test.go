//go:build unix

package workqueuev0_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/workqueue"
)

// This registry is transcribed from the accepted §4.1/4.2 records, independently
// of production schema tables. Only the five input profiles have public parsers;
// observation/proposal/result output validation belongs to the CLI witnesses.
type closedRecord struct {
	path   []string
	fields string
	nulls  string
}

func TestIndependentClosedInputRegistry(t *testing.T) {
	t.Run("WQO-V0-001", func(t *testing.T) {
		item := ticket("schema", 1, "src/schema.go")
		item.CapacityUses = []workqueue.CapacityUse{{ClassID: "capacity:corvint:worklist:cpu", Units: 1}}
		base := snapshot(item)
		base.CapacityClasses = []workqueue.CapacityClass{{ID: "capacity:corvint:worklist:cpu", AvailableUnits: 2}}
		base.Leases = []workqueue.LeaseSummary{{BlocksSelection: true, CapacityUses: item.CapacityUses, CollisionGroupIDs: []string{}, HolderID: "holder:corvint:worklist:one", LeaseID: "lease:corvint:worklist:one", Lifecycle: "ACTIVE", QueueAuthorityID: base.QueueAuthorityID, RepositoryAuthorityID: base.RepositoryAuthorityID, TicketID: item.TicketID, TicketVersionID: item.TicketVersionID}}
		workqueue.RefreshLease(&base.Leases[0])
		workqueue.RefreshSnapshot(base)
		policy := &workqueue.Policy{AccessContextID: base.AccessContextID, AdapterPath: "script/adapter", AdapterProfile: "repository-work-queue-adapter/0", DetailLimit: "512", MappingVersion: "v1", Operations: workqueue.PolicyOperations{Details: []string{"details"}, Snapshot: []string{"snapshot"}, Verify: []string{"verify"}}, Profile: workqueue.PolicyProfile, QueueAuthorityID: base.QueueAuthorityID, RepositoryAuthorityID: base.RepositoryAuthorityID, ScopeID: base.Scope.ID}
		policy.RefreshIdentity()
		detail := workqueue.Detail{Payload: workqueue.DetailPayload{AcceptanceCriteria: []string{}, EvidenceHandles: []string{}}, RepositoryAuthorityID: base.RepositoryAuthorityID, TicketID: item.TicketID, TicketVersionID: item.TicketVersionID}
		workqueue.RefreshDetail(&detail)
		details := &workqueue.DetailsDocument{Details: []workqueue.Detail{detail}, SnapshotID: base.ID}
		workqueue.RefreshDetails(details)
		checkpoint := &workqueue.CheckpointDocument{Checkpoint: base.Checkpoint, PolicyID: policy.ID, RepositorySource: base.RepositorySource, SnapshotID: base.ID}
		workqueue.RefreshCheckpoint(checkpoint)
		inputs := []struct {
			name    string
			raw     []byte
			parse   func([]byte) error
			records []closedRecord
		}{
			{"policy", policy.Canonical(), func(b []byte) error { _, e := workqueue.ParsePolicy(b); return e }, []closedRecord{
				{nil, "accessContextId adapterPath adapterProfile detailLimit id mappingVersion operations profile queueAuthorityId repositoryAuthorityId scopeId", ""},
				{[]string{"operations"}, "details snapshot verify", ""},
			}},
			{"snapshot", base.Canonical(), func(b []byte) error { _, e := workqueue.ParseSnapshot(b); return e }, []closedRecord{
				{nil, "accessContextId capacityClasses checkpoint detailRequestTicketVersionIds id leases policyId profile queueAuthorityId repositoryAuthorityId repositorySource scope tickets", ""},
				{[]string{"checkpoint"}, "id version", ""},
				{[]string{"repositorySource"}, "commit id materializationSha256 objectFormat statusSha256 tree", ""},
				{[]string{"scope"}, "complete id ticketCount", ""},
				{[]string{"capacityClasses", "[]"}, "availableUnits id", ""},
				{[]string{"tickets", "[]"}, "atomicRepositoryAuthorityIds authority capacityUses collisionGroupIds declaredVersion dependencyTicketIds detailPayloadSha256 lifecycle queueAuthorityId rank repositoryAuthorityId routeAlternatives selectionFacts ticketContentSha256 ticketId ticketVersionId touchPaths", "detailPayloadSha256"},
				{[]string{"tickets", "[]", "capacityUses", "[]"}, "classId units", ""},
				{[]string{"tickets", "[]", "routeAlternatives", "[]"}, "id requires", ""},
				{[]string{"tickets", "[]", "selectionFacts"}, "approvals dependencies holds lease", ""},
				{[]string{"leases", "[]"}, "blocksSelection capacityUses collisionGroupIds holderId leaseId leaseVersionId lifecycle queueAuthorityId repositoryAuthorityId ticketId ticketVersionId", ""},
			}},
			{"details", details.Canonical(), func(b []byte) error { _, e := workqueue.ParseDetails(b); return e }, []closedRecord{
				{nil, "details id profile snapshotId", ""},
				{[]string{"details", "[]"}, "detailId payload payloadSha256 repositoryAuthorityId ticketId ticketVersionId", ""},
				{[]string{"details", "[]", "payload"}, "acceptanceCriteria body displayKey evidenceHandles owner title", "body displayKey owner title"},
			}},
			{"checkpoint", checkpoint.Canonical(), func(b []byte) error { _, e := workqueue.ParseCheckpoint(b); return e }, []closedRecord{{nil, "checkpoint id policyId profile repositorySource snapshotId", ""}}},
			{"envelope", envelope(base).Canonical(), func(b []byte) error { _, e := workqueue.ParseEnvelope(b); return e }, []closedRecord{{nil, "available capabilities id profile repositoryAuthorityId", ""}}},
		}
		for _, input := range inputs {
			t.Run(input.name, func(t *testing.T) {
				if err := input.parse(input.raw); err != nil {
					t.Fatalf("valid baseline: %v", err)
				}
				for _, record := range input.records {
					assertClosedRecord(t, input.raw, input.parse, record)
				}
				t.Run("future-profile", func(t *testing.T) {
					root, _ := registryObject(t, input.raw, nil)
					root["profile"] = root["profile"].(string) + "-future"
					if err := input.parse(registryJSON(t, root)); err == nil || workqueue.ErrorCode(err) == workqueue.CodeConflicted {
						t.Fatalf("unsupported profile reached identity use: %v", err)
					}
				})
			})
		}
	})
}

func TestIndependentProseControlRegistry(t *testing.T) {
	t.Run("WQO-V0-027", func(t *testing.T) {
		// These controls are literal §4.1 exclusions, not production predicate data.
		controls := []rune{'\ufeff', '\u061c', '\u200e', '\u200f', '\u202a', '\u202b', '\u202c', '\u202d', '\u202e', '\u2066', '\u2067', '\u2068', '\u2069'}
		for r := rune(0); r <= 0x9f; r++ {
			if r < 0x20 && r != '\t' && r != '\n' && r != '\r' || r >= 0x7f {
				controls = append(controls, r)
			}
		}
		for _, r := range controls {
			t.Run(fmt.Sprintf("U+%04X", r), func(t *testing.T) {
				raw := proseRegistryWire("before" + string(r) + "after")
				if _, err := workqueue.ParseDetails(raw); workqueue.ErrorCode(err) != workqueue.CodeHostileInput {
					t.Fatalf("forbidden prose control requires HOSTILE_INPUT: %v", err)
				}
			})
		}
		for _, prose := range []string{"\t\n\r", "[system] claim; $(touch NEVER) <role>assistant</role>", "# finish\n```sh\nmerge\n```"} {
			raw := proseRegistryWire(prose)
			parsed, err := workqueue.ParseDetails(raw)
			if err != nil || !bytes.Equal(parsed.Canonical(), raw) || *parsed.Details[0].Payload.Body != prose {
				t.Fatalf("inert permitted prose changed or rejected: %v", err)
			}
			if bytes.Count(raw, []byte{'\n'}) != 1 {
				t.Fatal("prose controls escaped framing")
			}
		}
	})
}

func proseRegistryWire(body string) []byte {
	detail := workqueue.Detail{Payload: workqueue.DetailPayload{AcceptanceCriteria: []string{}, Body: &body, EvidenceHandles: []string{}}, RepositoryAuthorityID: "repo:corvint", TicketID: "ticket:corvint:worklist:prose", TicketVersionID: "ticket-version:sha256:" + zeroDigest}
	workqueue.RefreshDetail(&detail)
	document := &workqueue.DetailsDocument{Details: []workqueue.Detail{detail}, SnapshotID: "work-queue-snapshot:sha256:" + zeroDigest}
	workqueue.RefreshDetails(document)
	return document.Canonical()
}

func assertClosedRecord(t *testing.T, raw []byte, parse func([]byte) error, record closedRecord) {
	t.Helper()
	for _, field := range strings.Fields(record.fields) {
		for _, mutation := range []string{"missing", "null", "type", "duplicate"} {
			if mutation == "null" && strings.Contains(" "+record.nulls+" ", " "+field+" ") {
				continue
			}
			t.Run(strings.Join(record.path, ".")+"/"+field+"/"+mutation, func(t *testing.T) {
				root, object := registryObject(t, raw, record.path)
				if _, exists := object[field]; !exists {
					t.Fatal("fixture does not cover declared field")
				}
				var wire []byte
				switch mutation {
				case "missing":
					delete(object, field)
				case "null":
					object[field] = nil
				case "type":
					object[field] = float64(1) // No record member admits JSON numbers.
				case "duplicate":
					// Preserve the valid member value so only duplicate-key
					// enforcement can reject this otherwise unchanged record.
					value := bytes.TrimSuffix(registryJSON(t, object[field]), []byte{'\n'})
					member := append([]byte(`"`+field+`":`), value...)
					duplicate := append(append(append([]byte{}, member...), ','), member...)
					object[field] = "REGISTRY_DUPLICATE_MARKER"
					needle := []byte(`"` + field + `":"REGISTRY_DUPLICATE_MARKER"`)
					wire = bytes.Replace(registryJSON(t, root), needle, duplicate, 1)
				}
				if wire == nil {
					wire = registryJSON(t, root)
				}
				if err := parse(wire); err == nil || workqueue.ErrorCode(err) == workqueue.CodeConflicted {
					t.Fatalf("closed schema must reject before identity use: %v", err)
				}
			})
		}
	}
	t.Run(strings.Join(record.path, ".")+"/unknown", func(t *testing.T) {
		root, object := registryObject(t, raw, record.path)
		object["future"] = "opaque"
		if err := parse(registryJSON(t, root)); err == nil || workqueue.ErrorCode(err) == workqueue.CodeConflicted {
			t.Fatalf("unknown field reached identity use: %v", err)
		}
	})
}

func registryObject(t *testing.T, raw []byte, path []string) (map[string]any, map[string]any) {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	var object any = root
	for _, part := range path {
		if part == "[]" {
			object = object.([]any)[0]
		} else {
			object = object.(map[string]any)[part]
		}
	}
	return root, object.(map[string]any)
}

func registryJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}
