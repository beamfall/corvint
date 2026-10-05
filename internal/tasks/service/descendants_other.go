//go:build !linux

package service

import "github.com/Beamfall/corvint/internal/tasks/wire"

// helperRuntime refuses: without a child subreaper this platform cannot
// prove that a helper's detached descendants were retired.
func helperRuntime() (helperSpawner, error) {
	return nil, wire.Errorf(wire.CodeUnsupported, "/helper", "the helper profile is unsupported on this platform: detached helper descendants cannot be proved retired")
}

// bootClock is unknown here; restart debt then holds rather than charges.
func bootClock() (string, uint64, bool) { return "", 0, false }
