//go:build darwin || linux

package supervisor

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestCALV0137_HostWaitIdleCPU is the opt-in V1-0911 measurement of the CPU a
// supervisor spends waiting on an idle host: CORVINT_SUPERVISOR_IDLE_SECONDS
// seconds (a plain `go test` skips it) for each of a normal host and the
// detached OpenCode host whose escape scan runs every escapeScanEvery
// (CAL-V0-136, CAL-V0-137). The waiting loop runs in this process, so its
// cost is this process's user plus system time over the window, reported per
// minute; the host is `sleep`, which spends none. A measurement is accepted
// only when the host outlives the window, the deadline is what stopped it,
// and the stop was clean with the detached server gone.
func TestCALV0137_HostWaitIdleCPU(t *testing.T) {
	seconds, err := strconv.Atoi(os.Getenv("CORVINT_SUPERVISOR_IDLE_SECONDS"))
	if err != nil || seconds < 1 {
		t.Skip("set CORVINT_SUPERVISOR_IDLE_SECONDS to the window to measure")
	}
	cpu := func() time.Duration {
		var ru syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
			t.Fatal(err)
		}
		return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
	}
	window := time.Duration(seconds) * time.Second
	// The fixture outlives the window by ten minutes, so only the deadline
	// ends it.
	lifetime := "sleep " + strconv.Itoa(seconds+600)
	if !strings.Contains(detachedServer, "sleep 300") {
		t.Fatal("the detached server fixture no longer sleeps 300 seconds")
	}
	for _, mode := range []string{"normal", "detached"} {
		t.Run(mode, func(t *testing.T) {
			var c Capsule
			var pidf string
			if mode == "detached" {
				c, pidf = detachedCapsule(t, strings.Replace(detachedServer, "sleep 300", lifetime, 1)+" & wait")
			} else {
				c, _ = detachedCapsule(t, lifetime)
				c.Host = ""
			}
			ctx, cancel := context.WithTimeout(context.Background(), window)
			defer cancel()
			start, before := time.Now(), cpu()
			out, err := Run(ctx, os.Args[0], t.TempDir(), c, func(string, Boot, *Outcome) error { return nil })
			wall, used := time.Since(start), cpu()-before
			if !errors.Is(err, context.DeadlineExceeded) || out.Class != "INTERRUPTED" {
				t.Fatalf("the run ended as %s (%v) after %v, not at the %v deadline", out.Class, err, wall, window)
			}
			if wall < window {
				t.Fatalf("the run ended after %v, before the %v window", wall, window)
			}
			if !out.Clean {
				t.Fatalf("the stop at the deadline was not clean: %+v", out)
			}
			if pidf != "" && !serverGone(t, pidf) {
				t.Fatal("the detached server survived the stop")
			}
			t.Logf("%s host: %s over %v wall: %.3f CPU s per minute (class %s)", mode, used, wall.Round(time.Millisecond), used.Seconds()/wall.Minutes(), out.Class)
		})
	}
}

// BenchmarkCALV0136_ProcRowsOfThisHost is the per-scan cost of the detached
// host escape scan (escapeScanEvery) on this host's process table: the
// reader that CAL-V0-136 selects, which is /proc on Linux and the kernel
// table on Darwin.
func BenchmarkCALV0136_ProcRowsOfThisHost(b *testing.B) {
	rows, err := processRows()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportMetric(float64(len(rows)), "processes")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := processRows(); err != nil {
			b.Fatal(err)
		}
	}
}
