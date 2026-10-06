//go:build !unix

package store

import (
	"context"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func poolRunnerIdentity(int) (string, error) {
	return "", wire.Errorf(wire.CodeUnsupported, "pool", "process identity unavailable")
}
func executePoolGuarded(context.Context, *intent.PoolCommand, string, []string, func() error) (string, bool, wire.Digest) {
	return "UNKNOWN", false, wire.Sum(nil)
}

type poolCommandResult struct {
	Timing         snapshot.PoolSweepTiming
	Signal         *wire.Count
	Class          string
	Clean          bool
	Exit           *wire.Count
	Stdout, Stderr []byte
}

func poolRunnerIdentityContext(context.Context, int) (string, error) {
	return "", wire.Errorf(wire.CodeUnsupported, "pool", "process identity unavailable")
}
func executePoolCapturedGuarded(context.Context, *intent.PoolCommand, string, []string, func() error) poolCommandResult {
	return poolCommandResult{Class: "UNKNOWN"}
}
