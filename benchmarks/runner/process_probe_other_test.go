//go:build !unix

package main

// processGone has no signal-0 probe on this platform; the caller's polling
// loop keeps waiting until its own deadline instead of returning early.
func processGone(int) bool {
	return false
}
