package store_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// The scale measurements are opt-in: CORVINT_TASKS_SCALE=1 runs them.
// CORVINT_TASKS_SCALE_ITEMS sets the export size (default 2894 for the
// first import, 3000 for the write fixture) and CORVINT_TASKS_SCALE_WRITES
// the number of timed writes (default 20 of each kind).
func scaleEnv(t *testing.T, name string, fallback int) int {
	t.Helper()
	if os.Getenv("CORVINT_TASKS_SCALE") == "" {
		t.Skip("set CORVINT_TASKS_SCALE=1 to run the scale measurement")
	}
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		t.Fatalf("%s=%q", name, raw)
	}
	return n
}

// scaleExport is n synthetic items of about the Beamfall export's mean size
// (14 MB over 2,894 items): a 2 KiB block that is also the ticket body.
func scaleExport(n int) []byte {
	items := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("BF-%d", i+1)
		block := "## " + id + "\n" + strings.Repeat("synthetic backlog text. ", 85) + "\n"
		items = append(items, importItem(id, block, nil))
	}
	return importExport(items...)
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	i := int(float64(len(sorted)-1) * p)
	return sorted[i]
}

func timedImport(t *testing.T, repo *intent.Repository, n int) {
	t.Helper()
	export := scaleExport(n)
	start := time.Now()
	report := runImport(t, repo, export)
	t.Logf("first import: %d items, %d batches, %v", report.Items, len(report.Batches), time.Since(start).Round(time.Millisecond))
}

// TestCALV0018_FirstImportScale times one first import of a synthetic
// 2,894-item export (CAL-V0-018: under 5 minutes at load below the CPU count).
func TestCALV0018_FirstImportScale(t *testing.T) {
	n := scaleEnv(t, "CORVINT_TASKS_SCALE_ITEMS", 2894)
	timedImport(t, importStore(t, "ROADMAP"), n)
}

// TestCALV0026_WriteLockHoldScale times cheap §5.2 writes against a synthetic
// 3,000-ticket store: PAUSE/UNPAUSE pairs and ticket field mutations, and the
// intent tree digest every write still computes on its own. The
// command wall time bounds the lock hold from above (CAL-V0-026: p95 under
// 500 ms at load below the CPU count).
func TestCALV0026_WriteLockHoldScale(t *testing.T) {
	n := scaleEnv(t, "CORVINT_TASKS_SCALE_ITEMS", 3000)
	writes := scaleEnv(t, "CORVINT_TASKS_SCALE_WRITES", 20)
	repo := importStore(t, "ROADMAP")
	timedImport(t, repo, n)
	ctx := context.Background()
	tree, barrier, mutations := []time.Duration{}, []time.Duration{}, []time.Duration{}
	for i := 0; i < 5; i++ {
		start := time.Now()
		if _, err := intent.TreeDigest(repo.PrimaryWorktree); err != nil {
			t.Fatal(err)
		}
		tree = append(tree, time.Since(start))
	}
	for i := 0; i < writes; i++ {
		for _, op := range []string{transaction.Pause, transaction.Unpause} {
			start := time.Now()
			report, err := store.Barrier(ctx, repo, operator(), barrierRequest(op, fmt.Sprintf("%s-%d", strings.ToLower(op), i)), now(t))
			barrier = append(barrier, time.Since(start))
			if err != nil || report.Receipt == "" {
				t.Fatalf("%s: %+v %v", op, report, err)
			}
		}
	}
	for i := 0; i < writes; i++ {
		env := envelope(fmt.Sprintf("prioritize-%d", i), mutation.OpPrioritize, fixture.TicketID("BF-1"), strconv.Itoa(i+1), obj("priority", str("P1"), "order", str(strconv.Itoa(i))))
		start := time.Now()
		report, err := store.Mutate(ctx, repo, operator(), env, now(t))
		mutations = append(mutations, time.Since(start))
		if err != nil {
			t.Fatalf("mutate: %v", err)
		}
		if i == 0 {
			t.Logf("mutation outcome: %s %v %s", report.Outcome.Outcome, report.Outcome.Codes, report.Detail)
		}
	}
	for _, m := range []struct {
		name string
		d    []time.Duration
	}{{"intent tree digest", tree}, {"pause/unpause", barrier}, {"prioritize", mutations}} {
		sort.Slice(m.d, func(i, j int) bool { return m.d[i] < m.d[j] })
		t.Logf("%s: n=%d p50=%v p95=%v max=%v", m.name, len(m.d), percentile(m.d, 0.5).Round(time.Millisecond), percentile(m.d, 0.95).Round(time.Millisecond), m.d[len(m.d)-1].Round(time.Millisecond))
	}
}
