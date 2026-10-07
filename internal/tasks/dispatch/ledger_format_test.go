//go:build darwin || linux

package dispatch

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-130 and CAL-V0-132: a dispatcher restarted over a ledger in its own
// format adopts the live worker; a ledger written in another dispatch-state
// version, or carrying a member this build does not know, is refused as
// UNSUPPORTED_VERSION without reading, migrating or touching the worker.
func TestCALV0132_LedgerFromAnotherBuildRefusesAndSameFormatAdopts(t *testing.T) {
	c := testConfig(t, `sleep 300`)
	c.GlobalCap = 1
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	pid := d.ledger.Workers[0].PID
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ProgramDir(c, "prog"), "state.json")
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string]func([]byte) []byte{
		"next profile version": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"`+StateProfile+`"`), []byte(`"taskman-dispatch-state/2"`), 1)
		},
		"version 0 still recording a worker": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"`+StateProfile+`"`), []byte(`"taskman-dispatch-state/0"`), 1)
		},
		"unknown member": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`{`), []byte(`{"workerLimits":{},`), 1)
		},
		"unknown member beside progress": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`{`), []byte(`{"progress":{},"workerLimits":{},`), 1)
		},
	} {
		bad := edit(good)
		if bytes.Equal(bad, good) {
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
		if syscall.Kill(pid, 0) != nil {
			t.Fatalf("%s: refusal touched the worker", name)
		}
	}
	other := bytes.Replace(good, []byte(`"`+StateProfile+`"`), []byte(`"taskman-dispatch/0"`), 1)
	if err := os.WriteFile(path, other, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLedger(ProgramDir(c, "prog"), "prog"); err == nil || wire.CodeOf(err) == wire.CodeUnsupportedVersion {
		t.Fatalf("another profile: %v", err)
	}
	if err := os.WriteFile(path, good, 0o600); err != nil {
		t.Fatal(err)
	}
	d, err = Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.Running() != 1 || !has(kinds(t, d), "adopted") {
		t.Fatalf("not adopted: running %d events %v", d.Running(), kinds(t, d))
	}
}

// CAL-V0-132: encoding/json keeps the last of a repeated member and matches
// struct fields by case folding, so a ledger in this build's format that
// repeats a member, exactly or by case folding, at any depth is refused as
// MALFORMED before any worker action: a trailing "workers":[] or
// "WORKERS":[] cannot hide the recorded worker. A lone folded spelling keeps
// its CAL-V0-064 treatment.
func TestCALV0132_RepeatedLedgerMemberRefuses(t *testing.T) {
	c := testConfig(t, `sleep 300`)
	c.GlobalCap = 1
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	pid := d.ledger.Workers[0].PID
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ProgramDir(c, "prog"), "state.json")
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := bytes.TrimSuffix(bytes.TrimSpace(good), []byte("}"))
	suffix := func(members string) []byte { return append(append([]byte{}, body...), []byte(","+members+"}")...) }
	for name, bad := range map[string][]byte{
		"a trailing empty workers":    suffix(`"workers":[]`),
		"a repeated backoff":          suffix(`"backoff":{}`),
		"a WORKERS alias":             suffix(`"WORKERS":[]`),
		"a PROGRAM alias":             suffix(`"PROGRAM":"prog"`),
		"a repeated worker member":    bytes.Replace(good, []byte(`"pid":`), []byte(`"pid":1,"pid":`), 1),
		"a folded repeat in a worker": bytes.Replace(good, []byte(`"pid":`), []byte(`"PID":1,"pid":`), 1),
		"a repeated pressure sample":  suffix(`"pressure":{"sample":{},"sample":{}}`),
	} {
		if bytes.Equal(bad, good) {
			t.Fatalf("%s: fixture not reached", name)
		}
		if err := os.WriteFile(path, bad, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadLedger(ProgramDir(c, "prog"), "prog"); err == nil || wire.CodeOf(err) != wire.CodeMalformed {
			t.Errorf("%s: LoadLedger %v", name, err)
		}
		if d, err := Open("prog", c, q, io.Discard); err == nil || wire.CodeOf(err) != wire.CodeMalformed {
			t.Errorf("%s: Open %v", name, err)
			if err == nil {
				d.Close()
			}
		}
		if after, _ := os.ReadFile(path); !bytes.Equal(after, bad) {
			t.Errorf("%s: refused ledger rewritten", name)
		}
		if syscall.Kill(pid, 0) != nil {
			t.Fatalf("%s: refusal touched the worker", name)
		}
	}
}
