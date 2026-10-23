//go:build linux

package dispatch

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// The clock numbers of clock_gettime(2) and clock_settime(2).
const (
	clockRealtime  = 0
	clockMonotonic = 1
	clockBoottime  = 7
)

// runtimeNanotime is the reading time.Now carries as its monotonic clock.
//
//go:linkname runtimeNanotime runtime.nanotime
func runtimeNanotime() int64

func clockNanos(tb testing.TB, clock uintptr) int64 {
	tb.Helper()
	var ts syscall.Timespec
	if _, _, e := syscall.Syscall(syscall.SYS_CLOCK_GETTIME, clock, uintptr(unsafe.Pointer(&ts)), 0); e != 0 {
		tb.Fatalf("clock_gettime(%d): %v", clock, e)
	}
	return ts.Nano()
}

const (
	timensChild = "CORVINT_TEST_TIMENS_CHILD"
	timensBase  = "CORVINT_TEST_TIMENS_BASE_NS"
)

// CAL-V0-362, V1-1634: on Linux the monotonic reading that the host-sleep
// detector (ATR-V0-063) subtracts from the wall clock is CLOCK_MONOTONIC,
// which does not count suspended time, and not CLOCK_BOOTTIME, which does.
// A real suspend is not available to a test, so the test re-executes itself
// in a time namespace whose CLOCK_BOOTTIME runs three hours ahead of
// CLOCK_MONOTONIC, the separation a suspend leaves, and requires the Go
// runtime's monotonic clock to agree with CLOCK_MONOTONIC and to be three
// hours further from CLOCK_BOOTTIME than the host's own clocks already are.
// A host that suspended before the test starts with CLOCK_BOOTTIME ahead of
// CLOCK_MONOTONIC by its earlier suspends; the namespace offset adds to that
// separation and does not erase it, so the parent measures it and the child
// expects the sum.
func TestCALV0362_GoMonotonicClockIsNotBoottime(t *testing.T) {
	const ahead = 3 * time.Hour
	if os.Getenv(timensChild) == "1" {
		base, err := strconv.ParseInt(os.Getenv(timensBase), 10, 64)
		if err != nil {
			t.Fatalf("%s: %v", timensBase, err)
		}
		mono, boot, rt := clockNanos(t, clockMonotonic), clockNanos(t, clockBoottime), runtimeNanotime()
		if d := time.Duration(rt - mono); d < -time.Second || d > time.Second {
			t.Fatalf("runtime monotonic %d differs from CLOCK_MONOTONIC %d by %v", rt, mono, d)
		}
		want := ahead + time.Duration(base)
		if d := time.Duration(boot - rt); d < want-time.Second || d > want+time.Second {
			t.Fatalf("CLOCK_BOOTTIME %d is %v from the runtime monotonic clock, want about %v (offset %v plus the host's existing separation %v)", boot, d, want, ahead, time.Duration(base))
		}
		fmt.Printf("TIMENS-OK CLOCK_MONOTONIC=%d CLOCK_BOOTTIME=%d runtime.nanotime=%d host-separation=%v\n", mono, boot, rt, time.Duration(base))
		return
	}
	unshare, err := exec.LookPath("unshare")
	if err != nil {
		t.Skip("unshare is not installed: " + err.Error())
	}
	// The separation BOOTTIME-MONOTONIC accumulated by earlier host suspends.
	base := clockNanos(t, clockBoottime) - clockNanos(t, clockMonotonic)
	cmd := exec.Command(unshare, "--time", "--boottime="+strconv.Itoa(int(ahead/time.Second)), "--", os.Args[0], "-test.run=^TestCALV0362_GoMonotonicClockIsNotBoottime$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), timensChild+"=1", timensBase+"="+strconv.FormatInt(base, 10))
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	switch {
	case strings.Contains(out.String(), "TIMENS-OK"):
		t.Log(strings.TrimSpace(out.String()[strings.Index(out.String(), "TIMENS-OK"):]))
	case strings.Contains(out.String(), "unshare failed") || strings.Contains(out.String(), "unrecognized option"):
		t.Skip("this host cannot create a time namespace: " + strings.TrimSpace(out.String()))
	default:
		t.Fatalf("time namespace child: %v\n%s", err, out.String())
	}
}

// stepWall moves CLOCK_REALTIME by d.
func stepWall(tb testing.TB, d time.Duration) {
	tb.Helper()
	var ts syscall.Timespec
	if _, _, e := syscall.Syscall(syscall.SYS_CLOCK_GETTIME, clockRealtime, uintptr(unsafe.Pointer(&ts)), 0); e != 0 {
		tb.Fatalf("clock_gettime: %v", e)
	}
	ts = syscall.NsecToTimespec(ts.Nano() + int64(d))
	if _, _, e := syscall.Syscall(syscall.SYS_CLOCK_SETTIME, clockRealtime, uintptr(unsafe.Pointer(&ts)), 0); e != 0 {
		tb.Fatalf("clock_settime(%v): %v", d, e)
	}
}

// CAL-V0-362, V1-1634: with the real clocks, a forward step of the wall clock
// that leaves the monotonic clock behind, the signature of a suspend, is
// recorded once as a gap of the step's size. It steps the host's wall clock
// and so is opt-in: CORVINT_TASKS_CLOCK_STEP=1 and CAP_SYS_TIME. The clock is
// stepped back by the same amount before the test returns.
func TestCALV0362_RealWallClockStepIsRecordedAsASuspendGap(t *testing.T) {
	if os.Getenv("CORVINT_TASKS_CLOCK_STEP") != "1" {
		t.Skip("set CORVINT_TASKS_CLOCK_STEP=1 to step the host's wall clock")
	}
	const step = 3 * time.Hour
	d, err := Open("clockstep", testConfig(t, "exit 0"), &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	d.noteHostSleep() // the first tick only reads the clocks
	before := time.Now()
	time.Sleep(50 * time.Millisecond)
	stepWall(t, step) // a refusal (no CAP_SYS_TIME) fails here with nothing stepped
	restored := false
	restore := func() {
		if !restored {
			restored = true
			stepWall(t, -step)
		}
	}
	defer restore()
	d.noteHostSleep()
	after := time.Now()
	restore()
	if wall, mono := after.Round(0).Sub(before.Round(0)), after.Sub(before); wall < step || mono > time.Minute {
		t.Fatalf("the step moved the wall clock by %v and the monotonic clock by %v", wall, mono)
	}
	alerts := sleptAlerts(t, d)
	if len(alerts) != 1 {
		t.Fatalf("%d suspend alerts, want one", len(alerts))
	}
	rec, err := ReadHostSleep(HostSleepPath(d.dir))
	if err != nil || len(rec.Gaps) != 1 || rec.Gaps[0].Seconds < int(step/time.Second)-5 || rec.Gaps[0].Seconds > int(step/time.Second)+5 {
		t.Fatalf("record %+v %v", rec, err)
	}
	t.Logf("gap recorded: %+v", rec.Gaps[0])
}
