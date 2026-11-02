// SPDX-License-Identifier: Apache-2.0

package store

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// preloadOf is the CLI preload's capture of repo's intent tree.
func preloadOf(t *testing.T, repo *intent.Repository) *intent.Tree {
	t.Helper()
	tree, err := intent.TreeDigest(repo.IntentRoot())
	if err != nil {
		t.Fatal(err)
	}
	return &tree
}

// sameSizeEdit replaces the first byte of the title value in raw with another
// letter, keeping the length.
func sameSizeEdit(t *testing.T, raw []byte) []byte {
	t.Helper()
	at := bytes.Index(raw, []byte(`"title":"`))
	if at < 0 {
		t.Fatalf("no title in %s", raw)
	}
	out := bytes.Clone(raw)
	i := at + len(`"title":"`)
	if out[i] == 'Z' {
		out[i] = 'Y'
	} else {
		out[i] = 'Z'
	}
	return out
}

// restoreTree makes parent exactly the copy at backup.
func restoreTree(t *testing.T, parent, backup string) {
	t.Helper()
	if err := os.RemoveAll(parent); err != nil {
		t.Fatal(err)
	}
	copyTree(t, backup, parent)
}

// TestLGLV0845WriterTakesPreloadBytes: with the preload capture handed in,
// the writer route commits the write and publishes exactly what the same
// write publishes without it. A capture whose ticket bytes were replaced by
// other bytes of the same size makes the write refuse and publish nothing,
// so the writer audit took its bytes from the capture, and the pre-effect
// recheck, not the capture, decided what may be written.
func TestLGLV0845WriterTakesPreloadBytes(t *testing.T) {
	t.Parallel()
	repo := writerStore(t, 70)
	parent := filepath.Dir(repo.PrimaryWorktree)
	wcPath := journal.WriterCheckpointPath(repo.StateDir)
	backup := filepath.Join(t.TempDir(), "store")
	copyTree(t, parent, backup)

	now := WallClock()
	honest := preloadOf(t, repo)
	run := stagedWrite(func(ctx context.Context) (*Report, error) {
		return Mutate(WithPreloadTree(ctx, honest), repo, historyActor, historyCreate("preload-write", "preload write"), now)
	}, nil)
	if !run.completed() || !run.fast() {
		t.Fatalf("with the preload: %+v %v %v", run.rep, run.err, run.stages)
	}
	withPreload := storeFiles(t, parent, wcPath)

	restoreTree(t, parent, backup)
	plain := stagedWrite(func(ctx context.Context) (*Report, error) {
		return Mutate(ctx, repo, historyActor, historyCreate("preload-write", "preload write"), now)
	}, nil)
	if !plain.completed() || !plain.fast() {
		t.Fatalf("without the preload: %+v %v %v", plain.rep, plain.err, plain.stages)
	}
	for p, d := range storeFiles(t, parent, wcPath) {
		if withPreload[p] != d {
			t.Errorf("%s: with the preload %q, without %q", p, withPreload[p], d)
		}
	}

	restoreTree(t, parent, backup)
	forged := preloadOf(t, repo)
	for i, f := range forged.Files {
		if filepath.Dir(f.Path) == intent.TicketsDir {
			forged.Files[i].Raw = sameSizeEdit(t, f.Raw)
			break
		}
	}
	before := mutationPublished(t, repo)
	refused := stagedWrite(func(ctx context.Context) (*Report, error) {
		return Mutate(WithPreloadTree(ctx, forged), repo, historyActor, historyCreate("preload-write", "preload write"), WallClock())
	}, nil)
	if refused.completed() {
		t.Fatalf("a capture with other bytes committed: %+v %v", refused.rep, refused.stages)
	}
	if after := mutationPublished(t, repo); after != before {
		t.Fatalf("published %s, before %s", after, before)
	}
}

// TestLGLV0845ChangedAfterPreloadRefused: a ticket edited on disk after the
// preload captured it, keeping its size, never reaches an effect: the write
// that hands in the stale capture is refused and publishes nothing.
func TestLGLV0845ChangedAfterPreloadRefused(t *testing.T) {
	t.Parallel()
	repo := writerStore(t, 70)
	stale := preloadOf(t, repo)
	_, ticketFile := mutationBoundaryFiles(t, repo)
	raw, err := os.ReadFile(ticketFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ticketFile, sameSizeEdit(t, raw), 0o644); err != nil {
		t.Fatal(err)
	}
	before := mutationPublished(t, repo)
	run := stagedWrite(func(ctx context.Context) (*Report, error) {
		return Mutate(WithPreloadTree(ctx, stale), repo, historyActor, historyCreate("stale-preload", "stale preload"), WallClock())
	}, nil)
	if run.completed() || run.err == nil {
		t.Fatalf("a write over a stale capture committed: %+v %v %v", run.rep, run.err, run.stages)
	}
	if after := mutationPublished(t, repo); after != before {
		t.Fatalf("published %s, before %s", after, before)
	}
	t.Logf("refused %s: %v", wire.CodeOf(run.err), run.err)
}
