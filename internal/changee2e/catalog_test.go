package changee2e

import (
	"bytes"
	"errors"
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
	for code, body := range map[string]string{
		"e2e-blocks-invalid": `{"schema":"e2e-block-declarations/0","extra":true}`,
		"e2e-input-secret": `{"schema":"e2e-block-declarations/0","inputs":[{"input_id":"input:token","screen_id":"screen:admin:app.home",` +
			`"locator":"token","values":[{"value_id":"v","value":"ghp_abcdefghijklmnopqrstuvwxyz0123456789","class":"valid"}]}]}`,
	} {
		rev := commitFiles(t, root, map[string]*string{DeclarationsPath: text(body)}, code)
		if _, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: rev, Manifest: "appmap.json"}); refusalCode(err) != code {
			t.Fatalf("%s: got %v", code, err)
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
