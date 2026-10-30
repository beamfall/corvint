package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// proveJavaScriptRepository is a one-directory web project without a Go
// module (decision 0015): core is the changed module, app imports it by an
// extensioned relative specifier, side by a bare stem, lazy through a
// dynamic import in TypeScript, and doc only names it inside a comment.
func proveJavaScriptRepository(t *testing.T) string {
	t.Helper()
	root := cliRepository(t)
	files := map[string]string{
		"web/core.js":  "export function compute(values) {\n  return values.length\n}\n",
		"web/app.js":   "// entry point\nimport { compute } from './core.js'\n\nconsole.log(compute([]))\n",
		"web/side.mjs": "import './core'\n",
		"web/lazy.ts":  "export async function load() {\n  const core = await import('./core.js')\n  return core.compute([])\n}\n",
		"web/doc.js":   "// see ./core.js for compute\nexport const note = 'core'\n",
		".gitignore":   ".corvint/\n",
	}
	for relative, content := range files {
		file := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "javascript fixture")
	return root
}

func TestProveImpactJudgesJavaScriptImportAndReferenceRows(t *testing.T) {
	t.Parallel()
	root := proveJavaScriptRepository(t)
	receipt, _, stderr, code := runProveCLI(t, root, "web/core.js")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	seen := map[string]map[string]any{}
	for _, row := range proofRows(t, receipt) {
		seen[stringAt(row, "result")] = row
	}
	for path, line := range map[string]int{"web/app.js": 2, "web/side.mjs": 1, "web/lazy.ts": 2} {
		row, present := seen[path]
		if !present {
			t.Fatalf("no reverse-import row for %s: %v", path, seen)
		}
		if row["falsifier"] != falsifierReference || row["falsified"] != falsifiedPass || integerAt(row, "line") != line {
			t.Fatalf("%s: %v", path, row)
		}
	}
	if _, present := seen["web/doc.js"]; present {
		t.Fatalf("a commented specifier must not become a row: %v", seen["web/doc.js"])
	}
	if receipt["state"] != "READY" || proofField(t, receipt, "failed_results") != 0 {
		t.Fatalf("state %v proof %v", receipt["state"], receipt["proof"])
	}
	// The index emits no `reference` rows for web paths (samePackageReferences
	// is Go-only), so the reference arm is judged here over the same committed
	// blobs the packet cites.
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git unavailable")
	}
	revision := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	cited, err := readCitedBlobs(context.Background(), gitExecutable, root, revision, []string{"web/app.js", "web/core.js", "web/doc.js"})
	if err != nil {
		t.Fatal(err)
	}
	reference := func(path, reason string, line int) proveRow {
		return proveRow{Falsifier: falsifierReference, Kind: "reference", Authority: "syntax", Path: path, BlobHash: cited[path].oid, Line: line, reason: reason}
	}
	for _, test := range []struct {
		name string
		row  proveRow
		want string
	}{
		{"call site resolves", reference("web/app.js", "references compute declared by web/core.js", 4), falsifiedPass},
		{"import clause resolves", reference("web/app.js", "references compute declared by web/core.js", 2), falsifiedPass},
		{"comment is not an occurrence", reference("web/doc.js", "references compute declared by web/core.js", 1), falsifiedFail},
		{"string is not an occurrence", reference("web/doc.js", "references core declared by web/core.js", 2), falsifiedFail},
		{"declaring blob lacks the name", reference("web/app.js", "references console declared by web/core.js", 4), falsifiedFail},
		{"declaring blob dirty", reference("web/app.js", "references compute declared by web/dirty.js", 4), falsifiedNotRun},
	} {
		if got := falsifyRow(test.row, cited, map[string]struct{}{"web/dirty.js": {}}); got != test.want {
			t.Errorf("%s: got %s want %s", test.name, got, test.want)
		}
	}
}
