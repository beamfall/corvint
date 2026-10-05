package cli

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// SERVICE500-001: every ROLLED_BACK install answers RESTORED at the CLI,
// whether the lifecycle returned the RESTORED error (first attempt or the
// call completing a held rollback) or a replayed ROLLED_BACK record.
func TestSERVICE500_RolledBackAlwaysAnswersRestored(t *testing.T) {
	cmd := []string{"service", "install"}
	replay := wire.NewObject().Set("phase", wire.String("ROLLED_BACK")).Set("replayed", wire.Bool(true))
	for name, res := range map[string]*wire.Result{
		"error":  serviceResult(cmd, nil, wire.Errorf(wire.CodeRestored, "/operation", "install rolled back")),
		"replay": serviceResult(cmd, replay, nil),
	} {
		if res.Outcome != wire.OutcomeError || len(res.Codes) != 1 || res.Codes[0] != wire.CodeRestored {
			t.Fatalf("%s: want ERROR RESTORED, got %s %v", name, res.Outcome, res.Codes)
		}
	}
	ok := serviceResult(cmd, wire.NewObject().Set("phase", wire.String("INSTALLED")), nil)
	if ok.Outcome != wire.OutcomeOK || len(ok.Codes) != 0 {
		t.Fatalf("installed: %s %v", ok.Outcome, ok.Codes)
	}
}
