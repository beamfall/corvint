package lrfrepo

import (
	"bytes"
	"context"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/testplatform"
)

// prepareStableCEM writes the cem/1.0 producer map for base..target at the
// excluded sidecar path.
func prepareStableCEM(t *testing.T, root, base, target string) []byte {
	t.Helper()
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Prepare(context.Background(), workflow.PrepareOptions{
		Base: base, Target: target, Spec: wire.StableSpec,
	}); err != nil {
		testplatform.SkipWindowsRefusal(t, err)
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
	if err != nil || !wire.DeclaresStable(raw) {
		t.Fatalf("stable producer map: err=%v bytes=%.80s", err, raw)
	}
	return raw
}

// TestCEMV1149OCMPrepareAdmitsStableProducerMap (CEM-V1-149): OCM prepare and
// status admit a cem/1.0 producer map the stable verifier accepts, binding the
// same base, target and patch digest as the map.
func TestCEMV1149OCMPrepareAdmitsStableProducerMap(t *testing.T) {
	root, base, target := makeOCMPrepareRepository(t)
	raw := prepareStableCEM(t, root, base, target)
	document, err := wire.ParseStable(raw)
	if err != nil {
		t.Fatal(err)
	}
	options := PrepareOptions{
		MapPath: DefaultOCMMapPath, CEMPath: wire.ExcludedCEMPath,
		IntentPath: "docs/specs/intent.md", ExpectedBase: base, Target: target,
	}
	prepared, err := PrepareOCM(context.Background(), root, options)
	if err != nil || prepared.Resumed || prepared.Target != target {
		t.Fatalf("stable OCM prepare: err=%v", err)
	}
	ocm, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(DefaultOCMMapPath)))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"targetRevision":"` + target + `"`, `"patchSha256":"` + document.Change.PatchSha256 + `"`} {
		if !strings.Contains(string(ocm), want) {
			t.Fatalf("OCM map lacks %s: %s", want, ocm)
		}
	}
	result, err := ReadOCM(context.Background(), root, OCMReadOptions{
		OCMPath: DefaultOCMMapPath, CEMPath: wire.ExcludedCEMPath,
		ExpectedBase: base, Target: target, ExpectedBaseGiven: true, TargetGiven: true,
	})
	if err != nil || result == nil || result.State == "invalid" || result.VerificationCause != nil {
		t.Fatalf("stable OCM status: err=%v result=%+v", err, result)
	}
	resumed, err := PrepareOCM(context.Background(), root, options)
	if err != nil || !resumed.Resumed {
		t.Fatalf("stable OCM resume: err=%v", err)
	}
}

// TestCEMV1149OCMPrepareRefusesUnverifiedStableMap is the refusal control: a
// stable map whose base differs from the expected base, or whose bytes the
// stable verifier rejects, writes no OCM map.
func TestCEMV1149OCMPrepareRefusesUnverifiedStableMap(t *testing.T) {
	root, base, target := makeOCMPrepareRepository(t)
	raw := prepareStableCEM(t, root, base, target)
	options := PrepareOptions{
		MapPath: DefaultOCMMapPath, CEMPath: wire.ExcludedCEMPath,
		IntentPath: "docs/specs/intent.md", ExpectedBase: target, Target: target,
	}
	if _, err := PrepareOCM(context.Background(), root, options); CodeOf(err) != "base-revision-mismatch" {
		t.Fatalf("expected-base mismatch: code=%q err=%v", CodeOf(err), err)
	}
	tampered := bytes.Replace(raw, []byte(`"disposition":"unknown"`), []byte(`"disposition":"mechanical"`), 1)
	if bytes.Equal(tampered, raw) {
		t.Fatal("fixture has no unknown hunk to tamper")
	}
	writeOCMPrepareFile(t, root, wire.ExcludedCEMPath, string(tampered))
	options.ExpectedBase = base
	if _, err := PrepareOCM(context.Background(), root, options); CodeOf(err) == "" {
		t.Fatal("ocm prepare admitted a stable map the verifier does not accept")
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(DefaultOCMMapPath))); !os.IsNotExist(err) {
		t.Fatalf("refused prepare wrote an OCM map: %v", err)
	}
}

// TestCEMV1149OCMLinkAdmitsStableProducerMap (CEM-V1-149): ocm link over a
// cem/1.0 producer map links a stable hunk ID to a pinned test claim, and the
// linked OCM map still reads valid against the same stable map.
func TestCEMV1149OCMLinkAdmitsStableProducerMap(t *testing.T) {
	root, base, _ := makeOCMPrepareRepository(t)
	const testPath = "example_test.go"
	source := "package example\nimport \"testing\"\nfunc TestAnchors(t *testing.T) {\n t.Run(\"OCM-TEST-001 exact anchor\", func(t *testing.T) {})\n}\n"
	writeOCMPrepareFile(t, root, testPath, source)
	gitOCMPrepare(t, root, "add", testPath)
	gitOCMPrepare(t, root, "commit", "-qm", "test anchors")
	target := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	prepareStableCEM(t, root, base, target)
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Cite(context.Background(), workflow.CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "docs/specs/intent.md", Lines: "5:5", Relation: "specification",
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
	if err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParseStable(raw)
	if err != nil || len(document.Change.Hunks) == 0 {
		t.Fatalf("stable map: err=%v", err)
	}
	options := LinkOptions{
		MapPath: DefaultOCMMapPath, CEMPath: wire.ExcludedCEMPath, Obligation: "OCM-TEST-001",
		Hunks: []string{document.Change.Hunks[0].ID}, TestPath: testPath, ExpectedBase: base, Target: target,
	}
	if _, err := PrepareOCM(context.Background(), root, PrepareOptions{
		MapPath: options.MapPath, CEMPath: options.CEMPath, IntentPath: "docs/specs/intent.md", ExpectedBase: base, Target: target,
	}); err != nil {
		t.Fatal(err)
	}
	claims, err := enumerateClaims(testPath, gitOCMPrepare(t, root, "rev-parse", target+":"+testPath), []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	options.Claims = []string{caseClaim(t, claims).selector}
	linked, err := LinkOCM(context.Background(), root, options)
	if err != nil || len(linked.HunkIDs) != 1 || linked.HunkIDs[0] != document.Change.Hunks[0].ID {
		t.Fatalf("stable OCM link: err=%v result=%+v", err, linked)
	}
	checked, err := ReadOCM(context.Background(), root, OCMReadOptions{
		OCMPath: options.MapPath, CEMPath: options.CEMPath, ExpectedBase: base, Target: target,
		ExpectedBaseGiven: true, TargetGiven: true,
	})
	if err != nil || checked.Verification["valid"] != true {
		t.Fatalf("linked stable OCM read: err=%v verification=%v", err, checked)
	}
}

// TestCEMV1210OCMRefusesRetiredMap (CEM-V1-210): OCM prepare, link and
// status refuse a retired cem/0.x map with unsupported-spec and the recovery
// line, never invalid-cem or a silent fallback, and leave the OCM map as it was.
func TestCEMV1210OCMRefusesRetiredMap(t *testing.T) {
	root, base, target := makeOCMPrepareRepository(t)
	raw := prepareStableCEM(t, root, base, target)
	prepare := PrepareOptions{
		MapPath: DefaultOCMMapPath, CEMPath: wire.ExcludedCEMPath,
		IntentPath: "docs/specs/intent.md", ExpectedBase: base, Target: target,
	}
	if _, err := PrepareOCM(context.Background(), root, prepare); err != nil {
		t.Fatal(err)
	}
	ocmPath := filepath.Join(root, filepath.FromSlash(DefaultOCMMapPath))
	before, err := os.ReadFile(ocmPath)
	if err != nil {
		t.Fatal(err)
	}
	retired := bytes.Replace(raw, []byte(`"spec":"cem/1.0"`), []byte(`"spec":"cem/0.2"`), 1)
	if bytes.Equal(retired, raw) {
		t.Fatal("fixture map does not declare cem/1.0")
	}
	writeOCMPrepareFile(t, root, wire.ExcludedCEMPath, string(retired))
	refused := func(name string, err error) {
		t.Helper()
		if CodeOf(err) != cemcode.UnsupportedSpec || !strings.Contains(err.Error(), wire.RetiredSpecFix) {
			t.Fatalf("%s admitted a retired map: code=%q err=%v", name, CodeOf(err), err)
		}
	}
	prepare.Replace = true
	_, err = PrepareOCM(context.Background(), root, prepare)
	refused("prepare", err)
	_, err = LinkOCM(context.Background(), root, LinkOptions{
		MapPath: DefaultOCMMapPath, CEMPath: wire.ExcludedCEMPath, Obligation: "OCM-TEST-001",
		Hunks: []string{"h"}, ExpectedBase: base, Target: target,
	})
	refused("link", err)
	result, err := ReadOCM(context.Background(), root, OCMReadOptions{
		OCMPath: DefaultOCMMapPath, CEMPath: wire.ExcludedCEMPath,
		ExpectedBase: base, Target: target, ExpectedBaseGiven: true, TargetGiven: true,
	})
	if err != nil || result == nil || result.State != "invalid" {
		t.Fatalf("status over a retired map: err=%v result=%+v", err, result)
	}
	refused("status", result.VerificationCause)
	if after, err := os.ReadFile(ocmPath); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("a refused write changed the OCM map: %v", err)
	}
}
