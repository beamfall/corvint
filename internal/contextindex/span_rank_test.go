package contextindex

import (
	"context"
	"testing"
)

// spanFixtureFiles define RunEngine in one file and call it from another, so
// the span ranker has a core definition and a call site to emit.
var spanFixtureFiles = map[string]string{
	"go.mod": "module example.test/spans\n\ngo 1.27.0\n",
	"pkg/core/engine.go": "package core\n\n// RunEngine turns the crank once.\nfunc RunEngine(x int) int {\n\treturn crankOnce(x)\n}\n\n" +
		"func crankOnce(x int) int {\n\treturn x + 1\n}\n",
	"pkg/app/main.go":  "package app\n\nimport \"example.test/spans/pkg/core\"\n\n// Start runs the engine.\nfunc Start() int {\n\treturn core.RunEngine(1)\n}\n",
	"pkg/util/text.go": "package util\n\nfunc Trim(value string) string { return value }\n",
	"docs/engine.md":   "# engine\n\nThe engine turns a crank.\n",
}

func spanFixture(t *testing.T) (string, *Index) {
	t.Helper()
	root := impactRepositoryWithFiles(t, spanFixtureFiles)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return root, index
}

func spanRowsOf(t *testing.T, packet map[string]any) (map[string]any, []map[string]any) {
	t.Helper()
	block, ok := packet["spans"].(map[string]any)
	if !ok {
		t.Fatalf("packet has no spans block: %#v", packet["spans"])
	}
	return block, mapsFromAny(block["rows"])
}

func findSpan(rows []map[string]any, role, path string) map[string]any {
	for _, row := range rows {
		if row["role"] == role && row["path"] == path {
			return row
		}
	}
	return nil
}
