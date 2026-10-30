package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/defects"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTDRV0010_DefectWriteAppendsOnceAcrossStoreRetries: when Tasks state
// moves during the store read, the read retries, but one `defects add` or
// `defects update` still appends exactly one register line.
func TestTDRV0010_DefectWriteAppendsOnceAcrossStoreRetries(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.WriteIntent(t, r)
	moved := 0
	retrying := func(ticket string) Env {
		calls := 0
		return Env{Cwd: r.Root, afterRead: func() {
			calls++
			if calls == 1 {
				moved++
				fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/" + ticket + ".json": fixture.Ticket(ticket).Encode()})
			}
		}}
	}
	add := defectsCommand(retrying("A"), []string{"add", "--title", "proxy resets", "--location", "e2e/a.spec.ts:3", "--evidence", "ci run 7"})
	if add.Outcome != wire.OutcomeOK || len(add.Items) != 1 {
		t.Fatalf("add: %+v", add)
	}
	upd := defectsCommand(retrying("B"), []string{"update", "DEF-1", "--status", "CONFIRMED"})
	if upd.Outcome != wire.OutcomeOK || len(upd.Items) != 1 {
		t.Fatalf("update: %+v", upd)
	}
	if moved != 2 {
		t.Fatalf("the store read did not observe a move: %d", moved)
	}
	raw, err := os.ReadFile(filepath.Join(r.Root, ".git", defects.PrivateDir, defects.RegisterFile))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(raw), "\n"); n != 2 {
		t.Fatalf("register holds %d lines, want one ADD and one UPDATE:\n%s", n, raw)
	}
}
