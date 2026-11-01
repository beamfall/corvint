// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// churnLoop reports only successful creates as creates; refusals are counted
// apart, so a pool that cannot write reports zero throughput.
func TestChurnLoopCountsOnlySuccessfulCreates(t *testing.T) {
	for _, tc := range []struct {
		name            string
		fail            func(n int) bool
		creates, failed int
	}{
		{"all refused", func(int) bool { return true }, 0, 6},
		{"odd refused", func(n int) bool { return n%2 == 1 }, 3, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			res := churnLoop(ctx, func(n int) error {
				if calls++; calls == 6 {
					cancel()
				}
				if tc.fail(n) {
					return errors.New("refused")
				}
				return nil
			})
			if res.Creates != tc.creates || res.Failed != tc.failed {
				t.Fatalf("got %+v, want creates %d failed %d", res, tc.creates, tc.failed)
			}
		})
	}
}

// skipPOSIXWriters skips on Windows, where the /bin/sleep writers do not
// exist (V1-2490). Tasks imports no Core package, so this does not use
// internal/testplatform.
func skipPOSIXWriters(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell on this platform: the writers are the POSIX /bin/sleep tool")
	}
}

// A writer start that fails after others started stops and reaps those
// already running instead of abandoning them.
func TestStartWritersReapsStartedWritersWhenALaterStartFails(t *testing.T) {
	skipPOSIXWriters(t)
	var started []*exec.Cmd
	t0 := time.Now()
	pool, err := startWriters(context.Background(), [][]string{{"/bin/sleep", "60"}, {"/nonexistent/tasks-scale-writer"}}, func(_ int, c *exec.Cmd) { started = append(started, c) })
	if err == nil || pool != nil {
		t.Fatalf("startWriters: pool %v, err %v; want a start error", pool, err)
	}
	if len(started) != 2 || started[0].ProcessState == nil {
		t.Fatalf("the started writer was not reaped")
	}
	if d := time.Since(t0); d > 20*time.Second {
		t.Fatalf("reaping took %s; the writer ran to completion", d)
	}
}

// stop cancels running writers and waits for them.
func TestWriterPoolStopCancelsRunningWriters(t *testing.T) {
	skipPOSIXWriters(t)
	pool, err := startWriters(context.Background(), [][]string{{"/bin/sleep", "60"}, {"/bin/sleep", "60"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	pool.stop()
	if d := time.Since(t0); d > 20*time.Second {
		t.Fatalf("stop took %s", d)
	}
	for i, c := range pool.cmds {
		if c.ProcessState == nil || c.ProcessState.Success() {
			t.Fatalf("writer %d: state %v, want stopped", i, c.ProcessState)
		}
	}
}

// V1-3752 and V1-3753: a cp -a copy of a generated store refuses writes
// until relocate re-pins it, the refused write is measured as a non-sample
// carrying its outcome and codes, and after relocate the copy's write is a
// timing sample while the original store is untouched.
func TestCopiedStoreRelocatesAndRefusedWritesAreNotSamples(t *testing.T) {
	skipPOSIXWriters(t)
	if testing.Short() {
		t.Skip("generates a store")
	}
	dir := t.TempDir()
	orig, cp := filepath.Join(dir, "orig"), filepath.Join(dir, "copy")
	if err := gen([]string{"--root", orig, "--tickets", "58"}); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cp", "-a", orig, cp).CombinedOutput(); err != nil {
		t.Fatalf("cp -a: %v %s", err, out)
	}
	before := ticketFiles(t, orig)
	create := func(req string) []string {
		return []string{"ticket", "create", "--request-id", req, "--payload", createPayload(900, nil)}
	}
	refused, _ := measure(cp, create("copy-before"))
	if refused.Sample || refused.Outcome == "OK" || len(refused.Codes) == 0 || timingSample(refused) == nil {
		t.Fatalf("a refused write on the unrelocated copy counted as a sample: %+v", refused)
	}
	if err := relocate([]string{"--store", cp}); err != nil {
		t.Fatal(err)
	}
	ok, out := measure(cp, create("copy-after"))
	if !ok.Sample || ok.Outcome != "OK" || timingSample(ok) != nil {
		t.Fatalf("write after relocate: %+v %s", ok, out)
	}
	raw, err := os.ReadFile(manifestPath(cp))
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if want, _ := filepath.EvalSymlinks(cp); m.Root != want {
		t.Fatalf("relocated manifest root %q, want %q", m.Root, want)
	}
	if after := ticketFiles(t, orig); after != before {
		t.Fatalf("the original store changed from %d to %d ticket files", before, after)
	}
	if got := ticketFiles(t, cp); got != before+1 {
		t.Fatalf("the copy holds %d ticket files, want %d", got, before+1)
	}
}

func TestEnvelopeOutcomeNamesUndecodableOutput(t *testing.T) {
	if o, c := envelopeOutcome([]byte("not json")); o != "UNDECODABLE" || len(c) != 0 {
		t.Fatalf("got %s %v", o, c)
	}
	if o, c := envelopeOutcome([]byte(`{"outcome":"REFUSED","codes":["STALE_TICKET"]}`)); o != "REFUSED" || len(c) != 1 || c[0] != "STALE_TICKET" {
		t.Fatalf("got %s %v", o, c)
	}
	if err := timingSample(metrics{Exit: 0, Outcome: "REFUSED", Codes: []string{"STALE_TICKET"}}); err == nil {
		t.Fatal("a REFUSED run passed as a timing sample")
	}
}

func ticketFiles(t *testing.T, store string) int {
	t.Helper()
	es, err := os.ReadDir(filepath.Join(store, ".taskman", "tickets"))
	if err != nil {
		t.Fatal(err)
	}
	return len(es)
}
