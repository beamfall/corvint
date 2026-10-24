package frontiernextrepo

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/frontiernext"
	"github.com/Beamfall/corvint/internal/localauthority"
	"github.com/Beamfall/corvint/internal/testplatform"
)

const intent = "# Intent\n\n## Requirements\n\n- `PLE-V0-005`: `Encode` canonical encoding preserves string bytes.\n\n## Next\n"
const claimAnchor = "PLE-V0-005 canonical vectors"

const claimSource = "package tests\nimport \"testing\"\nfunc TestCodec(t *testing.T) {\n _ = []struct{ name string }{{name: \"PLE-V0-005 canonical vectors\"}}\n}\n"

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git: %v %s", err, out)
	}
	return strings.TrimSpace(string(out))
}
func write(t *testing.T, root, p string, b []byte) {
	t.Helper()
	full := filepath.Join(root, p)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, b, 0644); err != nil {
		t.Fatal(err)
	}
}
func fixtureRequest(t *testing.T, source, driver []byte, mutant bool) (string, frontiernext.Request, localauthority.PolicySnapshot) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q", "-b", "main")
	write(t, root, "docs/intent.md", []byte(intent))
	write(t, root, "tests/codec_test.go", []byte(claimSource))
	write(t, root, "tools/local-authority/driver.go", driver)
	write(t, root, "internal/wp3codec/codec.go", source)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "fixture independent base")
	base := git(t, root, "rev-parse", "HEAD")
	candidate := bytes.Replace(source, []byte("return out.Bytes(), nil"), []byte("encoded := out.Bytes()\n return encoded, nil"), 1)
	if mutant {
		candidate = bytes.Replace(candidate, []byte("encoded := out.Bytes()"), []byte("encoded := []byte(\"wrong\")"), 1)
	}
	if bytes.Equal(candidate, source) {
		t.Fatal("fixture mutation anchor missing")
	}
	write(t, root, "internal/wp3codec/codec.go", candidate)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "candidate encoding refactor")
	target := git(t, root, "rev-parse", "HEAD")
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = session.Prepare(context.Background(), workflow.PrepareOptions{Base: base, Target: target, Spec: wire.StableSpec}); err != nil {
		testplatform.SkipWindowsRefusal(t, err)
		t.Fatal(err)
	}
	session, err = workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = session.Cite(context.Background(), workflow.CiteOptions{MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "docs/intent.md", Lines: "5:5", Relation: "specification"}); err != nil {
		t.Fatal(err)
	}
	cem, err := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", wire.ExcludedCEMPath)
	git(t, root, "commit", "-qm", "bind fixture CEM")
	target = git(t, root, "rev-parse", "HEAD")
	parsed, err := wire.ParseStable(cem)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	diff, err := repo.CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(driver, []byte(claimAnchor))
	fields := map[string]any{"extractor": "corvint-test-claim/1", "path": "tools/local-authority/driver.go", "blobOid": git(t, root, "rev-parse", target+":tools/local-authority/driver.go"), "selector": "test:TestCanonicalOutput/case:ple-v0-canonical-vector", "span": map[string]any{"start": start, "end": start + len(claimAnchor)}, "spanSha256": localauthority.BytesDigest([]byte(claimAnchor))}
	claimRaw, _ := json.Marshal(fields)
	fields["id"] = "claim:sha256:" + localauthority.BytesDigest(claimRaw)
	istart := strings.Index(intent, "## Requirements")
	iend := strings.Index(intent, "## Next")
	old := map[string]any{"spec": "ocm/0.1-experimental", "targetRevision": target, "intentScope": map[string]any{"path": "docs/intent.md", "blobOid": git(t, root, "rev-parse", target+":docs/intent.md"), "span": map[string]any{"start": istart, "end": iend}, "spanSha256": localauthority.BytesDigest([]byte(intent[istart:iend]))}, "cem": map[string]any{"mapSha256": localauthority.BytesDigest(cem), "patchSha256": localauthority.BytesDigest(diff)}, "claims": []any{fields}, "obligations": []any{map[string]any{"id": "PLE-V0-005", "disposition": "linked", "reason": "change-and-test-linked", "hunkIds": []string{parsed.Change.Hunks[0].ID}, "claimIds": []string{fields["id"].(string)}}}}
	ocm, _ := json.Marshal(old)
	ocm = append(ocm, '\n')
	selection, _ := localauthority.Canonical(frontiernext.Selection{Profile: frontiernext.OCMProfile, LegacyOCMSHA256: localauthority.BytesDigest(ocm), Mode: localauthority.SelectionMode, Edges: []frontiernext.Edge{{ClaimID: fields["id"].(string), ObligationID: "PLE-V0-005", CheckID: "wp3codec-canonical-v0", Subject: "internal/wp3codec/codec.go", Function: "Encode"}}})
	d := strings.Repeat("a", 64)
	checks := []localauthority.Check{{ClaimSelector: "test:TestCanonicalOutput/case:ple-v0-canonical-vector", ID: "wp3codec-canonical-v0", Profile: localauthority.CheckProfile, DriverSHA256: localauthority.BytesDigest(driver), DriverPath: "tools/local-authority/driver.go", DriverUnit: "TestCanonicalOutput", Invocation: "wp3codec-wasip1-go1.27-v0", Subject: "internal/wp3codec/codec.go"}}
	sd, _ := localauthority.Digest(checks)
	e := localauthority.Enrollment{RepositoryID: d, PolicySHA256: d, Profile: localauthority.Profile, Nonce: d, Audience: "fixture", RootID: "fixture-unadmitted", Epoch: "1", Generation: "1", IssuedAt: "1000", ExpiresAt: "2000", Selection: localauthority.SelectionMode, Checks: checks, Binding: localauthority.Binding{Base: base, Target: target, Tree: git(t, root, "rev-parse", target+"^{tree}"), CEMSHA256: localauthority.BytesDigest(cem), OCMSHA256: localauthority.BytesDigest(selection), SelectionSHA256: sd, SourceSHA256: d, RecipeSHA256: d, WasmSHA256: d, WorkerSHA256: d}}
	r := frontiernext.Request{CEM: cem, OCM: ocm, Selection: selection, Enrollment: e}
	p := localauthority.PolicySnapshot{RepositoryID: d, PolicySHA256: d, Fixture: true, Current: true, RootID: e.RootID, Epoch: "1", Generation: "1", MinimumGeneration: "1", Audience: e.Audience, Checks: checks, TerminalSHA256: map[string]string{}}
	sign(t, &r, &p, "PASS")
	return root, r, p
}
func sign(t *testing.T, r *frontiernext.Request, p *localauthority.PolicySnapshot, status string) {
	key := ed25519.NewKeyFromSeed(make([]byte, 32))
	p.PublicKey = key.Public().(ed25519.PublicKey)
	r.Receipt = localauthority.Receipt{Payload: localauthority.Payload{Enrollment: r.Enrollment, CompletedAt: "1100", Cleanup: "COMPLETE", ExitCode: "0", Rows: []localauthority.Row{{Check: r.Enrollment.Checks[0], Status: status}}}}
	raw, err := localauthority.SigningBytes(r.Receipt.Payload)
	if err != nil {
		t.Fatal(err)
	}
	r.Receipt.Signature = hex.EncodeToString(ed25519.Sign(key, raw))
	d, _ := localauthority.Digest(r.Receipt)
	p.TerminalSHA256[r.Enrollment.Nonce] = d
}

// TestCEMV1210FixtureRecomputeRefusesEveryMap (CEM-V1-210): the experimental
// recompute reads the frozen frontier/0 universe and lrf/0, which admit no map
// since cem/0.x was retired. A cem/1.0 request refuses with unsupported-spec,
// and a retired cem/0.2 map refuses with unsupported-spec and the recovery
// line; neither reaches the edge evaluation.
func TestCEMV1210FixtureRecomputeRefusesEveryMap(t *testing.T) {
	source, err := os.ReadFile("../wp3codec/codec.go")
	if err != nil {
		t.Fatal(err)
	}
	driver := []byte("package driver\nfunc TestCanonicalOutput() {\n for _, group := range []struct{name string}{{name: \"PLE-V0-005 canonical vectors\"}} { if group.name == \"\" { panic(\"missing group\") } } }\n")
	root, r, _ := fixtureRequest(t, source, driver, false)
	retired := r
	retired.CEM = bytes.Replace(r.CEM, []byte(`"spec":"cem/1.0"`), []byte(`"spec":"cem/0.2"`), 1)
	if bytes.Equal(retired.CEM, r.CEM) {
		t.Fatal("fixture map lacks the cem/1.0 spec anchor")
	}
	for name, request := range map[string]frontiernext.Request{"cem/1.0": r, "cem/0.2": retired} {
		universe, err := Adapter{root}.Recompute(context.Background(), request, frontiernext.Selection{})
		if cemcode.CodeOf(err) != cemcode.UnsupportedSpec || len(universe.Eligible) != 0 || len(universe.Open) != 0 {
			t.Fatalf("%s: universe %+v err %v", name, universe, err)
		}
		if name == "cem/0.2" && !strings.Contains(err.Error(), wire.RetiredSpecFix) {
			t.Fatalf("%s: refusal lacks the recovery line: %v", name, err)
		}
	}
}
