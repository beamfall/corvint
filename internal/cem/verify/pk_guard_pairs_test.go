// SPDX-License-Identifier: Apache-2.0
package verify

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestPKGuardRecordedPairs is opt-in measurement, not a synthetic performance
// assertion. Each report retains the load, candidates, every result and wall time.
func TestPKGuardRecordedPairs(t *testing.T) {
	repo := os.Getenv("PK_GUARD_REPO")
	if repo == "" {
		t.Skip("set PK_GUARD_REPO to the repository holding the recorded pairs")
	}
	git := func(args ...string) []byte {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", repo}, args...)...)
		b, e := c.Output()
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	for _, pair := range [][2]string{{"85ba6321a", "95622955d"}, {"3465dd98c^1", "3465dd98c"}, {"6e0ce750c^1", "6e0ce750c"}} {
		base := strings.TrimSpace(string(git("rev-parse", pair[0])))
		target := strings.TrimSpace(string(git("rev-parse", pair[1])))
		raw := git("diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--full-index", "--unified=3", base, target, "--", ".", ":(exclude).corvint/change.cem.json")
		parsed, e := patch.Parse(raw)
		if e != nil {
			t.Fatal(e)
		}
		change := &wire.Map{BaseRevision: base}
		for _, h := range parsed.Hunks {
			change.Hunks = append(change.Hunks, wire.Hunk{ID: h.ID, Path: h.DisplayPath, Disposition: "unknown"})
		}
		for run := 0; run < 3; run++ {
			load, _ := os.ReadFile("/proc/loadavg")
			report := runGuardingTests(context.Background(), repo, &stableCapture{change: change, parsed: parsed, target: target}, DefaultHunkRevertLimits, goTestRunner(repo))
			out := struct {
				Base, Target, Load string
				Run                int
				Report             *GuardingTests
			}{base, target, strings.TrimSpace(string(load)), run, report}
			b, _ := json.Marshal(out)
			t.Log(string(b))
		}
	}
}
