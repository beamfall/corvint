//go:build darwin || linux

package dispatch

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// CAL-V0-109/110: the dispatcher's throttled event names the reason and the
// kernel memory level, and a restart keeps the recorded reason.
func TestCALV0110_ThrottledEventReportsReason(t *testing.T) {
	c := testConfig(t, "exit 0")
	p := issue497Config()
	p.TicksToChange = 1
	c.Pressure = &p
	q := &fakeQueue{}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	d.pressureSampler = (&pressureFeed{sample: darwinSample(1, MemoryPressureCritical)}).read
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	ev := throttled(t, d)
	if len(ev) != 1 || ev[0].Detail["level"] != "2" || ev[0].Detail["reason"] != "memory" || ev[0].Detail["memoryPressureLevel"] != "4" || ev[0].Detail["swapFraction"] != StateUnknown || !strings.Contains(ev[0].Message, "reason memory") || !strings.Contains(ev[0].Message, "memory pressure level 4") {
		t.Fatalf("throttled events %+v", ev)
	}
	// An UNKNOWN sample still names the reason and what it observed.
	unknown := darwinSample(1, MemoryPressureWarn)
	unknown.LoadKnown = false
	d.pressureSampler = (&pressureFeed{sample: unknown}).read
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	ev = throttled(t, d)
	if len(ev) != 2 || ev[1].Detail["sample"] != StateUnknown || ev[1].Detail["reason"] != "memory" || ev[1].Detail["memoryPressureLevel"] != "2" || !strings.Contains(ev[1].Message, "reason memory") || !strings.Contains(ev[1].Message, "load per CPU UNKNOWN, memory pressure level 2") {
		t.Fatalf("unknown throttled events %+v", ev)
	}
	d.pressureSampler = (&pressureFeed{sample: PressureSample{}}).read
	d.ledger.Pressure.State.Unknown = false
	d.pressureEmitted.Unknown = false
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if ev = throttled(t, d); len(ev) != 3 || !strings.Contains(ev[2].Message, "memory UNKNOWN") {
		t.Fatalf("fully unknown throttled events %+v", ev)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if st := d.ledger.Pressure.State; st.Level != 2 || !st.Unknown || !reflect.DeepEqual(st.Reason, []string{"memory"}) {
		t.Fatalf("restart state %+v", st)
	}
}

// CAL-V0-109/110: the ledger accepts a recorded reason and kernel memory
// level and refuses values outside their closed sets.
func TestCALV0110_LedgerReasonAndMemoryLevelValidated(t *testing.T) {
	dir := t.TempDir()
	write := func(pressure string) error {
		raw := `{"profile":"` + StateProfile + `","program":"prog","launchSeq":0,"eventSeq":0,"workers":[],"backoff":{},"progress":{},"pressure":` + pressure + `}`
		if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadLedger(dir, "prog")
		return err
	}
	valid := `{"state":{"level":2,"pendingLevel":2,"pendingTicks":0,"unknown":false,"reason":["load","memory"]},"sample":{"sampledAt":"2026-10-06T00:00:00Z","source":"darwin-sysctl-host","loadAverage":1,"cpus":1,"swapTotalBytes":0,"swapUsedBytes":0,"loadKnown":true,"cpuKnown":true,"swapKnown":false,"memoryPressureLevel":4,"memoryPressureKnown":true},"held":[]}`
	if err := write(valid); err != nil {
		t.Fatalf("valid record refused: %v", err)
	}
	for name, bad := range map[string]string{
		"reason name":     strings.Replace(valid, `"memory"]`, `"cpu"]`, 1),
		"reason order":    strings.Replace(valid, `["load","memory"]`, `["memory","load"]`, 1),
		"reason at calm":  strings.Replace(valid, `"level":2,"pendingLevel":2`, `"level":0,"pendingLevel":0`, 1),
		"memory level":    strings.Replace(valid, `"memoryPressureLevel":4`, `"memoryPressureLevel":3`, 1),
		"unflagged level": strings.Replace(valid, `,"memoryPressureKnown":true`, ``, 1),
	} {
		if err := write(bad); err == nil {
			t.Errorf("%s: invalid pressure record accepted", name)
		}
	}
}
