//go:build darwin

package dispatch

import "syscall"

// hostBootIdentity reads the kernel's hardware UUID and the per-boot session
// UUID (CAL-V0-227). Either is empty when the kernel does not report it.
func hostBootIdentity() (machine, boot string) {
	machine, _ = syscall.Sysctl("kern.uuid")
	boot, _ = syscall.Sysctl("kern.bootsessionuuid")
	return machine, boot
}
