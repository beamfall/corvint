package intent_test

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"testing"
)

func TestCALV0045_RetryPolicyBounds(t *testing.T) {
	t.Run("CAL-V0-045 retry policy bounds", func(t *testing.T) {
		for _, n := range []int{0, 1, 3, 4, 16, 17} {
			t.Run(fmt.Sprint(n), func(t *testing.T) {
				v := fixture.PolicyValue()
				retries, _ := v.Obj.Get("retries")
				retries.Obj.Set("admissionsPerRevision", wire.String(fmt.Sprint(n)))
				p, e := intent.DecodePolicy(wire.EncodeFile(v))
				if n > 16 {
					if e == nil {
						t.Fatal("accepted above bounded maximum")
					}
					return
				}
				if e != nil || p.AdmissionsPerRevision.Int() != int64(n) {
					t.Fatalf("policy %d: %v", n, e)
				}
			})
		}

	})
}
