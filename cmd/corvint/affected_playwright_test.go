package main

import (
	"bytes"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

func TestAffectedPlaywrightProfileEmitsProjectDistinctUnits(t *testing.T) {
	t.Run("AFP-V0-018 separate Playwright receipt remains deterministic and read-only", func(t *testing.T) {
		root := t.TempDir()
		writePlaywrightCLIFile(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
		writePlaywrightCLIFile(t, root, "playwright.config.ts", `
import { defineConfig } from "@playwright/test"
export default defineConfig({
  projects: [
    { name: "angular", grep: /@angular/, use: { browserName: "chromium" } },
    { name: "react", grep: /@react/, use: { browserName: "chromium" } },
  ],
})
`)
		writePlaywrightCLIFile(t, root, "src/page.ts", `export const page = "page"`)
		writePlaywrightCLIFile(t, root, "tests/page.spec.ts", `import { test } from "@playwright/test"; import { page } from "../src/page"; test("@angular @react page", () => page)`)
		affectedGit(t, root, "init", "-q")
		affectedGit(t, root, "config", "user.email", "corvint@example.test")
		affectedGit(t, root, "config", "user.name", "Corvint Test")
		affectedGit(t, root, "add", ".")
		affectedGit(t, root, "commit", "-qm", "fixture")
		writePlaywrightCLIFile(t, root, "src/page.ts", `export const page = "changed"`)

		first := runPlaywrightAffectedCLI(t, root)
		second := runPlaywrightAffectedCLI(t, root)
		if !bytes.Equal(first, second) {
			t.Fatalf("profile bytes changed across runs:\n%s\n%s", first, second)
		}
		var receipt map[string]any
		if err := json.Unmarshal(first, &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt["profile"] != typescript.PlaywrightProfile || receipt["tool"] != "affected" || receipt["mutates"] != false {
			t.Fatalf("receipt identity=%v", receipt)
		}
		plan := receipt["plan"].(map[string]any)
		if plan["scope"] != "BOUNDED" || plan["fallback"] != typescript.PlaywrightFallbackNone {
			t.Fatalf("plan scope=%v fallback=%v unknown=%v", plan["scope"], plan["fallback"], plan["unknown"])
		}
		selected := plan["selected"].([]any)
		if len(selected) != 2 {
			t.Fatalf("selected=%v", selected)
		}
		if selected[0].(map[string]any)["project"] != "angular" || selected[1].(map[string]any)["project"] != "react" {
			t.Fatalf("project units=%v", selected)
		}
		if status := affectedGit(t, root, "status", "--porcelain"); strings.TrimSpace(status) != "M src/page.ts" {
			t.Fatalf("command mutated fixture: %q", status)
		}
	})
}

func TestAffectedPlaywrightArgumentsFailClosed(t *testing.T) {
	root := affectedFixtureRepository(t)
	for _, arguments := range [][]string{
		{"--root", root, "affected", "--playwright-config", "../playwright.config.ts"},
		{"--root", root, "affected", "--playwright-config", "playwright.config.ts", "--provider", "record.json"},
	} {
		var stdout, stderr bytes.Buffer
		if code := runContext(t.Context(), arguments, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Fatalf("args=%v code=%d stdout=%s stderr=%s", arguments, code, stdout.String(), stderr.String())
		}
	}
}

func runPlaywrightAffectedCLI(t *testing.T, root string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(t.Context(), []string{"--root", root, "affected", "--playwright-config", "playwright.config.ts"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	return stdout.Bytes()
}

func writePlaywrightCLIFile(t *testing.T, root, relative, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
