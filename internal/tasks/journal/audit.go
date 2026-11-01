package journal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/evidencestore"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Identity pins the observed state for callers combining pure library results.
// It is not a cross-snapshot transaction or authority binding.
type Identity struct{ HeadSha256, InventorySha256, IntentTreeSha256 wire.Digest }

// Record is a selected canonical latest afterimage. A nil Raw means deletion;
// a non-nil zero-length Raw is present empty evidence.
type Record struct {
	Seq    wire.Size
	Sha256 *wire.Digest
	Raw    []byte
}

// HandoffPolicySelector names one current generation, never a renewed lease's
// GrantedSeq. It requests internal structural evidence, not admission authority.
type HandoffPolicySelector struct {
	AttemptID            string
	Generation           wire.Size
	OriginalPolicySha256 wire.Digest
	PoolID, MemberID     string
}

// HandoffPolicyHistory retains one original policy blob and bounded provenance.
// Compatible covers every policy post from that blob through the audited head;
// an incompatible intermediate revision remains incompatible after restoration.
type HandoffPolicyHistory struct {
	Selector                                      HandoffPolicySelector
	OriginalPolicy                                Record
	OriginalReceiptSha256                         wire.Digest
	FirstAttemptSeq                               wire.Size
	FirstAttemptSha256, FirstAttemptReceiptSha256 wire.Digest
	FirstPolicySha256, FirstConfigSha256          wire.Digest
	FirstCapabilitySha256, FirstAllocationSha256  wire.Digest
	FinalPolicySha256                             wire.Digest
	Compatible                                    bool
}

// Result claims only structural consistency and current projection agreement.
// Even known record codecs do not validate historical acceptance decisions.
type Result struct {
	Identity          Identity
	Head              *snapshot.Head
	LastSeq           wire.Size
	LastReceiptSha256 wire.Digest
	Pending           bool
	StagingPresent    bool
	// OrphanStageSlots names the descriptor-less staging slots this audit
	// set aside as a killed writer's orphans after a free writer lock proved
	// no writer live (CTS-V0-026). StagingPresent is then true; the next
	// writer removes them under its lock.
	OrphanStageSlots      []string
	StructuralConsistency string
	ProjectionAgreement   string
	SemanticCoverage      string
	HistoricalAcceptance  string
	ActorAuthentication   string
	Liveness              string
	RuntimeQualification  string
	Records               map[string]Record
	// RequestDigests retains bounded metadata, not historical afterimage bytes.
	// A writer reads the one requested projection under its change guard.
	RequestDigests map[string]wire.Digest
	HandoffPolicy  *HandoffPolicyHistory
	// IntentError is populated only by AuditForWrite. Private consistency is
	// still mandatory; stable intent divergence permits request replay only.
	IntentError error
	// Mode is ModeFull for an audit that walked every retained receipt.
	Mode          string
	fold          func(*snapshot.Receipt, wire.Digest) error
	folded        uint64
	foldErr       error
	chain         *chain
	request       *snapshot.Request
	requestTicket string
	// Set only by AuditForMutation: the per-observation selection and the
	// refusals a separate Audit of it would raise after the lookup succeeded.
	selection            map[string]bool
	selectErr, intentErr error
	// writerBase is the writer checkpoint a ModeWriter audit resumed from.
	writerBase *WriterCheckpoint
	// manifest is the segment manifest the audited head binds, if any; a
	// complete audit's writer checkpoint records its segment digests.
	manifest *snapshot.SegmentManifest
	// sealRebase marks the result a seal validated its range into: the
	// writer checkpoint it derives rebuilds the base (JSG-V0-009).
	sealRebase bool
	// listing is what a ModeWriter audit listed but did not read.
	listing map[string]ListedFile
	// liveness marks an AuditForLiveness result: the intent tree was not
	// listed and only queue.json and policy.json were read (JSG-V0-001).
	livenessWrite bool
	// resumeRead is what an AuditForResume walk consumed, by path; the
	// resume recheck rereads the tail against it (PSR-V0-116, proposed).
	resumeRead map[string]PhysicalFile
}

// Reader always streams receipt bytes, retaining only bounded path/digest
// metadata and explicitly selected records. All calls complete a fresh audit.
type Reader struct {
	Source          Source
	QueueID         wire.QueueID
	PrimaryWorktree string
	// SelectState also retains reservations.json, pools.json and every
	// attempt record, so one audit serves a whole read command.
	SelectState bool
	// Checkpoint opts a plain Audit into resuming from a writer-retained
	// complete audit (CAL-V0-059). It is rebound to the journal before use and
	// every refusal on that path is re-derived by the complete audit, so it
	// can only make an agreeing read cheaper. Result.Mode names what ran.
	Checkpoint *Checkpoint
	// IntentTree, when set, is the intent tree the caller's outer snapshot
	// hashed before this audit began and hashes again after it ends
	// (CAL-V0-140). Both captures take an intent file's bytes from it
	// instead of reading and hashing the file again; each capture still
	// stats every intent file into its inventory, and a file whose size
	// differs is read afresh, so a changed tree still moves the audit or
	// fails the outer snapshot's comparison.
	IntentTree *intent.Tree
	// ReceiptFold, when set, starts one fold per complete (ModeFull) audit
	// attempt over the receipts that attempt validates, in sequence order
	// from 1, so a caller that must fold every retained receipt does not
	// read and decode the journal again. The fold sees a receipt only after
	// the walk has fully validated it, is not called again after its first
	// error, and never changes the audit; Result.ReceiptFold reports it.
	ReceiptFold func() func(rc *snapshot.Receipt, sum wire.Digest) error
	// WriterLocked says the caller holds the writer lock, so no other native
	// writer can be publishing and any descriptor-less staging slot is a
	// killed writer's orphan: the audit refuses it at once instead of
	// waiting for it while blocking every other writer (CTS-V0-008).
	WriterLocked bool
	// ExpectHeadSha256, when set, is the head digest of the outer snapshot
	// the caller binds this audit to. An attempt whose first capture shows
	// any other head returns SNAPSHOT_MOVED at once, without walking or
	// falling back: the caller would discard any verdict about that head,
	// so its outer reader re-probes instead (V1-1060).
	ExpectHeadSha256 wire.Digest
	// PrefixFallback makes AuditForWriteObserved a lease preparation's
	// bounded audit (PSR-V0-050, proposed): one complete attempt, which gives
	// up as soon as head.json moves under it, then, only if the journal moved,
	// the prefix walk to the head that attempt captured (ModePrefix). A prefix
	// walk that refuses returns ErrPrefixDeclined, never its own refusal, so
	// the caller's complete audit derives every refusal itself.
	PrefixFallback bool
	// StopOnMove makes AuditForWriteObserved's complete audit one attempt
	// that gives up as soon as head.json moves under it (PSR-V0-150): the
	// caller's next preparation round samples the journal again, instead of
	// this audit walking it up to four times while another writer appends.
	// It changes no verdict: a settled attempt is the ordinary complete
	// audit, and a moved one is SNAPSHOT_MOVED either way. Native sources
	// only; ignored with PrefixFallback, whose complete attempt already stops.
	StopOnMove      bool
	afterCapture    func() // deterministic capture/body boundary witness
	afterPrefixWalk func() // deterministic prefix walk/recheck boundary witness
	divergentIntent string // set only on a value copy by Reconciliation
	unpauseTickets  bool   // set only on a value copy by BarrierRemoval or PendingBarrierRemoval
	pendingUnpause  bool   // set only on a value copy by PendingBarrierRemoval
	physical        *PhysicalObservation
	writerCache     bool
	intentOnly      bool
	handoffPolicy   *HandoffPolicySelector
	observedIntent  bool              // AuditForMutation: select queue, policy and observed tickets/releases
	writer          *WriterCheckpoint // AuditForWriter only
	writerBefore    *observation      // AuditForWriter's first capture, set for its second only
	liveness        bool              // AuditForLiveness: list no intent directory; read only queue.json and policy.json (JSG-V0-001)
	prefix          bool              // AuditWriterRefresh's prefix walk only
	through         uint64            // AuditThrough: walk receipts 1..through only (TCL-V0-820)
	fullAttempts    int               // AuditWriterRefresh: complete attempts before the prefix walk
	abortMoved      bool              // PrefixFallback's complete attempt: stop once head.json moves
	resume          *ResumeBase       // AuditForResume only
	walks           *WalkStats        // AuditForWriteObserved, AuditForResume: per-walk accounting, outlives the physical reset
	// orphanSlots are the descriptor-less staging slots, bound to the
	// metadata the refused observation listed, that a free writer lock
	// proved a killed writer's orphans (CTS-V0-026). Set only by audit on
	// its value copy after a spent CTS-V0-008 wait.
	orphanSlots map[string]os.FileInfo
}

// AuditForWrite carries one verified snapshot through request lookup and
// planning. It retains canonical mutable records with the existing selection
// budget, and only digests for historical requests. It writes nothing.
func (r Reader) AuditForWrite() (*Result, error) {
	r.writerCache = true
	return r.Audit()
}

// AuditForHandoff always completes one fresh full audit. It does not mutate a
// cached writer Result or put selector-dependent evidence into the read cache.
func (r Reader) AuditForHandoff(selector HandoffPolicySelector) (*Result, error) {
	q, err := snapshot.AttemptQueue(selector.AttemptID)
	if err != nil || q != r.QueueID {
		return nil, wire.Errorf(wire.CodeMalformed, "handoff selector", "attempt queue differs")
	}
	if n, err := wire.ParseSize("handoff generation", string(selector.Generation)); err != nil || n.Uint64() == 0 {
		return nil, wire.Errorf(wire.CodeMalformed, "handoff selector", "generation is missing or malformed")
	}
	if _, err := wire.ParseDigest("handoff policy", string(selector.OriginalPolicySha256)); err != nil {
		return nil, err
	}
	if (selector.PoolID == "") != (selector.MemberID == "") {
		return nil, wire.Errorf(wire.CodeMalformed, "handoff selector", "partial allocation")
	}
	r.writerCache, r.Checkpoint, r.handoffPolicy = true, nil, &selector
	return r.Audit()
}

type limits struct{ scan, selected int }

var profileLimits = limits{wire.MaxArchiveScanEntries, wire.MaxIntentTreeBytes}

type observation struct {
	headRaw []byte
	head    *snapshot.Head
	files   map[string]os.FileInfo
	// lite leaves receipts/, requests/ and evidence/ unlisted. Their directory
	// identities stay in the inventory, so a new receipt still moves it.
	lite bool
	// writer lists receipts/ by name only and leaves request shards unlisted
	// (AuditForWriter). Receipt names are still validated and digested into
	// the inventory, so an added, removed or renamed receipt moves it.
	writer        bool
	intentDigests map[string]wire.Digest
	receipts      []string
	identity      Identity
	staging       bool
	orphanSlots   []string // CTS-V0-026: granted orphan slots this capture set aside
	stage         *snapshot.StageObservation
	stageDigests  []stageDigest
	stageErr      error
	// intentRaw holds the intent bytes a writer capture read (AuditForWriter
	// only), so the walk and the second capture do not read them again.
	intentRaw map[string][]byte
	// at is when the capture began; ResumeBase trusts a file's stamps only
	// when they were already settled then (PSR-V0-052).
	at time.Time
	// seal is the retired prefix the store carries (JRT-V0-004).
	seal *RetiredPrefix
	// manifest is the segment manifest a taskman-journal-head/1 binds, read
	// and checked against the head; receipts 1..manifest.Through are sealed
	// (JSG-V0-006).
	manifest *snapshot.SegmentManifest
	// segErr is why segments/ and the listed receipts do not match the head.
	// It is returned by the walk, never by the capture, so a seal that moved
	// the store between the listing and the head read is a moved
	// observation, not a refusal (JSG-V0-007).
	segErr error
}

// receiptsQuiescence is how long a file must have been unmodified and
// unchanged before a resume base trusts its stamps (PSR-V0-052): at least
// the coarsest stamp granularity of a filesystem that can hold the journal
// (one second), so a change after the capture lands in a strictly later
// change time even where stamps are whole seconds.
const receiptsQuiescence = time.Second

// Audit checks all current projections and returns selected canonical bytes.
// Pending and uninitialized observations return a partial Result plus their
// closed error code. No result on any error is a usable request index.
func (r Reader) Audit(paths ...string) (*Result, error) {
	return r.audit(paths, "", profileLimits, true)
}

// BarrierRemoval preserves unrelated ticket edits while auditing a settled
// journal for UNPAUSE. Every canonical ticket must still have a regular physical
// projection. Queue, policy, private state and untracked tickets remain strict.
func (r Reader) BarrierRemoval(paths ...string) (*Result, error) {
	r.unpauseTickets = true
	return r.Audit(paths...)
}

// PendingBarrierRemoval is BarrierRemoval for redo of an interrupted UNPAUSE
// (V1-0309): it also admits the one pending receipt, so recovery tolerates the
// ticket divergence the fresh UNPAUSE allowed. The caller must establish that
// the pending receipt is an UNPAUSE that posts no intent path; a pending
// observation still returns REDO_PENDING, with ProjectionAgreement
// TICKETS_NOT_COMPARED (every other projection holds its pre or post value).
func (r Reader) PendingBarrierRemoval(paths ...string) (*Result, error) {
	r.unpauseTickets, r.pendingUnpause = true, true
	return r.Audit(paths...)
}

// Reconciliation returns canonical ticket bytes after a complete audit that
// permits only this target's physical projection to diverge. Every other intent
// and private projection remains strict; errors never supply write authority.
func (r Reader) Reconciliation(targetID string, paths ...string) (*Result, error) {
	target, err := wire.ParseTicketID("targetId", targetID)
	if err != nil {
		return nil, err
	}
	if target.QueueID() != r.QueueID.Raw {
		return nil, wire.Errorf(wire.CodeOutOfScope, "targetId", "target queue differs")
	}
	r.divergentIntent = "intent/tickets/" + target.Local + ".json"
	selected := append([]string{r.divergentIntent}, paths...)
	result, err := r.Audit(selected...)
	if err != nil {
		return result, err
	}
	record, ok := result.Records[r.divergentIntent]
	if !ok || record.Sha256 == nil {
		return result, wire.Errorf(wire.CodeMalformed, "targetId", "canonical target does not exist")
	}
	return result, nil
}

func (r Reader) ReconciliationRelease(releaseID string, paths ...string) (*Result, error) {
	if _, err := wire.ParseLabel("releaseId", releaseID); err != nil {
		return nil, err
	}
	r.divergentIntent = "intent/releases/" + releaseID + ".json"
	selected := append([]string{r.divergentIntent}, paths...)
	result, err := r.Audit(selected...)
	if err != nil {
		return result, err
	}
	record, ok := result.Records[r.divergentIntent]
	if !ok || record.Sha256 == nil {
		return result, wire.Errorf(wire.CodeMalformed, "releaseId", "canonical release does not exist")
	}
	return result, nil
}

func (r Reader) audit(paths []string, request string, lim limits, checkIntent bool) (*Result, error) {
	if r.Source == nil {
		return nil, wire.Errorf(wire.CodeMalformed, "/", "missing byte source")
	}
	q, err := wire.ParseQueueID("/queueId", r.QueueID.Raw)
	if err != nil {
		return nil, err
	}
	if q != r.QueueID {
		return nil, wire.Errorf(wire.CodeMalformed, "/queueId", "inconsistent parsed identity")
	}
	if _, err := wire.ParsePathText("/primaryWorktree", r.PrimaryWorktree); err != nil {
		return nil, err
	}
	selected := make(map[string]bool)
	for _, p := range paths {
		if _, err := snapshot.PostBound(p); err != nil {
			return nil, err
		}
		if len(selected) >= lim.scan+intent.MaxIntentRootEntries+wire.MaxTicketsPerQueue+wire.MaxReleasesPerQueue {
			return nil, wire.Errorf(wire.CodeLimitExceeded, p, "selected path metadata bound")
		}
		selected[p] = true
	}
	if r.Checkpoint != nil && request == "" && checkIntent && !r.writerCache && !r.intentOnly && !r.unpauseTickets && r.divergentIntent == "" {
		for attempt := 0; attempt < 4; attempt++ {
			result, err := r.auditAttempt(selected, request, lim, checkIntent, r.Checkpoint)
			if err == nil || err == errHeadPassed {
				return result, err
			}
			var pending inFlight
			if wire.CodeOf(err) != wire.CodeSnapshotMoved && !errors.As(err, &pending) {
				break
			}
		}
	}
	var wait snapshot.StageSlotWait
	var slotsHead []byte
	waited := false
	attempts := 4
	if r.fullAttempts > 0 {
		attempts = r.fullAttempts
	}
	for attempt := 0; attempt < attempts; {
		result, err := r.auditAttempt(selected, request, lim, checkIntent, nil)
		if err == errHeadPassed {
			return nil, err
		}
		if wire.CodeOf(err) == wire.CodeSnapshotMoved {
			attempt++
			continue
		}
		var slots slotsInFlight
		if errors.As(err, &slots) {
			if !waited {
				waited, slotsHead = true, slots.head
			}
			if r.orphanSlots != nil {
				// Descriptor-less slots other than the granted orphans
				// appeared after the writer lock proved free: a writer is
				// live again (CTS-V0-026).
				return nil, wire.Errorf(wire.CodeSnapshotMoved, "staging", "a writer staged new slots after the writer lock proved free; the read is retryable")
			}
			step, ok := wait.Next()
			if r.WriterLocked || !ok {
				if !r.WriterLocked && !bytes.Equal(slotsHead, slots.head) {
					// Writers committed while the slots stayed. That warrants
					// another bounded observation, not proof that the holders
					// are live: a retry under an unchanged head still refuses
					// MALFORMED (CAL-V0-340).
					return nil, wire.Errorf(wire.CodeSnapshotMoved, "staging", "ledger advanced during the %s wait for a writer's staging slots; the read is retryable", snapshot.StageSlotPatience)
				}
				if !r.WriterLocked && r.writerLockFree() {
					// Every writer holds the writer lock while it owns a
					// slot, so a free lock after the spent wait proves these
					// slots a killed writer's orphans: observe again with
					// exactly them set aside (CTS-V0-026).
					r.orphanSlots = slots.slots
					continue
				}
				return result, slots.error
			}
			stageSleep(step)
			continue
		}
		return result, err
	}
	return nil, wire.Errorf(wire.CodeSnapshotMoved, "/", "ledger moved during all four observations")
}

// writerLockFree is the CTS-V0-026 proof for a native source: the writer
// lock `<common>/taskman.lock` beside the `<common>/taskman` state directory
// is free this instant. Any other source, or a state directory not so named,
// proves nothing.
func (r Reader) writerLockFree() bool {
	var stateDir string
	switch n := r.Source.(type) {
	case Native:
		stateDir = n.StateDir
	case *Native:
		if n != nil {
			stateDir = n.StateDir
		}
	}
	stateDir = filepath.Clean(stateDir)
	if !filepath.IsAbs(stateDir) || filepath.Base(stateDir) != "taskman" {
		return false
	}
	return probeWriterLock(filepath.Dir(stateDir))
}

// probeWriterLock is writerLockFree; tests replace it.
var probeWriterLock = writerLockFree

// stageSleep is the CTS-V0-008 wait's pause (snapshot.StageSlotWait). A
// WriterLocked audit never waits. A pause is not one of the four
// SNAPSHOT_MOVED attempts, and an observation that moves between pauses
// still spends one.
var stageSleep = time.Sleep

func (r Reader) auditAttempt(selected map[string]bool, request string, lim limits, checkIntent bool, cp *Checkpoint) (result *Result, err error) {
	if r.physical != nil {
		*r.physical = PhysicalObservation{}
	}
	if r.walks != nil {
		started := time.Now()
		defer func() {
			r.walks.record(r.prefix, r.resume != nil, time.Since(started), wire.CodeOf(err) == wire.CodeSnapshotMoved)
		}()
	}
	var native *nativeRead
	var captured *observation
	switch n := r.Source.(type) {
	case Native:
		native = newNativeRead(n)
	case *Native:
		if n == nil {
			return nil, wire.Errorf(wire.CodeMalformed, "source", "nil native source")
		}
		native = newNativeRead(*n)
	}
	if native != nil {
		r.Source = native
		if r.physical != nil {
			native.physical = newPhysicalReads()
		}
		defer func() {
			cleanup := native.close()
			if cleanup != nil {
				result = nil
				err = wire.Errorf(wire.CodeUnsupportedFilesystem, "read lifetime", "%v; close: %v", err, cleanup)
			}
			if r.physical != nil {
				r.physical.Cleanup = cleanup
				if err == nil && result != nil && (result.Mode == ModeFull || result.Mode == ModeWriter || result.Mode == ModePrefix) && !result.Pending && !result.StagingPresent && result.IntentError == nil && result.selectErr == nil && result.intentErr == nil {
					r.physical.Files = native.physical.files
					r.physical.capture = captured
				}
			}
		}()
	}
	if r.writer != nil && native == nil {
		return nil, errCheckpoint("source", "writer checkpoint needs a native source")
	}
	before, e := r.capture(lim, cp != nil)
	if e != nil {
		return nil, e
	}
	captured = before
	if before.manifest != nil {
		r.Source = newSegmentView(r.Source, before.manifest)
	}
	if r.afterCapture != nil {
		r.afterCapture()
	}
	if r.ExpectHeadSha256 != "" && before.identity.HeadSha256 != r.ExpectHeadSha256 {
		return nil, errHeadPassed
	}
	if r.observedIntent {
		selected = observedSelection(before)
	}
	var bodyErr error
	if r.writer != nil {
		result, bodyErr = r.walkWriter(before, r.writer, selected, request, lim)
	} else if r.prefix {
		result, bodyErr = r.walkPrefix(before, lim)
	} else if cp != nil {
		result, bodyErr = r.walkTail(before, cp, selected, lim)
	} else {
		result, bodyErr = r.walk(before, selected, request, lim, checkIntent)
	}
	if result != nil && r.observedIntent {
		result.selection = selected
	}
	if result != nil {
		result.manifest = before.manifest
	}
	if r.resume != nil && bodyErr == nil {
		// The walk resumed from a base this command audited from receipt 1;
		// the base files must still be exactly what it read (PSR-V0-052).
		// Its own read lifetime closes into this attempt's cleanup, so a
		// failed close terminates as the audit's would (PSR-V0-057).
		check := newNativeRead(native.native)
		bodyErr = r.resume.held(check, result, lim, true)
		native.closeErr = errors.Join(native.closeErr, check.close())
		if native.physical != nil {
			result.resumeRead = native.physical.files
		}
	}
	if r.writer != nil {
		r.writerBefore = before
	}
	if r.prefix && bodyErr == nil {
		// A prefix walk that succeeded stands while receipts 1..H do; a
		// refusal needs the whole observation unchanged, below (CAL-V0-265).
		if r.afterPrefixWalk != nil {
			r.afterPrefixWalk()
		}
		held, e := prefixHeld(native.native, before, result, lim)
		if e != nil {
			return nil, e
		}
		if !held {
			return nil, moved("receipts")
		}
		return result, nil
	}
	after, e := r.capture(lim, cp != nil)
	if e != nil {
		return nil, e
	}
	if !sameObservation(before, after) {
		return nil, moved("inventory")
	}
	return result, bodyErr
}

// errHeadPassed is the SNAPSHOT_MOVED an attempt returns when the head no
// longer names Reader.ExpectHeadSha256.
var errHeadPassed = wire.Errorf(wire.CodeSnapshotMoved, "head.json", "head moved past the caller's snapshot")

func sameObservation(a, b *observation) bool {
	if a.identity != b.identity || len(a.files) != len(b.files) {
		return false
	}
	for p, info := range a.files {
		other, ok := b.files[p]
		if !ok || !os.SameFile(info, other) {
			return false
		}
	}
	return true
}

func optionalRead(s Source, p string, max int) ([]byte, bool, error) {
	raw, err := s.Read(p, max)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if len(raw) > max {
		return nil, false, wire.Errorf(wire.CodeLimitExceeded, p, "byte source exceeded read bound")
	}
	return raw, true, nil
}

func requiredRead(s Source, p string, max int) ([]byte, error) {
	raw, present, err := optionalRead(s, p, max)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, wire.Errorf(wire.CodeJournalForked, p, "required retained bytes absent")
	}
	if raw == nil {
		raw = []byte{}
	}
	return raw, nil
}

func (r Reader) capture(lim limits, lite bool) (*observation, error) {
	o := &observation{files: map[string]os.FileInfo{}, lite: lite, writer: r.writer != nil, intentDigests: map[string]wire.Digest{}, at: time.Now()}
	if o.writer {
		o.intentRaw = map[string][]byte{}
	}
	remaining := lim.scan
	if err := r.scan(o, ".", &remaining, true); err != nil {
		return nil, err
	}
	sort.Strings(o.receipts)
	intentRemaining := intent.MaxIntentRootEntries + wire.MaxTicketsPerQueue + wire.MaxReleasesPerQueue
	// A liveness write lists no intent directory and reads no ticket or
	// release (JSG-V0-001): only the queue and policy its model decodes,
	// by name, below.
	if !r.liveness {
		if err := r.scan(o, "intent", &intentRemaining, true); err != nil {
			return nil, err
		}
	}
	if _, ok := o.files["RESTORE_INCOMPLETE"]; ok {
		return nil, wire.Errorf(wire.CodeRestoreIncomplete, "RESTORE_INCOMPLETE", "interrupted restore")
	}
	if _, ok := o.files[RetireIncompleteFile]; ok {
		// Neither the old nor the new store is settled until the retire
		// that left the marker is rerun (JRT-V0-009).
		return nil, wire.Errorf(wire.CodeRestoreIncomplete, RetireIncompleteFile, "interrupted retire; rerun receipt retire or unretire")
	}
	if _, ok := o.files[RetiredPrefixFile]; ok {
		raw, err := requiredRead(r.Source, RetiredPrefixFile, MaxRetiredPrefixBytes)
		if err != nil {
			return nil, err
		}
		if o.seal, err = DecodeRetiredPrefix(raw); err != nil {
			return nil, err
		}
	}
	raw, present, err := optionalRead(r.Source, "head.json", wire.MaxJournalHeadBytes)
	if err != nil {
		return nil, err
	}
	o.headRaw = raw
	if present {
		o.head, err = snapshot.DecodeHead(raw)
		if err != nil {
			return nil, err
		}
	}
	var paths []string
	for p := range o.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	inventory := sha256.New()
	if o.writer {
		// The same bytes Fprintf("receipt-name\x00%s\n") wrote, without a
		// format pass per receipt (V1-1410).
		prefix, nl := []byte("receipt-name\x00"), []byte{'\n'}
		for _, name := range o.receipts {
			inventory.Write(prefix)
			io.WriteString(inventory, name)
			inventory.Write(nl)
		}
	}
	var intents []intent.File
	total := 0
	for _, p := range paths {
		info := o.files[p]
		fmt.Fprintf(inventory, "%s\x00%d\x00%d\x00%d\n", p, info.Mode(), info.Size(), info.ModTime().UnixNano())
		if strings.HasPrefix(p, "receipts/") && !info.IsDir() && !isTemp(p) {
			o.receipts = append(o.receipts, strings.TrimPrefix(p, "receipts/"))
		}
		if strings.HasPrefix(p, "intent/") && !info.IsDir() {
			max, err := snapshot.PostBound(p)
			if err != nil {
				return nil, err
			}
			if info.Size() < 1 && !strings.HasPrefix(p, "intent/tickets/") {
				return nil, wire.Errorf(wire.CodeMalformed, p, "empty intent file")
			}
			if info.Size() > int64(max) || info.Size() > int64(wire.MaxIntentTreeBytes-total) {
				return nil, wire.Errorf(wire.CodeLimitExceeded, p, "intent byte bound")
			}
			raw, sum, shared := r.sharedIntent(p, info.Size(), !o.writer)
			indexed := shared && raw == nil // a closed ticket the probe took from its index (TCX-V0-985)
			if !shared {
				raw, sum, shared = r.writerIntent(p, info)
			}
			if !shared {
				raw, err = requiredRead(r.Source, p, max)
				if err != nil {
					return nil, err
				}
				sum = wire.Sum(raw)
			}
			n := len(raw)
			if indexed {
				n = int(info.Size())
			}
			if n == 0 && !strings.HasPrefix(p, "intent/tickets/") {
				return nil, wire.Errorf(wire.CodeMalformed, p, "empty intent file")
			}
			if n > wire.MaxIntentTreeBytes-total {
				return nil, wire.Errorf(wire.CodeLimitExceeded, p, "intent tree grew beyond bound")
			}
			total += n
			o.intentDigests[p] = sum
			if o.writer {
				o.intentRaw[p] = raw
			}
			intents = append(intents, intent.File{Path: strings.TrimPrefix(p, "intent/"), Sha256: sum, Bytes: n})
		}
	}
	if r.liveness {
		for _, p := range []string{"intent/queue.json", "intent/policy.json"} {
			max, err := snapshot.PostBound(p)
			if err != nil {
				return nil, err
			}
			raw, err := requiredRead(r.Source, p, max)
			if err != nil {
				return nil, err
			}
			if len(raw) == 0 {
				return nil, wire.Errorf(wire.CodeMalformed, p, "empty intent file")
			}
			sum := wire.Sum(raw)
			o.intentDigests[p], o.intentRaw[p] = sum, raw
			fmt.Fprintf(inventory, "intent-digest\x00%s\x00%s\n", p, sum)
		}
	}
	if o.segErr, err = r.segments(o); err != nil {
		return nil, err
	}
	stageFiles, stageErr, err := r.readStage(o)
	o.stageErr = stageErr
	if err != nil {
		return nil, err
	}
	if o.stageErr == nil {
		o.stage, o.stageErr = snapshot.ObserveStage(stageFiles)
	}
	for _, f := range stageFiles {
		sum := wire.Sum(f.Raw)
		o.stageDigests = append(o.stageDigests, stageDigest{f.Name, sum})
		fmt.Fprintf(inventory, "staging/%s\x00%s\n", f.Name, sum)
	}
	o.identity = Identity{HeadSha256: wire.Sum(raw), InventorySha256: wire.Digest(hex.EncodeToString(inventory.Sum(nil))), IntentTreeSha256: intent.DigestOfFiles(intents)}
	if r.liveness {
		// The intent tree was not observed, so no digest of it is reported
		// (JSG-V0-002).
		o.identity.IntentTreeSha256 = ""
	}
	return o, nil
}

// sharedIntent returns the bytes and digest the outer snapshot hashed for
// intent path p, when it holds that file at the observed size. When digest
// alone is enough (no bytes are kept), a file the snapshot took from the
// closed-ticket index shares its digest with nil bytes (TCX-V0-985).
func (r Reader) sharedIntent(p string, size int64, digestOnly bool) ([]byte, wire.Digest, bool) {
	if r.IntentTree == nil {
		return nil, "", false
	}
	files := r.IntentTree.Files
	rel := strings.TrimPrefix(p, "intent/")
	i := sort.Search(len(files), func(i int) bool { return files[i].Path >= rel })
	if i == len(files) || files[i].Path != rel {
		return nil, "", false
	}
	if files[i].Raw == nil {
		if digestOnly && int64(files[i].Bytes) == size {
			return nil, files[i].Sha256, true
		}
		return nil, "", false
	}
	if int64(len(files[i].Raw)) != size {
		return nil, "", false
	}
	return files[i].Raw, files[i].Sha256, true
}

// writerIntent returns the bytes AuditForWriter's first capture read for
// intent path p while p is still the same file with the same mode, size and
// modification time (CAL-V0-189, proposed). This capture only confirms that
// nothing moved during the walk; the writer route rechecks the intent tree's
// content before any effect, so these stat stamps are never the evidence
// that the modeled intent bytes are current.
func (r Reader) writerIntent(p string, info os.FileInfo) ([]byte, wire.Digest, bool) {
	if r.writerBefore == nil {
		return nil, "", false
	}
	raw, ok := r.writerBefore.intentRaw[p]
	old := r.writerBefore.files[p]
	if !ok || old == nil || int64(len(raw)) != info.Size() || !os.SameFile(old, info) || old.Mode() != info.Mode() || old.Size() != info.Size() || !old.ModTime().Equal(info.ModTime()) {
		return nil, "", false
	}
	return raw, r.writerBefore.intentDigests[p], true
}

func isTemp(p string) bool {
	return p == "head.json.tmp" || ((strings.HasPrefix(p, "receipts/") || strings.HasPrefix(p, "effects/")) && strings.Contains(p, ".tmp-"))
}

func (r Reader) scan(o *observation, dir string, remaining *int, optional bool) error {
	max := *remaining
	if dir == "intent" && max > intent.MaxIntentRootEntries {
		max = intent.MaxIntentRootEntries
	}
	if dir == "intent/tickets" && max > wire.MaxTicketsPerQueue {
		max = wire.MaxTicketsPerQueue
	}
	if dir == "intent/releases" && max > wire.MaxReleasesPerQueue {
		max = wire.MaxReleasesPerQueue
	}
	if dir == "staging" && max > snapshot.MaxStageChildren {
		max = snapshot.MaxStageChildren
	}
	listing, err := r.Source.List(dir, max)
	if optional && os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if listing.DirectoryInfo == nil || !listing.DirectoryInfo.IsDir() {
		return wire.Errorf(wire.CodeMalformed, dir, "missing listed directory metadata")
	}
	o.files[dir] = listing.DirectoryInfo
	entries := listing.Entries
	if len(entries) > max {
		return wire.Errorf(wire.CodeLimitExceeded, dir, "directory scan bound")
	}
	*remaining -= len(entries)
	seen := map[string]bool{}
	for _, e := range entries {
		if e.Name == "" || e.Name == "." || e.Name == ".." || strings.ContainsAny(e.Name, "/\\") || seen[e.Name] {
			return wire.Errorf(wire.CodeJournalForked, dir, "duplicate or aliased inventory name")
		}
		seen[e.Name] = true
		p := e.Name
		if dir != "." {
			p = dir + "/" + e.Name
		}
		if e.Info == nil {
			return wire.Errorf(wire.CodeMalformed, p, "missing directory metadata")
		}
		if !e.Info.IsDir() && !e.Info.Mode().IsRegular() {
			return wire.Errorf(wire.CodeUnsupportedFilesystem, p, "only regular files and directories allowed")
		}
		if p == evidencestore.Dir && e.Info.IsDir() {
			// The witness receipt store (TOL-V0-137) sits beside the
			// journal: no receipt posts its files, so it is neither
			// scanned nor part of the inventory fingerprint, and a put
			// never moves a reader's or writer's observation.
			continue
		}
		o.files[p] = e.Info
		if e.Info.IsDir() {
			if !allowedDir(p) {
				return wire.Errorf(wire.CodeMalformed, p, "unexpected store directory")
			}
			if p == "worktrees" || (o.lite && (p == "receipts" || p == "requests" || p == "evidence")) || (o.writer && strings.HasPrefix(p, "requests/")) {
				continue
			}
			if o.writer && p == "receipts" {
				if err := r.scanReceiptNames(o, remaining); err != nil {
					return err
				}
				continue
			}
			if err := r.scan(o, p, remaining, false); err != nil {
				return err
			}
			continue
		}
		if dir == "staging" {
			if !snapshot.StageName(e.Name) {
				return wire.Errorf(wire.CodeMalformed, p, "unexpected staging child")
			}
			continue
		}
		if isTemp(p) {
			o.staging = true
			continue
		}
		if strings.HasPrefix(p, "receipts/") {
			if _, err := receiptSeq(strings.TrimPrefix(p, "receipts/")); err != nil {
				return err
			}
			continue
		}
		if p == "head.json" || p == "RESTORE_INCOMPLETE" || p == RetiredPrefixFile || p == RetireIncompleteFile {
			continue
		}
		if dir == snapshot.SegmentsDir {
			if _, _, ok := snapshot.ParseSegmentName(e.Name); !ok && e.Name != snapshot.SealMarkerName {
				if _, ok := snapshot.ParseSegmentManifestName(e.Name); !ok {
					return wire.Errorf(wire.CodeJournalForked, p, "unexpected segments file")
				}
			}
			continue
		}
		if strings.HasPrefix(p, "effects/") && (strings.HasSuffix(p, ".boot") || strings.HasSuffix(p, ".ack")) {
			name := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(p, "effects/"), ".boot"), ".ack")
			if _, err := wire.ParseDigest(p, name); err != nil {
				return err
			}
			continue
		}
		if _, err := snapshot.PostBound(p); err != nil {
			return err
		}
	}
	return nil
}

// scanReceiptNames lists receipts/ without a per-entry lstat (AuditForWriter).
// Every name is validated as scan validates it; a temporary name marks the
// observation unsettled. Receipt files are opened only when walked. Both
// captures list exactly (JSG-V0-007): the CAL-V0-224 probe and listing
// reuse are retired, since a segmented store keeps at most 1,023 names here.
func (r Reader) scanReceiptNames(o *observation, remaining *int) error {
	native, ok := nativeOf(r.Source)
	if !ok {
		return errCheckpoint("receipts", "writer checkpoint needs a native source")
	}
	info, names, err := native.Names("receipts", *remaining)
	if err != nil {
		return err
	}
	if info == nil || !info.IsDir() {
		return wire.Errorf(wire.CodeMalformed, "receipts", "missing listed directory metadata")
	}
	if len(names) > *remaining {
		return wire.Errorf(wire.CodeLimitExceeded, "receipts", "directory scan bound")
	}
	*remaining -= len(names)
	o.files["receipts"] = info
	return receiptNames(o, names)
}

// receiptNames validates listed receipt names in listing order with scan's
// checks and first-error order, marks the observation unsettled on a
// temporary name and appends every settled name to o.receipts (V1-1410). A
// settled name is fixed-format, so its duplicates are tracked by sequence
// without hashing or formatting a string per name; temporary names keep a
// map, and any other name refuses at its own position before any later
// name is examined.
func receiptNames(o *observation, names []string) error {
	aliased := func() error {
		return wire.Errorf(wire.CodeJournalForked, "receipts", "duplicate or aliased inventory name")
	}
	dense := make([]bool, len(names)+1)
	var sparse map[uint64]struct{}
	var temps map[string]struct{}
	o.receipts = slices.Grow(o.receipts, len(names))
	for _, name := range names {
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
			return aliased()
		}
		if strings.Contains(name, ".tmp-") {
			if temps == nil {
				temps = map[string]struct{}{}
			}
			if _, dup := temps[name]; dup {
				return aliased()
			}
			temps[name] = struct{}{}
			o.staging = true
			continue
		}
		seq, ok := receiptNameSeq(name)
		if !ok {
			return wire.Errorf(wire.CodeJournalForked, name, "invalid receipt filename")
		}
		if seq < uint64(len(dense)) {
			if dense[seq] {
				return aliased()
			}
			dense[seq] = true
		} else {
			if sparse == nil {
				sparse = map[uint64]struct{}{}
			}
			if _, dup := sparse[seq]; dup {
				return aliased()
			}
			sparse[seq] = struct{}{}
		}
		o.receipts = append(o.receipts, name)
	}
	return nil
}

func allowedDir(p string) bool {
	switch p {
	case "staging", "receipts", "requests", "attempts", "effects", "pinned", "evidence", "worktrees", "intent/tickets", "intent/releases", snapshot.SegmentsDir, evidencestore.Dir:
		return true
	}
	if strings.HasPrefix(p, "requests/") && len(p) == len("requests/")+2 {
		_, err := strconv.ParseUint(strings.TrimPrefix(p, "requests/"), 16, 8)
		return err == nil && strings.ToLower(p) == p
	}
	return false
}

func receiptSeq(name string) (uint64, error) {
	n, ok := receiptNameSeq(name)
	if !ok {
		return 0, wire.Errorf(wire.CodeJournalForked, name, "invalid receipt filename")
	}
	return n, nil
}

// receiptNameSeq parses a canonical receipt name, twelve ASCII digits that
// are not all zero followed by ".json", without formatting or allocating:
// exactly the names snapshot.ReceiptName produces for 1..999999999999.
func receiptNameSeq(name string) (uint64, bool) {
	if len(name) != 17 || name[12:] != ".json" {
		return 0, false
	}
	var n uint64
	for i := 0; i < 12; i++ {
		c := name[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + uint64(c-'0')
	}
	return n, n != 0
}
