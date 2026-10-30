//go:build darwin

package authoritystore

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

func verifyPiRuntime(ctx context.Context, root RootDocument, pin PiRuntime) error {
	if root.Profile != PiRootProfile || !pin.valid() || runtime.GOARCH != pin.Architecture || pin.HostImage.Path != filepath.Join(filepath.Dir(root.Consumer.Path), "pi-protected") || verifyProtectedRelease(ctx, root) != nil {
		return errUnavailable
	}
	build, err := syscall.Sysctl("kern.osversion")
	if err != nil || build != pin.OSBuild {
		return errUnavailable
	}
	boot, err := syscall.Sysctl("kern.bootsessionuuid")
	if err != nil || boot != pin.BootSessionUUID {
		return errUnavailable
	}
	image := func(ctx context.Context, pid uint32, image Image, cdhash string) error {
		if _, err := verifyImage(ctx, image); err != nil {
			return errUnavailable
		}
		flags := make([]byte, 4)
		if csopsRead(pid, 0, flags) != nil || !validPiCodeFlags(binary.LittleEndian.Uint32(flags)) {
			return errUnavailable
		}
		return verifyHostImage(ctx, pid, image, cdhash)
	}
	return verifyImmediateParent(ctx, uint32(os.Getpid()), root.Consumer.Path, DirectRuntime(pin), inspectProcess, image)
}

// CS_RUNTIME and CS_VALID are required; debugging and get-task-allow refuse.
// The admitted full image/CDHash binds the exact allow-jit-only entitlements.
func validPiCodeFlags(flags uint32) bool {
	return flags&0x00010001 == 0x00010001 && flags&(0x10000000|0x00000004) == 0
}
