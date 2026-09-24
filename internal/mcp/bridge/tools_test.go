package bridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// TestContextToolReturnsBoundPacketWithoutWrites covers MCPV0-024: the packet
// is `corvint context`'s, bound to the tree it was compiled from, and nothing
// under the root (including .git) changes.
func TestContextToolReturnsBoundPacketWithoutWrites(t *testing.T) {
	if runtimeUnsupported() {
		t.Skip("native context is qualified only on Darwin and Linux")
	}
	root := makeRepository(t)
	registry, err := NewTaskReview(root)
	if err != nil {
		t.Fatal(err)
	}
	before := rootDigest(t, root)
	result, callErr := registry.Call(context.Background(), ToolContext,
		[]byte(`{"task":"change widget Value","subject":"internal/widget/widget.go","limit":5}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	assertObservedBinding(t, result)
	if result.Receipt["tool"] != "context" || result.Receipt["mutates"] != false ||
		result.Receipt["revision"] != result.Repository.TreeRevision {
		t.Fatalf("context receipt is not the bound packet: %#v", result.Receipt)
	}
	assertCanonicalObject(t, result)
	untracked, callErr := registry.Call(context.Background(), ToolContext,
		[]byte(`{"task":"change widget Value","subject":"internal/widget/missing.go"}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	if untracked.State != "ABSTAINED" || untracked.Abstention.Reason != "NOT_TRACKED_AT_REVISION" || untracked.Receipt != nil {
		t.Fatalf("untracked subject = %#v", untracked)
	}
	if after := rootDigest(t, root); after != before {
		t.Fatal("corvint.context wrote under the repository root")
	}
}

// TestCEMReportToolPreviewsCLIReportWithoutPublishing covers MCPV0-025: the
// preview's markdown is byte-identical to the report `corvint cem report`
// publishes, and the preview itself writes nothing.
func TestCEMReportToolPreviewsCLIReportWithoutPublishing(t *testing.T) {
	if runtimeUnsupported() {
		t.Skip("CEM Git reads are qualified only on Darwin and Linux")
	}
	root, base, target := cemRepository(t)
	registry, err := NewTaskReview(root)
	if err != nil {
		t.Fatal(err)
	}
	arguments := `{"map":".corvint/change.cem.json","expectedBase":"` + base + `","target":"` + target + `","maxUnknown":0}`
	before := rootDigest(t, root)
	result, callErr := registry.Call(context.Background(), ToolCEMReport, []byte(arguments))
	if callErr != nil {
		t.Fatal(callErr)
	}
	if after := rootDigest(t, root); after != before {
		t.Fatal("corvint.cem.report wrote under the repository root")
	}
	assertObservedBinding(t, result)
	assertCanonicalObject(t, result)
	markdown, _ := result.Receipt["markdown"].(string)
	if result.Receipt["tool"] != "cem-report" || result.Receipt["mutates"] != false || markdown == "" {
		t.Fatalf("cem report receipt = %#v", result.Receipt)
	}
	session, openErr := workflow.Open(root)
	if openErr != nil {
		t.Fatal(openErr)
	}
	limit := 0
	published, readErr := session.Read(context.Background(), "report", workflow.ReadOptions{
		MapPath: ".corvint/change.cem.json", ExpectedBase: base, Target: target,
		Limits: workflow.PolicyLimits{MaxUnknown: &limit},
	})
	if readErr != nil {
		t.Fatal(readErr)
	}
	written, fileErr := os.ReadFile(published["report"].(string))
	if fileErr != nil {
		t.Fatal(fileErr)
	}
	if string(written) != markdown {
		t.Fatal("preview markdown differs from the published CLI report")
	}
	for _, field := range []string{"ok", "counts", "policyIssues", "verification", "patchSource", "excludedPath", "warnings"} {
		if !reflect.DeepEqual(canonicalValue(t, published[field]), canonicalValue(t, result.Receipt[field])) {
			t.Fatalf("field %s differs: cli=%#v mcp=%#v", field, published[field], result.Receipt[field])
		}
	}
}

// TestCEMReportToolRefusesMapsOutsideTheRoot: a symlinked map or ancestor is
// a CEM read refusal, and a 0.1 map (whose patch is out of band) is refused.
func TestCEMReportToolRefusesMapsOutsideTheRoot(t *testing.T) {
	if runtimeUnsupported() {
		t.Skip("CEM Git reads are qualified only on Darwin and Linux")
	}
	root, base, target := cemRepository(t)
	outside := t.TempDir()
	mapBytes, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(outside, "map.cem.json"), string(mapBytes))
	if err := os.Symlink(filepath.Join(outside, "map.cem.json"), filepath.Join(root, "linked.cem.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "legacy.cem.json"), `{"spec":"cem/0.1","baseRevision":"`+base+
		`","patchSha256":"`+strings.Repeat("0", 64)+`","evidence":[],"hunks":[]}`)
	registry, newErr := NewTaskReview(root)
	if newErr != nil {
		t.Fatal(newErr)
	}
	for mapPath, want := range map[string]string{
		"linked.cem.json": "cem-map-unavailable", "linked/map.cem.json": "cem-map-unavailable",
		"missing.cem.json": "cem-map-unavailable", "legacy.cem.json": "cem-map-unsupported",
	} {
		arguments := `{"map":"` + mapPath + `","expectedBase":"` + base + `","target":"` + target + `"}`
		if result, callErr := registry.Call(context.Background(), ToolCEMReport, []byte(arguments)); callErr == nil || callErr.Code != want {
			t.Fatalf("map %s: result=%#v error=%#v, want %s", mapPath, result, callErr, want)
		}
	}
}

// TestNewToolsAbstainOverTheBridgeBudget: both tools inherit the MCPV0-010
// budget; an oversized receipt is withheld, never truncated.
func TestNewToolsAbstainOverTheBridgeBudget(t *testing.T) {
	registry, err := NewTaskReview(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	revision, commit := strings.Repeat("b", 40), strings.Repeat("a", 40)
	oversized := strings.Repeat("x", maxResultBytes)
	operations := registry.operations
	operations.platform = "linux"
	operations.context = func(context.Context, string, contextInput) (*contextindex.Index, map[string]any, error) {
		index := &contextindex.Index{ObjectFormat: "sha1", CommitRevision: commit, Revision: revision, ProfileID: "generic"}
		return index, map[string]any{"tool": "context", "mutates": false, "revision": revision, "payload": oversized}, nil
	}
	operations.probe = func(context.Context, string) (gokernel.Repository, error) {
		return gokernel.Repository{
			CommitRevision: commit, TreeRevision: revision, ObjectFormat: "sha1", ProfileID: "generic",
			WorktreeState: "clean", DirtyPathsSHA: strings.Repeat("c", 64),
		}, nil
	}
	operations.cemReport = func(context.Context, string, workflow.ReadOptions) (map[string]any, error) {
		return map[string]any{"tool": "cem-report", "mutates": false, "markdown": oversized}, nil
	}
	registry = registryWithOperations(registry, operations)
	for tool, arguments := range map[string]string{
		ToolContext:   `{"task":"change widget Value"}`,
		ToolCEMReport: `{"map":"m.cem.json","expectedBase":"` + commit + `","target":"` + commit + `"}`,
	} {
		result, callErr := registry.Call(context.Background(), tool, []byte(arguments))
		if callErr != nil {
			t.Fatal(callErr)
		}
		if result.State != "ABSTAINED" || result.Abstention.Reason != "OUTPUT_BUDGET_EXCEEDED" || result.Receipt != nil {
			t.Fatalf("%s oversized result = %#v", tool, result)
		}
	}
}

// TestCEMReportToolAbstainsWhenTheCheckoutMoves: the binding brackets the
// CEM read; a probe that disagrees across it abstains instead of binding.
func TestCEMReportToolAbstainsWhenTheCheckoutMoves(t *testing.T) {
	registry, err := NewTaskReview(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	probes := 0
	operations := registry.operations
	operations.probe = func(context.Context, string) (gokernel.Repository, error) {
		probes++
		return gokernel.Repository{
			CommitRevision: strings.Repeat("a", 40), TreeRevision: strings.Repeat(string(rune('a'+probes)), 40),
			ObjectFormat: "sha1", ProfileID: "generic", WorktreeState: "clean", DirtyPathsSHA: strings.Repeat("c", 64),
		}, nil
	}
	operations.cemReport = func(context.Context, string, workflow.ReadOptions) (map[string]any, error) {
		return map[string]any{"tool": "cem-report", "mutates": false, "markdown": "report"}, nil
	}
	registry = registryWithOperations(registry, operations)
	oid := strings.Repeat("a", 40)
	result, callErr := registry.Call(context.Background(), ToolCEMReport,
		[]byte(`{"map":"m.cem.json","expectedBase":"`+oid+`","target":"`+oid+`"}`))
	if callErr != nil || result.State != "ABSTAINED" || result.Abstention.Reason != "REPOSITORY_STATE_UNSTABLE" {
		t.Fatalf("moved checkout result=%#v error=%#v", result, callErr)
	}
	operations.cemReport = func(context.Context, string, workflow.ReadOptions) (map[string]any, error) {
		return nil, errors.New("unregistered")
	}
	registry = registryWithOperations(registry, operations)
	if _, callErr := registry.Call(context.Background(), ToolCEMReport,
		[]byte(`{"map":"m.cem.json","expectedBase":"`+oid+`","target":"`+oid+`"}`)); callErr == nil || callErr.Code != "internal-error" {
		t.Fatalf("unregistered CEM failure = %#v", callErr)
	}
}

// cemRepository commits one change on the fixture and prepares its canonical
// map at the fixed path, returning the root and both full object IDs.
func cemRepository(t *testing.T) (string, string, string) {
	t.Helper()
	root := makeRepository(t)
	base := strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD")))
	writeFile(t, filepath.Join(root, "internal/widget/widget.go"), "package widget\n\nfunc Value() int { return 2 }\n")
	gitOutput(t, root, "commit", "-q", "-am", "change")
	target := strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD")))
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Prepare(context.Background(), workflow.PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	return root, base, target
}

// rootDigest hashes every path, mode, and regular-file content under root,
// .git included.
func rootDigest(t *testing.T, root string) string {
	t.Helper()
	digest := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		digest.Write([]byte(relative + "\x00" + info.Mode().String() + "\x00"))
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			digest.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func canonicalValue(t *testing.T, value any) string {
	t.Helper()
	encoded, err := gokernel.CanonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
