package contextindex

import (
	"context"
	"time"
)

// Cancel deterministically during the index scan, not merely before invocation.
type localPromptCancelContext struct{ calls, cancelAt int }

func (ctx *localPromptCancelContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (ctx *localPromptCancelContext) Done() <-chan struct{}       { return nil }
func (ctx *localPromptCancelContext) Value(any) any               { return nil }
func (ctx *localPromptCancelContext) Err() error {
	ctx.calls++
	if ctx.calls >= ctx.cancelAt {
		return context.Canceled
	}
	return nil
}
