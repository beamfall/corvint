package witnesscollapse

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func stableCollapseFixture(t *testing.T) (stableReadFixture, StableInputs) {
	t.Helper()
	fixture := materializeStableRead(t)
	if runtime.Version() != fixture.acceptGo {
		t.Skipf("NOT_RUN: %s is not an admitted structural tuple for the public ACCEPT case", runtime.Version())
	}
	return fixture, StableInputs{Repository: fixture.repo, ExpectedBase: fixture.expectedBase, Target: fixture.target, ArtifactRoot: fixture.artifacts}
}

// TestStableCollapseRetainsFullMap covers CEM-V1-073: an accepted stable map
// collapses under the stable profile, and the report links to the full map
// digest, every original hunk with its caller witness states, the artifact
// checks and the NOT_OBSERVED axes; caller mutation state is not criterion
// satisfaction. The report verifies by recompilation.
func TestStableCollapseRetainsFullMap(t *testing.T) {
	fixture, inputs := stableCollapseFixture(t)
	ctx := context.Background()
	report, err := CompileStable(ctx, fixture.raw, nil, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if report.Profile != StableProfile || report.CEM.Spec != wire.StableSpec || report.CEM.MapSHA256 != *report.Stable.Verification.MapSha256 ||
		report.CEM.BaseRevision != fixture.expectedBase || len(report.Hunks) != 2 || report.Claim != Claim {
		t.Fatalf("stable report = %+v", report)
	}
	binding := report.Stable
	if binding.CriterionSatisfaction != "NOT_OBSERVED" || binding.Verification.Outcome != "ACCEPT" || len(binding.Verification.ArtifactChecks) != 6 {
		t.Fatalf("stable binding = %+v", binding)
	}
	for _, axis := range []string{"criterionAdequacy", "criterionDiscrimination", "runnerExecution", "nativeAuthority"} {
		if binding.Verification.Axes[axis] != "NOT_OBSERVED" {
			t.Fatalf("axis %s = %q", axis, binding.Verification.Axes[axis])
		}
	}
	document, _, err := Canonical(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"state":"survived"`, `"killed":1`, `"survived":1`, `"criterion_satisfaction":"NOT_OBSERVED"`, `"runnerReceipts"`} {
		if !bytes.Contains(document, []byte(want)) {
			t.Fatalf("canonical stable report lost %s", want)
		}
	}
	if err := VerifyStable(ctx, fixture.raw, report, inputs); err != nil {
		t.Fatalf("verify stable: %v", err)
	}
	var decoded Report
	if err := json.Unmarshal(document, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := VerifyStable(ctx, fixture.raw, decoded, inputs); err != nil {
		t.Fatalf("a serialized stable report did not verify: %v", err)
	}
	decoded.Stable.CriterionSatisfaction = "SATISFIED"
	if err := VerifyStable(ctx, fixture.raw, decoded, inputs); CodeOf(err) != "report-mismatch" {
		t.Fatalf("a serialized report claiming criterion satisfaction verified: %v", err)
	}
	report.Hunks[0].RawWitnessCount++
	if err := VerifyStable(ctx, fixture.raw, report, inputs); CodeOf(err) != "report-mismatch" {
		t.Fatalf("edited report verified: %v", err)
	}
}

// TestStableCollapseRefusals covers CEM-V1-074 and CEM-V1-210: a retired
// cem/0.x map refuses with unsupported-spec and the fix line; a wrong base, a missing artifact root, edited map
// bytes and a tampered artifact refuse with the full verifier result retained
// and no collapse; an unknown witness still fails after admission.
func TestStableCollapseRefusals(t *testing.T) {
	fixture, inputs := stableCollapseFixture(t)
	ctx := context.Background()
	if _, err := CompileStable(ctx, []byte(`{"spec":"cem/0.2"}`), nil, inputs); CodeOf(err) != cemcode.UnsupportedSpec ||
		!strings.Contains(err.Error(), wire.RetiredSpecFix) {
		t.Fatalf("a retired map was not refused with unsupported-spec and the fix line: %v", err)
	}
	if _, err := CompileStable(ctx, []byte(`not json`), nil, inputs); CodeOf(err) != "not-stable" {
		t.Fatalf("a non-map input error=%v, want not-stable", err)
	}
	refused := func(name string, raw []byte, inputs StableInputs) {
		t.Helper()
		report, err := CompileStable(ctx, raw, nil, inputs)
		if CodeOf(err) != "stable-verification-refused" || report.Stable == nil || report.Stable.Verification.Outcome == "ACCEPT" ||
			report.Hunks != nil || report.Groups != nil {
			t.Fatalf("%s: %v %+v", name, err, report)
		}
	}
	wrongBase := inputs
	wrongBase.ExpectedBase = strings.Repeat("0", 40)
	refused("wrong base", fixture.raw, wrongBase)
	noArtifacts := inputs
	noArtifacts.ArtifactRoot = ""
	refused("no artifact root", fixture.raw, noArtifacts)
	refused("edited map", bytes.Replace(fixture.raw, []byte("caller metadata only"), []byte("caller metadata ONLY"), 1), inputs)

	subject := strings.Repeat("a", 64)
	unknown := "evidence:sha256:" + strings.Repeat("f", 64)
	if _, err := CompileStable(ctx, fixture.raw, []Attribution{{WitnessEvidenceID: unknown, Kind: CauseGenerator, CauseSubjectSHA256: subject, AttestationEvidenceID: unknown}}, inputs); CodeOf(err) != "unknown-witness" {
		t.Fatalf("unknown witness: %v", err)
	}

	report, err := CompileStable(ctx, fixture.raw, nil, inputs)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(fixture.artifacts)
	if err != nil || len(entries) == 0 {
		t.Fatalf("artifacts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixture.artifacts, entries[0].Name()), []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	refused("tampered artifact", fixture.raw, inputs)
	if err := VerifyStable(ctx, fixture.raw, report, inputs); CodeOf(err) != "stable-verification-refused" {
		t.Fatalf("a report verified after its artifact drifted: %v", err)
	}
}
