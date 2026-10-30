package verify

import (
	"errors"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/patch"
)

// TestStablePatchRefusalKeepsFrozenVocabulary pins CEM-V1-302: the stable
// profile's closed issue vocabulary has no metadata-only code, so a
// metadata-only-change-unsupported parse refusal reports malformed-patch, the
// stable result it produced before V1-0609, and every other error passes
// through unchanged.
func TestStablePatchRefusalKeepsFrozenVocabulary(t *testing.T) {
	t.Parallel()
	_, err := patch.Parse([]byte("diff --git a/m.sh b/m.sh\nold mode 100644\nnew mode 100755\n"))
	if cemcode.CodeOf(err) != cemcode.MetadataOnlyChangeUnsupported {
		t.Fatalf("parse error = %v, want %s", err, cemcode.MetadataOnlyChangeUnsupported)
	}
	mapped := stablePatchRefusal(err)
	if cemcode.CodeOf(mapped) != cemcode.MalformedPatch || cemcode.MessageOf(mapped) != cemcode.MessageOf(err) {
		t.Fatalf("stable refusal = %v, want malformed-patch with the same message", mapped)
	}
	other := cemcode.New(cemcode.BinaryPatch, "patch is empty")
	if got := stablePatchRefusal(other); got != other {
		t.Fatalf("unrelated error rewritten: %v", got)
	}
	plain := errors.New("plain")
	if got := stablePatchRefusal(plain); got != plain {
		t.Fatalf("plain error rewritten: %v", got)
	}
}
