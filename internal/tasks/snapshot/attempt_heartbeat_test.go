package snapshot

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"testing"
)

func TestCALV0048_HeartbeatLegacyRoundTrip(t *testing.T) {
	a := accountingAttempt()
	legacy, e := a.Encode()
	if e != nil {
		t.Fatal(e)
	}
	b, e := DecodeAttempt(legacy)
	if e != nil || b.LastHeartbeatAt != nil || b.RetryReasons != nil {
		t.Fatalf("legacy: %v", e)
	}
	again, e := b.Encode()
	if e != nil || !bytes.Equal(legacy, again) {
		t.Fatal("legacy bytes changed")
	}
	at := wire.Timestamp("2026-09-30T23:30:00Z")
	a.LastHeartbeatAt = &at
	raw, e := a.Encode()
	if e != nil {
		t.Fatal(e)
	}
	b, e = DecodeAttempt(raw)
	if e != nil || *b.LastHeartbeatAt != at {
		t.Fatalf("heartbeat: %v", e)
	}
	if _, e := DecodeAttempt(bytes.Replace(raw, []byte(string(at)), []byte("bad"), 1)); e == nil {
		t.Fatal("invalid timestamp accepted")
	}
}
func TestCALV0049_ReasonTotalsAndClosedSchema(t *testing.T) {
	a := accountingAttempt()
	a.RetryCount = "2"
	a.RetryReasons = map[string]wire.Count{"EXPIRED": "1", "RELEASED": "1", "FAILED": "0", "UNKNOWN": "0"}
	raw, e := a.Encode()
	if e != nil {
		t.Fatal(e)
	}
	for _, bad := range [][]byte{bytes.Replace(raw, []byte(`"EXPIRED":"1"`), []byte(`"EXPIRED":"2"`), 1), bytes.Replace(raw, []byte(`"UNKNOWN":"0"`), []byte(`"UNKNOWN":"0","EXTRA":"0"`), 1)} {
		if _, e := DecodeAttempt(bad); e == nil {
			t.Fatal("bad reason accounting accepted")
		}
	}
}
