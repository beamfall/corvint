package contextindex

import (
	"testing"
)

// DIRTY-CACHE-019: only a `<name>.cem.json` file directly under
// `.corvint/changes/` is a sealed CEM; the sidecar and any other layout are not.
func TestSealedChangeEvidencePathLayout(t *testing.T) {
	for value, want := range map[string]bool{
		".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json": true,
		".corvint/changes/batch.cem.json":                                    true,
		".corvint/changes/.cem.json":                                         false,
		".corvint/changes/notes.json":                                        false,
		".corvint/changes/nested/x.cem.json":                                 false,
		".corvint/change.cem.json":                                           false,
		"docs/.corvint/changes/x.cem.json":                                   false,
		"x/.corvint/changes/x.cem.json":                                      false,
	} {
		if got := sealedChangeEvidencePath(value); got != want {
			t.Errorf("sealedChangeEvidencePath(%q) = %v, want %v", value, got, want)
		}
		wantReason := ""
		if want {
			wantReason = sealedChangeEvidenceReason
		}
		if got := ForbiddenPathReason(value); got != wantReason {
			t.Errorf("ForbiddenPathReason(%q) = %q, want %q", value, got, wantReason)
		}
	}
}
