//go:build darwin

package authoritystore

import (
	"context"
	"os"
	"runtime"
	"syscall"
)

func verifyDirectRuntime(ctx context.Context, root RootDocument, pin DirectRuntime) error {
	if root.Profile != DirectRootProfile || !pin.valid() || runtime.GOARCH != pin.Architecture || verifyProtectedRelease(ctx, root) != nil {
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
	return verifyDirectParent(ctx, uint32(os.Getpid()), root.Consumer.Path, pin, inspectProcess, verifyHostImage)
}

// Only production kernel readers reach this private seam. Both the child and
// parent must retain their full metadata around two mapped-code observations.
func verifyDirectParent(ctx context.Context, pid uint32, consumerPath string, pin DirectRuntime, inspect func(uint32) (processIdentity, error), image func(context.Context, uint32, Image, string) error) error {
	if !pin.valid() {
		return errUnavailable
	}
	return verifyImmediateParent(ctx, pid, consumerPath, pin, inspect, image)
}
func verifyImmediateParent(ctx context.Context, pid uint32, consumerPath string, pin DirectRuntime, inspect func(uint32) (processIdentity, error), image func(context.Context, uint32, Image, string) error) error {
	if !validNativePin(pin) || ctx.Err() != nil {
		return errUnavailable
	}
	child, err := inspect(pid)
	if err != nil || child.PID != pid || child.Path != consumerPath || child.Parent != pin.HostInstance.PID {
		return errUnavailable
	}
	host, err := inspect(child.Parent)
	if err != nil || host.PID != pin.HostInstance.PID || host.Started != pin.HostInstance.Started || host.StartedUsec != pin.HostInstance.StartedUsec || host.Path != pin.HostImage.Path {
		return errUnavailable
	}
	if image(ctx, host.PID, pin.HostImage, pin.HostCDHash) != nil {
		return errUnavailable
	}
	hostAfter, err := inspect(host.PID)
	if err != nil || hostAfter != host {
		return errUnavailable
	}
	childAfter, err := inspect(pid)
	if err != nil || childAfter != child || image(ctx, host.PID, pin.HostImage, pin.HostCDHash) != nil || ctx.Err() != nil {
		return errUnavailable
	}
	// Recheck after the last potentially blocking image read as well.
	hostFinal, eh := inspect(host.PID)
	childFinal, ec := inspect(pid)
	if eh != nil || ec != nil || hostFinal != host || childFinal != child {
		return errUnavailable
	}
	return nil
}
