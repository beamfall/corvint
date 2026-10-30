package intent

import (
	"bytes"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// handoffIgnoredKeys are the top-level policy keys that never bind a live
// attempt's clean handoff. policyVersion changes on every update;
// holderLiveness (CAL-V0-120/121) sets only a derived read-time holder status
// that never fences (CAL-V0-123). A profile without a key rejects it in
// DecodePolicy before the projection runs, so naming it here grants nothing.
var handoffIgnoredKeys = []string{"policyVersion", "holderLiveness"}

// HandoffPolicyCompatible is the narrow release-only projection. DecodePolicy
// validates both profiles, but its sorted Pools are not the comparison input:
// independently parsed wire trees preserve every array and optional member.
// Only original is trusted to hold the allocation: a current policy that
// removed or moved the allocated member is incompatible, not malformed.
func HandoffPolicyCompatible(original, current []byte, pool, member string) (bool, error) {
	a, err := handoffPolicyTree(original)
	if err != nil {
		return false, err
	}
	b, err := handoffPolicyTree(current)
	if err != nil {
		return false, err
	}
	allocated := pool != "" || member != ""
	if allocated {
		if _, err := wire.ParseLabel("handoff pool", pool); err != nil {
			return false, err
		}
		if _, err := wire.ParseLabel("handoff member", member); err != nil {
			return false, err
		}
		if err := handoffAllocation(a, pool, member); err != nil {
			return false, err
		}
	}
	handoffWithoutAddedMembers(a, b)
	if allocated && handoffAllocation(b, pool, member) != nil {
		return false, nil
	}
	return bytes.Equal(wire.Encode(a), wire.Encode(b)), nil
}

func handoffPolicyTree(raw []byte) (wire.Value, error) {
	if _, err := DecodePolicy(raw); err != nil {
		return wire.Value{}, err
	}
	v, err := wire.Parse(raw) // a fresh tree; never mutate a caller's policy
	if err != nil {
		return wire.Value{}, err
	}
	for _, key := range handoffIgnoredKeys {
		v.Obj = handoffWithout(v.Obj, key)
	}
	return v, nil
}

// handoffAllocation normalizes the allocated pool's reservation map to the
// own member's entry. Only this pool normalizes an empty/absent map. Every
// own-member reservation and every other pool/member/config byte stays bound.
func handoffAllocation(v wire.Value, pool, member string) error {
	p := handoffPool(v, pool)
	if p == nil {
		return wire.Errorf(wire.CodeMalformed, "handoff pool", "allocated pool is absent")
	}
	members, _ := p.Obj.Get("members")
	found := false
	for _, m := range members.Arr {
		found = found || m.Str == member
	}
	if !found {
		return wire.Errorf(wire.CodeMalformed, "handoff member", "member is absent from allocated pool")
	}
	reservations, present := p.Obj.Get("reservedFor")
	own, hasOwn := wire.Value{}, false
	if present {
		own, hasOwn = reservations.Obj.Get(member)
	}
	if hasOwn {
		p.Obj.Set("reservedFor", wire.ObjectValue(wire.NewObject().Set(member, own)))
	} else {
		*p.Obj = *handoffWithout(p.Obj, "reservedFor")
	}
	return nil
}

// handoffWithoutAddedMembers removes, from current only, every member that a
// same-id original pool lacks, with that member's own reservedFor and
// memberConfig entries (CAL-V0-122). A map emptied by the removal is dropped
// only where the original pool omits it. Removed, reordered or reconfigured
// existing members, pool settings and added or removed pools stay bound. The
// v0 pool profile has no size limit, so no raised limit is normalized.
func handoffWithoutAddedMembers(original, current wire.Value) {
	pools, _ := current.Obj.Get("pools")
	for _, p := range pools.Arr {
		id, _ := p.Obj.Get("id")
		base := handoffPool(original, id.Str)
		if base == nil {
			continue
		}
		existing := map[string]bool{}
		baseMembers, _ := base.Obj.Get("members")
		for _, m := range baseMembers.Arr {
			existing[m.Str] = true
		}
		members, _ := p.Obj.Get("members")
		kept, added := []wire.Value{}, map[string]bool{}
		for _, m := range members.Arr {
			if existing[m.Str] {
				kept = append(kept, m)
			} else {
				added[m.Str] = true
			}
		}
		if len(added) == 0 {
			continue
		}
		p.Obj.Set("members", wire.Array(kept...))
		for _, field := range []string{"reservedFor", "memberConfig"} {
			entries, present := p.Obj.Get(field)
			if !present || entries.Kind != wire.KindObject {
				continue
			}
			left := wire.NewObject()
			for _, name := range entries.Obj.Keys {
				if !added[name] {
					left.Set(name, entries.Obj.Vals[name])
				}
			}
			if _, inBase := base.Obj.Get(field); len(left.Keys) == 0 && !inBase {
				*p.Obj = *handoffWithout(p.Obj, field)
			} else {
				p.Obj.Set(field, wire.ObjectValue(left))
			}
		}
	}
}

func handoffPool(v wire.Value, id string) *wire.Value {
	pools, _ := v.Obj.Get("pools")
	for i := range pools.Arr {
		if got, _ := pools.Arr[i].Obj.Get("id"); got.Str == id {
			return &pools.Arr[i]
		}
	}
	return nil
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
