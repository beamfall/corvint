package tcq

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// TestBlobAnalysisMatchesOracle checks the whole per-blob surface — unit spans,
// body digests, execution keys, hygiene axes, unit identities, unit wire rows,
// and the re-derived anchor candidates — against bytes captured from the Python
// oracle.
func TestBlobAnalysisMatchesOracle(t *testing.T) {
	vectors := loadVectors(t)
	for path, expected := range vectors.Blobs {
		t.Run(path, func(t *testing.T) {
			analysis, err := analyzeBlob(path, expected.OID, []byte(expected.Source))
			if err != nil {
				t.Fatalf("analyzeBlob: %v", err)
			}
			wantFailure := ""
			if expected.Error != nil {
				wantFailure = *expected.Error
			}
			if analysis.failure != wantFailure {
				t.Fatalf("failure = %q, oracle %q", analysis.failure, wantFailure)
			}
			checkUnits(t, analysis, expected.UnitRows())
			checkCandidates(t, analysis, expected.CandidateKeys())
		})
	}
}

func TestPythonGrammarFailureRetainsAnchorWithoutPublishingUnit(t *testing.T) {
	const source = "def test_match(value):\n" +
		"    \"\"\"REQ-1\"\"\"\n" +
		"    match value:\n" +
		"        case 1:\n" +
		"            return 1\n"
	const anchor = `"""REQ-1"""`
	start := strings.Index(source, anchor)

	blob, err := analyzePythonBlob("pkg/test_match.py", "blob-oid", []byte(source))
	if err != nil {
		t.Fatalf("analyzePythonBlob: %v", err)
	}
	if blob.failure != reasonUnsupportedPythonGrammar {
		t.Fatalf("blob failure = %q, want %q", blob.failure, reasonUnsupportedPythonGrammar)
	}
	association, units := associate(blob, "test:test_match#doc", int64(start), int64(start+len(anchor)))
	if association.profile != anchorPythonDocstring {
		t.Errorf("association profile = %q, want %q", association.profile, anchorPythonDocstring)
	}
	if association.reason != reasonUnsupportedPythonGrammar {
		t.Errorf("association reason = %q, want %q", association.reason, reasonUnsupportedPythonGrammar)
	}
	if association.unit != nil || len(units) != 0 || len(blob.allUnits) != 0 {
		t.Errorf("grammar refusal published units: association=%+v units=%+v blob units=%+v", association.unit, units, blob.allUnits)
	}
}

func TestGoRunCaseAssociatesAsTableCase(t *testing.T) {
	const source = "package sample\n\nimport \"testing\"\n\n" +
		"func TestRun(t *testing.T) {\n" +
		"\tt.Run(\"leaf-two\", func(t *testing.T) { t.Log(\"x\") })\n" +
		"}\n"
	start := strings.Index(source, "leaf-two")

	blob, err := analyzeGoBlob("pkg/run_test.go", "blob-oid", []byte(source))
	if err != nil {
		t.Fatalf("analyzeGoBlob: %v", err)
	}
	association, _ := associate(blob, "test:TestRun/case:leaf-two", int64(start), int64(start+len("leaf-two")))
	if association.profile != anchorGoTableCase || association.reason != "" {
		t.Fatalf("association = %+v, want %s with no reason", association, anchorGoTableCase)
	}
	if association.unit == nil || association.unit.runtimeName != "TestRun/leaf-two" || association.unit.associationKind != kindGoTableCase {
		t.Fatalf("association unit = %+v, want GO_TABLE_CASE TestRun/leaf-two", association.unit)
	}
}

// TestGoCaseSelectorMustNameTheScannedParent covers TCQ-V0-016/018: a
// `func TestX(` inside a raw string makes the comment-masked claim extractor
// name a lookalike parent, and that selector must not associate with the case
// leaf of the real enclosing test.
func TestGoCaseSelectorMustNameTheScannedParent(t *testing.T) {
	const source = "package sample\n\nimport \"testing\"\n\n" +
		"func TestReal(t *testing.T) {\n" +
		"\t_ = `\nfunc TestFake(t *testing.T) {`\n" +
		"\tt.Run(\"leaf\", func(t *testing.T) { t.Log(\"x\") })\n" +
		"}\n"
	start := int64(strings.Index(source, "leaf"))
	blob, err := analyzeGoBlob("pkg/run_test.go", "blob-oid", []byte(source))
	if err != nil {
		t.Fatalf("analyzeGoBlob: %v", err)
	}
	lookalike, _ := associate(blob, "test:TestFake/case:leaf", start, start+4)
	if lookalike.profile != anchorGoTableCase || lookalike.unit != nil || lookalike.reason != reasonClaimAssociationMissing {
		t.Fatalf("lookalike association = %+v, want %s abstaining with %s", lookalike, anchorGoTableCase, reasonClaimAssociationMissing)
	}
}

func checkUnits(t *testing.T, analysis blobAnalysis, want []string) {
	t.Helper()
	got := make([]string, 0, len(analysis.allUnits))
	for _, unit := range analysis.allUnits {
		got = append(got, unitFingerprint(unit))
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("unit count = %d, oracle %d\n got: %v\nwant: %v", len(got), len(want), got, want)
	}
	for index := range got {
		if got[index] != want[index] {
			t.Errorf("unit[%d] =\n %s\noracle\n %s", index, got[index], want[index])
		}
	}
}

func unitFingerprint(unit testUnit) string {
	return fmt.Sprintf("%s|%d|%d|%s|%s|%s|%s|%v|%v|%s|%s",
		unit.path, unit.bodyStart, unit.bodyEnd, unit.bodySHA256, unit.executionKey,
		unit.extractorProfile, unit.associationKind, unit.empty, unit.skipped,
		unit.identity(), string(canonicalValue(unit.wireValue())))
}

func checkCandidates(t *testing.T, analysis blobAnalysis, want map[string]bool) {
	t.Helper()
	got := map[string]bool{}
	for _, candidate := range analysis.candidates {
		got[candidateKey(candidate.producer, candidate.selector, candidate.start, candidate.end)] = true
	}
	for key := range want {
		if !got[key] {
			t.Errorf("missing re-derived candidate %s", key)
		}
	}
	for key := range got {
		if !want[key] {
			t.Errorf("unexpected re-derived candidate %s", key)
		}
	}
}

func candidateKey(producer, selector string, start, end int) string {
	return fmt.Sprintf("%s|%s|%d|%d", producer, selector, start, end)
}
