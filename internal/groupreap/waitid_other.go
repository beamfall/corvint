//go:build aix || dragonfly || freebsd || netbsd || openbsd || solaris

package groupreap

import "errors"

// leaderUnreaped is unavailable here; Wait signals the group after reaping.
func leaderUnreaped(int) error { return errors.ErrUnsupported }

// liveTracking is false: Wait reaps before it could release a recorded group,
// so KillLive could signal a group whose leader was already reaped.
const liveTracking = false
