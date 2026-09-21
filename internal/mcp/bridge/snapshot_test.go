package bridge

import (
	"context"
	"encoding/json"
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
				if _, err := dirty.CanonicalJSON(); err != nil {
					t.Fatal(err)
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
