package bridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// stablePacketMap is the public sealed positive cem/1.0 map.
const stablePacketMap = "../../../protocol/cem-1.0/stable/repository-envelope-packet/inherited/fixtures/maps/sha1-positive-sealed.json"

// MCPV0-036: corvint.cem.report advertises one optional repository-relative
// `artifacts` directory under the same closed path rule as `map`.
func TestCEMReportToolAdvertisesArtifacts(t *testing.T) {
	registry, err := NewTaskReview(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range registry.Tools() {
		if tool.Name != ToolCEMReport {
			continue
		}
		properties, _ := tool.InputSchema["properties"].(map[string]any)
		artifacts, _ := properties["artifacts"].(map[string]any)
		if artifacts == nil || !reflect.DeepEqual(artifacts, properties["map"]) {
			t.Fatalf("artifacts schema = %v, want the map path schema %v", artifacts, properties["map"])
		}
		if required := tool.InputSchema["required"]; !reflect.DeepEqual(required, []any{"map", "expectedBase", "target"}) {
			t.Fatalf("required = %v", required)
		}
		return
	}
	t.Fatal("task-review does not advertise corvint.cem.report")
}

// MCPV0-036: the artifacts directory resolves under the bound root, and an
// absent one stays absent; an unsafe one is invalid-arguments.
func TestCEMReportToolResolvesArtifactsUnderTheRoot(t *testing.T) {
	registry, err := NewTaskReview(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	oid := strings.Repeat("a", 40)
	var seen []workflow.ReadOptions
	operations := registry.operations
	operations.cemReport = func(_ context.Context, _, _ string, options workflow.ReadOptions) (map[string]any, error) {
		seen = append(seen, options)
		return map[string]any{"tool": "cem-report", "mutates": false, "markdown": "report"}, nil
	}
	registry = registryWithOperations(registry, operations)
	head := `{"map":"m.cem.json","expectedBase":"` + oid + `","target":"` + oid + `"`
	for _, arguments := range []string{head + `,"artifacts":"build/artifacts"}`, head + `}`} {
		if _, callErr := registry.Call(context.Background(), ToolCEMReport, []byte(arguments)); callErr != nil {
			t.Fatalf("%s: %v", arguments, callErr)
		}
	}
	want := filepath.Join(registry.root, "build", "artifacts")
	if len(seen) != 2 || seen[0].ArtifactRoot != want || !seen[0].ArtifactsGiven || seen[1].ArtifactsGiven || seen[1].ArtifactRoot != "" {
		t.Fatalf("read options = %+v, want artifact root %s then none", seen, want)
	}
	for _, artifacts := range []string{"", "/abs", "../outside", "a/../b", ".git/objects", "a/.GIT", `a\b`, "a//b", "a/./b"} {
		encoded, _ := json.Marshal(artifacts)
		if _, callErr := registry.Call(context.Background(), ToolCEMReport, []byte(head+`,"artifacts":`+string(encoded)+`}`)); callErr == nil || callErr.Code != "invalid-arguments" {
			t.Fatalf("artifacts %q = %v, want invalid-arguments", artifacts, callErr)
		}
	}
	if len(seen) != 2 {
		t.Fatal("an invalid artifacts directory reached the CEM read")
	}
}

// MCPV0-037: a cem/1.0 refusal before the verifier is its own bridge code; a
// retired 0.x map's unsupported-spec refusal is cem-map-invalid (CEM-V1-210)
// and any other invalid-arguments refusal stays cem-map-unsupported.
func TestCEMReportToolNamesStableRefusals(t *testing.T) {
	err := workflowStableRefusal(t)
	for _, c := range []struct {
		err  error
		want string
	}{
		{err, "cem-map-stable-refused"},
		{cemcode.New(cemcode.UnsupportedSpec, "cem/0.2 is retired; fix: "+wire.RetiredSpecFix), "cem-map-invalid"},
		{cemcode.New(cemcode.InvalidArguments, "invalid arguments"), "cem-map-unsupported"},
	} {
		if got := cemFailure(context.Background(), c.err); got == nil || got.Code != c.want {
			t.Fatalf("%v maps to %v, want %s", c.err, got, c.want)
		}
	}
}

// workflowStableRefusal produces one real pre-verifier stable refusal: a
// policy ceiling on a cem/1.0 map.
func workflowStableRefusal(t *testing.T) error {
	t.Helper()
	root := t.TempDir()
	data, err := os.ReadFile(stablePacketMap)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "change.cem.json"), string(data))
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	limit := 0
	oid := strings.Repeat("a", 40)
	envelope, err := session.Read(context.Background(), workflow.ActionReportPreview, workflow.ReadOptions{
		MapPath: "change.cem.json", ExpectedBase: oid, Target: oid, Limits: workflow.PolicyLimits{MaxUnknown: &limit},
	})
	if err == nil {
		t.Fatalf("a stable policy ceiling was read: %v", envelope)
	}
	return err
}

// MCPV0-036, MCPV0-037: a stable map's preview is a READY receipt whose
// stableVerification is the verifier's complete result for the same inputs,
// with no 0.x verification; a policy ceiling is cem-map-stable-refused.
func TestCEMReportToolPreviewsStableMap(t *testing.T) {
	if runtimeUnsupported() {
		t.Skip("CEM Git reads are qualified only on Darwin and Linux")
	}
	root := makeRepository(t)
	data, err := os.ReadFile(stablePacketMap)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "change.cem.json"), string(data))
	if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD")))
	registry, newErr := NewTaskReview(root)
	if newErr != nil {
		t.Fatal(newErr)
	}
	before := rootDigest(t, root)
	head := `{"map":"change.cem.json","expectedBase":"` + base + `","target":"` + base + `"`
	for _, artifacts := range []string{"artifacts", ""} {
		arguments := head + `}`
		options := verify.StableOptions{Repository: root, ExpectedBase: base, Target: base}
		if artifacts != "" {
			arguments = head + `,"artifacts":"` + artifacts + `"}`
			options.ArtifactRoot = filepath.Join(root, artifacts)
		}
		result, callErr := registry.Call(context.Background(), ToolCEMReport, []byte(arguments))
		if callErr != nil {
			t.Fatalf("%s: %v", arguments, callErr)
		}
		assertObservedBinding(t, result)
		assertCanonicalObject(t, result)
		want, exit := verify.Stable(context.Background(), data, options)
		encoded, _ := json.Marshal(want)
		var wantValue, gotValue any
		if err := json.Unmarshal(encoded, &wantValue); err != nil {
			t.Fatal(err)
		}
		encoded, _ = json.Marshal(result.Receipt["stableVerification"])
		if err := json.Unmarshal(encoded, &gotValue); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotValue, wantValue) {
			t.Fatalf("receipt stableVerification differs from the verifier:\n got %v\nwant %v", gotValue, wantValue)
		}
		markdown, _ := result.Receipt["markdown"].(string)
		if result.Receipt["tool"] != "cem-report" || result.Receipt["mutates"] != false || result.Receipt["ok"] != (exit == 0) ||
			!strings.Contains(markdown, "- Spec: `cem/1.0`\n") || result.Receipt["recordSetSha256"] == nil {
			t.Fatalf("stable receipt = %#v", result.Receipt)
		}
		for _, key := range []string{"verification", "counts", "policyIssues", "worklist"} {
			if _, present := result.Receipt[key]; present {
				t.Fatalf("stable receipt synthesized the 0.x field %q", key)
			}
		}
	}
	if result, callErr := registry.Call(context.Background(), ToolCEMReport, []byte(head+`,"maxUnknown":0}`)); callErr == nil || callErr.Code != "cem-map-stable-refused" {
		t.Fatalf("stable ceiling = %#v %#v, want cem-map-stable-refused", result, callErr)
	}
	writeFile(t, filepath.Join(root, "other.cem.json"), `{"spec":"cem/9"}`)
	if _, callErr := registry.Call(context.Background(), ToolCEMReport, []byte(`{"map":"other.cem.json","expectedBase":"`+base+`","target":"`+base+`"}`)); callErr == nil || callErr.Code != "cem-map-invalid" {
		t.Fatalf("cem/9 = %#v, want cem-map-invalid", callErr)
	}
	if err := os.Remove(filepath.Join(root, "other.cem.json")); err != nil {
		t.Fatal(err)
	}
	if after := rootDigest(t, root); after != before {
		t.Fatal("corvint.cem.report wrote under the repository root")
	}
}

// MCPV0-036, CEM-V1-017: the server resolves --root for its 0.x reads, but a
// cem/1.0 report is admitted against the root exactly as the caller named it,
// byte for byte: each accepted spelling (the real root and a symlinked alias)
// reproduces `cem verify-stable --repository <same string>`, and the server
// refuses every unclean or relative spelling at start.
func TestCEMReportToolAdmitsTheCallerNamedRoot(t *testing.T) {
	if runtimeUnsupported() {
		t.Skip("CEM Git reads are qualified only on Darwin and Linux")
	}
	root := makeRepository(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(alias); err != nil || resolved != root {
		t.Fatalf("alias resolves to %q, %v", resolved, err)
	}
	data, err := os.ReadFile(stablePacketMap)
	if err != nil {
		t.Fatal(err)
	}
	// Drop the structural mechanical hunks so every Go release reaches
	// repository admission instead of the structural runtime refusal.
	document, err := wire.ParseStable(data)
	if err != nil {
		t.Fatal(err)
	}
	kept := document.Change.Hunks[:0]
	for _, hunk := range document.Change.Hunks {
		if hunk.Disposition != "mechanical" || !wire.StructuralReasons[hunk.Reason] {
			kept = append(kept, hunk)
		}
	}
	document.Change.Hunks = kept
	if data, err = wire.EncodeStable(document); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "change.cem.json"), string(data))
	if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD")))
	// The server judges --root byte for byte and accepts only an absolute,
	// clean root: every other S1 spelling is refused at start, so no form
	// reaches admission differently from `cem verify-stable --repository
	// <same string>`.
	for _, refused := range []string{alias + "/../" + filepath.Base(root), root + "/", "./" + filepath.Base(root), filepath.Base(root), "./alias"} {
		if registry, newErr := NewTaskReview(refused); newErr == nil || newErr.Code != "invalid-root" {
			t.Fatalf("--root %q = %v %v, want invalid-root", refused, registry, newErr)
		}
	}
	stable := func(repository string) any {
		result, _ := verify.Stable(context.Background(), data, verify.StableOptions{
			Repository: repository, ExpectedBase: base, Target: base, ArtifactRoot: filepath.Join(repository, "artifacts"),
		})
		encoded, _ := json.Marshal(result)
		var value any
		if err := json.Unmarshal(encoded, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	if reflect.DeepEqual(stable(alias), stable(root)) {
		t.Fatalf("fixture does not discriminate the alias: %v", stable(alias))
	}
	for _, accepted := range []string{root, alias} {
		registry, newErr := NewTaskReview(accepted)
		if newErr != nil {
			t.Fatal(newErr)
		}
		if registry.root != root || registry.callerRoot != accepted {
			t.Fatalf("registry root = %q caller %q, want the resolved %q and the caller's %q", registry.root, registry.callerRoot, root, accepted)
		}
		result, callErr := registry.Call(context.Background(), ToolCEMReport, []byte(`{"map":"change.cem.json","artifacts":"artifacts","expectedBase":"`+base+`","target":"`+base+`"}`))
		if callErr != nil {
			t.Fatal(callErr)
		}
		encoded, _ := json.Marshal(result.Receipt["stableVerification"])
		var got any
		if err := json.Unmarshal(encoded, &got); err != nil {
			t.Fatal(err)
		}
		if want := stable(accepted); !reflect.DeepEqual(got, want) {
			t.Fatalf("report through --root %s = %v\nwant verify-stable --repository %s = %v", accepted, got, accepted, want)
		}
	}
}

// V1-1992: a CEM Git read that exhausted its operation or output bound is a
// capacity refusal, never a missing repository (MCPV0-025, DCP-V1-158).
func TestCEMReportGitCapacityIsCapacityExceeded(t *testing.T) {
	for _, code := range []string{cemcode.GitBudgetExceeded, cemcode.GitOutputExceeded} {
		if got := cemFailure(context.Background(), cemcode.New(code, "bound")); got == nil || got.Code != "corpus-capacity-exceeded" {
			t.Fatalf("%s maps to %v, want corpus-capacity-exceeded", code, got)
		}
	}
}
