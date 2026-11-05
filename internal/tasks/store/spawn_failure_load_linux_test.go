//go:build linux

// SPDX-License-Identifier: Apache-2.0
package store_test

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// ATR-V0-7204: the foreground qualification owns and joins its synthetic
// workload. The diagnostic from the original regression remains in every run.
// This is opt-in because its acceptance calls for a sustained host load >=40.
func TestATRV07204_LoadedSpawnedFailureQualification(t *testing.T) {
	if os.Getenv("CORVINT_TASKS_SPAWN_FAILURE_LOAD") != "1" {
		t.Skip("set CORVINT_TASKS_SPAWN_FAILURE_LOAD=1 for 200 loaded runs")
	}
	previous := runtime.GOMAXPROCS(48)
	defer runtime.GOMAXPROCS(previous)
	stop := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < 48; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			// Keep the load runnable without starving the foreground trial's
			// control/cleanup thread; load qualification still uses /proc/loadavg.
			if err := syscall.Setpriority(syscall.PRIO_PROCESS, syscall.Gettid(), 19); err != nil {
				t.Errorf("load worker niceness: %v", err)
			}
			for {
				select {
				case <-stop:
					return
				default:
				}
			}
		}()
	}
	defer func() { close(stop); workers.Wait() }()
	// A fixed two-minute warmup lets the one-minute host average converge.
	// No load polling and no independent/detached stress process is used.
	raw, err := os.ReadFile("/proc/loadavg")
	initial := strings.Fields(string(raw))
	load := 0.0
	if err == nil && len(initial) > 0 {
		load, _ = strconv.ParseFloat(initial[0], 64)
	}
	if load < 40 {
		time.Sleep(2 * time.Minute)
	}
	for i := 0; i < 200; i++ {
		raw, err := os.ReadFile("/proc/loadavg")
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Fields(string(raw))
		if len(fields) == 0 {
			t.Fatal("load NOT_OBSERVED")
		}
		load, err := strconv.ParseFloat(fields[0], 64)
		if err != nil || load < 40 {
			t.Fatalf("run %d requires load >=40: %q %v", i, raw, err)
		}
		t.Logf("run %d load1=%.2f", i+1, load)
		if !t.Run(fmt.Sprintf("run-%03d", i+1), TestCALV0074_SpawnedFailureIsNotNoExec) {
			return
		}
	}
}
