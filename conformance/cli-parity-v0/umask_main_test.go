package main

import (
	"os"
	"testing"
)

// TestMain pins the process umask exactly as main does, so tests that call
// replay directly digest the same file modes as the CLI under any caller
// umask, such as WSL's default 0002 (V1-3251).
func TestMain(m *testing.M) {
	setProcessUmask()
	os.Exit(m.Run())
}
