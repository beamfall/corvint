//go:build linux

package store

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PSR-V0-204 (V1-2425): on Linux the pool runner identity comes from
// /proc/<pid>/stat and /proc/stat without a process listing. Recovery
// re-reads a `linux:` record from /proc and treats a btime one second off
// (another process's read, V1-1091) as the same runner, a btime further off
// with the same start tick (a clock step) as unknown, and another start tick
// as a different runner; a listed record keeps the listing, and a failed /proc read
// falls back to the listing at preparation but stays an error at recovery.
func TestPSRV0204_LinuxRunnerIdentityNeedsNoListing(t *testing.T) {
	original, originalNative := poolProbe, poolNativeIdentity
	t.Cleanup(func() { poolProbe, poolNativeIdentity = original, originalNative })
	listings := 0
	poolProbe = func(ctx context.Context, argv ...string) ([]byte, error) {
		listings++
		if len(argv) > 2 && argv[1] == "-p" && strings.HasPrefix(argv[len(argv)-1], "lstart") {
			return []byte("Thu Oct 22 09:00:00 2026\n"), nil
		}
		return nil, context.DeadlineExceeded
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	identity, e := poolRunnerIdentityContext(ctx, os.Getpid())
	if e != nil || !strings.HasPrefix(identity, "linux:") || len(identity) > 128 || listings != 0 {
		t.Fatalf("identity %q %v after %d listings, want a /proc identity and none", identity, e, listings)
	}
	if again, e := poolRecordedIdentity(os.Getpid(), identity); e != nil || again != identity || listings != 0 {
		t.Fatalf("recorded /proc identity re-read as %q %v after %d listings", again, e, listings)
	}
	boot, start, ok := supervisor.SplitLinuxIdentity(identity)
	if !ok {
		t.Fatalf("identity %q does not parse", identity)
	}
	shifted := func(d int64, tick string) string {
		return "linux:" + strconv.FormatUint(uint64(int64(boot)+d), 10) + ":" + tick
	}
	for _, d := range []int64{-1, 1} {
		if again, e := poolRecordedIdentity(os.Getpid(), shifted(d, start)); e != nil || again != shifted(d, start) {
			t.Fatalf("a record with btime %+d re-read as %q %v, want the same runner", d, again, e)
		}
	}
	if again, e := poolRecordedIdentity(os.Getpid(), shifted(0, start+"0")); e != nil || again == shifted(0, start+"0") || again == "" {
		t.Fatalf("another start tick re-read as %q %v, want a different runner", again, e)
	}
	// A clock step shifts btime while the runner lives: the same start tick
	// with btime further off is unknown, never gone (fail closed).
	for _, d := range []int64{-2, 2, 3600} {
		again, e := poolRecordedIdentity(os.Getpid(), shifted(d, start))
		if e == nil || wire.CodeOf(e) != wire.CodeQuiescenceUnproved || again != "" {
			t.Fatalf("a record with btime %+d re-read as %q %v, want an unknown runner, not gone", d, again, e)
		}
	}
	if listings != 0 {
		t.Fatalf("%d listings for /proc records", listings)
	}
	if legacy, e := poolRecordedIdentity(os.Getpid(), "Thu Oct 22 09:00:00 2026"); e != nil || legacy != "Thu Oct 22 09:00:00 2026" || listings != 1 {
		t.Fatalf("listed record re-read as %q %v after %d listings, want the listing", legacy, e, listings)
	}
	if gone, e := poolRunnerIdentityContext(ctx, 1<<30); e != nil || gone != "" {
		t.Fatalf("absent process identity %q %v, want none", gone, e)
	}
	if gone, e := poolRecordedIdentity(1<<30, identity); e != nil || gone != "" {
		t.Fatalf("absent recorded runner %q %v, want none", gone, e)
	}
	poolNativeIdentity = func(int) (string, error) { return "", errors.New("proc read failed") }
	if listed, e := poolRunnerIdentityContext(ctx, os.Getpid()); e != nil || listed != "Thu Oct 22 09:00:00 2026" || listings != 2 {
		t.Fatalf("a failed /proc read at preparation gave %q %v after %d listings, want the listing", listed, e, listings)
	}
	if _, e := poolRecordedIdentity(os.Getpid(), identity); e == nil {
		t.Fatal("a failed /proc read at recovery made the runner readable or gone")
	}
}
