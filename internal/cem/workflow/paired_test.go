package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// pairedFixture prepares a cem/1.0 map over the app range of one calc
// repository and returns a second calc repository whose range carries the
// tests, with the app root, the tests root and the tests range ends.
func pairedFixture(t *testing.T) (app, tests, testsBase, testsHead, hunk string) {
	t.Helper()
	app, base, target := makeCalcRepo(t)
	if _, err := openSession(t, app).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, document, err := openSession(t, app).readProducerMap(wire.ExcludedCEMPath)
	if err != nil {
		t.Fatal(err)
	}
	tests, testsBase, testsHead = makeCalcRepo(t)
	return app, tests, testsBase, testsHead, document.Hunks[0].ID
}

func citePaired(t *testing.T, app, tests, pairedRange, hunk string) (map[string]any, error) {
	t.Helper()
	return openSession(t, app).CitePaired(ctx(), PairedCiteOptions{
		MapPath: wire.ExcludedCEMPath, Output: "corvint/paired.json", Range: pairedRange, Root: tests,
		Hunk: hunk, EvidencePath: "pkg/calc/calc_test.go", Lines: "1:3", Relation: "test-claim",
	})
}

// TestCAXV0032CitePairedResolvesTheRangeHead checks CAX-V0-032 (#1941): a
// paired citation resolves both range ends in the second repository, cites a
// regular blob at the head into a canonical cem-paired-evidence/1 document
// bound to the map's change, leaves the map bytes unchanged, is idempotent,
// and is refused when either end does not resolve, when base is not an
// ancestor of head, when the range is malformed, or when the document already
// pins a different range.
func TestCAXV0032CitePairedResolvesTheRangeHead(t *testing.T) {
	app, tests, testsBase, testsHead, hunk := pairedFixture(t)
	mapBefore, err := os.ReadFile(filepath.Join(app, filepath.FromSlash(wire.ExcludedCEMPath)))
	if err != nil {
		t.Fatal(err)
	}
	result, err := citePaired(t, app, tests, "tests="+testsBase+"..main", hunk)
	if err != nil {
		t.Fatal(err)
	}
	if result["added"] != true || result["hunkId"] != hunk {
		t.Fatalf("cite-paired result %v", result)
	}
	if again, err := citePaired(t, app, tests, "tests="+testsBase+".."+testsHead, hunk); err != nil || again["added"] != false {
		t.Fatalf("repeated citation: %v %v", again, err)
	}
	raw, err := os.ReadFile(filepath.Join(app, "corvint", "paired.json"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParsePairedEvidence(raw)
	if err != nil {
		t.Fatal(err)
	}
	mapAfter, _ := os.ReadFile(filepath.Join(app, filepath.FromSlash(wire.ExcludedCEMPath)))
	if string(mapAfter) != string(mapBefore) {
		t.Fatal("a paired citation changed the cem/1.0 map")
	}
	if _, err := wire.BindPairedEvidence(document, mapAfter); err != nil {
		t.Fatal(err)
	}
	if document.Range != (wire.PairedRange{Name: "tests", BaseRevision: testsBase, HeadRevision: testsHead}) || len(document.Citations) != 1 {
		t.Fatalf("paired document %+v", document)
	}
	for name, pairedRange := range map[string]string{
		"unresolved base":  "tests=0000000000000000000000000000000000000001.." + testsHead,
		"unresolved head":  "tests=" + testsBase + "..no-such-branch",
		"reversed range":   "tests=" + testsHead + ".." + testsBase,
		"malformed range":  "Tests:" + testsBase,
		"other pinned end": "other=" + testsBase + ".." + testsHead,
	} {
		if _, err := citePaired(t, app, tests, pairedRange, hunk); err == nil {
			t.Fatalf("%s: cite-paired was not refused", name)
		} else if strings.HasPrefix(name, "unresolved") && !strings.Contains(err.Error(), "does not resolve") {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := openSession(t, app).CitePaired(ctx(), PairedCiteOptions{
		MapPath: wire.ExcludedCEMPath, Output: wire.ExcludedCEMPath, Range: "tests=" + testsBase + ".." + testsHead, Root: tests,
		Hunk: hunk, EvidencePath: "pkg/calc/calc_test.go", Lines: "1:3", Relation: "test-claim",
	}); cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("--paired-output naming the map: %v", err)
	}
}

// TestCAXV0033PairedStatusReverifiesTheRange checks CAX-V0-033: status over
// the document re-resolves the pinned range and every citation and reports
// the counts; a citation whose blob no longer matches, or a document edited
// out of canonical form, refuses the whole document.
func TestCAXV0033PairedStatusReverifiesTheRange(t *testing.T) {
	app, tests, testsBase, testsHead, hunk := pairedFixture(t)
	if _, err := citePaired(t, app, tests, "tests="+testsBase+".."+testsHead, hunk); err != nil {
		t.Fatal(err)
	}
	status := func() (map[string]any, error) {
		return openSession(t, app).PairedStatus(ctx(), PairedStatusOptions{
			MapPath: wire.ExcludedCEMPath, Paired: "corvint/paired.json", Root: tests,
		})
	}
	result, err := status()
	if err != nil {
		t.Fatal(err)
	}
	counts, _ := result["counts"].(map[string]any)
	if result["ok"] != true || counts["citations"] != 1 || counts["pairedHunks"] != 1 || len(result["evidenceSha256"].(string)) != 64 {
		t.Fatalf("paired status %v", result)
	}
	path := filepath.Join(app, "corvint", "paired.json")
	raw, _ := os.ReadFile(path)
	document, _ := wire.ParsePairedEvidence(raw)
	document.Citations[0].BlobOid = strings.Repeat("a", 40)
	drifted, err := wire.EncodePairedEvidence(*document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, drifted, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := status(); cemcode.CodeOf(err) != cemcode.EvidenceDrift {
		t.Fatalf("drifted citation: %v", err)
	}
	if err := os.WriteFile(path, append([]byte(" "), raw...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := status(); cemcode.CodeOf(err) != cemcode.InvalidField {
		t.Fatalf("non-canonical document: %v", err)
	}
}
