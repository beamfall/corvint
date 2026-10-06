package dispatch

import (
	"bytes"
	"context"
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
	Problems            []string  `json:"problems,omitempty"`
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
func SamplePressure(ctx context.Context) PressureSample { return samplePressure(ctx, time.Now().UTC()) }

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

func parseLinuxCPUs(raw []byte) (int, error) {
	seen := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == "cpu" || !strings.HasPrefix(fields[0], "cpu") {
			continue
		}
		id, err := strconv.ParseUint(strings.TrimPrefix(fields[0], "cpu"), 10, 32)
		if err != nil || fields[0] != "cpu"+strconv.FormatUint(id, 10) || len(fields) < 5 || seen[fields[0]] {
			return 0, fmt.Errorf("invalid or duplicate CPU record")
		}
		for _, x := range fields[1:] {
			if _, err := strconv.ParseUint(x, 10, 64); err != nil {
				return 0, fmt.Errorf("invalid CPU counters")
			}
		}
		seen[fields[0]] = true
	}
	if len(seen) == 0 {
		return 0, fmt.Errorf("missing host CPU records")
	}
	return len(seen), nil
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

// parseDarwinPressure fills s from the output of one
// `sysctl vm.loadavg kern.memorystatus_vm_pressure_level hw.logicalcpu`.
// V1-0862: the kernel memory-pressure level replaces sticky swap use, so a
// Darwin sample carries no swap and never falls back to it.
func parseDarwinPressure(s PressureSample, raw []byte) PressureSample {
	const loadKey, memoryKey, cpuKey = "vm.loadavg", "kern.memorystatus_vm_pressure_level", "hw.logicalcpu"
	values := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || (key != loadKey && key != memoryKey && key != cpuKey) {
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
	s.MemoryPressureLevel, err = parseDarwinMemoryPressure([]byte(values[memoryKey]))
	s.MemoryPressureKnown = err == nil
	if err != nil {
		s.Problems = append(s.Problems, "memory: "+err.Error())
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
func readPressureFile(ctx context.Context, path string, max int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
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
