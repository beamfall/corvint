package obligation

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// TestTOLV0103_BoundaryTestKindWith: declared patterns extend the closed
// naming set; the closed set answers first, a pattern without "/" matches
// the base name, one with "/" the whole path, and "*" never crosses "/"
// (TOL-V0-103).
func TestTOLV0103_BoundaryTestKindWith(t *testing.T) {
	patterns := []intent.ObligationTestFilePattern{
		{Kind: "sql", Pattern: "deploy/*.sql"},
		{Kind: "rails", Pattern: "*_check.rb"},
		{Kind: "shadow", Pattern: "*_test.go"},
	}
	for p, want := range map[string]string{
		"deploy/schema.sql": "sql", "deploy/sub/schema.sql": "", "other/schema.sql": "",
		"app/users_check.rb": "rails", "users_check.rb": "rails",
		"pkg/a_test.go": "go", "deploy/main.tf": "",
	} {
		if got := BoundaryTestKindWith(p, patterns); got != want {
			t.Errorf("BoundaryTestKindWith(%q) = %q, want %q", p, got, want)
		}
	}
	if got := BoundaryTestKindWith("deploy/schema.sql", nil); got != "" {
		t.Errorf("no patterns: %q", got)
	}
	b := NewBoundaryInventory(nil, []string{"deploy"})
	b.Patterns = patterns[:1]
	b.Visit("deploy/schema.sql")
	b.Visit("deploy/main.tf")
	if b.TestFiles != 1 || b.Kinds["sql"] != 1 {
		t.Fatalf("inventory: %d %v", b.TestFiles, b.Kinds)
	}
}
