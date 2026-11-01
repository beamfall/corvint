package breakagemap

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-770, EEP-V0-773: a grafted or shallow checkout pins no origin root commit and binds no
// repository whose base ancestry the compile would answer from rewritten parents.
func TestBreakageRefusesRewrittenAncestry_EEPV0770(t *testing.T) {
	m, bindings, _ := fixture(t)
	sources := []string{}
	for _, s := range m.Sources {
		sources = append(sources, s.Repository+":"+s.Path)
	}
	for _, fixture := range []struct {
		name, reason string
		apply        func() func()
	}{
		{"grafted", "checkout rewrites ancestry with info/grafts", func() func() { return ancestrytest.Graft(t, bindings["api"], "HEAD") }},
		{"shallow", "checkout is shallow", func() func() { return ancestrytest.Shallow(t, bindings["api"], "HEAD") }},
	} {
		undo := fixture.apply()
		if _, err := BuildManifest(context.Background(), bindings, sources); err == nil || err.Error() != "api: "+fixture.reason {
			t.Errorf("%s: BuildManifest error = %v", fixture.name, err)
		}
		r, err := Compile(context.Background(), encode(t, m), bindings, "api:pkg/api.go:Changed", "")
		if err != nil {
			t.Fatal(err)
		}
		unbound := false
		for _, u := range r.Unknowns {
			unbound = unbound || (u.Subject == "api" && strings.Contains(u.Reason, fixture.reason))
		}
		if !unbound {
			t.Errorf("%s: api bound: %+v", fixture.name, r.Unknowns)
		}
		undo()
	}
	if _, err := BuildManifest(context.Background(), bindings, sources); err != nil {
		t.Fatalf("plain checkout refused: %v", err)
	}
}
