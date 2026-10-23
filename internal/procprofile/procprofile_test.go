package procprofile

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func lookupOf(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func requireProfile(t *testing.T, path string) {
	t.Helper()
	if err := Verify(path); err != nil {
		t.Fatal(err)
	}
}

// PRB-V0-002: unset or empty knobs are off and create nothing.
func TestPRBV0002KnobsOffByDefault(t *testing.T) {
	dir := t.TempDir()
	for _, values := range []map[string]string{{}, {"CORVINT_CPUPROFILE": "", "CORVINT_MEMPROFILE": ""}, {"CPUPROFILE": filepath.Join(dir, "bare")}} {
		ran := false
		if code := Run(context.Background(), lookupOf(values), func(err error) { t.Fatal(err) }, func() int { ran = true; return 7 }); code != 7 || !ran {
			t.Fatalf("Run(%v) = %d ran=%t", values, code, ran)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("off knobs wrote %v", entries)
	}
}

// PRB-V0-001, PRB-V0-003: both knobs write a CPU and a heap pprof file and keep the body's status.
func TestPRBV0001BothKnobsWriteProfiles(t *testing.T) {
	dir := t.TempDir()
	cpu, mem := filepath.Join(dir, "cpu.pprof"), filepath.Join(dir, "heap.pprof")
	code := Run(context.Background(), lookupOf(map[string]string{"CORVINT_CPUPROFILE": cpu, "CORVINT_MEMPROFILE": mem}), func(err error) { t.Fatal(err) }, func() int {
		sink := make([][]byte, 0, 64)
		for i := range 64 {
			sink = append(sink, bytes.Repeat([]byte{byte(i)}, 4096))
		}
		_ = sink
		return 3
	})
	if code != 3 {
		t.Fatalf("status %d, want the body's 3", code)
	}
	requireProfile(t, cpu)
	requireProfile(t, mem)
}

// PRB-V0-005: {pid} expands to the process ID.
func TestPRBV0005PIDPlaceholder(t *testing.T) {
	dir := t.TempDir()
	Run(context.Background(), lookupOf(map[string]string{"CORVINT_MEMPROFILE": filepath.Join(dir, "heap-{pid}.pprof")}), func(err error) { t.Fatal(err) }, func() int { return 0 })
	requireProfile(t, filepath.Join(dir, "heap-"+strconv.Itoa(os.Getpid())+".pprof"))
}

// PRB-V0-006: a knob that cannot be honoured refuses with status 2 before the body runs.
func TestPRBV0006UnusableKnobRefusesBeforeBody(t *testing.T) {
	dir := t.TempDir()
	same := filepath.Join(dir, "same.pprof")
	for name, values := range map[string]map[string]string{
		"missing cpu dir": {"CORVINT_CPUPROFILE": filepath.Join(dir, "absent", "cpu.pprof")},
		"missing mem dir": {"CORVINT_MEMPROFILE": filepath.Join(dir, "absent", "heap.pprof")},
		"same file":       {"CORVINT_CPUPROFILE": same, "CORVINT_MEMPROFILE": same},
		"nul byte":        {"CORVINT_CPUPROFILE": "bad\x00path"},
	} {
		var reported []string
		ran := false
		code := Run(context.Background(), lookupOf(values), func(err error) { reported = append(reported, err.Error()) }, func() int { ran = true; return 0 })
		if code != 2 || ran || len(reported) != 1 || !strings.HasPrefix(reported[0], "profile-") {
			t.Fatalf("%s: code=%d ran=%t reported=%q", name, code, ran, reported)
		}
	}
	if _, err := os.Stat(same); err == nil {
		t.Fatal("same-file refusal still created the file")
	}
	// A later session can still profile: the refused one released the CPU profiler.
	cpu := filepath.Join(dir, "after.pprof")
	Run(context.Background(), lookupOf(map[string]string{"CORVINT_CPUPROFILE": cpu}), func(err error) { t.Fatal(err) }, func() int { return 0 })
	requireProfile(t, cpu)
}

// PRB-V0-004: ending the context (a caught SIGINT or SIGTERM) writes both profiles while the
// body is still running; the exit path's Stop does not write them again.
func TestPRBV0004ContextEndWritesProfilesBeforeExit(t *testing.T) {
	dir := t.TempDir()
	cpu, mem := filepath.Join(dir, "cpu.pprof"), filepath.Join(dir, "heap.pprof")
	session, err := Start(lookupOf(map[string]string{"CORVINT_CPUPROFILE": cpu, "CORVINT_MEMPROFILE": mem}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	session.StopOnDone(ctx)
	cancel()
	<-session.done
	requireProfile(t, cpu)
	requireProfile(t, mem)
	before, _ := os.ReadFile(mem)
	if err := session.Stop(); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(mem); !bytes.Equal(before, after) {
		t.Fatal("exit-path Stop rewrote the heap profile")
	}
}

// PRB-V0-007: a write failure at exit is reported as profile-write-failed.
func TestPRBV0007WriteFailureReported(t *testing.T) {
	session, err := Start(lookupOf(map[string]string{"CORVINT_MEMPROFILE": filepath.Join(t.TempDir(), "heap.pprof")}))
	if err != nil {
		t.Fatal(err)
	}
	_ = session.mem.Close() // the heap write at exit now fails
	if err := session.Stop(); err == nil || !strings.HasPrefix(err.Error(), "profile-write-failed: CORVINT_MEMPROFILE") {
		t.Fatalf("Stop() = %v", err)
	}
}

// PRB-V0-011: CRB-V0-018's exact runtime-settings list names both knobs.
func TestPRBV0011KnobsInExactSettingsList(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "specs", "corvint-rebrand-v0.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, rest, found := strings.Cut(string(data), "- `CRB-V0-018`:")
	paragraph, _, _ := strings.Cut(rest, "\n\n")
	paragraph = strings.Join(strings.Fields(paragraph), " ")
	for _, knob := range Knobs {
		if !found || !strings.Contains(paragraph, "`"+knob+"`") {
			t.Errorf("CRB-V0-018 does not list %s", knob)
		}
	}
}

// PRB-V0-006: a refusal leaves an existing profile file intact, removes a file it created, and
// catches two names that reach one file.
func TestPRBV0006RefusalWritesNothing(t *testing.T) {
	dir := t.TempDir()
	badCPU := filepath.Join(dir, "absent", "cpu.pprof")
	existing := filepath.Join(dir, "existing.pprof")
	if err := os.WriteFile(existing, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(dir, "fresh.pprof")
	for _, mem := range []string{existing, fresh} {
		if _, err := Start(lookupOf(map[string]string{"CORVINT_CPUPROFILE": badCPU, "CORVINT_MEMPROFILE": mem})); err == nil {
			t.Fatal("unwritable CPU path admitted")
		}
	}
	if data, _ := os.ReadFile(existing); string(data) != "keep" {
		t.Fatalf("refusal truncated an existing file: %q", data)
	}
	if _, err := os.Stat(fresh); !os.IsNotExist(err) {
		t.Fatalf("refusal left a created file behind: %v", err)
	}
	link := filepath.Join(dir, "link.pprof")
	if err := os.Symlink(existing, link); err != nil {
		t.Skip("no symlinks:", err)
	}
	_, err := Start(lookupOf(map[string]string{"CORVINT_CPUPROFILE": link, "CORVINT_MEMPROFILE": existing}))
	var knob *Error
	if !errors.As(err, &knob) || knob.Code != "profile-invalid" {
		t.Fatalf("two names for one file: %v", err)
	}
	if data, _ := os.ReadFile(existing); string(data) != "keep" {
		t.Fatalf("same-file refusal truncated the file: %q", data)
	}
}
