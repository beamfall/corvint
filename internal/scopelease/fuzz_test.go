package scopelease

import "testing"

// FuzzOverlapsIsSoundForCoveredPaths checks the property Acquire relies on to
// keep Check from ever finding a path under two live leases: when two
// admitted scopes both cover one admitted touched path (SCL-V0-009), the
// conservative overlap rule (SCL-V0-002) must report them as overlapping.
func FuzzOverlapsIsSoundForCoveredPaths(f *testing.F) {
	f.Add("internal/**", "internal/scopelease/**", "internal/scopelease/lease.go")
	f.Add("src/**/x.go", "src/a/*/x.go", "src/a/b/x.go")
	f.Add("src/[ab]/x.go", "src/a/x.go", "src/a/x.go")
	f.Add("Internal/x", "internal/x", "internal/x")
	f.Fuzz(func(t *testing.T, held, wanted, touched string) {
		held, heldErr := normalizePath(held)
		wanted, wantedErr := normalizePath(wanted)
		touched, touchedErr := normalizePath(touched)
		if heldErr != nil || wantedErr != nil || touchedErr != nil {
			return
		}
		if !covers(Lease{Paths: []string{held}}, touched) || !covers(Lease{Paths: []string{wanted}}, touched) {
			return
		}
		if !overlaps(held, wanted) || !overlaps(wanted, held) {
			t.Fatalf("%q and %q both cover %q but do not overlap", held, wanted, touched)
		}
	})
}
