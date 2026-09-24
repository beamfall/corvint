package golang

import "testing"

// SetMaxPathTokens lowers the per-package path token bound for one test.
func SetMaxPathTokens(t testing.TB, bound int) {
	previous := maxPathTokens
	maxPathTokens = bound
	t.Cleanup(func() { maxPathTokens = previous })
}
