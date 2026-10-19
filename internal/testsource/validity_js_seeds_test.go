package testsource

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

const seedRulesJSON = `{"profile":"corvint-seed-rules/0","resources":[
 {"name":"booking","calls":["seedBooking"],"paths":["/api/bookings"],"tenant":"tenant","range":{"start":"start","end":"end","bounds":"inclusive"}},
 {"name":"shift","calls":["seedShift"],"tenant":"tenant","date":"date","weekly":"weekly","until":"until"},
 {"name":"user","calls":["seedUser"],"tenant":"tenant","unique":[["email"]]}
]}`

func seedRulesFixture(t *testing.T) SeedRules {
	t.Helper()
	rules, err := DecodeSeedRules([]byte(seedRulesJSON))
	if err != nil {
		t.Fatal(err)
	}
	return rules
}

// seedRows returns the seed findings as "rule test location cause".
func seedRows(findings []testvalidity.SourceFinding) []string {
	rows := []string{}
	for _, f := range findings {
		rows = append(rows, f.Rule+" "+f.Test+" "+f.Location+" "+f.Cause)
	}
	return rows
}

const seedSpec = `import { test, expect } from '@playwright/test';
import { seedBooking, seedShift } from './seeds';

test.describe('bookings', () => {
  test.beforeAll(async ({ request }) => {
    await seedBooking(request, { tenant: 't1', start: 1, end: 5 });
    await seedBooking(request, { tenant: 't1', start: 5, end: 9 });
  });

  test('lists bookings', async ({ page }) => {
    await expect(page.getByRole('table')).toBeVisible();
  });

  test('shows one booking', async ({ page }) => {
    await expect(page.getByRole('row')).toHaveCount(2);
  });
});

test('weekly shift', async ({ request }) => {
  await seedShift(request, { tenant: 't1', date: '2026-10-05', weekly: true, until: '2026-12-31' });
});

test('one-off shift', async ({ request }) => {
  await seedShift(request, { tenant: 't1', date: '2026-10-19' });
});

test('other tenant shift', async ({ request }) => {
  await seedShift(request, { tenant: 't2', date: '2026-10-19' });
});

test('disjoint bookings', async ({ request }) => {
  await seedBooking(request, { tenant: 't3', start: 10, end: 12 });
  await seedBooking(request, { tenant: 't3', start: 13, end: 15 });
});

test('unresolved booking', async ({ request }) => {
  await seedBooking(request, { tenant: 't4', start: Date.now(), end: 5 });
});
`

// TestSeedAcceptance: the five V1-1935 acceptance criteria on one file. Seeds [1,5] and [5,9]
// under an inclusive overlap rule are reported; a date equal to a sibling's weekly recurrence in
// the same tenant is reported, the same date in another tenant is not; disjoint seeds are silent;
// the seeding hook lists the tests its failure would skip; an unresolved field abstains
// (TSA-V0-241..245).
func TestSeedAcceptance(t *testing.T) {
	name := "e2e/bookings.spec.ts"
	findings := SeedFindings(name, seedSpec, seedRulesFixture(t))
	want := []string{
		"SEED-VALIDATION-UNRESOLVED e2e/bookings.spec.ts:36 e2e/bookings.spec.ts:37 e2e/bookings.spec.ts:37",
		"SEED-VALIDATION-REJECTED e2e/bookings.spec.ts:5 e2e/bookings.spec.ts:7 e2e/bookings.spec.ts:6",
		"SEED-SIBLING-COLLISION e2e/bookings.spec.ts:23 e2e/bookings.spec.ts:24 e2e/bookings.spec.ts:20",
		"SEED-HOOK-BLAST-RADIUS e2e/bookings.spec.ts:5 e2e/bookings.spec.ts:5 e2e/bookings.spec.ts:10",
	}
	if got := seedRows(findings); !reflect.DeepEqual(got, want) {
		t.Fatalf("findings\n got %q\nwant %q", got, want)
	}
	for _, f := range findings {
		if f.Severity != testvalidity.SeverityAdvisory {
			t.Errorf("%s severity %s", f.Rule, f.Severity)
		}
	}
	if !strings.HasPrefix(findings[0].Message, "abstained:") {
		t.Errorf("unresolved message %q", findings[0].Message)
	}
	if !strings.Contains(findings[1].Message, "at a shared bound under inclusive bounds") {
		t.Errorf("overlap message %q", findings[1].Message)
	}
	if !strings.Contains(findings[2].Message, "2026-10-19") {
		t.Errorf("recurrence message %q", findings[2].Message)
	}
	if !strings.Contains(findings[3].Message, "skips 2 test(s): e2e/bookings.spec.ts:10, e2e/bookings.spec.ts:14") {
		t.Errorf("blast radius message %q", findings[3].Message)
	}
}

// TestSeedExclusiveBounds: touching ranges do not overlap under exclusive bounds, and an empty or
// reversed range is rejected on its own (TSA-V0-241, TSA-V0-242).
func TestSeedExclusiveBounds(t *testing.T) {
	rules := seedRulesFixture(t)
	rules.Resources[0].Range.Bounds = "exclusive"
	findings := SeedFindings("e2e/bookings.spec.ts", seedSpec, rules)
	for _, f := range findings {
		if f.Rule == testvalidity.RuleSeedValidationRejected {
			t.Fatalf("touching exclusive ranges reported: %+v", f)
		}
	}
	text := `import { test } from '@playwright/test';
test('reversed', async ({ request }) => {
  await seedBooking(request, { tenant: 't1', start: 9, end: 2 });
  await request.post('/api/bookings?x=1', { data: { tenant: 't1', start: '2026-10-01', end: '2026-10-01' } });
});
`
	got := seedRows(SeedFindings("e2e/reversed.spec.ts", text, rules))
	want := []string{
		"SEED-VALIDATION-REJECTED e2e/reversed.spec.ts:2 e2e/reversed.spec.ts:3 e2e/reversed.spec.ts:3",
		"SEED-VALIDATION-REJECTED e2e/reversed.spec.ts:2 e2e/reversed.spec.ts:4 e2e/reversed.spec.ts:4",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings\n got %q\nwant %q", got, want)
	}
}

// TestSeedUniqueAndForms: a unique key seeded twice by sibling tests collides; a different key, a
// non-literal record and a hook with no seeds produce nothing beyond the abstention
// (TSA-V0-243, TSA-V0-244, TSA-V0-245).
func TestSeedUniqueAndForms(t *testing.T) {
	text := `import { test } from '@playwright/test';
test.beforeEach(async ({ page }) => { await page.goto('/'); });
test('first user', async ({ request }) => {
  await seedUser(request, { tenant: 't1', email: 'a@example.test' });
});
test('second user', async ({ request }) => {
  await seedUser(request, { tenant: 't1', email: 'a@example.test' });
});
test('third user', async ({ request }) => {
  await seedUser(request, { tenant: 't1', email: 'b@example.test' });
});
test('spread user', async ({ request }) => {
  await seedUser(request, record);
});
`
	got := seedRows(SeedFindings("e2e/users.spec.ts", text, seedRulesFixture(t)))
	want := []string{
		"SEED-VALIDATION-UNRESOLVED e2e/users.spec.ts:12 e2e/users.spec.ts:13 e2e/users.spec.ts:13",
		"SEED-SIBLING-COLLISION e2e/users.spec.ts:6 e2e/users.spec.ts:7 e2e/users.spec.ts:4",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings\n got %q\nwant %q", got, want)
	}
}

// TestDecodeSeedRules: the rule file is a closed profile; a malformed one is refused.
func TestDecodeSeedRules(t *testing.T) {
	for _, bad := range []string{
		`{"profile":"other","resources":[{"name":"a","calls":["x"],"unique":[["k"]]}]}`,
		`{"profile":"corvint-seed-rules/0","resources":[]}`,
		`{"profile":"corvint-seed-rules/0","resources":[{"name":"a","unique":[["k"]]}]}`,
		`{"profile":"corvint-seed-rules/0","resources":[{"name":"a","calls":["x"]}]}`,
		`{"profile":"corvint-seed-rules/0","resources":[{"name":"a","calls":["x"],"range":{"start":"s","end":"e","bounds":"open"}}]}`,
		`{"profile":"corvint-seed-rules/0","resources":[{"name":"a","calls":["x"],"weekly":"w","unique":[["k"]]}]}`,
		`{"profile":"corvint-seed-rules/0","resources":[{"name":"a","calls":["x"],"unique":[[""]]}]}`,
		`{"profile":"corvint-seed-rules/0","resources":[{"name":"a","calls":["x"],"unique":[["k"]],"extra":1}]}`,
		`{"profile":"corvint-seed-rules/0","resources":[{"name":"a","calls":["x"],"unique":[["k"]]}]} {}`,
		`{"profile":"corvint-seed-rules/0","resources":[{"name":"a","calls":["x"],"unique":[["k"]]}]} ]`,
	} {
		if _, err := DecodeSeedRules([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

// TestSourceValiditySeedRules: `tests validity` reads the committed rule file at the revision and
// adds the advisory seed findings to the file report without changing hygiene; an invalid rule
// file abstains once per file; no rule file means no seed findings (TSA-V0-240, TSA-V0-246).
func TestSourceValiditySeedRules(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{"e2e/bookings.spec.ts": seedSpec})
	repo := OpenRepo(context.Background(), root)
	rules := func() []string {
		report, err := SourceValidity(repo, SourceValidityOptions{Files: []string{"e2e/bookings.spec.ts"}})
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range report.Tests {
			if test.Hygiene.State != testvalidity.HygieneEligible {
				t.Errorf("hygiene %s %+v", test.Test, test.Hygiene)
			}
		}
		out := []string{}
		for _, f := range report.Files[0].Findings {
			if strings.HasPrefix(f.Rule, "SEED-") {
				out = append(out, f.Rule)
			}
		}
		return out
	}
	if got := rules(); len(got) != 0 {
		t.Fatalf("no rule file, got %q", got)
	}
	commitFiles(t, root, map[string]string{SeedRulesPath: seedRulesJSON})
	if got := rules(); len(got) != 4 {
		t.Fatalf("rule file, got %q", got)
	}
	commitFiles(t, root, map[string]string{SeedRulesPath: `{"profile":"corvint-seed-rules/0"}`})
	if got := rules(); !reflect.DeepEqual(got, []string{testvalidity.RuleSeedValidationUnresolved}) {
		t.Fatalf("invalid rule file, got %q", got)
	}
}

// TestSeedReviewForms: a later key replaces an earlier one; a spread record abstains; a boolean
// and a string are different values; two seeds of one test are not siblings; RFC 3339 timestamps
// compare as instants, and a date against a timestamp abstains (TSA-V0-242..244).
func TestSeedReviewForms(t *testing.T) {
	text := `import { test } from '@playwright/test';
test('later key wins', async ({ request }) => {
  await seedUser(request, { tenant: 't1', email: 'a@example.test', email: 'b@example.test' });
  await seedUser(request, { tenant: 't1', email: 'b@example.test' });
});
test('spread record', async ({ request }) => {
  await seedUser(request, { ...base, tenant: 't2', email: 'c@example.test' });
});
test('typed tenant', async ({ request }) => {
  await seedUser(request, { tenant: true, email: 'd@example.test' });
  await seedUser(request, { tenant: 'true', email: 'd@example.test' });
});
test('one owner', async ({ request }) => {
  await seedShift(request, { tenant: 't3', date: '2026-10-19' });
  await seedShift(request, { tenant: 't3', date: '2026-10-19' });
});
test('offsets', async ({ request }) => {
  await seedBooking(request, { tenant: 't4', start: '2026-10-19T10:00:00+02:00', end: '2026-10-19T09:00:00Z' });
  await seedBooking(request, { tenant: 't5', start: '2026-10-19T10:00:00Z', end: '2026-10-19T09:00:00+02:00' });
  await seedBooking(request, { tenant: 't6', start: '2026-10-20', end: '2026-10-19T09:00:00Z' });
});
`
	got := seedRows(SeedFindings("e2e/review.spec.ts", text, seedRulesFixture(t)))
	want := []string{
		"SEED-VALIDATION-UNRESOLVED e2e/review.spec.ts:6 e2e/review.spec.ts:7 e2e/review.spec.ts:7",
		"SEED-VALIDATION-REJECTED e2e/review.spec.ts:17 e2e/review.spec.ts:19 e2e/review.spec.ts:19",
		"SEED-VALIDATION-REJECTED e2e/review.spec.ts:2 e2e/review.spec.ts:4 e2e/review.spec.ts:3",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings\n got %q\nwant %q", got, want)
	}
}
