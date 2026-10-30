//go:build darwin || linux

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func issue502Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ESC-V0-006: dispatch status loads a ledger whose seen.escalations sits
// beside progress history or a pool-sweep record (the members that switch
// LoadLedger to its strict allowlist) and still reports the hold.
func TestIssue502_DispatchStatusLoadsHoldBesideStrictMembers(t *testing.T) {
	root := t.TempDir()
	digest := strings.Repeat("a", 64)
	now := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	sweep := &dispatch.PoolSweepRecord{PoolSweepRequest: dispatch.PoolSweepRequest{
		WorkRoot: root, Program: "prog", Queue: "queue:a:q", Pool: "db", Member: "a", Allocation: digest, Definition: digest,
		RequestID: issue502Hex("prog\x00queue:a:q\x00" + digest), Actor: "tester", ActorRole: "OPERATOR", ConfigDigest: digest, TimeoutSeconds: 15,
	}, Phase: "TERMINAL", Started: now, Observed: now, Result: dispatch.PoolSweepResult{Outcome: "COMPLETED"}}
	for name, fill := range map[string]func(*dispatch.Ledger){
		"progress": func(l *dispatch.Ledger) {
			l.Progress = map[string]*dispatch.ProgressHistory{"ticket:a:q:t": {Current: digest, Seen: []string{digest}}}
		},
		"poolSweeps": func(l *dispatch.Ledger) {
			l.PoolSweeps = map[string]*dispatch.PoolSweepRecord{issue502Hex("queue:a:q\x00db\x00a"): sweep}
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(root, name)
			l := &dispatch.Ledger{Profile: dispatch.StateProfile, Program: "prog", Workers: []*dispatch.Worker{}, Backoff: map[string]*dispatch.BackoffState{},
				Seen: &dispatch.Seen{Tickets: map[string]string{}, Claims: map[string]string{}, Lanes: map[string]string{}, Escalations: map[string][]string{"ticket:a:q:t": {"q-a", "q-b"}}}}
			fill(l)
			raw, err := json.MarshalIndent(l, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "state.json"), append(raw, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			loaded, err := dispatch.LoadLedger(dir, "prog")
			if err != nil {
				t.Fatal("status cannot load the ledger:", err)
			}
			v, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, dir, loaded, nil, now), "escalationPending")
			if want := `[{"requests":["q-a","q-b"],"ticket":"ticket:a:q:t"}]`; !ok || string(wire.Encode(v)) != want {
				t.Fatalf("escalationPending = %s", wire.Encode(v))
			}
		})
	}
}
