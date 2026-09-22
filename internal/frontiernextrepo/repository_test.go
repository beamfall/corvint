package frontiernextrepo

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/frontiernext"
	"github.com/Beamfall/corvint/internal/localauthority"
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
	if _, err = session.Prepare(context.Background(), workflow.PrepareOptions{Base: base, Target: target}); err != nil {
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
	parsed, err := wire.ParseMap(cem)
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
	old := map[string]any{"spec": "ocm/0.1-experimental", "targetRevision": target, "intentScope": map[string]any{"path": "docs/intent.md", "blobOid": git(t, root, "rev-parse", target+":docs/intent.md"), "span": map[string]any{"start": istart, "end": iend}, "spanSha256": localauthority.BytesDigest([]byte(intent[istart:iend]))}, "cem": map[string]any{"mapSha256": localauthority.BytesDigest(cem), "patchSha256": localauthority.BytesDigest(diff)}, "claims": []any{fields}, "obligations": []any{map[string]any{"id": "PLE-V0-005", "disposition": "linked", "reason": "change-and-test-linked", "hunkIds": []string{parsed.Hunks[0].ID}, "claimIds": []string{fields["id"].(string)}}}}
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
func TestExactGitCWRFixtureRelation(t *testing.T) {
	source, err := os.ReadFile("../wp3codec/codec.go")
	if err != nil {
		t.Fatal(err)
	}
	root, r, p := fixtureRequest(t, source, []byte("package driver\nfunc TestCanonicalOutput() {\n for _, group := range []struct{name string}{{name: \"PLE-V0-005 canonical vectors\"}} { if group.name == \"\" { panic(\"missing group\") } } }\n"), false)
	result, err := frontiernext.ComputeFixture(context.Background(), r, p, time.Unix(1200, 0), Adapter{root})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "EMPTY" || result.Authority != "NONE" {
		t.Fatalf("fixture relation: %+v", result)
	}
	sign(t, &r, &p, "FAIL")
	result, err = frontiernext.ComputeFixture(context.Background(), r, p, time.Unix(1200, 0), Adapter{root})
	if err != nil || result.State != "OPEN" {
		t.Fatalf("failed driver: %+v %v", result, err)
	}
}

func TestUnsupportedAndAmbiguousDefinitionsStayOpen(t *testing.T) {
	t.Run("CWR-V0-006 unresolved and ambiguous definitions abstain", func(t *testing.T) {
		cases := []struct{ source, symbol string }{
			{"package p\nfunc Encode() {}\nfunc Encode() {}\n", "Encode"},
			{"package p\ntype T int\nfunc (T) Encode() {}\n", "Encode"},
			{"package p\nfunc Other() {}\n", "Encode"},
		}
		for _, c := range cases {
			if _, _, ok := functionSpan("codec.go", []byte(c.source), c.symbol); ok {
				t.Fatal("unsupported definition qualified")
			}
		}

	})
}
func TestNeighborMaterialEditCannotQualifyWhitespace(t *testing.T) {
	t.Run("CWR-V0-007 neighboring edit cannot witness whitespace", func(t *testing.T) {
		h := &patch.Hunk{NewRange: wire.Range{Start: 1, Count: 5}, Body: []patch.BodyLine{
			{Prefix: ' ', Payload: []byte("func Encode() {\n")},
			{Prefix: '-', Payload: []byte(" return value\n")},
			{Prefix: '+', Payload: []byte("  return value\n")},
			{Prefix: ' ', Payload: []byte("}\n")},
			{Prefix: '-', Payload: []byte("func Other() { return 1 }\n")},
			{Prefix: '+', Payload: []byte("func Other() { return 2 }\n")},
		}}
		if materialHunk(h, 1, 3) {
			t.Fatal("neighbor edit qualified whitespace-only function")
		}
		if !materialHunk(h, 4, 4) {
			t.Fatal("material target line refused")
		}

	})
}
func TestArbitraryNativeEdgeAndDroppedSelectionRemainOpen(t *testing.T) {
	t.Run("PLE-V0-008 unsupported edges and dropped selection remain open", func(t *testing.T) {
		source, err := os.ReadFile("../wp3codec/codec.go")
		if err != nil {
			t.Fatal(err)
		}
		root, r, p := fixtureRequest(t, source, []byte("package driver\nfunc TestCanonicalOutput() {\n for _, group := range []struct{name string}{{name: \"PLE-V0-005 canonical vectors\"}} { if group.name == \"\" { panic(\"missing group\") } } }\n"), false)
		var selection frontiernext.Selection
		if err := localauthority.Decode(r.Selection, &selection); err != nil {
			t.Fatal(err)
		}
		selection.Edges = append(selection.Edges, frontiernext.Edge{ClaimID: selection.Edges[0].ClaimID, ObligationID: "PLE-V0-005", CheckID: "arbitrary-native-junit", Subject: "internal/wp3codec/codec.go", Function: "Encode"})
		r.Selection, _ = localauthority.Canonical(selection)
		// Even a fixture-signed envelope cannot turn an unadmitted reporter into a protected check.
		r.Enrollment.Binding.OCMSHA256 = localauthority.BytesDigest(r.Selection)
		sign(t, &r, &p, "PASS")
		result, err := frontiernext.ComputeFixture(context.Background(), r, p, time.Unix(1200, 0), Adapter{root})
		if err != nil || result.State != "OPEN" {
			t.Fatalf("native edge: %+v %v", result, err)
		}
		// Removing the edge without a new independent enrollment refuses its binding.
		selection.Edges = selection.Edges[:1]
		r.Selection, _ = localauthority.Canonical(selection)
		if _, err := frontiernext.ComputeFixture(context.Background(), r, p, time.Unix(1200, 0), Adapter{root}); err == nil {
			t.Fatal("dropped edge accepted")
		}
		selection.Edges = []frontiernext.Edge{}
		r.Selection, _ = localauthority.Canonical(selection)
		if _, err := frontiernext.ComputeFixture(context.Background(), r, p, time.Unix(1200, 0), Adapter{root}); err == nil {
			t.Fatal("empty selection accepted")
		}

	})
}

func TestDistinctNativeLegacyClaimCannotDisappear(t *testing.T) {
	source, err := os.ReadFile("../wp3codec/codec.go")
	if err != nil {
		t.Fatal(err)
	}
	driver := []byte("package driver\nfunc TestCanonicalOutput() {\n for _, group := range []struct{name string}{{name: \"PLE-V0-005 canonical vectors\"}} { if group.name == \"\" { panic(\"missing group\") } } }\n")
	root, r, p := fixtureRequest(t, source, driver, false)
	start := strings.Index(claimSource, claimAnchor)
	native := map[string]any{"extractor": "corvint-test-claim/1", "path": "tests/codec_test.go", "blobOid": git(t, root, "rev-parse", r.Enrollment.Binding.Target+":tests/codec_test.go"), "selector": "test:TestCodec/case:ple-v0-canonical-vector", "span": map[string]any{"start": start, "end": start + len(claimAnchor)}, "spanSha256": localauthority.BytesDigest([]byte(claimAnchor))}
	raw, _ := json.Marshal(native)
	id := "claim:sha256:" + localauthority.BytesDigest(raw)
	native["id"] = id
	var original map[string]any
	if err := json.Unmarshal(r.OCM, &original); err != nil {
		t.Fatal(err)
	}
	original["claims"] = append(original["claims"].([]any), native)
	sort.Slice(original["claims"].([]any), func(i, j int) bool {
		return original["claims"].([]any)[i].(map[string]any)["id"].(string) < original["claims"].([]any)[j].(map[string]any)["id"].(string)
	})
	obligation := original["obligations"].([]any)[0].(map[string]any)
	ids := []string{obligation["claimIds"].([]any)[0].(string), id}
	sort.Strings(ids)
	obligation["claimIds"] = ids
	r.OCM, _ = json.Marshal(original)
	r.OCM = append(r.OCM, '\n')
	var selection frontiernext.Selection
	if err := localauthority.Decode(r.Selection, &selection); err != nil {
		t.Fatal(err)
	}
	selection.LegacyOCMSHA256 = localauthority.BytesDigest(r.OCM)
	for _, explicit := range []bool{false, true} {
		if explicit {
			selection.Edges = append(selection.Edges, frontiernext.Edge{ClaimID: id, ObligationID: "PLE-V0-005", CheckID: "wp3codec-canonical-v0", Subject: "internal/wp3codec/codec.go", Function: "Encode"})
		}
		r.Selection, _ = localauthority.Canonical(selection)
		r.Enrollment.Binding.OCMSHA256 = localauthority.BytesDigest(r.Selection)
		sign(t, &r, &p, "PASS")
		result, err := frontiernext.ComputeFixture(context.Background(), r, p, time.Unix(1200, 0), Adapter{root})
		if err != nil || result.State != "OPEN" {
			t.Fatalf("distinct native claim explicit=%v: %+v %v", explicit, result, err)
		}
	}
}

func TestUndeclaredProductionSiblingCannotHideBehindCapsule(t *testing.T) {
	source, err := os.ReadFile("../wp3codec/codec.go")
	if err != nil {
		t.Fatal(err)
	}
	driver := []byte("package driver\nfunc TestCanonicalOutput() {\n for _, group := range []struct{name string}{{name: \"PLE-V0-005 canonical vectors\"}} { if group.name == \"\" { panic(\"missing group\") } } }\n")
	root, r, _ := fixtureRequest(t, source, driver, false)
	write(t, root, "internal/wp3codec/shadow.go", []byte("package wp3codec\nfunc Encode() {}\n"))
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "undeclared production source")
	target := git(t, root, "rev-parse", "HEAD")
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	if !completePackage(context.Background(), repo, r.Enrollment.Binding.Target, "internal/wp3codec/codec.go") {
		t.Fatal("baseline package refused")
	}
	if completePackage(context.Background(), repo, target, "internal/wp3codec/codec.go") {
		t.Fatal("undeclared duplicate source omitted")
	}
}

// This is a synthetic in-process current-policy seam, not key admission. The
// policy is never published to the protected store and grants no native support.
func TestAuthenticatedFailureAndMixedRowsStayOpen(t *testing.T) {
	t.Run("PLE-V0-008 authenticated failure cannot close mixed checks", func(t *testing.T) {
		source, err := os.ReadFile("../wp3codec/codec.go")
		if err != nil {
			t.Fatal(err)
		}
		driver := []byte("package driver\nfunc TestCanonicalOutput() {\n for _, group := range []struct{name string}{{name: \"PLE-V0-005 canonical vectors\"}} { if group.name == \"\" { panic(\"missing group\") } } }\n")
		root, r, p := fixtureRequest(t, source, driver, false)
		p.Fixture = false
		p.Accepted = true
		sign(t, &r, &p, "FAIL")
		result, err := frontiernext.Compute(context.Background(), r, p, time.Unix(1200, 0), Adapter{root})
		if err != nil || result.State != "OPEN" || result.Authority != "VERIFIED" {
			t.Fatalf("authenticated failure: %+v %v", result, err)
		}
		second := r.Enrollment.Checks[0]
		second.ID = "wp3codec-z-v0"
		r.Enrollment.Checks = append(r.Enrollment.Checks, second)
		p.Checks = r.Enrollment.Checks
		r.Enrollment.Binding.SelectionSHA256, _ = localauthority.Digest(r.Enrollment.Checks)
		var selection frontiernext.Selection
		if err := localauthority.Decode(r.Selection, &selection); err != nil {
			t.Fatal(err)
		}
		secondEdge := selection.Edges[0]
		secondEdge.CheckID = second.ID
		selection.Edges = append(selection.Edges, secondEdge)
		r.Selection, _ = localauthority.Canonical(selection)
		r.Enrollment.Binding.OCMSHA256 = localauthority.BytesDigest(r.Selection)
		for _, status := range []string{"FAIL", "PASS", "SKIP"} {
			r.Receipt.Payload.Enrollment = r.Enrollment
			r.Receipt.Payload.Rows = []localauthority.Row{{Check: r.Enrollment.Checks[0], Status: "PASS"}, {Check: second, Status: status}}
			key := ed25519.NewKeyFromSeed(make([]byte, 32))
			raw, err := localauthority.SigningBytes(r.Receipt.Payload)
			if err != nil {
				t.Fatal(err)
			}
			r.Receipt.Signature = hex.EncodeToString(ed25519.Sign(key, raw))
			p.TerminalSHA256[r.Enrollment.Nonce], _ = localauthority.Digest(r.Receipt)
			result, err = frontiernext.Compute(context.Background(), r, p, time.Unix(1200, 0), Adapter{root})
			if err != nil {
				t.Fatal(err)
			}
			state, authority := "OPEN", "VERIFIED"
			if status == "PASS" {
				state = "EMPTY"
			}
			if status == "SKIP" {
				authority = "NONE"
			}
			if result.State != state || result.Authority != authority {
				t.Fatalf("mixed %s: %+v", status, result)
			}
			// A mandatory check cannot disappear merely because another edge covers
			// every legacy claim. Its enrollment floor remains an explicit obligation.
			completeSelection := r.Selection
			selection.Edges = selection.Edges[:1]
			r.Selection, _ = localauthority.Canonical(selection)
			r.Enrollment.Binding.OCMSHA256 = localauthority.BytesDigest(r.Selection)
			r.Receipt.Payload.Enrollment = r.Enrollment
			raw, err = localauthority.SigningBytes(r.Receipt.Payload)
			if err != nil {
				t.Fatal(err)
			}
			r.Receipt.Signature = hex.EncodeToString(ed25519.Sign(key, raw))
			p.TerminalSHA256[r.Enrollment.Nonce], _ = localauthority.Digest(r.Receipt)
			omitted, err := frontiernext.Compute(context.Background(), r, p, time.Unix(1200, 0), Adapter{root})
			if err != nil || omitted.State != "OPEN" || omitted.Authority != authority {
				t.Fatalf("omitted mandatory %s: %+v %v", status, omitted, err)
			}
			r.Selection = completeSelection
			if err := localauthority.Decode(r.Selection, &selection); err != nil {
				t.Fatal(err)
			}
			r.Enrollment.Binding.OCMSHA256 = localauthority.BytesDigest(r.Selection)

		}

	})
}
