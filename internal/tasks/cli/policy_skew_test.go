package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCTSV0991_SkewEnvelopeNamesTheBinary: a read meeting a newer policy key
// refuses UNSUPPORTED_VERSION, names this binary's version, and writes
// nothing (CTS-V0-991).
func TestCTSV0991_SkewEnvelopeNamesTheBinary(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	v := fixture.PolicyValue()
	v.Obj.Set("zFutureKey", wire.Array())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	before := fixture.TreeSnapshot(t, r.Root)
	x := atm(t, r.Root, nil, "queue", "status")
	if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeUnsupportedVersion) {
		t.Fatalf("queue status over a newer policy: %s", x.stdout)
	}
	if w := strings.Join(x.res.Warnings, "\n"); !strings.Contains(w, "corvint-tasks "+cli.Version+"+build."+cli.Build) || !strings.Contains(w, "zFutureKey") {
		t.Errorf("skew refusal does not name the binary and the key: %v", x.res.Warnings)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.Root)) {
		t.Fatal("the refusing read wrote state")
	}
}
