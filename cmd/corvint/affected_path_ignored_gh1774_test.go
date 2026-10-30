package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// GitHub #1774 (V1-2674, AFP-V0-440): a --path plan in a worktree with a gitignored, untracked
// playwright-report/ bundle plans at HEAD, cached or not, and the receipt never names the report.
// A listed ignored file, an untracked report Git does not ignore and a modified tracked file the
// graph reads still refuse with unsupported-affected-mixed-worktree.
func TestAffectedPathListIgnoredReportPlans_GH1774(t *testing.T) {
	t.Parallel()
	const report = "playwright-report/trace/assets/codeMirrorModule-3f2a.js"
	fixture := func(t *testing.T, gitignore string) string {
		t.Helper()
		root := t.TempDir()
		writePlaywrightCLIFile(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
		writePlaywrightCLIFile(t, root, "tests/page.spec.ts", `import { test } from "@playwright/test"; test("page", () => {})`)
		writePlaywrightCLIFile(t, root, "tests/other.spec.ts", `import { test } from "@playwright/test"; test("other", () => {})`)
		if gitignore != "" {
			writePlaywrightCLIFile(t, root, ".gitignore", gitignore)
		}
		affectedGit(t, root, "init", "-q")
		affectedGit(t, root, "config", "user.email", "corvint@example.test")
		affectedGit(t, root, "config", "user.name", "Corvint Test")
		affectedGit(t, root, "add", ".")
		affectedGit(t, root, "commit", "-qm", "fixture")
		writePlaywrightCLIFile(t, root, report, "export async function load(name) { return import(name) }\n")
		writePlaywrightCLIFile(t, root, "playwright-report/index.html", "<html></html>\n")
		return root
	}
	refuses := func(t *testing.T, root, want string, options ...string) {
		t.Helper()
		receipt, stdout, stderr, code := runAffectedCLI(t, root, options...)
		head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		if code != 2 || len(stdout) != 0 || receipt != nil || !strings.Contains(stderr, "unsupported-affected-mixed-worktree") ||
			!strings.Contains(stderr, want) || !strings.Contains(stderr, head) {
			t.Fatalf("%q: exit %d stdout=%s stderr=%s", options, code, stdout, stderr)
		}
	}

	t.Run("ignored and untracked report is no graph input", func(t *testing.T) {
		t.Parallel()
		root := fixture(t, "/playwright-report/\n")
		cache := filepath.Join(t.TempDir(), "cache")
		for _, options := range [][]string{
			{"--no-cache", "--path", "tests/page.spec.ts"},
			{"--cache", cache, "--path", "tests/page.spec.ts"},
			{"--cache", cache, "--path", "tests/page.spec.ts"},
		} {
			receipt, stdout, stderr, code := runAffectedCLI(t, root, options...)
			if code != 0 || receipt == nil {
				t.Fatalf("%q: exit %d stderr=%s", options, code, stderr)
			}
			if strings.Contains(string(stdout), "playwright-report") {
				t.Fatalf("%q: receipt names the ignored report: %s", options, stdout)
			}
			if !strings.Contains(string(stdout), "tests/page.spec.ts") {
				t.Fatalf("%q: receipt does not plan the listed spec: %s", options, stdout)
			}
		}
		if status := affectedGit(t, root, "status", "--porcelain"); strings.TrimSpace(status) != "" {
			t.Fatalf("a --path plan must not write: %q", status)
		}
		// Naming the ignored file itself still refuses.
		refuses(t, root, report, "--path", report)
	})

	t.Run("untracked report Git does not ignore stays dirty", func(t *testing.T) {
		t.Parallel()
		refuses(t, fixture(t, ""), "playwright-report/", "--path", "tests/page.spec.ts")
	})

	// The exclusion is applied only when nothing outside the TypeScript units
	// changes: an ignored file another plugin lists or fails to parse refuses.
	for name, file := range map[string][2]string{
		"ignored Python file another plugin lists":    {"playwright-report/gen.py", "import os\n"},
		"ignored Go file the Go adapter cannot parse": {"playwright-report/trace/gen.go", "not go source\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := fixture(t, "/playwright-report/\n")
			writePlaywrightCLIFile(t, root, file[0], file[1])
			if strings.HasSuffix(file[0], ".go") {
				writePlaywrightCLIFile(t, root, "go.mod", "module example.test/report\n\ngo 1.22\n")
				affectedGit(t, root, "add", "go.mod")
				affectedGit(t, root, "commit", "-qm", "go module")
			}
			refuses(t, root, "playwright-report/", "--no-cache", "--path", "tests/page.spec.ts")
		})
	}

	t.Run("modified tracked file stays dirty", func(t *testing.T) {
		t.Parallel()
		root := fixture(t, "/playwright-report/\n")
		writePlaywrightCLIFile(t, root, "tests/other.spec.ts", `import { test } from "@playwright/test"; test("other changed", () => {})`)
		refuses(t, root, "tests/other.spec.ts", "--path", "tests/page.spec.ts")
	})
}
