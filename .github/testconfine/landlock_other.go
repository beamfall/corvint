//go:build !linux

package testconfine

import "errors"

// ABI reports no Landlock outside Linux.
func ABI() int { return 0 }

// ExecConfined cannot confine outside Linux, so it refuses.
func ExecConfined([]Rule, string, []string, []string) error {
	return errors.New("landlock is unavailable outside linux")
}
