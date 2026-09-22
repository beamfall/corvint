//go:build aix || dragonfly || freebsd || netbsd || openbsd || solaris

package gokernel

import "errors"

// waitLeaderUnreaped is unavailable here; waitGroupLeader signals after Wait.
func waitLeaderUnreaped(int) error { return errors.ErrUnsupported }
