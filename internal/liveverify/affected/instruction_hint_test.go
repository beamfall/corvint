package affected

import (
	"path/filepath"
	"slices"
	"testing"
)

// AFP-V0-763 (V1-2077): an undeclared agent instruction file stays an
// UNOWNED_DIRTY_PATH, now with a hint naming the documentation declaration;
// another unowned path gets no hint; a declared instruction file is
// DECLARED_DOCUMENTATION and no unknown.
func TestInstructionFileUnknownNamesTheDeclarationRoute_AFPV0763(t *testing.T) {
	t.Parallel()
	graph, err := documentationGraph(t, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"AGENTS.md", "README.txt", "sub/CLAUDE.md"})
	hints := map[string]string{}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == UnknownUnownedDirtyPath {
			hints[unknown.Detail] = unknown.Hint
		}
	}
	if hints["AGENTS.md"] != instructionUnownedHint || hints["sub/CLAUDE.md"] != instructionUnownedHint || hints["README.txt"] != "" || len(hints) != 3 {
		t.Fatalf("unowned hints = %#v", hints)
	}
	if plan.Scope != ScopeUnknown {
		t.Fatalf("scope = %s, want UNKNOWN for an undeclared instruction file", plan.Scope)
	}
	declared, err := documentationGraph(t, `{"profile":"corvint-documentation-paths/0","paths":["AGENTS.md","CLAUDE.md"]}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	classified := Select(declared, []string{"AGENTS.md", "CLAUDE.md"})
	if len(classified.Unknown) != 0 || classified.Scope != ScopeBounded || !slices.Equal(groupPaths(classified, ClassDeclaredDocumentation), []string{"AGENTS.md", "CLAUDE.md"}) {
		t.Fatalf("declared instruction files: scope %s unknown %v nonSource %v", classified.Scope, classified.Unknown, classified.NonSource)
	}
}

// AFP-V0-763: this repository declares its own instruction files.
func TestRepositoryDeclaresItsInstructionFiles_AFPV0763(t *testing.T) {
	t.Parallel()
	_, declared, err := ReadDocumentationDeclaration(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if !slices.Contains(declared, name) {
			t.Errorf("%s does not declare %s", DocumentationPathsPath, name)
		}
	}
}
