package intent

import (
	"bytes"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// HandoffPolicyCompatible is the narrow release-only projection. DecodePolicy
// validates both profiles, but its sorted Pools are not the comparison input:
// independently parsed wire trees preserve every array and optional member.
func HandoffPolicyCompatible(original, current []byte, pool, member string) (bool, error) {
	a, err := handoffPolicyProjection(original, pool, member)
	if err != nil {
		return false, err
	}
	b, err := handoffPolicyProjection(current, pool, member)
	if err != nil {
		return false, err
	}
	return bytes.Equal(wire.Encode(a), wire.Encode(b)), nil
}

func handoffPolicyProjection(raw []byte, pool, member string) (wire.Value, error) {
	if _, err := DecodePolicy(raw); err != nil {
		return wire.Value{}, err
	}
	v, err := wire.Parse(raw) // a fresh tree; never mutate a caller's policy
	if err != nil {
		return wire.Value{}, err
	}
	v.Obj = handoffWithout(v.Obj, "policyVersion")
	if pool == "" && member == "" {
		return v, nil
	}
	if _, err := wire.ParseLabel("handoff pool", pool); err != nil {
		return wire.Value{}, err
	}
	if _, err := wire.ParseLabel("handoff member", member); err != nil {
		return wire.Value{}, err
	}
	pools, _ := v.Obj.Get("pools")
	for _, p := range pools.Arr {
		id, _ := p.Obj.Get("id")
		if id.Str != pool {
			continue
		}
		members, _ := p.Obj.Get("members")
		found := false
		for _, m := range members.Arr {
			found = found || m.Str == member
		}
		if !found {
			return wire.Value{}, wire.Errorf(wire.CodeMalformed, "handoff member", "member is absent from allocated pool")
		}
		reservations, present := p.Obj.Get("reservedFor")
		own, hasOwn := wire.Value{}, false
		if present {
			own, hasOwn = reservations.Obj.Get(member)
		}
		// Only this pool normalizes an empty/absent map. Every own-member
		// reservation and every other pool/member/config byte stays bound.
		if hasOwn {
			p.Obj.Set("reservedFor", wire.ObjectValue(wire.NewObject().Set(member, own)))
		} else {
			without := handoffWithout(p.Obj, "reservedFor")
			*p.Obj = *without
		}
		return v, nil
	}
	return wire.Value{}, wire.Errorf(wire.CodeMalformed, "handoff pool", "allocated pool is absent")
}

func handoffWithout(o *wire.Object, key string) *wire.Object {
	copy := wire.NewObject()
	for _, name := range o.Keys {
		if name != key {
			copy.Set(name, o.Vals[name])
		}
	}
	return copy
}
