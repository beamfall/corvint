package appmap

import (
	"strings"
	"testing"
)

const effectFieldPage = `import { Locator, Page } from '@playwright/test';

export class FieldPage {
  readonly page: Page;
  readonly save: Locator;
  readonly title: Locator;

  constructor(page: Page) {
    this.page = page;
    this.save = page.getByRole('button', { name: 'Save' });
    this.title = page.getByTestId('title');
  }

  async open() {
    await this.page.goto('/#!/fields');
  }

  async swap() {
    this.save = this.page.getByRole('button', { name: 'Other' });
  }
}
`

// V1-3889 (ANL-V0-035, ANL-V0-920): the Playwright page-object style, a field declared on the
// class and assigned in its constructor, is read rather than member-assignment; a method that
// reassigns a declared field stays unknown.
func TestANLV0920ConstructorAssignsDeclaredFields(t *testing.T) {
	root, _ := fixtureRepo(t)
	writeFile(t, root, "e2e/pages/home.page.ts", effectHomePage)
	writeFile(t, root, "e2e/pages/teesheet.page.ts", effectSheetPage)
	writeFile(t, root, "e2e/pages/field.page.ts", effectFieldPage)
	withField := func(step string) string {
		return strings.Replace(effectSpec(step), "import { TeeSheetPage }", "import { FieldPage } from '../pages/field.page';\nimport { TeeSheetPage }", 1)
	}
	writeFile(t, root, "e2e/specs/field-construct.spec.ts", withField("const fields = new FieldPage(page); await fields.open();"))
	writeFile(t, root, "e2e/specs/field-swap.spec.ts", withField("const fields = new FieldPage(page); await fields.swap();"))
	rev := commitAll(t, root, "fields")
	m := build(t, root, rev)
	for _, c := range []struct{ spec, effect, ref, call string }{
		{"field-construct", EffectNavigation, "", ""},
		{"field-swap", EffectUnknown, "e2e/pages/field.page.ts:19", "member-assignment"},
	} {
		edges := stepEdges(m, "e2e/specs/"+c.spec+".spec.ts")
		if len(edges) == 0 {
			t.Fatalf("%s: no step edges", c.spec)
		}
		// The step runs between the HomePage and TeeSheetPage constructions: the first edge.
		if e := edges[0]; e.Effect != c.effect || e.EffectRef != c.ref || e.EffectCall != c.call {
			t.Errorf("%s: got %s %s %s, want %s %s %s", c.spec, e.Effect, e.EffectRef, e.EffectCall, c.effect, c.ref, c.call)
		}
	}
}
