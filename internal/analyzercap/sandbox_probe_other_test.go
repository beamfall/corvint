//go:build !darwin

package analyzercap

import "testing"

func requireSandboxedPayload(*testing.T, string) {}
