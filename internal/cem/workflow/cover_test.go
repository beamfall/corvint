package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/coverprofile"
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
	if err != nil || result["covered"] != 0 || result["uncovered"] != 1 || result["notInstrumented"] != 0 {
		t.Fatalf("uncovered envelope: %v %v", err, result)
	}
	witness = readMapAt(t, root, witnessMap).Hunks[0].Coverage
	if witness == nil || witness.State != wire.CoverageUncovered || len(witness.Covered) != 0 || witness.Mode != "count" {
		t.Fatalf("uncovered witness = %+v", witness)
	}
	if text := reportTextAt(t, root, base, target, witnessMap); !strings.Contains(text, "Coverage `uncovered`") {
		t.Fatalf("report with an uncovered witness:\n%s", text)
	}

	// TCQ-V0-091: a hunk whose path the profile never names is uncovered in
	// the map, and the envelope names it not-instrumented apart from an
	// instrumented hunk no block reaches.
	writeFile(t, root, "cover.out", "mode: set\nexample.com/m/other/b.go:1.1,1.20 1 1\n")
	result, err = openSession(t, root).Cover(ctx(), CoverOptions{
		MapPath: wire.ExcludedCEMPath, Coverprofile: "cover.out", TestRun: "go test ./other/...", Output: witnessMap,
	})
	if err != nil || result["covered"] != 0 || result["uncovered"] != 1 || result["notInstrumented"] != 1 {
		t.Fatalf("not-instrumented envelope: %v %v", err, result)
	}
	if hunks, ok := result["notInstrumentedHunks"].([]string); !ok || len(hunks) != 1 || hunks[0] != document.Hunks[0].ID {
		t.Fatalf("notInstrumentedHunks = %#v", result["notInstrumentedHunks"])
	}
	if witness = readMapAt(t, root, witnessMap).Hunks[0].Coverage; witness == nil || witness.State != wire.CoverageUncovered {
		t.Fatalf("not-instrumented witness = %+v", witness)
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
		// TCQ-V0-097: a zero-count match still makes the hunk path ambiguous.
		"ambiguous-zero-count": {"mode: set\na/pkg/a.go:12.1,12.2 1 0\nb/pkg/a.go:12.1,12.2 1 1\n", "go test", "coverprofile paths a/pkg/a.go, b/pkg/a.go all match hunk path pkg/a.go"},
		"malformed":            {"example.com/m/pkg/a.go:12.1,12.2 1 1\n", "go test", "coverprofile: coverage mode is absent"},
		"empty-test-run":       {"mode: set\n", "", "--test-run"},
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

// TestTCQV0065CoverAcceptsANonGoLineProfile checks TCQ-V0-065: cover reads
// line coverage from any language once it is written in the coverprofile line
// format, so a Ruby hunk gets a covered witness from a converted profile and an
// uncovered one when its added line has a zero count. The witness is coverage
// only; it is not an execution or discrimination witness.
func TestTCQV0065CoverAcceptsANonGoLineProfile(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "lib/greet.rb", "def greet(name)\n  \"hi #{name}\"\nend\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "lib/greet.rb", "def greet(name)\n  \"hello #{name}\"\nend\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		profile string
		state   string
	}{
		{"mode: set\nlib/greet.rb:1.1,1.16 1 1\nlib/greet.rb:2.1,2.19 1 1\n", wire.CoverageCovered},
		{"mode: count\nlib/greet.rb:1.1,1.16 1 3\nlib/greet.rb:2.1,2.19 1 0\n", wire.CoverageUncovered},
	} {
		writeFile(t, root, "ruby-cover.out", c.profile)
		if _, err := openSession(t, root).Cover(ctx(), CoverOptions{
			MapPath: wire.ExcludedCEMPath, Coverprofile: "ruby-cover.out", TestRun: "bundle exec rspec", Output: witnessMap,
		}); err != nil {
			t.Fatalf("cover over a Ruby profile: %v", err)
		}
		document := readMapAt(t, root, witnessMap)
		if len(document.Hunks) != 1 || document.Hunks[0].Path != "lib/greet.rb" {
			t.Fatalf("hunks = %+v", document.Hunks)
		}
		if witness := document.Hunks[0].Coverage; witness == nil || witness.State != c.state || witness.TestRun != "bundle exec rspec" {
			t.Fatalf("Ruby witness = %+v, want %s", witness, c.state)
		}
	}
}

// TestTCQV0097CoverKeepsCoveredLinesAsIntervals checks TCQ-V0-097: a block
// spanning lines 1 to 4294967295 is intersected with the added lines as an
// interval, so cover completes within a stated bound (2 s) and records the
// covered added line, and a cancelled context refuses without writing.
func TestTCQV0097CoverKeepsCoveredLinesAsIntervals(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "lib/greet.rb", "def greet(name)\n  \"hi #{name}\"\nend\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "lib/greet.rb", "def greet(name)\n  \"hello #{name}\"\nend\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "wide.out", "mode: set\nlib/greet.rb:1.1,4294967295.2 1 1\n")
	cancelled, cancel := context.WithCancel(ctx())
	cancel()
	if _, err := openSession(t, root).Cover(cancelled, CoverOptions{
		MapPath: wire.ExcludedCEMPath, Coverprofile: "wide.out", TestRun: "bundle exec rspec", Output: witnessMap,
	}); err == nil {
		t.Fatal("cover ignored a cancelled context")
	}
	started := time.Now()
	if _, err := openSession(t, root).Cover(ctx(), CoverOptions{
		MapPath: wire.ExcludedCEMPath, Coverprofile: "wide.out", TestRun: "bundle exec rspec", Output: witnessMap,
	}); err != nil {
		t.Fatalf("cover over a 2^32-1 line block: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("cover over a 2^32-1 line block took %v, want under 2s", elapsed)
	}
	witness := readMapAt(t, root, witnessMap).Hunks[0].Coverage
	if witness == nil || witness.State != wire.CoverageCovered || len(witness.Covered) != 1 || witness.Covered[0] != (wire.Range{Start: 2, Count: 1}) {
		t.Fatalf("witness = %+v, want covered line 2", witness)
	}
}

// TestTCQV0097CoveredRangesIntersectMergedIntervals checks the interval
// intersection against the line-set semantics it replaces: overlapping and
// adjacent blocks merge, zero-count blocks cover nothing, and added lines fold
// into ascending, non-adjacent ranges.
func TestTCQV0097CoveredRangesIntersectMergedIntervals(t *testing.T) {
	t.Parallel()
	blocks := []coverprofile.Block{
		{ProfilePath: "m/a.go", StartLine: 10, EndLine: 12, Count: 1},
		{ProfilePath: "m/a.go", StartLine: 3, EndLine: 4, Count: 2},
		{ProfilePath: "m/a.go", StartLine: 5, EndLine: 5, Count: 1},
		{ProfilePath: "m/a.go", StartLine: 11, EndLine: 20, Count: 1},
		{ProfilePath: "m/a.go", StartLine: 30, EndLine: 40, Count: 0},
		{ProfilePath: "m/zero.go", StartLine: 1, EndLine: 9, Count: 0},
	}
	byPath := coveredIntervalsByPath(blocks)
	if got, want := byPath["m/a.go"], []lineInterval{{3, 5}, {10, 20}}; !slices.Equal(got, want) {
		t.Fatalf("intervals = %v, want %v", got, want)
	}
	if got, ok := byPath["m/zero.go"]; !ok || len(got) != 0 {
		t.Fatalf("zero-count path = %v, %v; want a key with no intervals", got, ok)
	}
	got := coveredRanges([]int64{1, 3, 4, 5, 6, 10, 11, 21, 35}, byPath["m/a.go"])
	want := []wire.Range{{Start: 3, Count: 3}, {Start: 10, Count: 2}}
	if !slices.Equal(got, want) {
		t.Fatalf("coveredRanges = %v, want %v", got, want)
	}
	if got := coveredRanges([]int64{1}, nil); got == nil || len(got) != 0 {
		t.Fatalf("coveredRanges with no intervals = %#v, want non-nil empty", got)
	}
}
