//go:build darwin || linux

package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestMain keeps the work source scratch these tests acquire under the test
// temporary root instead of the fixed /tmp (V1-2768).
func TestMain(m *testing.M) {
	removeRunParent, err := testsupport.PrivateWorkRunParent()
	if err != nil {
		fmt.Fprintln(os.Stderr, "work run parent:", err)
		os.Exit(1)
	}
	code := m.Run()
	if err := removeRunParent(); err != nil {
		fmt.Fprintln(os.Stderr, "remove work run parent:", err)
		code = 1
	}
	os.Exit(code)
}
