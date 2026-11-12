package contextindex

import (
	"context"
)

// objectOnlyPinsKey marks a build whose sources come only from Git objects,
// never from a worktree copy that hashes to the same blob.
type objectOnlyPinsKey struct{}

func objectOnlyPins(ctx context.Context) bool {
	only, _ := ctx.Value(objectOnlyPinsKey{}).(bool)
	return only
}
