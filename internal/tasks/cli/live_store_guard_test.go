// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"context"
	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// PUR-V0-2211: a qualification child started from a linked lane cannot pause
// the fixture primary. The actual checkout is never a write target.
func TestPURV02211_LanePauseRefusesPrimary(t *testing.T) {
	r := receiptFixture(t)
	git(t, r.Root, "init", "-q", "-b", "main")
	tcdCommit(t, r.Root, "fixture genesis")
	lane := filepath.Join(t.TempDir(), "lane")
	git(t, r.Root, "worktree", "add", "-q", "-b", "lane", lane)
	t.Setenv(authority.ForbidCommonDirEnv, r.CommonDir)
	before, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	for _, verb := range []string{"pause", "unpause"} {
		cmd := exec.Command(cli.CompiledTasksBinary(t), verb, "--request-id", "lane-"+verb)
		cmd.Dir = lane
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "OUT_OF_SCOPE") {
			t.Fatalf("lane %s: %v %s", verb, err, out)
		}
		fixture.AssertUntouched(t, r, before, intents, "lane child "+verb)
	}
}

// PUR-V0-2210: queue status exposes the stale request and age without writing.
func TestPURV02210_QueueStatusWarnsStaleBarrier(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-2*time.Hour))
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	report, err := store.Barrier(context.Background(), repo, mutation.Binding{ID: "tester", Role: "OWNER"}, store.BarrierRequest{QueueID: fixture.QueueID, RequestID: "old-request", Operation: transaction.Pause}, wire.Timestamp(time.Now().UTC().Add(-90*time.Minute).Format(time.RFC3339)))
	if err != nil {
		t.Fatal(err)
	}
	if report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("pause: %+v", report)
	}
	r := &fixture.Repo{Root: root, StateDir: repo.StateDir, IntentDir: filepath.Join(repo.IntentRoot(), intent.Dir)}
	before, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	x := atm(t, r.Root, nil, "queue", "status", "--summary")
	if x.code != 0 || !strings.Contains(string(x.stdout), "STALE_BARRIER: pause request old-request, age ") {
		t.Fatalf("status: %s", x.stdout)
	}
	fixture.AssertUntouched(t, r, before, intents, "status warning")
}
