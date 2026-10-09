//go:build darwin || linux

package bridge

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/flowcoverage/testfixture"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// flowsRepository is makeRepository plus one committed flow intent under flows/.
func flowsRepository(t *testing.T) string {
	t.Helper()
	root := makeRepository(t)
	intent := appflows.FlowIntent{
		Schema: appflows.FlowIntentSchema, FlowID: "search", Revision: 1, Kind: "ui", Actor: "shopper", Preconditions: []string{},
		Steps:      []appflows.FlowStep{{StepID: "run-search", Action: "search"}},
		Outcomes:   []appflows.FlowOutcome{{OutcomeID: "listed", Behavior: "listed shown", Matcher: "toBeVisible", Locator: "results", Value: "listed"}},
		Variations: []appflows.FlowVariation{}, Links: []appflows.FlowLink{},
	}
	raw, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "flows", "search.json"), string(raw))
	gitOutput(t, root, "add", "flows")
	gitOutput(t, root, "commit", "-q", "-m", "flows")
	return root
}

func flowsCall(t *testing.T, registry *Registry, tool, arguments string) (Result, *Error) {
	t.Helper()
	return registry.Call(context.Background(), tool, []byte(arguments))
}

// AFU-V1-034: a probe that changes across the flows verb abstains instead of binding the receipt.
func TestAFUV1034FlowsAbstainWhenTheCheckoutMoves(t *testing.T) {
	registry, err := NewFlows(flowsRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	probes := 0
	operations := registry.operations
	observe := operations.probe
	operations.probe = func(ctx context.Context, root string) (gokernel.Repository, error) {
		probes++
		repository, err := observe(ctx, root)
		repository.DirtyPathsSHA = strings.Repeat(string(rune('a'+probes)), 64)
		return repository, err
	}
	registry = registryWithOperations(registry, operations)
	result, callErr := flowsCall(t, registry, ToolFlowsMap, `{"flows":"flows"}`)
	if callErr != nil {
		t.Fatal(callErr)
	}
	if probes != 2 || result.State != "ABSTAINED" || result.Abstention.Reason != "REPOSITORY_STATE_UNSTABLE" || result.Receipt != nil {
		t.Fatalf("moving checkout = %d probes, %#v", probes, result)
	}
}

// AFU-V1-034: an input file reached through a symlinked parent directory is refused.
func TestAFUV1034FlowsRefuseSymlinkedInputParent(t *testing.T) {
	root := flowsRepository(t)
	registry, err := NewFlows(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "logs", "runs.jsonl"), "")
	if result, callErr := flowsCall(t, registry, ToolFlowsMap, `{"flows":"flows","evidence":["logs/runs.jsonl"]}`); callErr != nil || result.State != "READY" {
		t.Fatalf("regular parent: %#v %#v", callErr, result)
	}
	if err := os.Symlink(filepath.Join(root, "logs"), filepath.Join(root, "runs")); err != nil {
		t.Fatal(err)
	}
	if _, callErr := flowsCall(t, registry, ToolFlowsMap, `{"flows":"flows","evidence":["runs/runs.jsonl"]}`); callErr == nil || callErr.Code != "flows-refused" {
		t.Fatalf("symlinked parent: %#v", callErr)
	}
}

// AFU-V1-034: the flows tools leave every repository byte, Git metadata included, unchanged.
func TestAFUV1034FlowsLeaveRepositoryBytesUnchanged(t *testing.T) {
	root := flowsRepository(t)
	registry, err := NewFlows(root)
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD~1")))
	before := rootDigest(t, root)
	for tool, arguments := range map[string]string{
		ToolFlowsMap:      `{"flows":"flows"}`,
		ToolFlowsGaps:     `{"flows":"flows"}`,
		ToolFlowsImpact:   `{"flows":"flows","base":"` + base + `"}`,
		ToolFlowsNavigate: `{"flows":"flows"}`,
	} {
		if result, callErr := flowsCall(t, registry, tool, arguments); callErr != nil || result.State != "READY" {
			t.Fatalf("%s: %#v %#v", tool, callErr, result)
		}
	}
	if after := rootDigest(t, root); after != before {
		t.Fatal("a flows tool changed repository bytes")
	}
}

// V1-0349: in a blob:none sparse clone whose blobs outside the root cone stay on the promisor
// remote, no MCP read tool reaches the remote and every flows tool refuses with a coded error,
// including through a Git that ignores GIT_NO_LAZY_FETCH, as Git before 2.46 does for a diff's
// blob prefetch (invariant 4, MCPV0-017). The remote's upload-pack touches a sentinel first, so a
// fetch attempt leaves it even though the removed source cannot serve one. It sets PATH, so it is
// not parallel.
func TestMCPReadsRefuseAMissingPromisorObjectWithoutFetching(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for _, dropsLazyFetchGuard := range []bool{false, true} {
		source := flowsRepository(t)
		base := strings.TrimSpace(string(gitOutput(t, source, "rev-parse", "HEAD")))
		gitOutput(t, source, "config", "uploadpack.allowFilter", "true")
		sentinel := filepath.Join(t.TempDir(), "fetch-attempted")
		clone, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		gitOutput(t, source, "clone", "-q", "-c", "protocol.file.allow=always", "--filter=blob:none", "--sparse", "file://"+source, clone)
		gitOutput(t, clone, "config", "remote.origin.uploadpack", "touch '"+sentinel+"' && git-upload-pack")
		if err := os.RemoveAll(source); err != nil {
			t.Fatal(err)
		}
		if dropsLazyFetchGuard {
			shim := t.TempDir()
			writeFile(t, filepath.Join(shim, "git"), "#!/bin/sh\nunset GIT_NO_LAZY_FETCH\nexec '"+realGit+"' \"$@\"\n")
			if err := os.Chmod(filepath.Join(shim, "git"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
		registry, callErr := NewFlows(clone)
		if callErr != nil {
			t.Fatal(callErr)
		}
		review, callErr := NewTaskReview(clone)
		if callErr != nil {
			t.Fatal(callErr)
		}
		task := `{"task":"orient contributor roadmap ticket workflow"}`
		for _, call := range []struct {
			registry        *Registry
			tool, arguments string
		}{
			{registry, ToolQuery, task}, {registry, ToolImpact, `{"paths":["internal/widget/widget.go"]}`},
			{registry, ToolStatus, `{}`}, {review, ToolContext, task}, {review, ToolCEMReport, cemArguments(t, clone)},
			{registry, ToolFlowsMap, `{"flows":"flows"}`}, {registry, ToolFlowsGaps, `{"flows":"flows"}`},
			{registry, ToolFlowsImpact, `{"flows":"flows","base":"` + base + `"}`}, {registry, ToolFlowsNavigate, `{"flows":"flows"}`},
		} {
			_, callErr := call.registry.Call(context.Background(), call.tool, []byte(call.arguments))
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatalf("drops guard %v: %s reached the promisor remote (sentinel stat: %v)", dropsLazyFetchGuard, call.tool, err)
			}
			if strings.HasPrefix(call.tool, "corvint.flows.") && (callErr == nil || callErr.Code == "") {
				t.Fatalf("drops guard %v: %s served flows whose intent blob is missing: %#v", dropsLazyFetchGuard, call.tool, callErr)
			}
		}
	}
}

// V1-0349: corvint.flows.coverage, which reads the denominator's committed intents, refuses with a
// coded error and never reaches the promisor remote when a blob:none sparse clone left the intent
// blob there, including through a Git that ignores GIT_NO_LAZY_FETCH. A full blob:none clone of the
// same source is the control: it serves the page, so the sparse refusal is the missing blob's. The
// remote's upload-pack touches a sentinel first, so a fetch attempt leaves it even though the
// removed source cannot serve one. It sets PATH, so it is not parallel.
func TestMCPFlowsCoverageRefusesAMissingPromisorObjectWithoutFetching(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	arguments := []byte(`{"denominator":"denominator.json","receipts":"runs.json","offset":0,"limit":1}`)
	for _, dropsLazyFetchGuard := range []bool{false, true} {
		source := testfixture.Repository(t)
		gitOutput(t, source, "config", "uploadpack.allowFilter", "true")
		sentinel := filepath.Join(t.TempDir(), "fetch-attempted")
		clone := func(flags ...string) string {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			gitOutput(t, source, append(append([]string{"clone", "-q", "-c", "protocol.file.allow=always", "--filter=blob:none"}, flags...), "file://"+source, root)...)
			gitOutput(t, root, "config", "remote.origin.uploadpack", "touch '"+sentinel+"' && git-upload-pack")
			return root
		}
		sparse, full := clone("--sparse"), clone()
		if err := os.RemoveAll(source); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(sparse, "flows")); !os.IsNotExist(err) {
			t.Fatalf("the sparse clone checked out flows/, so its intent blob is local (stat: %v)", err)
		}
		if dropsLazyFetchGuard {
			shim := t.TempDir()
			writeFile(t, filepath.Join(shim, "git"), "#!/bin/sh\nunset GIT_NO_LAZY_FETCH\nexec '"+realGit+"' \"$@\"\n")
			if err := os.Chmod(filepath.Join(shim, "git"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
		for _, test := range []struct {
			name, root string
			refused    bool
		}{{"full", full, false}, {"sparse", sparse, true}} {
			registry, callErr := NewFlows(test.root)
			if callErr != nil {
				t.Fatal(callErr)
			}
			result, callErr := registry.Call(context.Background(), ToolFlowsCoverage, arguments)
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatalf("drops guard %v: coverage over the %s clone reached the promisor remote (sentinel stat: %v)", dropsLazyFetchGuard, test.name, err)
			}
			if test.refused && (callErr == nil || callErr.Code != "flows-refused") {
				t.Fatalf("drops guard %v: coverage served a denominator whose intent blob is missing: %#v", dropsLazyFetchGuard, callErr)
			}
			if !test.refused && (callErr != nil || result.State != "READY") {
				t.Fatalf("drops guard %v: control coverage over a clone holding every HEAD blob did not serve READY: %#v %#v", dropsLazyFetchGuard, callErr, result)
			}
		}
	}
}

// V1-1039: corvint.cem.report over a full blob:none clone whose CEM map is present in the
// worktree, with every HEAD blob local and only the cited base blob left on the promisor remote,
// gets past the map read to the Git diff and refuses with repository-unavailable, the MCPV0-025
// code for a Git read failure (cemcode repository-object-unavailable), without reaching the
// remote, also through a Git that ignores GIT_NO_LAZY_FETCH (invariant 4, MCPV0-017). A clone
// without the filter serves READY from the same map, so the refusal is the missing blob's. The
// remote's upload-pack touches a sentinel first, so a fetch attempt leaves it even though the
// removed source cannot serve one; a final lazy cat-file with the file transport allowed shows the
// sentinel records one. It sets PATH, so it is not parallel.
func TestMCPCEMReportRefusesAMissingCitedBlobWithAValidMapWithoutFetching(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	source, base, target := cemRepository(t)
	mapBytes, err := os.ReadFile(filepath.Join(source, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	baseBlob := strings.TrimSpace(string(gitOutput(t, source, "rev-parse", base+":internal/widget/widget.go")))
	gitOutput(t, source, "config", "uploadpack.allowFilter", "true")
	sentinel := filepath.Join(t.TempDir(), "fetch-attempted")
	clone := func(flags ...string) string {
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		gitOutput(t, source, append(append([]string{"clone", "-q", "-c", "protocol.file.allow=always"}, flags...), "file://"+source, root)...)
		gitOutput(t, root, "config", "remote.origin.uploadpack", "touch '"+sentinel+"' && git-upload-pack")
		writeFile(t, filepath.Join(root, ".corvint", "change.cem.json"), string(mapBytes))
		return root
	}
	partial, complete := clone("--filter=blob:none"), clone()
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	present := func(root, object string) bool {
		command := exec.Command("git", "-C", root, "cat-file", "-e", object)
		command.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=")
		return command.Run() == nil
	}
	if present(partial, baseBlob) || !present(complete, baseBlob) || !present(partial, target+":internal/widget/widget.go") {
		t.Fatal("fixture: the partial clone must lack only the base blob and the complete clone must hold it")
	}
	arguments := []byte(`{"map":".corvint/change.cem.json","expectedBase":"` + base + `","target":"` + target + `"}`)
	for _, dropsLazyFetchGuard := range []bool{false, true} {
		if dropsLazyFetchGuard {
			shim := t.TempDir()
			writeFile(t, filepath.Join(shim, "git"), "#!/bin/sh\nunset GIT_NO_LAZY_FETCH\nexec '"+realGit+"' \"$@\"\n")
			if err := os.Chmod(filepath.Join(shim, "git"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
		for _, test := range []struct {
			name, root string
			refused    bool
		}{{"complete", complete, false}, {"partial", partial, true}} {
			registry, err := NewTaskReview(test.root)
			if err != nil {
				t.Fatal(err)
			}
			// Record the CEM code under the sanitized bridge code, so the refusal is shown to be
			// the missing object and not a map or other Git failure.
			cemCode := ""
			operations := registry.operations
			report := operations.cemReport
			operations.cemReport = func(ctx context.Context, root string, options workflow.ReadOptions) (map[string]any, error) {
				receipt, err := report(ctx, root, options)
				cemCode = cemcode.CodeOf(err)
				return receipt, err
			}
			registry = registryWithOperations(registry, operations)
			before := rootDigest(t, test.root)
			result, callErr := registry.Call(context.Background(), ToolCEMReport, arguments)
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatalf("drops guard %v: cem.report over the %s clone reached the promisor remote (sentinel stat: %v)", dropsLazyFetchGuard, test.name, err)
			}
			if after := rootDigest(t, test.root); after != before {
				t.Fatalf("drops guard %v: cem.report over the %s clone changed repository bytes", dropsLazyFetchGuard, test.name)
			}
			if test.refused && (callErr == nil || callErr.Code != "repository-unavailable" || cemCode != cemcode.RepositoryObjectUnavailable) {
				t.Fatalf("drops guard %v: cem.report with a missing cited blob = %#v (CEM code %q) %#v, want repository-unavailable from %s", dropsLazyFetchGuard, callErr, cemCode, result, cemcode.RepositoryObjectUnavailable)
			}
			if !test.refused && (callErr != nil || result.State != "READY" || result.Receipt["tool"] != "cem-report") {
				t.Fatalf("drops guard %v: control cem.report over the complete clone = %#v %#v, want READY", dropsLazyFetchGuard, callErr, result)
			}
		}
	}
	lazy := exec.Command("git", "-C", partial, "cat-file", "-p", baseBlob)
	lazy.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=0")
	_ = lazy.Run()
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("a lazy fetch with the file transport allowed left no sentinel, so the sentinel proves nothing: %v", err)
	}
}
