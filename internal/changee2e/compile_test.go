package changee2e

import (
	"bytes"
	"strings"
	"testing"
)

// CCE-V0-005, CCE-V0-007: one REACHED obligation compiles to one scenario built only from admitted
// block text, byte-identical on rerun, with the five-value header and its digest.
func TestCCEV0005CompileFromAdmittedBlocks(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	r, err := Compile(bg, diffOpts(root, base, head))
	if err != nil {
		t.Fatal(err)
	}
	again, err := Compile(bg, diffOpts(root, base, head))
	if err != nil {
		t.Fatal(err)
	}
	if r.State != Compiled || len(r.Compiled()) != 1 || !bytes.Equal(r.Encoded(), again.Encoded()) ||
		!bytes.Equal(r.Compiled()[0].Text, again.Compiled()[0].Text) {
		t.Fatalf("%s", r.Encoded())
	}
	sc := r.Compiled()[0]
	src := string(sc.Text)
	t.Logf("\n%s", src)
	if strings.Contains(src, "TODO") || !VerifyHeader(sc.Text) || sc.Placement != "e2e/corvint-compiled/book-tee-time-booked.spec.ts" {
		t.Fatalf("scenario:\n%s", src)
	}
	lines := strings.Split(src, "\n")
	for i, f := range headerFields {
		if !strings.HasPrefix(lines[1+i], "// "+f+": ") {
			t.Fatalf("header line %d: %q", 1+i, lines[1+i])
		}
	}
	for _, want := range []string{"// head: " + head, "// base: " + base, "page.goto('/#!/home')", "new ClubPage(page)", ".seedOpenSlots()",
		".goToTeeSheets()", ".selectSlot()", ".book()", ".cleanup()", "expect(page.getByTestId('slot-status')).toHaveText('Booked')"} {
		if !strings.Contains(src, want) {
			t.Fatalf("missing %q in:\n%s", want, src)
		}
	}
	if !strings.Contains(lines[sc.AssertionLine-1], "toHaveText('Booked')") || sc.AssertionID != "assert:book-tee-time/booked" {
		t.Fatalf("assertion line %d: %q", sc.AssertionLine, lines[sc.AssertionLine-1])
	}
	tampered := bytes.Replace(sc.Text, []byte("'Booked'"), []byte("'Bookd'"), 1)
	if VerifyHeader(tampered) {
		t.Fatal("tampered scenario verified")
	}
}

// CCE-V0-006: each missing block kind is named and the obligation is INCOMPLETE, with no
// selector, URL or value emitted.
func TestCCEV0006MissingBlocksIncomplete(t *testing.T) {
	root, base := fixtureRepo(t)
	commitBehaviourChange(t, root)
	head := commitFiles(t, root, map[string]*string{DeclarationsPath: nil}, "drop declarations")
	r, err := Compile(bg, diffOpts(root, base, head))
	if err != nil {
		t.Fatal(err)
	}
	if r.State != Incomplete || len(r.Compiled()) != 0 || len(r.Scenarios) != 1 {
		t.Fatalf("%s", r.Encoded())
	}
	e := r.Scenarios[0]
	if e.State != Incomplete || len(e.Missing) != 1 || e.Missing[0].Kind != KindFixture || e.Missing[0].ID != "club has open slots" || e.Placement != "" {
		t.Fatalf("%+v", e)
	}
	for _, leak := range []string{"/#!/", "slot-status", "Booked", "nav-teesheets"} {
		if bytes.Contains(r.Encoded(), []byte(leak)) {
			t.Fatalf("INCOMPLETE report leaks %q:\n%s", leak, r.Encoded())
		}
	}
	// An UNKNOWN obligations report never compiles to COMPILED.
	orphan := commitFiles(t, root, map[string]*string{"app/shared/orphan.html": text("<p/>\n")}, "orphan")
	r, err = Compile(bg, diffOpts(root, head, orphan))
	if err != nil {
		t.Fatal(err)
	}
	if r.State != Incomplete || len(r.Unknown) != 1 {
		t.Fatalf("%s", r.Encoded())
	}
}
