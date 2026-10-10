package obligation

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// TestTOLV0025_ScannerSkipsCommentsRegexAndInterpolation: the heuristic
// scanner ignores test calls inside comments, regular expression literals
// and template interpolations, keeps line numbers across multi-line
// literals, and recognizes describe-level test.fail and extended `*Test`
// fixtures.
func TestTOLV0025_ScannerSkipsCommentsRegexAndInterpolation(t *testing.T) {
	src := "/* test('AC-1 in a block comment', () => {}) */\n" +
		"const re = /test\\('AC-2'/g;\n" +
		"const s = `multi\nline ${test('AC-3 interpolated', () => {})}`;\n" +
		"adminTest.describe('admin', () => {\n" +
		"  adminTest.fail(true, 'known broken');\n" +
		"  adminTest('AC-4 audit', async () => {});\n" +
		"});\n" +
		"test('AC-5 a / b', async () => { const x = 1 / 2; });\n"
	l := &ticket.ObligationLedger{Prefix: "AC"}
	for _, id := range []string{"AC-1", "AC-2", "AC-3", "AC-4", "AC-5"} {
		l.Entries = append(l.Entries, ticket.ObligationEntry{ID: id, State: ticket.ObligationOpen})
	}
	var got []string
	for _, f := range CheckSources(l, map[string][]byte{"a.spec.ts": []byte(src)}) {
		got = append(got, f.Kind+":"+f.ID+":"+itoa(f.Line))
	}
	want := "MIXED_EXPECTED_FAIL:AC-4:7,UNNAMED:AC-1:0,UNNAMED:AC-2:0,UNNAMED:AC-3:0"
	if strings.Join(got, ",") != want {
		t.Fatalf("got %s\nwant %s", strings.Join(got, ","), want)
	}
	if !SpecFile("e2e/a.spec.ts") || !SpecFile("x.test.mjs") || SpecFile("e2e/helpers.ts") || SpecFile("a.spec.py") {
		t.Fatal("spec file pattern")
	}
}

// TestTOLV0025_DescribeTitleIDsReachNestedTests: an id in a describe title
// is named by every test inside it, as runtime witnessing reads the title
// path: a nested test.fail test makes it MIXED_EXPECTED_FAIL unless that
// title marks it expected-fail, and it counts toward SPLIT_TESTS.
func TestTOLV0025_DescribeTitleIDsReachNestedTests(t *testing.T) {
	src := "test.describe('AC-1 ordinary obligation', () => {\n" +
		"  test.fail('known failure', async () => {});\n" +
		"});\n" +
		"test.describe('AC-2 expected-fail group', () => {\n" +
		"  test.fail('known failure', async () => {});\n" +
		"});\n" +
		"test.describe('AC-3 group', () => {\n" +
		"  test('AC-4 first', async () => {});\n" +
		"  test('AC-5 second', async () => {});\n" +
		"});\n"
	l := &ticket.ObligationLedger{Prefix: "AC"}
	for _, id := range []string{"AC-1", "AC-2", "AC-3", "AC-4", "AC-5"} {
		l.Entries = append(l.Entries, ticket.ObligationEntry{ID: id, State: ticket.ObligationOpen})
	}
	var got []string
	for _, f := range CheckSources(l, map[string][]byte{"a.spec.ts": []byte(src)}) {
		got = append(got, f.Kind+":"+f.ID+":"+itoa(f.Line))
	}
	if want := "MIXED_EXPECTED_FAIL:AC-1:2"; strings.Join(got, ",") != want {
		t.Fatalf("got %s\nwant %s", strings.Join(got, ","), want)
	}
}
