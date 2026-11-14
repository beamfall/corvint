package linecov

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// hostLoad1 reports the 1-minute load average from /proc/loadavg or, on
// Darwin and the BSDs, `sysctl -n vm.loadavg`; ok is false when the host does
// not report it. internal/perfbudget reads the same sources, but it imports
// this package, so the test keeps its own reader.
func hostLoad1() (load float64, ok bool) {
	text := ""
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		text = string(data)
	} else if runtime.GOOS == "darwin" || strings.HasSuffix(runtime.GOOS, "bsd") {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "sysctl", "-n", "vm.loadavg").Output()
		if err != nil {
			return 0, false
		}
		text = strings.Trim(strings.TrimSpace(string(out)), "{}")
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return 0, false
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	return load, err == nil
}

// contendedPlanBudgetFactor bounds what contention can excuse: the loaded
// nightly gate runners measured at most 1.22 times the GLS-V0-003 budget, so a
// p95 past twice the budget fails even on a contended host.
const contendedPlanBudgetFactor = 2

// planBudgetContended is PRB-V0-028's contention threshold applied to a plan
// budget run: a 1-minute load average before or after that reached the core
// count. An unobserved load is not contention, so the budget stays judged.
func planBudgetContended(before, after float64, observed bool, cores int) bool {
	return observed && (before >= float64(cores) || after >= float64(cores))
}

// V1-5003: only an observed load that reached the core count withholds the
// GLS-V0-003 verdict; Linux and Darwin hosts must observe it.
func TestPlanBudgetContended(t *testing.T) {
	for _, c := range []struct {
		name          string
		before, after float64
		observed      bool
		want          bool
	}{
		{"quiet", 1, 2.9, true, false},
		{"contended before", 3, 0.5, true, true},
		{"contended after", 0.5, 7, true, true},
		{"unobserved", 9, 9, false, false},
	} {
		if got := planBudgetContended(c.before, c.after, c.observed, 3); got != c.want {
			t.Errorf("%s: contended %v, want %v", c.name, got, c.want)
		}
	}
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		if load, ok := hostLoad1(); !ok || load < 0 {
			t.Fatalf("1-minute load average NOT_OBSERVED on %s: %v %v", runtime.GOOS, load, ok)
		}
	}
}
