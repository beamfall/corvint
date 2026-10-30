//go:build windows

package releasegate

import "errors"

// Scan returns its structured Windows UNSUPPORTED result before policy access.
// Keep the helper fail-closed if it is ever reached through a future path.
func readPinnedPolicy(string, int) ([]byte, error) {
	return nil, errors.New("external policy descriptor containment is unsupported on Windows")
}
