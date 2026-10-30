package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// PoolClaim is the ATR-V0-120 pre-launch claim: the native `claim --pool`
// of the assignment's ticket for the worker, with its admission and waitlist
// order (PSR-V0-132), returning the attempt and the member it allocated
// (CAL-V0-204).
func (q dispatchQueue) PoolClaim(ctx context.Context, a dispatch.Assignment, holder string, leaseMinutes int, requestID string) (dispatch.PoolHold, error) {
	args := []string{a.Ticket, "--holder", holder, "--pool", a.Pool, "--request-id", requestID}
	if a.NextStage != "" {
		args = append(args, "--stage", a.NextStage)
	}
	if leaseMinutes > 0 {
		args = append(args, "--lease-minutes", strconv.Itoa(leaseMinutes))
	}
	r := leaseCommand(q.quiet(), "claim", args)
	if err := leaseOutcome(r); err != nil {
		return dispatch.PoolHold{}, err
	}
	if len(r.Items) == 0 || r.Items[0].Obj == nil {
		return dispatch.PoolHold{}, fmt.Errorf("claim result has no item")
	}
	item := r.Items[0].Obj
	attempt, _ := item.Get("attemptId")
	generation, _ := item.Get("generation")
	alloc, _ := item.Get("poolAllocation")
	if alloc.Obj == nil {
		return dispatch.PoolHold{}, fmt.Errorf("claim of %s with pool %s allocated no member", a.Ticket, a.Pool)
	}
	member, _ := alloc.Obj.Get("memberId")
	id, _ := alloc.Obj.Get("allocationId")
	return dispatch.PoolHold{Attempt: attempt.Str, Generation: generation.Str, Member: member.Str, Allocation: id.Str}, nil
}

// PoolRelease is the ATR-V0-122 return of the exact allocation the
// dispatcher's claim took (CAL-V0-201).
func (q dispatchQueue) PoolRelease(ctx context.Context, a dispatch.Attempt, allocation, requestID string) error {
	return leaseOutcome(leaseCommand(q.quiet(), "pool release", []string{"--attempt", a.ID, "--generation", a.Generation, "--allocation", allocation, "--request-id", requestID}))
}
