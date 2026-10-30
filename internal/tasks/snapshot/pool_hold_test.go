package snapshot

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PSR-V0-268: hold is an additive key that binds exactly a HELD entry, with
// expiresAt after heldAt, and a HELD entry carries no attempt.
func TestPSRV0268_HoldCodec(t *testing.T) {
	en := recoveryEntry("a", "HELD")
	en.Hold = &PoolHold{HeldAt: "2026-10-25T07:00:00Z", ExpiresAt: "2026-10-25T08:00:00Z"}
	raw, err := recoveryState(t, en).Encode()
	if err != nil {
		t.Fatal(err)
	}
	st, err := DecodePools(raw)
	if err != nil || st.Entries[0].Hold == nil || *st.Entries[0].Hold != *en.Hold {
		t.Fatalf("roundtrip %v", err)
	}
	if again, _ := st.Encode(); !bytes.Equal(raw, again) {
		t.Fatal("roundtrip is not byte-identical")
	}
	if legacy, _ := recoveryState(t, recoveryEntry("a", "QUARANTINED")).Encode(); bytes.Contains(legacy, []byte(`"hold"`)) {
		t.Fatal("legacy entry encodes a hold key")
	}
	if !en.Hold.Expired("2026-10-25T08:00:00Z") || en.Hold.Expired("2026-10-25T07:59:59Z") {
		t.Fatal("Expired disagrees with expiresAt")
	}
	for name, mutate := range map[string]func(string) string{
		"hold on QUARANTINED": func(s string) string { return strings.Replace(s, `"state":"HELD"`, `"state":"QUARANTINED"`, 1) },
		"HELD without hold": func(s string) string {
			return strings.Replace(s, `"hold":{"expiresAt":"2026-10-25T08:00:00Z","heldAt":"2026-10-25T07:00:00Z"},`, ``, 1)
		},
		"expiry not after": func(s string) string {
			return strings.Replace(s, `"expiresAt":"2026-10-25T08:00:00Z"`, `"expiresAt":"2026-10-25T07:00:00Z"`, 1)
		},
		"extra hold key": func(s string) string {
			return strings.Replace(s, `"heldAt":"2026-10-25T07:00:00Z"`, `"heldAt":"2026-10-25T07:00:00Z","x":"y"`, 1)
		},
		"HELD with attempt": func(s string) string {
			return strings.Replace(s, `"attemptId":""`, `"attemptId":"attempt:acme:main:0123456789abcdef0123456789abcdef"`, 1)
		},
	} {
		bad := mutate(string(raw))
		if bad == string(raw) {
			t.Fatalf("%s: mutation did not apply to %s", name, raw)
		}
		if _, err := DecodePools([]byte(bad)); wire.CodeOf(err) != wire.CodeMalformed {
			t.Errorf("%s: %v", name, err)
		}
	}
}
