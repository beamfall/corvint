package cli_test

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestCALV0236_SupersededReplayWarningLeavesResultBytes: after a release and
// a re-claim move the attempt to its next generation, replaying an earlier
// renew or heartbeat request ID prints the same result bytes it printed
// before the release, plus one advisory warning naming the current
// generation.
func TestCALV0236_SupersededReplayWarningLeavesResultBytes(t *testing.T) {
	// leaseCLIStore claims at base+10m, here a minute ago, with a five-minute lease.
	root, claims := leaseCLIStore(t, 1, time.Now().UTC().Add(-11*time.Minute))
	c := claims[0]
	generation := string(c.Generation)
	verbs := [][]string{
		{"renew", "--attempt", c.AttemptID, "--generation", generation, "--request-id", "renew-a", "--lease-minutes", "30"},
		{"attempt", "heartbeat", "--attempt", c.AttemptID, "--generation", generation, "--request-id", "heartbeat-a"},
	}
	before := make([][]byte, len(verbs))
	for i, args := range verbs {
		if r := handoffCLI(t, root, args...); r.code != 0 || field(r.res.Items[0], "replayed").Bool {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		r := handoffCLI(t, root, args...)
		if r.code != 0 || !field(r.res.Items[0], "replayed").Bool {
			t.Fatalf("%v replay: %s", args, r.stdout)
		}
		before[i] = r.stdout
	}
	if r := handoffCLI(t, root, "release", "--attempt", c.AttemptID, "--generation", generation, "--request-id", "release-a"); r.code != 0 {
		t.Fatalf("release: %s", r.stdout)
	}
	again := handoffCLI(t, root, "claim", c.Ticket, "--holder", "holder", "--lease-minutes", "5", "--scope", "src/a", "--request-id", "claim-again")
	current := field(again.res.Items[0], "generation").Str
	if again.code != 0 || current == generation || current == "" {
		t.Fatalf("re-claim: %s", again.stdout)
	}
	for i, args := range verbs {
		r := handoffCLI(t, root, args...)
		var advisory []string
		kept := r.res.Warnings[:0:0]
		for _, w := range r.res.Warnings {
			if strings.HasPrefix(w, "advisory, not authoritative") {
				advisory = append(advisory, w)
				continue
			}
			kept = append(kept, w)
		}
		if r.code != 0 || len(advisory) != 1 || !strings.Contains(advisory[0], "now at generation "+current) {
			t.Fatalf("%v superseded replay: %s", args, r.stdout)
		}
		r.res.Warnings = kept
		stripped, err := r.res.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(stripped, before[i]) {
			t.Fatalf("%v replayed result changed:\nbefore %s\nafter  %s", args, before[i], stripped)
		}
	}
	if note := field(handoffCLI(t, root, "renew", "--help", "--verbose").res.Items[0], "note").Str; !strings.Contains(note, "CAL-V0-236") {
		t.Fatalf("renew note lacks the superseded replay warning: %s", note)
	}
}
