// SPDX-License-Identifier: Apache-2.0

// Package barrierwarning observes stale Tasks barriers without taking a writer lock.
package barrierwarning

import (
	"fmt"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Message reports an advisory warning. Missing receipt evidence stays NOT_OBSERVED.
// The caller supplies its existing snapshot and policy; no full journal walk is needed.
func Message(repo *intent.Repository, policy *intent.Policy, snap *snapshot.Snapshot, now time.Time) string {
	if snap == nil || snap.Barrier == nil {
		return ""
	}
	limit := int64(3600)
	if policy.BarrierWarningSeconds != "" {
		limit = policy.BarrierWarningSeconds.Int()
	}
	since, err := time.Parse(time.RFC3339, string(snap.Barrier.Since))
	if err != nil {
		return ""
	}
	age := int64(now.Sub(since) / time.Second)
	if age <= limit {
		return ""
	}
	request := "NOT_OBSERVED"
	name, err := snapshot.ReceiptName(snap.Barrier.SinceSeq.Uint64())
	if err == nil {
		src := journal.Logical(journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.IntentRoot()})
		raw, err := src.Read("receipts/"+name, wire.MaxReceiptFileBytes)
		if err == nil {
			rc, err := snapshot.DecodeReceipt(raw)
			if err == nil && rc.Seq == snap.Barrier.SinceSeq && rc.RequestID != nil && rc.Kind == "PAUSE" {
				for _, post := range rc.Post {
					if post.Path == "barrier.json" && post.Sha256 != nil && *post.Sha256 == wire.Sum(snap.BarrierRaw) {
						request = *rc.RequestID
					}
				}
			}
		}
	}
	return fmt.Sprintf("STALE_BARRIER: pause request %s, age %ds (policy limit %ds), since %s; operator review required before unpause", request, age, limit, snap.Barrier.Since)
}
