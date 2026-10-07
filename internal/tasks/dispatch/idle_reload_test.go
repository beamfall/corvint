//go:build darwin || linux

package dispatch

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// CAL-V0-127 with CAL-V0-139: a changed configuration file ends an idle
// skip, so it still applies at the next tick; an unchanged or already
// refused file keeps the skip without being read again, and restoring the
// applied file ends it.
func TestCALV0127_ConfigChangeEndsAnIdleSkip(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].Match = &Match{Labels: []string{"no-such-label"}}
	clock := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	q := &witnessQueue{witness: "w1"}
	q.obs.Tickets = []Ticket{ticket("T-1", "P1", 1)}
	path := filepath.Join(t.TempDir(), "dispatch.json")
	writeRaw := func(raw []byte) {
		t.Helper()
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	applied, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	writeRaw(applied)
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Now = func() time.Time { return clock }
	reads := 0
	d.WatchConfig(func() ([]byte, fs.FileInfo, error) { reads++; return readWithInfo(path) }, func() (fs.FileInfo, error) { return os.Lstat(path) }, applied)
	observed := 0
	tick := func(want int, why string) {
		t.Helper()
		before := reads
		if err := d.Tick(context.Background()); err != nil {
			t.Fatalf("%s: tick: %v", why, err)
		}
		if q.observes != want {
			t.Fatalf("%s: %d full observations, want %d", why, q.observes, want)
		}
		if skipped := q.observes == observed; skipped && reads != before {
			t.Fatalf("%s: a skipped tick read the unchanged configuration file %d times", why, reads-before)
		}
		observed = q.observes
	}
	tick(1, "first tick")
	tick(2, "a tick that records the observation reads in full")
	tick(2, "an unchanged configuration file keeps the idle skip")

	next := cloneConfig(t, c)
	next.Roles[0].Prompt = "changed {ticketLocal}"
	raw, err := json.Marshal(next)
	if err != nil {
		t.Fatal(err)
	}
	writeRaw(raw)
	tick(3, "a changed configuration file reads in full")
	if d.Config.Roles[0].Prompt != "changed {ticketLocal}" || d.ledger.Config == nil || d.ledger.Config.AppliedSha256 != digest(raw) {
		t.Fatalf("change not applied at the next tick: %+v", d.ledger.Config)
	}
	tick(4, "the applied record changed the ledger")
	tick(4, "the new fixed point skips")

	invalid := []byte(`{"profile":"nope"}`)
	writeRaw(invalid)
	tick(5, "an invalid file reads in full and is refused")
	if r := d.ledger.Config.Refused; r == nil || r.Sha256 != digest(invalid) {
		t.Fatalf("refusal %+v", d.ledger.Config)
	}
	tick(6, "the refusal changed the ledger")
	tick(6, "an already refused file keeps the skip")

	writeRaw(raw)
	tick(7, "restoring the applied file reads in full")
	if d.ledger.Config.Refused != nil {
		t.Fatalf("restored file kept the refusal: %+v", d.ledger.Config)
	}
}

// readWithInfo reads path with the stat of the descriptor it read, as the
// CLI's configuration reader does.
func readWithInfo(path string) ([]byte, fs.FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}
	return raw, st, nil
}

// CAL-V0-127 with CAL-V0-139: a file replaced between the named file's stat
// and the read keeps the read bytes paired with their own file's stat, so a
// later swap back to the other file is still seen as a change.
func TestCALV0139_ConfigReadPairsItsBytesWithTheirOwnStat(t *testing.T) {
	c := testConfig(t, "exit 0")
	d, err := Open("prog", c, &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	dir := t.TempDir()
	path, asideA, asideB := filepath.Join(dir, "dispatch.json"), filepath.Join(dir, "a"), filepath.Join(dir, "b")
	applied := []byte(`{"version":"A"}`)
	move := func(from, to string) {
		t.Helper()
		if err := os.Rename(from, to); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, applied, 0o600); err != nil {
		t.Fatal(err)
	}
	var between func()
	d.WatchConfig(func() ([]byte, fs.FileInfo, error) {
		if between != nil {
			between()
		}
		return readWithInfo(path)
	}, func() (fs.FileInfo, error) { return os.Lstat(path) }, applied)

	// File B replaces the applied file A; A is restored between the stat
	// and the read.
	move(path, asideA)
	if err := os.WriteFile(path, []byte(`{"version":"B, changed"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	between = func() { move(path, asideB); move(asideA, path) }
	if d.configPending() {
		t.Fatal("the applied bytes read back are pending")
	}
	between = nil
	// Swapping B back must be seen even though its stat is the one the
	// named file had before that read.
	move(path, asideA)
	move(asideB, path)
	if !d.configPending() {
		t.Fatal("a swapped-in changed file was paired with the applied bytes' digest")
	}
	move(path, asideB)
	move(asideA, path)
	if d.configPending() {
		t.Fatal("the restored applied file is pending")
	}
}
