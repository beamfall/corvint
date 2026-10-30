package fixture

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CTS-V0-025: the Tasks-side skip fires only on Windows and only for the
// up-front platform refusal, so darwin and linux never lose a test.
func TestCTSV0025_UnsupportedPlatformSkipsOnlyOnWindows(t *testing.T) {
	refusal := wire.Errorf(wire.CodeUnsupported, "platform", "Corvint Tasks is unsupported on this platform")
	for _, goos := range []string{"darwin", "linux"} {
		if reason, skip := unsupportedPlatformSkip(goos, refusal); skip || reason != "" {
			t.Fatalf("%s: skipped with %q, want no skip on a supported platform", goos, reason)
		}
	}
	if _, skip := unsupportedPlatformSkip("windows", nil); skip {
		t.Fatal("windows: skipped without a platform refusal")
	}
	reason, skip := unsupportedPlatformSkip("windows", refusal)
	if !skip || !strings.Contains(reason, "Corvint Tasks is unsupported on this platform") {
		t.Fatalf("windows: skip=%v reason=%q, want a skip quoting the refusal", skip, reason)
	}
}
