package authority

import (
	"errors"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Role names one destination class of the state dir or the intent store
// (§3.4, §3.1). It is the exported form of the reviewed internal role
// vocabulary; the mapping from a role and a name to a directory, an
// admitted filename and a byte bound is unchanged.
type Role uint8

// The publication roles a callable writer may target.
const (
	RoleDescriptor   = Role(fixtureDescriptor)
	RoleReceipt      = Role(fixtureReceipt)
	RoleEvidence     = Role(fixtureEvidence)
	RolePin          = Role(fixturePin)
	RoleRequest      = Role(fixtureRequest)
	RoleVersion      = Role(fixtureVersion)
	RoleHead         = Role(fixtureHead)
	RoleBarrier      = Role(fixtureBarrier)
	RolePools        = Role(fixturePools)
	RolePrograms     = Role(fixturePrograms)
	RoleReservations = Role(fixtureReservations)
	RoleQueue        = Role(fixtureQueue)
	RolePolicy       = Role(fixturePolicy)
	RoleImportMap    = Role(fixtureImportMap)
	RoleTicket       = Role(fixtureTicket)
	RoleRelease      = Role(fixtureRelease)
	RoleAttempt      = Role(fixtureAttempt)
)

// Target is one publication destination: a role plus the exact entry name
// that role admits ("000000000001.json" for a receipt, "head.json" for the
// head). Names outside the role's grammar are refused by location().
type Target struct {
	Role Role
	Name string
}

// Slot is a staging slot under `staging/` (§5.6): "a00".."a10", or
// "active.json.tmp" for the descriptor.
type Slot string

// Stage is one prepared, fully synced staging file that has not yet been
// published. It is owned by the Session that prepared it.
type Stage struct{ inner *fixtureStage }

// Sha256 is the digest of the staged bytes as the session observed them
// after writing and syncing, not as the caller supplied them.
func (s *Stage) Sha256() wire.Digest { return s.inner.digest }

// Session is a callable durable writer bound to one held lock and one
// resolved repository authority (decision 0003). It performs the §5.2
// effects and nothing else: it authenticates no one, mints no binding, and
// decides no policy. The caller supplies an already-validated plan and an
// already-held lock; the session refuses anything outside the state dir and
// intent store it pinned when it opened.
//
// Every method is serialized on the session and spans its own checks,
// native call, sync and cleanup. Close releases the pinned descriptors; it
// does not release the lock, which the caller owns.
type Session struct{ inner *fixtureSession }

// NewSession pins the repository authority's directories under a held lock.
// The lock must be held for the whole life of the session.
func NewSession(repo *intent.Repository, lock *Lock) (*Session, error) {
	inner, err := newFixtureSession(repo, lock)
	if err != nil {
		return nil, err
	}
	return &Session{inner: inner}, nil
}

// Close releases every descriptor the session pinned and reports the first
// failure joined with any later one. It is safe to call more than once.
func (s *Session) Close() error { return s.inner.close() }

// Mkdir creates one admitted child directory of the state dir or intent
// store by key ("state", "receipts", "evidence", "pinned", "requests",
// "staging", "intent", "tickets", or "requests/<xx>") and syncs both the
// new directory and its parent. The Git common dir is never created.
func (s *Session) Mkdir(key string) error { return s.inner.mkdir(key) }

// Prepare writes data into the staging slot for role, fully syncs it, and
// returns the staged file. Bounds are the role's own §1 limits; an empty
// body is refused for every role except evidence.
func (s *Session) Prepare(slot Slot, role Role, data []byte) (*Stage, error) {
	inner, err := s.inner.prepare(fixtureSlot(slot), fixtureRole(role), data)
	if err != nil {
		return nil, err
	}
	return &Stage{inner: inner}, nil
}

// Preparation is one staging slot, role and body for PrepareAll.
type Preparation struct {
	Slot Slot
	Role Role
	Data []byte
}

// PrepareAll is Prepare for several slots in one operation (LGL-V0-013
// proposed): every file is written and confirmed in order, then all of them
// are fully synced together before any is returned, so a filesystem that
// groups commits pays about one sync latency for the batch. Each returned
// stage is exactly as durable as one Prepare returns. On any failure every
// file the call created is cleaned up and no stage is returned.
func (s *Session) PrepareAll(items []Preparation) ([]*Stage, error) {
	inner := make([]fixturePrepare, len(items))
	for i, it := range items {
		inner[i] = fixturePrepare{slot: fixtureSlot(it.Slot), role: fixtureRole(it.Role), data: it.Data}
	}
	staged, err := s.inner.prepareAll(inner)
	if err != nil {
		return nil, err
	}
	stages := make([]*Stage, len(staged))
	for i, st := range staged {
		stages[i] = &Stage{inner: st}
	}
	return stages, nil
}

// Link publishes a stage to an absent destination with link(2): the
// destination is either absent or complete and is never replaced. An
// existing destination is refused with nothing changed. This is the §5.2
// commit point when the target is a receipt.
func (s *Session) Link(stage *Stage, t Target) error {
	if stage == nil {
		return fixtureRefused
	}
	err := s.inner.link(stage.inner, fixtureTarget{role: fixtureRole(t.Role), name: t.Name})
	var effect *fixtureEffectError
	if t.Role == RoleReceipt && errors.As(err, &effect) {
		// Link committed, but its completion failed before the caller could
		// record the receipt. Conservatively forbid retry (CAL-V0-215).
		return wire.WithoutRetry(err)
	}
	return err
}

// Replace publishes a stage over a mutable destination (head, barrier or
// reservations only) after checking that the destination currently holds
// expected. A nil expected requires the destination to be absent and
// degenerates to Link. No other role may be replaced.
func (s *Session) Replace(stage *Stage, t Target, expected *wire.Digest) error {
	if stage == nil {
		return fixtureRefused
	}
	return s.inner.replace(stage.inner, fixtureTarget{role: fixtureRole(t.Role), name: t.Name}, expected)
}

// RemoveStage clears one staging slot after its contents have been
// published (§5.6). A slot left occupied with no live descriptor is an
// unassigned slot, which every later reader refuses, so a writer removes
// each slot it used.
func (s *Session) RemoveStage(slot Slot) error { return s.inner.removeStage(fixtureSlot(slot)) }

// RemoveOrphanStages clears the staging slots "a00".."a10" that a writer
// killed between its first Prepare and its own cleanup left behind
// (CAL-V0-019). It removes nothing while either descriptor ("active.json" or
// "active.json.tmp") exists, leaving that staging for the journal audit, and
// refuses while this session owns a stage: the native writer
// never writes a descriptor, so under the held lock every such slot is an
// orphan. An unpublished slot never reached a destination, a replaced one was
// consumed by its rename, and a linked one leaves its destination as a second
// link, so removing the slot changes no published state. It reports how many
// slots it removed.
func (s *Session) RemoveOrphanStages() (int, error) { return s.inner.removeOrphanStages() }

// RemoveBarrier unlinks only barrier.json after checking its exact pre digest,
// then syncs the retained parent directory. The caller must first commit the
// UNPAUSE receipt. An already absent barrier is synced as an idempotent removal.
func (s *Session) RemoveBarrier(expected wire.Digest) error {
	return s.inner.removeBarrier(fixtureTarget{fixtureBarrier, "barrier.json"}, expected)
}

// SkipStagingSync makes every later staging-directory sync of this session
// keep its boundary and checks but make no sync call (LGL-V0-005 proposed).
// Only a heartbeat or renew session sets it: every file sync and every
// destination directory sync still runs, so what the session commits is as
// durable as before, and a crash can leave only the staging residue a killed
// writer already leaves, which the next writer or audit sets aside.
func (s *Session) SkipStagingSync() {
	s.inner.mu.Lock()
	defer s.inner.mu.Unlock()
	s.inner.stagingSyncSkipped = true
}

// DeferPublishSyncs opens a window in which each Link or Replace still
// crosses every boundary and check in order but queues the directory syncs it
// makes after publishing, until FlushPublishSyncs issues them together
// (LGL-V0-016 proposed). A caller opens it only over publications that are
// independent of each other, and flushes before any step whose durability
// relies on them; Mkdir, Prepare, PrepareAll and RemoveBarrier sync at once.
func (s *Session) DeferPublishSyncs() {
	s.inner.mu.Lock()
	defer s.inner.mu.Unlock()
	s.inner.publishSyncsDeferred = true
}

// FlushPublishSyncs closes the window DeferPublishSyncs opened and issues
// every queued directory sync concurrently, returning only when all have
// returned; any failure is returned. With nothing queued it syncs nothing.
func (s *Session) FlushPublishSyncs() error { return s.inner.flushPublishSyncs() }

// SyncDuration is the wall time this session has spent in file and directory
// sync calls so far. It is an issue 494 diagnostic, never proof or authority.
func (s *Session) SyncDuration() time.Duration {
	s.inner.mu.Lock()
	defer s.inner.mu.Unlock()
	return s.inner.syncTime
}

// SyncCalls is how many file and directory sync calls this session has made
// so far (LGL-V0-006 proposed). Like SyncDuration it is a diagnostic, never
// proof or authority.
func (s *Session) SyncCalls() int {
	s.inner.mu.Lock()
	defer s.inner.mu.Unlock()
	return s.inner.syncCalls
}
