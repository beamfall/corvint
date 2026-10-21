package cli_test

import (
	"testing"
	"time"
)

// JSG-V0-002: a liveness write does not observe the intent tree, so its
// result reports no intent digest.
func TestJSGV0002_LivenessResultReportsNoIntentDigest(t *testing.T) {
	root, claims := leaseCLIStore(t, 1, time.Now().UTC().Add(-10*time.Minute))
	c := claims[0]
	for i, verb := range [][]string{{"attempt", "heartbeat"}, {"renew", "--lease-minutes", "30"}} {
		args := append(verb, "--attempt", c.AttemptID, "--generation", string(c.Generation), "--request-id", []string{"jsg-heartbeat", "jsg-renew"}[i])
		x := atm(t, root, nil, args...)
		if x.code != 0 {
			t.Fatalf("%v: %s", verb, x.stdout)
		}
		if s := x.res.Snapshot; s != nil && s.IntentTreeSha256 != nil {
			t.Fatalf("%v reported intent digest %s", verb, *s.IntentTreeSha256)
		}
	}
}
