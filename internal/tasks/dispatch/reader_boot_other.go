//go:build !darwin && !linux

package dispatch

// hostBootIdentity reports nothing on an unsupported platform, so no reboot
// proof is ever available there (CAL-V0-227).
func hostBootIdentity() (machine, boot string) { return "", "" }
