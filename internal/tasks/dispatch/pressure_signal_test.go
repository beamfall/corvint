package dispatch

import (
	"reflect"
	"strings"
	"testing"
)

// darwinSample is a Darwin-shaped sample: load and the kernel memory-pressure
// level, no swap.
func darwinSample(load float64, memory int) PressureSample {
	return PressureSample{Source: "darwin-sysctl-host", LoadAverage: load * 10, CPUs: 10, LoadKnown: true, CPUKnown: true, MemoryPressureLevel: memory, MemoryPressureKnown: true}
}

// CAL-V0-109 (issue 636): a sticky high swap sample with a normal kernel
// memory-pressure level no longer holds the level; it releases after dwell.
func TestCALV0109_StickySwapWithNormalPressureReleases(t *testing.T) {
	c := issue497Config()
	sticky := issue497Sample(1, .95) // swap far above swapCritical
	sticky.MemoryPressureLevel, sticky.MemoryPressureKnown = MemoryPressureNormal, true
	s := PressureState{Level: 2, PendingLevel: 2, Reason: []string{PressureSignalSwap}}
	s = issue497Step(t, c, s, sticky)
	if s.Level != 2 || s.PendingLevel != 0 || s.PendingTicks != 1 || s.Unknown {
		t.Fatalf("first calm tick: %+v", s)
	}
	s = issue497Step(t, c, s, sticky)
	if s.Level != 0 || s.Reason != nil {
		t.Fatalf("sticky swap with normal pressure kept the level: %+v", s)
	}
	// The same swap without a kernel level is still the Linux signal.
	s = issue497Step(t, c, PressureState{Level: 2, PendingLevel: 2}, issue497Sample(1, .95))
	if s.Level != 2 || s.PendingTicks != 0 {
		t.Fatalf("linux swap signal changed: %+v", s)
	}
}

// CAL-V0-109: normal is calm, warn targets level 1 and critical level 2; an
// invalid or unflagged level is UNKNOWN and never falls back to swap.
func TestCALV0109_KernelMemoryLevelMapping(t *testing.T) {
	c := issue497Config()
	c.TicksToChange = 1
	for _, row := range []struct {
		load         float64
		memory, from int
		want         int
	}{{1, MemoryPressureNormal, 2, 0}, {1, MemoryPressureWarn, 0, 1}, {1, MemoryPressureCritical, 0, 2}, {1, MemoryPressureWarn, 2, 2}, {4, MemoryPressureNormal, 2, 2}, {8, MemoryPressureNormal, 0, 2}} {
		s := issue497Step(t, c, PressureState{Level: row.from, PendingLevel: row.from}, darwinSample(row.load, row.memory))
		if s.Level != row.want || s.Unknown {
			t.Fatalf("%+v: %+v", row, s)
		}
	}
	for _, sample := range []PressureSample{darwinSample(1, 3), darwinSample(1, 0), func() PressureSample {
		x := issue497Sample(1, .1)
		x.MemoryPressureLevel = MemoryPressureNormal // level without the known flag
		return x
	}(), func() PressureSample {
		x := darwinSample(1, 8)
		x.SwapKnown, x.SwapTotalBytes = true, 100 // calm swap must not stand in
		return x
	}()} {
		s := issue497Step(t, c, PressureState{Level: 2, PendingLevel: 2}, sample)
		if s.Level != 2 || !s.Unknown {
			t.Fatalf("invalid memory level fabricated a signal: %+v from %+v", s, sample)
		}
	}
	// Bounding keeps an invalid level UNKNOWN rather than exposing swap.
	bad := issue497Sample(1, .1)
	bad.MemoryPressureLevel, bad.MemoryPressureKnown = 3, true
	bounded := boundPressureSample(bad)
	if bounded.MemoryPressureKnown || bounded.MemoryPressureLevel != 0 || bounded.SwapKnown || len(bounded.Problems) != 1 {
		t.Fatalf("bounded invalid level %+v", bounded)
	}
	if s := issue497Step(t, c, PressureState{Level: 2, PendingLevel: 2}, bounded); !s.Unknown {
		t.Fatalf("bounded invalid level became known: %+v", s)
	}
}

// CAL-V0-109: the Darwin sysctl output parses load, CPUs and the kernel
// level; it carries no swap, and an unexpected or missing field is UNKNOWN.
func TestCALV0109_DarwinSysctlParsing(t *testing.T) {
	s := parseDarwinPressure(PressureSample{Source: "darwin-sysctl-host"}, []byte("vm.loadavg: { 63.40 50.00 40.00 }\nkern.memorystatus_vm_pressure_level: 2\nhw.logicalcpu: 10\n"))
	if load, ok := s.LoadPerCPU(); !ok || load != 6.34 {
		t.Fatalf("load %v %v", load, ok)
	}
	if level, ok := s.MemoryPressure(); !ok || level != MemoryPressureWarn || s.SwapKnown || len(s.Problems) != 0 {
		t.Fatalf("memory %+v", s)
	}
	for _, raw := range []string{"0", "3", "5", "", "1 2", "-1", "NaN", "0x1"} {
		if _, err := parseDarwinMemoryPressure([]byte(raw)); err == nil {
			t.Fatalf("invalid level %q accepted", raw)
		}
	}
	missing := parseDarwinPressure(PressureSample{}, []byte("vm.loadavg: { 1.00 1.00 1.00 }\nhw.logicalcpu: 10\n"))
	if missing.MemoryPressureKnown || !missing.LoadKnown || len(missing.Problems) != 1 || !strings.HasPrefix(missing.Problems[0], "memory: ") {
		t.Fatalf("missing level %+v", missing)
	}
	if _, ok := pressureSignals(issue497Config(), missing); ok {
		t.Fatal("missing level produced a memory signal")
	}
	for _, raw := range []string{
		"vm.loadavg: { 1.00 1.00 1.00 }\nvm.swapusage: total = 1.00M used = 0.50M free = 0.50M\nhw.logicalcpu: 10",
		"vm.loadavg: { 1.00 1.00 1.00 }\nkern.memorystatus_vm_pressure_level: 1\nkern.memorystatus_vm_pressure_level: 1\nhw.logicalcpu: 10",
	} {
		if bad := parseDarwinPressure(PressureSample{}, []byte(raw)); bad.LoadKnown || bad.MemoryPressureKnown || len(bad.Problems) != 1 {
			t.Fatalf("unexpected sysctl output accepted: %+v", bad)
		}
	}
}

// CAL-V0-110: the level records the signals that set it, keeps them while
// held or UNKNOWN, and clears them on release.
func TestCALV0110_ReasonRecordedAtLevelChange(t *testing.T) {
	c := issue497Config()
	c.TicksToChange = 1
	step := func(s PressureState, sample PressureSample) PressureState { return issue497Step(t, c, s, sample) }
	for _, row := range []struct {
		from   int
		sample PressureSample
		level  int
		reason []string
	}{
		{0, issue497Sample(5, .1), 1, []string{"load"}},
		{0, issue497Sample(1, .85), 1, []string{"swap"}},
		{0, issue497Sample(5, .85), 1, []string{"load", "swap"}},
		{0, issue497Sample(9, .85), 2, []string{"load"}},
		{1, issue497Sample(5, .95), 2, []string{"swap"}},
		{0, darwinSample(1, MemoryPressureWarn), 1, []string{"memory"}},
		{0, darwinSample(9, MemoryPressureCritical), 2, []string{"load", "memory"}},
		{1, darwinSample(9, MemoryPressureWarn), 2, []string{"load"}},
	} {
		s := step(PressureState{Level: row.from, PendingLevel: row.from, Reason: []string{"load"}}, row.sample)
		if s.Level != row.level || !reflect.DeepEqual(s.Reason, row.reason) || !ValidPressureReason(s.Level, s.Reason) {
			t.Fatalf("%+v: %+v", row, s)
		}
	}
	s := step(PressureState{}, darwinSample(9, MemoryPressureNormal))
	// Held at 2 by a lower signal and through an UNKNOWN sample: reason kept.
	s = step(s, darwinSample(1, MemoryPressureWarn))
	s = step(s, PressureSample{})
	if s.Level != 2 || !s.Unknown || !reflect.DeepEqual(s.Reason, []string{"load"}) || PressureReasonText(s) != "load" {
		t.Fatalf("held reason %+v", s)
	}
	s = step(s, darwinSample(1, MemoryPressureNormal))
	if s.Level != 0 || s.Reason != nil || PressureReasonText(s) != "NONE" {
		t.Fatalf("release reason %+v", s)
	}
	if PressureReasonText(PressureState{Level: 2}) != StateUnknown {
		t.Fatal("a level without a recorded reason must be UNKNOWN")
	}
	for _, bad := range [][]string{{"cpu"}, {"swap", "load"}, {"load", "load"}} {
		if ValidPressureReason(2, bad) {
			t.Fatalf("invalid reason %v accepted", bad)
		}
	}
	if ValidPressureReason(0, []string{"load"}) {
		t.Fatal("reason at level 0 accepted")
	}
}
