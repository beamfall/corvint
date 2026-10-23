//go:build !darwin && !linux

package cli

import "testing"

// testMain has no process-group checks where the dispatcher cannot start
// worker groups.
func testMain(m *testing.M) int { return m.Run() }
