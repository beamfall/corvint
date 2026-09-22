package workqueue

import (
	"fmt"
	"testing"
)

func TestResolveStatePrecedence(t *testing.T) {
	t.Run("WQO-V0-014", func(t *testing.T) {
		// Literal truth table ordered by contradiction, failure, drift, partial, inability bits.
		want := []string{
			"VALIDATED_AT", "UNKNOWN", "PARTIAL", "PARTIAL",
			"STALE", "STALE", "STALE", "STALE",
			"UNKNOWN", "UNKNOWN", "UNKNOWN", "UNKNOWN",
			"UNKNOWN", "UNKNOWN", "UNKNOWN", "UNKNOWN",
			"CONFLICTED", "CONFLICTED", "CONFLICTED", "CONFLICTED",
			"CONFLICTED", "CONFLICTED", "CONFLICTED", "CONFLICTED",
			"CONFLICTED", "CONFLICTED", "CONFLICTED", "CONFLICTED",
			"CONFLICTED", "CONFLICTED", "CONFLICTED", "CONFLICTED",
		}
		for bits, state := range want {
			t.Run(fmt.Sprintf("%05b", bits), func(t *testing.T) {
				facts := StateFacts{IdentityContradiction: bits&16 != 0, MutationOrAdapterFailure: bits&8 != 0,
					Stale: bits&4 != 0, PositivePartial: bits&2 != 0, Unable: bits&1 != 0}
				if got := ResolveState(facts); got != state {
					t.Fatalf("ResolveState(%+v) = %s; want %s", facts, got, state)
				}
			})
		}
	})
}
