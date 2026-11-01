package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0869_WorktreeSnapshotRefusesAMovingTree: queue snapshot
// --worktree rereads the tree after the body, as the inventory reader does:
// a tree that keeps changing is refused SNAPSHOT_MOVED with nothing
// written, and one change is absorbed by a fresh attempt whose export
// holds the settled state only.
type snapRes struct {
	Outcome string   `json:"outcome"`
	Codes   []string `json:"codes"`
}

func TestTCLV0869_WorktreeSnapshotRefusesAMovingTree(t *testing.T) {
	run := func(r *fixture.Repo, out string, change func(int)) *snapRes {
		t.Helper()
		calls := 0
		var buf bytes.Buffer
		queueSnapshot(Env{Cwd: t.TempDir(), Stdout: &buf, Stderr: &buf, afterRead: func() {
			calls++
			change(calls)
		}}, []string{"--worktree", r.Root, "--out", out})
		var res snapRes
		if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
			t.Fatalf("decode %s: %v", buf.String(), err)
		}
		return &res
	}
	t.Run("moving", func(t *testing.T) {
		r := fixture.TempRepo(t)
		fixture.WriteIntent(t, r, fixture.Ticket("A"))
		out := filepath.Join(t.TempDir(), "s.json")
		got := run(r, out, func(n int) {
			v := fixture.Ticket("A")
			v.Title = strings.Repeat("x", n+1)
			fixture.WriteIntent(t, r, v)
		})
		if len(got.Codes) != 1 || got.Codes[0] != string(wire.CodeSnapshotMoved) {
			t.Fatalf("moving tree: %+v", got)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("a refused snapshot wrote %s: %v", out, err)
		}
	})
	t.Run("settles", func(t *testing.T) {
		r := fixture.TempRepo(t)
		fixture.WriteIntent(t, r, fixture.Ticket("A"))
		out := filepath.Join(t.TempDir(), "s.json")
		got := run(r, out, func(n int) {
			if n == 1 {
				v := fixture.Ticket("A")
				v.Title = "settled title"
				fixture.WriteIntent(t, r, v)
			}
		})
		raw, err := os.ReadFile(out)
		if got.Outcome != "OK" || err != nil || !bytes.Contains(raw, []byte("settled title")) {
			t.Fatalf("settled tree: %+v %v %.300s", got, err, raw)
		}
	})
}
