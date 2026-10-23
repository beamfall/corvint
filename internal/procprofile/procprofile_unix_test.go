//go:build unix

package procprofile

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// PRB-V0-006: a non-regular destination such as /dev/null is not truncated, so naming it never
// refuses or erases the other knob's existing profile.
func TestPRBV0006DeviceDestinationKeepsOtherProfile(t *testing.T) {
	mem := filepath.Join(t.TempDir(), "heap.pprof")
	if err := os.WriteFile(mem, []byte("old heap"), 0o600); err != nil {
		t.Fatal(err)
	}
	code := Run(context.Background(), lookupOf(map[string]string{"CORVINT_CPUPROFILE": os.DevNull, "CORVINT_MEMPROFILE": mem}), func(err error) { t.Fatal(err) }, func() int { return 4 })
	if code != 4 {
		t.Fatalf("status %d, want the body's 4", code)
	}
	requireProfile(t, mem)
}

// PRB-V0-007: the CPU profiler drops its writer's errors; a CPU profile write that fails (here a
// pipe whose reader is gone, where Close still succeeds) is reported as profile-write-failed and
// keeps the body's status.
func TestPRBV0007CPUWriteFailureReported(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	_ = reader.Close()
	var reported []string
	code := Run(context.Background(), lookupOf(map[string]string{"CORVINT_CPUPROFILE": "/dev/fd/" + strconv.Itoa(int(writer.Fd()))}), func(err error) { reported = append(reported, err.Error()) }, func() int { return 5 })
	if code != 5 || len(reported) != 1 || !strings.HasPrefix(reported[0], "profile-write-failed: CORVINT_CPUPROFILE") {
		t.Fatalf("code=%d reported=%q", code, reported)
	}
}
