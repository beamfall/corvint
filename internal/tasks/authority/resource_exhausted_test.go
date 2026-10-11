//go:build darwin || linux

package authority

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-215: post-link completion failures have a committed effect even
// though the caller has not yet recorded the successful receipt publication.
func TestCALV0215_ReceiptLinkExhaustion(t *testing.T) {
	for _, errno := range []error{syscall.EMFILE, syscall.ENFILE} {
		for _, after := range []bool{false, true} {
			t.Run(errno.Error()+map[bool]string{false: "/before", true: "/after"}[after], func(t *testing.T) {
				inner, r := fixtureHarness(t)
				raw := []byte(`{"profile":"test-receipt"}` + "\n")
				// Authority publishes opaque bounded bytes; canonical encoding is proved first.
				if _, err := wire.Parse(raw); err != nil {
					t.Fatal(err)
				}
				stage := fixturePrepared(t, inner, fixtureReceipt, string(raw))
				session := &Session{inner: inner}
				point := "native:link"
				if after {
					point = "sync:receipts"
				}
				hit := 0
				inner.before = func(at string) error {
					if at == point {
						hit++
						return &os.SyscallError{Syscall: "open", Err: errno}
					}
					return nil
				}
				err := session.Link(&Stage{inner: stage}, Target{Role: RoleReceipt, Name: "000000000001.json"})
				inner.before = nil
				if hit != 1 || !errors.Is(err, errno) || wire.CodeOf(err) != wire.CodeResourceExhausted || wire.RetryForbidden(err) != after {
					t.Fatalf("hit=%d forbidden=%v err=%v", hit, wire.RetryForbidden(err), err)
				}
				path := filepath.Join(r.StateDir, "receipts", "000000000001.json")
				if after {
					fixtureBytes(t, path, string(raw))
				} else {
					fixtureAbsent(t, path)
				}
			})
		}
	}
}
