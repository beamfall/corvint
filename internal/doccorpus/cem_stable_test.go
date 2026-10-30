package doccorpus

import (
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// stablePolicyBlob is the base blob of policy.txt in the public packet; the
// sealed map's only evidence item cites its single line.
const (
	stablePolicyBlob     = "36e5ee3e49a2a60b602d0c7f2ce5453280af1066"
	stablePolicyLine     = "Change value under criterion A.\n"
	stablePolicyEvidence = "evidence:sha256:88b6a51668e594464e0afab9f198c7cabf93a7f8383d33f4ec5ea93cbe451a49"
)

func stableDocFixture(t *testing.T) (stableReadFixture, StableCEMInputs, *Artifact) {
	t.Helper()
	fixture := materializeStableRead(t)
	if runtime.Version() != fixture.acceptGo {
		t.Skipf("NOT_RUN: %s is not an admitted structural tuple for the public ACCEPT case", runtime.Version())
	}
	anchor := Anchor{Path: "policy.txt", Blob: stablePolicyBlob, Start: 1, End: 1, SpanSHA256: Digest([]byte(stablePolicyLine))}
	artifact := &Artifact{Schema: Schema, SHA256: strings.Repeat("a", 64), Claims: []Claim{{ID: "claim-policy", Text: "generated summary",
		Evidence: Evidence{Derivation: "generated", Trust: "generated", Anchors: []Anchor{anchor}}}}}
	return fixture, StableCEMInputs{Repository: fixture.repo, ExpectedBase: fixture.expectedBase, Target: fixture.target, ArtifactRoot: fixture.artifacts}, artifact
}

// TestStableCEMProjectionCitesVerifiedSpan covers CEM-V1-075 and CEM-V1-076:
// a documentation claim projects through an accepted stable map to its
// original immutable span, bound by evidence identity, with the full map
// digest and spec, the unmodified verifier result, reference counts and
// NOT_OBSERVED authority; no writer argv is offered and no map is encoded.
func TestStableCEMProjectionCitesVerifiedSpan(t *testing.T) {
	t.Parallel()
	fixture, inputs, artifact := stableDocFixture(t)
	receipt, err := StableCEMProjection(context.Background(), artifact, fixture.raw, "", inputs)
	if err != nil {
		t.Fatal(err)
	}
	if receipt["schema"] != StableCEMSchema || receipt["outcome"] != "PROJECTED" || receipt["cem_spec"] != wire.StableSpec ||
		receipt["cem_sha256"] != Digest(fixture.raw) || receipt["base_revision"] != fixture.expectedBase || receipt["native_authority"] != "NOT_OBSERVED" || receipt["mutates"] != false {
		t.Fatalf("stable projection receipt = %v", receipt)
	}
	result := receipt["cem_verification"].(verify.StableResult)
	if result.Outcome != "ACCEPT" || result.Profile != "cem-stable-verification/1" || *result.MapSha256 != Digest(fixture.raw) || result.Axes["nativeAuthority"] != "NOT_OBSERVED" {
		t.Fatalf("stable projection verification = %+v", result)
	}
	references := receipt["references"].(map[string]any)
	if references["artifacts"] != 6 || references["criterion_links"] != 1 || references["semantics"] != "REFERENCE_INTEGRITY_ONLY" {
		t.Fatalf("stable projection references = %v", references)
	}
	rows := receipt["claims"].([]any)
	if len(rows) != 1 {
		t.Fatalf("claims = %v", rows)
	}
	row := rows[0].(map[string]any)
	if row["evidence_id"] != stablePolicyEvidence || row["bound_in_cem"] != true {
		t.Fatalf("claim row = %v", row)
	}
	if _, ok := row["cite"]; ok {
		t.Fatalf("stable projection offered a writer argv: %v", row)
	}
	encoded, err := Encode(receipt)
	if err != nil || strings.Contains(string(encoded), `"spec":"cem/1.0","baseRevision"`) {
		t.Fatalf("stable projection encoded a map or failed: %v", err)
	}
	if err := RefuseCEMProjection(fixture.raw); err == nil || !strings.Contains(err.Error(), "docs corpus cem-stable") {
		t.Fatalf("the retired projection did not name the stable operation: %v", err)
	}
	var retired *Error
	if err := RefuseCEMProjection([]byte(`{"spec":"cem/0.2"}`)); !errors.As(err, &retired) || retired.Code != cemcode.UnsupportedSpec || !strings.Contains(retired.Message, wire.RetiredSpecFix) {
		t.Fatalf("a cem/0.x map was not refused with the retired-spec fix (CEM-V1-210): %v", err)
	}
}

// TestStableCEMProjectionRefusals covers CEM-V1-075 and CEM-V1-076: a wrong
// base, edited map bytes, a tampered artifact, a missing artifact root and a
// 0.x map refuse with no claim row; the verifier result is kept; a claim whose
// span differs at the verified base refuses instead of citing.
func TestStableCEMProjectionRefusals(t *testing.T) {
	t.Parallel()
	fixture, inputs, artifact := stableDocFixture(t)
	ctx := context.Background()
	refused := func(name string, data []byte, inputs StableCEMInputs) {
		t.Helper()
		receipt, err := StableCEMProjection(ctx, artifact, data, "", inputs)
		var corpusErr *Error
		if !errors.As(err, &corpusErr) || corpusErr.Code != "corpus-stable-refused" || receipt["outcome"] != "REFUSED" || len(receipt["claims"].([]any)) != 0 {
			t.Fatalf("%s: %v %v", name, err, receipt)
		}
		if retired := strings.Contains(string(data), `"spec":"cem/0.`); retired != strings.Contains(corpusErr.Message, wire.RetiredSpecFix) {
			t.Fatalf("%s: message %q names the recovery line=%v, want %v (CEM-V1-210)", name, corpusErr.Message, !retired, retired)
		}
		if result := receipt["cem_verification"].(verify.StableResult); result.Outcome == "ACCEPT" {
			t.Fatalf("%s: refused receipt carries an ACCEPT result", name)
		}
	}
	wrongBase := inputs
	wrongBase.ExpectedBase = strings.Repeat("0", 40)
	refused("wrong base", fixture.raw, wrongBase)
	noArtifacts := inputs
	noArtifacts.ArtifactRoot = ""
	refused("no artifact root", fixture.raw, noArtifacts)
	refused("edited map", []byte(strings.Replace(string(fixture.raw), "caller metadata only", "caller metadata ONLY", 1)), inputs)
	refused("0.x map", []byte(`{"spec":"cem/0.3"}`), inputs)
	refused("case-variant spec", []byte(`{"spec":"cem/0.3","Spec":"cem/1.0"}`), inputs)

	edited := *artifact
	edited.Claims = []Claim{artifact.Claims[0]}
	edited.Claims[0].Evidence.Anchors = []Anchor{artifact.Claims[0].Evidence.Anchors[0]}
	edited.Claims[0].Evidence.Anchors[0].SpanSHA256 = Digest([]byte("edited\n"))
	if receipt, err := StableCEMProjection(ctx, &edited, fixture.raw, "", inputs); err == nil || receipt != nil {
		t.Fatalf("an edited claim span projected: %v", receipt)
	}

	entries, err := os.ReadDir(fixture.artifacts)
	if err != nil || len(entries) == 0 {
		t.Fatalf("artifacts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixture.artifacts, entries[0].Name()), []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	refused("tampered artifact", fixture.raw, inputs)
}
