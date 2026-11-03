package registry_test

import (
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// TestFamiliesAreDisjointAndComplete_LGPV0010 checks the dispatch table: each
// family has Build and Parse, no runner is registered by two families, and
// Lookup returns the registering family for every runner.
func TestFamiliesAreDisjointAndComplete_LGPV0010(t *testing.T) {
	seen := map[string]string{}
	for _, f := range registry.Families() {
		if f.Name == "" || f.Runners == nil || f.Build == nil || f.Parse == nil {
			t.Fatalf("incomplete family %+v", f.Name)
		}
		for _, r := range f.Runners() {
			if prev, ok := seen[r]; ok {
				t.Errorf("runner %s registered by %s and %s", r, prev, f.Name)
			}
			seen[r] = f.Name
			if got, ok := registry.Lookup(r); !ok || got.Name != f.Name {
				t.Errorf("Lookup(%s) = %s, want %s", r, got.Name, f.Name)
			}
		}
	}
	if len(seen) != len(registry.Runners()) {
		t.Errorf("Runners() has %d names, families %d", len(registry.Runners()), len(seen))
	}
	if _, ok := registry.Lookup("no-such-runner"); ok {
		t.Error("unknown runner resolved")
	}
	if _, e := registry.Build(tr.Request{Runner: "no-such-runner", Executable: "/x", ReportDir: "/r"}); e == nil || !strings.Contains(e.Error(), "unsupported runner") {
		t.Errorf("unknown runner built: %v", e)
	}
	if _, e := registry.Parse(tr.Input{Runner: "no-such-runner"}); e == nil || !strings.Contains(e.Error(), "unsupported runner") {
		t.Errorf("unknown runner parsed: %v", e)
	}
}
