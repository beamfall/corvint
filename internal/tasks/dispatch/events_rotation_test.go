package dispatch

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestCALV0058_EventLogRotatesOnceAt16MiB drives events.jsonl past the real
// 16 MiB threshold: the next append renames it to events.jsonl.1 whole and
// restarts the live log with exactly the new line, and a second rotation
// replaces events.jsonl.1, so the log never holds more than two segments.
func TestCALV0058_EventLogRotatesOnceAt16MiB(t *testing.T) {
	dir := t.TempDir()
	live := filepath.Join(dir, "events.jsonl")
	seed := func(seq uint64) []byte {
		line, err := json.Marshal(Event{Profile: EventProfile, Seq: seq, At: "2026-10-07T00:00:00Z", Program: "p", Kind: "state", Message: string(bytes.Repeat([]byte("x"), 4000))})
		if err != nil {
			t.Fatal(err)
		}
		line = append(line, '\n')
		raw := bytes.Repeat(line, maxEventsBytes/len(line)+1)
		if int64(len(raw)) <= maxEventsBytes {
			t.Fatalf("seed of %d bytes does not cross %d", len(raw), maxEventsBytes)
		}
		if err := os.WriteFile(live, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return raw
	}
	assertRotated := func(old []byte, seq uint64) {
		t.Helper()
		got, err := os.ReadFile(live + ".1")
		if err != nil || !bytes.Equal(got, old) {
			t.Fatalf("events.jsonl.1 = %d bytes (%v), want the %d-byte previous log", len(got), err, len(old))
		}
		st, err := os.Stat(live)
		if err != nil || st.Size() >= 4<<10 {
			t.Fatalf("live log after rotation: %v, %v", st, err)
		}
		events, err := ReadEvents(dir, 10)
		if err != nil || len(events) != 1 || events[0].Seq != seq || events[0].Kind != "alert" {
			t.Fatalf("live events = %+v, %v; want only seq %d", events, err, seq)
		}
		if _, err := os.Stat(live + ".2"); !os.IsNotExist(err) {
			t.Fatalf("a third segment exists: %v", err)
		}
	}

	first := seed(1)
	if err := appendEventLog(dir, Event{Profile: EventProfile, Seq: 100, Kind: "alert", Message: "after rotation"}); err != nil {
		t.Fatal(err)
	}
	assertRotated(first, 100)

	// Below the threshold the live log only grows.
	if err := appendEventLog(dir, Event{Profile: EventProfile, Seq: 101, Kind: "alert", Message: "no rotation"}); err != nil {
		t.Fatal(err)
	}
	if events, _ := ReadEvents(dir, 10); len(events) != 2 {
		t.Fatalf("live events below the threshold = %+v", events)
	}

	second := seed(2)
	if err := appendEventLog(dir, Event{Profile: EventProfile, Seq: 200, Kind: "alert", Message: "second rotation"}); err != nil {
		t.Fatal(err)
	}
	assertRotated(second, 200)
}
