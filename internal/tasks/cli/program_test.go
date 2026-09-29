package cli_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestSupervisorCLIRefusalAndReadsDoNotMutate(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	config := filepath.Join(t.TempDir(), "program.json")
	fixture.Write(t, config, []byte("{}\n"))
	state := fixture.TreeSnapshot(t, r.StateDir)
	intent := fixture.TreeSnapshot(t, r.IntentDir)
	bad := atm(t, r.Root, nil, "run", "--program", "p", "--config", config, "--role", "unknown")
	if bad.res.Outcome != wire.OutcomeError {
		t.Fatalf("invalid role: %+v", bad.res)
	}
	for _, args := range [][]string{{"program", "show"}, {"pending"}} {
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 0 {
			t.Fatalf("%v: %+v", args, x.res)
		}
	}
	if !reflect.DeepEqual(state, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(intent, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("supervisor refusal/read mutated initialized store")
	}
}
