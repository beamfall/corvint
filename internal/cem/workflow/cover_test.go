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

// witnessMap is where a test writes the witnessed cem/1.0 map, which never
// replaces the input map or lands on the Core sidecar path (V1-0335).
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
	document, err := wire.ParseStable(data)
	if err != nil {
		t.Fatal(err)
	}
	return stableMechanics(document)
}

// reportTextAt renders the stable report of one map with the repository root
// as its artifact root and the full target object id.
func reportTextAt(t *testing.T, root, base, target, mapPath string) string {
	t.Helper()
	result, err := openSession(t, root).Read(ctx(), "report", ReadOptions{
		MapPath: mapPath, ExpectedBase: base, Target: target, ArtifactRoot: root, ArtifactsGiven: true,
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
// never absent), the cem/1.0 map still verifies, and the stable review
// projection names each hunk's coverage state. The tcq/0 report downgrade
// is retired with the 0.x report (CEM-V1-217).
func TestCoverRecordsCoverageWitnessAndReportDowngrades(t *testing.T) {
	t.Parallel()
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
	if text := reportTextAt(t, root, base, target, wire.ExcludedCEMPath); !strings.Contains(text, "Coverage `NOT_PRODUCED`") {
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
	if document.Spec != wire.StableSpec || witness == nil || witness.State != wire.CoverageCovered ||
		witness.TestRun != "go test ./pkg/..." || witness.Mode != "set" || len(witness.Covered) != 1 ||
		witness.Covered[0] != (wire.Range{Start: 12, Count: 1}) {
		t.Fatalf("spec %s witness %+v", document.Spec, witness)
	}
	if status := stableStatus(t, root, witnessMap, base, target, PolicyLimits{}); stableOutcome(status) != "ACCEPT" {
		t.Fatalf("status after cover: %v", status)
	}
	if text := reportTextAt(t, root, base, target, witnessMap); !strings.Contains(text, "Coverage `covered`") {
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
	if text := reportTextAt(t, root, base, target, witnessMap); !strings.Contains(text, "Coverage `uncovered`") {
		t.Fatalf("report with an uncovered witness:\n%s", text)
	}
}

// TestCoverRefusesAmbiguousAndInvalidInputs pins the TCQ-V0-051 failure
// modes: two profile paths matching one hunk path, a malformed profile, an
// empty test-run identity; a refusal leaves the map untouched.
func TestCoverRefusesAmbiguousAndInvalidInputs(t *testing.T) {
	t.Parallel()
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
	if document := readMap(t, root); document.Spec != wire.StableSpec || document.Hunks[0].Coverage != nil {
		t.Fatalf("refused cover mutated the map: %+v", document)
	}
}
