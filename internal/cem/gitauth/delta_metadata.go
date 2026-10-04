package gitauth

import (
	"bytes"
	"context"
)

// DeltaCommitMessage returns only the verified bounded commit's message bytes.
// Callers must project opaque keys and must never emit this untrusted prose.
func (r *Repository) DeltaCommitMessage(ctx context.Context, oid string) ([]byte, error) {
	body, err := r.boundedObject(ctx, oid, "commit", 1<<20)
	if err != nil {
		return nil, err
	}
	_, message, ok := bytes.Cut(body, []byte("\n\n"))
	if !ok {
		return nil, unavailable("commit metadata framing unavailable")
	}
	return message, nil
}
