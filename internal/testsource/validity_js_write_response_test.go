package testsource

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// writeFindings returns the write-response findings of one JS/TS file as "rule location cause".
func writeFindings(t *testing.T, name, text string) ([]string, []testvalidity.SourceFinding) {
	t.Helper()
	examples, reason := JSExampleSources(name, text)
	if reason != "" {
		t.Fatalf("reader abstained: %s", reason)
	}
	rows, all := []string{}, []testvalidity.SourceFinding{}
	for _, line := range exampleLines(examples) {
		for _, f := range examples[line].Findings {
			if f.Rule == testvalidity.RuleWriteResponseUnasserted || f.Rule == testvalidity.RuleWriteResponseUnresolved {
				rows = append(rows, f.Rule+" "+f.Location+" "+f.Cause)
				all = append(all, f)
			}
		}
	}
	return rows, all
}

const writeResponseSpec = `import { test, expect } from '@playwright/test';

test('creates a record, asserting only the dialog', async ({ page }) => {
  await page.getByRole('button', { name: 'Save' }).click();
  await page.waitForResponse(r => r.url().includes('/api/records') && r.request().method() === 'POST');
  await expect(page.getByRole('dialog')).toBeHidden();
});

test('creates a record, asserting the response', async ({ page }) => {
  const created = page.waitForResponse(r => r.url().includes('/api/records') && r.request().method() === 'POST');
  await page.getByRole('button', { name: 'Save' }).click();
  const response = await created;
  expect(response.status()).toBe(201);
  await expect(page.getByRole('dialog')).toBeHidden();
});

test('loads the list', async ({ page }) => {
  await page.getByRole('link', { name: 'Records' }).click();
  await page.waitForResponse(r => r.url().includes('/api/records') && r.request().method() === 'GET');
  await expect(page.getByRole('table')).toBeVisible();
});

test('waits on a bare route', async ({ page }) => {
  await page.getByRole('button', { name: 'Save' }).click();
  await page.waitForResponse('**/api/records');
  await expect(page.getByRole('dialog')).toBeHidden();
});
`

// TestWriteResponseUnassertedAcceptance: the four V1-1933 acceptance criteria on one file. A
// click-submit step whose awaited POST has no response assertion is reported and cites the in-file
// pattern; the same step with the wait started before the click and a status assertion after it
// is not; a GET is not; a wait with no resolvable method abstains (TSA-V0-230..233).
func TestWriteResponseUnassertedAcceptance(t *testing.T) {
	name := "e2e/records.spec.ts"
	rows, all := writeFindings(t, name, writeResponseSpec)
	want := []string{
		"WRITE-RESPONSE-UNASSERTED e2e/records.spec.ts:4 e2e/records.spec.ts:5",
		"WRITE-RESPONSE-UNRESOLVED e2e/records.spec.ts:24 e2e/records.spec.ts:25",
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("findings\n got %q\nwant %q", rows, want)
	}
	if !strings.Contains(all[0].Message, "POST") || !strings.Contains(all[0].Message, "as e2e/records.spec.ts:10 does") {
		t.Errorf("message does not cite the in-file pattern: %s", all[0].Message)
	}
	if all[0].Severity != testvalidity.SeverityAdvisory || all[1].Severity != testvalidity.SeverityAdvisory {
		t.Errorf("severities %q %q", all[0].Severity, all[1].Severity)
	}
	if !strings.HasPrefix(all[1].Message, "abstained:") {
		t.Errorf("unresolved message %q", all[1].Message)
	}
}

// TestWriteResponseNoExemplar: with no correct pattern in the file the finding names none
// (TSA-V0-232).
func TestWriteResponseNoExemplar(t *testing.T) {
	text := strings.SplitN(writeResponseSpec, "test('creates a record, asserting the response'", 2)[0]
	_, all := writeFindings(t, "e2e/records.spec.ts", text)
	if len(all) != 1 || strings.Contains(all[0].Message, " does") {
		t.Fatalf("findings %+v", all)
	}
}

const writeResponseForms = `import { test, expect } from '@playwright/test';

test('promise all with a status assertion', async ({ page }) => {
  const [resp] = await Promise.all([
    page.waitForResponse(r => r.request().method() === 'PUT'),
    page.getByRole('button', { name: 'Update' }).click(),
  ]);
  expect(resp.ok()).toBeTruthy();
});

test('body assertion through a local', async ({ page }) => {
  const p = page.waitForResponse(r => r.request().method() === 'DELETE' && r.url().endsWith('/api/records/1'));
  await page.keyboard.press('Delete');
  const body = await (await p).json();
  expect(body.deleted).toBe(true);
});

test('then assertion', async ({ page }) => {
  const done = page.waitForResponse(r => r.request().method() === 'PATCH').then(r => expect(r.status()).toBe(200));
  await page.getByText('Rename').click();
  await done;
});

test('steps are separate', async ({ page }) => {
  const p = page.waitForResponse(r => r.request().method() === 'POST');
  await test.step('submit', async () => {
    await page.getByRole('button', { name: 'Save' }).click();
    await page.waitForRequest(q => q.method() === 'POST');
  });
  expect((await p).status()).toBe(201);
});

test('absolute url abstains', async ({ page }) => {
  await page.getByRole('button').click();
  await page.waitForResponse(r => r.url() === 'https://other.example/api' && r.request().method() === 'POST');
});

test('negated method abstains', async ({ page }) => {
  await page.getByRole('button').click();
  await page.waitForResponse(r => r.request().method() !== 'GET');
});

test('no action, no finding', async ({ page }) => {
  await page.goto('/records');
  await page.waitForResponse(r => r.request().method() === 'POST');
});

test('no wait, no finding', async ({ page }) => {
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByRole('dialog')).toBeHidden();
});
`

// TestWriteResponseForms: Promise.all destructuring, a local alias of the body, a then callback
// and a status assertion bind the response; a step body is read apart from the example around
// it; an absolute URL and a negated method abstain; a step with no action or no wait is silent
// (TSA-V0-230, TSA-V0-231, TSA-V0-233).
func TestWriteResponseForms(t *testing.T) {
	rows, all := writeFindings(t, "e2e/forms.spec.ts", writeResponseForms)
	want := []string{
		"WRITE-RESPONSE-UNASSERTED e2e/forms.spec.ts:27 e2e/forms.spec.ts:28",
		"WRITE-RESPONSE-UNRESOLVED e2e/forms.spec.ts:34 e2e/forms.spec.ts:35",
		"WRITE-RESPONSE-UNRESOLVED e2e/forms.spec.ts:39 e2e/forms.spec.ts:40",
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("findings\n got %q\nwant %q", rows, want)
	}
	if !strings.Contains(all[0].Message, "as e2e/forms.spec.ts:5 does") {
		t.Errorf("exemplar %s", all[0].Message)
	}
}

// TestWriteResponseKeepsHygiene: the rule is advisory, so the reported example stays ELIGIBLE in
// the CLI report (TSA-V0-234).
func TestWriteResponseKeepsHygiene(t *testing.T) {
	file, tests := readSourceValidity("e2e/records.spec.ts", writeResponseSpec, testvalidity.RubySupport{})
	if file.Uncertain {
		t.Fatalf("uncertain %s", file.Reason)
	}
	found := false
	for _, f := range file.Findings {
		found = found || f.Rule == testvalidity.RuleWriteResponseUnasserted
	}
	if !found {
		t.Fatalf("finding not in the file report")
	}
	for _, test := range tests {
		if test.Test == "e2e/records.spec.ts:3" && test.Hygiene.State != testvalidity.HygieneEligible {
			t.Errorf("hygiene %+v", test.Hygiene)
		}
	}
}

// TestWriteResponseLateWaitIsNotExemplar: an asserted wait started after its action is not
// reported, but it is not the cited pattern either, since the response can arrive before the wait
// starts (TSA-V0-232).
func TestWriteResponseLateWaitIsNotExemplar(t *testing.T) {
	text := `import { test, expect } from '@playwright/test';

test('late but asserted', async ({ page }) => {
  await page.getByRole('button', { name: 'Save' }).click();
  const response = await page.waitForResponse(r => r.request().method() === 'POST');
  expect(response.status()).toBe(201);
});

test('unasserted', async ({ page }) => {
  await page.getByRole('button', { name: 'Save' }).click();
  await page.waitForResponse(r => r.request().method() === 'POST');
});
`
	rows, all := writeFindings(t, "e2e/late.spec.ts", text)
	if !reflect.DeepEqual(rows, []string{"WRITE-RESPONSE-UNASSERTED e2e/late.spec.ts:10 e2e/late.spec.ts:11"}) || strings.Contains(all[0].Message, " does") {
		t.Fatalf("findings %q %+v", rows, all)
	}
}

// TestWriteResponseReviewForms: any negation abstains, and a then callback that never reads its
// response is not an assertion of it (TSA-V0-231, TSA-V0-233).
func TestWriteResponseReviewForms(t *testing.T) {
	text := `import { test, expect } from '@playwright/test';

test('wrapped negation', async ({ page }) => {
  await page.getByRole('button').click();
  await page.waitForResponse(r => !(r.request().method() === 'POST'));
});

test('then ignores the response', async ({ page }) => {
  const done = page.waitForResponse(r => r.request().method() === 'POST').then(r => expect(page.getByRole('dialog')).toBeHidden());
  await page.getByRole('button', { name: 'Save' }).click();
  await done;
});
`
	rows, _ := writeFindings(t, "e2e/review.spec.ts", text)
	want := []string{
		"WRITE-RESPONSE-UNRESOLVED e2e/review.spec.ts:4 e2e/review.spec.ts:5",
		"WRITE-RESPONSE-UNASSERTED e2e/review.spec.ts:10 e2e/review.spec.ts:9",
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("findings\n got %q\nwant %q", rows, want)
	}
}
