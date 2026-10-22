package changee2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// moduleGraphFixture commits, at base, a helper module the router anchor imports next to a comment
// that names it, so impact's reference hop cannot prove the reference, then changes the helper.
func moduleGraphFixture(t *testing.T) (root, base, head string) {
	t.Helper()
	root, _ = fixtureRepo(t)
	routes := *overlayText(t, "app/routes.js")
	base = commitFiles(t, root, map[string]*string{
		"app/lib/format.js": text("export function formatDate(d) { return String(d); }\n"),
		"app/routes.js":     text("import { formatDate } from './lib/format.js'; // format helpers\n" + routes),
	}, "helper module")
	head = commitFiles(t, root, map[string]*string{"app/lib/format.js": text("export function formatDate(d) { return new Date(d).toISOString(); }\n")}, "change helper")
	return root, base, head
}

// CCE-V0-054 (V1-1685): the head tree's TJAA module graph widens reach from a changed module to
// every screen a transitive importer is placed on, and never removes an unknown span.
func TestCCEV0054ModuleGraphWidensReach(t *testing.T) {
	root, base, head := moduleGraphFixture(t)
	plain, err := BuildObligations(bg, diffOpts(root, base, head))
	if err != nil {
		t.Fatal(err)
	}
	if plain.State != Unknown || len(plain.Obligations) != 0 || plain.ModuleGraph != nil ||
		!strings.Contains(strings.Join(plain.Relations.NotConsulted, ","), ModuleGraphBasis) {
		t.Fatalf("baseline: %s", plain.Encoded())
	}
	baseline, _ := json.Marshal(plain.Unknown)
	if !strings.Contains(string(baseline), "app/lib/format.js") {
		t.Fatalf("baseline leaves the helper unplaced: %s", plain.Encoded())
	}
	o := diffOpts(root, base, head)
	o.ModuleGraph = true
	wide, err := BuildObligations(bg, o)
	if err != nil {
		t.Fatal(err)
	}
	// Every baseline span stands with its reason; the reached screens' map unknowns may join them.
	kept := map[string]bool{}
	for _, u := range wide.Unknown {
		b, _ := json.Marshal(u)
		kept[string(b)] = true
	}
	for _, u := range plain.Unknown {
		if b, _ := json.Marshal(u); !kept[string(b)] {
			t.Fatalf("the module graph removed or demoted %s:\n%s", b, wide.Encoded())
		}
	}
	if wide.State != Unknown {
		t.Fatalf("state = %s, want UNKNOWN", wide.State)
	}
	if wide.ModuleGraph == nil || wide.ModuleGraph.Unavailable != "" || wide.ModuleGraph.Digest == "" ||
		!strings.Contains(strings.Join(wide.Relations.Consulted, ","), ModuleGraphBasis) {
		t.Fatalf("module graph not named as consulted: %s", wide.Encoded())
	}
	var ob *Obligation
	for i := range wide.Obligations {
		if wide.Obligations[i].ID == "obligation:book-tee-time/booked" {
			ob = &wide.Obligations[i]
		}
	}
	if ob == nil || ob.State != Reached || ob.Chain[0] != "paths:app/lib/format.js" ||
		ob.Chain[1] != teesheetsScreen+" ("+ModuleGraphBasis+":app/routes.js)" || len(ob.Spans) != 1 || ob.Spans[0].Path != "app/lib/format.js" {
		t.Fatalf("the importer's screen outcome was not reached: %s", wide.Encoded())
	}
	// A change with no importer stays where impact left it.
	lone := commitFiles(t, root, map[string]*string{"app/lib/unused.js": text("export const x = 1;\n")}, "unimported module")
	o.Base, o.Head = head, lone
	alone, err := BuildObligations(bg, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(alone.Obligations) != 0 {
		t.Fatalf("an unimported module reached: %s", alone.Encoded())
	}
}

// CCE-V0-054: the module graph never reads the worktree; an uncommitted importer reaches nothing.
func TestCCEV0054ModuleGraphReadsHeadTree(t *testing.T) {
	root, _ := fixtureRepo(t)
	base := commitFiles(t, root, map[string]*string{"app/lib/format.js": text("export const f = 1;\n")}, "helper")
	head := commitFiles(t, root, map[string]*string{"app/lib/format.js": text("export const f = 2;\n")}, "change helper")
	routes := "import { f } from './lib/format.js'; // format helpers\n" + *overlayText(t, "app/routes.js")
	writeWorktree(t, root, "app/routes.js", routes)
	o := diffOpts(root, base, head)
	o.ModuleGraph = true
	r, err := BuildObligations(bg, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Obligations) != 0 || r.ModuleGraph == nil || r.ModuleGraph.Unavailable != "" {
		t.Fatalf("a worktree importer reached: %s", r.Encoded())
	}
}

func writeWorktree(t *testing.T, root, p, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(p)), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
