package authority_test

import (
	"context"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
)

func TestCALV0026_LockObserverReportsOnceAfterRelease(t *testing.T) {
	t.Run("CAL-V0-026 observed lock hold", func(t *testing.T) {
		fixtureRepo := fixture.TempRepo(t)
		repo, err := intent.Resolve(fixtureRepo.Root)
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		ctx := authority.WithLockObserver(context.Background(), func(held time.Duration) {
			calls++
			if held <= 0 {
				t.Fatal("nonpositive hold")
			}
			other, err := authority.AcquireLock(context.Background(), repo, authority.LockOptions{Wait: time.Second})
			if err != nil {
				t.Fatalf("callback ran before release: %v", err)
			}
			if err = other.Close(); err != nil {
				t.Fatal(err)
			}
		})
		lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if err = lock.Close(); err != nil {
			t.Fatal(err)
		}
		if err = lock.Close(); err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("observed %d releases", calls)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err = authority.AcquireLock(cancelled, repo, authority.LockOptions{}); err == nil {
			t.Fatal("cancelled acquisition succeeded")
		}
		if calls != 1 {
			t.Fatal("failed acquisition was observed")
		}
	})
}
