package authority

import (
	"testing"
	"time"
)

// CAL-V0-111: an explicit caller wait replaces the §1 default up to
// MaxCallerLockWait; without one, the frozen 30-second bound applies.
func TestCALV0111_CallerWaitBound(t *testing.T) {
	for _, c := range []struct {
		opts LockOptions
		want time.Duration
	}{
		{LockOptions{}, DefaultLockWait},
		{LockOptions{Wait: 45 * time.Second}, MaxLockWait},
		{LockOptions{Wait: 5 * time.Second}, 5 * time.Second},
		{LockOptions{CallerWait: time.Second}, time.Second},
		{LockOptions{CallerWait: 45 * time.Second, Wait: time.Second}, 45 * time.Second},
		{LockOptions{CallerWait: 400 * time.Second}, MaxCallerLockWait},
	} {
		if got := c.opts.wait(); got != c.want {
			t.Errorf("%+v: wait %v, want %v", c.opts, got, c.want)
		}
	}
	if MaxLockWait != 30*time.Second || MaxCallerLockWait != 300*time.Second {
		t.Fatalf("bounds %v %v", MaxLockWait, MaxCallerLockWait)
	}
}
