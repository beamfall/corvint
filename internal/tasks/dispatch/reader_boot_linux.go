//go:build linux

package dispatch

import (
	"os"
	"strings"
)

// hostBootIdentity reads the machine ID and the kernel's random per-boot ID
// (CAL-V0-227). Either is empty when it cannot be read.
func hostBootIdentity() (machine, boot string) {
	read := func(path string) string {
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) > 256 {
			return ""
		}
		return strings.TrimSpace(string(raw))
	}
	return read("/etc/machine-id"), read("/proc/sys/kernel/random/boot_id")
}
