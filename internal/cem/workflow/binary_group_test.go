package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// binaryFixtureRange commits, over a base holding a text file and a binary
// fixture, a target that edits the text file, modifies the fixture and adds
// a second binary fixture, and returns the range.
func binaryFixtureRange(t *testing.T) (root, base, target string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "t.txt", "a\nkeep\n")
	writeFile(t, root, "testdata/logo.bin", "logo\x00v1\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base = gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "t.txt", "b\nkeep\n")
	writeFile(t, root, "testdata/logo.bin", "logo\x00v2-longer\n")
	writeFile(t, root, "testdata/new.bin", "new\x00payload\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-qm", "target")
	return root, base, gitCmd(t, root, "rev-parse", "HEAD")
}

// TestCEMV1983PrepareBindsBinaryFixture is the V1-2752 regression (AC1, AC2;
// CEM-V1-980, CEM-V1-981, CEM-V1-983): prepare over a range that changes
// binary fixtures beside a text file succeeds, maps only the textual hunk,
// and reports each binary path with its change, mode, both blob object IDs
// and sizes as an excluded row. The native stable verifier accepts the map,
// and a text-only prepare carries no binary member.
func TestCEMV1983PrepareBindsBinaryFixture(t *testing.T) {
	root, base, target := binaryFixtureRange(t)
	result, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target})
	if err != nil {
		t.Fatalf("Prepare over text plus binary fixtures: %v", err)
	}
	logoOld := gitCmd(t, root, "rev-parse", base+":testdata/logo.bin")
	logoNew := gitCmd(t, root, "rev-parse", target+":testdata/logo.bin")
	newBin := gitCmd(t, root, "rev-parse", target+":testdata/new.bin")
	rows, _ := json.Marshal(result["binary"])
	want := `[{"baseBlobOid":"` + logoOld + `","baseSize":8,"change":"modify","disposition":"excluded","mode":"100644","path":"testdata/logo.bin","targetBlobOid":"` + logoNew + `","targetSize":15},` +
		`{"baseBlobOid":null,"baseSize":null,"change":"create","disposition":"excluded","mode":"100644","path":"testdata/new.bin","targetBlobOid":"` + newBin + `","targetSize":12}]`
	if string(rows) != want {
		t.Fatalf("binary rows\n got %s\nwant %s", rows, want)
	}
	_, document, err := openSession(t, root).readProducerMap(wire.ExcludedCEMPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Hunks) != 1 || document.Hunks[0].Path != "t.txt" {
		t.Fatalf("mapped hunks %+v, want only the t.txt hunk", document.Hunks)
	}

	raw, err := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
	if err != nil {
		t.Fatal(err)
	}
	verdict, _ := verify.Stable(ctx(), raw, verify.StableOptions{Repository: root, ExpectedBase: base, Target: target, ArtifactRoot: root})
	encoded, _ := json.Marshal(verdict)
	if verdict.PatchSha256 == nil || *verdict.PatchSha256 != document.PatchSha256 {
		t.Fatalf("stable verify did not bind the prepared patch: %s", encoded)
	}
	if verdict.Stage != "complete" || verdict.Outcome != "ACCEPT" || verdict.Code != nil {
		t.Fatalf("stable verify did not accept the map over binary sections: %s", encoded)
	}

	textRoot, textBase, textTarget := binaryFixtureRange(t)
	gitCmd(t, textRoot, "reset", "-q", "--hard", textBase)
	writeFile(t, textRoot, "t.txt", "c\n")
	gitCmd(t, textRoot, "commit", "-qam", "text only")
	textTarget = gitCmd(t, textRoot, "rev-parse", "HEAD")
	textResult, err := openSession(t, textRoot).Prepare(ctx(), PrepareOptions{Base: textBase, Target: textTarget})
	if err != nil {
		t.Fatal(err)
	}
	if _, present := textResult["binary"]; present {
		t.Fatalf("text-only prepare result has a binary member: %v", textResult)
	}
}

// TestCEMV1984CiteRefusesBinaryPath pins CEM-V1-984 (V1-2752): a citation
// whose evidence span lies in a binary path the patch changes refuses
// cite-span-not-stable naming the path, and a citation into a text file of
// the same map still succeeds.
func TestCEMV1984CiteRefusesBinaryPath(t *testing.T) {
	root, base, target := binaryFixtureRange(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, document, err := openSession(t, root).readProducerMap(wire.ExcludedCEMPath)
	if err != nil {
		t.Fatal(err)
	}
	hunk := document.Hunks[0].ID
	_, err = openSession(t, root).Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: hunk, EvidencePath: "testdata/logo.bin", Bytes: "0:4", Relation: "specification",
	})
	if cemcode.CodeOf(err) != cemcode.CiteSpanNotStable || !strings.Contains(cemcode.MessageOf(err), `"testdata/logo.bin"`) ||
		!strings.Contains(cemcode.MessageOf(err), "CEM-V1-984") {
		t.Fatalf("cite into a changed binary path: %v", err)
	}
	if _, err := openSession(t, root).Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: hunk, EvidencePath: "t.txt", Lines: "2:2", Relation: "specification",
	}); err != nil {
		t.Fatalf("cite into the text file of a map with binary groups: %v", err)
	}
}

// TestCEMV1985OverBoundBinaryRefuses pins CEM-V1-985 (V1-2752): a binary
// fixture whose blob exceeds gitauth.MaxBlobBytes is never bound silently.
// Prepare refuses repository-object-unavailable naming the path before any
// map is written, and a map begun over the same canonical patch is refused
// with the same code by the candidate (`cem verify`) and stable verifiers,
// although binary bodies are never read and a creation's base side is absent.
func TestCEMV1985OverBoundBinaryRefuses(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "t.txt", "a\nkeep\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "t.txt", "b\nkeep\n")
	writeFile(t, root, "testdata/big.bin", "\x00"+strings.Repeat("x", gitauth.MaxBlobBytes))
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-qm", "target")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	overBound := func(stage string, err error) {
		t.Helper()
		if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable ||
			!strings.Contains(cemcode.MessageOf(err), `"testdata/big.bin"`) || !strings.Contains(cemcode.MessageOf(err), "CEM-V1-985") {
			t.Fatalf("%s over a %d-byte binary blob: %v", stage, gitauth.MaxBlobBytes+1, err)
		}
	}

	_, err = openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target})
	overBound("prepare", err)
	if _, statErr := os.Stat(filepath.Join(root, wire.ExcludedCEMPath)); !os.IsNotExist(statErr) {
		t.Fatalf("refused prepare left a map: %v", statErr)
	}

	session := openSession(t, root)
	if err := session.openRepository(); err != nil {
		t.Fatal(err)
	}
	patchBytes, err := session.repository.CanonicalDiff(ctx(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	patchPath := filepath.Join(t.TempDir(), "change.patch")
	if err := os.WriteFile(patchPath, patchBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Begin(ctx(), BeginOptions{PatchPath: patchPath, Base: base, Output: wire.ExcludedCEMPath}); err != nil {
		t.Fatalf("begin over the canonical patch: %v", err)
	}
	_, document, err := openSession(t, root).readProducerMap(wire.ExcludedCEMPath)
	if err != nil {
		t.Fatal(err)
	}
	overBound("candidate verify", verify.Candidate(ctx(), session.repository, document, patchBytes))

	raw, err := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
	if err != nil {
		t.Fatal(err)
	}
	verdict, _ := verify.Stable(ctx(), raw, verify.StableOptions{Repository: root, ExpectedBase: base, Target: target, ArtifactRoot: root})
	if encoded, _ := json.Marshal(verdict); verdict.Code == nil || *verdict.Code != cemcode.RepositoryObjectUnavailable {
		t.Fatalf("stable verify over a %d-byte binary blob: %s", gitauth.MaxBlobBytes+1, encoded)
	}
}
