// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCTWV07100_BatchRefineProfile retains per-chunk phase costs for the
// V1-4260 369-entry labels-only workload. It is opt-in, never a timing gate.
func TestCTWV07100_BatchRefineProfile(t *testing.T) {
	path := os.Getenv("CORVINT_TASKS_BATCH_REFINE_PROFILE")
	if path == "" {
		t.Skip("set CORVINT_TASKS_BATCH_REFINE_PROFILE to a JSON report path")
	}
	repo := historyStoreAt(t, filepath.Join(t.TempDir(), "repo"), 880, 13500)
	files, err := os.ReadDir(filepath.Join(repo.IntentRoot(), ".taskman", "tickets"))
	if err != nil {
		t.Fatal(err)
	}
	envs := make([][]byte, 369)
	for i := range envs {
		raw, err := os.ReadFile(filepath.Join(repo.IntentRoot(), ".taskman", "tickets", files[i].Name()))
		if err != nil {
			t.Fatal(err)
		}
		v, err := wire.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := v.Obj.Get("ticketId")
		rev, _ := v.Obj.Get("revision")
		// The fixture uses the same native envelope as the CLI's batch path.
		e := wire.NewObject().Set("profile", wire.String(mutation.Profile)).
			Set("requestId", wire.String(fmt.Sprintf("labels/%d", i))).
			Set("queueId", wire.String("queue:"+strings.Join(strings.Split(id.Str, ":")[1:3], ":"))).
			Set("actor", wire.ObjectValue(wire.NewObject().Set("id", wire.String(historyActor.ID)).Set("role", wire.String(historyActor.Role)))).
			Set("operation", wire.String(mutation.OpRefine)).Set("targetId", id).Set("expectedRevision", rev).
			Set("issuedAt", wire.String(string(WallClock()))).
			Set("payload", wire.ObjectValue(wire.NewObject().Set("labels", wire.Strings([]string{"tier-test"}))))
		envs[i] = wire.EncodeFile(wire.ObjectValue(e))
	}
	type chunk struct {
		Entries        int
		WallMS, LockMS float64
		Phases         map[string]float64
	}
	chunks := []chunk{}
	previous := 0
	current := chunk{Phases: map[string]float64{}}
	start := time.Now()
	last := start
	ctx := authority.WithLockObserver(context.Background(), func(d time.Duration) { current.LockMS += ms(d) })
	ctx = context.WithValue(ctx, mutationStageKey{}, func(stage string) {
		name, raw, ok := strings.Cut(stage, "=")
		if !ok {
			return
		}
		d, err := time.ParseDuration(raw)
		if err == nil {
			current.Phases[name] += ms(d)
		}
	})
	ctx = WithBatchYieldHook(ctx, func(next int) {
		current.Entries = next - previous
		previous = next
		current.WallMS = ms(time.Since(last))
		chunks = append(chunks, current)
		current = chunk{Phases: map[string]float64{}}
		last = time.Now()
	})
	before := hostLoad()
	report, err := MutateBatch(ctx, repo, historyActor, envs, WallClock())
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range report.Entries {
		if e.Err != nil || e.Report == nil || e.Report.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("entry %d: %+v report=%+v", i, e, e.Report)
		}
	}
	current.Entries = len(envs) - previous
	current.WallMS = ms(time.Since(last))
	chunks = append(chunks, current)
	out := map[string]any{"entries": len(envs), "wallMS": ms(time.Since(start)), "loadBefore": before, "loadAfter": hostLoad(), "chunks": chunks, "goVersion": runtime.Version(), "cpuCount": runtime.NumCPU()}
	b, _ := json.MarshalIndent(out, "", "  ")
	if err := os.WriteFile(path, append(b, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("369 refines: %.0f ms, %d chunks, report %s", ms(time.Since(start)), report.Chunks, path)
}
