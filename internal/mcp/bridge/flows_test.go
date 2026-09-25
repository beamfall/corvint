//go:build darwin || linux

package bridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
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
