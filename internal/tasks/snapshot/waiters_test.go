package snapshot

import (
	"bytes"
	"strings"
	"testing"
)

func waiterState(t *testing.T, waiters ...PoolWaiter) *PoolState {
	t.Helper()
	st := sharedPoolState(t)
	st.Waiters = waiters
	return st
}

// PSR-V0-131: the waitlist is additive. A state without waiters encodes
// exactly as before; entries round-trip in pool-then-ticket order; a
// malformed, duplicate or foreign entry refuses.
func TestPSRV0131_WaitersCodec(t *testing.T) {
	legacy, err := waiterState(t).Encode()
	if err != nil || bytes.Contains(legacy, []byte(`"waiters"`)) {
		t.Fatalf("legacy pools %v %s", err, legacy)
	}
	b := PoolWaiter{PoolID: "db", TicketID: "ticket:acme:main:BB-01", Kind: WaiterGrace, Holder: "builder", Priority: "P2", Seq: "4", Since: "2026-09-06T00:00:00Z", ExpiresAt: "2026-09-06T00:02:00Z"}
	a := PoolWaiter{PoolID: "db", TicketID: "ticket:acme:main:AA-01", Kind: WaiterWait, AttemptID: testAttempt, Generation: "1", Holder: "builder", Priority: "P1", Seq: "3", Since: "2026-09-06T00:00:00Z", ExpiresAt: "2026-09-06T00:10:00Z"}
	raw, err := waiterState(t, b, a).Encode()
	if err != nil {
		t.Fatal(err)
	}
	p, err := DecodePools(raw)
	if err != nil || len(p.Waiters) != 2 || p.Waiters[0] != a || p.Waiters[1] != b {
		t.Fatalf("decode %v %+v", err, p)
	}
	if again, _ := p.Encode(); !bytes.Equal(raw, again) {
		t.Fatal("pools roundtrip")
	}
	if !WaiterBefore(b, a) {
		t.Fatal("GRACE does not lead WAIT")
	}
	for name, bad := range map[string]string{
		"empty":     strings.Replace(string(raw), string(raw[bytes.Index(raw, []byte(`"waiters":`)):len(raw)-2]), `"waiters":[]`, 1),
		"kind":      strings.Replace(string(raw), `"kind":"GRACE"`, `"kind":"HOLD"`, 1),
		"no-gen":    strings.Replace(string(raw), `,"generation":"1"`, ``, 1),
		"seq":       strings.Replace(string(raw), `"seq":"3"`, `"seq":"0"`, 1),
		"expiry":    strings.Replace(string(raw), `"expiresAt":"2026-09-06T00:10:00Z"`, `"expiresAt":"2026-09-06T00:00:00Z"`, 1),
		"duplicate": strings.Replace(string(raw), `"ticketId":"ticket:acme:main:BB-01"`, `"ticketId":"ticket:acme:main:AA-01"`, 1),
		"foreign":   strings.Replace(string(raw), `"attemptId":"`+testAttempt+`"`, `"attemptId":"attempt:other:main:57ddbdeca215924fd0ea543f91045dad"`, 1),
		"extra":     strings.Replace(string(raw), `"kind":"WAIT"`, `"kind":"WAIT","x":"1"`, 1),
		"priority":  strings.Replace(string(raw), `"priority":"P2"`, `"priority":"P9"`, 1),
	} {
		if bad == string(raw) {
			t.Fatalf("%s: substitution missed", name)
		}
		if _, err := DecodePools([]byte(bad)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}
