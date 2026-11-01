package flowdocs

import (
	"context"
	"strings"
	"testing"
)

// TestAFUV1811MultiFileDiagramCallsNamePath (V1-2338): a flow whose calls span several files names
// each call's path in its diagram; a single-file flow renders as before.
func TestAFUV1811MultiFileDiagramCallsNamePath(t *testing.T) {
	root, rev := inventoryFixture(t)
	r, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "app", Flows: []byte(inventoryInput)})
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range r.Manifest.Flows {
		paths := map[string]bool{}
		for _, c := range f.Calls {
			paths[c.Anchor.Path] = true
		}
		diagram := string(r.Files[strings.TrimPrefix(f.ID, "flowdocs:flow:")+"/flow-diagram.md"])
		lines := 0
		for _, line := range strings.Split(diagram, "\n") {
			if !strings.Contains(line, "Source contains call") {
				continue
			}
			lines++
			named := strings.Contains(line, " in app/a.rb at line ") || strings.Contains(line, " in app/b.js at line ")
			if named != (len(paths) > 1) {
				t.Errorf("%s (%d paths): %q", f.Name, len(paths), line)
			}
		}
		if lines != len(f.Calls) || lines == 0 {
			t.Fatalf("%s: %d call lines for %d calls:\n%s", f.Name, lines, len(f.Calls), diagram)
		}
		if f.Name == "Checkout" && len(paths) != 2 {
			t.Fatalf("checkout calls do not span both members: %+v", f.Calls)
		}
	}
	if got := mermaidPath("app/x y#;.rb"); got != "app/x_y__.rb" {
		t.Fatalf("mermaidPath = %q", got)
	}
}
