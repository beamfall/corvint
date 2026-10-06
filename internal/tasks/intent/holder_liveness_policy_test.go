package intent_test

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func holderLivenessDef(ttl string) *wire.Object {
	return wire.NewObject().Set("heartbeatTTLSeconds", wire.String(ttl))
}

// TestCALV0120_PolicyHolderLivenessOptIn pins the optional holderLiveness
// policy key: omission keeps the canonical bytes and the 600-second default,
// a closed in-range definition sets the TTL, and anything else refuses.
func TestCALV0120_PolicyHolderLivenessOptIn(t *testing.T) {
	raw := fixture.PolicyBytes()
	p, err := intent.DecodePolicy(raw)
	if err != nil || p.HolderLiveness != nil || p.HeartbeatTTLSeconds() != intent.DefaultHeartbeatTTLSeconds || intent.DefaultHeartbeatTTLSeconds != 600 || !bytes.Equal(p.Raw, raw) || bytes.Contains(raw, []byte("holderLiveness")) {
		t.Fatalf("default policy: %v %+v", err, p)
	}
	decode := func(def wire.Value) (*intent.Policy, []byte, error) {
		v := fixture.PolicyValue()
		v.Obj.Set("holderLiveness", def)
		b := wire.EncodeFile(v)
		p, err := intent.DecodePolicy(b)
		return p, b, err
	}
	for _, ttl := range []string{"300", "1800", "86400"} {
		p, b, err := decode(wire.ObjectValue(holderLivenessDef(ttl)))
		if err != nil || p.HolderLiveness == nil || string(p.HolderLiveness.HeartbeatTTLSeconds) != ttl || !bytes.Equal(p.Raw, b) {
			t.Fatalf("ttl %s: %v %+v", ttl, err, p)
		}
		if got := wire.CountOf(p.HeartbeatTTLSeconds()); string(got) != ttl {
			t.Fatalf("ttl %s effective %s", ttl, got)
		}
	}
	for name, tc := range map[string]struct {
		def  wire.Value
		code string
	}{
		"below minimum":  {wire.ObjectValue(holderLivenessDef("299")), wire.CodeLimitExceeded},
		"zero":           {wire.ObjectValue(holderLivenessDef("0")), wire.CodeLimitExceeded},
		"above maximum":  {wire.ObjectValue(holderLivenessDef("86401")), wire.CodeLimitExceeded},
		"not a count":    {wire.ObjectValue(holderLivenessDef("ten")), wire.CodeMalformed},
		"unknown member": {wire.ObjectValue(holderLivenessDef("600").Set("autoRelease", wire.Bool(true))), wire.CodeMalformed},
		"null":           {wire.Null(), wire.CodeMalformed},
		"empty":          {wire.ObjectValue(wire.NewObject()), wire.CodeMalformed},
		"not an object":  {wire.String("600"), wire.CodeMalformed},
	} {
		if _, _, err := decode(tc.def); wire.CodeOf(err) != tc.code {
			t.Fatalf("%s: got %v, want %s", name, err, tc.code)
		}
	}
}
