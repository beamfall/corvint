//go:build windows

package releasegate

import (
	"context"
	"errors"
)

// The standard library cannot assign a Windows Job Object, so descendant
// containment is unavailable. Refuse before launching Git rather than claim
// process safety under an incomplete platform implementation.
func runGitProcess(context.Context, gitIdentity, string, int, []byte, ...string) ([]byte, error) {
	return nil, errors.New("Windows Git process containment is unsupported")
}
