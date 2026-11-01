package dogfoodflow

import (
	"bytes"
	"context"
	"testing"
)

// TestASDV0998FailPrintsDefaultRecovery: a FAIL without its own fix line gets
// the code's default recovery with BASE filled in; a call's own fix stays alone.
func TestASDV0998FailPrintsDefaultRecovery(t *testing.T) {
	base := "0123456789abcdef0123456789abcdef01234567"
	for _, test := range []struct {
		reason string
		lines  []string
		want   string
	}{
		{"dogfood-report-drift", nil, "dogfood-check: FAIL dogfood-report-drift\n  fix: the recorded evidence no longer matches this base and HEAD; rerun corvint dogfood change " + base + " on this HEAD, then rerun dogfood check\n"},
		{"cem-map-missing", []string{"  missing citations: x"}, "dogfood-check: FAIL cem-map-missing\n  missing citations: x\n  fix: bind the change first: commit the .corvint/change.cem.json the binding step wrote (docs/DOGFOOD.md section 4), then rerun corvint dogfood change " + base + " and dogfood check\n"},
		{"dogfood-report-drift", []string{"  fix: own"}, "dogfood-check: FAIL dogfood-report-drift\n  fix: own\n"},
		{"unmapped-code", nil, "dogfood-check: FAIL unmapped-code\n"},
	} {
		var stderr bytes.Buffer
		c := &check{flow: flow{ctx: context.Background(), prefix: "dogfood-check", stderr: &stderr, base: base}}
		code, err := boundary(c.ctx, func() int { c.fail(test.reason, test.lines...); return 0 }, func() {})
		if err != nil || code != 1 || stderr.String() != test.want {
			t.Errorf("%s %v: code=%d err=%v stderr=%q, want %q", test.reason, test.lines, code, err, stderr.String(), test.want)
		}
	}
	for reason, recovery := range failRecoveries {
		if recovery == "" || recoveryLine(reason, base, nil) == "" {
			t.Errorf("%s has no recovery", reason)
		}
	}
}
