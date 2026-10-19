package testsource

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// briefAbstentions renders abstentions as "RULE reason test location cause".
func briefAbstentions(r ValidityReport) []string {
	out := []string{}
	for _, a := range r.Abstentions {
		out = append(out, strings.Join([]string{a.Rule, a.Reason, a.Test, a.Location, a.Cause}, " "))
	}
	return out
}

func wantAbstentions(t *testing.T, name string, r ValidityReport, want ...string) {
	t.Helper()
	if want == nil {
		want = []string{}
	}
	if got := briefAbstentions(r); !reflect.DeepEqual(got, want) {
		t.Errorf("%s: abstentions\n%q\nwant\n%q", name, got, want)
	}
}

// TestUnstubbedOutboundInputCrossFile: a file that imports a sibling stub module (its specifier or a
// bound name contains a stub word) withholds an uncovered input as a stub-table-cross-file
// abstention instead of a finding; a file with no stub of its own records one file-level abstention
// at the import; a package import or a sibling import naming no stub word changes nothing
// (TSA-V0-300..302).
func TestUnstubbedOutboundInputCrossFile(t *testing.T) {
	stubbed := `import { test } from '@playwright/test';
import { stubCrm } from './stubs';
test.beforeEach(async ({ page }) => {
  await page.route('**/api/lookup?name=Acme', (r) => r.fulfill({ json: {} }));
});
test('creates', async ({ page }) => {
  await createCustomer(page, { name: 'Globex' });
});
`
	r := ValidityFindings("e2e/customer.spec.ts", stubbed)
	wantFindings(t, "sibling with stubs", r)
	wantAbstentions(t, "sibling with stubs", r,
		"UNSTUBBED-OUTBOUND-INPUT stub-table-cross-file e2e/customer.spec.ts:6 e2e/customer.spec.ts:7 e2e/customer.spec.ts:2")
	if len(r.Abstentions) == 1 && !strings.Contains(r.Abstentions[0].Message, `withheld: this file imports "./stubs"`) {
		t.Errorf("message %q", r.Abstentions[0].Message)
	}
	if r.Uncertain || len(r.Findings) != 0 {
		t.Errorf("report %+v", r)
	}

	unread := `const { test } = require('../support/crm-mocks');
test('looks up', async ({ request }) => {
  await request.get('/api/lookup?name=Globex');
});
test('again', async ({ request }) => {
  await request.get('/api/lookup?name=Initech');
});
`
	r = ValidityFindings("e2e/lookup.spec.ts", unread)
	wantFindings(t, "no local stub", r)
	wantAbstentions(t, "no local stub", r,
		"UNSTUBBED-OUTBOUND-INPUT stub-table-cross-file e2e/lookup.spec.ts:1 e2e/lookup.spec.ts:1 e2e/lookup.spec.ts:1")
	if len(r.Abstentions) == 1 && !strings.Contains(r.Abstentions[0].Message, `"../support/crm-mocks"`) {
		t.Errorf("message %q", r.Abstentions[0].Message)
	}

	packageOnly := strings.Replace(stubbed, "import { stubCrm } from './stubs';\n", "import { stubCrm } from '@acme/stubs';\n", 1)
	r = ValidityFindings("e2e/customer.spec.ts", packageOnly)
	wantFindings(t, "package import", r,
		"UNSTUBBED-OUTBOUND-INPUT advisory e2e/customer.spec.ts:7 e2e/customer.spec.ts:4 creates")
	wantAbstentions(t, "package import", r)

	// A sibling import that names no stub word is not a stub module: the finding is reported, and a
	// file without stubs records nothing.
	fixtures := strings.Replace(stubbed, "import { stubCrm } from './stubs';\n", "import { test as t2 } from './fixtures';\n", 1)
	r = ValidityFindings("e2e/customer.spec.ts", fixtures)
	wantFindings(t, "fixtures import", r,
		"UNSTUBBED-OUTBOUND-INPUT advisory e2e/customer.spec.ts:7 e2e/customer.spec.ts:4 creates")
	wantAbstentions(t, "fixtures import", r)
	named := strings.Replace(stubbed, "import { stubCrm } from './stubs';\n", "import { mockCrm } from './helpers';\n", 1)
	if r := ValidityFindings("e2e/customer.spec.ts", named); len(r.Findings) != 0 || len(r.Abstentions) != 1 {
		t.Errorf("bound stub name: %+v", r)
	}
	wantAbstentions(t, "plain sibling", ValidityFindings("e2e/lookup.spec.ts", strings.Replace(unread, "crm-mocks", "crm", 1)))

	comma := strings.Replace(unread, "require('../support/crm-mocks')", "require('../support/crm-mocks',)", 1)
	if r := ValidityFindings("e2e/lookup.spec.ts", comma); len(r.Abstentions) != 1 {
		t.Errorf("trailing comma require: %+v", r.Abstentions)
	}
	names := []string{"mockCrm"}
	for i := 0; i < 40; i++ {
		names = append(names, fmt.Sprintf("helper%d", i))
	}
	long := strings.Replace(stubbed, "import { stubCrm } from './stubs';\n", "import {\n  "+strings.Join(names, ", ")+",\n} from './helpers';\n", 1)
	if r := ValidityFindings("e2e/customer.spec.ts", long); len(r.Findings) != 0 || len(r.Abstentions) != 1 {
		t.Errorf("long import clause: %+v", r)
	}

	noTests := "import { stubs } from './stubs';\nexport const x = 1;\n"
	wantAbstentions(t, "no tests", ValidityFindings("e2e/helpers.spec.ts", noTests))
}

// TestCleanupHandleOutlivedCrossFile: a cleanup closure capturing a test-scoped handle on a
// registry the file does not declare is a cleanup-registry-cross-file abstention whose cause is the
// import; a closure capturing no handle records none, and neither is a finding (TSA-V0-303,
// TSA-V0-304, TSA-V0-167).
func TestCleanupHandleOutlivedCrossFile(t *testing.T) {
	src := `import { expect } from '@playwright/test';
import { test } from './fixtures';
test('x', async ({ registry, request }) => {
  registry.add(() => console.log('done'));
  registry.add(() => request.delete('/x'));
  registry.add(() => request.delete('/y'));
});
`
	r := ValidityFindings("e2e/imported.spec.ts", src)
	wantFindings(t, "imported registry", r)
	wantAbstentions(t, "imported registry", r,
		"CLEANUP-HANDLE-OUTLIVED cleanup-registry-cross-file e2e/imported.spec.ts:3 e2e/imported.spec.ts:5 e2e/imported.spec.ts:2",
		"CLEANUP-HANDLE-OUTLIVED cleanup-registry-cross-file e2e/imported.spec.ts:3 e2e/imported.spec.ts:6 e2e/imported.spec.ts:2")
	if len(r.Abstentions) > 0 && !strings.Contains(r.Abstentions[0].Message, "`registry`") || len(r.Abstentions) > 0 && !strings.Contains(r.Abstentions[0].Message, "`request`") {
		t.Errorf("message %q", r.Abstentions[0].Message)
	}
	if r.Uncertain {
		t.Error("abstention made the report uncertain")
	}

	noCapture := `import { test } from './fixtures';
test('x', async ({ registry }) => {
  registry.add(() => console.log('done'));
});
`
	wantAbstentions(t, "no capture", ValidityFindings("e2e/nocapture.spec.ts", noCapture))

	// A registration nested in a call on an undeclared fixture is still read for the local registry.
	nested := `const test = base.extend({
  registry: [async ({}, use) => { await use(new Registry()); }, { scope: 'worker' }],
});
test('x', async ({ registry, request, external }) => {
  external.run(registry.add(() => request.delete('/x')));
});
`
	r = ValidityFindings("e2e/nested.spec.ts", nested)
	wantFindings(t, "nested registration", r,
		"CLEANUP-HANDLE-OUTLIVED advisory e2e/nested.spec.ts:5 e2e/nested.spec.ts:2 x")
	wantAbstentions(t, "nested registration", r)

	// A fixture file reads the rule alone, so its cross-file registration abstains at that line.
	fixtureFile := `import { test as base } from './base';
export const test = base.extend({
  api: async ({ registry, request }, use) => {
    registry.add(() => request.delete('/x'));
    await use(request);
  },
});
`
	r = ValidityFindings("e2e/fixtures.ts", fixtureFile)
	wantFindings(t, "fixture file", r)
	wantAbstentions(t, "fixture file", r,
		"CLEANUP-HANDLE-OUTLIVED cleanup-registry-cross-file e2e/fixtures.ts:4 e2e/fixtures.ts:4 e2e/fixtures.ts:1")
}

// TestVacuousEmptyEqualitySeeded: a declared fixture whose function seeds (directly or through a
// declared fixture it requests) and a beforeEach or beforeAll hook in scope that seeds suppress the
// finding (TSA-V0-305, TSA-V0-306).
func TestVacuousEmptyEqualitySeeded(t *testing.T) {
	src := `import { test as base, expect } from '@playwright/test';
const test = base.extend({
  todos: async ({ request }, use) => { await seedTodos(request, 3); await use(); },
  board: async ({ todos, page }, use) => { await use(page); },
  posted: async ({ request }, use) => { await request.post('/api/rows', { data: {} }); await use(); },
});
test('fixture seeds', async ({ todos, page }) => {
  await expect(page.locator('li')).toHaveCount(served.length);
});
test('transitive', async ({ board }) => {
  await expect(board.locator('li')).toHaveCount(served.length);
});
test('posted', async ({ posted, page }) => {
  expect(shown.length).toBe(served.length);
});
test.describe('hooked', () => {
  test.beforeEach(async ({ request }) => { await createUser(request); });
  test('hook seeds', async ({ page }) => {
    await expect(page.locator('tr')).toHaveCount(served.length);
  });
});
test.describe('other', () => {
  test('sibling describe', async ({ page }) => {
    await expect(page.locator('tr')).toHaveCount(served.length);
  });
});
`
	wantFindings(t, "seeded", ValidityFindings("e2e/seeded.spec.ts", src),
		"VACUOUS-EMPTY-EQUALITY advisory e2e/seeded.spec.ts:24 e2e/seeded.spec.ts:24 sibling describe")

	all := `test.beforeAll(async () => { await db.insert(rows); });
test('file hook seeds', async ({ page }) => {
  await expect(page.locator('tr')).toHaveCount(served.length);
});
`
	wantFindings(t, "file hook", ValidityFindings("e2e/all.spec.ts", all))
}

// TestVacuousEmptyEqualityNotSeeded: a declared fixture or hook without a seeding call, a page
// method whose name starts with a seeding word, a bare add( and a method declaration named like a
// seeding call do not suppress the finding; a
// Playwright option fixture is not a cross-file fixture (TSA-V0-305..307).
func TestVacuousEmptyEqualityNotSeeded(t *testing.T) {
	src := `const test = base.extend({
  board: async ({ page }, use) => { await page.addInitScript(() => {}); await use(page); },
  tags: async ({}, use) => { const s = new Set(); s.add(1); await use(s); },
});
test.beforeEach(async ({ page }) => { await page.goto('/'); });
test('init script', async ({ board, baseURL }) => {
  await expect(board.locator('li')).toHaveCount(served.length);
});
test('bare add', async ({ tags, page }) => {
  await expect(page.locator('li')).toHaveCount(served.length);
});
test.describe('declared only', () => {
  test.beforeEach(async () => { const factory = { seedTodos() {} }; });
  test('method declaration', async ({ page }) => {
    await expect(page.locator('li')).toHaveCount(served.length);
  });
});
`
	r := ValidityFindings("e2e/plain.spec.ts", src)
	wantFindings(t, "not seeded", r,
		"VACUOUS-EMPTY-EQUALITY advisory e2e/plain.spec.ts:7 e2e/plain.spec.ts:7 init script",
		"VACUOUS-EMPTY-EQUALITY advisory e2e/plain.spec.ts:10 e2e/plain.spec.ts:10 bare add",
		"VACUOUS-EMPTY-EQUALITY advisory e2e/plain.spec.ts:15 e2e/plain.spec.ts:15 method declaration")
	wantAbstentions(t, "not seeded", r)
}

// TestVacuousEmptyEqualityCrossFileFixture: a test requesting a fixture the file does not declare
// is withheld as a fixture-cross-file abstention whose cause is the import (TSA-V0-307).
func TestVacuousEmptyEqualityCrossFileFixture(t *testing.T) {
	src := `import { expect } from '@playwright/test';
import { test } from './fixtures';
test('imported fixture', async ({ seededTodos, page }) => {
  await expect(page.locator('li')).toHaveCount(served.length);
});
`
	r := ValidityFindings("e2e/todos.spec.ts", src)
	wantFindings(t, "cross-file fixture", r)
	wantAbstentions(t, "cross-file fixture", r,
		"VACUOUS-EMPTY-EQUALITY fixture-cross-file e2e/todos.spec.ts:3 e2e/todos.spec.ts:4 e2e/todos.spec.ts:2")
	if len(r.Abstentions) == 1 && !strings.Contains(r.Abstentions[0].Message, "`seededTodos`") {
		t.Errorf("message %q", r.Abstentions[0].Message)
	}
}

// TestEitherOrOutcomeJSONAnnotation: a sibling member `corvint-validity-allow` naming the key with
// `EITHER-OR-OUTCOME: <reason>` exempts that key in the same object; a missing reason, another
// rule, another key, another object, a nested object, a non-object value, another member name,
// data on the line above and invalid JSON exempt nothing, and the finding's message documents the
// form (TSA-V0-308..310).
func TestEitherOrOutcomeJSONAnnotation(t *testing.T) {
	positive := `{
  "rows": [
    {
      "id": "r1",
      "expected_status": "502 or 504",
      "corvint-validity-allow": {"expected_status": "EITHER-OR-OUTCOME: the gateway is non-deterministic"}
    }
  ]
}
`
	wantFindings(t, "positive", ValidityFindings("contracts/gw.json", positive))

	negatives := `{
  "rows": [
    {
      "expected_status": "401 or 403",
      "corvint-validity-allow": {"expected_status": "EITHER-OR-OUTCOME:"}
    },
    {
      "expected_status": "401 or 403",
      "corvint-validity-allow": {"expected_status": "MEMOIZED-BLOCK-EXPECTATION: x"}
    },
    {
      "expected_status": "401 or 403",
      "corvint-validity-allow": {"expected_code": "EITHER-OR-OUTCOME: x"}
    },
    {
      "expected_status": "401 or 403",
      "corvint-validity-allow": "EITHER-OR-OUTCOME: x"
    },
    {
      "expected_status": "401 or 403",
      "nested": {"corvint-validity-allow": {"expected_status": "EITHER-OR-OUTCOME: x"}}
    },
    {
      "corvint-validity-allow": {"expected_status": "EITHER-OR-OUTCOME: other object"}
    },
    {
      "expected_status": "401 or 403"
    },
    {
      "note": "corvint-validity-allow EITHER-OR-OUTCOME: data above",
      "expected_status": "401 or 403"
    },
    {
      "expected_status": "401 or 403",
      "allow": {"expected_status": "EITHER-OR-OUTCOME: another member name"}
    }
  ]
}
`
	r := ValidityFindings("contracts/admin.json", negatives)
	wantFindings(t, "negatives", r,
		"EITHER-OR-OUTCOME invalid contracts/admin.json:4 contracts/admin.json:4 expected_status",
		"EITHER-OR-OUTCOME invalid contracts/admin.json:8 contracts/admin.json:8 expected_status",
		"EITHER-OR-OUTCOME invalid contracts/admin.json:12 contracts/admin.json:12 expected_status",
		"EITHER-OR-OUTCOME invalid contracts/admin.json:16 contracts/admin.json:16 expected_status",
		"EITHER-OR-OUTCOME invalid contracts/admin.json:20 contracts/admin.json:20 expected_status",
		"EITHER-OR-OUTCOME invalid contracts/admin.json:27 contracts/admin.json:27 expected_status",
		"EITHER-OR-OUTCOME invalid contracts/admin.json:31 contracts/admin.json:31 expected_status",
		"EITHER-OR-OUTCOME invalid contracts/admin.json:34 contracts/admin.json:34 expected_status",
	)
	if len(r.Findings) > 0 && !strings.Contains(r.Findings[0].Message, "annotate instead with a sibling member `"+`"corvint-validity-allow": {"expected_status": "EITHER-OR-OUTCOME: <reason>"}`+"` in the same object") {
		t.Errorf("message %q", r.Findings[0].Message)
	}

	invalid := strings.Replace(positive, "\"id\": \"r1\",", "\"id\": \"r1\"", 1)
	wantFindings(t, "invalid json", ValidityFindings("contracts/gw.json", invalid),
		"EITHER-OR-OUTCOME invalid contracts/gw.json:5 contracts/gw.json:5 expected_status")
	wantFindings(t, "two roots", ValidityFindings("contracts/gw.json", positive+"{}\n"),
		"EITHER-OR-OUTCOME invalid contracts/gw.json:5 contracts/gw.json:5 expected_status")

	yml := "rows:\n  - expected_status: 401 or 403\n"
	if r := ValidityFindings("contracts/admin.yml", yml); len(r.Findings) != 1 || strings.Contains(r.Findings[0].Message, "sibling member") {
		t.Errorf("yaml message %+v", r.Findings)
	}
}

// TestSourceValidityAbstentionsWire: readSourceValidity carries the abstentions, which change no
// axis, and the JSON omits the field when there are none (TSA-V0-300).
func TestSourceValidityAbstentionsWire(t *testing.T) {
	src := `import { test } from './fixtures';
test('x', async ({ registry, request }) => {
  registry.add(() => request.delete('/x'));
});
`
	file, tests := readSourceValidity("e2e/x.spec.ts", src, testvalidity.RubySupport{})
	if len(file.Abstentions) != 1 || file.Abstentions[0].Reason != testvalidity.AbstainCleanupRegistryCrossFile {
		t.Fatalf("abstentions %+v", file.Abstentions)
	}
	if len(tests) != 1 || tests[0].Hygiene.State != testvalidity.HygieneEligible {
		t.Fatalf("tests %+v", tests)
	}
	body, err := json.Marshal(file)
	if err != nil || !strings.Contains(string(body), `"abstentions":[{"rule":"CLEANUP-HANDLE-OUTLIVED","reason":"cleanup-registry-cross-file"`) {
		t.Fatalf("json %s %v", body, err)
	}
	plain, _ := readSourceValidity("e2e/y.spec.ts", "test('y', async () => { expect(1).toBe(1); });\n", testvalidity.RubySupport{})
	if body, _ := json.Marshal(plain); strings.Contains(string(body), "abstentions") {
		t.Errorf("empty abstentions serialized: %s", body)
	}
}
