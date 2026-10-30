package testsource

import (
	"reflect"
	"testing"
)

// TestSourceModel proves the file classes read, the declaration model (describe nesting, test
// modifiers, runtime annotations) and that an unbalanced file is marked uncertain (TSA-V0-010).
func TestSourceModel(t *testing.T) {
	for name, want := range map[string][2]bool{
		"e2e/cart.spec.ts": {true, false}, "src/a.test.jsx": {true, false}, "e2e/pages/login.ts": {false, true},
		"e2e/Login.page.ts": {false, true}, "e2e/LoginPage.mjs": {false, true}, "src/app.ts": {false, false},
		"e2e/cart.spec.go": {false, false},
	} {
		if got := [2]bool{IsTestFile(name), IsPageObjectFile(name)}; got != want {
			t.Errorf("%s: test/page-object = %v, want %v", name, got, want)
		}
	}
	s := parseSource("e2e/a.spec.ts", `test.describe('cart', () => {
  describe.serial('totals', () => {
    test.only('adds', async () => { expect(1).toBe(2); });
  });
  test.skip(true, 'not on webkit');
});
it('top', () => {});`)
	got := []string{}
	for _, d := range s.decls {
		entry := d.kind + ":" + d.title
		if d.kind == "test" {
			entry += ":" + joinTitles(s.describes(d))
		}
		if d.modifiers["only"] || d.modifiers["skip"] || d.modifiers["serial"] {
			entry += ":mod"
		}
		got = append(got, entry)
	}
	want := []string{"describe:cart", "describe:totals:mod", "test:adds:cart>totals:mod", "annotation::mod", "test:top:"}
	if !reflect.DeepEqual(got, want) || s.uncertain {
		t.Fatalf("declarations = %v (uncertain %v), want %v", got, s.uncertain, want)
	}
	if !parseSource("e2e/b.spec.ts", "test('x', () => { expect(1).toBe(1);\n").uncertain {
		t.Fatal("an unbalanced file was not marked uncertain")
	}
}

func joinTitles(titles []string) string {
	out := ""
	for i, title := range titles {
		if i > 0 {
			out += ">"
		}
		out += title
	}
	return out
}
