package witness

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
)

// stableTargetTree is the tree of the packet's target commit; it holds the
// sealed map at .corvint/change.cem.json as the target sidecar.
const stableTargetTree = "4e37efcc75f598ed96569ba53aa8675be0fc6603"

func stableWitnessFixture(t *testing.T) (stableReadFixture, *contextindex.Index) {
	t.Helper()
	fixture := materializeStableRead(t)
	if runtime.Version() != fixture.acceptGo {
		t.Skipf("NOT_RUN: %s is not an admitted structural tuple for the public ACCEPT case", runtime.Version())
	}
	return fixture, &contextindex.Index{Root: fixture.repo, Revision: stableTargetTree, CommitRevision: fixture.target}
}

// TestStableWitnessSourceAdmitsVerifiedMap covers CEM-V1-070 and CEM-V1-072:
// an exact cem/1.0 sidecar is bound only through an ACCEPT verifier result,
// keeps the complete result with its map digest and NOT_OBSERVED axes, and its
// citations stay non-closing.
func TestStableWitnessSourceAdmitsVerifiedMap(t *testing.T) {
	fixture, index := stableWitnessFixture(t)
	source, document := readCEMSource(context.Background(), gitrun.NewDefaultBudget(), index,
		Options{Base: fixture.expectedBase, StableArtifacts: fixture.artifacts})
	if source.Status != "BOUND" || document == nil {
		t.Fatalf("stable source = %q %q, want BOUND", source.Status, source.Detail)
	}
	if source.Hunks != 2 || source.Basis != 1 {
		t.Fatalf("stable counts hunks=%d basis=%d, want 2 and 1", source.Hunks, source.Basis)
	}
	binding := source.Stable
	if binding == nil || binding.Profile != StableBindingProfile || binding.Verification == nil || binding.CriterionSatisfaction != "NOT_OBSERVED" {
		t.Fatalf("stable binding = %+v", binding)
	}
	result := binding.Verification
	if result.Outcome != "ACCEPT" || result.Profile != "cem-stable-verification/1" || result.Spec != "cem/1.0" || result.Sidecar != "EXACT" ||
		result.MapSha256 == nil || *result.MapSha256 != sha256Hex(fixture.raw) {
		t.Fatalf("stable verification = %+v", result)
	}
	for axis, want := range map[string]string{"changeIntegrity": "VERIFIED", "referenceIntegrity": "REFERENCE_INTEGRITY_ONLY",
		"nativeAuthority": "NOT_OBSERVED", "criterionAdequacy": "NOT_OBSERVED", "runnerExecution": "NOT_OBSERVED"} {
		if result.Axes[axis] != want {
			t.Fatalf("axis %s = %q, want %q", axis, result.Axes[axis], want)
		}
	}
	if artifacts, ok := result.References["artifacts"].([]wire.StableArtifact); len(result.Hunks) != 2 || !ok || len(artifacts) != 6 {
		t.Fatalf("verifier result lost hunks or references: %d hunks, %v", len(result.Hunks), result.References)
	}
	obligations := []Obligation{{ID: "change:app.txt", Kind: KindChange, Path: "app.txt"}, {ID: "change:sort.go", Kind: KindChange, Path: "sort.go"}}
	if unattached := attachWitnesses(obligations, source, document); unattached != 0 {
		t.Fatalf("unattached = %d", unattached)
	}
	if len(obligations[0].Witnesses) != 1 || obligations[0].Witnesses[0].Relation != "specification" || obligations[0].Witnesses[0].Closing {
		t.Fatalf("app.txt witnesses = %+v, want one non-closing specification citation", obligations[0].Witnesses)
	}
	report, err := Compile(context.Background(), index, Options{Base: fixture.expectedBase, StableArtifacts: fixture.artifacts})
	if err != nil || report.Profile != StableProfile || report.Sources[0].Status != "BOUND" || report.Summary.Closed != 0 {
		t.Fatalf("stable report: %v %+v", err, report)
	}
	encoded, err := json.Marshal(source)
	if err != nil || !bytes.Contains(encoded, []byte(`"mapSha256":"`+sha256Hex(fixture.raw)+`"`)) || !bytes.Contains(encoded, []byte(`"criterionSatisfaction":"NOT_OBSERVED"`)) {
		t.Fatalf("encoded source lost the stable binding: %s %v", encoded, err)
	}
}

// TestStableWitnessSourceRefusals covers CEM-V1-071: a missing artifact root,
// a wrong base, and a tampered artifact refuse with the full result retained
// and no citation applied.
func TestStableWitnessSourceRefusals(t *testing.T) {
	fixture, index := stableWitnessFixture(t)
	ctx, budget := context.Background(), gitrun.NewDefaultBudget()

	source, document := readCEMSource(ctx, budget, index, Options{Base: fixture.expectedBase})
	if source.Status != "UNSUPPORTED" || document != nil || source.Stable == nil || source.Stable.Verification != nil || source.Hunks != 2 {
		t.Fatalf("no artifact root: %+v", source)
	}

	wrongBase := strings.Repeat("0", 40)
	source, document = readCEMSource(ctx, budget, index, Options{Base: wrongBase, StableArtifacts: fixture.artifacts})
	if source.Status == "BOUND" || document != nil || source.Stable.Verification == nil || source.Stable.Verification.Outcome == "ACCEPT" {
		t.Fatalf("wrong base admitted: %+v", source)
	}

	entries, err := os.ReadDir(fixture.artifacts)
	if err != nil || len(entries) == 0 {
		t.Fatalf("artifacts: %v", err)
	}
	tampered := filepath.Join(fixture.artifacts, entries[0].Name())
	if err := os.WriteFile(tampered, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source, document = readCEMSource(ctx, budget, index, Options{Base: fixture.expectedBase, StableArtifacts: fixture.artifacts})
	result := source.Stable.Verification
	if source.Status != "INVALID" || document != nil || result == nil || result.Outcome != "REJECT" || result.MapSha256 == nil || *result.MapSha256 != sha256Hex(fixture.raw) {
		t.Fatalf("tampered artifact: %+v %+v", source, result)
	}
	if source.Hunks != 2 || source.Basis != 1 {
		t.Fatalf("refusal lost the map's counts: %+v", source)
	}
}

// TestStableWitnessReportProfile covers CEM-V1-072: a report built over a
// stable source takes the stable profile and keeps the no-bound-map
// precondition when the source is not admitted; a 0.x source keeps its profile.
func TestStableWitnessReportProfile(t *testing.T) {
	report := &Report{Profile: Profile, Sources: []Source{{Name: "cem", Status: "UNSUPPORTED", Stable: &StableBinding{}}}}
	found := false
	for _, item := range preconditions(report) {
		found = found || item.ID == "WITNESS-P3-NO-BOUND-EVIDENCE-MAP"
	}
	if !found {
		t.Fatal("an unadmitted stable source did not keep WITNESS-P3")
	}
	encoded, err := json.Marshal(Source{Name: "cem", Status: "BOUND"})
	if err != nil || bytes.Contains(encoded, []byte("stable")) {
		t.Fatalf("a 0.x source gained a stable field: %s", encoded)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
