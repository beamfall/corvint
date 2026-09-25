package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

const whisperTarget = importReorderTarget + "\n// Whisper lower-cases s.\nfunc Whisper(s string) string { return strings.ToLower(s) }\n"

// witnessMap is where a test writes the cem/0.3 map, which never replaces
// the cem/0.2 input or lands on the Core sidecar path (V1-0335).
const witnessMap = ".corvint/witness.cem.json"

func readMap(t *testing.T, root string) *wire.Map {
	t.Helper()
	return readMapAt(t, root, wire.ExcludedCEMPath)
}

func readMapAt(t *testing.T, root, relative string) *wire.Map {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParseMap(data)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func reportText(t *testing.T, root, base string) string {
	t.Helper()
	return reportTextAt(t, root, base, wire.ExcludedCEMPath)
}

func reportTextAt(t *testing.T, root, base, mapPath string) string {
	t.Helper()
	result, err := openSession(t, root).Read(ctx(), "report", ReadOptions{
		MapPath: mapPath, ExpectedBase: base, Target: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(result["report"].(string))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// TestCoverRecordsCoverageWitnessAndReportDowngrades is TCQ-V0-051..054
// end to end: cover intersects one explicit coverprofile with the hunk's
// added lines and records a witness on every hunk (covered or uncovered,
// never absent), the map becomes cem/0.3 and still verifies, and the reviewer
// report downgrades a test claim without a covering witness with a visible
// reason.
func TestCoverRecordsCoverageWitnessAndReportDowngrades(t *testing.T) {
	root, base, target := makeGoRepo(t, importReorderTarget, whisperTarget)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "pkg/a.go", Lines: "1:2", Relation: "test-claim",
	}); err != nil {
		t.Fatal(err)
	}
	if text := reportText(t, root, base); !strings.Contains(text, "downgraded from tested; reason `no-coverage-witness`") {
		t.Fatalf("report without a witness:\n%s", text)
	}

	covered := "mode: set\nexample.com/m/pkg/a.go:9.41,9.72 1 1\nexample.com/m/pkg/a.go:12.43,12.71 1 1\n"
	writeFile(t, root, "cover.out", covered)
	result, err := openSession(t, root).Cover(ctx(), CoverOptions{
		MapPath: wire.ExcludedCEMPath, Coverprofile: "cover.out", TestRun: "go test ./pkg/...", Output: witnessMap,
	})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(covered))
	if result["covered"] != 1 || result["uncovered"] != 0 || result["profileSha256"] != hex.EncodeToString(digest[:]) {
		t.Fatalf("cover envelope = %v", result)
	}
	document := readMapAt(t, root, witnessMap)
	witness := document.Hunks[0].Coverage
	if document.Spec != wire.Spec03 || witness == nil || witness.State != wire.CoverageCovered ||
		witness.TestRun != "go test ./pkg/..." || witness.Mode != "set" || len(witness.Covered) != 1 ||
		witness.Covered[0] != (wire.Range{Start: 12, Count: 1}) {
		t.Fatalf("spec %s witness %+v", document.Spec, witness)
	}
	status, err := openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: witnessMap, ExpectedBase: base, Target: "HEAD",
	})
	if err != nil || status["state"] != "ready-for-ci" {
		t.Fatalf("status after cover: %v %v", err, status)
	}
	if text := reportTextAt(t, root, base, witnessMap); !strings.Contains(text, "tested (test run `go test ./pkg/...`, coverprofile `"+hex.EncodeToString(digest[:])+"`)") {
		t.Fatalf("report with a covering witness:\n%s", text)
	}

	// A context line the profile reaches is not coverage of the change.
	writeFile(t, root, "cover.out", "mode: count\nexample.com/m/pkg/a.go:9.41,9.72 1 7\nexample.com/m/pkg/a.go:12.43,12.71 1 0\n")
	result, err = openSession(t, root).Cover(ctx(), CoverOptions{
		MapPath: wire.ExcludedCEMPath, Coverprofile: "cover.out", TestRun: "go test -run TestShout ./pkg/...", Output: witnessMap,
	})
	if err != nil || result["covered"] != 0 || result["uncovered"] != 1 {
		t.Fatalf("uncovered envelope: %v %v", err, result)
	}
	witness = readMapAt(t, root, witnessMap).Hunks[0].Coverage
	if witness == nil || witness.State != wire.CoverageUncovered || len(witness.Covered) != 0 || witness.Mode != "count" {
		t.Fatalf("uncovered witness = %+v", witness)
	}
	if text := reportTextAt(t, root, base, witnessMap); !strings.Contains(text, "downgraded from tested; reason `coverage-witness-uncovered`") {
		t.Fatalf("report with an uncovered witness:\n%s", text)
	}
}

// TestCoverRefusesAmbiguousAndInvalidInputs pins the TCQ-V0-051 failure
// modes: two profile paths matching one hunk path, a malformed profile, an
// empty test-run identity; a refusal leaves the map untouched.
func TestCoverRefusesAmbiguousAndInvalidInputs(t *testing.T) {
	root, base, target := makeGoRepo(t, importReorderTarget, whisperTarget)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	cover := func(profile, testRun string) error {
		writeFile(t, root, "cover.out", profile)
		_, err := openSession(t, root).Cover(ctx(), CoverOptions{
			MapPath: wire.ExcludedCEMPath, Coverprofile: "cover.out", TestRun: testRun,
		})
		return err
	}
	cases := map[string]struct{ profile, testRun, want string }{
		"ambiguous-path": {"mode: set\na/pkg/a.go:12.1,12.2 1 1\nb/pkg/a.go:12.1,12.2 1 1\n", "go test", "all match hunk path pkg/a.go"},
		"malformed":      {"example.com/m/pkg/a.go:12.1,12.2 1 1\n", "go test", "coverprofile: coverage mode is absent"},
		"empty-test-run": {"mode: set\n", "", "--test-run"},
	}
	for name, tc := range cases {
		err := cover(tc.profile, tc.testRun)
		if err == nil || cemcode.CodeOf(err) != cemcode.InvalidArguments || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want invalid-arguments containing %q", name, err, tc.want)
		}
	}
	if document := readMap(t, root); document.Spec != wire.Spec02 || document.Hunks[0].Coverage != nil {
		t.Fatalf("refused cover mutated the map: %+v", document)
	}
}
