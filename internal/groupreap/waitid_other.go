//go:build aix || dragonfly || freebsd || netbsd || openbsd || solaris

package groupreap

import "errors"

// waitidAvailable is false: these platforms are not 1.0 targets and keep the
// legacy order, signalling the group after the reap (V1-0652 limitation).
const waitidAvailable = false

// leaderUnreaped is unavailable here; Wait signals the group after reaping.
func leaderUnreaped(int) error { return errors.ErrUnsupported }
