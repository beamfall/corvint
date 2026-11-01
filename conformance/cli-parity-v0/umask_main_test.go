package main

import (
	"os"
	"testing"
)

// TestMain pins the process umask as main does, so the fixtures the manifest
// replay and the frozen-divergence digests create carry the same file modes
// under a group-writable umask such as 0002 as under 022 (V1-3251).
func TestMain(m *testing.M) {
	setProcessUmask()
	os.Exit(m.Run())
}
