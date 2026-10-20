package wire

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func externalStableMap(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "protocol", "cem-1.0", "stable", "external-criterion", "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestCEMV1330ExternalCriterionWire proves the external form parses with no
// evidenceId and no Tasks member, encodes exactly its six keys, identifies
// over its non-id members and round-trips losslessly (CEM-V1-330, 332).
func TestCEMV1330ExternalCriterionWire(t *testing.T) {
	d, err := ParseStable(externalStableMap(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Criteria) != 1 {
		t.Fatalf("criteria = %+v", d.Criteria)
	}
	c := d.Criteria[0]
	if !c.External() || c.Plain() || c.Tasks() || c.ExternalRepository != "requirements" || c.ExternalSpan == nil || c.ExternalSpan.Start >= c.ExternalSpan.End {
		t.Fatalf("criterion = %+v, want one external criterion", c)
	}
	if got := stableCriterionIdentity(c); got != c.ID {
		t.Fatalf("identity %s, want %s", got, c.ID)
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]any
	if err := json.Unmarshal(encoded, &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 6 {
		t.Fatalf("external criterion encodes as %s, want six keys", encoded)
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
		t.Fatal("lossy external stable round trip")
	}
}

// TestCEMV1331ExternalCriterionRefusals proves the closed external selection
// and graph rules: a missing or foreign member, an invalid name, OID or span,
// and a Tasks member beside external members each refuse with the closed
// code order, and two external criteria on one span are a duplicate
// reference (CEM-V1-330..332).
func TestCEMV1331ExternalCriterionRefusals(t *testing.T) {
	base := externalStableMap(t)
	for _, test := range []struct {
		name   string
		mutate func(m map[string]any, c map[string]any)
		code   string
	}{
		{"missing-blob-oid", func(_ map[string]any, c map[string]any) { delete(c, "externalBlobOid") }, cemcode.MissingField},
		{"missing-repository", func(_ map[string]any, c map[string]any) { delete(c, "externalRepository") }, cemcode.MissingField},
		{"with-evidence-id", func(m map[string]any, c map[string]any) {
			c["evidenceId"] = m["evidence"].([]any)[0].(map[string]any)["id"]
		}, cemcode.UnknownField},
		{"with-ticket-id", func(_ map[string]any, c map[string]any) { c["ticketId"] = "T-1" }, cemcode.MissingField},
		{"foreign-key", func(_ map[string]any, c map[string]any) { c["locator"] = "x" }, cemcode.UnknownField},
		{"span-foreign-key", func(_ map[string]any, c map[string]any) {
			c["externalSpan"].(map[string]any)["line"] = 1
		}, cemcode.UnknownField},
		{"leading-period-name", func(_ map[string]any, c map[string]any) { c["externalRepository"] = ".git" }, cemcode.InvalidField},
		{"long-name", func(_ map[string]any, c map[string]any) { c["externalRepository"] = strings.Repeat("a", 65) }, cemcode.InvalidField},
		{"uppercase-oid", func(_ map[string]any, c map[string]any) {
			c["externalBlobOid"] = strings.ToUpper(c["externalBlobOid"].(string))
		}, cemcode.InvalidField},
		{"negative-start", func(_ map[string]any, c map[string]any) {
			c["externalSpan"].(map[string]any)["start"] = -1
		}, "invalid-json"},
		{"inverted-span", func(_ map[string]any, c map[string]any) {
			s := c["externalSpan"].(map[string]any)
			s["start"], s["end"] = s["end"], s["start"]
		}, cemcode.InvalidField},
		{"span-not-object", func(_ map[string]any, c map[string]any) { c["externalSpan"] = []any{1, 2} }, cemcode.InvalidField},
		{"duplicate-span", func(m map[string]any, c map[string]any) {
			second := StableCriterion{CriterionIndex: 1, CriterionSha256: c["criterionSha256"].(string), ExternalRepository: c["externalRepository"].(string), ExternalBlobOid: c["externalBlobOid"].(string)}
			s := c["externalSpan"].(map[string]any)
			second.ExternalSpan = &StableExternalSpan{Start: int64(s["start"].(float64)), End: int64(s["end"].(float64))}
			second.ID = stableCriterionIdentity(second)
			m["criterionBindings"] = append(m["criterionBindings"].([]any), map[string]any{"id": second.ID, "criterionIndex": 1, "criterionSha256": second.CriterionSha256,
				"externalRepository": second.ExternalRepository, "externalBlobOid": second.ExternalBlobOid, "externalSpan": s})
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

// TestCEMV1331PlainAndTasksIdentityUnchanged proves the additive members do
// not move existing identities: the plain public case still identifies as
// published (CEM-V1-331).
func TestCEMV1331PlainAndTasksIdentityUnchanged(t *testing.T) {
	d, err := ParseStable(neutralStableMap(t))
	if err != nil {
		t.Fatal(err)
	}
	c := d.Criteria[0]
	if c.External() || stableCriterionIdentity(c) != c.ID {
		t.Fatalf("plain criterion identity moved: %+v", c)
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "external") {
		t.Fatalf("plain criterion encodes external members: %s", encoded)
	}
}
