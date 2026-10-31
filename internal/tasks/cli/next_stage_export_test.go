package cli

import (
	"context"
	"io"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// ObserveDispatch is the dispatcher's native observation of the store at root.
func ObserveDispatch(root string) (*dispatch.Observation, error) {
	return dispatchQueue{env: Env{Cwd: root, Stdout: io.Discard, Stderr: io.Discard}}.Observe(context.Background())
}

// ObserveDispatchFor is the observation of a dispatcher running config c.
func ObserveDispatchFor(root string, c *dispatch.Config) (*dispatch.Observation, error) {
	return newDispatchQueue(Env{Cwd: root, Stdout: io.Discard, Stderr: io.Discard}, c).Observe(context.Background())
}
