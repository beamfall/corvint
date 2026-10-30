package perfbudget

import (
	"context"
	"fmt"
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

// LoadFunc reports the 1, 5 and 15 minute load averages, or nil with a NOT_OBSERVED reason.
type LoadFunc func(context.Context) ([]float64, string)

func observeHost(ctx context.Context, load LoadFunc) Host {
	h := Host{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoVersion: runtime.Version(), Cores: runtime.NumCPU()}
	h.LoadBefore, h.LoadUnknown = load(ctx)
	return h
}

// NotJudgedReason is why latency and memory cannot be judged on this host record, or "" when
// they can (PRB-V0-039): the stated threshold is PRB-V0-028's `contended`, a 1-minute load
// average before or after the run that reached the core count, and an unobserved load is no
// contention record at all.
func (h Host) NotJudgedReason() string {
	switch {
	case h.Contended == nil || len(h.LoadBefore) == 0 || len(h.LoadAfter) == 0:
		reason := h.LoadUnknown
		if reason == "" {
			reason = "load average NOT_OBSERVED"
		}
		return "no host-contention record (" + reason + ")"
	case *h.Contended:
		return fmt.Sprintf("host contended: 1-minute load average %.2f before and %.2f after the run, threshold %d (the core count)", h.LoadBefore[0], h.LoadAfter[0], h.Cores)
	}
	return ""
}

func (h *Host) finish(ctx context.Context, load LoadFunc) {
	var unknown string
	h.LoadAfter, unknown = load(ctx)
	if h.LoadUnknown == "" {
		h.LoadUnknown = unknown
	}
	if len(h.LoadBefore) > 0 && len(h.LoadAfter) > 0 {
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
		out, err := outputContained(exec.CommandContext(ctx, "sysctl", "-n", "vm.loadavg"))
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
