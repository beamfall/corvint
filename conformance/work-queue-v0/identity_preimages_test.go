//go:build unix

package workqueuev0_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/workqueue"
)

// These complete literal preimages and digests were frozen using an independent
// standard-library oracle before running the consumer. Refresh calls below are
// the system under test, never constructors for expected identities.
type identityGolden struct {
	Name, Kind, Profile, Body, Preimage, Expected, SelfField, Wire, RawSHA256 string
}

func identityGoldens(t *testing.T) map[string]identityGolden {
	t.Helper()
	raw, err := os.ReadFile("testdata/identity_preimages.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []identityGolden
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 15 {
		t.Fatalf("identity registry has %d rows; want all 15", len(rows))
	}
	result := make(map[string]identityGolden, len(rows))
	for _, row := range rows {
		if _, exists := result[row.Name]; exists {
			t.Fatalf("duplicate identity row %s", row.Name)
		}
		result[row.Name] = row
	}
	return result
}

func independentIdentitySHA(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func decodeIdentityBody(t *testing.T, raw string, value any) {
	t.Helper()
	if err := json.Unmarshal([]byte(raw), value); err != nil {
		t.Fatal(err)
	}
}

func identityReceipt(t *testing.T, raw string) workqueue.AdapterReceipt {
	t.Helper()
	var value struct {
		workqueue.AdapterReceipt
		ExitCode    *string `json:"exitCode"`
		StderrBytes string  `json:"stderrBytes"`
		StdoutBytes string  `json:"stdoutBytes"`
	}
	decodeIdentityBody(t, raw, &value)
	count := func(raw string) workqueue.Count {
		n, err := strconv.ParseUint(raw, 10, 31)
		if err != nil {
			t.Fatal(err)
		}
		return workqueue.Count(n)
	}
	value.AdapterReceipt.StderrBytes = count(value.StderrBytes)
	value.AdapterReceipt.StdoutBytes = count(value.StdoutBytes)
	if value.ExitCode != nil {
		n := count(*value.ExitCode)
		value.AdapterReceipt.ExitCode = &n
	}
	return value.AdapterReceipt
}

func identityObservation(t *testing.T, raw string) workqueue.Observation {
	t.Helper()
	var value workqueue.Observation
	decodeIdentityBody(t, raw, &value)
	var envelope struct {
		Receipts []json.RawMessage `json:"adapterReceipts"`
	}
	decodeIdentityBody(t, raw, &envelope)
	value.AdapterReceipts = nil
	for _, receipt := range envelope.Receipts {
		value.AdapterReceipts = append(value.AdapterReceipts, identityReceipt(t, string(receipt)))
	}
	return value
}

func identityConsumerResult(t *testing.T, row identityGolden, all map[string]identityGolden) (string, []byte) {
	t.Helper()
	// A wrong preexisting self-ID exposes accidental self inclusion. Reference
	// identities come from the frozen fixtures and must remain in parent bodies.
	const trap = "self-id-must-be-excluded"
	switch row.Name {
	case "policy":
		var value workqueue.Policy
		decodeIdentityBody(t, row.Body, &value)
		value.ID = trap
		value.RefreshIdentity()
		return value.ID, value.Canonical()
	case "repository-source":
		var value workqueue.RepositorySource
		decodeIdentityBody(t, row.Body, &value)
		value.ID = trap
		workqueue.RefreshRepositorySource(&value)
		return value.ID, nil
	case "ticket-version":
		var value workqueue.TicketSummary
		decodeIdentityBody(t, row.Body, &value)
		value.TicketVersionID = trap
		workqueue.RefreshTicket(&value)
		return value.TicketVersionID, nil
	case "lease-version":
		var value workqueue.LeaseSummary
		decodeIdentityBody(t, row.Body, &value)
		value.LeaseVersionID = trap
		workqueue.RefreshLease(&value)
		return value.LeaseVersionID, nil
	case "detail-payload":
		var value workqueue.DetailPayload
		decodeIdentityBody(t, row.Body, &value)
		return workqueue.DetailPayloadDigest(value), nil
	case "detail-record":
		var value workqueue.Detail
		decodeIdentityBody(t, row.Body, &value)
		value.DetailID = trap
		workqueue.RefreshDetail(&value)
		if value.PayloadSHA256 != all["detail-payload"].Expected {
			t.Fatalf("nested detail payload identity = %s", value.PayloadSHA256)
		}
		return value.DetailID, nil
	case "snapshot":
		value, err := workqueue.ParseSnapshot([]byte(row.Wire))
		if err != nil {
			t.Fatal(err)
		}
		value.ID = trap
		workqueue.RefreshSnapshot(value)
		return value.ID, value.Canonical()
	case "queue-source":
		policy, err := workqueue.ParsePolicy([]byte(all["policy"].Wire))
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := workqueue.ParseSnapshot([]byte(all["snapshot"].Wire))
		if err != nil {
			t.Fatal(err)
		}
		return workqueue.QueueSourceIdentity(policy, snapshot), nil
	case "details":
		var value workqueue.DetailsDocument
		decodeIdentityBody(t, row.Body, &value)
		value.ID = trap
		workqueue.RefreshDetails(&value)
		return value.ID, value.Canonical()
	case "checkpoint":
		var value workqueue.CheckpointDocument
		decodeIdentityBody(t, row.Body, &value)
		value.ID = trap
		workqueue.RefreshCheckpoint(&value)
		return value.ID, value.Canonical()
	case "receipt":
		value := identityReceipt(t, row.Body)
		value.ID = trap
		workqueue.RefreshReceipt(&value)
		return value.ID, nil
	case "observation":
		value := identityObservation(t, row.Body)
		value.ID = trap
		workqueue.RefreshObservation(&value)
		return value.ID, nil
	case "envelope":
		value, err := workqueue.ParseEnvelope([]byte(row.Wire))
		if err != nil {
			t.Fatal(err)
		}
		value.ID = trap
		workqueue.RefreshEnvelope(value)
		return value.ID, value.Canonical()
	case "proposal":
		snapshot, err := workqueue.ParseSnapshot([]byte(all["snapshot"].Wire))
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := workqueue.ParseEnvelope([]byte(all["envelope"].Wire))
		if err != nil {
			t.Fatal(err)
		}
		observation := identityObservation(t, all["observation"].Wire)
		snapshot.ObservationID = observation.ID
		snapshot.QueueSourceID = observation.QueueSourceID
		snapshot.ObservationState = observation.State
		snapshot.ObservationUnknowns = observation.Unknowns
		value, err := workqueue.ProposeWave(snapshot, envelope, workqueue.CollisionClosure{Complete: true}, 1)
		if err != nil {
			t.Fatal(err)
		}
		return value.ID, value.Canonical()
	case "command-result":
		observation := identityObservation(t, all["observation"].Wire)
		value := workqueue.CommandResult{ID: trap, Observation: &observation, State: "OK"}
		workqueue.RefreshCommandResult(&value)
		return value.ID, value.Canonical()
	default:
		t.Fatalf("no consumer check for golden %q", row.Name)
		return "", nil
	}
}

func TestIndependentIdentityPreimages(t *testing.T) {
	t.Run("WQO-V0-002", func(t *testing.T) {
		all := identityGoldens(t)
		for _, name := range []string{"policy", "repository-source", "snapshot", "queue-source", "ticket-version", "lease-version", "detail-payload", "detail-record", "details", "checkpoint", "receipt", "observation", "envelope", "proposal", "command-result"} {
			row, exists := all[name]
			if !exists {
				t.Fatalf("missing identity golden %s", name)
			}
			t.Run(name, func(t *testing.T) {
				if row.Preimage != row.Kind+"\x00"+row.Profile+"\x00"+row.Body {
					t.Fatal("literal preimage does not match its complete domain-separated body")
				}
				var body map[string]any
				decodeIdentityBody(t, row.Body, &body)
				canonical, err := json.Marshal(body)
				if err != nil || string(canonical) != row.Body {
					t.Fatalf("golden body is not canonical: %v", err)
				}
				if _, exists := body[row.SelfField]; exists {
					t.Fatal("self-ID was included in preimage")
				}
				wantDigest := strings.TrimPrefix(row.Expected, row.Kind+":sha256:")
				if got := independentIdentitySHA([]byte(row.Preimage)); got != wantDigest {
					t.Fatalf("independent SHA-256 = %s; frozen digest = %s", got, wantDigest)
				}
				if independentIdentitySHA([]byte(row.Preimage+"\n")) == wantDigest {
					t.Fatal("terminal transport LF entered the content preimage")
				}
				got, raw := identityConsumerResult(t, row, all)
				if got != row.Expected {
					t.Fatalf("consumer ID = %s; independent golden = %s", got, row.Expected)
				}
				if raw != nil && !bytes.Equal(raw, []byte(row.Wire)) {
					t.Fatalf("consumer canonical bytes differ from complete golden\ngot %s\nwant %s", raw, row.Wire)
				}
				if independentIdentitySHA([]byte(row.Wire)) != row.RawSHA256 || independentIdentitySHA([]byte(strings.TrimSuffix(row.Wire, "\n"))) == row.RawSHA256 {
					t.Fatal("raw hash must include exactly one terminal LF")
				}
			})
		}
	})
}

func TestIndependentIdentitySelfInclusionRejected(t *testing.T) {
	t.Run("WQO-V0-002", func(t *testing.T) {
		all := identityGoldens(t)
		parsers := map[string]func([]byte) error{
			"policy": func(raw []byte) error { _, err := workqueue.ParsePolicy(raw); return err },
			"snapshot": func(raw []byte) error {
				value, err := workqueue.ParseSnapshot(raw)
				if err != nil {
					return err
				}
				// Snapshot grammar parsing deliberately precedes semantic and identity validation.
				result := workqueue.ValidateSnapshot(value)
				if result.State != workqueue.StateValidated {
					return fmt.Errorf("snapshot validation: %s", result.State)
				}
				return nil
			},
			"details":    func(raw []byte) error { _, err := workqueue.ParseDetails(raw); return err },
			"checkpoint": func(raw []byte) error { _, err := workqueue.ParseCheckpoint(raw); return err },
			"envelope":   func(raw []byte) error { _, err := workqueue.ParseEnvelope(raw); return err },
		}
		for name, parse := range parsers {
			t.Run(name, func(t *testing.T) {
				row := all[name]
				if err := parse([]byte(row.Wire)); err != nil {
					t.Fatalf("valid independent golden rejected: %v", err)
				}
				var body map[string]any
				decodeIdentityBody(t, row.Wire, &body)
				body["id"] = row.Kind + ":sha256:" + independentIdentitySHA([]byte(row.Kind+"\x00"+row.Profile+"\x00"+strings.TrimSuffix(row.Wire, "\n")))
				raw, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				if err := parse(append(raw, '\n')); err == nil {
					t.Fatal("identity including its own field was accepted")
				}
			})
		}
	})
}

func TestIndependentIdentityRawStatusBinding(t *testing.T) {
	t.Run("WQO-V0-002", func(t *testing.T) {
		row := identityGoldens(t)["repository-source"]
		var clean workqueue.RepositorySource
		decodeIdentityBody(t, row.Body, &clean)
		const emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		if clean.StatusSHA256 != emptySHA256 || independentIdentitySHA(nil) != emptySHA256 {
			t.Fatal("clean raw status must hash zero bytes")
		}
		// Byte-binding unit evidence only: production status capture is verified
		// by the runner suite, not by this independently authored byte oracle.
		seen := map[string]bool{}
		for _, raw := range []string{"", " M src/a.go\x00", " M src/a.go\n", " A src/a.go\x00"} {
			var body map[string]any
			decodeIdentityBody(t, row.Body, &body)
			body["statusSha256"] = independentIdentitySHA([]byte(raw))
			canonical, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			want := row.Kind + ":sha256:" + independentIdentitySHA(append([]byte(row.Kind+"\x00"+row.Profile+"\x00"), canonical...))
			value := clean
			value.StatusSHA256 = body["statusSha256"].(string)
			workqueue.RefreshRepositorySource(&value)
			if value.ID != want || seen[value.ID] {
				t.Fatalf("raw status %q identity = %s; want unique %s", raw, value.ID, want)
			}
			seen[value.ID] = true
		}
	})
}
