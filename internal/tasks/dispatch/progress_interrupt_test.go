//go:build darwin || linux

package dispatch

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"
)

// TestATRV0167_InterruptAfterProgressSaveKeepsAccounting cancels the tick
// inside the declared-progress save, after the ledger without the granted
// worker (or with the unparked key) is durable and before finish runs. The
// saved ledger already holds the reset ladder and the recovered
// infrastructure episode (V1-0769).
func TestATRV0167_InterruptAfterProgressSaveKeepsAccounting(t *testing.T) {
	for _, mode := range []string{"granted-worker", "unparked-key"} {
		t.Run(mode, func(t *testing.T) {
			c := ladderConfig(t)
			source := filepath.Join(c.WorkRoot, "states.json")
			c.WorkState = &WorkState{Kind: "command", Argv: []string{"/bin/cat", source}}
			q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t", "P1", 1)}}}
			d, err := Open("prog", c, q, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = d.Close() })
			key := "ticket:a:q:t"
			writeProgress(t, source, "A")
			if _, _, _, err := d.admitProgress(context.Background(), progressObservation("A"), nil); err != nil {
				t.Fatal(err)
			}
			d.ledger.Escalation = map[string]*EscalationState{key: {Streak: 2, Tiers: map[string]int{c.Roles[0].Name: 2}}}
			if mode == "granted-worker" {
				base := baseFingerprint(progressObservation(""), key)
				d.ledger.Workers = []*Worker{{ID: "ended", Key: key, Ticket: key, Role: c.Roles[0].Name,
					BaseFingerprint: base, ProgressDigest: progressDigest("A"), Fingerprint: progressFingerprint(base, progressDigest("A")), Started: time.Now()}}
				d.ledger.InfraRetry = map[string]*InfraEpisode{key: {AcceptanceRevision: "1", Sessions: 1, Charged: 1, Limit: 3, CooldownUntil: time.Now(), Launch: "ended", State: InfraRunning}}
			} else {
				parkProgress(t, d)
			}
			if err := d.ledger.save(d.dir); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			d.progressSave = func(l *Ledger, dir string) error {
				err := l.save(dir)
				if err == nil {
					cancel()
				}
				return err
			}
			writeProgress(t, source, "B")
			if err := d.Tick(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("tick: %v", err)
			}
			if len(eventsOf(t, d, "finished")) != 0 || len(eventsOf(t, d, "unparked")) != 0 {
				t.Fatal("the interrupt did not land between the progress save and finish")
			}
			l, err := LoadLedger(d.dir, d.Program)
			if err != nil {
				t.Fatal(err)
			}
			if len(l.Workers) != 0 || l.Backoff[key] != nil || l.Progress[key].Current != progressDigest("B") {
				t.Fatal("progress admission was not committed")
			}
			if e := l.Escalation[key]; e != nil {
				t.Fatalf("interrupted tick lost the progress accounting: ladder %+v survives", *e)
			}
			if e := l.InfraRetry[key]; mode == "granted-worker" && (e == nil || e.Launch != "" || e.State != InfraRecovered) {
				t.Fatalf("interrupted tick lost the infrastructure settlement: %+v", e)
			}
		})
	}
}
