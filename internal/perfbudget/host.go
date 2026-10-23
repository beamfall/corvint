package perfbudget

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Host is the host-contention record each run and baseline carries (GitHub #1406): what else
// the host was doing is as much a part of a latency number as the number itself.
type Host struct {
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
	GoVersion string `json:"go_version"`
	Cores     int    `json:"cores"`
	// LoadBefore and LoadAfter are the 1, 5 and 15 minute load averages around the run; null
	// with LoadUnknown when the host does not report them.
	LoadBefore  []float64 `json:"load_before"`
	LoadAfter   []float64 `json:"load_after"`
	LoadUnknown string    `json:"load_unknown,omitempty"`
	// Contended is true when either 1-minute load average reached the core count, so latency
	// verdicts from this run are confounded by other work on the host.
	Contended *bool `json:"contended"`
}

func observeHost(ctx context.Context) Host {
	h := Host{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoVersion: runtime.Version(), Cores: runtime.NumCPU()}
	h.LoadBefore, h.LoadUnknown = loadAverage(ctx)
	return h
}

func (h *Host) finish(ctx context.Context) {
	var unknown string
	h.LoadAfter, unknown = loadAverage(ctx)
	if h.LoadUnknown == "" {
		h.LoadUnknown = unknown
	}
	if h.LoadBefore != nil && h.LoadAfter != nil {
		contended := h.LoadBefore[0] >= float64(h.Cores) || h.LoadAfter[0] >= float64(h.Cores)
		h.Contended = &contended
	}
}

// loadAverage reads the load averages from /proc/loadavg or, on Darwin and the BSDs, from
// `sysctl -n vm.loadavg`.
func loadAverage(ctx context.Context) ([]float64, string) {
	var text string
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		text = string(data)
	} else if runtime.GOOS == "darwin" || strings.HasSuffix(runtime.GOOS, "bsd") {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "sysctl", "-n", "vm.loadavg").Output()
		if err != nil {
			return nil, "load average NOT_OBSERVED: sysctl vm.loadavg failed"
		}
		text = strings.Trim(strings.TrimSpace(string(out)), "{}")
	} else {
		return nil, "load average NOT_OBSERVED: no /proc/loadavg or vm.loadavg on " + runtime.GOOS
	}
	fields := strings.Fields(text)
	if len(fields) < 3 {
		return nil, "load average NOT_OBSERVED: unparseable"
	}
	load := make([]float64, 3)
	for i := range 3 {
		value, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return nil, "load average NOT_OBSERVED: unparseable"
		}
		load[i] = value
	}
	return load, ""
}
