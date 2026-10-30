//go:build unix

package workqueuev0

import (
	"slices"
	"testing"
)

// WQO-V0-095: the replay harness forwards the operator's scratch parent override
// into its hand-built child environments and leaves the presets alone when unset.
func TestWithWorkRunParent(t *testing.T) {
	preset := []string{"TZ=UTC", "PATH=/usr/bin:/bin"}
	t.Setenv("CORVINT_WORK_RUN_PARENT", "/private/tmp/parent")
	got := withWorkRunParent(preset)
	if !slices.Contains(got, "CORVINT_WORK_RUN_PARENT=/private/tmp/parent") || len(preset) != 2 {
		t.Fatalf("override not forwarded or preset mutated: %v %v", got, preset)
	}
	if !slices.Contains(fixedEnv(), "CORVINT_WORK_RUN_PARENT=/private/tmp/parent") {
		t.Fatal("fixedEnv drops the override")
	}
}
