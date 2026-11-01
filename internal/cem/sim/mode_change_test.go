package sim

import (
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/patch"
)

// TestCEMV1931ModeChangeBaseSimulation pins CEM-V1-931 (V1-3404): a group
// with an old mode/new mode pair applies its hunks only when the base path is
// in the old mode, and refuses diff-metadata-mismatch otherwise.
func TestCEMV1931ModeChangeBaseSimulation(t *testing.T) {
	t.Parallel()
	parsed, err := patch.Parse([]byte("diff --git a/m.sh b/m.sh\nold mode 100644\nnew mode 100755\n--- a/m.sh\n+++ b/m.sh\n@@ -1 +1 @@\n-a\n+b\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Simulate(parsed, modeSource{"m.sh": {[]byte("a\n"), "100644"}}); err != nil {
		t.Fatalf("Simulate over a base in old mode: %v", err)
	}
	err = Simulate(parsed, modeSource{"m.sh": {[]byte("a\n"), "100755"}})
	if cemcode.CodeOf(err) != cemcode.DiffMetadataMismatch {
		t.Fatalf("Simulate over a base in the new mode = %v, want %s", err, cemcode.DiffMetadataMismatch)
	}
}
