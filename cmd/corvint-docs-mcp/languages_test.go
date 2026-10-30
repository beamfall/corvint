package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/docsbridge"
)

// languageFixtureHandler commits a repository holding a TypeScript module, a
// Java module and an owner page, and serves it through the real tool handler.
func languageFixtureHandler(t *testing.T) *toolHandler {
	t.Helper()
	_, handler := languageFixture(t)
	return handler
}

func languageFixture(t *testing.T) (string, *toolHandler) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"owner.md":                  "# Web\nDelivery status: experimental\n\n## Agent digest\n- Claim: renders badges\n- Blocked on: behavior validation\n\n## Requirements\nNever promote drafts.\n",
		"web/package.json":          "{\"name\": \"web\"}\n",
		"web/badge.ts":              "export function renderBadge(label: string): string {\n  return label\n}\n\nexport class BadgeCache {}\n\nconst hidden = 1\n",
		"web/badge.test.ts":         "export function testRenderBadge() {}\n",
		"web/legacy.java":           "class Legacy {}\n",
		"java/src/Ledger.java":      "public class Ledger {}\n",
		"java/src/LedgerTests.java": "public class LedgerTests {}\n",
	}
	for name, text := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "fixture"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	registry, err := docsbridge.New(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, &toolHandler{registry: registry}
}

func callDocsTool(t *testing.T, handler *toolHandler, name string, arguments map[string]any) map[string]any {
	t.Helper()
	result, failure := handler.call(context.Background(), map[string]any{"_meta": map[string]any{}, "name": name, "arguments": arguments})
	if failure != nil {
		t.Fatalf("%s: failure=%#v", name, failure)
	}
	return result
}

// TestDocsMCPDraftsTypeScriptModule: corvint.docs_draft accepts a TypeScript
// module directory, drafts only its exported non-test declarations, names the
// omitted Java source and the unobserved importers, and corvint.docs_consume
// rederives the same bytes (SDD-V0-018, V1-3072).
func TestDocsMCPDraftsTypeScriptModule(t *testing.T) {
	handler := languageFixtureHandler(t)
	draft := callDocsTool(t, handler, "corvint.docs_draft", map[string]any{"source": "owner.md", "package": "web"})
	if draft["isError"] != false {
		t.Fatalf("SDD-V0-018 TypeScript module refused: %#v", draft["structuredContent"])
	}
	markdown := draft["structuredContent"].(map[string]any)["markdown"].(string)
	for _, want := range []string{
		"## `\"func renderBadge\"`", "## `\"class BadgeCache\"`",
		"export function renderBadge(label: string): string {",
		"Non-Go languages drafted: TypeScript.", "importers of these sources are UNKNOWN",
		"Sources omitted because the native index extracts no declarations from their language: Java.",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("draft lacks %q:\n%s", want, markdown)
		}
	}
	for _, unwanted := range []string{"hidden", "testRenderBadge", "Legacy"} {
		if strings.Contains(markdown, unwanted) {
			t.Fatalf("draft cites %q, which is not an exported non-test declaration:\n%s", unwanted, markdown)
		}
	}
	consume := callDocsTool(t, handler, "corvint.docs_consume", map[string]any{"source": "owner.md", "package": "web", "task": "renderBadge", "draft": markdown})
	structured := consume["structuredContent"].(map[string]any)
	if consume["isError"] != false || structured["state"] != "READY" || structured["validation"] != "SOURCE_REDERIVED" {
		t.Fatalf("consume: %#v", structured)
	}
}

// TestDocsMCPRefusesUnsupportedLanguageByName: a module directory whose code
// the native index reads only as text is refused with the language named,
// never drafted empty (SDD-V0-018).
func TestDocsMCPRefusesUnsupportedLanguageByName(t *testing.T) {
	handler := languageFixtureHandler(t)
	result := callDocsTool(t, handler, "corvint.docs_draft", map[string]any{"source": "owner.md", "package": "java/src"})
	structured := result["structuredContent"].(map[string]any)
	if result["isError"] != true || structured["code"] != "unsupported-documentation-source" ||
		structured["error"] != "unsupported language: the native index extracts no declarations from Java" {
		t.Fatalf("SDD-V0-018 refusal: %#v", structured)
	}
}

// TestDocsMCPDryRunAcceptsUntrackedOrInlineOwner: corvint.docs_draft with
// dry_run previews over an untracked working-tree owner or inline
// owner_markdown, marks the result NOT_ADMISSIBLE, changes no repository
// state, and corvint.docs_consume refuses the dry-run markdown; without
// dry_run the untracked owner is still refused (SDD-V0-019, V1-3081, GH #1978).
func TestDocsMCPDryRunAcceptsUntrackedOrInlineOwner(t *testing.T) {
	root, handler := languageFixture(t)
	owner := "# Draft owner\n\n## Agent digest\n- Claim: first web draft\n"
	if err := os.WriteFile(filepath.Join(root, "draft-owner.md"), []byte(owner), 0o644); err != nil {
		t.Fatal(err)
	}
	status := func() string {
		command := exec.Command("git", "status", "--porcelain", "--ignored")
		command.Dir = root
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git status: %v %s", err, output)
		}
		return string(output)
	}
	before := status()
	refused := callDocsTool(t, handler, "corvint.docs_draft", map[string]any{"source": "draft-owner.md", "package": "web"})
	if refused["isError"] != true || refused["structuredContent"].(map[string]any)["code"] != "unsupported-documentation-source" {
		t.Fatalf("normal path accepted an untracked owner: %#v", refused)
	}
	for name, arguments := range map[string]map[string]any{
		"untracked": {"source": "draft-owner.md", "package": "web", "dry_run": true},
		"inline":    {"source": "docs/unwritten.md", "package": "web", "dry_run": true, "owner_markdown": owner},
	} {
		result := callDocsTool(t, handler, "corvint.docs_draft", arguments)
		structured := result["structuredContent"].(map[string]any)
		if result["isError"] != false || structured["dry_run"] != true || structured["admissibility"] != "NOT_ADMISSIBLE" || structured["authority"] != "NONE" || structured["mutates"] != false {
			t.Fatalf("%s dry run: %#v", name, structured)
		}
		markdown := structured["markdown"].(string)
		if !strings.Contains(markdown, "Admissibility: NOT_ADMISSIBLE") || !strings.Contains(markdown, "    - Claim: first web draft") || !strings.Contains(markdown, "export function renderBadge") {
			t.Fatalf("%s dry-run markdown:\n%s", name, markdown)
		}
		consume := callDocsTool(t, handler, "corvint.docs_consume", map[string]any{"source": "owner.md", "package": "web", "task": "renderBadge", "draft": markdown})
		if consume["isError"] != true || !strings.Contains(consume["structuredContent"].(map[string]any)["error"].(string), "dry-run draft is not admissible") {
			t.Fatalf("%s: consume accepted a dry-run draft: %#v", name, consume["structuredContent"])
		}
	}
	misuse := callDocsTool(t, handler, "corvint.docs_draft", map[string]any{"source": "owner.md", "package": "web", "owner_markdown": owner})
	if misuse["isError"] != true || misuse["structuredContent"].(map[string]any)["code"] != "invalid-arguments" {
		t.Fatalf("owner_markdown without dry_run: %#v", misuse)
	}
	if after := status(); after != before {
		t.Fatalf("dry run changed repository state:\n%s\n---\n%s", before, after)
	}
}
