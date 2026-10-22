package changee2e

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func refusalCode(err error) string {
	var r *Refusal
	if errors.As(err, &r) {
		return r.Code
	}
	return ""
}

// CCE-V0-001, CCE-V0-002: the catalog is deterministic and every admitted block of the five
// catalogued kinds is bound to its blob and span; inferred or unknown sources are unadmitted.
func TestCCEV0001CatalogDeterministicAndBound(t *testing.T) {
	root, rev := fixtureRepo(t)
	opts := CatalogOptions{Root: root, Rev: rev, Manifest: "appmap.json"}
	a, err := BuildCatalog(bg, opts)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: "main", Manifest: "appmap.json"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Encoded(), b.Encoded()) || a.Revision != rev || a.Schema != CatalogSchema || a.Declarations != "DECLARED" {
		t.Fatalf("catalog not deterministic or not pinned:\n%s", a.Encoded())
	}
	kinds := map[string]int{}
	for _, blk := range a.Blocks {
		kinds[blk.Kind]++
		if blk.Blob == "" || blk.Span.Path == "" || blk.Span.Start < 1 || blk.Span.End < blk.Span.Start || len(blk.Span.SHA256) != 64 {
			t.Fatalf("block %s is not bound to a blob and span: %+v", blk.ID, blk)
		}
	}
	for _, k := range []string{KindPageObject, KindPath, KindOutcome, KindInput, KindFixture} {
		if kinds[k] == 0 {
			t.Fatalf("no admitted %s block: %v", k, kinds)
		}
	}
	for _, want := range []string{"method:e2e/pages/home.page.ts#goToTeeSheets", "path:entry:screen:admin:app.home", "outcome:book-tee-time/booked",
		"input:slot-note", "fixture:open-slots"} {
		if a.Block(want) == nil {
			t.Fatalf("missing admitted block %s", want)
		}
	}
	if u := a.Block("path:entry:screen:admin:app.home"); u.URL != "/#!/home" {
		t.Fatalf("entry URL %q", u.URL)
	}
	unadmitted := map[string]string{}
	for _, u := range a.Unadmitted {
		unadmitted[u.ID] = u.Reason
	}
	// A workflow helper is not a page object; a guarded or parameterised entry, a non-compilable
	// outcome and an inferred map edge are named, never admitted.
	if a.Block("method:e2e/workflows/booking.ts#bookFirstSlot") != nil {
		t.Fatal("workflow helper admitted")
	}
	if a.Block("path:entry:screen:admin:app.clubs.teesheets") != nil || unadmitted["path:entry:screen:admin:app.clubs.teesheets"] == "" {
		t.Fatalf("parameterised entry admitted: %v", unadmitted)
	}
	if a.Block("outcome:view-members/listed") != nil || unadmitted["outcome:view-members/listed"] != "outcome-not-compilable" {
		t.Fatalf("visibility outcome admitted: %v", unadmitted)
	}
	edge := false
	for _, u := range a.Unadmitted {
		edge = edge || (u.Kind == KindPath && u.Reason == "edge-unproven")
	}
	if !edge || len(a.Unknowns) == 0 {
		t.Fatalf("edges or map unknowns not carried: %+v %+v", a.Unadmitted, a.Unknowns)
	}
}

// CCE-V0-002: missing declarations are a named unknown; malformed or secret-shaped declarations
// refuse and admit nothing from the file.
func TestCCEV0002DeclarationsFailClosed(t *testing.T) {
	root, _ := fixtureRepo(t)
	rev := commitFiles(t, root, map[string]*string{DeclarationsPath: nil}, "drop declarations")
	c, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: rev, Manifest: "appmap.json"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Declarations != "NOT_DECLARED" || c.Block("fixture:open-slots") != nil {
		t.Fatalf("declarations %s", c.Declarations)
	}
	found := false
	for _, u := range c.Unknowns {
		found = found || u.Reason == "blocks-not-declared"
	}
	if !found {
		t.Fatalf("unknowns %+v", c.Unknowns)
	}
	secretInput := func(value string) string {
		return `{"schema":"e2e-block-declarations/0","inputs":[{"input_id":"input:token","screen_id":"screen:admin:app.home",` +
			`"locator":"token","values":[{"value_id":"v","value":"` + value + `","class":"valid"}]}]}`
	}
	// Values the shared secret screen recognizes refuse alongside the keyword screen.
	for _, c := range []struct{ code, body string }{
		{"e2e-blocks-invalid", `{"schema":"e2e-block-declarations/0","extra":true}`},
		// Prefixes are split so hosted push protection does not read the fixtures as live tokens.
		{"e2e-input-secret", secretInput("gh" + "p_abcdefghijklmnopqrstuvwxyz0123456789")},
		{"e2e-input-secret", secretInput("h" + "f_abcdefghijklmnopqrstuvwxyzABCD")},
		{"e2e-input-secret", secretInput("xo" + "xb-123456789012-1234567890123-abcdefghijklmnopqrstuvwx")},
	} {
		rev := commitFiles(t, root, map[string]*string{DeclarationsPath: text(c.body)}, c.body)
		if _, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: rev, Manifest: "appmap.json"}); refusalCode(err) != c.code {
			t.Fatalf("%s: got %v", c.body, err)
		}
	}
}

// CCE-V0-001: --out inside the repository refuses.
func TestCCEV0001OutInsideRootRefuses(t *testing.T) {
	root, _ := fixtureRepo(t)
	for _, out := range []string{root, root + "/.corvint/x.json", root + "/.git/x"} {
		if _, err := CheckOut(root, out); refusalCode(err) != "e2e-out-not-allowed" {
			t.Fatalf("%s: %v", out, err)
		}
	}
	if _, err := CheckOut(root, t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

// CCE-V0-024: an outcome with an empty expected value is unadmitted `outcome-vacuous`, so no
// scenario asserts a value any element text satisfies. Today the AMAP flow parser already refuses
// an empty value (the map is then unavailable), so the catalog check is the second line.
func TestCCEV0024OutcomeVacuous(t *testing.T) {
	for _, tc := range []struct{ matcher, locator, value, want string }{
		{"toHaveText", "slot-status", "Booked", ""},
		{"toHaveText", "slot-status", "", "outcome-vacuous"},
		{"toHaveText", "", "Booked", "outcome-not-compilable"},
		{"toBeVisible", "slot-status", "", "outcome-not-compilable"},
	} {
		if got := outcomeAdmissionReason(tc.matcher, tc.locator, tc.value); got != tc.want {
			t.Fatalf("%s/%s/%q: %q, want %q", tc.matcher, tc.locator, tc.value, got, tc.want)
		}
	}
	root, _ := fixtureRepo(t)
	data, err := os.ReadFile(filepath.Join(root, "flows", "book-tee-time.json"))
	if err != nil {
		t.Fatal(err)
	}
	vacuous := strings.Replace(string(data), `"value": "Booked"`, `"value": ""`, 1)
	rev := commitFiles(t, root, map[string]*string{"flows/book-tee-time.json": text(vacuous)}, "vacuous outcome")
	if _, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: rev, Manifest: "appmap.json"}); refusalCode(err) != "e2e-map-unavailable" {
		t.Fatalf("an empty outcome value must not reach a catalog: %v", err)
	}
}
