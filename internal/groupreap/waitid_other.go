//go:build aix || dragonfly || freebsd || netbsd || openbsd || solaris

package groupreap

import "errors"

// leaderUnreaped is unavailable here; Wait signals the group after reaping.
func leaderUnreaped(int) error { return errors.ErrUnsupported }
