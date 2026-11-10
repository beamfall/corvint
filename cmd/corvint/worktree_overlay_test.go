package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// TestContextWorktreeOverlayCLI is DIRTY-CACHE-940..942 for `corvint
// context`: rows citing the dirty tracked file are worktree-overlay evidence
// at low confidence, coverage names the overlay, and nothing is written.
func TestContextWorktreeOverlayCLI(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	if err := os.WriteFile(filepath.Join(root, "cache", "demux.go"), []byte("package cache\n\nfunc Split(key string) string { return wombatShard(key) }\n\nfunc wombatShard(key string) string { return key }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := repositoryListing(t, root)
	run := func(extra ...string) map[string]any {
		var stdout, stderr bytes.Buffer
		arguments := append([]string{"--root", root, "context", "--task", "where is wombatShard defined"}, extra...)
		if code := runContext(context.Background(), arguments, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		var packet map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &packet); err != nil {
			t.Fatal(err)
		}
		return packet
	}
	packet := run(worktreeOverlayFlag)
	cited := false
	for _, raw := range anyList(packet["results"]) {
		row := raw.(map[string]any)
		if row["id"] != "cache/demux.go" {
			continue
		}
		cited = true
		for _, rawEvidence := range anyList(row["evidence"]) {
			evidence := rawEvidence.(map[string]any)
			if evidence["authority"] != contextindex.WorktreeOverlayAuthority || evidence["confidence"] != "low" {
				t.Fatalf("overlaid row evidence = %#v", evidence)
			}
		}
	}
	if !cited {
		t.Fatalf("the overlay packet does not find the worktree-only identifier: %v", packet["results"])
	}
	coverage := packet["coverage"].(map[string]any)
	if !strings.Contains(strings.Join(anyStringList(coverage["uncertainty"]), "\n"), "--worktree-overlay 1 tracked paths") {
		t.Fatalf("coverage uncertainty = %v", coverage["uncertainty"])
	}
	plain := run()
	for _, raw := range anyList(plain["results"]) {
		row := raw.(map[string]any)
		for _, rawEvidence := range anyList(row["evidence"]) {
			if rawEvidence.(map[string]any)["authority"] == contextindex.WorktreeOverlayAuthority {
				t.Fatalf("default packet carries overlay evidence: %v", row)
			}
		}
	}
	if after := repositoryListing(t, root); after != before {
		t.Fatalf("context --worktree-overlay wrote to the repository:\n%s\n%s", before, after)
	}
}

func anyList(value any) []any {
	list, _ := value.([]any)
	return list
}

func anyStringList(value any) []string {
	var out []string
	for _, item := range anyList(value) {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}
