package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// CAL-V0-125: status reports the derived CPU utilization, UNKNOWN until a
// tick has a previous sample to compare with.
func TestCALV0125_DispatchStatusCPUUtilization(t *testing.T) {
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	l := &dispatch.Ledger{Program: "prog", Pressure: &dispatch.PressureRecord{
		Sample: dispatch.PressureSample{SampledAt: now, Source: "linux-proc-host", CPUUtilization: .625, CPUUtilizationKnown: true},
		Held:   []dispatch.HeldLaunch{},
	}}
	p := pressureField(t, dispatchStatusValue(&dispatch.Config{}, "/s", l, nil, now), "pressure")
	if got := pressureField(t, p, "cpuUtilization").Str; got != "0.625" {
		t.Fatalf("cpuUtilization = %q", got)
	}
	l.Pressure.Sample.CPUUtilizationKnown = false
	p = pressureField(t, dispatchStatusValue(&dispatch.Config{}, "/s", l, nil, now), "pressure")
	if got := pressureField(t, p, "cpuUtilization").Str; got != "UNKNOWN" {
		t.Fatalf("unknown cpuUtilization = %q", got)
	}
}

// CAL-V0-127: status reports this run's reload record, the refused change
// (UNKNOWN digest when unreadable) or NONE, and omits it before any change.
func TestCALV0127_DispatchStatusConfigRecord(t *testing.T) {
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	l := &dispatch.Ledger{Program: "prog"}
	if _, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, "/s", l, nil, now), "config"); ok {
		t.Fatal("status reported a config record before any change")
	}
	sum := strings.Repeat("b", 64)
	l.Config = &dispatch.ConfigRecord{AppliedSha256: sum, AppliedAt: now}
	v, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, "/s", l, nil, now), "config")
	if !ok || pressureField(t, v, "appliedSha256").Str != sum || pressureField(t, v, "appliedAt").Str != "2026-10-06T00:00:00Z" || pressureField(t, v, "refused").Str != "NONE" {
		t.Fatalf("config = %+v", v)
	}
	l.Config.Refused = &dispatch.ConfigRefusal{At: now, Reason: "read: gone"}
	v, _ = statusField(t, dispatchStatusValue(&dispatch.Config{}, "/s", l, nil, now), "config")
	r := pressureField(t, v, "refused")
	if pressureField(t, r, "sha256").Str != "UNKNOWN" || pressureField(t, r, "reason").Str != "read: gone" || pressureField(t, r, "at").Str != "2026-10-06T00:00:00Z" {
		t.Fatalf("refused = %+v", r)
	}
}
