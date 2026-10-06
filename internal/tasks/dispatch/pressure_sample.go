package dispatch

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	pressureLoadBytes      = 1 << 10
	pressureMemBytes       = 64 << 10
	pressureCPUBytes       = 256 << 10
	pressureStdoutBytes    = 4 << 10
	pressureStderrBytes    = 1 << 10
	pressureCommandTimeout = 2 * time.Second
	pressurePipeGrace      = 250 * time.Millisecond
)

// PressureSample reports independent known/unknown observations. Swap bytes
// represent allocated swap utilization (the Linux memory signal). A Darwin
// sample instead carries the kernel memory-pressure level and no swap, because
// macOS swap use is sticky after pressure passes (V1-0862).
type PressureSample struct {
	SampledAt           time.Time `json:"sampledAt"`
	Source              string    `json:"source"`
	LoadAverage         float64   `json:"loadAverage"`
	CPUs                int       `json:"cpus"`
	SwapTotalBytes      uint64    `json:"swapTotalBytes"`
	SwapUsedBytes       uint64    `json:"swapUsedBytes"`
	LoadKnown           bool      `json:"loadKnown"`
	CPUKnown            bool      `json:"cpuKnown"`
	SwapKnown           bool      `json:"swapKnown"`
	MemoryPressureLevel int       `json:"memoryPressureLevel,omitempty"`
	MemoryPressureKnown bool      `json:"memoryPressureKnown,omitempty"`
	// Cumulative host CPU busy and total ticks (CAL-V0-125): Linux reads
	// the aggregate /proc/stat line; Darwin cannot observe them.
	CPUBusyTicks  uint64 `json:"cpuBusyTicks,omitempty"`
	CPUTotalTicks uint64 `json:"cpuTotalTicks,omitempty"`
	CPUTicksKnown bool   `json:"cpuTicksKnown,omitempty"`
	// CPUUtilization is derived by the dispatcher from the tick deltas
	// since the previous sample of this run; the first sample has none.
	CPUUtilization      float64  `json:"cpuUtilization,omitempty"`
	CPUUtilizationKnown bool     `json:"cpuUtilizationKnown,omitempty"`
	Problems            []string `json:"problems,omitempty"`
}

// Darwin kern.memorystatus_vm_pressure_level values.
const (
	MemoryPressureNormal   = 1
	MemoryPressureWarn     = 2
	MemoryPressureCritical = 4
)

func validMemoryPressureLevel(n int) bool {
	return n == MemoryPressureNormal || n == MemoryPressureWarn || n == MemoryPressureCritical
}

// MemoryPressure returns the observed kernel memory-pressure level.
func (s PressureSample) MemoryPressure() (int, bool) {
	if !s.MemoryPressureKnown || !validMemoryPressureLevel(s.MemoryPressureLevel) {
		return 0, false
	}
	return s.MemoryPressureLevel, true
}

// LoadPerCPU normalizes the first load average by positive host-visible CPUs.
func (s PressureSample) LoadPerCPU() (float64, bool) {
	if !s.LoadKnown || !s.CPUKnown || s.CPUs <= 0 || !finiteNonnegative(s.LoadAverage) {
		return 0, false
	}
	return s.LoadAverage / float64(s.CPUs), true
}

// CPUUtilizationFraction returns the derived busy/total tick fraction.
func (s PressureSample) CPUUtilizationFraction() (float64, bool) {
	if !s.CPUUtilizationKnown || !finiteNonnegative(s.CPUUtilization) || s.CPUUtilization > 1 {
		return 0, false
	}
	return s.CPUUtilization, true
}

// withCPUUtilization derives cur's CPU utilization from the cumulative tick
// deltas since prev (CAL-V0-125). It stays UNKNOWN without a previous
// counter sample from the same source (the first sample of a run), and when
// the counters did not advance, went backwards, or are inconsistent.
func withCPUUtilization(prev, cur PressureSample) PressureSample {
	cur.CPUUtilization, cur.CPUUtilizationKnown = 0, false
	if !cur.CPUTicksKnown {
		return cur
	}
	if !prev.CPUTicksKnown || prev.Source != cur.Source {
		cur.Problems = append(cur.Problems, "cpu: no previous tick counters in this run")
		return cur
	}
	if cur.CPUBusyTicks > cur.CPUTotalTicks || prev.CPUBusyTicks > prev.CPUTotalTicks || cur.CPUTotalTicks <= prev.CPUTotalTicks || cur.CPUBusyTicks < prev.CPUBusyTicks {
		cur.Problems = append(cur.Problems, "cpu: tick counters did not advance consistently")
		return cur
	}
	busy, total := cur.CPUBusyTicks-prev.CPUBusyTicks, cur.CPUTotalTicks-prev.CPUTotalTicks
	if busy > total {
		cur.Problems = append(cur.Problems, "cpu: tick counters did not advance consistently")
		return cur
	}
	cur.CPUUtilization, cur.CPUUtilizationKnown = float64(busy)/float64(total), true
	return cur
}

// SwapFraction treats explicitly observed zero total/used as no allocated
// swap (fraction zero), avoiding a fabricated value for unavailable metrics.
func (s PressureSample) SwapFraction() (float64, bool) {
	if !s.SwapKnown || s.SwapUsedBytes > s.SwapTotalBytes {
		return 0, false
	}
	if s.SwapTotalBytes == 0 {
		return 0, true
	}
	return float64(s.SwapUsedBytes) / float64(s.SwapTotalBytes), true
}

// SamplePressure reads fixed local OS metrics; unsupported inputs stay UNKNOWN.
func SamplePressure(ctx context.Context) PressureSample {
	return samplePressure(ctx, time.Now().UTC(), pressureWantAll)
}

// pressureWant names the observations one sample needs (CAL-V0-125): a
// sampler reads and parses nothing for a signal the configuration does not
// select. A host sampler ignores what it cannot observe.
type pressureWant struct{ load, cpu, swap, memory bool }

var pressureWantAll = pressureWant{load: true, cpu: true, swap: true, memory: true}

// want is the selection for goos: its Signals entry, or without one the
// default signals (load, and memory where observed, else swap).
func (c PressureConfig) want(goos string) pressureWant {
	names, ok := c.Signals[goos]
	if !ok {
		return pressureWant{load: true, swap: true, memory: true}
	}
	return pressureWant{load: contains(names, PressureSignalLoad), cpu: contains(names, PressureSignalCPU), swap: contains(names, PressureSignalSwap), memory: contains(names, PressureSignalMemory)}
}

// pressureOpen opens one host metric file; tests substitute it.
type pressureOpen func(path string) (io.ReadCloser, error)

func osPressureOpen(path string) (io.ReadCloser, error) { return os.Open(path) }

// sampleLinuxPressure reads only the /proc files the selection needs:
// /proc/loadavg for load, the leading cpu lines of /proc/stat for load (the
// CPU count) or cpu (the aggregate ticks), and /proc/meminfo for swap.
func sampleLinuxPressure(ctx context.Context, now time.Time, want pressureWant, open pressureOpen) PressureSample {
	s := PressureSample{SampledAt: now, Source: "linux-proc-host"}
	if want.load {
		raw, err := readPressureFile(ctx, open, "/proc/loadavg", pressureLoadBytes)
		if err == nil {
			s.LoadAverage, err = parseLoad(raw, false)
		}
		s.LoadKnown = err == nil
		if err != nil {
			s.Problems = append(s.Problems, "load: "+err.Error())
		}
	}
	if want.load || want.cpu {
		raw, statErr := readStatCPULines(ctx, open, "/proc/stat", pressureCPUBytes)
		if want.load {
			err := statErr
			if err == nil {
				s.CPUs, err = parseLinuxCPUs(raw)
			}
			s.CPUKnown = err == nil
			if err != nil {
				s.Problems = append(s.Problems, "cpus: "+err.Error())
			}
		}
		if want.cpu {
			err := statErr
			if err == nil {
				s.CPUBusyTicks, s.CPUTotalTicks, err = parseLinuxCPUTicks(raw)
			}
			s.CPUTicksKnown = err == nil
			if err != nil {
				s.Problems = append(s.Problems, "cpu ticks: "+err.Error())
			}
		}
	}
	if want.swap {
		raw, err := readPressureFile(ctx, open, "/proc/meminfo", pressureMemBytes)
		if err == nil {
			s.SwapTotalBytes, s.SwapUsedBytes, err = parseLinuxSwap(raw)
		}
		s.SwapKnown = err == nil
		if err != nil {
			s.Problems = append(s.Problems, "swap: "+err.Error())
		}
	}
	return s
}

// sampleDarwinPressure runs one sysctl for the selected keys only: load
// needs vm.loadavg and hw.logicalcpu, memory the kernel pressure level.
// Darwin observes neither CPU ticks nor (by V1-0862) swap.
func sampleDarwinPressure(ctx context.Context, now time.Time, want pressureWant, run func(context.Context, []string) ([]byte, error)) PressureSample {
	s := PressureSample{SampledAt: now, Source: "darwin-sysctl-host"}
	argv := []string{"/usr/sbin/sysctl"}
	if want.load {
		argv = append(argv, darwinLoadKey)
	}
	if want.memory {
		argv = append(argv, darwinMemoryKey)
	}
	if want.load {
		argv = append(argv, darwinCPUKey)
	}
	if len(argv) == 1 {
		return s
	}
	raw, err := run(ctx, argv)
	if err != nil {
		s.Problems = []string{err.Error()}
		return s
	}
	return parseDarwinPressure(s, raw, want)
}

func parseLoad(raw []byte, darwin bool) (float64, error) {
	fields := strings.Fields(string(raw))
	if darwin {
		if len(fields) != 5 || fields[0] != "{" || fields[4] != "}" {
			return 0, fmt.Errorf("invalid Darwin load format")
		}
		fields = fields[1:4]
	} else if len(fields) != 5 {
		return 0, fmt.Errorf("invalid Linux load format")
	}
	var first float64
	for i, v := range fields[:3] {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || !finiteNonnegative(n) {
			return 0, fmt.Errorf("invalid load average")
		}
		if i == 0 {
			first = n
		}
	}
	return first, nil
}

func parseCPUs(raw []byte) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || n <= 0 || n > 1<<20 {
		return 0, fmt.Errorf("invalid CPU count")
	}
	return n, nil
}

// statCPULines calls fn for each leading cpu line of /proc/stat and stops at
// the first other line, which the kernel prints only after them.
func statCPULines(raw []byte, fn func(fields []string) error) error {
	for len(raw) > 0 {
		line, rest, _ := bytes.Cut(raw, []byte("\n"))
		if !bytes.HasPrefix(line, []byte("cpu")) {
			return nil
		}
		if err := fn(strings.Fields(string(line))); err != nil {
			return err
		}
		raw = rest
	}
	return nil
}

func parseLinuxCPUs(raw []byte) (int, error) {
	seen := map[string]bool{}
	err := statCPULines(raw, func(fields []string) error {
		if fields[0] == "cpu" {
			return nil
		}
		id, err := strconv.ParseUint(strings.TrimPrefix(fields[0], "cpu"), 10, 32)
		if err != nil || fields[0] != "cpu"+strconv.FormatUint(id, 10) || len(fields) < 5 || seen[fields[0]] {
			return fmt.Errorf("invalid or duplicate CPU record")
		}
		for _, x := range fields[1:] {
			if _, err := strconv.ParseUint(x, 10, 64); err != nil {
				return fmt.Errorf("invalid CPU counters")
			}
		}
		seen[fields[0]] = true
		return nil
	})
	if err != nil {
		return 0, err
	}
	if len(seen) == 0 {
		return 0, fmt.Errorf("missing host CPU records")
	}
	return len(seen), nil
}

// parseLinuxCPUTicks reads the aggregate "cpu" line of /proc/stat: total is
// user+nice+system+idle+iowait+irq+softirq+steal (guest time is already
// inside user and nice) and busy is total less idle and iowait.
func parseLinuxCPUTicks(raw []byte) (uint64, uint64, error) {
	var busy, total uint64
	found := false
	err := statCPULines(raw, func(fields []string) error {
		if fields[0] != "cpu" {
			return nil
		}
		if found || len(fields) < 9 {
			return fmt.Errorf("invalid or duplicate aggregate CPU record")
		}
		found = true
		var idle uint64
		for i, x := range fields[1:9] {
			n, err := strconv.ParseUint(x, 10, 64)
			if err != nil || n > math.MaxUint64-total {
				return fmt.Errorf("invalid aggregate CPU counters")
			}
			total += n
			if i == 3 || i == 4 {
				idle += n
			}
		}
		busy = total - idle
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	if !found {
		return 0, 0, fmt.Errorf("missing aggregate CPU record")
	}
	return busy, total, nil
}

func parseLinuxSwap(raw []byte) (uint64, uint64, error) {
	values := map[string]uint64{}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || (fields[0] != "SwapTotal:" && fields[0] != "SwapFree:") {
			continue
		}
		if len(fields) != 3 || fields[2] != "kB" {
			return 0, 0, fmt.Errorf("invalid swap format")
		}
		if _, ok := values[fields[0]]; ok {
			return 0, 0, fmt.Errorf("duplicate swap field")
		}
		n, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil || n > math.MaxUint64/1024 {
			return 0, 0, fmt.Errorf("invalid swap size")
		}
		values[fields[0]] = n * 1024
	}
	total, totalOK := values["SwapTotal:"]
	free, freeOK := values["SwapFree:"]
	if !totalOK || !freeOK || free > total {
		return 0, 0, fmt.Errorf("missing or inconsistent swap fields")
	}
	return total, total - free, nil
}

const darwinLoadKey, darwinMemoryKey, darwinCPUKey = "vm.loadavg", "kern.memorystatus_vm_pressure_level", "hw.logicalcpu"

// parseDarwinPressure fills s from the output of one sysctl of the selected
// keys of vm.loadavg, kern.memorystatus_vm_pressure_level and hw.logicalcpu.
// V1-0862: the kernel memory-pressure level replaces sticky swap use, so a
// Darwin sample carries no swap and never falls back to it.
func parseDarwinPressure(s PressureSample, raw []byte, want pressureWant) PressureSample {
	loadKey, memoryKey, cpuKey := darwinLoadKey, darwinMemoryKey, darwinCPUKey
	values := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || (!(want.load && (key == loadKey || key == cpuKey)) && !(want.memory && key == memoryKey)) {
			s.Problems = []string{"invalid pressure sysctl fields"}
			return s
		}
		if _, duplicate := values[key]; duplicate {
			s.Problems = []string{"duplicate pressure sysctl field"}
			return s
		}
		values[key] = strings.TrimSpace(value)
	}
	var err error
	if want.load {
		s.LoadAverage, err = parseLoad([]byte(values[loadKey]), true)
		s.LoadKnown = err == nil
		if err != nil {
			s.Problems = append(s.Problems, "load: "+err.Error())
		}
		s.CPUs, err = parseCPUs([]byte(values[cpuKey]))
		s.CPUKnown = err == nil
		if err != nil {
			s.Problems = append(s.Problems, "cpus: "+err.Error())
		}
	}
	if want.memory {
		s.MemoryPressureLevel, err = parseDarwinMemoryPressure([]byte(values[memoryKey]))
		s.MemoryPressureKnown = err == nil
		if err != nil {
			s.Problems = append(s.Problems, "memory: "+err.Error())
		}
	}
	return s
}

func parseDarwinMemoryPressure(raw []byte) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || !validMemoryPressureLevel(n) {
		return 0, fmt.Errorf("invalid Darwin memory pressure level")
	}
	return n, nil
}

// readPressureFile is deliberately synchronous: cancellation cannot abandon a
// goroutine inside kernel I/O. Byte bounds are enforced, wall-time is not.
func readPressureFile(ctx context.Context, open pressureOpen, path string, max int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(f, int64(max)+1))
	closeErr := f.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(raw) > max {
		return nil, fmt.Errorf("pressure metric exceeds %d bytes", max)
	}
	return raw, nil
}

// readStatCPULines reads only the leading cpu lines of /proc/stat (at most
// max bytes) and stops at the first other line, so the long intr and
// softirq lines are never copied or tokenized (CAL-V0-125).
func readStatCPULines(ctx context.Context, open pressureOpen, path string, max int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := open(path)
	if err != nil {
		return nil, err
	}
	r := bufio.NewReaderSize(f, 4<<10)
	var out []byte
	for {
		if p, _ := r.Peek(3); string(p) != "cpu" {
			break
		}
		line, err := r.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			err = fmt.Errorf("pressure CPU record exceeds %d bytes", 4<<10)
		}
		if err != nil && err != io.EOF {
			f.Close()
			return nil, err
		}
		if len(out)+len(line) > max {
			f.Close()
			return nil, fmt.Errorf("pressure metric exceeds %d bytes", max)
		}
		out = append(out, line...)
		if err == io.EOF {
			break
		}
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// pressureOutput rejects excess during collection rather than buffering it.
type pressureOutput struct {
	buffer bytes.Buffer
	max    int
}

func (b *pressureOutput) Write(p []byte) (int, error) {
	if len(p) > b.max-b.buffer.Len() {
		return 0, fmt.Errorf("pressure command output exceeds %d bytes", b.max)
	}
	return b.buffer.Write(p)
}

func runPressureCommand(ctx context.Context, argv []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, pressureCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = []string{"LC_ALL=C", "PATH=/usr/bin:/bin:/usr/sbin:/sbin"}
	cmd.WaitDelay = pressurePipeGrace
	stdout, stderr := &pressureOutput{max: pressureStdoutBytes}, &pressureOutput{max: pressureStderrBytes}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	// Run includes Wait: return values never advertise an unjoined command.
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pressure command: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return stdout.buffer.Bytes(), nil
}
