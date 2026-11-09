package bridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	registry, err := newTaskReview(root)
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

// TestNewToolsAbstainOverTheBridgeBudget: the context tool inherits the MCPV0-010
// budget; an oversized receipt is withheld, never truncated.
func TestNewToolsAbstainOverTheBridgeBudget(t *testing.T) {
	registry, err := newTaskReview(makeRepository(t))
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
	registry = registryWithOperations(registry, operations)
	for tool, arguments := range map[string]string{
		ToolContext: `{"task":"change widget Value"}`,
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
