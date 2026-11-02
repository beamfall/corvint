package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// gateBeatCeiling is the longest gap between a gate run's heartbeats; it
// matches attempt run's cadence (ATR-V0-002).
const gateBeatCeiling = 240 * time.Second

// gateBeatWriteBound bounds each heartbeat write a gate run makes.
const gateBeatWriteBound = 30 * time.Second

// gateBeatInterval is the gap between a gate run's heartbeats for a policy
// heartbeat TTL: a third of the TTL, at most gateBeatCeiling, so two missed
// beats still leave the holder fresh (LGL-V0-810). Tests shorten it.
var gateBeatInterval = func(ttlSeconds int64) time.Duration {
	d := time.Duration(ttlSeconds) * time.Second / 3
	if d <= 0 || d > gateBeatCeiling {
		return gateBeatCeiling
	}
	return d
}

// gateHeartbeat keeps a gate run's attempt holder fresh while its gate
// program runs (LGL-V0-810, V1-3941): each beat is an ordinary HEARTBEAT
// lease write by the gate run's actor at the attempt's generation, under its
// own request id and a short store-lock hold. The first beat that does not
// complete (fenced, released, a supervised attempt) or fails ends the
// beating; the gate run itself is unaffected. stop ends the beating and
// waits for any write in flight, so the gate result is recorded after it.
func gateHeartbeat(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, policy *intent.Policy, clock func() time.Time) (stop func() int) {
	every := gateBeatInterval(policy.HeartbeatTTLSeconds())
	sum := sha256.Sum256([]byte(choice.RequestID))
	prefix := "gate-beat-" + hex.EncodeToString(sum[:6]) + "-" + strconv.FormatInt(time.Now().UnixNano(), 36) + "-"
	done := make(chan struct{})
	var wg sync.WaitGroup
	beats := 0
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			if !gateBeat(ctx, repo, actor, choice, prefix+strconv.Itoa(beats+1), clock) {
				return
			}
			beats++
		}
	}()
	var once sync.Once
	return func() int {
		once.Do(func() { close(done) })
		wg.Wait()
		return beats
	}
}

// gateBeat writes one heartbeat and reports whether it completed.
func gateBeat(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, requestID string, clock func() time.Time) bool {
	now, err := wire.ParseTimestamp("recordedAt", clock().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return false
	}
	write, cancel := context.WithTimeout(ctx, gateBeatWriteBound)
	defer cancel()
	beat := LeaseChoice{QueueID: choice.QueueID, RequestID: requestID, Root: choice.Root, Derive: NoScopeDeriver,
		Lease: transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: choice.Lease.AttemptID, Generation: choice.Lease.Generation}}
	report, err := Lease(write, repo, actor, beat, now)
	return err == nil && report != nil && report.Outcome.Outcome == mutation.OutcomeCompleted
}
