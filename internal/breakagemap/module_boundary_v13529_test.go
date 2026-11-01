package breakagemap

import (
	"context"
	"strings"
	"testing"
)

// BKM-V0-710 (V1-3529): with two repositories whose API repository tracks go.mod at its root, a
// manifest that omits that go.mod withholds the Go mapping with an unknown naming api:go.mod and the
// --source that declares it; declaring it yields the syntax edges; a declared go.mod whose blob
// differs from the pinned tree's says so instead of claiming it is undeclared. The reasons are the
// shared BKM-V0-955 wording.
func TestModuleBoundaryNamesTheMissingGoMod_V13529(t *testing.T) {
	m, bindings, _ := fixture(t)
	reason := func(m Manifest) (string, int) {
		r, err := Compile(context.Background(), encode(t, m), bindings, "api:pkg/api.go:Changed", "")
		if err != nil {
			t.Fatal(err)
		}
		syntax := 0
		for _, edge := range r.Edges {
			if edge.Relation == nil && edge.Kind == "syntax-call" {
				syntax++
			}
		}
		for _, u := range r.Unknowns {
			if strings.Contains(u.Reason, "module boundary") {
				return u.Reason, syntax
			}
		}
		return "", syntax
	}
	if why, syntax := reason(m); why != "" || syntax == 0 {
		t.Fatalf("declared root go.mod: unknown %q, %d syntax edges", why, syntax)
	}
	if m.Sources[0].Repository != "api" || m.Sources[0].Path != "go.mod" {
		t.Fatalf("fixture order changed: %+v", m.Sources[0])
	}
	omitted := m
	omitted.Sources = append([]Source(nil), m.Sources[1:]...)
	why, syntax := reason(omitted)
	if want := "api:go.mod is tracked at the pinned commit but not a declared manifest source; add `--source api:go.mod`"; !strings.Contains(why, want) || syntax != 0 {
		t.Fatalf("omitted go.mod: unknown %q, %d syntax edges; want it to name %q", why, syntax, want)
	}
	mismatched := m
	mismatched.Sources = append([]Source(nil), m.Sources...)
	mismatched.Sources[0].Blob = strings.Repeat("a", 40)
	if why, _ := reason(mismatched); !strings.Contains(why, "api:go.mod is declared with blob "+strings.Repeat("a", 40)+" but the pinned tree has ") {
		t.Fatalf("mismatched go.mod: unknown %q", why)
	}
}
