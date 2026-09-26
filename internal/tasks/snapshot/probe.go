package snapshot

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Snapshot is one consistent observation of the state dir (§3.4) taken
// under the TM-V0-008 protocol.
type Snapshot struct {
	StateDir       string
	Version        []byte
	Head           *Head
	HeadRaw        []byte
	HeadSha256     wire.Digest
	Barrier        *Barrier
	BarrierRaw     []byte // nil when barrier.json is absent
	LastReceiptRaw []byte
	IntentTree     wire.Digest // zero when the reader has no intent tree function
}

// Reader configures the protocol. IntentTree, when set, is evaluated inside
// the snapshot and re-checked afterwards, so a read never mixes an intent
// tree from one snapshot with journal state from another. Retries defaults
// to the §TM-V0-008 value of three.
type Reader struct {
	StateDir   string
	IntentTree func() (wire.Digest, error)
	Retries    int
}

// Exists reports whether the state dir has been initialised at all
// (VERSION present). It reads nothing else.
func Exists(stateDir string) bool {
	fi, err := os.Lstat(filepath.Join(stateDir, "VERSION"))
	return err == nil && fi.Mode().IsRegular()
}

// Probe performs the head-and-slots check once: it reports UNINITIALIZED,
// UNSUPPORTED_VERSION, UNSUPPORTED_FILESYSTEM, RESTORE_INCOMPLETE,
// REDO_PENDING or JOURNAL_FORKED verbatim and otherwise returns the
// observed head, barrier and last receipt.
func Probe(stateDir string) (*Snapshot, error) {
	if err := intent.CheckNoSymlink(stateDir); err != nil {
		return nil, err
	}
	fi, err := os.Lstat(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, wire.Errorf(wire.CodeUninitialized, stateDir, "state dir does not exist; run `corvint-tasks init` (TCP-02)")
		}
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, stateDir, "cannot stat: %v", err)
	}
	if !fi.IsDir() {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, stateDir, "state dir is not a directory")
	}
	if _, err := os.Lstat(filepath.Join(stateDir, "RESTORE_INCOMPLETE")); err == nil {
		return nil, wire.Errorf(wire.CodeRestoreIncomplete, stateDir, "an interrupted restore left its marker; only `archive restore` may continue")
	}
	s := &Snapshot{StateDir: stateDir}
	ver, err := intent.ReadFile(filepath.Join(stateDir, "VERSION"), 64)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, wire.Errorf(wire.CodeUninitialized, stateDir, "VERSION is absent; run `corvint-tasks init` (TCP-02)")
		}
		return nil, err
	}
	if string(ver) != VersionBytes {
		return nil, wire.Errorf(wire.CodeUnsupportedVersion, filepath.Join(stateDir, "VERSION"), "state version %q is not %q; reads never migrate", string(ver), VersionBytes)
	}
	s.Version = ver
	headRaw, err := intent.ReadFile(filepath.Join(stateDir, "head.json"), wire.MaxJournalHeadBytes)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, wire.Errorf(wire.CodeUninitialized, stateDir, "head.json is absent; `corvint-tasks init` did not complete")
		}
		return nil, err
	}
	head, err := DecodeHead(headRaw)
	if err != nil {
		return nil, prefixWhere(err, "head.json")
	}
	s.Head = head
	s.HeadRaw = headRaw
	s.HeadSha256 = wire.Sum(headRaw)
	// From here on a failure returns the partial snapshot with the error so
	// a read can still report the head it saw (REDO_PENDING, JOURNAL_FORKED).
	if err := checkSlots(stateDir, head); err != nil {
		return s, err
	}
	name, err := ReceiptName(head.LastSeq.Uint64())
	if err != nil {
		return s, err
	}
	last, err := intent.ReadFile(filepath.Join(stateDir, "receipts", name), wire.MaxReceiptFileBytes)
	if err != nil {
		if os.IsNotExist(err) {
			return s, wire.Errorf(wire.CodeJournalForked, filepath.Join(stateDir, "receipts", name), "head names receipt %s which is absent (head ahead of receipts)", name)
		}
		return s, err
	}
	if wire.Sum(last) != *head.LastReceiptSha256 {
		return s, wire.Errorf(wire.CodeJournalForked, filepath.Join(stateDir, "receipts", name), "receipt digest differs from head.lastReceiptSha256")
	}
	s.LastReceiptRaw = last
	braw, err := intent.ReadFile(filepath.Join(stateDir, "barrier.json"), wire.MaxBarrierBytes)
	if err == nil {
		b, err := DecodeBarrier(braw)
		if err != nil {
			return nil, prefixWhere(err, "barrier.json")
		}
		s.Barrier = b
		s.BarrierRaw = braw
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

// checkSlots applies the slot rule: `<lastSeq+2>` present is JOURNAL_FORKED
// (checked first so a fork is never reported as a mere pending redo, N4b);
// `<lastSeq+1>` present is REDO_PENDING.
func checkSlots(stateDir string, head *Head) error {
	seq := head.LastSeq.Uint64()
	for _, delta := range []uint64{2, 1} {
		if seq+delta < seq {
			return wire.Errorf(wire.CodeMalformed, "head.json/lastSeq", "lastSeq overflow")
		}
		name, err := ReceiptName(seq + delta)
		if err != nil {
			return err
		}
		p := filepath.Join(stateDir, "receipts", name)
		if _, err := os.Lstat(p); err == nil {
			if delta == 2 {
				return wire.Errorf(wire.CodeJournalForked, p, "receipt %s exists more than one beyond head.lastSeq %s", name, head.LastSeq)
			}
			return wire.Errorf(wire.CodeRedoPending, p, "receipt %s is committed but not yet applied to head.json; a mutating command will redo it", name)
		} else if !os.IsNotExist(err) {
			return wire.Errorf(wire.CodeUnsupportedFilesystem, p, "cannot stat: %v", err)
		}
	}
	return nil
}

// Read runs the whole protocol: probe, intent tree, body, re-probe,
// compare; retry at most Retries times; then NOT_RUN/SNAPSHOT_MOVED. The
// body receives a snapshot it may read files under; the returned snapshot
// is the one the body last saw when the read succeeded, or the partial
// snapshot a failed probe observed (possibly nil) together with the error.
//
// A body failure is re-probed before it is reported (TM-V0-008): when the
// re-probe shows the same snapshot the store is stable and the body's own
// error (JOURNAL_FORKED, MALFORMED, ...) is returned verbatim; when the
// re-probe shows a different snapshot the body read across a concurrent
// commit or intent edit, so the failure is a move, not a fact about the
// store, and the attempt is retried like any other moved read. A store
// that is both moving and corrupt therefore ends as SNAPSHOT_MOVED, never
// as a success; a corrupt store that is stable is never reported as moved.
func (r Reader) Read(body func(s *Snapshot) error) (*Snapshot, error) {
	retries := r.Retries
	if retries == 0 {
		retries = 3
	} else if retries < 0 {
		retries = 0
	}
	var last *Snapshot
	for attempt := 0; attempt <= retries; attempt++ {
		s, err := r.probe()
		if err != nil {
			return s, err
		}
		bodyErr := body(s)
		again, err := r.probe()
		if err != nil {
			// The store is no longer in a readable state (a commit in flight
			// shows as REDO_PENDING, a fork as JOURNAL_FORKED); report the
			// re-probe verbatim, as a read after a successful body would.
			return again, err
		}
		if Same(s, again) {
			return s, bodyErr
		}
		last = again
	}
	return last, wire.Errorf(wire.CodeSnapshotMoved, r.StateDir, "the store changed during every one of %d read attempts", retries+1)
}

// probe is Probe plus the intent tree digest when the reader has one.
func (r Reader) probe() (*Snapshot, error) {
	s, err := Probe(r.StateDir)
	if err != nil {
		return s, err
	}
	if r.IntentTree != nil {
		d, err := r.IntentTree()
		if err != nil {
			return s, err
		}
		s.IntentTree = d
	}
	return s, nil
}

// Same reports whether two probes observed the same snapshot: identical head
// bytes, identical barrier presence and bytes, identical intent tree digest.
func Same(a, b *Snapshot) bool {
	if !bytes.Equal(a.HeadRaw, b.HeadRaw) {
		return false
	}
	if (a.BarrierRaw == nil) != (b.BarrierRaw == nil) || !bytes.Equal(a.BarrierRaw, b.BarrierRaw) {
		return false
	}
	return a.IntentTree == b.IntentTree
}

// EnvelopeSnapshot renders the `snapshot` object of a command result for a
// successful probe.
func (s *Snapshot) EnvelopeSnapshot(primaryWorktreeSha256 wire.Digest, pendingRedo bool) *wire.Snapshot {
	out := &wire.Snapshot{PendingRedo: pendingRedo}
	if s == nil {
		return out
	}
	if s.Head != nil {
		seq := s.Head.LastSeq
		out.HeadSeq = &seq
		if s.Head.LastReceiptSha256 != nil {
			d := *s.Head.LastReceiptSha256
			out.HeadReceiptSha256 = &d
		}
	}
	if s.IntentTree != "" {
		d := s.IntentTree
		out.IntentTreeSha256 = &d
	}
	if primaryWorktreeSha256 != "" {
		d := primaryWorktreeSha256
		out.PrimaryWorktreeSha256 = &d
	}
	if s.Barrier != nil {
		out.Barrier = &wire.BarrierRef{Scope: s.Barrier.Scope, Reason: s.Barrier.Reason}
	}
	return out
}

func prefixWhere(err error, file string) error {
	e, ok := err.(*wire.Error)
	if !ok {
		return err
	}
	where := file
	if e.Where != "" && e.Where != "/" {
		where = file + e.Where
	}
	return &wire.Error{Code: e.Code, Where: where, Msg: e.Msg}
}
