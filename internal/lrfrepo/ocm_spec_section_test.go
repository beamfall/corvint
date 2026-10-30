package lrfrepo

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestRDGV0021MovedDefinitionsAreEnumerated pins V1-0798, V1-0988 and V1-0572: definitions that
// once sat in amendment sections after `## Requirements` now sit inside it, so OCM enumerates them.
func TestRDGV0021MovedDefinitionsAreEnumerated(t *testing.T) {
	for _, test := range []struct {
		spec string
		want []string
	}{
		// CAL-V0-073 and CAL-V0-106 stay baseline rows: with them the leases Requirements section
		// would exceed the 256-obligation OCM limit (maxObligations).
		{"corvint-tasks-agent-leases-v0.md", []string{"CAL-V0-072", "CAL-V0-105"}},
		{"local-trace-producer-migration-v0.md", []string{
			"LTPM-V0-001", "LTPM-V0-002", "LTPM-V0-003", "LTPM-V0-004", "LTPM-V0-005", "LTPM-V0-006",
			"LTPM-V0-007", "LTPM-V0-008", "LTPM-V0-009", "LTPM-V0-010", "LTPM-V0-011", "LTPM-V0-012",
			"LTPM-V0-013", "LTPM-V0-014", "LTPM-V0-015", "LTPM-V0-016",
		}},
	} {
		t.Run(test.spec, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "docs", "specs", test.spec))
			if err != nil {
				t.Fatal(err)
			}
			_, requirements, _, err := requirementsFromBlob("docs/specs/"+test.spec, "0123456789abcdef", data)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range test.want {
				if !slices.Contains(requirements, id) {
					t.Errorf("OCM does not enumerate %s from %s", id, test.spec)
				}
			}
		})
	}
}
