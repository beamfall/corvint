//go:build darwin

package authoritystore

import (
	"context"
	"testing"
)

func TestPiNativeParentAndCodePolicy(t *testing.T) {
	t.Run("PPI-V0-007 exact process and hardened code policy", func(t *testing.T) {
		for _, flags := range []uint32{0, 1, 0x10000, 0x10010001, 0x10005} {
			if validPiCodeFlags(flags) {
				t.Fatalf("unsafe code flags accepted: %x", flags)
			}
		}
		if !validPiCodeFlags(0x10001) {
			t.Fatal("hardened valid code refused")
		}
		pin := DirectRuntime(piFixture())
		child := processIdentity{PID: 500, Parent: pin.HostInstance.PID, Started: 99, Path: "/test-only-consumer"}
		host := processIdentity{PID: pin.HostInstance.PID, Parent: 100, Started: pin.HostInstance.Started, StartedUsec: pin.HostInstance.StartedUsec, Path: pin.HostImage.Path}
		for _, mode := range []string{"valid", "tool-child", "same-image-sibling", "birth", "mapped-image", "after-drift"} {
			images := 0
			inspect := func(pid uint32) (processIdentity, error) {
				if pid == child.PID {
					v := child
					if mode == "tool-child" || mode == "same-image-sibling" {
						v.Parent++
					}
					return v, nil
				}
				v := host
				if mode == "birth" || mode == "after-drift" && images == 2 {
					v.StartedUsec++
				}
				return v, nil
			}
			image := func(context.Context, uint32, Image, string) error {
				images++
				if mode == "mapped-image" {
					return errUnavailable
				}
				return nil
			}
			err := verifyImmediateParent(context.Background(), child.PID, child.Path, pin, inspect, image)
			if (err == nil) != (mode == "valid") {
				t.Fatalf("%s: %v", mode, err)
			}
		}
	})
}
