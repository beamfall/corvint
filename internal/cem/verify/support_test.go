package verify

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// supportCase is one synthetic hunk: its patch paths and added lines and its
// map disposition and cited evidence IDs.
type supportCase struct {
	id, oldPath, newPath string
	added                []string
	disposition          string
	evidence             []string
}

func supportInputs(cases []supportCase, records map[string]string, spans map[string]string) (*wire.Map, *patch.Patch, map[string][]byte) {
	document := &wire.Map{Spec: wire.StableSpec}
	parsed := &patch.Patch{}
	for id, path := range records {
		document.Evidence = append(document.Evidence, wire.Evidence{ID: id, Path: path})
	}
	for _, c := range cases {
		hunk := &patch.Hunk{ID: c.id, Body: []patch.BodyLine{{Prefix: ' ', Payload: []byte("context\n")}}}
		if c.oldPath != "" {
			oldPath := c.oldPath
			hunk.OldPath = &oldPath
		}
		if c.newPath != "" {
			newPath := c.newPath
			hunk.NewPath = &newPath
		}
		for _, line := range c.added {
			hunk.Body = append(hunk.Body, patch.BodyLine{Prefix: '+', Payload: []byte(line)})
		}
		parsed.Hunks = append(parsed.Hunks, hunk)
		mapped := wire.Hunk{ID: c.id, Disposition: c.disposition}
		for _, evidenceID := range c.evidence {
			mapped.Basis = append(mapped.Basis, wire.Basis{EvidenceID: evidenceID, Relation: "specification"})
		}
		document.Hunks = append(document.Hunks, mapped)
	}
	bound := map[string][]byte{}
	for id, span := range spans {
		bound[id] = []byte(span)
	}
	return document, parsed, bound
}

func supportJSON(t *testing.T, citation *Citation) []byte {
	t.Helper()
	encoded, err := json.Marshal(citation)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func supported(id, path string, added []string, evidence ...string) supportCase {
	return supportCase{id: id, oldPath: path, newPath: path, added: added, disposition: "supported", evidence: evidence}
}

// TestCEMV1081IDAnchorRequiresBoundedTokenAndDefinition falsifies prefix and
// suffix ID matches and mention-only spans.
func TestCEMV1081IDAnchorRequiresBoundedTokenAndDefinition(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		added string
		span  string
		want  string
	}{
		{"defined", "x // `CEM-SM-007`\n", "intro\n  \t- `CEM-SM-007`: rename is strict\n", "id"},
		{"mention only", "x // `CEM-SM-007`\n", "See CEM-SM-0070 for the wider rule.\n", ""},
		{"span defines a longer ID", "x // `CEM-SM-007`\n", "- `CEM-SM-0070`: wider rule\n", ""},
		{"hunk adds a longer ID", "x // `CEM-SM-0070`\n", "- `CEM-SM-007`: rename is strict\n", ""},
		{"hunk ID has a letter prefix", "x // aCEM-SM-007\n", "- `CEM-SM-007`: rename is strict\n", ""},
		{"hunk ID has an underscore suffix", "x // CEM-SM-007_old\n", "- `CEM-SM-007`: rename is strict\n", ""},
		{"definition without the backtick colon", "x // CEM-SM-007\n", "- CEM-SM-007: rename is strict\n", ""},
		{"definition not at line start", "x // CEM-SM-007\n", "see - `CEM-SM-007`: rename\n", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			document, parsed, spans := supportInputs(
				[]supportCase{supported("hunk:a", "src/a.go", []string{test.added}, "evidence:1")},
				map[string]string{"evidence:1": "docs/spec.md"}, map[string]string{"evidence:1": test.span})
			citation := ComputeCitation(document, parsed, spans)
			row := citation.Hunks[0]
			if strings.Join(row.Anchors, ",") != test.want {
				t.Fatalf("anchors = %v, want %q", row.Anchors, test.want)
			}
			wantTier := TierCited
			if test.want != "" {
				wantTier = TierAnchored
			}
			if row.Tier != wantTier {
				t.Fatalf("tier = %s, want %s", row.Tier, wantTier)
			}
		})
	}
}

// TestCEMV1081PathAnchorUsesPostImageAndBasenameFloor checks the post-image
// path, the old path for a pure deletion and the six-byte basename floor.
func TestCEMV1081PathAnchorUsesPostImageAndBasenameFloor(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name             string
		oldPath, newPath string
		span             string
		want             bool
	}{
		{"full path", "src/a.go", "src/a.go", "edit src/a.go here", true},
		{"short basename alone", "src/a.go", "src/a.go", "edit a.go here", false},
		{"six-byte basename", "pkg/abc.go", "pkg/abc.go", "edit abc.go here", true},
		{"rename names the old path only", "src/old_name.go", "src/new_name.go", "edit src/old_name.go", false},
		{"rename names the new basename", "src/old_name.go", "src/new_name.go", "edit new_name.go", true},
		{"pure deletion names the old path", "src/gone.txt", "", "drop src/gone.txt", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := supportCase{id: "hunk:a", oldPath: test.oldPath, newPath: test.newPath, added: []string{"compute()\n"}, disposition: "supported", evidence: []string{"evidence:1"}}
			document, parsed, spans := supportInputs([]supportCase{c}, map[string]string{"evidence:1": "docs/spec.md"}, map[string]string{"evidence:1": test.span})
			row := ComputeCitation(document, parsed, spans).Hunks[0]
			if got := strings.Join(row.Anchors, ",") == "path"; got != test.want {
				t.Fatalf("anchors = %v, want path=%v", row.Anchors, test.want)
			}
		})
	}
}

// TestCEMV1081DirectDependencyIsNotAnAnchor pins V1-2343: a new file's hunk
// whose cited span is a direct dependency (the function it calls) is tiered
// `cited`, since CEM-V1-081 anchors only on a defined ID the hunk adds or on
// the hunk path, and the same hunk citing a span that names its path is
// `anchored`. SpanAnchors, which citation drafts use, agrees.
func TestCEMV1081DirectDependencyIsNotAnAnchor(t *testing.T) {
	t.Parallel()
	dependency := "func Helper(value int) int {\n"
	for _, test := range []struct {
		span string
		tier string
	}{
		{dependency, TierCited},
		{"| `internal/x/caller.go` | calls `Helper` |\n", TierAnchored},
	} {
		c := supportCase{id: "hunk:a", newPath: "internal/x/caller.go", added: []string{"package x\n", "func Caller() int { return Helper(1) }\n"}, disposition: "supported", evidence: []string{"evidence:1"}}
		document, parsed, spans := supportInputs([]supportCase{c}, map[string]string{"evidence:1": "internal/x/helper.go"}, map[string]string{"evidence:1": test.span})
		if row := ComputeCitation(document, parsed, spans).Hunks[0]; row.Tier != test.tier {
			t.Errorf("span %q: tier = %s, want %s", test.span, row.Tier, test.tier)
		}
		if kinds, _ := SpanAnchors(parsed.Hunks[0], []byte(test.span)); (len(kinds) > 0) != (test.tier == TierAnchored) {
			t.Errorf("span %q: SpanAnchors = %v, disagrees with tier %s", test.span, kinds, test.tier)
		}
	}
}

// TestCEMV1081Annotation checks the neutral-token rule.
func TestCEMV1081Annotation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		added []string
		want  bool
	}{
		{"id comment", []string{"// `CEM-SM-007`\n"}, true},
		{"short token and id", []string{"x // `CEM-SM-007`\n"}, true},
		{"stop terms and punctuation", []string{"# They MUST also have: these.\n", "\n"}, true},
		{"short code line of code-word terms", []string{"\treturn value - 1\n"}, false},
		{"real identifier", []string{"return compute(value)\n"}, false},
		{"bounded id is the only neutral long token", []string{"// CEM-SM-0070x\n"}, false},
		{"no added lines", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := supportCase{id: "hunk:a", oldPath: "src/a.go", newPath: "src/a.go", added: test.added, disposition: "unknown"}
			document, parsed, spans := supportInputs([]supportCase{c}, nil, nil)
			citation := ComputeCitation(document, parsed, spans)
			if citation.Hunks[0].Annotation != test.want || citation.Hunks[0].Tier != TierUnknown {
				t.Fatalf("row = %+v, want annotation %v and tier unknown", citation.Hunks[0], test.want)
			}
		})
	}
}

// TestCEMV1082BlockShapeAndCounts pins the exact JSON bytes of one block:
// tiers by precedence, duplicate evidence counted once, fan-out ordering,
// same-path citations per basis entry and concentration. Under the shift
// hunk:a inherits hunk:c's {a,b}, and evidence:a still names src/a.go, so
// shifted-anchored is 1 and specificity is 0.
func TestCEMV1082BlockShapeAndCounts(t *testing.T) {
	t.Parallel()
	cases := []supportCase{
		supported("hunk:c", "src/c.go", []string{"compute()\n"}, "evidence:b", "evidence:a"),
		{id: "hunk:m", oldPath: "src/m.go", newPath: "src/m.go", added: []string{"  compute()\n"}, disposition: "mechanical"},
		supported("hunk:a", "src/a.go", []string{"compute()\n"}, "evidence:a", "evidence:self"),
		{id: "hunk:u", oldPath: "src/u.go", newPath: "src/u.go", added: []string{"// `CEM-SM-007`\n"}, disposition: "unknown"},
		supported("hunk:b", "src/b.go", []string{"compute()\n"}, "evidence:b"),
	}
	// hunk:a cites evidence:a twice under two relations.
	document, parsed, spans := supportInputs(cases,
		map[string]string{"evidence:a": "docs/a.md", "evidence:b": "docs/b.md", "evidence:self": "src/a.go"},
		map[string]string{"evidence:a": "about src/a.go", "evidence:b": "nothing", "evidence:self": "package a"})
	for index := range document.Hunks {
		if document.Hunks[index].ID == "hunk:a" {
			document.Hunks[index].Basis = append(document.Hunks[index].Basis, wire.Basis{EvidenceID: "evidence:a", Relation: "constraint"})
		}
	}
	got := supportJSON(t, ComputeCitation(document, parsed, spans))
	want := `{"profile":"cem-citation-support/0","hunks":[` +
		`{"hunkId":"hunk:a","tier":"anchored","anchors":["path"],"annotation":false},` +
		`{"hunkId":"hunk:b","tier":"cited","anchors":[],"annotation":false},` +
		`{"hunkId":"hunk:c","tier":"cited","anchors":[],"annotation":false},` +
		`{"hunkId":"hunk:m","tier":"mechanical-proved","anchors":[],"annotation":false},` +
		`{"hunkId":"hunk:u","tier":"unknown","anchors":[],"annotation":true}],` +
		`"hunksByClass":{"held":0,"mechanical-proved":1,"anchored":1,"cited":2,"unknown":1},` +
		`"annotationHunks":1,"distinctEvidence":3,` +
		`"evidenceFanOut":[["evidence:a",2],["evidence:b",2],["evidence:self",1]],` +
		`"maxFanOut":2,"samePathCitations":1,"concentratedHunks":2,"invariant":false,"specificity":0}`
	if string(got) != want {
		t.Fatalf("citation =\n%s\nwant\n%s", got, want)
	}
}

// TestCEMV1082ShiftNull covers the shift null's edge cases: a single
// supported hunk, all-equal basis sets and a negative specificity.
func TestCEMV1082ShiftNull(t *testing.T) {
	t.Parallel()
	single, parsed, spans := supportInputs([]supportCase{supported("hunk:a", "src/alpha.go", []string{"compute()\n"}, "evidence:1")},
		map[string]string{"evidence:1": "docs/x.md"}, map[string]string{"evidence:1": "src/alpha.go"})
	if c := ComputeCitation(single, parsed, spans); !c.Invariant || c.Specificity != 0 || c.HunksByClass.Anchored != 1 {
		t.Fatalf("single supported hunk = %s", supportJSON(t, c))
	}
	equal, parsed, spans := supportInputs([]supportCase{
		supported("hunk:a", "src/alpha.go", []string{"compute()\n"}, "evidence:1"),
		supported("hunk:b", "src/beta.go", []string{"compute()\n"}, "evidence:1"),
	}, map[string]string{"evidence:1": "docs/x.md"}, map[string]string{"evidence:1": "src/alpha.go"})
	if c := ComputeCitation(equal, parsed, spans); !c.Invariant || c.Specificity != 0 {
		t.Fatalf("equal basis sets = %s", supportJSON(t, c))
	}
	crossed, parsed, spans := supportInputs([]supportCase{
		supported("hunk:a", "src/alpha.go", []string{"compute()\n"}, "evidence:names-beta"),
		supported("hunk:b", "src/beta.go", []string{"compute()\n"}, "evidence:names-alpha"),
	}, map[string]string{"evidence:names-alpha": "docs/x.md", "evidence:names-beta": "docs/y.md"},
		map[string]string{"evidence:names-alpha": "see src/alpha.go", "evidence:names-beta": "see src/beta.go"})
	if c := ComputeCitation(crossed, parsed, spans); c.Invariant || c.Specificity != -2 || c.HunksByClass.Cited != 2 {
		t.Fatalf("crossed citations = %s", supportJSON(t, c))
	}
}

// TestCEMV1082HunkOrderDoesNotChangeTheBlock is fixture (e) at the function
// level: permuting map hunks, patch hunks and evidence records, and repeating
// the computation, yields byte-identical JSON.
func TestCEMV1082HunkOrderDoesNotChangeTheBlock(t *testing.T) {
	t.Parallel()
	cases := []supportCase{
		supported("hunk:3", "src/gamma.go", []string{"gamma()\n"}, "evidence:g", "evidence:shared"),
		supported("hunk:1", "src/alpha.go", []string{"alpha() // CEM-XY-101\n"}, "evidence:a"),
		supported("hunk:2", "src/beta.go", []string{"beta()\n"}, "evidence:shared"),
		{id: "hunk:4", oldPath: "src/d.go", newPath: "src/d.go", added: []string{"x\n"}, disposition: "unknown"},
	}
	records := map[string]string{"evidence:a": "docs/a.md", "evidence:g": "docs/g.md", "evidence:shared": "AGENTS.md"}
	spanText := map[string]string{"evidence:a": "- `CEM-XY-101`: alpha", "evidence:g": "src/gamma.go", "evidence:shared": "rules"}
	document, parsed, spans := supportInputs(cases, records, spanText)
	first := supportJSON(t, ComputeCitation(document, parsed, spans))
	if again := supportJSON(t, ComputeCitation(document, parsed, spans)); !bytes.Equal(first, again) {
		t.Fatalf("repeated computation differs:\n%s\n%s", first, again)
	}
	for rotation := 1; rotation < len(cases); rotation++ {
		rotated := append(append([]supportCase(nil), cases[rotation:]...), cases[:rotation]...)
		document, parsed, spans := supportInputs(rotated, records, spanText)
		for left, right := 0, len(document.Evidence)-1; left < right; left, right = left+1, right-1 {
			document.Evidence[left], document.Evidence[right] = document.Evidence[right], document.Evidence[left]
		}
		if got := supportJSON(t, ComputeCitation(document, parsed, spans)); !bytes.Equal(first, got) {
			t.Fatalf("rotation %d changed the block:\n%s\n%s", rotation, first, got)
		}
	}
}
