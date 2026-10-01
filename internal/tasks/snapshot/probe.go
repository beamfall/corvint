package snapshot

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
// to the §TM-V0-008 value of three; Patience to DefaultPatience.
type Reader struct {
	StateDir   string
	IntentTree func() (wire.Digest, error)
	// Retries bounds the re-reads after a moved snapshot: zero means three,
	// negative means none.
	Retries int
	// Patience bounds the wall-clock time one read waits for an in-flight
	// writer (CTS-V0-006): a committed receipt that head.json does not name
	// yet (REDO_PENDING) is re-probed with backoff until it is applied or the
	// budget is spent, and the pause before each moved re-read is charged to
	// the same budget. Zero means DefaultPatience; negative means no waiting,
	// so a pending redo is reported at once and moved re-reads do not pause.
	Patience time.Duration
	// Sleep is the wait primitive; nil means time.Sleep. Tests use it to
	// finish a simulated writer during the wait.
	Sleep func(time.Duration)
}

// DefaultPatience is how long a read waits for an in-flight writer before it
// reports REDO_PENDING or SNAPSHOT_MOVED. The §5.2 effect order links the
// receipt in before the post files and the head, so a healthy writer leaves
// the pending window within milliseconds; two seconds covers a loaded host,
// while a writer that crashed in the window is still reported, not hidden.
// It is a variable only so a test binary can opt out of waiting
// (internal/tasks/fixture sets it to NoPatience); production code never
// assigns it.
var DefaultPatience = 2 * time.Second

// NoPatience is the Reader.Patience (or DefaultPatience) value that reports a
// pending redo at once and never pauses before a moved re-read.
const NoPatience time.Duration = -1

// readBackoff is the pause before re-read n (zero-based): 25 ms doubling to
// a 400 ms ceiling, so a short window costs little and a long wait does not
// hammer the store.
func readBackoff(n int) time.Duration {
	d := 25 * time.Millisecond << uint(n)
	if n > 4 || d > 400*time.Millisecond {
		return 400 * time.Millisecond
	}
	return d
}

// transient reports whether a probe failure is one a concurrent writer causes
// and a later probe can clear: a pending redo, or an intent file replaced
// between stat and open while the tree digest was being taken.
func transient(err error) bool {
	switch wire.CodeOf(err) {
	case wire.CodeRedoPending, wire.CodeSnapshotMoved:
		return true
	}
	return false
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
// compare; retry a moved snapshot at most Retries times; wait out an
// in-flight writer for at most Patience; then NOT_RUN/SNAPSHOT_MOVED or
// REDO_PENDING. The body receives a snapshot it may read files under; the
// returned snapshot is the one the body last saw when the read succeeded,
// or the partial snapshot a failed probe observed (possibly nil) together
// with the error.
//
// A body failure is re-probed before it is reported (TM-V0-008): when the
// re-probe shows the same snapshot the store is stable and the body's own
// error (JOURNAL_FORKED, MALFORMED, ...) is returned verbatim; when the
// re-probe shows a different snapshot the body read across a concurrent
// commit or intent edit, so the failure is a move, not a fact about the
// store, and the attempt is retried like any other moved read. A store
// that is both moving and corrupt therefore ends as SNAPSHOT_MOVED, never
// as a success; a corrupt store that is stable is never reported as moved.
//
// A probe that finds a committed receipt head.json does not name yet
// (REDO_PENDING) is a writer part-way through the §5.2 effect order, not a
// fact about the store either (CTS-V0-006): the read pauses with backoff
// and probes again until the writer has applied the receipt or Patience is
// spent, and only then reports REDO_PENDING, naming the wait. A moved
// snapshot is likewise paused on and re-read, uncounted, while Patience
// lasts; the Retries bound applies to the unpaused re-reads after it is
// spent, so a test binary without patience sees exactly the TM-V0-008
// attempt count. Nothing is written or locked while waiting; a reader never
// redoes the receipt.
func (r Reader) Read(body func(s *Snapshot) error) (*Snapshot, error) {
	retries := r.Retries
	if retries == 0 {
		retries = 3
	} else if retries < 0 {
		retries = 0
	}
	patience := r.Patience
	if patience == 0 {
		patience = DefaultPatience
	}
	if patience < 0 {
		patience = 0
	}
	sleep := r.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	start := time.Now()
	deadline := start.Add(patience)
	waits, stall := 0, 0
	// wait pauses for the next backoff step of the current stall, clipped
	// to the remaining budget, and reports false once the budget is spent.
	wait := func() bool {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}
		d := readBackoff(stall)
		if d > remaining {
			d = remaining
		}
		waits++
		stall++
		sleep(d)
		return true
	}
	var last *Snapshot
	attempts, moved := 0, 0
	for {
		s, err := r.probe()
		if err != nil {
			if transient(err) && wait() {
				continue
			}
			return s, afterWait(err, start, waits)
		}
		stall = 0
		attempts++
		bodyErr := body(s)
		again, err := r.probe()
		if err != nil {
			// The store is no longer in a readable state: a fork is reported
			// verbatim, as a read after a successful body would; a commit in
			// flight (REDO_PENDING) is waited out like one seen before the body.
			if transient(err) && wait() {
				continue
			}
			return again, afterWait(err, start, waits)
		}
		if Same(s, again) {
			return s, bodyErr
		}
		last = again
		if wait() {
			continue
		}
		moved++
		if moved > retries {
			break
		}
	}
	return last, afterWait(wire.Errorf(wire.CodeSnapshotMoved, r.StateDir, "the store changed during every one of %d read attempts", attempts), start, waits)
}

// afterWait names the wait on a failure reported after at least one pause,
// so a consumer can tell a waited-out writer from an instant refusal; an
// error reported without waiting is returned verbatim.
func afterWait(err error, start time.Time, waits int) error {
	e, ok := err.(*wire.Error)
	if !ok || waits == 0 {
		return err
	}
	return &wire.Error{Code: e.Code, Where: e.Where, Msg: fmt.Sprintf("%s; still so after %d pauses totalling %s for a concurrent writer; the read is retryable", e.Msg, waits, time.Since(start).Round(time.Millisecond))}
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
