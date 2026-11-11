package gitrun

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ALO-V0-017: replacing a legacy budget at each call cannot reset physical
// admission. Both ordinary and explicitly pre-reserved calls use the same cap.
func TestOperationBudgetCountsPhysicalNestedSpawns(t *testing.T) {
	budget, err := NewOperationBudget(64, time.Now().Add(time.Minute), false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithOperationBudget(context.Background(), budget)
	for i := 0; i < 64; i++ {
		options := Options{Env: os.Environ(), StdoutLimit: 4096}
		var raw []byte
		if i%2 == 0 {
			raw, err = Run(ctx, NewDefaultBudget(), options, "--version")
		} else {
			raw, err = RunReserved(ctx, time.Second, options, "--version")
		}
		if err != nil || !strings.HasPrefix(string(raw), "git version ") {
			t.Fatalf("spawn %d: %q %v", i+1, raw, err)
		}
	}
	if _, err := Run(ctx, NewDefaultBudget(), Options{StdoutLimit: 4096}, "--version"); err == nil || err.Error() != "aggregate-git-operation-limit" {
		t.Fatalf("65th spawn admitted: %v", err)
	}
	if budget.Used() != 64 {
		t.Fatalf("physical count=%d", budget.Used())
	}
}

// ALO-V0-017: concurrent callers share admission, and failed starts stay charged.
func TestOperationBudgetConcurrentAdmissionAndFailedStart(t *testing.T) {
	budget, _ := NewOperationBudget(64, time.Now().Add(time.Minute), false)
	var admitted atomic.Int32
	var group sync.WaitGroup
	for i := 0; i < 128; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := budget.reserve(context.Background(), time.Second); err == nil {
				admitted.Add(1)
			}
		}()
	}
	group.Wait()
	if admitted.Load() != 64 || budget.Used() != 64 {
		t.Fatalf("admissions=%d used=%d", admitted.Load(), budget.Used())
	}
	budget, _ = NewOperationBudget(1, time.Now().Add(time.Minute), false)
	ctx := WithOperationBudget(context.Background(), budget)
	if _, err := Run(ctx, NewDefaultBudget(), Options{Binary: "/nonexistent/aggregate-test-git", StdoutLimit: 4096}); err == nil {
		t.Fatal("bad executable succeeded")
	}
	if budget.Used() != 1 {
		t.Fatal("failed start was refunded")
	}
}

// ALO-V0-017: quota transfer reserves closing work without refreshing time or
// accepting missing, duplicate or oversized worker accounting.
func TestOperationBudgetOriginalDeadlineAndWorkerQuota(t *testing.T) {
	now := time.Now()
	budget, _ := NewOperationBudget(64, now.Add(time.Minute), false)
	budget.now = func() time.Time { return now }
	if _, err := budget.reserve(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}
	quota, remaining, err := budget.Delegate(8)
	if err != nil || quota != 55 || remaining != time.Minute {
		t.Fatalf("quota=%d remaining=%s err=%v", quota, remaining, err)
	}
	if err := budget.Settle(56); err == nil {
		t.Fatal("oversized accounting admitted")
	}
	if err := budget.Settle(20); err != nil {
		t.Fatal(err)
	}
	if err := budget.Settle(20); err == nil {
		t.Fatal("duplicate accounting admitted")
	}
	if budget.Used() != 21 {
		t.Fatalf("used=%d", budget.Used())
	}
	now = now.Add(59 * time.Second)
	duration, err := budget.reserve(context.Background(), 10*time.Second)
	if err != nil || duration != time.Second {
		t.Fatalf("refreshed deadline: %s %v", duration, err)
	}
	now = now.Add(time.Second)
	if _, err := budget.reserve(context.Background(), time.Second); err == nil || err.Error() != "aggregate-operation-deadline" {
		t.Fatalf("expired work admitted: %v", err)
	}
}
