package cli_test

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestJSGV0013_JournalSealVerbs checks the operator verbs' envelopes: a
// store below the seal trigger seals nothing and unseal of an unsegmented
// store is a no-op, both OK; extra arguments, a bare `journal` and an
// unknown verb are usage errors with a null snapshot.
func TestJSGV0013_JournalSealVerbs(t *testing.T) {
	root, _ := leaseCLIStore(t, 1, time.Now().UTC())
	for _, verb := range []string{"seal", "unseal"} {
		x := atm(t, root, nil, "journal", verb)
		if x.code != 0 || len(x.res.Items) != 1 {
			t.Fatalf("journal %s: %s", verb, x.stdout)
		}
		item := x.res.Items[0]
		if field(item, "sealedThroughBefore").Str != "0" || field(item, "sealedThroughAfter").Str != "0" || field(item, "segments").Str != "0" || field(item, "headSeq").Str == "0" {
			t.Fatalf("journal %s: %s", verb, x.stdout)
		}
	}
	if x := atm(t, root, nil, "receipt", "audit"); x.code != 0 {
		t.Fatalf("audit after the verbs: %s", x.stdout)
	}
	for _, args := range [][]string{{"journal"}, {"journal", "compact"}, {"journal", "seal", "--all"}, {"journal", "unseal", "x"}} {
		x := atm(t, root, nil, args...)
		if x.res.Outcome != wire.OutcomeError || x.res.Snapshot != nil {
			t.Errorf("%v: %s", args, x.stdout)
		}
	}
}
