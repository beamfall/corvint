//go:build darwin || linux

package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"
)

// TestPSRV0891_RefreshBoundaryLatency is the opt-in V1-0916 measurement, not
// a timing assertion. CORVINT_TASKS_REFRESH_BOUNDARY names the receipt count
// (for example 13000). On a historyStoreAt fixture of 880 tickets it runs 600
// sequential ticket creates, so at least one crosses the 512-receipt refresh
// boundary, and logs the median, p99 and maximum write wall time and the wall
// time and route of every write that ran the scheduled refresh. With
// CORVINT_TASKS_REFRESH_BOUNDARY_COMPLETE=1 every refresh is the complete
// audit, for the before figure.
func TestPSRV0891_RefreshBoundaryLatency(t *testing.T) {
	raw := os.Getenv("CORVINT_TASKS_REFRESH_BOUNDARY")
	if raw == "" {
		t.Skip("set CORVINT_TASKS_REFRESH_BOUNDARY=13000 to measure")
	}
	receipts, err := strconv.Atoi(raw)
	if err != nil || receipts < 1000 {
		t.Fatalf("CORVINT_TASKS_REFRESH_BOUNDARY=%q", raw)
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := historyStoreAt(t, filepath.Join(base, "repo"), 880, receipts)
	if os.Getenv("CORVINT_TASKS_REFRESH_BOUNDARY_COMPLETE") == "1" {
		completeRefreshOnly(t, repo)
	}
	var walls []time.Duration
	for i := 0; i < 600; i++ {
		route := ""
		ctx := context.WithValue(context.Background(), mutationStageKey{}, func(s string) {
			if s == "refresh.incremental" || s == "refresh.audited" && route == "" {
				route = s
			}
		})
		start := time.Now()
		rep, err := Mutate(ctx, repo, historyActor, historyCreate(fmt.Sprintf("boundary-%d", i), "boundary"), WallClock())
		d := time.Since(start)
		if err != nil || rep.Outcome.Outcome != "COMPLETED" {
			t.Fatalf("write %d: %v %+v", i, err, rep)
		}
		walls = append(walls, d)
		if d > time.Second && route == "" {
			t.Logf("write %d took %v without a refresh", i, d)
		}
		if route != "" {
			if route == "refresh.audited" {
				route = "complete"
			} else {
				route = "incremental"
			}
			t.Logf("write %d ran the %s refresh: %v", i, route, d)
		}
	}
	sort.Slice(walls, func(i, j int) bool { return walls[i] < walls[j] })
	t.Logf("receipts %d: n=%d p50=%v p99=%v max=%v load %s", receipts, len(walls), walls[len(walls)/2], walls[len(walls)*99/100], walls[len(walls)-1], hostLoad())
}
