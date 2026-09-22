//go:build !unix

package main

import "testing"

// mustCreateFIFO has no equivalent on this platform: FIFOs are a unix
// concept, so the FIFO-admission assertion in
// TestInventoryProjectionAndExpectedFileProtection does not apply here.
func mustCreateFIFO(t *testing.T, _ string) {
	t.Skip("named pipes are not supported on this platform")
}
