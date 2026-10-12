package wire

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// stableMemberCase is one shared CEM-V1-093..095 vector. The portable
// consumer reads the same file, so both consumers agree on every code.
type stableMemberCase struct {
	Name   string                     `json:"name"`
	Code   string                     `json:"code"`
	Set    map[string]json.RawMessage `json:"set"`
	Delete []string                   `json:"delete"`
}

func stableMemberCases(t *testing.T) []stableMemberCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "interop", "cem01-go", "testdata", "stable-map-members", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []stableMemberCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) < 40 {
		t.Fatalf("shared vectors = %d, want at least 40", len(file.Cases))
	}
	return file.Cases
}

func (c stableMemberCase) apply(t *testing.T, base []byte) []byte {
	t.Helper()
	var root map[string]json.RawMessage
	if err := json.Unmarshal(base, &root); err != nil {
		t.Fatal(err)
	}
	for k, v := range c.Set {
		root[k] = v
	}
	for _, k := range c.Delete {
		delete(root, k)
	}
	out, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestCEMV1093StableMemberVectors runs every shared vector through the
// native parser: accepts parse and round-trip losslessly, refusals return the
// recorded code (missing-field, then unknown-field, then invalid-field) and
// the optional members are checked only after every required member and
// before the reference graph.
func TestCEMV1093StableMemberVectors(t *testing.T) {
	base := neutralStableMap(t)
	for _, c := range stableMemberCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			d, err := ParseStable(c.apply(t, base))
			got := ""
			if err != nil {
				got = cemcode.CodeOf(err)
			}
			want := c.Code
			if got != want {
				t.Fatalf("ParseStable code %q (%v), want %q", got, err, want)
			}
			if want != "" {
				return
			}
			encoded, err := EncodeStable(d)
			if err != nil {
				t.Fatal(err)
			}
			again, err := ParseStable(encoded)
			if err != nil {
				t.Fatal(err)
			}
			d.original, again.original = nil, nil
			if !reflect.DeepEqual(d, again) {
				t.Fatal("lossy stable round trip of the optional members")
			}
		})
	}
}

// TestCEMV1093ProvenanceDecoded proves a present provenance decodes field
// for field, an absent one stays nil and is not emitted, and provenance is not
// part of the reference set a result echoes.
func TestCEMV1093ProvenanceDecoded(t *testing.T) {
	base := neutralStableMap(t)
	d, err := ParseStable(base)
	if err != nil {
		t.Fatal(err)
	}
	if d.Provenance != nil || d.Role != "" || d.EffectiveRole() != StableRoleChange {
		t.Fatalf("native map provenance=%v role=%q, want absent and change", d.Provenance, d.Role)
	}
	encoded, err := EncodeStable(d)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["provenance"]; ok {
		t.Fatal("encoder emitted an absent provenance")
	}
	if _, ok := root["role"]; ok {
		t.Fatal("encoder emitted an absent role")
	}
	c := stableMemberCase{Set: map[string]json.RawMessage{"provenance": json.RawMessage(`{"sourceSpec":"cem/0.3","sourceBlobOid":"0123456789abcdef0123456789abcdef01234567","sourceSha256":"` + stableTestDigest + `","bindCommit":"89abcdef0123456789abcdef0123456789abcdef","converter":"corvint-cem-convert/1.0"}`)}}
	d, err = ParseStable(c.apply(t, base))
	if err != nil {
		t.Fatal(err)
	}
	want := StableProvenance{"cem/0.3", "0123456789abcdef0123456789abcdef01234567", stableTestDigest, "89abcdef0123456789abcdef0123456789abcdef", "corvint-cem-convert/1.0"}
	if d.Provenance == nil || *d.Provenance != want {
		t.Fatalf("provenance = %+v, want %+v", d.Provenance, want)
	}
	for k := range d.StableReferences() {
		if k == "provenance" || k == "role" {
			t.Fatalf("reference echo carries %s", k)
		}
	}
}

// stableTestDigest is an arbitrary well-formed SHA-256 value.
const stableTestDigest = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

// TestCEMV1094ProducerOnlyEmpty proves the closed producer-only list is
// exactly the four reference arrays and that a map emitting all four empty
// parses with an empty reference set.
func TestCEMV1094ProducerOnlyEmpty(t *testing.T) {
	if want := []string{"criterionBindings", "runnerReceipts", "criterionLinks", "artifacts"}; !reflect.DeepEqual(StableProducerOnlyKeys, want) {
		t.Fatalf("producer-only keys = %v, want %v", StableProducerOnlyKeys, want)
	}
	set := map[string]json.RawMessage{}
	for _, k := range StableProducerOnlyKeys {
		set[k] = json.RawMessage(`[]`)
	}
	d, err := ParseStable(stableMemberCase{Set: set}.apply(t, neutralStableMap(t)))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Criteria)+len(d.Receipts)+len(d.Links)+len(d.Artifacts) != 0 || len(d.Change.Hunks) == 0 || len(d.Change.Evidence) == 0 {
		t.Fatalf("producer-only empty map decoded %d/%d/%d/%d references, %d hunks, %d evidence", len(d.Criteria), len(d.Receipts), len(d.Links), len(d.Artifacts), len(d.Change.Hunks), len(d.Change.Evidence))
	}
	// Removing any one producer-only member is still a missing member.
	for _, k := range StableProducerOnlyKeys {
		_, err := ParseStable(stableMemberCase{Set: set, Delete: []string{k}}.apply(t, neutralStableMap(t)))
		if cemcode.CodeOf(err) != cemcode.MissingField {
			t.Fatalf("without %s: %v, want missing-field", k, err)
		}
	}
}

// TestCEMV1095Role proves every role value parses and round-trips, absence
// reads as change, and the role is not part of the echoed reference set.
func TestCEMV1095Role(t *testing.T) {
	for _, role := range []string{StableRoleChange, StableRoleBindChore, StableRoleSealChore} {
		raw, _ := json.Marshal(role)
		d, err := ParseStable(stableMemberCase{Set: map[string]json.RawMessage{"role": raw}}.apply(t, neutralStableMap(t)))
		if err != nil {
			t.Fatal(err)
		}
		if d.Role != role || d.EffectiveRole() != role {
			t.Fatalf("role = %q effective %q, want %q", d.Role, d.EffectiveRole(), role)
		}
		encoded, err := EncodeStable(d)
		if err != nil {
			t.Fatal(err)
		}
		var root map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &root); err != nil {
			t.Fatal(err)
		}
		if string(root["role"]) != string(raw) {
			t.Fatalf("encoded role %s, want %s", root["role"], raw)
		}
	}
}

// TestCEMV1093SchemaMembers proves the public stable schema and the parser
// name the same required, optional, provenance and role members.
func TestCEMV1093SchemaMembers(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "protocol", "cem-1.0", "stable", "stable.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		AdditionalProperties *bool                      `json:"additionalProperties"`
		Required             []string                   `json:"required"`
		Properties           map[string]json.RawMessage `json:"properties"`
		Defs                 map[string]struct {
			Type                 string                     `json:"type"`
			AdditionalProperties *bool                      `json:"additionalProperties"`
			Required             []string                   `json:"required"`
			Properties           map[string]json.RawMessage `json:"properties"`
			Enum                 []string                   `json:"enum"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	if schema.AdditionalProperties == nil || *schema.AdditionalProperties || !reflect.DeepEqual(schema.Required, StableRequiredRootKeys) {
		t.Fatalf("schema root required=%v additionalProperties=%v", schema.Required, schema.AdditionalProperties)
	}
	if len(schema.Properties) != len(StableRequiredRootKeys)+len(StableOptionalRootKeys) {
		t.Fatalf("schema root properties = %d", len(schema.Properties))
	}
	for _, k := range StableOptionalRootKeys {
		if _, ok := schema.Properties[k]; !ok {
			t.Fatalf("schema lacks optional %s", k)
		}
	}
	p := schema.Defs["provenance"]
	if p.Type != "object" || p.AdditionalProperties == nil || *p.AdditionalProperties || !reflect.DeepEqual(p.Required, StableProvenanceKeys) || len(p.Properties) != len(StableProvenanceKeys) {
		t.Fatalf("schema provenance = %+v", p)
	}
	if r := schema.Defs["role"]; !reflect.DeepEqual(r.Enum, []string{StableRoleChange, StableRoleBindChore, StableRoleSealChore}) {
		t.Fatalf("schema role enum = %v", r.Enum)
	}
}
