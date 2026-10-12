package wire

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func neutralStableMap(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "protocol", "cem-1.0", "stable", "neutral-criterion", "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestCEMV1083PlainCriterionWire proves the plain criterion form parses with
// no Tasks member, round-trips losslessly with exactly its four keys, and
// identifies over criterionIndex, criterionSha256 and evidenceId.
func TestCEMV1083PlainCriterionWire(t *testing.T) {
	d, err := ParseStable(neutralStableMap(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Criteria) != 1 || !d.Criteria[0].Plain() || d.Criteria[0].EvidenceID != d.Change.Evidence[0].ID {
		t.Fatalf("criteria = %+v, want one plain criterion on the case evidence", d.Criteria)
	}
	encoded, err := json.Marshal(d.Criteria[0])
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]any
	if err := json.Unmarshal(encoded, &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 4 {
		t.Fatalf("plain criterion encodes as %s, want four keys", encoded)
	}
	whole, err := EncodeStable(d)
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseStable(whole)
	if err != nil {
		t.Fatal(err)
	}
	d.original, again.original = nil, nil
	if !reflect.DeepEqual(d, again) {
		t.Fatal("lossy plain stable round trip")
	}
}

// TestCEMV1083PlainCriterionRefusals proves the closed-form selection and the
// plain graph rules: a Tasks member beside plain members is a missing Tasks
// member, evidenceId beside the full Tasks form and any foreign key are unknown,
// and two plain criteria on one evidence span are a duplicate reference.
func TestCEMV1083PlainCriterionRefusals(t *testing.T) {
	base := neutralStableMap(t)
	tasks := map[string]any{"ticketId": "T-1", "acceptanceRevision": "1", "acceptanceSha256": "1", "captureSha256": "2", "verificationSha256": "3",
		"claimTicketSha256": "4", "snapshotHeadReceiptSha256": "5", "snapshotHeadArtifactSha256": "6"}
	for _, test := range []struct {
		name   string
		mutate func(m map[string]any, c map[string]any)
		code   string
	}{
		{"plain-with-ticket-id", func(_ map[string]any, c map[string]any) { c["ticketId"] = "T-1" }, cemcode.MissingField},
		{"tasks-with-evidence-id", func(_ map[string]any, c map[string]any) {
			for k, v := range tasks {
				c[k] = v
			}
		}, cemcode.UnknownField},
		{"plain-unknown-key", func(_ map[string]any, c map[string]any) { c["provider"] = "x" }, cemcode.UnknownField},
		{"plain-missing-evidence-id", func(_ map[string]any, c map[string]any) { delete(c, "evidenceId") }, cemcode.MissingField},
		{"plain-invalid-evidence-id", func(_ map[string]any, c map[string]any) { c["evidenceId"] = "evidence:sha256:XYZ" }, cemcode.InvalidField},
		{"plain-duplicate-evidence", func(m map[string]any, c map[string]any) {
			second := StableCriterion{CriterionIndex: 1, CriterionSha256: c["criterionSha256"].(string), EvidenceID: c["evidenceId"].(string)}
			second.ID = stableCriterionIdentity(second)
			m["criterionBindings"] = append(m["criterionBindings"].([]any), map[string]any{"id": second.ID, "criterionIndex": 1, "criterionSha256": second.CriterionSha256, "evidenceId": second.EvidenceID})
		}, "duplicate-reference"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal(base, &m); err != nil {
				t.Fatal(err)
			}
			test.mutate(m, m["criterionBindings"].([]any)[0].(map[string]any))
			raw, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseStable(raw); cemcode.CodeOf(err) != test.code {
				t.Fatalf("ParseStable error %v, want %s", err, test.code)
			}
		})
	}
}
