//go:build linux

package procgroup

import "sync/atomic"

// graceHeldQuiet is the pre-reap grace probe's second route to quiescence on
// Linux (V1-2399, PGO-V0-022). There signal 0 to the group keeps succeeding
// while the exited leader is held unreaped as a zombie, so a group that obeyed
// SIGTERM would read live until the grace expired. Once leaderHeld reports the
// leader's exit observed, with the leader still held unreaped by run's waitid
// goroutine, quietProof (groupreap.HeldGroupQuiet: the /proc zombie-only
// proof) may prove the group quiet instead. A proof failure proves nothing.
func graceHeldQuiet(leaderHeld *atomic.Bool, quietProof func(int) (bool, error)) func(int) bool {
	return func(pid int) bool {
		if !leaderHeld.Load() {
			return false
		}
		quiet, err := quietProof(pid)
		return err == nil && quiet
	}
}
