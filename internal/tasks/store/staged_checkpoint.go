package store

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
)

// stagedInfix names the private temporaries a lease commit stages before
// the writer lock. Each staging process creates its own name, so no lock is
// needed to keep two of them apart.
const stagedInfix = ".staged-"

// stagedMaxAge is how old a staged temporary must be before a later stager
// removes it as a leftover of a stopped process. A live stager renames or
// removes its own temporary within one lease commit; removing one too early
// only means that commit retains nothing, which costs a later read or write
// one complete audit.
const stagedMaxAge = 10 * time.Minute

// stageCreate creates a staged temporary; a test replaces it to fail staging.
var stageCreate = os.CreateTemp

// derivedWritten, when a test sets it, observes every write of derived
// checkpoint bytes: the destination path and the byte count.
var derivedWritten func(path string, n int)

// noteDerived reports one derived write to derivedWritten.
func noteDerived(path string, n int) {
	if derivedWritten != nil {
		derivedWritten(path, n)
	}
}

// stagedFile is derived bytes already written beside their destination, so
// the commit under the writer lock only renames them into place (JSG-V0-015,
// V1-0850). A nil stagedFile stages nothing.
type stagedFile struct {
	path, tmp string
}

// stageDerived writes raw to a new private (0600) temporary beside path. It
// returns nil, leaving nothing behind, when the temporary cannot be written:
// the bytes are derived state, so failing to stage them only skips their
// retention.
func stageDerived(path string, raw []byte) *stagedFile {
	sweepStaged(path)
	f, err := stageCreate(filepath.Dir(path), filepath.Base(path)+stagedInfix+"*")
	if err != nil {
		return nil
	}
	_, err = f.Write(raw)
	noteDerived(path, len(raw))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(f.Name())
		return nil
	}
	return &stagedFile{path: path, tmp: f.Name()}
}

// sweepStaged removes staged temporaries of path older than stagedMaxAge.
// It lists only path's directory, the parent of the state directory, never
// the store.
func sweepStaged(path string) {
	dir, prefix := filepath.Dir(path), filepath.Base(path)+stagedInfix
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), prefix) || !e.Type().IsRegular() {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > stagedMaxAge {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// publish renames the staged bytes over their destination. The caller holds
// the writer lock and has checked that the head they were derived from is
// still the store's head. It reports whether the bytes are now in place.
func (s *stagedFile) publish() bool {
	if s == nil || s.tmp == "" {
		return false
	}
	err := os.Rename(s.tmp, s.path)
	if err != nil {
		os.Remove(s.tmp)
	}
	s.tmp = ""
	return err == nil
}

// discard removes a temporary that was not published.
func (s *stagedFile) discard() {
	if s != nil && s.tmp != "" {
		os.Remove(s.tmp)
		s.tmp = ""
	}
}

// stageReadCheckpoint derives and stages the read checkpoint proof supports,
// the O(store) part of retainCheckpoint, before the writer lock. It returns
// nil when proof supports none.
func stageReadCheckpoint(repo *intent.Repository, proof *journal.Result) *stagedFile {
	cp := proof.Checkpoint()
	if cp == nil {
		return nil
	}
	return stageDerived(journal.CheckpointPath(repo.StateDir), cp.Encode())
}

// stagedWriterCheckpoint is a writer checkpoint encoded before the writer
// lock under the invalidation token the lease preparation read. Its fresh
// base, the part that grows with the store, is staged; its delta is bounded
// by the active set and is written under the lock.
type stagedWriterCheckpoint struct {
	wc    *journal.WriterCheckpoint
	token writerToken
	base  *stagedFile
	delta []byte
	// failed marks a fresh base that could not be encoded or staged: the
	// commit then retains no writer checkpoint rather than encoding the base
	// under the lock (JSG-V0-015, JSG-V0-891).
	failed bool
}

// stageWriterCheckpoint encodes wc under since and stages its fresh base. It
// returns nil when there is nothing O(store) to stage: no checkpoint, an
// unreadable token or a base the checkpoint was decoded with; and a failed
// result when a fresh base could not be encoded or staged.
func stageWriterCheckpoint(repo *intent.Repository, wc *journal.WriterCheckpoint, since writerToken) *stagedWriterCheckpoint {
	if wc == nil || !since.ok || !wc.FreshBase() || wc.Seq.Uint64() < writerCheckpointMin(repo) {
		return nil
	}
	wc.Invalidation = since.digest()
	failed := &stagedWriterCheckpoint{wc: wc, token: since, failed: true}
	base, delta, err := wc.Encode()
	if err != nil {
		return failed
	}
	staged := stageDerived(journal.WriterBasePath(repo.StateDir), base)
	if staged == nil {
		return failed
	}
	return &stagedWriterCheckpoint{wc: wc, token: since, base: staged, delta: delta}
}

// discard removes the staged base if it was not published.
func (s *stagedWriterCheckpoint) discard() {
	if s != nil {
		s.base.discard()
	}
}

// retainStagedWriterCheckpoint is retainWriterCheckpoint for a checkpoint
// staged before the lock. The caller holds the writer lock. The staged bytes
// are used only when they carry the token this commit retains under, which
// is the token the lock holder reads now; otherwise, as when the complete
// audit just refuted the bound checkpoint and replaced the token, it falls
// back to encoding under the lock.
func retainStagedWriterCheckpoint(repo *intent.Repository, wc *journal.WriterCheckpoint, since writerToken, staged *stagedWriterCheckpoint) {
	if staged != nil && staged.failed && staged.wc == wc {
		return
	}
	if staged == nil || staged.wc != wc || staged.token != since {
		retainWriterCheckpoint(repo, wc, since)
		return
	}
	if !since.ok || writerInvalidation(repo) != since {
		return
	}
	// As retainWriterCheckpoint: the new base lands before the delta that
	// binds it (JSG-V0-004).
	if !staged.base.publish() {
		return
	}
	replaceDerived(journal.WriterCheckpointPath(repo.StateDir), staged.delta)
}
