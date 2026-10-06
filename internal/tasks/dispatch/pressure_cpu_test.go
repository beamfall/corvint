//go:build darwin || linux

package dispatch

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func cpuTicks(source string, busy, total uint64) PressureSample {
	return PressureSample{Source: source, CPUBusyTicks: busy, CPUTotalTicks: total, CPUTicksKnown: true}
}

// cpuConfig selects cpu on Linux with 0.5/0.8/0.95 thresholds.
func cpuConfig() PressureConfig {
	c := issue497Config()
	c.CalmCPU, c.CPUHigh, c.CPUCritical = .5, .8, .95
	return c
}

// CAL-V0-125: utilization is the busy/total tick delta since the previous
// sample; the first sample, a source change and counters that do not advance
// consistently stay UNKNOWN.
func TestCALV0125_CPUUtilizationDelta(t *testing.T) {
	got := withCPUUtilization(cpuTicks("linux-proc-host", 100, 1000), cpuTicks("linux-proc-host", 400, 1500))
	if x, ok := got.CPUUtilizationFraction(); !ok || x != .6 || len(got.Problems) != 0 {
		t.Fatalf("delta = %v %v %+v", x, ok, got)
	}
	for name, tc := range map[string]struct {
		prev, cur PressureSample
		problem   string
	}{
		"first sample":     {PressureSample{}, cpuTicks("linux-proc-host", 400, 1500), "no previous tick counters"},
		"source changed":   {cpuTicks("other", 100, 1000), cpuTicks("linux-proc-host", 400, 1500), "no previous tick counters"},
		"no advance":       {cpuTicks("s", 100, 1000), cpuTicks("s", 100, 1000), "did not advance"},
		"total regressed":  {cpuTicks("s", 100, 1000), cpuTicks("s", 100, 900), "did not advance"},
		"busy regressed":   {cpuTicks("s", 500, 1000), cpuTicks("s", 400, 1500), "did not advance"},
		"busy over total":  {cpuTicks("s", 100, 1000), cpuTicks("s", 1600, 1500), "did not advance"},
		"busy delta large": {cpuTicks("s", 0, 1000), cpuTicks("s", 900, 1500), "did not advance"},
		"current unknown":  {cpuTicks("s", 100, 1000), PressureSample{Source: "s", CPUUtilization: .4, CPUUtilizationKnown: true}, ""},
	} {
		got := withCPUUtilization(tc.prev, tc.cur)
		if _, ok := got.CPUUtilizationFraction(); ok || got.CPUUtilizationKnown || got.CPUUtilization != 0 {
			t.Errorf("%s: utilization known %+v", name, got)
		}
		if tc.problem == "" && len(got.Problems) != 0 || tc.problem != "" && (len(got.Problems) != 1 || !strings.Contains(got.Problems[0], tc.problem)) {
			t.Errorf("%s: problems %q", name, got.Problems)
		}
	}
}

// CAL-V0-125: Linux ticks come from the aggregate line of the /proc/stat
// read that already counts CPUs.
func TestCALV0125_LinuxCPUTicksParsing(t *testing.T) {
	stat := "cpu  10 20 30 400 50 6 7 8 90 100\ncpu0 5 10 15 200 25 3 3 4 45 50\nintr 1\n"
	busy, total, err := parseLinuxCPUTicks([]byte(stat))
	if err != nil || total != 531 || busy != 81 {
		t.Fatalf("ticks = %d %d %v", busy, total, err)
	}
	for name, raw := range map[string]string{
		"missing":   "cpu0 1 2 3 4 5 6 7 8\n",
		"duplicate": "cpu 1 2 3 4 5 6 7 8\ncpu 1 2 3 4 5 6 7 8\n",
		"short":     "cpu 1 2 3 4 5 6 7\n",
		"invalid":   "cpu 1 2 x 4 5 6 7 8\n",
		"overflow":  "cpu 18446744073709551615 1 0 0 0 0 0 0\n",
	} {
		if _, _, err := parseLinuxCPUTicks([]byte(raw)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

// CAL-V0-126: a per-OS selection replaces the default signals; an OS
// without an entry keeps the default, so Linux behavior is unchanged.
func TestCALV0126_SignalSelection(t *testing.T) {
	c := cpuConfig()
	hotLoad := darwinSample(9, MemoryPressureNormal)
	step := func(c PressureConfig, goos string, s PressureState, sample PressureSample) PressureState {
		t.Helper()
		next, err := stepPressure(c, goos, s, sample)
		if err != nil {
			t.Fatal(err)
		}
		return next
	}
	// Default Darwin: an inflated load raises pressure.
	s := step(c, "darwin", step(c, "darwin", PressureState{}, hotLoad), hotLoad)
	if s.Level != 2 || !reflect.DeepEqual(s.Reason, []string{"load"}) {
		t.Fatalf("default darwin %+v", s)
	}
	// Memory only on Darwin: the same load no longer participates, and an
	// UNKNOWN load does not make the sample UNKNOWN.
	c.Signals = map[string][]string{"darwin": {"memory"}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	unknownLoad := hotLoad
	unknownLoad.LoadKnown = false
	for _, sample := range []PressureSample{hotLoad, unknownLoad} {
		if s := step(c, "darwin", PressureState{}, sample); s.Level != 0 || s.PendingTicks != 0 || s.Unknown {
			t.Fatalf("memory-only darwin %+v", s)
		}
	}
	crit := darwinSample(0, MemoryPressureCritical)
	if s := step(c, "darwin", step(c, "darwin", PressureState{}, crit), crit); s.Level != 2 || !reflect.DeepEqual(s.Reason, []string{"memory"}) {
		t.Fatalf("memory-only darwin critical %+v", s)
	}
	// Linux without an entry: load plus swap, ticks ignored.
	linux := issue497Sample(1, .1)
	linux.CPUUtilization, linux.CPUUtilizationKnown = .99, true
	if s := step(c, "linux", PressureState{}, linux); s.Level != 0 || s.PendingTicks != 0 || s.Unknown {
		t.Fatalf("default linux %+v", s)
	}
	// Linux selecting cpu: utilization sets the level and the reason.
	c.Signals["linux"] = []string{"cpu", "swap"}
	s = step(c, "linux", step(c, "linux", PressureState{}, linux), linux)
	if s.Level != 2 || !reflect.DeepEqual(s.Reason, []string{"cpu"}) {
		t.Fatalf("cpu linux %+v", s)
	}
	linux.CPUUtilization = .85
	if s := step(c, "linux", step(c, "linux", PressureState{}, linux), linux); s.Level != 1 || !reflect.DeepEqual(s.Reason, []string{"cpu"}) {
		t.Fatalf("cpu linux high %+v", s)
	}
	// The first sample has no utilization: UNKNOWN keeps the level.
	first := issue497Sample(1, .1)
	if s := step(c, "linux", PressureState{Level: 1, PendingLevel: 0, PendingTicks: 1, Reason: []string{"cpu"}}, first); !s.Unknown || s.Level != 1 || s.PendingLevel != 1 || s.PendingTicks != 0 {
		t.Fatalf("first cpu sample %+v", s)
	}
	if !ValidPressureReason(1, []string{"cpu", "load"}) || ValidPressureReason(1, []string{"load", "cpu"}) {
		t.Fatal("cpu reason validation")
	}
}

// CAL-V0-125/126: selection and CPU thresholds are validated.
func TestCALV0126_SignalSelectionValidation(t *testing.T) {
	for name, mutate := range map[string]func(*PressureConfig){
		"unsupported os":    func(c *PressureConfig) { c.Signals = map[string][]string{"windows": {"load"}} },
		"empty list":        func(c *PressureConfig) { c.Signals = map[string][]string{"linux": {}} },
		"duplicate":         func(c *PressureConfig) { c.Signals = map[string][]string{"linux": {"load", "load"}} },
		"unknown name":      func(c *PressureConfig) { c.Signals = map[string][]string{"linux": {"disk"}} },
		"cpu on darwin":     func(c *PressureConfig) { c.Signals = map[string][]string{"darwin": {"cpu"}} },
		"swap on darwin":    func(c *PressureConfig) { c.Signals = map[string][]string{"darwin": {"swap"}} },
		"memory on linux":   func(c *PressureConfig) { c.Signals = map[string][]string{"linux": {"memory"}} },
		"cpu no thresholds": func(c *PressureConfig) { c.CalmCPU, c.CPUHigh, c.CPUCritical = 0, 0, 0 },
		"cpu over one":      func(c *PressureConfig) { c.CPUCritical = 1.5 },
		"cpu out of order":  func(c *PressureConfig) { c.CPUHigh = .99 },
	} {
		c := cpuConfig()
		c.Signals = map[string][]string{"linux": {"cpu"}}
		mutate(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	c := issue497Config()
	c.CPUHigh = .8 // configured thresholds are validated even when unselected
	if err := c.Validate(); err == nil {
		t.Fatal("partial cpu thresholds accepted")
	}
	if err := issue497Config().Validate(); err != nil {
		t.Fatalf("config without cpu or signals refused: %v", err)
	}
}

// pressureTicks returns successive samples whose cumulative ticks advance.
type pressureTicks struct {
	base PressureSample
	n    uint64
}

func (f *pressureTicks) read(context.Context, time.Time, pressureWant) PressureSample {
	f.n++
	s := f.base
	s.CPUBusyTicks, s.CPUTotalTicks, s.CPUTicksKnown = 300*f.n, 1000*f.n, true
	return s
}

// CAL-V0-125: the dispatcher derives utilization from the previous sample
// of this run, so the first sample (and the first after a restart) has
// none; the ledger and the throttled event carry the derived value.
func TestCALV0125_DispatcherFirstTickUnknown(t *testing.T) {
	c := testConfig(t, "exit 0")
	p := cpuConfig()
	p.TicksToChange = 1
	p.Signals = map[string][]string{"linux": {PressureSignalCPU, PressureSignalLoad}}
	c.Pressure = &p
	q := &fakeQueue{}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	d.goos = "linux"
	base := darwinSample(9, MemoryPressureNormal)
	base.Source = "fake"
	d.pressureSampler = (&pressureTicks{base: base}).read
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s := d.ledger.Pressure.Sample; s.CPUUtilizationKnown || !strings.Contains(strings.Join(s.Problems, ";"), "no previous tick counters") {
		t.Fatalf("first tick sample %+v", s)
	}
	if ev := throttled(t, d); len(ev) != 1 || ev[0].Detail["cpuUtilization"] != StateUnknown {
		t.Fatalf("first throttled %+v", ev)
	}
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if x, ok := d.ledger.Pressure.Sample.CPUUtilizationFraction(); !ok || x != .3 {
		t.Fatalf("second tick utilization %v %v", x, ok)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.goos = "linux"
	if d.ledger.Pressure.Sample.CPUTicksKnown {
		t.Fatal("restart kept the tick baseline")
	}
	d.pressureSampler = (&pressureTicks{base: base, n: 5}).read
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.ledger.Pressure.Sample.CPUUtilizationKnown {
		t.Fatal("first tick after restart derived utilization")
	}
	bad := *d.ledger.Pressure
	bad.Sample.CPUUtilization, bad.Sample.CPUUtilizationKnown = 1.5, true
	if bad.validate() == nil {
		t.Fatal("ledger accepted utilization above one")
	}
	bad.Sample.CPUUtilization, bad.Sample.CPUUtilizationKnown = .5, false
	if bad.validate() == nil {
		t.Fatal("ledger accepted an unknown utilization value")
	}
}

// countedFile counts the bytes a sampler reads from one fake metric file.
type countedFile struct {
	r    io.Reader
	read *int
}

func (f countedFile) Read(p []byte) (int, error) {
	n, err := f.r.Read(p)
	*f.read += n
	return n, err
}

func (countedFile) Close() error { return nil }

// CAL-V0-125: the Linux sampler opens only the /proc files the selected
// signals need, and reads /proc/stat only up to the end of its cpu lines.
func TestCALV0125_LinuxSamplingFollowsSelection(t *testing.T) {
	stat := "cpu  300 0 100 500 100 0 0 0 0 0\ncpu0 150 0 50 250 50 0 0 0 0 0\ncpu1 150 0 50 250 50 0 0 0 0 0\nintr " + strings.Repeat("1 ", 32<<10) + "\nctxt 9\n"
	files := map[string]string{
		"/proc/loadavg": "1.50 1.00 0.50 1/100 42\n",
		"/proc/stat":    stat,
		"/proc/meminfo": "SwapTotal: 1000 kB\nSwapFree: 750 kB\n",
	}
	for name, tc := range map[string]struct {
		want  pressureWant
		opens []string
	}{
		"none":    {pressureWant{}, nil},
		"load":    {pressureWant{load: true}, []string{"/proc/loadavg", "/proc/stat"}},
		"cpu":     {pressureWant{cpu: true}, []string{"/proc/stat"}},
		"swap":    {pressureWant{swap: true}, []string{"/proc/meminfo"}},
		"memory":  {pressureWant{memory: true}, nil},
		"default": {PressureConfig{}.want("linux"), []string{"/proc/loadavg", "/proc/stat", "/proc/meminfo"}},
		"all":     {pressureWant{load: true, cpu: true, swap: true}, []string{"/proc/loadavg", "/proc/stat", "/proc/meminfo"}},
	} {
		var opened []string
		statRead := 0
		open := func(path string) (io.ReadCloser, error) {
			opened = append(opened, path)
			raw, ok := files[path]
			if !ok {
				return nil, fs.ErrNotExist
			}
			n := new(int)
			if path == "/proc/stat" {
				n = &statRead
			}
			return countedFile{strings.NewReader(raw), n}, nil
		}
		s := sampleLinuxPressure(context.Background(), time.Unix(0, 0), tc.want, open)
		if !slices.Equal(opened, tc.opens) || len(s.Problems) != 0 {
			t.Fatalf("%s: opened %v problems %v", name, opened, s.Problems)
		}
		if s.LoadKnown != tc.want.load || s.CPUKnown != tc.want.load || s.CPUTicksKnown != tc.want.cpu || s.SwapKnown != tc.want.swap {
			t.Fatalf("%s: sample %+v", name, s)
		}
		if tc.want.cpu && (s.CPUBusyTicks != 400 || s.CPUTotalTicks != 1000) {
			t.Fatalf("%s: ticks %d/%d", name, s.CPUBusyTicks, s.CPUTotalTicks)
		}
		if tc.want.load && s.CPUs != 2 {
			t.Fatalf("%s: cpus %d", name, s.CPUs)
		}
		if statRead > 8<<10 {
			t.Fatalf("%s: read %d bytes of /proc/stat past its cpu lines", name, statRead)
		}
	}
	// The parsers stop at the first non-cpu line: a malformed later line is
	// never tokenized.
	if busy, total, err := parseLinuxCPUTicks([]byte("cpu  1 0 1 1 0 0 0 0\nintr x\ncpu bad\n")); err != nil || busy != 2 || total != 3 {
		t.Fatalf("ticks %d/%d %v", busy, total, err)
	}
}

// CAL-V0-125: the Darwin sampler asks sysctl only for the selected keys and
// runs nothing when no Darwin signal is selected.
func TestCALV0125_DarwinSamplingFollowsSelection(t *testing.T) {
	out := map[string]string{darwinLoadKey: "{ 2.00 1.00 1.00 }", darwinMemoryKey: "1", darwinCPUKey: "4"}
	for name, tc := range map[string]struct {
		want pressureWant
		argv []string
	}{
		"none":   {pressureWant{cpu: true, swap: true}, nil},
		"load":   {pressureWant{load: true}, []string{"/usr/sbin/sysctl", darwinLoadKey, darwinCPUKey}},
		"memory": {pressureWant{memory: true}, []string{"/usr/sbin/sysctl", darwinMemoryKey}},
		"both":   {PressureConfig{}.want("darwin"), []string{"/usr/sbin/sysctl", darwinLoadKey, darwinMemoryKey, darwinCPUKey}},
	} {
		var argv []string
		run := func(_ context.Context, a []string) ([]byte, error) {
			argv = a
			var b bytes.Buffer
			for _, k := range a[1:] {
				b.WriteString(k + ": " + out[k] + "\n")
			}
			return b.Bytes(), nil
		}
		s := sampleDarwinPressure(context.Background(), time.Unix(0, 0), tc.want, run)
		if !slices.Equal(argv, tc.argv) || len(s.Problems) != 0 || s.LoadKnown != tc.want.load || s.MemoryPressureKnown != tc.want.memory || s.CPUTicksKnown {
			t.Fatalf("%s: argv %v sample %+v", name, argv, s)
		}
	}
	if s := parseDarwinPressure(PressureSample{}, []byte(darwinLoadKey+": { 1.00 1.00 1.00 }\n"+darwinMemoryKey+": 1\n"), pressureWant{memory: true}); len(s.Problems) != 1 {
		t.Fatalf("an unrequested sysctl field was accepted: %+v", s)
	}
}

// CAL-V0-125: without cpu selected the dispatcher asks for no CPU ticks,
// records no "no previous tick counters" problem, and writes no CPU tick or
// utilization field to the ledger, which older binaries then still read.
func TestCALV0125_UnselectedCPULeavesLedgerUnchanged(t *testing.T) {
	c := testConfig(t, "exit 0")
	p := cpuConfig() // thresholds configured, but no Signals entry selects cpu
	c.Pressure = &p
	d, err := Open("prog", c, &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.goos = "linux"
	ticks := &pressureTicks{base: issue497Sample(1, .1)}
	var wants []pressureWant
	d.pressureSampler = func(ctx context.Context, now time.Time, w pressureWant) PressureSample {
		wants = append(wants, w)
		return ticks.read(ctx, now, w) // a sampler that ignores the selection
	}
	for i := 0; i < 2; i++ {
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(wants) != 2 || wants[0].cpu || !wants[0].load || !wants[0].swap {
		t.Fatalf("selection passed to the sampler %+v", wants)
	}
	raw, err := os.ReadFile(filepath.Join(d.dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"cpuBusyTicks", "cpuTotalTicks", "cpuTicksKnown", "cpuUtilization", "no previous tick counters"} {
		if bytes.Contains(raw, []byte(field)) {
			t.Fatalf("ledger carries %s without cpu selected:\n%s", field, raw)
		}
	}
	if _, err := LoadLedger(d.dir, "prog"); err != nil {
		t.Fatal(err)
	}
}
