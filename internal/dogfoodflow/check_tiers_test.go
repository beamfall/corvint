package dogfoodflow

import (
	"fmt"
	"strings"
	"testing"
)

// tierStatusLine is a stable `cem status` line whose map declares the given
// paths and whose recomputed citation block gives them the given tiers.
func tierStatusLine(paths, tiers []string) []byte {
	hunks, rows := []string{}, []string{}
	for i, path := range paths {
		id := fmt.Sprintf("hunk:sha256:%064x", i+1)
		hunks = append(hunks, fmt.Sprintf(`{"id":%q,"path":%q}`, id, path))
		rows = append(rows, fmt.Sprintf(`{"hunkId":%q,"tier":%q,"anchors":[]}`, id, tiers[i]))
	}
	return []byte(`{"ok":true,"state":"accepted","tool":"cem-status","stableVerification":{"outcome":"ACCEPT","hunks":[` +
		strings.Join(hunks, ",") + `],"citation":{"profile":"cem-citation/1","hunks":[` + strings.Join(rows, ",") + `]}}}` + "\n")
}

// DCW-V0-112 (V1-1760): a docs-only change bound with a non-anchoring
// citation (tier `cited`) is named before seal with its path and the CEM-V1-081
// rule it misses; an anchoring citation, and a line with no citation block,
// produce no warning; the warnings are bounded.
func TestDCWV0112CheckWarnsOnHunksBelowAnchored(t *testing.T) {
	got := belowAnchoredWarnings(tierStatusLine([]string{"docs/build-log/entry.md", "docs/specs/x.md", "a.go"}, []string{"cited", "anchored", "unknown"}))
	if len(got) != 2 ||
		got[0] != `dogfood-check: WARN tier-below-anchored path="docs/build-log/entry.md" hunk="hunk:sha256:`+fmt.Sprintf("%064x", 1)+`" tier=cited `+tierBelowAnchoredFix ||
		!strings.HasPrefix(got[1], `dogfood-check: WARN tier-below-anchored path="a.go" `) || !strings.Contains(got[1], " tier=unknown rule=CEM-V1-081 ") {
		t.Fatalf("warnings = %q", got)
	}
	for name, line := range map[string][]byte{
		"anchored":    tierStatusLine([]string{"docs/build-log/entry.md"}, []string{"anchored"}),
		"mechanical":  tierStatusLine([]string{"a.go"}, []string{"mechanical-proved"}),
		"no-citation": []byte(`{"ok":true,"stableVerification":{"outcome":"ACCEPT","citation":null}}`),
		"not-json":    []byte("not json\n"),
	} {
		if got := belowAnchoredWarnings(line); len(got) != 0 {
			t.Errorf("%s: warnings = %q, want none", name, got)
		}
	}
	paths, tiers := []string{}, []string{}
	for i := range maxTierWarnings + 3 {
		paths, tiers = append(paths, fmt.Sprintf("p%d\nforged", i)), append(tiers, "cited")
	}
	got = belowAnchoredWarnings(tierStatusLine(paths, tiers))
	if len(got) != maxTierWarnings+1 || got[maxTierWarnings] != "dogfood-check: WARN tier-below-anchored more=3" || strings.Contains(strings.Join(got, ""), "\n") {
		t.Fatalf("bounded warnings: %d lines, last %q", len(got), got[len(got)-1])
	}
}
