package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

var leaseTimingFields = []string{"profile", "totalMillis", "transactions", "preparationRounds", "admissionWaitMillis", "guardMillis", "snapshotReadMillis", "validationMillis", "monitorCloseMillis", "lockWaitMillis", "lockHoldMillis", "journalWriteMillis", "fsyncMillis"}

func leaseTimingOf(t *testing.T, item wire.Value) wire.Value {
	t.Helper()
	timing, ok := item.Obj.Get("timing")
	if !ok || timing.Obj == nil {
		t.Fatalf("no timing object: %s", wire.Encode(item))
	}
	if keys := timing.Obj.Keys; len(keys) != len(leaseTimingFields) {
		t.Fatalf("timing members %v, want %v", keys, leaseTimingFields)
	}
	for _, key := range leaseTimingFields[1:] {
		if _, err := wire.ParseSize(key, field(timing, key).Str); err != nil {
			t.Fatalf("timing %s: %v (%s)", key, err, wire.Encode(timing))
		}
	}
	if field(timing, "profile").Str != cli.LeaseTimingProfile {
		t.Fatalf("timing profile: %s", wire.Encode(timing))
	}
	return timing
}

// TestGH494_LeaseTimingIsOptInDiagnostic: `--timing` adds one closed
// per-phase object on OK and ERROR results of claim, renew, attempt heartbeat
// and release; it is not part of the request, so the same request ID without
// the flag replays the timed write, and default output carries no timing.
func TestGH494_LeaseTimingIsOptInDiagnostic(t *testing.T) {
	root, claimed := leaseCLIStore(t, 1, time.Now().UTC().Add(-12*time.Minute).Truncate(time.Second))
	a := claimed[0]
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	head := func() []byte {
		raw, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}

	renew := []string{"renew", "--attempt", a.AttemptID, "--generation", string(a.Generation), "--request-id", "renew-timed"}
	timed := atm(t, root, nil, append(renew, "--timing")...)
	if timed.res.Outcome != wire.OutcomeOK {
		t.Fatalf("timed renew: %+v", timed.res)
	}
	timing := leaseTimingOf(t, timed.res.Items[0])
	if field(timing, "transactions").Str != "1" || field(timing, "preparationRounds").Str == "0" {
		t.Fatalf("renew counts: %s", wire.Encode(timing))
	}
	after := head()

	plain := atm(t, root, nil, renew...)
	if plain.res.Outcome != wire.OutcomeOK {
		t.Fatalf("replayed renew: %+v", plain.res)
	}
	if _, ok := plain.res.Items[0].Obj.Get("timing"); ok {
		t.Fatalf("default output carries timing: %s", wire.Encode(plain.res.Items[0]))
	}
	if !field(plain.res.Items[0], "replayed").Bool || field(plain.res.Items[0], "attemptId").Str != a.AttemptID || field(plain.res.Items[0], "generation").Str != string(a.Generation) || !bytes.Equal(head(), after) {
		t.Fatalf("untimed retry of a timed request did not replay: %s vs %s", wire.Encode(plain.res.Items[0]), wire.Encode(timed.res.Items[0]))
	}

	heartbeat := atm(t, root, nil, "attempt", "heartbeat", "--attempt", a.AttemptID, "--generation", string(a.Generation), "--request-id", "beat-timed", "--timing")
	if heartbeat.res.Outcome != wire.OutcomeOK {
		t.Fatalf("timed heartbeat: %+v", heartbeat.res)
	}
	leaseTimingOf(t, heartbeat.res.Items[0])

	release := atm(t, root, nil, "release", "--attempt", a.AttemptID, "--generation", string(a.Generation), "--request-id", "release-timed", "--timing")
	if release.res.Outcome != wire.OutcomeOK {
		t.Fatalf("timed release: %+v", release.res)
	}
	leaseTimingOf(t, release.res.Items[0])

	// An ERROR result still reports the phases that ran.
	failed := atm(t, root, nil, "claim", "NOT A TICKET", "--holder", "h", "--request-id", "claim-timed", "--timing")
	if failed.res.Outcome != wire.OutcomeError || len(failed.res.Items) != 1 {
		t.Fatalf("timed failing claim: %+v", failed.res)
	}
	leaseTimingOf(t, failed.res.Items[0])

	for _, args := range [][]string{
		{"reap", "--request-id", "reap-timed", "--timing"},
		{"widen", "--attempt", a.AttemptID, "--generation", string(a.Generation), "--request-id", "widen-timed", "--whole-repository", "--timing"},
	} {
		refused := atm(t, root, nil, args...)
		if refused.res.Outcome != wire.OutcomeError || !hasCode(refused.res, wire.CodeMalformed) || len(refused.res.Items) != 0 {
			t.Fatalf("%v: %+v", args, refused.res)
		}
	}
}
