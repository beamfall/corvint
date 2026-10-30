package changee2e

import "testing"

const reporterPage = `export default class RequireExecutedTestsReporter {
  onBegin() {}

  onEnd() {
    return { status: 'passed' };
  }
}
`

// CCE-V0-139: a Playwright Reporter class under a declared page_objects directory lists its methods
// under unadmitted, admits no page-object block for them, and leaves real page objects admitted.
func TestCCEV0139ReporterIsNotAPageObject(t *testing.T) {
	root, _, head := vocabularyRepo(t, map[string]*string{"e2e/pages/require-executed.reporter.ts": text(reporterPage)})
	cat, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: head})
	if err != nil {
		t.Fatal(err)
	}
	admitted, reporters := 0, false
	for _, b := range cat.Blocks {
		if b.Kind != KindPageObject {
			continue
		}
		if b.File == "e2e/pages/require-executed.reporter.ts" {
			t.Fatalf("a reporter method was admitted as a page-object block: %+v", b)
		}
		admitted++
	}
	for _, u := range cat.Unadmitted {
		if u == (Unadmitted{Kind: KindPageObject, ID: "method:e2e/pages/require-executed.reporter.ts#onEnd", Reason: "page-object-is-reporter"}) {
			reporters = true
		}
	}
	if !reporters {
		t.Fatalf("the reporter's onEnd is not unadmitted as page-object-is-reporter: %+v", cat.Unadmitted)
	}
	if admitted == 0 {
		t.Fatal("real page-object methods in the same directory are no longer admitted")
	}
}

// typedReporter imports the Reporter API module and declares a helper method beside its lifecycle
// method; the import alone marks it a reporter (V1-2956).
const typedReporter = `import type { Reporter, TestCase, TestResult } from '@playwright/test/reporter';

export default class VisualParityReporter implements Reporter {
  onTestEnd(test: TestCase, result: TestResult) {}

  summary() {
    return 'ok';
  }
}
`

// lifecyclePage is a page object that names a method onEnd: its Page import and selector keep it
// admitted (V1-2956).
const lifecyclePage = `import { Page } from '@playwright/test';

export class WizardPage {
  constructor(private readonly page: Page) {}

  async onEnd() {
    await this.page.getByRole('button', { name: 'Finish' }).click();
  }
}
`

// CCE-V0-139 (V1-2956): admission uses the structural signal. A reporter importing the Reporter
// API module is unadmitted whatever its method names; a page object that declares onEnd but holds
// a Page stays admitted; the catalog admits only page-object methods.
func TestCCEV0139ReporterAdmissionIsStructural(t *testing.T) {
	root, _, head := vocabularyRepo(t, map[string]*string{
		"e2e/pages/visual-parity.reporter.ts": text(typedReporter),
		"e2e/pages/wizard.page.ts":            text(lifecyclePage),
	})
	cat, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: head})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, b := range cat.Blocks {
		if b.Kind == KindPageObject {
			got[b.ID] = true
		}
	}
	for _, id := range []string{"method:e2e/pages/visual-parity.reporter.ts#onTestEnd", "method:e2e/pages/visual-parity.reporter.ts#summary"} {
		if got[id] {
			t.Fatalf("reporter method %s was admitted as a page-object block", id)
		}
	}
	if !got["method:e2e/pages/wizard.page.ts#onEnd"] {
		t.Fatalf("a page object's onEnd is no longer admitted: %v", got)
	}
	// The map lists only parameterless methods, so summary is the reporter's one listed method: no
	// lifecycle name is visible, and only the import identifies the class.
	var rows []string
	for _, u := range cat.Unadmitted {
		if u.Reason == "page-object-is-reporter" {
			rows = append(rows, u.ID)
		}
	}
	if len(rows) != 1 || rows[0] != "method:e2e/pages/visual-parity.reporter.ts#summary" {
		t.Fatalf("want only the reporter's summary unadmitted as page-object-is-reporter, got %v", rows)
	}
}
