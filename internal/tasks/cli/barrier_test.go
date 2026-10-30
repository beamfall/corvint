package cli_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestTMV0016_AS27_CLIBarrierWorkflow(t *testing.T) {
	r := receiptFixture(t)
	pause := []string{"pause", "--request-id", "pause"}
	if x := atm(t, r.Root, nil, pause...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("pause: %+v", x.res)
	}
	status := atm(t, r.Root, nil, "queue", "status")
	if status.res.Snapshot.Barrier == nil || status.res.Snapshot.Barrier.Scope != "ADMISSION" {
		t.Fatalf("status: %+v", status.res)
	}
	// UNPAUSE's exception must survive the CLI; ordinary intent.Load rejects D.
	path := filepath.Join(r.IntentDir, "tickets", "AT-0002.json")
	fixture.Write(t, path, []byte{})
	before := fixture.TreeSnapshot(t, r.IntentDir)
	x := atm(t, r.Root, nil, "unpause", "--request-id", "remove")
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "ticketId").Str != "" {
		t.Fatalf("unpause: %+v", x.res)
	}
	if !reflect.DeepEqual(before, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("CLI unpause changed empty D")
	}
	if _, err := os.Lstat(filepath.Join(r.StateDir, "barrier.json")); !os.IsNotExist(err) {
		t.Fatalf("barrier remains: %v", err)
	}
	x = atm(t, r.Root, nil, pause...)
	if x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "replayed").Bool {
		t.Fatalf("replay: %+v", x.res)
	}
	if _, err := os.Lstat(filepath.Join(r.StateDir, "barrier.json")); !os.IsNotExist(err) {
		t.Fatal("retry pause reintroduced barrier")
	}
	inspect := atm(t, r.Root, nil, "reconcile", "inspect", "ticket:acme:main:AT-0002")
	if inspect.res.Outcome != wire.OutcomeOK || inspect.res.Snapshot.Barrier != nil {
		t.Fatalf("inspection: %+v", inspect.res)
	}
}

func TestTMV0016_AS27_CLIBarrierClosedArguments(t *testing.T) {
	r := receiptFixture(t)
	before, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	for _, verb := range []string{"pause", "unpause"} {
		for _, args := range [][]string{{}, {"--request-id"}, {"--request-id", ""}, {"--request-id", "x", "--request-id", "y"}, {"--request-id", "x", "--role", "WORKER"}, {"--request-id", "x", "--reason", "CUTOVER"}, {"--request-id", "x", "--role", "OWNER", "--role", "OPERATOR"}, {"unexpected"}} {
			x := atm(t, r.Root, nil, append([]string{verb}, args...)...)
			if x.res.Outcome == wire.OutcomeOK {
				t.Fatalf("invalid args accepted: %v", args)
			}
			fixture.AssertUntouched(t, r, before, intents, "invalid barrier arguments")
		}
	}
	uninitialized := fixture.TempRepo(t)
	if x := atm(t, uninitialized.Root, nil, "pause", "--request-id", "x"); !hasCode(x.res, wire.CodeUninitialized) {
		t.Fatalf("uninitialized: %+v", x.res)
	}
}
