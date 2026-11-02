// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PUR-V0-1921 (V1-4261): a store whose common directory CORVINT_TASKS_FORBID_COMMON_DIR
// names refuses `pause` and every other writer OUT_OF_SCOPE and is left byte-identical, so
// a test or qualification child launched in the wrong directory cannot pause the live
// store and leave it paused. Unset, the same pause succeeds.
func TestPURV01921_ForbiddenStoreRefusesPause(t *testing.T) {
	r := receiptFixture(t)
	t.Setenv(authority.ForbidCommonDirEnv, r.CommonDir)
	before, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	for _, args := range [][]string{
		{"pause", "--request-id", "guarded-pause"},
		{"pause", "--request-id", "guarded-pause-op", "--role", "OPERATOR"},
		{"unpause", "--request-id", "guarded-unpause"},
	} {
		x := atm(t, r.Root, nil, args...)
		if !hasCode(x.res, wire.CodeOutOfScope) {
			t.Fatalf("%v under the guard: %+v, want OUT_OF_SCOPE", args, x.res)
		}
		fixture.AssertUntouched(t, r, before, intents, "guarded "+args[0])
	}
	if _, err := os.Lstat(filepath.Join(r.StateDir, "barrier.json")); !os.IsNotExist(err) {
		t.Fatalf("guarded pause left a barrier: %v", err)
	}
	t.Setenv(authority.ForbidCommonDirEnv, filepath.Join(t.TempDir(), "elsewhere"))
	if x := atm(t, r.Root, nil, "pause", "--request-id", "free-pause"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("pause outside the guard: %+v", x.res)
	}
}
