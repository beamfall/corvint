//go:build darwin || linux

package wire

// PlatformRefusal returns nil: Darwin and Linux are the supported Tasks
// platforms (CTS-V0-025).
func PlatformRefusal() error { return nil }
