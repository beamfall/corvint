package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// proveGoRepository is a two-package module: core declares symbols, helper
// references them in the same package, and app imports core.
func proveGoRepository(t *testing.T) string {
	t.Helper()
	root := cliRepository(t)
	files := map[string]string{
		"go.mod":             "module example.test/prove\n\ngo 1.27.0\n",
		"pkg/core/core.go":   "package core\n\n// ComputeTotal sums.\nfunc ComputeTotal(values []int) int { return len(values) }\n\ntype Ledger struct{ Rows int }\n",
		"pkg/core/helper.go": "package core\n\nfunc helper() int {\n\treturn ComputeTotal(nil)\n}\n",
		"pkg/app/app.go":     "package app\n\nimport (\n\t\"fmt\"\n\t\"example.test/prove/pkg/core\"\n)\n\nfunc Run() { fmt.Println(core.ComputeTotal(nil)) }\n",
		".gitignore":         ".corvint/\n",
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
	affectedGit(t, root, "commit", "-qm", "go fixture")
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", "self-observations.jsonl"), []byte("{\"seed\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestProveImpactJudgesGoImportAndReferenceRows(t *testing.T) {
	t.Parallel()
	root := proveGoRepository(t)
	before := treeDigest(t, root)
	receipt, first, stderr, code := runProveCLI(t, root, "pkg/core/core.go")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if receipt["state"] != "READY" {
		t.Fatalf("state %v: %v", receipt["state"], receipt["proof"])
	}
	seen := map[string]string{}
	appLines := map[int]bool{}
	for _, row := range proofRows(t, receipt) {
		seen[stringAt(row, "result")+"|"+stringAt(row, "falsifier")] = stringAt(row, "falsified")
		if stringAt(row, "result") == "pkg/app/app.go" {
			appLines[integerAt(row, "line")] = true
		}
	}
	// The import row cites the import spec line; proposed GPK-V0-067 adds the
	// `core.ComputeTotal` call on line 8 as reference evidence on the same row.
	if len(appLines) != 2 || !appLines[5] || !appLines[8] {
		t.Fatalf("app.go rows must cite the import spec line 5 and the call line 8: %v", appLines)
	}
	for key, want := range map[string]string{
		"pkg/core/core.go|" + falsifierHistory:     falsifiedPass,
		"pkg/app/app.go|" + falsifierReference:     falsifiedPass,
		"pkg/core/helper.go|" + falsifierReference: falsifiedPass,
	} {
		if seen[key] != want {
			t.Fatalf("%s: got %q want %s in %v", key, seen[key], want, seen)
		}
	}
	if proofField(t, receipt, "failed_results") != 0 || proofField(t, receipt, "proven_results") < 3 {
		t.Fatalf("proof: %v", receipt["proof"])
	}
	if _, second, _, _ := runProveCLI(t, root, "pkg/core/core.go"); string(first) != string(second) {
		t.Fatal("two runs over one tree differ")
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("prove impact changed the repository or its ignored ledger")
	}
	packet, _ := receipt["packet"].(map[string]any)
	if packet["mode"] != "impact" {
		t.Fatalf("embedded packet mode %v", packet["mode"])
	}
}

// TestProveImpactRefusesWorktreeModeAndUnknownBase: the untracked worktree
// form stays refused; an unavailable --base commit is change mode's own typed
// range refusal (FPK-V0-017).
func TestProveImpactRefusesWorktreeModeAndUnknownBase(t *testing.T) {
	t.Parallel()
	root := proveGoRepository(t)
	for _, test := range []struct {
		arguments []string
		code      string
	}{
		{[]string{"--base", strings.Repeat("a", 40)}, "unsupported-impact-range"},
		{[]string{"--working-tree-untracked", "pkg/core/core.go"}, "invalid-arguments"},
	} {
		_, stdout, stderr, code := runProveCLI(t, root, test.arguments...)
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, test.code) {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q", test.arguments, code, stdout, stderr)
		}
	}
}

func TestJudgeReferenceVerdicts(t *testing.T) {
	t.Parallel()
	citing := []byte("package core\n\nfunc helper() int {\n\treturn ComputeTotal(nil)\n}\n\nvar label = \"ComputeTotal\"\n")
	importer := []byte("package app\n\nimport (\n\t\"fmt\"\n\t\"example.test/prove/pkg/core\"\n)\n\nvar _ = fmt.Sprint(core.Ledger{})\n")
	declaring := []byte("package core\n\nfunc ComputeTotal(values []int) int { return len(values) }\n\ntype Ledger struct{}\n\nvar Rows, Columns int\n")
	cited := map[string]citedBlob{
		"pkg/core/helper.go": {oid: "h", objectType: "blob", content: citing},
		"pkg/app/app.go":     {oid: "a", objectType: "blob", content: importer},
		"pkg/core/core.go":   {oid: "c", objectType: "blob", content: declaring},
		"pkg/core/broken.go": {oid: "b", objectType: "blob", content: []byte("package core\n\nfunc (\n")},
	}
	dirty := map[string]struct{}{"pkg/core/dirty.go": {}}
	reference := func(path, blob, reason string, line int) proveRow {
		return proveRow{Falsifier: falsifierReference, Path: path, BlobHash: blob, Line: line, reason: reason}
	}
	for _, test := range []struct {
		name string
		row  proveRow
		want string
	}{
		{"reference on its line", reference("pkg/core/helper.go", "h", "references ComputeTotal declared by pkg/core/core.go", 4), falsifiedPass},
		{"reference on another line", reference("pkg/core/helper.go", "h", "references ComputeTotal declared by pkg/core/core.go", 3), falsifiedFail},
		{"name only inside a string", reference("pkg/core/helper.go", "h", "references ComputeTotal declared by pkg/core/core.go", 7), falsifiedFail},
		{"declaring blob lacks the name", reference("pkg/core/helper.go", "h", "references helper declared by pkg/core/core.go", 3), falsifiedFail},
		{"var group declares the name", reference("pkg/app/app.go", "a", "references Columns declared by pkg/core/core.go", 8), falsifiedFail},
		{"declaring blob dirty", reference("pkg/core/helper.go", "h", "references ComputeTotal declared by pkg/core/dirty.go", 4), falsifiedNotRun},
		{"declaring blob missing", reference("pkg/core/helper.go", "h", "references ComputeTotal declared by pkg/core/gone.go", 4), falsifiedFail},
		{"citing blob unparseable", reference("pkg/core/broken.go", "b", "references ComputeTotal declared by pkg/core/core.go", 1), falsifiedFail},
		{"citing blob replaced", reference("pkg/core/helper.go", "zzz", "references ComputeTotal declared by pkg/core/core.go", 4), falsifiedFail},
		{"unreadable claim", reference("pkg/core/helper.go", "h", "mentions ComputeTotal", 4), falsifiedFail},
		{"import on its line", reference("pkg/app/app.go", "a", "imports example.test/prove/pkg/core", 5), falsifiedPass},
		{"import on another line", reference("pkg/app/app.go", "a", "imports example.test/prove/pkg/core", 4), falsifiedFail},
		{"import of another path", reference("pkg/app/app.go", "a", "imports example.test/prove/pkg/other", 5), falsifiedFail},
		{"citing path dirty", reference("pkg/core/dirty.go", "d", "imports fmt", 1), falsifiedNotRun},
	} {
		if got := falsifyRow(test.row, cited, dirty); got != test.want {
			t.Errorf("%s: got %s, want %s", test.name, got, test.want)
		}
	}
	if !goDeclaresAtTopLevel(declaring, "Columns") || !goDeclaresAtTopLevel(declaring, "Ledger") || goDeclaresAtTopLevel(declaring, "values") {
		t.Error("top-level names must cover func, type, and every var in a group, and nothing nested")
	}
}

func TestFalsifierAssignmentByKindAndLanguage(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		row  proveRow
		want string
	}{
		{proveRow{Kind: "reverse-import", Authority: "syntax", Path: "pkg/app/app.go"}, falsifierReference},
		{proveRow{Kind: "reference", Authority: "syntax", Path: "pkg/core/helper.go"}, falsifierReference},
		{proveRow{Kind: "reverse-import", Authority: "syntax", Path: "app/main.py"}, falsifierReference},
		{proveRow{Kind: "reverse-import", Authority: "syntax", Path: "web/app.ts"}, falsifierReference},
		{proveRow{Kind: "reference", Authority: "syntax", Path: "web/app.jsx"}, falsifierReference},
		{proveRow{Kind: "reverse-import", Authority: "syntax", Path: "web/types.d.ts"}, falsifierReference},
		{proveRow{Kind: "reverse-import", Authority: "syntax", Path: "src/main.rs"}, falsifierHistory},
		{proveRow{Kind: "symbol", Authority: "syntax", Path: "src/main.rs"}, falsifierHistory},
		{proveRow{Kind: "path", Authority: "git-tree", Path: "pkg/core/core.go"}, falsifierHistory},
		{proveRow{Kind: "instructions", Authority: "project-instructions", Path: "AGENTS.md"}, falsifierHistory},
		{proveRow{Kind: "learned-path", Authority: "git-history", Path: "docs/x.md"}, falsifierNone},
	} {
		if got := falsifierFor(test.row); got != test.want {
			t.Errorf("%+v: got %s want %s", test.row, got, test.want)
		}
	}
	paths := citedPaths([]proveRow{
		{Falsifier: falsifierReference, Path: "pkg/core/helper.go", reason: "references ComputeTotal declared by pkg/core/core.go"},
		{Falsifier: falsifierNone, Path: "docs/x.md"},
	}, false)
	if strings.Join(paths, ",") != "pkg/core/core.go,pkg/core/helper.go" {
		t.Fatalf("cited paths must include the declaring path and skip none rows: %q", paths)
	}
}
