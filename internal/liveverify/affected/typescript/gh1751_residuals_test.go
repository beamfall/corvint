package typescript

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func gh1751SelectedTests(plan affected.Plan) []string {
	var got []string
	for _, selection := range plan.Selected {
		got = append(got, strings.Join(selection.Tests, ","))
	}
	slices.Sort(got)
	return got
}

// GitHub #1751 (V1-2666, AFP-V0-495): a test-less TypeScript helper that reads a declared
// documentation path carries the change to the test that imports it, witnessed by the chain from
// the helper, instead of dropping it with no selection and no unknown.
func TestTestlessDocumentationReaderSelectsImportingTest_AFPV0495(t *testing.T) {
	root := gh1751GenericRoot(t, "import { setup } from '../src/contracts'\nimport { test } from 'vitest'\ntest('c', () => setup())\n")
	write(t, root, "src/contracts.ts", "import { readFileSync } from 'node:fs'\nexport const setup = () => readFileSync('docs/contracts/_setup.json')\n")
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"docs/contracts/_setup.json"})
	if got := gh1751SelectedTests(plan); !reflect.DeepEqual(got, []string{"tests/contract.test.ts"}) || plan.Scope != affected.ScopeBounded {
		t.Fatalf("selected=%v; plan %+v", got, plan)
	}
	witness := plan.Selected[0].Witness
	if witness.Kind != affected.WitnessDependency || witness.DirtyPath != "docs/contracts/_setup.json" || len(witness.Via) != 2 || !strings.HasSuffix(witness.Via[0], ":src/contracts.ts") {
		t.Fatalf("witness %+v", witness)
	}
}

// GitHub #1751 (V1-2667, TJAA-V0-126): a resolved import specifier is not a path token, so a
// dirty src/a/helper.js no longer selects a test that imports a different ./helper.js; the
// importer of the dirty module and a test that reads the same text as a file path stay selected.
func TestResolvedImportSpecifierIsNotPathToken_TJAA126(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/b/helper.js", "export const h = 1\n")
	write(t, root, "src/b/c.test.ts", "import { h } from './helper.js'\nimport { test } from 'vitest'\ntest('c', () => h)\n")
	write(t, root, "src/a/helper.js", "export const t = 2\n")
	write(t, root, "src/a/a.test.ts", "import { t as v } from './helper.js'\nimport { test } from 'vitest'\ntest('a', () => v)\n")
	write(t, root, "src/a/reader.test.ts", "import { readFileSync } from 'node:fs'\nimport { test } from 'vitest'\ntest('r', () => readFileSync('./helper.js'))\n")
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/a/helper.js"})
	if got := gh1751SelectedTests(plan); !reflect.DeepEqual(got, []string{"src/a/a.test.ts", "src/a/reader.test.ts"}) {
		t.Fatalf("selected=%v; plan %+v", got, plan)
	}
}

// A module with its own createRequire keeps a resolved specifier's token: the adapter resolved
// `./helper.js` beside the test, but the require may load the root file (TJAA-V0-126).
func TestCreateRequireKeepsResolvedSpecifierToken_TJAA126(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "helper.js", "module.exports = 1\n")
	write(t, root, "tests/helper.js", "module.exports = 2\n")
	write(t, root, "tests/reader.test.ts", "import { createRequire } from 'node:module'\nimport { test } from 'vitest'\nconst require = createRequire(new URL('../package.json', import.meta.url))\nconst h = require('./helper.js')\ntest('r', () => h)\n")
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"helper.js"})
	if got := gh1751SelectedTests(plan); !slices.Contains(got, "tests/reader.test.ts") {
		t.Fatalf("selected=%v; plan %+v", got, plan)
	}
}

// pathTokensSkipping leaves out only the quoted literal at a resolved specifier's offset; the
// same text elsewhere, and an unskipped specifier, are still tokens (TJAA-V0-126).
func TestPathTokensSkipOnlyTheResolvedSpecifier_TJAA126(t *testing.T) {
	body := "import x from './x.js'\nconst p = readFileSync('./x.js')\nimport y from './y.js'\n"
	skip := map[int]string{strings.Index(body, "./x.js"): "./x.js"}
	if got, _ := pathTokensSkipping(body, skip); !reflect.DeepEqual(got, []string{"./x.js", "./y.js"}) {
		t.Fatalf("tokens %v", got)
	}
	body = "import x from './x.js'\n"
	if got, bounded := pathTokensSkipping(body, map[int]string{strings.Index(body, "./x.js"): "./x.js"}); got != nil || bounded {
		t.Fatalf("tokens %v bounded=%v", got, bounded)
	}
	if got, _ := pathTokensSkipping(body, map[int]string{strings.Index(body, "./x.js"): "./x"}); !reflect.DeepEqual(got, []string{"./x.js"}) {
		t.Fatalf("a mismatched specifier must keep its literal: %v", got)
	}
}

// AFP-V0-494: the Playwright plan's graph-frontier detail carries an unknown's operator hint
// after its bare path.
func TestGraphFrontierDetailCarriesHint_AFPV0494(t *testing.T) {
	base := affected.Plan{Unknown: []affected.Unknown{{Reason: affected.UnknownUnownedDirtyPath, Detail: "node_modules", Hint: "hint text"}}}
	if detail := graphFrontierDetail(base, nil, nil, nil); !strings.Contains(detail, "UNOWNED_DIRTY_PATH node_modules (hint text)") {
		t.Fatalf("detail %q", detail)
	}
}
