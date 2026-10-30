package appmap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AMAP-V0-359: a page_object_screens entry that names a state present in the map whose screen is
// UNKNOWN reads unresolved-screen with that screen's own blocking reason, and its remedy leaves the
// entry's state alone; unknown-state is kept for a state the map does not hold.
func TestAMAPV0359UnresolvedScreenIsNotUnknownState(t *testing.T) {
	root, _ := fixtureRepo(t)
	raw, err := os.ReadFile(filepath.Join(root, "appmap.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := string(raw)
	declared := `"page_object_screens": [{"path": "e2e/pages/teesheet.page.ts", "state": "app.dynamic"}, {"path": "e2e/pages/home.page.ts", "state": "app.absent"}]`
	old := `"page_object_screens": [{"path": "e2e/pages/teesheet.page.ts", "state": "app.clubs.teesheets"}]`
	if !strings.Contains(manifest, old) {
		t.Fatal("fixture manifest changed shape")
	}
	writeFile(t, root, "appmap.json", strings.Replace(manifest, old, declared, 1))
	m := build(t, root, commitAll(t, root, "declare"))

	byPath := map[string]Unknown{}
	for _, u := range m.Unknowns {
		if u.Kind == "page-object" {
			byPath[u.Path] = u
		}
	}
	dyn := byPath["e2e/pages/teesheet.page.ts"]
	if dyn.Reason != "unresolved-screen" || dyn.Detail != screenID(m.App, "app.dynamic")+" UNKNOWN: non-literal-url" {
		t.Fatalf("present-but-unresolved state: %+v", dyn)
	}
	absent := byPath["e2e/pages/home.page.ts"]
	if absent.Reason != "unknown-state" {
		t.Fatalf("absent state: %+v", absent)
	}
	r := UnknownRemedy(m, dyn)
	if r == nil || r.Key != "" || !strings.Contains(r.Uncertainty, "non-literal-url") || !strings.Contains(r.Uncertainty, "needs no change") ||
		strings.Contains(r.Uncertainty, "correct the entry") {
		t.Fatalf("unresolved-screen remedy: %+v", r)
	}
	r = UnknownRemedy(m, absent)
	if r == nil || r.Key != RemedyKeyPageObjectScreens || !strings.Contains(r.Uncertainty, "names no state in the map; correct the entry's state") {
		t.Fatalf("unknown-state remedy: %+v", r)
	}
}
