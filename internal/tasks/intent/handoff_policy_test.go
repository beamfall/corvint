package intent_test

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func handoffPolicyValue() wire.Value {
	v := fixture.PolicyValue()
	v.Obj.Set("pools", wire.Array(
		wire.ObjectValue(wire.NewObject().Set("id", wire.String("lanes")).Set("members", wire.Strings([]string{"one", "two"}))),
		wire.ObjectValue(wire.NewObject().Set("id", wire.String("other")).Set("members", wire.Strings([]string{"three"}))),
	))
	return v
}

func TestCALV0044_PolicyProjectionPreservesRawSemantics(t *testing.T) {
	for _, name := range []string{"version", "other-reservation", "own-reservation", "other-pool", "members-order", "pools-order", "empty-map", "capacity", "budget", "retry", "gate", "roles", "environment", "unknown"} {
		t.Run(name, func(t *testing.T) {
			original := wire.EncodeFile(handoffPolicyValue())
			v, _ := wire.Parse(original)
			v.Obj.Set("policyVersion", wire.String("2"))
			pools, _ := v.Obj.Get("pools")
			want := false
			switch name {
			case "version":
				want = true
			case "other-reservation":
				pools.Arr[0].Obj.Set("reservedFor", wire.ObjectValue(wire.NewObject().Set("two", wire.String("review"))))
				want = true
			case "own-reservation":
				pools.Arr[0].Obj.Set("reservedFor", wire.ObjectValue(wire.NewObject().Set("one", wire.String("review"))))
			case "other-pool":
				pools.Arr[1].Obj.Set("reservedFor", wire.ObjectValue(wire.NewObject().Set("three", wire.String("review"))))
			case "members-order":
				pools.Arr[0].Obj.Set("members", wire.Strings([]string{"two", "one"}))
			case "pools-order":
				pools.Arr[0], pools.Arr[1] = pools.Arr[1], pools.Arr[0]
			case "empty-map":
				pools.Arr[0].Obj.Set("reservedFor", wire.ObjectValue(wire.NewObject()))
				want = true
			case "capacity":
				x, _ := v.Obj.Get("capacity")
				x.Obj.Set("maxActiveAttempts", wire.String("2"))
			case "budget":
				x, _ := v.Obj.Get("budgets")
				x.Obj.Set("ticketMultiplier", wire.String("5"))
			case "retry":
				x, _ := v.Obj.Get("retries")
				x.Obj.Set("admissionsPerRevision", wire.String("4"))
			case "gate":
				x, _ := v.Obj.Get("gates")
				x.Arr[0].Obj.Set("required", wire.Bool(false))
			case "roles":
				v.Obj.Set("roles", wire.ObjectValue(wire.NewObject().Set("IMPORTER", wire.Strings([]string{"CREATE"}))))
			case "environment":
				x, _ := v.Obj.Get("environment")
				x.Obj.Set("allowedEnvKeys", wire.Strings([]string{"PATH"}))
			case "unknown":
				v.Obj.Set("unknown", wire.Bool(true))
			}
			current := wire.EncodeFile(v)
			beforeA, beforeB := append([]byte(nil), original...), append([]byte(nil), current...)
			got, err := intent.HandoffPolicyCompatible(original, current, "lanes", "one")
			if got != want || (want && err != nil) {
				t.Fatalf("compatible=%v error=%v, want %v", got, err, want)
			}
			if !bytes.Equal(original, beforeA) || !bytes.Equal(current, beforeB) {
				t.Fatal("comparison mutated input bytes")
			}
			if want && name != "version" {
				if noPool, _ := intent.HandoffPolicyCompatible(original, current, "", ""); noPool {
					t.Fatal("no-pool handoff ignored a pool change")
				}
			}
		})
	}
}

func handoffHealth(seconds string) wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("health", wire.ObjectValue(wire.NewObject().Set("argv", wire.Strings([]string{"true"})).Set("cwd", wire.String("REPOSITORY")).Set("env", wire.Strings([]string{})).Set("timeoutSeconds", wire.String(seconds)))))
}

// CAL-V0-122: a purely additive pool change (a new member, with only that
// member's own reservation and configuration) is handoff-compatible for pooled
// and no-pool attempts. Every other change to the allocated member, its pool's
// settings, other existing members, the pool set or budgets still fences.
func TestCALV0122_AdditivePoolMemberIsHandoffCompatible(t *testing.T) {
	cases := map[string]struct {
		change         func(v wire.Value, pools wire.Value)
		pooled, noPool bool
	}{
		"member-add": {func(_, p wire.Value) {
			p.Arr[0].Obj.Set("members", wire.Strings([]string{"lane-3", "one", "two"}))
		}, true, true},
		"member-add-last": {func(_, p wire.Value) {
			p.Arr[0].Obj.Set("members", wire.Strings([]string{"one", "two", "zz"}))
		}, true, true},
		"member-add-own-settings": {func(_, p wire.Value) {
			p.Arr[0].Obj.Set("members", wire.Strings([]string{"lane-3", "one", "two"}))
			p.Arr[0].Obj.Set("reservedFor", wire.ObjectValue(wire.NewObject().Set("lane-3", wire.String("review"))))
			p.Arr[0].Obj.Set("memberConfig", wire.ObjectValue(wire.NewObject().Set("lane-3", handoffHealth("5"))))
		}, true, true},
		"member-add-other-pool": {func(_, p wire.Value) {
			p.Arr[1].Obj.Set("members", wire.Strings([]string{"four", "three"}))
			p.Arr[1].Obj.Set("reservedFor", wire.ObjectValue(wire.NewObject().Set("four", wire.String("review"))))
		}, true, true},
		"own-member-removed": {func(_, p wire.Value) {
			p.Arr[0].Obj.Set("members", wire.Strings([]string{"lane-3", "two"}))
		}, false, false},
		"own-member-moved": {func(_, p wire.Value) {
			p.Arr[0].Obj.Set("members", wire.Strings([]string{"two"}))
			p.Arr[1].Obj.Set("members", wire.Strings([]string{"one", "three"}))
		}, false, false},
		"other-member-replaced": {func(_, p wire.Value) {
			p.Arr[0].Obj.Set("members", wire.Strings([]string{"lane-3", "one"}))
		}, false, false},
		"own-member-config": {func(_, p wire.Value) {
			p.Arr[0].Obj.Set("members", wire.Strings([]string{"lane-3", "one", "two"}))
			p.Arr[0].Obj.Set("memberConfig", wire.ObjectValue(wire.NewObject().Set("one", handoffHealth("5"))))
		}, false, false},
		"own-member-reservation": {func(_, p wire.Value) {
			p.Arr[0].Obj.Set("members", wire.Strings([]string{"lane-3", "one", "two"}))
			p.Arr[0].Obj.Set("reservedFor", wire.ObjectValue(wire.NewObject().Set("one", wire.String("implement"))))
		}, false, false},
		"pool-setting": {func(_, p wire.Value) {
			p.Arr[0].Obj.Set("members", wire.Strings([]string{"lane-3", "one", "two"}))
			p.Arr[0].Obj.Set("priorityAdmission", wire.Bool(true))
		}, false, false},
		"pool-added": {func(v, p wire.Value) {
			v.Obj.Set("pools", wire.Array(p.Arr[0], p.Arr[1], wire.ObjectValue(wire.NewObject().Set("id", wire.String("third")).Set("members", wire.Strings([]string{"lane-3"})))))
		}, false, false},
		"budget-with-member": {func(v, p wire.Value) {
			p.Arr[0].Obj.Set("members", wire.Strings([]string{"lane-3", "one", "two"}))
			x, _ := v.Obj.Get("budgets")
			x.Obj.Set("ticketMultiplier", wire.String("5"))
		}, false, false},
		"capacity-with-member": {func(v, p wire.Value) {
			p.Arr[0].Obj.Set("members", wire.Strings([]string{"lane-3", "one", "two"}))
			x, _ := v.Obj.Get("capacity")
			x.Obj.Set("maxActiveAttempts", wire.String("2"))
		}, false, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			original := wire.EncodeFile(handoffPolicyValue())
			v, _ := wire.Parse(original)
			v.Obj.Set("policyVersion", wire.String("2"))
			pools, _ := v.Obj.Get("pools")
			tc.change(v, pools)
			current := wire.EncodeFile(v)
			beforeA, beforeB := append([]byte(nil), original...), append([]byte(nil), current...)
			got, err := intent.HandoffPolicyCompatible(original, current, "lanes", "one")
			if err != nil || got != tc.pooled {
				t.Fatalf("pooled compatible=%v error=%v, want %v", got, err, tc.pooled)
			}
			if got, err := intent.HandoffPolicyCompatible(original, current, "", ""); err != nil || got != tc.noPool {
				t.Fatalf("no-pool compatible=%v error=%v, want %v", got, err, tc.noPool)
			}
			if !bytes.Equal(original, beforeA) || !bytes.Equal(current, beforeB) {
				t.Fatal("comparison mutated input bytes")
			}
		})
	}
	// The original policy alone attests the allocation; a missing original
	// member is malformed history, never compatibility.
	original := wire.EncodeFile(handoffPolicyValue())
	if got, err := intent.HandoffPolicyCompatible(original, original, "lanes", "lane-3"); got || wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("absent original member: %v %v", got, err)
	}
}

// CAL-V0-123: a change limited to the optional holderLiveness key (derived
// read-time holder status, CAL-V0-120/121) is handoff-compatible. A profile
// that does not define the key refuses it in DecodePolicy first.
func TestCALV0123_HolderLivenessIsHandoffCompatible(t *testing.T) {
	original := wire.EncodeFile(handoffPolicyValue())
	liveness := func(ttl string) wire.Value {
		return wire.ObjectValue(wire.NewObject().Set("heartbeatTTLSeconds", wire.String(ttl)))
	}
	v, _ := wire.Parse(original)
	v.Obj.Set("holderLiveness", liveness("900"))
	if _, err := intent.DecodePolicy(wire.EncodeFile(v)); err != nil {
		t.Skipf("this policy profile does not define holderLiveness (CAL-V0-120): %v", err)
	}
	for _, step := range []struct {
		name string
		set  func(v wire.Value)
		want bool
	}{
		{"added", func(v wire.Value) { v.Obj.Set("holderLiveness", liveness("900")) }, true},
		{"changed-and-member-added", func(v wire.Value) {
			v.Obj.Set("holderLiveness", liveness("1200"))
			pools, _ := v.Obj.Get("pools")
			pools.Arr[0].Obj.Set("members", wire.Strings([]string{"lane-3", "one", "two"}))
		}, true},
		{"with-budget", func(v wire.Value) {
			v.Obj.Set("holderLiveness", liveness("900"))
			x, _ := v.Obj.Get("budgets")
			x.Obj.Set("ticketMultiplier", wire.String("5"))
		}, false},
	} {
		v, _ := wire.Parse(original)
		v.Obj.Set("policyVersion", wire.String("2"))
		step.set(v)
		current := wire.EncodeFile(v)
		for _, alloc := range [][2]string{{"lanes", "one"}, {"", ""}} {
			if got, err := intent.HandoffPolicyCompatible(original, current, alloc[0], alloc[1]); err != nil || got != step.want {
				t.Fatalf("%s %v: compatible=%v error=%v", step.name, alloc, got, err)
			}
			// Removing the key is the same change in reverse.
			if got, err := intent.HandoffPolicyCompatible(current, original, alloc[0], alloc[1]); step.name == "added" && (err != nil || !got) {
				t.Fatalf("removed %v: compatible=%v error=%v", alloc, got, err)
			}
		}
	}
}
