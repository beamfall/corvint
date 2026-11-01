package typescript

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestJSXQuoteAfterLessThanIsALiteral_V13612 pins TJAA-V0-871 (V1-3612): in a
// .tsx source a quote after `<` opens a literal (a string literal type
// argument or a comparison), so the file's static import stays an edge and the
// test that really imports the changed module is selected; a quote after `>`
// may be JSX text and still raises typescript:unparsed-source.
func TestJSXQuoteAfterLessThanIsALiteral_V13612(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/dep.ts", `export const dep = 1`)
	write(t, root, "src/other.ts", `export const other = 2`)
	write(t, root, "tests/a.test.tsx", "import { test } from \"vitest\"\nimport { dep } from \"../src/dep\"\n"+
		"const [state] = useState<\"idle\" | \"busy\">(\"idle\")\n"+
		"type Style = ComponentProps<\"button\">[\"style\"]\n"+
		"const less = dep < \"b\"\n"+
		"const view = <p>{state}</p>\n")
	write(t, root, "tests/b.test.ts", "import { test } from \"vitest\"\nimport { other } from \"../src/other\"\n")
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/dep.ts"})
	selected := plan.SelectedTests()
	if plan.Scope == affected.ScopeUnknown || len(plan.Unknown) != 0 || len(selected) != 1 || !strings.Contains(strings.Join(selected, " "), "a.test.tsx") {
		t.Fatalf("scope=%s selected=%v unknown=%v; want the importing test alone, bounded", plan.Scope, selected, plan.Unknown)
	}

	write(t, root, "tests/a.test.tsx", "import { test } from \"vitest\"\nimport { dep } from \"../src/dep\"\nconst view = <p>\"quoted\"</p>\n")
	graph, err = affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan = affected.Select(graph, []string{"src/dep.ts"})
	want := affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: FrontierUnparsedSource + ":tests/a.test.tsx:3 `\"quoted\"</p>`"}
	if plan.Scope != affected.ScopeUnknown || len(plan.Unknown) == 0 || plan.Unknown[0] != want {
		t.Fatalf("scope=%s unknown=%v; want %v", plan.Scope, plan.Unknown, want)
	}
}
