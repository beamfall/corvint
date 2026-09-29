//go:build !unix

package store

import (
	"context"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func poolRunnerIdentity(int) (string, error) {
	return "", wire.Errorf(wire.CodeUnsupported, "pool", "process identity unavailable")
}
func executePool(context.Context, *intent.PoolCommand, string, []string) (string, bool, wire.Digest) {
	return "UNKNOWN", false, wire.Sum(nil)
}
