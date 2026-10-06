package cli

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func pressureField(t *testing.T, v wire.Value, key string) wire.Value {
	t.Helper()
	got, ok := v.Obj.Get(key)
	if !ok {
		t.Fatalf("status lacks %q", key)
	}
	return got
}

// CAL-V0-068: status exposes the recorded level, the newest sample's
// observed inputs (UNKNOWN when unobserved), the active cap and held work,
// and omits the pressure object when pressure is not recorded.
func TestCALV0068_DispatchStatusPressure(t *testing.T) {
	now := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	pc := &dispatch.PressureConfig{LevelCaps: map[string]int{"1": 3, "2": 1}}
	l := &dispatch.Ledger{Program: "prog", Pressure: &dispatch.PressureRecord{
		State:  dispatch.PressureState{Level: 2, PendingLevel: 0, PendingTicks: 1},
		Sample: dispatch.PressureSample{SampledAt: now, Source: "sysctl", LoadAverage: 40, CPUs: 10, SwapTotalBytes: 1000, SwapUsedBytes: 950, LoadKnown: true, CPUKnown: true, SwapKnown: true},
		Held:   []dispatch.HeldLaunch{{Role: "impl", Key: "ticket:a:q:t2", Ticket: "ticket:a:q:t2"}},
	}}
	p := pressureField(t, dispatchStatusValue(&dispatch.Config{Pressure: pc}, "/s", l, nil, now), "pressure")
	for key, want := range map[string]string{"level": "2", "pendingLevel": "0", "pendingTicks": "1", "sample": "OBSERVED", "sampledAt": "2026-10-04T00:00:00Z", "source": "sysctl", "loadAverage": "40.000", "cpus": "10", "loadPerCpu": "4.000", "swapFraction": "0.950", "swapUsedBytes": "950", "swapTotalBytes": "1000", "cap": "1"} {
		if got := pressureField(t, p, key).Str; got != want {
			t.Errorf("%s = %q want %q", key, got, want)
		}
	}
	if held := pressureField(t, p, "held").Arr; len(held) != 1 || pressureField(t, held[0], "key").Str != "ticket:a:q:t2" || pressureField(t, held[0], "role").Str != "impl" {
		t.Fatalf("held = %+v", held)
	}
	l.Pressure.State = dispatch.PressureState{Level: 0, Unknown: true}
	l.Pressure.Sample = dispatch.PressureSample{Problems: []string{"swap unavailable"}}
	p = pressureField(t, dispatchStatusValue(&dispatch.Config{}, "/s", l, nil, now), "pressure")
	for key, want := range map[string]string{"sample": "UNKNOWN", "sampledAt": "UNKNOWN", "source": "UNKNOWN", "loadAverage": "UNKNOWN", "cpus": "UNKNOWN", "loadPerCpu": "UNKNOWN", "swapFraction": "UNKNOWN", "swapUsedBytes": "UNKNOWN", "cap": "NONE"} {
		if got := pressureField(t, p, key).Str; got != want {
			t.Errorf("unknown %s = %q want %q", key, got, want)
		}
	}
	if problems := pressureField(t, p, "problems").Arr; len(problems) != 1 || problems[0].Str != "swap unavailable" {
		t.Fatalf("problems = %+v", problems)
	}
	l.Pressure.State.Level = 2
	if got := pressureField(t, dispatchStatusValue(&dispatch.Config{}, "/s", l, nil, now), "pressure"); pressureField(t, got, "cap").Str != "UNKNOWN" {
		t.Fatal("throttled level without a configuration reported a cap")
	}
	l.Pressure.State = dispatch.PressureState{}
	if got := pressureField(t, dispatchStatusValue(&dispatch.Config{Pressure: pc}, "/s", l, nil, now), "pressure"); pressureField(t, got, "cap").Str != "NONE" {
		t.Fatal("calm level reported a cap")
	}
	l.Pressure = nil
	if _, ok := dispatchStatusValue(&dispatch.Config{Pressure: pc}, "/s", l, nil, now).Obj.Get("pressure"); ok {
		t.Fatal("status reported pressure without a record")
	}
}

// CAL-V0-109/110: status reports the recorded reason (NONE at level 0,
// UNKNOWN for a level recorded without one) and the kernel memory level.
func TestCALV0110_DispatchStatusReason(t *testing.T) {
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	pc := &dispatch.PressureConfig{LevelCaps: map[string]int{"1": 3, "2": 1}}
	l := &dispatch.Ledger{Program: "prog", Pressure: &dispatch.PressureRecord{
		State:  dispatch.PressureState{Level: 2, PendingLevel: 2, Reason: []string{"load", "memory"}},
		Sample: dispatch.PressureSample{SampledAt: now, Source: "darwin-sysctl-host", LoadAverage: 63.4, CPUs: 10, LoadKnown: true, CPUKnown: true, MemoryPressureLevel: dispatch.MemoryPressureCritical, MemoryPressureKnown: true},
		Held:   []dispatch.HeldLaunch{},
	}}
	status := func() wire.Value {
		return pressureField(t, dispatchStatusValue(&dispatch.Config{Pressure: pc}, "/s", l, nil, now), "pressure")
	}
	for key, want := range map[string]string{"reason": "load,memory", "memoryPressureLevel": "4", "swapFraction": "UNKNOWN", "swapUsedBytes": "UNKNOWN", "loadPerCpu": "6.340"} {
		if got := pressureField(t, status(), key).Str; got != want {
			t.Errorf("%s = %q want %q", key, got, want)
		}
	}
	l.Pressure.State.Reason = nil
	l.Pressure.Sample = dispatch.PressureSample{}
	if p := status(); pressureField(t, p, "reason").Str != "UNKNOWN" || pressureField(t, p, "memoryPressureLevel").Str != "UNKNOWN" {
		t.Fatal("unrecorded reason or memory level fabricated")
	}
	l.Pressure.State = dispatch.PressureState{}
	if got := pressureField(t, status(), "reason").Str; got != "NONE" {
		t.Fatalf("calm reason %q", got)
	}
}
