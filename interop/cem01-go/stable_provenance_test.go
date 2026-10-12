package main

// CEM-V1-093..095 portable conformance: the shared vectors under
// testdata/stable-map-members return the same codes as the native parser, and
// a map whose producer-only members are all empty verifies ACCEPT with every
// authority axis NOT_OBSERVED, unchanged by provenance or role.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

type stableMemberCase struct {
	Name   string                     `json:"name"`
	Code   string                     `json:"code"`
	Set    map[string]json.RawMessage `json:"set"`
	Delete []string                   `json:"delete"`
}

func stableMemberCases(t *testing.T) []stableMemberCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "stable-map-members", "cases.json"))
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

func neutralMapBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(neutralStableRoot(t), "neutral-criterion", "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestCEMV1093PortableStableMemberVectors is the portable side of the shared
// vectors: the wire decode, then the reference graph, as the native parser.
func TestCEMV1093PortableStableMemberVectors(t *testing.T) {
	base := neutralMapBytes(t)
	for _, c := range stableMemberCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			m, e := decodeStable(c.apply(t, base))
			if e == nil {
				e = validateStableReferences(&m)
			}
			got := ""
			if e != nil {
				got = e.code
			}
			if got != c.Code {
				t.Fatalf("code %q, want %q", got, c.Code)
			}
		})
	}
}

// TestCEMV1095PortableRoleAndProvenanceDecoded proves the decoded values and
// that neither member reaches the echoed reference set.
func TestCEMV1095PortableRoleAndProvenanceDecoded(t *testing.T) {
	m, e := decodeStable(neutralMapBytes(t))
	if e != nil || m.Provenance != nil || m.Role != "" {
		t.Fatalf("native map provenance=%v role=%q err=%v, want absent", m.Provenance, m.Role, e)
	}
	c := stableMemberCase{Set: map[string]json.RawMessage{
		"role":       json.RawMessage(`"bind-chore"`),
		"provenance": json.RawMessage(`{"sourceSpec":"cem/0.1","sourceBlobOid":"0123456789abcdef0123456789abcdef01234567","sourceSha256":"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210","bindCommit":"89abcdef0123456789abcdef0123456789abcdef","converter":"corvint-cem-convert/2"}`),
	}}
	m, e = decodeStable(c.apply(t, neutralMapBytes(t)))
	if e != nil {
		t.Fatal(e)
	}
	want := stableProvenance{"cem/0.1", "0123456789abcdef0123456789abcdef01234567", "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210", "89abcdef0123456789abcdef0123456789abcdef", "corvint-cem-convert/2"}
	if m.Role != "bind-chore" || m.Provenance == nil || *m.Provenance != want {
		t.Fatalf("role=%q provenance=%+v", m.Role, m.Provenance)
	}
	refs, err := json.Marshal(m.stableReferences)
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]any
	if err := json.Unmarshal(refs, &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 4 {
		t.Fatalf("echoed references %s, want the four arrays only", refs)
	}
}

// TestCEMV1094PortableProducerOnlyEmptyAccept verifies the neutral public map
// with the four producer-only members emitted empty: ACCEPT, an
// EMPTY_REFERENCE_SET reference axis and every authority axis NOT_OBSERVED.
// Adding provenance and role changes nothing but the map digest.
func TestCEMV1094PortableProducerOnlyEmptyAccept(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir := neutralCaseFiles(t)
	repo, base, target := neutralRepository(t, dir)
	empty := map[string]json.RawMessage{"criterionBindings": json.RawMessage(`[]`), "runnerReceipts": json.RawMessage(`[]`), "criterionLinks": json.RawMessage(`[]`), "artifacts": json.RawMessage(`[]`)}
	plain := stableMemberCase{Set: empty}.apply(t, neutralMapBytes(t))
	r, exit := runStableCanonical(context.Background(), repo, plain, base, target, dir, nil)
	if exit != 0 || r.Outcome != "ACCEPT" || r.Axes["referenceIntegrity"] != "EMPTY_REFERENCE_SET" || r.Axes["changeIntegrity"] != "VERIFIED" {
		b, _ := json.Marshal(r)
		t.Fatalf("runStableCanonical exit=%d result=%s, want ACCEPT with EMPTY_REFERENCE_SET", exit, b)
	}
	checked := map[string]bool{"changeIntegrity": true, "referenceIntegrity": true, "structuralProof": true, "coverageValidation": true, "discriminationValidation": true}
	for axis, value := range r.Axes {
		if !checked[axis] && value != "NOT_OBSERVED" {
			t.Fatalf("axis %s = %s, want NOT_OBSERVED", axis, value)
		}
	}
	marked := stableMemberCase{Set: map[string]json.RawMessage{"role": json.RawMessage(`"seal-chore"`), "provenance": json.RawMessage(`{"sourceSpec":"cem/0.2","sourceBlobOid":"` + base + `","sourceSha256":"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210","bindCommit":"` + target + `","converter":"corvint-cem-convert/1"}`)}}.apply(t, plain)
	again, exit2 := runStableCanonical(context.Background(), repo, marked, base, target, dir, nil)
	first, second := neutralResultMap(t, r), neutralResultMap(t, again)
	if first["mapSha256"] == second["mapSha256"] {
		t.Fatal("marked map has the same digest")
	}
	if first["verdictSha256"] == second["verdictSha256"] {
		t.Fatal("verdict digest did not move with mapSha256 (CEM-V1-066)")
	}
	for _, member := range []string{"mapSha256", "verdictSha256"} {
		delete(first, member)
		delete(second, member)
	}
	if exit2 != exit || !reflect.DeepEqual(first, second) {
		b, _ := json.Marshal(again)
		t.Fatalf("provenance and role changed the result: exit=%d %s", exit2, b)
	}
}
