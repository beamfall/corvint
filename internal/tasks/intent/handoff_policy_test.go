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
