//go:build darwin || linux

package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-132 (proposed amendment): a dispatcher drained under the build that
// wrote taskman-dispatch-state/0 restarts under a build that writes /1. The
// drained ledger is adopted under /0's closed member set, keeps its backoff
// history and is rewritten as /1 by the next save; a /0 ledger that still
// records a worker or carries a member /0 never had refuses
// UNSUPPORTED_VERSION without being rewritten.
func TestCALV0132_DrainedPreviousVersionLedgerIsAdopted(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.GlobalCap = 1
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	// The worker launches, exits and is reaped: the dispatcher drains.
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(d.ledger.Workers) != 1 {
		t.Fatalf("workers %d, want the launched one", len(d.ledger.Workers))
	}
	for i := 0; i < 200 && len(d.ledger.Workers) > 0; i++ {
		time.Sleep(10 * time.Millisecond)
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.ledger.Workers) != 0 {
		t.Fatal("the worker was not reaped")
	}
	cooldown := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	key := "ticket:a:q:t1"
	d.ledger.Backoff[key] = &BackoffState{Fingerprint: Fingerprint(&q.obs, key), NoProgress: 2, Parked: true, CooldownUntil: cooldown}
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ProgramDir(c, "prog"), "state.json")
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	drained := bytes.Replace(current, []byte(`"`+StateProfile+`"`), []byte(`"`+drainedStateProfile+`"`), 1)
	if bytes.Equal(drained, current) || bytes.Contains(drained, []byte(`"config"`)) {
		t.Fatal("version 0 fixture not reached")
	}

	// Refusals first: none of these rewrites the file.
	with := func(name, value string) []byte {
		t.Helper()
		var members map[string]json.RawMessage
		if err := json.Unmarshal(drained, &members); err != nil {
			t.Fatal(err)
		}
		members[name] = json.RawMessage(value)
		raw, err := json.Marshal(members)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	for name, bad := range map[string][]byte{
		"a recorded worker":   with("workers", `[{"id":"w1"}]`),
		"the config record":   with("config", `{"appliedSha256":"`+strings.Repeat("a", 64)+`","appliedAt":"2026-10-07T12:00:00Z"}`),
		"a CPU sample field":  with("pressure", `{"sample":{"cpuBusyTicks":1}}`),
		"a case-folded alias": with("CONFIG", `{}`),
		"an unknown member":   with("workerLimits", `{}`),
	} {
		if bytes.Equal(bad, drained) {
			t.Fatalf("%s: fixture not reached", name)
		}
		if err := os.WriteFile(path, bad, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadLedger(ProgramDir(c, "prog"), "prog"); wire.CodeOf(err) != wire.CodeUnsupportedVersion {
			t.Fatalf("%s: LoadLedger %v", name, err)
		}
		if _, err := Open("prog", c, q, io.Discard); wire.CodeOf(err) != wire.CodeUnsupportedVersion {
			t.Fatalf("%s: Open %v", name, err)
		}
		if after, _ := os.ReadFile(path); !bytes.Equal(after, bad) {
			t.Fatalf("%s: refused ledger rewritten", name)
		}
	}

	// The drained ledger is adopted without being rewritten by the read.
	if err := os.WriteFile(path, drained, 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := LoadLedger(ProgramDir(c, "prog"), "prog")
	if err != nil {
		t.Fatalf("drained version 0 ledger: %v", err)
	}
	if l.Profile != StateProfile || l.Config != nil || l.Backoff[key] == nil || !l.Backoff[key].CooldownUntil.Equal(cooldown) {
		t.Fatalf("adopted ledger %+v", l)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, drained) {
		t.Fatal("reading the drained ledger rewrote it")
	}

	// Restart through the dispatcher entry point; the next save writes /1.
	d, err = Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatalf("restart over the drained ledger: %v", err)
	}
	defer d.Close()
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(saved, []byte(`"`+StateProfile+`"`)) || bytes.Contains(saved, []byte(drainedStateProfile)) {
		t.Fatalf("restart did not persist %s: %s", StateProfile, saved)
	}
	if l, err := LoadLedger(ProgramDir(c, "prog"), "prog"); err != nil || l.Backoff[key] == nil || !l.Backoff[key].CooldownUntil.Equal(cooldown) {
		t.Fatalf("history after the rewrite %+v %v", l, err)
	}
}
