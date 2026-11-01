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

// TestWorktreeOverlayFlagParses is DIRTY-CACHE-940: --worktree-overlay is a
// no-value flag on query and context, absent by default, and refused with
// context --expand.
func TestWorktreeOverlayFlagParses(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	parsed, err := parse([]string{"--root", root, "query", "--task", "split the key", worktreeOverlayFlag, "--limit", "3"})
	if err != nil || !parsed.queryOverlay || parsed.queryLimit != 3 || parsed.queryTask != "split the key" {
		t.Fatalf("query parse = %+v, %v", parsed, err)
	}
	if plain, err := parse([]string{"--root", root, "query", "--task", "split the key"}); err != nil || plain.queryOverlay {
		t.Fatalf("default query parse = %+v, %v", plain, err)
	}
	if _, err := parse([]string{"--root", root, "query", "--task", "t", worktreeOverlayFlag + "=true"}); err == nil {
		t.Fatal("--worktree-overlay=true accepted; the flag takes no value")
	}
	options, isContext, err := parseTaskContextInvocation([]string{"--root", root, "context", "--task", "t", worktreeOverlayFlag})
	if err != nil || !isContext || !options.overlay {
		t.Fatalf("context parse = %+v, %v, %v", options, isContext, err)
	}
	if plain, _, err := parseTaskContextInvocation([]string{"--root", root, "context", "--task", "t"}); err != nil || plain.overlay {
		t.Fatalf("default context parse = %+v, %v", plain, err)
	}
	if _, _, err := parseTaskContextInvocation([]string{"--root", root, "context", "--expand", "cv1:x", worktreeOverlayFlag}); err == nil {
		t.Fatal("--worktree-overlay with --expand accepted")
	}
}

// TestQueryWorktreeOverlayCLI is DIRTY-CACHE-940..942 through the binary: the
// overlay receipt reads the dirty tracked file's worktree bytes under the HEAD
// revision, cites its worktree blob id as advisory low-confidence evidence,
// names the overlay scope, and writes nothing; the default receipt is the
// committed-tree receipt.
func TestQueryWorktreeOverlayCLI(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	target := filepath.Join(root, "internal", "parser", "token.go")
	if err := os.WriteFile(target, []byte("package parser\n\n// ParseToken validates token parser delimiters with the quokkaDelimiter rule.\nfunc ParseToken(input string) string { return quokkaDelimiter(input) }\n\nfunc quokkaDelimiter(input string) string { return input }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := repositoryBytesDigest(t, root)
	status := gitOutput(t, root, "status", "--porcelain=v1", "-uall")
	worktreeBlob := gitOutput(t, root, "hash-object", "internal/parser/token.go")
	headBlob := gitOutput(t, root, "rev-parse", "HEAD:internal/parser/token.go")
	tree := gitOutput(t, root, "rev-parse", "HEAD^{tree}")

	overlay := runRepositoryQueryProcess(t, root, worktreeOverlayFlag)
	if overlay.exit != 0 {
		t.Fatalf("overlay exit %d: %s", overlay.exit, overlay.stderr)
	}
	receipt := decodeRepositoryQueryContext(t, overlay.stdout)
	freshness := objectField(t, receipt, "freshness")
	if receipt["revision"] != tree || freshness["state"] != "mixed-worktree" || freshness["scope"] != contextindex.WorktreeOverlayScope {
		t.Fatalf("revision=%v freshness=%#v, want HEAD tree %s under the overlay scope", receipt["revision"], freshness, tree)
	}
	if count, ok := freshness["overlay_path_count"].(float64); !ok || count != 1 {
		t.Fatalf("overlay_path_count = %#v", freshness["overlay_path_count"])
	}
	cited := false
	for _, raw := range arrayField(t, receipt, "results") {
		result := raw.(map[string]any)
		for _, rawEvidence := range anyList(result["evidence"]) {
			evidence := rawEvidence.(map[string]any)
			if evidence["path"] != "internal/parser/token.go" {
				continue
			}
			cited = true
			if evidence["blob_hash"] != worktreeBlob || evidence["authority"] != contextindex.WorktreeOverlayAuthority || evidence["confidence"] != "low" {
				t.Fatalf("overlaid evidence = %#v (worktree %s, HEAD %s)", evidence, worktreeBlob, headBlob)
			}
			if result["advisory"] != true || result["worktree_overlay"] != true {
				t.Fatalf("overlaid result = %#v", result)
			}
		}
	}
	if !cited {
		t.Fatalf("no result cites the dirty tracked file: %s", overlay.stdout)
	}
	if after := repositoryBytesDigest(t, root); after != before {
		t.Fatal("query --worktree-overlay changed repository bytes")
	}
	if after := gitOutput(t, root, "status", "--porcelain=v1", "-uall"); after != status {
		t.Fatalf("status changed: %q, want %q", after, status)
	}

	plain := decodeRepositoryQueryContext(t, runRepositoryQueryProcess(t, root).stdout)
	if plainFreshness := objectField(t, plain, "freshness"); plainFreshness["scope"] == contextindex.WorktreeOverlayScope || plainFreshness["overlay_path_count"] != nil {
		t.Fatalf("default freshness carries overlay fields: %#v", plainFreshness)
	}
	for _, raw := range arrayField(t, plain, "results") {
		for _, rawEvidence := range anyList(raw.(map[string]any)["evidence"]) {
			if evidence := rawEvidence.(map[string]any); evidence["path"] == "internal/parser/token.go" && evidence["blob_hash"] != headBlob {
				t.Fatalf("default evidence = %#v, want HEAD blob %s", evidence, headBlob)
			}
		}
	}
}

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
