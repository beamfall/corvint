package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// citationUnitPath names the one file each fixture hunk changes.
func citationUnitPath(index int) string { return fmt.Sprintf("src/unit_%02d.txt", index) }

// citationRepo commits a base with AGENTS.md, a per-unit notes file and n
// unit files, then a target that appends one line to every unit, giving
// exactly n hunks. Unit 5 adds a requirement ID the notes define.
func citationRepo(t *testing.T, n int) (root, base, target string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	var agents, notes strings.Builder
	agents.WriteString("# Rules\n")
	for index := 0; index < n; index++ {
		fmt.Fprintf(&agents, "Rule %d: keep every review small and readable.\n", index)
	}
	notes.WriteString("- src/unit_00.txt and src/unit_01.txt share one counter.\n")
	for index := 1; index < n; index++ {
		if index == 5 {
			fmt.Fprintf(&notes, "- `CEM-XY-105`: %s advances its counter.\n", citationUnitPath(index))
			continue
		}
		fmt.Fprintf(&notes, "- %s advances its counter.\n", citationUnitPath(index))
	}
	writeFile(t, root, "AGENTS.md", agents.String())
	writeFile(t, root, "docs/notes.md", notes.String())
	for index := 0; index < n; index++ {
		writeFile(t, root, citationUnitPath(index), fmt.Sprintf("counter = %d\n", index))
	}
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base = gitCmd(t, root, "rev-parse", "HEAD")
	for index := 0; index < n; index++ {
		added := fmt.Sprintf("advance(%d)\n", index)
		if index == 5 {
			added = "advance(5) // CEM-XY-105\n"
		}
		writeFile(t, root, citationUnitPath(index), fmt.Sprintf("counter = %d\n%s", index, added))
	}
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	target = gitCmd(t, root, "rev-parse", "HEAD")
	return root, base, target
}

// statusWithCitations prepares base..target, cites every hunk with the span
// choose names for it and returns the stable status with --citation.
func statusWithCitations(t *testing.T, root, base, target string, choose func(wire.Hunk) (string, string)) map[string]any {
	t.Helper()
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, document, err := openSession(t, root).readProducerMap(wire.ExcludedCEMPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, hunk := range document.Hunks {
		evidencePath, lines := choose(hunk)
		if _, err := openSession(t, root).Cite(ctx(), CiteOptions{
			MapPath: wire.ExcludedCEMPath, Hunk: hunk.ID, EvidencePath: evidencePath, Lines: lines, Relation: "specification",
		}); err != nil {
			t.Fatal(err)
		}
	}
	return citationStatus(t, root, base, target)
}

// citationStatus reads the stable status of the sidecar with --citation.
func citationStatus(t *testing.T, root, base, target string) map[string]any {
	t.Helper()
	result, err := openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target,
		ArtifactRoot: root, ArtifactsGiven: true, Citation: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// acceptedCitation requires an accepted stable status and returns the
// verifier's support block and that block's JSON bytes (CEM-V1-092).
func acceptedCitation(t *testing.T, result map[string]any) (*verify.Citation, []byte) {
	t.Helper()
	verification, _ := result["stableVerification"].(StableVerification)
	if verification.Result.Outcome != "ACCEPT" || verification.Result.Citation == nil {
		encoded, _ := json.MarshalIndent(result, "", "  ")
		t.Fatalf("status did not accept with a citation block: %s", encoded)
	}
	encoded, err := json.Marshal(verification.Result.Citation)
	if err != nil {
		t.Fatal(err)
	}
	return verification.Result.Citation, encoded
}

func unitIndex(t *testing.T, path string) int {
	t.Helper()
	var index int
	if _, err := fmt.Sscanf(path, "src/unit_%02d.txt", &index); err != nil {
		t.Fatalf("unexpected hunk path %q", path)
	}
	return index
}

// TestCEMV1080RubberStampAndCarefulMapsDiffer is fixture (a): one 24-hunk
// patch, a rubber-stamp map citing one AGENTS.md span for every hunk and a
// careful per-hunk map; both accept and their citation integers differ.
func TestCEMV1080RubberStampAndCarefulMapsDiffer(t *testing.T) {
	t.Parallel()
	rubberRoot, base, target := citationRepo(t, 24)
	rubber, rubberJSON := acceptedCitation(t, statusWithCitations(t, rubberRoot, base, target, func(wire.Hunk) (string, string) {
		return "AGENTS.md", "1:1"
	}))
	carefulRoot, carefulBase, carefulTarget := citationRepo(t, 24)
	if carefulBase != base || carefulTarget != target {
		t.Fatalf("fixture repositories are not reproducible: %s..%s vs %s..%s", base, target, carefulBase, carefulTarget)
	}
	careful, carefulJSON := acceptedCitation(t, statusWithCitations(t, carefulRoot, base, target, func(hunk wire.Hunk) (string, string) {
		line := unitIndex(t, hunk.Path) + 1 // notes line i+1 names unit i
		if line <= 2 {
			line = 1 // units 0 and 1 share the first notes line
		}
		return "docs/notes.md", fmt.Sprintf("%d:%d", line, line)
	}))
	t.Logf("rubber-stamp citation: %s", rubberJSON)
	t.Logf("careful citation: %s", carefulJSON)

	if rubber.DistinctEvidence != 1 || rubber.MaxFanOut != 24 || rubber.ConcentratedHunks != 24 ||
		rubber.HunksByClass != (verify.CitationClasses{Cited: 24}) || !rubber.Invariant || rubber.Specificity != 0 ||
		rubber.SamePathCitations != 0 || rubber.AnnotationHunks != 0 || len(rubber.EvidenceFanOut) != 1 || rubber.EvidenceFanOut[0].Count != 24 {
		t.Fatalf("rubber-stamp citation = %s", rubberJSON)
	}
	// After the cyclic shift only unit 1 or unit 0 can still be anchored, and
	// only when the two sharing units are neighbours in hunkId order.
	shared := map[string]bool{hunkForPath(t, carefulRoot, 0): true, hunkForPath(t, carefulRoot, 1): true}
	wantSpecificity := 24
	for index, row := range careful.Hunks {
		next := careful.Hunks[(index+1)%len(careful.Hunks)]
		if shared[row.HunkID] && shared[next.HunkID] {
			wantSpecificity = 23
		}
	}
	if careful.DistinctEvidence != 23 || careful.MaxFanOut != 2 || careful.ConcentratedHunks != 2 ||
		careful.HunksByClass != (verify.CitationClasses{Anchored: 24}) || careful.Invariant || careful.Specificity != wantSpecificity ||
		careful.SamePathCitations != 0 || careful.AnnotationHunks != 0 {
		t.Fatalf("careful citation = %s", carefulJSON)
	}
	for _, row := range careful.Hunks {
		want := []string{"path"}
		if row.HunkID == hunkForPath(t, carefulRoot, 5) {
			want = []string{"id", "path"}
		}
		if strings.Join(row.Anchors, ",") != strings.Join(want, ",") || row.Tier != verify.TierAnchored {
			t.Fatalf("careful hunk %s = %+v, want anchors %v", row.HunkID, row, want)
		}
	}
	if bytes.Equal(rubberJSON, carefulJSON) {
		t.Fatal("rubber-stamp and careful maps produced identical citation blocks")
	}
}

// hunkForPath returns the hunk ID the committed map assigns to unit index.
func hunkForPath(t *testing.T, root string, index int) string {
	t.Helper()
	_, document, err := openSession(t, root).readProducerMap(wire.ExcludedCEMPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, hunk := range document.Hunks {
		if hunk.Path == citationUnitPath(index) {
			return hunk.ID
		}
	}
	t.Fatalf("no hunk for unit %d", index)
	return ""
}

// TestCEMV1082DisjointUmbrellaSpansHaveNoSpecificity is fixture (b):
// AGENTS.md split into 20 disjoint spans, one per hunk, naming no hunk path
// and defining no hunk ID. Diverse evidence alone must not look specific.
func TestCEMV1082DisjointUmbrellaSpansHaveNoSpecificity(t *testing.T) {
	t.Parallel()
	root, base, target := citationRepo(t, 20)
	citation, encoded := acceptedCitation(t, statusWithCitations(t, root, base, target, func(hunk wire.Hunk) (string, string) {
		line := unitIndex(t, hunk.Path) + 2 // line 1 is the heading
		return "AGENTS.md", fmt.Sprintf("%d:%d", line, line)
	}))
	if citation.Specificity != 0 || citation.Invariant || citation.DistinctEvidence != 20 || citation.MaxFanOut != 1 ||
		citation.HunksByClass != (verify.CitationClasses{Cited: 20}) {
		t.Fatalf("disjoint AGENTS.md spans citation = %s", encoded)
	}
}

// TestCEMV1081IDBoundaryAndAnnotation is fixtures (c) and (d): a span that
// only mentions CEM-SM-0070 never anchors a hunk adding CEM-SM-007; comment
// and ID-only lines are annotation, a real identifier line is not.
func TestCEMV1081IDBoundaryAndAnnotation(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "docs/ids.md", "See CEM-SM-0070 for the wider rename rule.\n")
	for _, name := range []string{"a", "b", "c"} {
		writeFile(t, root, "src/"+name+".go", "package "+name+"\n")
	}
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "src/a.go", "package a\nx // `CEM-SM-007`\n")
	writeFile(t, root, "src/b.go", "package b\n// `CEM-SM-007`\n")
	writeFile(t, root, "src/c.go", "package c\nreturn compute(value)\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	citation, encoded := acceptedCitation(t, statusWithCitations(t, root, base, target, func(wire.Hunk) (string, string) {
		return "docs/ids.md", "1:1"
	}))
	_, document, err := openSession(t, root).readProducerMap(wire.ExcludedCEMPath)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"src/a.go": true, "src/b.go": true, "src/c.go": false}
	byID := map[string]string{}
	for _, hunk := range document.Hunks {
		byID[hunk.ID] = hunk.Path
	}
	for _, row := range citation.Hunks {
		path := byID[row.HunkID]
		if row.Tier != verify.TierCited || len(row.Anchors) != 0 || row.Annotation != want[path] {
			t.Fatalf("%s = %+v; block %s", path, row, encoded)
		}
	}
	if citation.AnnotationHunks != 2 || citation.HunksByClass != (verify.CitationClasses{Cited: 3}) {
		t.Fatalf("citation = %s", encoded)
	}
}

// TestCEMV1082CitationIsDeterministic is fixture (e) at the envelope level:
// two status reads of one map are byte-identical, and the block keeps its
// pinned key order.
func TestCEMV1082CitationIsDeterministic(t *testing.T) {
	t.Parallel()
	root, base, target := citationRepo(t, 6)
	first := statusWithCitations(t, root, base, target, func(hunk wire.Hunk) (string, string) {
		line := unitIndex(t, hunk.Path) + 1
		return "docs/notes.md", fmt.Sprintf("%d:%d", line, line)
	})
	_, firstJSON := acceptedCitation(t, first)
	second := citationStatus(t, root, base, target)
	_, secondJSON := acceptedCitation(t, second)
	firstEnvelope, _ := json.Marshal(first)
	secondEnvelope, _ := json.Marshal(second)
	if !bytes.Equal(firstJSON, secondJSON) || !bytes.Equal(firstEnvelope, secondEnvelope) {
		t.Fatalf("status is not byte-deterministic:\n%s\n%s", firstJSON, secondJSON)
	}
	if !bytes.HasPrefix(firstJSON, []byte(`{"profile":"cem-citation-support/0","hunks":[`)) {
		t.Fatalf("citation keys are not in the pinned order: %s", firstJSON)
	}
}
