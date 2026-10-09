//go:build darwin || linux

package store

import (
	"context"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-1047: a lease gate's group is SIGKILLed while its exited leader is
// still unreaped. Here the leader exits at once and leaves a group member
// that writes past the output cap a second later. With the old order the
// leader was reaped first and that overflow later triggered a group kill
// after the reap (OUTPUT_LIMIT); now the member is gone before the reap, so
// the run is the leader's own exit and no kill follows the reap.
func TestV11047LeaseGateKillsGroupBeforeTheReap(t *testing.T) {
	over := wire.MaxGateOutputBytes + 1<<20
	script := "(/bin/sleep 1; /usr/bin/head -c " + strconv.Itoa(over) + " /dev/zero) & exit 0"
	def := &intent.GateDefinition{Argv: []string{"/bin/sh", "-c", script}, TimeoutSeconds: wire.CountOf(30)}
	out := &cappedOutput{}
	run := execute(context.Background(), def, t.TempDir(), []string{}, out)
	if run.class != "EXIT" || run.exitCode == nil || run.exitCode.Int() != 0 {
		t.Fatalf("class=%s exit=%v signal=%v overflow=%v, want EXIT 0", run.class, run.exitCode, run.signal, out.overflow)
	}
	if out.overflow {
		t.Fatal("the leftover group member wrote after the leader was reaped")
	}
}
