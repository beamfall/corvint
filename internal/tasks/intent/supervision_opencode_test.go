package intent_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

func TestCALV0076_PolicyHostOpenCode(t *testing.T) {
	p, e := supervisionPolicy(t, `"host":"opencode",`)
	if e != nil {
		t.Fatal(e)
	}
	if p.Supervision.SupervisedHost() != intent.SupervisedHostOpenCode {
		t.Fatalf("host %q", p.Supervision.Host)
	}
	for _, host := range []string{`"OpenCode"`, `"open-code"`, `" opencode"`} {
		if _, e := supervisionPolicy(t, `"host":`+host+`,`); e == nil {
			t.Fatalf("host %s admitted", host)
		}
	}
}
