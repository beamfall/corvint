package store

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestRC3_SyntheticStore is an opt-in generator, not a check: it leaves a
// settled historyStoreAt fixture at CORVINT_TASKS_SYNTH_ROOT (which must not
// exist) for binary-level measurement, because a copy of a live store refuses
// relocation. CORVINT_TASKS_SYNTH_TICKETS and CORVINT_TASKS_SYNTH_RECEIPTS
// size it (defaults 880 and 3,000, the live store on 2026-10-06).
func TestRC3_SyntheticStore(t *testing.T) {
	root := os.Getenv("CORVINT_TASKS_SYNTH_ROOT")
	if root == "" {
		t.Skip("set CORVINT_TASKS_SYNTH_ROOT to an absent directory")
	}
	if !filepath.IsAbs(root) {
		t.Fatalf("root %q is not absolute", root)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("root %q exists or cannot be checked: %v", root, err)
	}
	size := func(name string, def int) int {
		raw := os.Getenv(name)
		if raw == "" {
			return def
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			t.Fatalf("%s=%q", name, raw)
		}
		return n
	}
	tickets := size("CORVINT_TASKS_SYNTH_TICKETS", 880)
	receipts := size("CORVINT_TASKS_SYNTH_RECEIPTS", 3000)
	if receipts <= tickets+1 {
		t.Fatalf("receipts %d must exceed tickets+1 (%d)", receipts, tickets+1)
	}
	historyStoreAt(t, root, tickets, receipts)
}
