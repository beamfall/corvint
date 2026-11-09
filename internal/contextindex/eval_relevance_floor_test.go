package contextindex

import (
	"context"
	"testing"
)

// TestEvalStrongestSupportCountsQueryWords pins the two properties the floor
// depends on: support is counted in words of the query as written rather than
// in the stems terms() derives from them, and a result carried into the packet
// behind another result contributes no support of its own.
func TestEvalStrongestSupportCountsQueryWords(t *testing.T) {
	ordered := evalOrderedTerms("autumn leaves rendering parity")
	supportIndex := map[string]map[string]struct{}{
		"decision:docs/adr/0001.md": intersectionSet([]string{"leaves", "leave"}),
		"feature:parity-render":     intersectionSet([]string{"rendering", "parity"}),
	}
	stemOnly := []map[string]any{{"kind": "decision", "id": "docs/adr/0001.md"}}
	if support := evalStrongestSupport(stemOnly, supportIndex, ordered); support != 1 {
		t.Fatalf("stem variants counted as %d words, want 1", support)
	}
	carried := []map[string]any{{"kind": "symbol", "id": "internal/render/render.go:Draw"}}
	if support := evalStrongestSupport(carried, supportIndex, ordered); support != 0 {
		t.Fatalf("unscored carried result counted as %d words, want 0", support)
	}
	twoWords := []map[string]any{{"kind": "feature", "id": "parity-render"}}
	if support := evalStrongestSupport(twoWords, supportIndex, ordered); support != 2 {
		t.Fatalf("two matched words counted as %d, want 2", support)
	}
}

// V1-3052: the exact-naming exemption reads the path and the name as two
// pointers. A path whose stem equals the declaration's name does not also name
// the declaration, so "Explain add.py" with `def add` stays under the scope
// check, and neither does another path-like token (`other/add.py`,
// `docs/Add.md`); the `./path` and `path:Name` spellings name it.
func TestEvalScopeNamesExactly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "add.py", "def add():\n    return 1\n")
	writeTestFile(t, root, "pkg/add.go", "package pkg\nfunc Add() {}\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "fixture")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		text, id, name string
		want           bool
	}{
		{text: "Explain add.py", id: "add.py:add", name: "add"},
		{text: "Explain add in add.py", id: "add.py:add", name: "add", want: true},
		{text: "Locate Add in ./pkg/add.go.", id: "pkg/add.go:Add", name: "Add", want: true},
		{text: "Locate `pkg/add.go:Add`", id: "pkg/add.go:Add", name: "Add", want: true},
		{text: "Locate Add in add.go", id: "pkg/add.go:Add", name: "Add"},
		{text: "Locate Add in missing.go", id: "missing.go:Add", name: "Add"},
		{text: "Locate Sub in pkg/add.go", id: "pkg/add.go:Add", name: "Add"},
		{text: "Explain add.py and other/add.py", id: "add.py:add", name: "add"},
		{text: "Locate pkg/add.go per docs/Add.md", id: "pkg/add.go:Add", name: "Add"},
		{text: "Locate Server.Run in pkg/add.go", id: "pkg/add.go:Server.Run", name: "Server.Run", want: true},
	} {
		scope := evalScope{index: index, text: testCase.text}
		result := map[string]any{"kind": "symbol", "id": testCase.id, "name": testCase.name}
		if got := scope.namesExactly(result); got != testCase.want {
			t.Errorf("namesExactly(%q, %s) = %v, want %v", testCase.text, testCase.id, got, testCase.want)
		}
	}
}
