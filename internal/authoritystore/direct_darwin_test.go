//go:build darwin

package authoritystore

import (
	"context"
	"testing"
)

func TestDirectImmediateParentKernelBracketing(t *testing.T) {
	t.Run("DCLI-V0-002 immediate parent kernel bracketing", func(t *testing.T) {
		pin := directFixture()
		child := processIdentity{PID: 500, Parent: pin.HostInstance.PID, Started: 11, StartedUsec: 22, Path: "/test-only-consumer"}
		host := processIdentity{PID: pin.HostInstance.PID, Parent: 100, Started: pin.HostInstance.Started, StartedUsec: pin.HostInstance.StartedUsec, Path: pin.HostImage.Path}
		for _, name := range []string{"valid", "sibling-identical-image", "extra-shell", "birth-reuse", "different-path", "child-reparent", "host-reparent", "host-birth-drift", "child-birth-drift", "mapped-image-first", "mapped-image-second", "post-image-drift"} {
			t.Run(name, func(t *testing.T) {
				reads := map[uint32]int{}
				images := 0
				inspect := func(pid uint32) (processIdentity, error) {
					reads[pid]++
					v := host
					if pid == child.PID {
						v = child
					}
					if pid == child.PID && (name == "sibling-identical-image" || name == "extra-shell") {
						v.Parent++
					}
					if pid == host.PID && name == "birth-reuse" {
						v.Started++
					}
					if pid == host.PID && name == "different-path" {
						v.Path += "-other"
					}
					if reads[pid] > 1 && pid == child.PID && name == "child-reparent" {
						v.Parent++
					}
					if reads[pid] > 1 && pid == host.PID && name == "host-reparent" {
						v.Parent++
					}
					if reads[pid] > 1 && pid == host.PID && name == "host-birth-drift" {
						v.StartedUsec++
					}
					if reads[pid] > 1 && pid == child.PID && name == "child-birth-drift" {
						v.StartedUsec++
					}
					if images == 2 && name == "post-image-drift" && pid == host.PID {
						v.Path += "-other"
					}
					return v, nil
				}
				image := func(ctx context.Context, pid uint32, img Image, hash string) error {
					images++
					if pid != host.PID || img != pin.HostImage || hash != pin.HostCDHash {
						t.Fatal("wrong kernel image binding")
					}
					if (name == "mapped-image-first" && images == 1) || (name == "mapped-image-second" && images == 2) {
						return errUnavailable
					}
					return nil
				}
				err := verifyDirectParent(context.Background(), child.PID, child.Path, pin, inspect, image)
				if (err == nil) != (name == "valid") {
					t.Fatalf("DCLI-V0-002/006 %s: %v", name, err)
				}
				if name == "valid" && (images != 2 || reads[child.PID] != 3 || reads[host.PID] != 3) {
					t.Fatal("incomplete bracketing")
				}
			})
		}
		// Same mapped image and lifetime is deliberately accepted: no exec epoch or
		// native event witness is asserted by these observations (DCLI-V0-003/004).

	})
}
