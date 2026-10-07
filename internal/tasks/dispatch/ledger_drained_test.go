//go:build darwin || linux

package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-132 (proposed amendment) and CAL-V0-160: a dispatcher drained under
// the build that wrote taskman-dispatch-state/1 restarts under a build that
// writes /2. The drained ledger is adopted under /1's closed member set,
// keeps its backoff history, starts its budget history at the adoption and
// is rewritten as /2 by the next save; a /1 ledger that still records a
// worker or carries a member /1 never had refuses UNSUPPORTED_VERSION
// without being rewritten.
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
	// Version 1 had no budget history: the fixture drops what version 2
	// recorded for the launch.
	var current1 map[string]json.RawMessage
	if err := json.Unmarshal(drained, &current1); err != nil {
		t.Fatal(err)
	}
	if _, ok := current1["budget"]; !ok {
		t.Fatal("the launch recorded no budget history")
	}
	delete(current1, "budget")
	if drained, err = json.Marshal(current1); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(drained, current) || bytes.Contains(drained, []byte(`"budget"`)) {
		t.Fatal("version 1 fixture not reached")
	}

	// Refusals first: none of these rewrites the file. A repeated or
	// case-aliased member, at any depth, could otherwise hide a worker or a
	// member version 1 never had from the drained-only check.
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
	// prefix puts members before the fixture's own, so that a duplicate
	// precedes the member the struct decoder would keep.
	prefix := func(members string) []byte {
		return append([]byte("{"+members+","), drained[1:]...)
	}
	for name, bad := range map[string][]byte{
		"a recorded worker":                      with("workers", `[{"id":"w1"}]`),
		"the budget history":                     with("budget", `{"sessions":[],"truncated":"0001-01-01T00:00:00Z","historyFrom":"0001-01-01T00:00:00Z","held":[]}`),
		"a case-folded alias":                    with("BUDGET", `{}`),
		"an unknown member":                      with("workerLimits", `{}`),
		"a worker hidden by a duplicate workers": prefix(`"workers":[{"id":"w1"}]`),
		"a duplicate pressure":                   prefix(`"pressure":{"sample":{"cpuBusyTicks":1}},"pressure":{"sample":{}}`),
		"a duplicate pressure.sample":            prefix(`"pressure":{"sample":{"cpuBusyTicks":1},"sample":{}}`),
		"a WORKERS alias":                        prefix(`"WORKERS":[{"id":"w1"}]`),
		"a PROGRAM alias":                        prefix(`"PROGRAM":"prog"`),
		"a nested alias":                         prefix(`"pressure":{"SAMPLE":{}}`),
		"a nested unknown member":                prefix(`"pressure":{"sample":{"futureField":1}}`),
		"an unknown backoff member":              with("backoff", `{"k":{"futureField":1}}`),
		"a trailing worker value":                append(append([]byte{}, drained...), []byte(` {"workers":[{"id":"w1"}]}`)...),
		"a Profile alias":                        bytes.Replace(drained, []byte(`"profile":`), []byte(`"Profile":`), 1),
	} {
		if bytes.Equal(bad, drained) {
			t.Fatalf("%s: fixture not reached", name)
		}
		if err := os.WriteFile(path, bad, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadLedger(ProgramDir(c, "prog"), "prog"); wire.CodeOf(err) != wire.CodeUnsupportedVersion {
			t.Errorf("%s: LoadLedger %v", name, err)
		}
		if d, err := Open("prog", c, q, io.Discard); wire.CodeOf(err) != wire.CodeUnsupportedVersion {
			t.Errorf("%s: Open %v", name, err)
			if err == nil {
				d.Close()
			}
		}
		if after, _ := os.ReadFile(path); !bytes.Equal(after, bad) {
			t.Errorf("%s: refused ledger rewritten", name)
		}
	}

	// The drained ledger is adopted without being rewritten by the read.
	if err := os.WriteFile(path, drained, 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := LoadLedger(ProgramDir(c, "prog"), "prog")
	if err != nil {
		t.Fatalf("drained version 1 ledger: %v", err)
	}
	if l.Profile != StateProfile || l.Budget == nil || l.Budget.HistoryFrom.IsZero() || len(l.Budget.Sessions) != 0 || l.Backoff[key] == nil || !l.Backoff[key].CooldownUntil.Equal(cooldown) {
		t.Fatalf("adopted ledger %+v", l)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, drained) {
		t.Fatal("reading the drained ledger rewrote it")
	}

	// Restart through the dispatcher entry point; the next save writes /2.
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
