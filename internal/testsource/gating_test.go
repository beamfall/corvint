package testsource

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDuplicatesGateCountsBeyondOutputBound proves the gate counts unresolved pairs before the
// output lists are bounded: resolving the first pairs never hides an unresolved one beyond the
// bound (TSA-V0-020, TSA-V0-021).
func TestDuplicatesGateCountsBeyondOutputBound(t *testing.T) {
	saved := maxPairs
	maxPairs = 1
	defer func() { maxPairs = saved }()
	root := t.TempDir()
	base := commitFiles(t, root, map[string]string{"README.md": "x\n"})
	commitFiles(t, root, map[string]string{
		"e2e/a.spec.ts": cartA,
		"e2e/b.spec.ts": strings.ReplaceAll(cartA, "'cart'", "'cart b'"),
		"e2e/c.spec.ts": strings.ReplaceAll(cartA, "'cart'", "'cart c'"),
		DecisionsPath: `{"schema":"corvint-test-duplicate-decisions/0","decisions":[
 {"decision":"distinct-from","test":"e2e/a.spec.ts :: cart > adds an item","other":"e2e/b.spec.ts :: cart b > adds an item","reason":"different carts"}]}`,
	})
	options := duplicateOptions()
	options.MaxLineTests = 3
	options.Base, options.Gate = base, "V1-1592"
	report, err := Duplicates(OpenRepo(context.Background(), root), options)
	if err != nil {
		t.Fatal(err)
	}
	if report.ResolvedCount != 1 || report.UnresolvedCount != 2 || len(report.Unresolved) != 1 || !report.Truncated {
		t.Fatalf("counts resolved=%d unresolved=%d listed=%+v truncated=%v", report.ResolvedCount, report.UnresolvedCount,
			report.Unresolved, report.Truncated)
	}
}

// TestDuplicatesGateSeesDeletionOnlyChange proves a change that only deletes lines inside a test,
// here a distinguishing assertion, makes that test changed for the gate (TSA-V0-021).
func TestDuplicatesGateSeesDeletionOnlyChange(t *testing.T) {
	root := t.TempDir()
	extra := strings.Replace(cartA, "    await expect(page).toHaveURL('/cart');\n",
		"    await expect(page).toHaveURL('/cart');\n    await expect(page.getByTestId('coupon')).toBeVisible();\n", 1)
	other := strings.ReplaceAll(cartA, "'cart'", "'cart b'")
	base := commitFiles(t, root, map[string]string{"e2e/a.spec.ts": extra, "e2e/b.spec.ts": other})
	commitFiles(t, root, map[string]string{"e2e/a.spec.ts": cartA})
	options := duplicateOptions()
	options.Threshold, options.MaxLineTests = 0.9, 3
	options.Base, options.Gate = base, "V1-1592"
	report, err := Duplicates(OpenRepo(context.Background(), root), options)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(report.ChangedTests, ",") != "e2e/a.spec.ts :: cart > adds an item" || report.UnresolvedCount != 1 {
		t.Fatalf("changed=%v unresolved=%+v", report.ChangedTests, report.Unresolved)
	}
}

// TestChangedPathsIgnoreAttributesAndDecodeNames proves an uncommitted attribute marking test files
// binary cannot hide their hunks, and that paths holding a space or a quote are read exactly
// (TSA-V0-010, TSA-V0-012).
func TestChangedPathsIgnoreAttributesAndDecodeNames(t *testing.T) {
	root := t.TempDir()
	base := commitFiles(t, root, map[string]string{"README.md": "x\n"})
	wait := "test('w', async ({ page }) => {\n  await page.waitForTimeout(5);\n  await expect(page.getByTestId('a')).toHaveText('b');\n});\n"
	commitFiles(t, root, map[string]string{"e2e/plain.spec.ts": wait, "e2e/with space.spec.ts": wait, "e2e/quo\"te.spec.ts": wait})
	want := "e2e/plain.spec.ts:fixed-wait,e2e/quo\"te.spec.ts:fixed-wait,e2e/with space.spec.ts:fixed-wait"
	lint := func(phase string) {
		t.Helper()
		report, err := Lint(OpenRepo(context.Background(), root), LintOptions{Base: base, Revision: "HEAD"})
		if err != nil {
			t.Fatal(err)
		}
		got := []string{}
		for _, f := range report.Findings {
			got = append(got, f.Path+":"+f.Rule)
		}
		if strings.Join(got, ",") != want {
			t.Fatalf("%s: findings = %v, want %s", phase, got, want)
		}
	}
	lint("names")
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.spec.ts -diff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lint("attributes")
}

// TestLintAllLinesReadsUnchangedFiles proves --all-lines lints every test file at the revision,
// not only the changed ones (TSA-V0-012).
func TestLintAllLinesReadsUnchangedFiles(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{"e2e/legacy.spec.ts": oldSpec})
	report, err := Lint(OpenRepo(context.Background(), root), LintOptions{Base: "HEAD", Revision: "HEAD", AllLines: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(report.Files, ",") != "e2e/legacy.spec.ts" || len(report.Findings) != 2 {
		t.Fatalf("files=%v findings=%+v", report.Files, report.Findings)
	}
}

// TestRenderKeepsRegexAndSubstitutions proves different regular expressions and template
// substitutions never normalise alike, so they neither pair as duplicates nor read as a value
// compared with itself (TSA-V0-013, TSA-V0-020).
func TestRenderKeepsRegexAndSubstitutions(t *testing.T) {
	s := parseSource("e2e/a.spec.ts", "expect(a).toMatch(/foo/i);\nexpect(a).toMatch(/bar/i);\nexpect(`${x}`).toBe(`${y}`);\n")
	first, _ := s.readExpect(0)
	second, _ := s.readExpect(first.end + 2)
	if a, b := s.render(0, first.end+1), s.render(first.end+2, second.end+1); a == b || a != "expect(a).toMatch(/foo/i)" {
		t.Fatalf("regex renders %q and %q", a, b)
	}
	report := LintText("e2e/a.spec.ts", "test('t', () => {\n  expect(`${actual}`).toBe(`${expected}`);\n  expect(`${same}`).toBe(`${same}`);\n});\n")
	got := []int{}
	for _, f := range report.Findings {
		if f.Rule == RuleTautological {
			got = append(got, f.Line)
		}
	}
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("tautological findings on lines %v, want [3]", got)
	}
}
