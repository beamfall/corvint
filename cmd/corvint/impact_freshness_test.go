package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runFreshnessImpact(t *testing.T, root string, args ...string) (map[string]any, []byte) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"--root", root, "impact"}, args...), strings.NewReader(""), &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["mutates"] != false {
		t.Fatal(payload)
	}
	return payload["context"].(map[string]any), stdout.Bytes()
}

// DIRTY-CACHE-014: staged, unstaged and untracked status never changes read authority.
func TestImpactMixedCLIReadOnly(t *testing.T) {
	for _, kind := range []string{"unstaged", "staged", "untracked"} {
		t.Run(kind, func(t *testing.T) {
			root := impactCLIRepository(t)
			name := "pkg/main_test.go"
			if kind == "untracked" {
				name = "notes.txt"
			}
			if err := os.WriteFile(filepath.Join(root, name), []byte("// MUTABLE_CANARY\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if kind == "staged" {
				affectedGit(t, root, "add", name)
			}
			before := treeDigest(t, root)
			got, raw := runFreshnessImpact(t, root, "pkg/main.go")
			commit := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
			warning, _ := got["warning"].(string)
			if !strings.Contains(warning, commit) || !strings.Contains(warning, "1 dirty paths") || bytes.Contains(raw, []byte("MUTABLE_CANARY")) {
				t.Fatalf("bad warning/content: %s", raw)
			}
			if treeDigest(t, root) != before {
				t.Fatal("impact wrote repository state")
			}
		})
	}
}

func TestImpactRequireCleanCLI(t *testing.T) {
	root := impactCLIRepository(t)
	_, ordinary := runFreshnessImpact(t, root, "pkg/main.go")
	_, strict := runFreshnessImpact(t, root, "--require-clean", "pkg/main.go")
	if !bytes.Equal(ordinary, strict) {
		t.Fatal("clean strict receipt changed")
	}
	if err := os.WriteFile(filepath.Join(root, "pkg/main.go"), []byte("package main // DIRTY_CANARY\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, _ := runFreshnessImpact(t, root, "./pkg/main.go", "pkg/main.go")
	if got["warning"] != nil {
		t.Fatal("normalized all-dirty request warned")
	}
	for _, limit := range []string{"-1", "0", "51"} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{"--root", root, "impact", "--require-clean", "--limit=" + limit, "pkg/main.go"}, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Fatalf("invalid strict limit %s: exit=%d out=%s err=%s", limit, code, stdout.String(), stderr.String())
		}
	}
	for _, target := range []string{"pkg/main.go", "pkg/main_test.go"} {
		before := treeDigest(t, root)
		// If a provider ran, the external section would contain a missing-file refusal.
		got, raw := runFreshnessImpact(t, root, "--require-clean", target, "--provider", "absent-provider.json")
		abstention, _ := got["abstention"].(map[string]any)
		if got["state"] != "NEEDS_WIDENING" || abstention["active"] != true || abstention["reason"] != "mixed-worktree" || len(got["results"].([]any)) != 0 || got["external"] != nil || got["warning"] == nil {
			t.Fatalf("not an empty abstention: %s", raw)
		}
		if treeDigest(t, root) != before {
			t.Fatal("strict impact wrote repository state")
		}
	}
}

func TestImpactRequireCleanArguments(t *testing.T) {
	for _, args := range [][]string{
		{"--require-clean=true", "pkg/main.go"},
		{"--require-clean", "--require-clean", "pkg/main.go"},
		{"--require-clean", "--working-tree-untracked", "pkg/new.go"},
		{"--require-clean", "--base", strings.Repeat("a", 40)},
	} {
		if _, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, args, "darwin"); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if !strings.Contains(impactHelp, "--require-clean") || !strings.Contains(impactHelp, "NEEDS_WIDENING") {
		t.Fatal("missing help")
	}
}
