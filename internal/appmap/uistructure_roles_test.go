package appmap

import (
	"encoding/json"
	"reflect"
	"testing"
)

// OUE-V0-075 (GH #1831): a role token outside the closed ARIA role vocabulary never reaches the
// digest; it reads as #other and is counted dropped, so two snapshots differing only in such a
// token share one digest, while a real role change still changes it.
func TestOUEV0075ScrubAriaClosedRoles(t *testing.T) {
	a, n, dropped, ok := ScrubAriaDigest("- main:\n  - sksecretTokenValue\n  - button \"Pay\"")
	if !ok || n != 3 || dropped != 2 {
		t.Fatalf("nodes %d dropped %d ok %v", n, dropped, ok)
	}
	b, _, _, _ := ScrubAriaDigest("- main:\n  - anotherFreeWord\n  - button \"Send\"")
	if a != b {
		t.Fatal("a free token in the role position must not change the structure digest")
	}
	c, _, _, _ := ScrubAriaDigest("- main:\n  - sksecretTokenValue\n  - link \"Pay\"")
	if a == c {
		t.Fatal("a role change must change the structure digest")
	}
	// A vocabulary prefix followed by other characters is not that role.
	d1, _, dr, _ := ScrubAriaDigest("- main:\n  - button123\n  - button \"Pay\"")
	d2, _, _, _ := ScrubAriaDigest("- main:\n  - link456\n  - button \"Pay\"")
	if d1 != a || d2 != a || dr != 2 {
		t.Fatalf("a role-prefixed token must read #other and count dropped (dropped %d)", dr)
	}
	if _, _, d, _ := ScrubAriaDigest("- text: hello\n- img \"logo\"\n- treeitem [expanded]"); d != 2 {
		t.Fatalf("vocabulary roles must be kept; dropped %d", d)
	}
}

// OUE-V0-076: a structure-changed screen names the tests that saw its base-only or head-only
// digests; every other state carries an empty list.
func TestOUEV0076StructureDiffNamesTests(t *testing.T) {
	base := &UIRecords{Revision: uiTestRevision, Traces: []UITrace{{AriaSnapshots: 1}}, Structures: []UIStructure{
		{Screen: "screen:a", Digests: []UIStructureDigest{{Digest: "d1", Tests: []string{"t-same"}}}},
		{Screen: "screen:b", Digests: []UIStructureDigest{{Digest: "d2", Tests: []string{"t-old"}}, {Digest: "d5", Tests: []string{"t-shared"}}}},
	}}
	head := &UIRecords{Revision: uiTestRevision, Traces: []UITrace{{AriaSnapshots: 1}}, Structures: []UIStructure{
		{Screen: "screen:a", Digests: []UIStructureDigest{{Digest: "d1", Tests: []string{"t-same"}}}},
		{Screen: "screen:b", Digests: []UIStructureDigest{{Digest: "d9", Tests: []string{"t-new", "t-old"}}, {Digest: "d5", Tests: []string{"t-shared"}}}},
		{Screen: "screen:c", Digests: []UIStructureDigest{{Digest: "d3", Tests: []string{"t-one-side"}}}},
	}}
	raw, err := DiffUIStructures(base, head)
	if err != nil {
		t.Fatal(err)
	}
	var d UIStructureDiff
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, s := range d.Screens {
		if s.Tests == nil {
			t.Fatalf("%s: tests must be a list", s.Screen)
		}
		got[s.Screen] = s.Tests
	}
	want := map[string][]string{"screen:a": {}, "screen:b": {"t-new", "t-old"}, "screen:c": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tests per screen: %v", got)
	}
}
