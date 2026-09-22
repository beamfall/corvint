package bridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/plansnapshot"
)

func TestMCPExplicitSnapshotRetainsImmutableEvidenceInMixedWorktree(t *testing.T) {
	t.Run("MCPV0-020 TestMCPExplicitSnapshotRetainsImmutableEvidenceInMixedWorktree", func(t *testing.T) {
		root := makeRepository(t)
		revision := strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD")))
		snapshot := plansnapshot.Receipt{Schema: plansnapshot.Schema, Base: revision, Commit: revision, Tree: strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD^{tree}"))), Paths: []string{}, Digest: plansnapshot.PathDigest([]string{})}
		registry, err := New(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, tool := range []string{ToolQuery, ToolImpact} {
			t.Run(tool, func(t *testing.T) {
				args := map[string]any{"snapshot": snapshot}
				if tool == ToolQuery {
					args["task"] = "orient contributor roadmap ticket workflow"
				} else {
					args["paths"] = []string{"internal/widget/widget.go"}
				}
				raw, _ := json.Marshal(args)
				clean, err := registry.Call(context.Background(), tool, raw)
				if err != nil || clean.State != "READY" {
					t.Fatalf("%+v %v", clean, err)
				}
				writeFile(t, filepath.Join(root, "AGENTS.md"), "dirty authority must not be used\n")
				writeFile(t, filepath.Join(root, "internal/widget/widget.go"), "package invalid\n")
				writeFile(t, filepath.Join(root, "untracked.go"), "package invalid\n")
				dirty, err := registry.Call(context.Background(), tool, raw)
				if err != nil || dirty.State != "READY" {
					t.Fatalf("%+v %v", dirty, err)
				}
				if dirty.Repository.WorktreeState != "MIXED" || !reflect.DeepEqual(clean.Receipt, dirty.Receipt) {
					t.Fatalf("snapshot evidence changed: clean=%+v dirty=%+v", clean, dirty)
				}
				if dirty.Receipt["snapshot"].(map[string]any)["accepting"] != false {
					t.Fatal("accepting evidence")
				}
				wire, err := dirty.CanonicalJSON()
				if err != nil {
					t.Fatal(err)
				}
				if tool == ToolQuery {
					var decoded map[string]any
					if err := json.Unmarshal(wire, &decoded); err != nil {
						t.Fatal(err)
					}
					coverage := decoded["receipt"].(map[string]any)["coverage"].(map[string]any)
					uncertainty := coverage["uncertainty"].([]any)
					found := false
					for _, item := range uncertainty {
						if item == "history and local traces are outside the immutable planning snapshot" {
							found = true
						}
					}
					if !found || coverage["authoritative_results"].(float64) < 1 {
						t.Fatalf("lost authoritative evidence or uncertainty: %v", coverage)
					}
				}
				args["snapshot"] = nil
				raw, _ = json.Marshal(args)
				if _, err := registry.Call(context.Background(), tool, raw); err == nil {
					t.Fatal("null admitted")
				}
				bad := snapshot
				bad.Tree = bad.Commit
				args["snapshot"] = bad
				raw, _ = json.Marshal(args)
				if _, err := registry.Call(context.Background(), tool, raw); err == nil {
					t.Fatal("mismatch admitted")
				}
			})
		}
	})
}

func TestMCPSnapshotRejectsSymlinkAndGitlinkTrees(t *testing.T) {
	t.Run("MCPV0-020 unsupported immutable tree shapes fail closed", func(t *testing.T) {
		for _, kind := range []string{"symlink", "gitlink"} {
			t.Run(kind, func(t *testing.T) {
				root := makeRepository(t)
				base := strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD")))
				if kind == "symlink" {
					if err := os.Symlink("AGENTS.md", filepath.Join(root, "link")); err != nil {
						t.Fatal(err)
					}
					gitOutput(t, root, "add", "link")
				} else {
					gitOutput(t, root, "update-index", "--add", "--cacheinfo", "160000,"+base+",link")
				}
				gitOutput(t, root, "commit", "-qm", "unsupported tree")
				paths := []string{"link"}
				snapshot := plansnapshot.Receipt{Schema: plansnapshot.Schema, Base: base, Commit: strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD"))), Tree: strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD^{tree}"))), Paths: paths, Digest: plansnapshot.PathDigest(paths)}
				registry, err := New(root)
				if err != nil {
					t.Fatal(err)
				}
				for _, tool := range []string{ToolQuery, ToolImpact} {
					args := map[string]any{"snapshot": snapshot}
					if tool == ToolQuery {
						args["task"] = "orient contributor roadmap ticket workflow"
					} else {
						args["paths"] = []string{"internal/widget/widget.go"}
					}
					raw, _ := json.Marshal(args)
					result, err := registry.Call(context.Background(), tool, raw)
					if err == nil || err.Code != "repository-unavailable" || result.State == "READY" {
						t.Fatalf("%s admitted %s: %+v %v", tool, kind, result, err)
					}
				}
			})
		}
	})
}
