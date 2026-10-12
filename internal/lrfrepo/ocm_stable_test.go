package lrfrepo

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
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
