package contextindex

import (
	"context"
)

// ImportGraphRecord is one independently revision-bound graph-imports/1 record.
// An edge is a syntax observation, never a claim of runtime dependency closure.
type ImportGraphRecord struct {
	Schema   string         `json:"schema"`
	Revision string         `json:"revision"`
	Type     string         `json:"type"`
	From     string         `json:"from,omitempty"`
	To       string         `json:"to,omitempty"`
	Kind     string         `json:"kind,omitempty"`
	Evidence map[string]any `json:"evidence,omitempty"`
	Unknown  map[string]any `json:"unknown,omitempty"`
}

// objectOnlyPinsKey marks a build whose sources come only from Git objects,
// never from a worktree copy that hashes to the same blob.
type objectOnlyPinsKey struct{}

func objectOnlyPins(ctx context.Context) bool {
	only, _ := ctx.Value(objectOnlyPinsKey{}).(bool)
	return only
}
