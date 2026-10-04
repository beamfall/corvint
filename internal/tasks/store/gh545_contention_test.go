package store_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func h545Int(t *testing.T, key string, fallback, min, max int) int {
	t.Helper()
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < min || n > max {
		t.Fatalf("%s must be an integer in [%d,%d]", key, min, max)
	}
	return n
}

// TestGH545ClaimStarvation is the opt-in issue 545 qualification: fresh CLI
// processes, mixed claim/heartbeat/renew/release writers and a long receipt
// history. It measures whole-operation time and outcome, not lock hold, and
// fails when any writer is refused or exceeds the per-operation bound. It is
// separate from the unchanged issue 494 wave and asserts no latency budget
// beyond the stated bound.
func TestGH545ClaimStarvation(t *testing.T) {
	binary, evidence := os.Getenv("GH545_CLI"), os.Getenv("GH545_EVIDENCE_DIR")
	if binary == "" {
		t.Skip("set GH545_CLI and GH545_EVIDENCE_DIR to absolute paths")
	}
	if !filepath.IsAbs(binary) || !filepath.IsAbs(evidence) {
		t.Fatal("absolute binary/evidence paths required")
	}
	if _, err := os.Stat(evidence); err != nil {
		t.Fatal(err)
	}
	receipts := h545Int(t, "GH545_RECEIPTS", 7140, 100, 50000)
	workers := h545Int(t, "GH545_WORKERS", 10, 2, 32)
	bound := time.Duration(h545Int(t, "GH545_MAX_OPERATION_SECONDS", 60, 1, 600)) * time.Second
	verbs := []string{"claim", "heartbeat", "renew", "heartbeat", "release"}

	s := newLeaseStore(t)
	policy := fixture.PolicyValue()
	policy.Obj.Set("policyVersion", str("3"))
	policy.Obj.Set("capacity", obj("maxActiveAttempts", str(strconv.Itoa(workers)), "maxWorkersTotal", str(strconv.Itoa(workers)), "classes", wire.Array()))
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	rep, err := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("h545-capacity", "2", wire.EncodeFile(policy)), now(t))
	if err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, err)
	}
	ids := make([]string, workers)
	for i := range ids {
		ids[i] = s.ticket(t, fmt.Sprintf("h545-worker-%d", i))
	}
	fr := &fixture.Repo{Root: s.repo.PrimaryWorktree, CommonDir: s.repo.CommonDir, StateDir: s.repo.StateDir, IntentDir: filepath.Join(s.repo.PrimaryWorktree, ".taskman")}
	gitRun(t, fr.Root, "init", "-q", "-b", "main")
	gitRun(t, fr.Root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "h545 fixture")
	head, err := snapshot.DecodeHead(h494Read(t, filepath.Join(fr.StateDir, "head.json")))
	if err != nil {
		t.Fatal(err)
	}
	for seq := head.LastSeq.Uint64() + 1; seq <= uint64(receipts); seq++ {
		id := fmt.Sprintf("h545-history-%d", seq)
		path, _ := snapshot.RequestPath(id)
		h494AppendReceipt(t, fr, "MUTATION", map[string][]byte{path: h494RequestBytes(id, seq, wire.Sum([]byte(id)), false)}, id, true, true, false)
	}
	if !h494Audit(t, s, evidence, "audit-before") {
		return
	}

	type op struct {
		Worker    int     `json:"worker"`
		Step      int     `json:"step"`
		Verb      string  `json:"verb"`
		ElapsedMS float64 `json:"elapsedMS"`
		Error     string  `json:"error"`
	}
	var mu sync.Mutex
	var ops []op
	var wg sync.WaitGroup
	start := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(len(verbs)*workers)*bound)
	defer cancel()
	began := time.Now()
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			<-start
			attempt, generation := "", ""
			for step, verb := range verbs {
				args := []string{verb}
				if verb == "heartbeat" {
					args = []string{"attempt", "heartbeat"}
				}
				args = append(args, "--request-id", fmt.Sprintf("h545-%d-%d-%s", i, step, verb))
				if verb == "claim" {
					args = append(args, id, "--holder", fmt.Sprintf("agent-%d", i), "--lease-minutes", "60", "--scope", fmt.Sprintf("src/%d", i))
				} else {
					args = append(args, "--attempt", attempt, "--generation", generation)
					if verb == "renew" {
						args = append(args, "--lease-minutes", "60")
					}
				}
				at := time.Now()
				result, err := h494CLI(t, ctx, binary, fr.Root, evidence, fmt.Sprintf("worker-%02d-%d-%s", i, step, verb), args)
				row := op{Worker: i, Step: step, Verb: verb, ElapsedMS: float64(time.Since(at)) / float64(time.Millisecond)}
				if err != nil {
					row.Error = err.Error()
				}
				mu.Lock()
				ops = append(ops, row)
				mu.Unlock()
				if err != nil {
					return
				}
				if verb == "claim" {
					attempt, generation = result.Items[0].AttemptID, string(result.Items[0].Generation)
				}
			}
		}(i, id)
	}
	close(start)
	wg.Wait()
	elapsed := time.Since(began)
	cancel()

	sort.Slice(ops, func(a, b int) bool {
		if ops[a].Worker != ops[b].Worker {
			return ops[a].Worker < ops[b].Worker
		}
		return ops[a].Step < ops[b].Step
	})
	completed, slow, worst := 0, 0, 0.0
	var failures []string
	for _, o := range ops {
		if o.ElapsedMS > worst {
			worst = o.ElapsedMS
		}
		if o.Error != "" {
			failures = append(failures, fmt.Sprintf("worker %d %s: %s", o.Worker, o.Verb, o.Error))
			continue
		}
		completed++
		if o.ElapsedMS > float64(bound)/float64(time.Millisecond) {
			slow++
		}
	}
	expected := len(verbs) * workers
	h494Save(t, evidence, "workload.json", map[string]any{"binary": binary, "receipts": receipts, "workers": workers, "verbs": verbs, "elapsedMS": float64(elapsed) / float64(time.Millisecond), "expectedOperations": expected, "completedOperations": completed, "operationsOverBound": slow, "boundSeconds": bound.Seconds(), "worstOperationMS": worst, "operations": ops, "limits": []string{"synthetic small-receipt history on one host", "one wave; no p95, fairness or latency-budget claim", "evidence writes add overhead"}})
	t.Logf("issue 545: %d/%d operations, %d over %s, worst %.0f ms, wave %.0f ms at %d receipts and %d workers", completed, expected, slow, bound, worst, float64(elapsed)/float64(time.Millisecond), receipts, workers)
	h494Audit(t, s, evidence, "audit-after")
	if completed != expected || slow != 0 {
		t.Errorf("issue 545 starvation: completed=%d/%d overBound=%d\n%s", completed, expected, slow, strings.Join(failures, "\n"))
	}
}
