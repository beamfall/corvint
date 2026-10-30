package authority

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Experimental J3-01: only authority-package tests construct this mechanism.
// The harness owns a disposable repository, is its sole writer, and keeps mount
// topology stable. Neither these paths nor a flock establish actor authority.
// No transaction, receipt sequencing, layout acceptance or content-CAS is here.
var fixtureRefused = errors.New("fixture filesystem identity, role or lifetime refused")
var fixtureUnsupported = errors.New("fixture filesystem platform or mount observation unsupported")

type fixtureEffectError struct {
	effect string
	err    error
}

func (e *fixtureEffectError) Error() string {
	return "fixture: " + e.effect + " performed; required completion failed: " + e.err.Error()
}
func (e *fixtureEffectError) Unwrap() error { return e.err }
func fixtureAfter(effect string, err error) error {
	if err == nil {
		return nil
	}
	return &fixtureEffectError{effect, err}
}

type fixtureRole uint8

const (
	fixtureDescriptor fixtureRole = iota + 1
	fixtureReceipt
	fixtureEvidence
	fixturePin
	fixtureRequest
	fixtureVersion
	fixtureHead
	fixtureBarrier
	fixtureReservations
	fixturePools
	fixturePrograms
	fixtureQueue
	fixturePolicy
	fixtureImportMap
	fixtureTicket
	fixtureRelease
	fixtureAttempt
)

type fixtureTarget struct {
	role fixtureRole
	name string
}
type fixtureSlot string

type fixtureMount struct {
	device            uint64
	filesystem, mount string
}
type fixtureParent struct {
	root         *os.Root
	file         *os.File
	info         os.FileInfo
	mount        fixtureMount
	parent, name string
}
type fixtureStage struct {
	owner  *fixtureSession
	slot   fixtureSlot
	role   fixtureRole
	info   os.FileInfo
	digest wire.Digest
	size   int64
}
type fixtureSession struct {
	lockInfo os.FileInfo
	// Lock order: mu, then lock.mu. Both span an entire operation including
	// checks, native calls, sync, cleanup and temporary-handle close. close
	// takes mu then calls Lock.Close (which takes lock.mu); never the reverse.
	mu       sync.Mutex
	lock     *Lock
	repo     intent.Repository
	parents  map[string]*fixtureParent
	stages   map[*fixtureStage]bool
	closed   bool
	closeErr error
	// worktree is the retained root holding `.taskman`: "primary", or
	// "worktree" when CTW-V0-002 selected a linked intent worktree.
	worktree string
	// Per-session deterministic syscall boundaries; tests do not swap globals.
	before    func(string) error
	write     func(*os.File, []byte) (int, error)
	closeFile func(*os.File) error
	closeRoot func(*os.Root) error
	observe   func(*os.File, fixtureMount) (fixtureMount, error)
	// fdinfo retains, per pinned descriptor, the open procfs fdinfo file the
	// Linux mount observation re-reads on every check (LGL-V0-002,
	// proposed); closeHandles closes it. Other platforms leave it empty.
	fdinfo map[*os.File]*os.File
	// syncTime sums the session's file and directory sync calls (issue 494
	// diagnostics only); guarded by mu like every other session field.
	syncTime time.Duration
	// syncCalls counts those calls (LGL-V0-006 proposed), like syncTime a
	// diagnostic only; guarded by mu.
	syncCalls int
	// stagingSyncSkipped makes syncStaging skip only its directory sync call
	// (LGL-V0-005 proposed); guarded by mu.
	stagingSyncSkipped bool
	// publishSyncsDeferred opens a window in which the directory syncs a link
	// or replace makes after publishing are queued in pendingSyncs instead of
	// called, and flushPublishSyncs issues them together (LGL-V0-016
	// proposed); guarded by mu.
	publishSyncsDeferred bool
	pendingSyncs         []*os.File
}

func fixtureDirectory(key string) (parent, name string, ok bool) {
	switch key {
	case "common":
		return "primary", ".git", true
	case "state":
		return "common", "taskman", true
	case "intent":
		return "primary", intent.Dir, true
	case "staging", "receipts", "evidence", "pinned", "requests", "attempts":
		return "state", key, true
	case "tickets":
		return "intent", "tickets", true
	case "releases":
		return "intent", "releases", true
	}
	if strings.HasPrefix(key, "requests/") && fixtureHex(strings.TrimPrefix(key, "requests/"), 2) {
		return "requests", strings.TrimPrefix(key, "requests/"), true
	}
	return "", "", false
}
func fixtureHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func (t fixtureTarget) location() (string, string, int, error) {
	name := t.name
	switch t.role {
	case fixtureDescriptor:
		if name == "active.json" {
			return "staging", name, 2658, nil
		}
	case fixtureReceipt:
		raw := strings.TrimSuffix(name, ".json")
		seq, err := strconv.ParseUint(raw, 10, 64)
		if err == nil && seq > 0 && name == fmt.Sprintf("%012d.json", seq) {
			return "receipts", name, wire.MaxReceiptFileBytes, nil
		}
	case fixtureEvidence:
		if fixtureHex(name, 64) {
			return "evidence", name, wire.MaxEvidenceBlobBytes, nil
		}
	case fixturePin:
		if strings.HasSuffix(name, ".json") && fixtureHex(strings.TrimSuffix(name, ".json"), 64) {
			return "pinned", name, wire.MaxPinnedBytes, nil
		}
	case fixtureRequest:
		if strings.HasSuffix(name, ".json") && fixtureHex(strings.TrimSuffix(name, ".json"), 64) {
			return "requests/" + name[:2], name, wire.MaxAttemptRecordBytes, nil
		}
	case fixtureVersion:
		if name == "VERSION" {
			return "state", name, len("taskman-state/0\n"), nil
		}
	case fixtureHead:
		if name == "head.json" {
			return "state", name, wire.MaxJournalHeadBytes, nil
		}
	case fixtureBarrier:
		if name == "barrier.json" {
			return "state", name, wire.MaxBarrierBytes, nil
		}
	case fixturePrograms:
		if name == "programs.json" {
			return "state", name, 1 << 20, nil
		}
	case fixturePools:
		if name == "pools.json" {
			return "state", name, wire.MaxPoolStateBytes, nil
		}
	case fixtureReservations:
		if name == "reservations.json" {
			return "state", name, wire.MaxReservationSetBytes, nil
		}
	case fixtureQueue:
		if name == "queue.json" {
			return "intent", name, wire.MaxQueueFileBytes, nil
		}
	case fixturePolicy:
		if name == "policy.json" {
			return "intent", name, wire.MaxPolicyFileBytes, nil
		}
	case fixtureImportMap:
		if name == "import-map.json" {
			return "intent", name, wire.MaxImportMapBytes, nil
		}
	case fixtureTicket:
		token := strings.TrimSuffix(name, ".json")
		_, err := wire.ParseToken("", token, 64)
		if err == nil && strings.HasSuffix(name, ".json") {
			return "tickets", name, wire.MaxTicketFileBytes, nil
		}
	case fixtureRelease:
		id := strings.TrimSuffix(name, ".json")
		_, err := wire.ParseLabel("", id)
		if err == nil && strings.HasSuffix(name, ".json") {
			return "releases", name, wire.MaxReleaseFileBytes, nil
		}
	case fixtureAttempt:
		id := strings.TrimSuffix(name, ".json")
		_, err := wire.ParseIdentifier("", id)
		if err == nil && strings.HasPrefix(id, "attempt:") && strings.HasSuffix(name, ".json") {
			return "attempts", name, wire.MaxAttemptRecordBytes, nil
		}
	}
	return "", "", 0, fixtureRefused
}
func fixtureStageLimit(slot fixtureSlot, role fixtureRole) (int, error) {
	if slot == "active.json.tmp" && role == fixtureDescriptor {
		return 2658, nil
	}
	if len(slot) != 3 || slot[0] != 'a' || slot < "a00" || slot > "a10" || slot[1] < '0' || slot[1] > '1' || slot[2] < '0' || slot[2] > '9' {
		return 0, fixtureRefused
	}
	limits := map[fixtureRole]int{
		fixtureReceipt: wire.MaxReceiptFileBytes, fixtureEvidence: wire.MaxEvidenceBlobBytes,
		fixturePin: wire.MaxPinnedBytes, fixtureRequest: wire.MaxAttemptRecordBytes,
		fixtureVersion: len("taskman-state/0\n"), fixtureHead: wire.MaxJournalHeadBytes,
		fixturePrograms: 1 << 20, fixturePools: wire.MaxPoolStateBytes, fixtureBarrier: wire.MaxBarrierBytes, fixtureReservations: wire.MaxReservationSetBytes,
		fixtureQueue: wire.MaxQueueFileBytes, fixturePolicy: wire.MaxPolicyFileBytes,
		fixtureImportMap: wire.MaxImportMapBytes, fixtureTicket: wire.MaxTicketFileBytes, fixtureRelease: wire.MaxReleaseFileBytes,
		fixtureAttempt: wire.MaxAttemptRecordBytes,
	}
	limit, ok := limits[role]
	if !ok {
		return 0, fixtureRefused
	}
	return limit, nil
}

func newFixtureSession(repo *intent.Repository, lock *Lock) (_ *fixtureSession, err error) {
	if !supportedPlatform {
		return nil, fixtureUnsupported
	}
	if repo == nil || lock == nil {
		return nil, fixtureRefused
	}
	lock.mu.Lock()
	defer lock.mu.Unlock()
	if lock.closed || lock.f == nil || lock.acquired.IsZero() || lock.path != repo.LockPath {
		return nil, fixtureRefused
	}
	again, err := intent.Resolve(repo.PrimaryWorktree)
	if err != nil {
		return nil, err
	}
	if again.CommonDir != repo.CommonDir || again.StateDir != repo.StateDir || again.LockPath != repo.LockPath || again.PrimaryWorktree != repo.PrimaryWorktree || again.IntentRoot() != repo.IntentRoot() || again.IntentHEAD() != repo.IntentHEAD() {
		return nil, fixtureRefused
	}
	s := &fixtureSession{lock: lock, repo: *again, parents: map[string]*fixtureParent{}, stages: map[*fixtureStage]bool{}, write: (*os.File).Write, closeFile: (*os.File).Close, closeRoot: (*os.Root).Close, worktree: "primary"}
	s.observe = s.observeMount
	defer func() {
		if err != nil {
			err = errors.Join(err, s.closeHandles())
		}
	}()
	s.lockInfo, err = lock.f.Stat()
	if err != nil {
		return nil, err
	}
	root, err := safeopen.Root(repo.PrimaryWorktree)
	if err != nil {
		return nil, err
	}
	if err = s.retain("primary", "", "", root); err != nil {
		return nil, err
	}
	if again.IntentLinked() {
		if err = s.retainWorktree(); err != nil {
			return nil, err
		}
	}
	for _, key := range []string{"common", "state", "staging", "receipts", "evidence", "pinned", "requests", "intent", "tickets", "releases", "attempts"} {
		if err = s.pinExisting(key); err != nil {
			return nil, err
		}
	}
	// Request shards are pinned on first use (pinShard, CAL-V0-234): a
	// session that publishes into one shard does not open and re-verify
	// all 256 on every operation.
	if err = s.check(); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *fixtureSession) retain(key, parent, name string, root *os.Root) (err error) {
	f, err := safeopen.InRoot(root, ".", os.O_RDONLY, 0, true)
	if err != nil {
		return errors.Join(err, s.closeRoot(root))
	}
	info, statErr := f.Stat()
	mount, mountErr := s.observe(f, fixtureMount{})
	if err = errors.Join(statErr, mountErr); err != nil {
		return errors.Join(err, s.closeFile(f), s.closeRoot(root))
	}
	if mount.mount == "" || mount.filesystem == "" {
		return errors.Join(fixtureRefused, s.closeFile(f), s.closeRoot(root))
	}
	if base := s.parents["primary"]; base != nil && base.mount != mount {
		return errors.Join(fixtureRefused, s.closeFile(f), s.closeRoot(root))
	}
	s.parents[key] = &fixtureParent{root, f, info, mount, parent, name}
	return nil
}

// retainWorktree pins a linked intent worktree as a second root. Intent
// publication links from staging under the common dir, so the root must be on
// the primary's mount (CTW-V0-008); retain refuses any other mount.
func (s *fixtureSession) retainWorktree() error {
	root, err := safeopen.Root(s.repo.IntentRoot())
	if err != nil {
		return err
	}
	if err = s.retain("worktree", "", "", root); err != nil {
		if errors.Is(err, fixtureRefused) {
			return wire.Errorf(wire.CodeUnsupportedFilesystem, s.repo.IntentRoot(), "the linked intent worktree must be a local directory on the primary worktree's mount (CTW-V0-008)")
		}
		return err
	}
	p := s.parents["worktree"]
	if err = s.sameWorktree(p.root, p.info); err != nil {
		delete(s.parents, "worktree")
		refusal := wire.Errorf(wire.CodeUnsupportedFilesystem, s.repo.IntentRoot(), "the pinned linked intent worktree is not the directory admission validated (CTW-V0-008): %v", err)
		if closeErr := errors.Join(s.closeFile(p.file), s.closeRoot(p.root)); closeErr != nil {
			return errors.Join(refusal, closeErr)
		}
		return refusal
	}
	s.worktree = "worktree"
	return nil
}

// sameWorktree checks that a pinned linked intent worktree is the directory
// admission validated: its path still names the pinned directory, and the
// `.git` file read through the pinned root points back to the admitted
// registration (CTW-V0-004). Admission compares paths; this closes the window
// in which the path is swapped before the root is opened.
func (s *fixtureSession) sameWorktree(root *os.Root, info os.FileInfo) error {
	now, err := os.Lstat(s.repo.IntentRoot())
	if err != nil {
		return err
	}
	if !os.SameFile(now, info) {
		return errors.New("the worktree path names a different directory than the pinned one")
	}
	f, err := safeopen.InRoot(root, ".git", os.O_RDONLY, 0, false)
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(f, 4*wire.KiB+1))
	if err = errors.Join(err, s.closeFile(f)); err != nil {
		return err
	}
	back := strings.TrimRight(string(raw), "\r\n")
	if len(raw) > 4*wire.KiB || !strings.HasPrefix(back, "gitdir: ") || strings.ContainsAny(back, "\r\n") {
		return errors.New("the pinned worktree .git file is not a single gitdir: line")
	}
	back = strings.TrimPrefix(back, "gitdir: ")
	if !filepath.IsAbs(back) {
		back = filepath.Join(s.repo.IntentRoot(), back)
	}
	got, err := os.Lstat(filepath.Clean(back))
	if err != nil {
		return err
	}
	admin, err := os.Lstat(s.repo.IntentGitDir)
	if err != nil {
		return err
	}
	if !os.SameFile(got, admin) {
		return errors.New("the pinned worktree .git file points to a different registration")
	}
	return nil
}

// parentKey names the retained parent of a directory key: `.taskman` lives in
// the linked intent worktree when one is retained, otherwise in the primary.
func (s *fixtureSession) parentKey(key, parent string) string {
	if key == "intent" {
		return s.worktree
	}
	return parent
}

func (s *fixtureSession) pinExisting(key string) error {
	parent, name, ok := fixtureDirectory(key)
	if !ok {
		return fixtureRefused
	}
	parent = s.parentKey(key, parent)
	p := s.parents[parent]
	if p == nil {
		return nil
	}
	_, err := p.root.Lstat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	root, err := safeopen.SubRoot(p.root, name)
	if err != nil {
		return err
	}
	return s.retain(key, parent, name, root)
}
func (s *fixtureSession) boundary(name string) error {
	if s.before == nil {
		return nil
	}
	return s.before(name)
}
func (s *fixtureSession) operation(fn func() error) error {
	if s == nil {
		return fixtureRefused
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.lock == nil || s.parents == nil {
		return fixtureRefused
	}
	s.lock.mu.Lock()
	defer s.lock.mu.Unlock()
	if err := s.check(); err != nil {
		return err
	}
	return fn()
}
func (s *fixtureSession) check() (err error) {
	if s.lock.closed || s.lock.f == nil || s.lock.path != s.repo.LockPath {
		return fixtureRefused
	}
	root, err := safeopen.Root(s.repo.PrimaryWorktree)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.closeRoot(root)) }()
	info, err := root.Stat(".")
	if err != nil || !os.SameFile(info, s.parents["primary"].info) {
		return fixtureRefused
	}
	if s.worktree == "worktree" {
		if err = s.checkWorktree(); err != nil {
			return err
		}
	}
	for _, p := range s.parents {
		if p.parent != "" {
			cur, e := s.parents[p.parent].root.Lstat(p.name)
			if e != nil || !cur.IsDir() || !os.SameFile(cur, p.info) {
				return fixtureRefused
			}
		}
		mount, e := s.observe(p.file, p.mount)
		if e != nil {
			return e
		}
		if mount != p.mount {
			return fixtureRefused
		}
	}
	held, err := s.lock.f.Stat()
	if err != nil {
		return err
	}
	cur, err := s.parents["common"].root.Lstat(LockFileName)
	if err != nil || !cur.Mode().IsRegular() || !os.SameFile(cur, held) || !os.SameFile(held, s.lockInfo) {
		return fixtureRefused
	}
	return nil
}

// checkWorktree reopens the linked intent worktree by path and requires the
// retained root, as check does for the primary.
func (s *fixtureSession) checkWorktree() (err error) {
	root, err := safeopen.Root(s.repo.IntentRoot())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.closeRoot(root)) }()
	info, err := root.Stat(".")
	if err != nil || !os.SameFile(info, s.parents["worktree"].info) {
		return fixtureRefused
	}
	return nil
}
func (s *fixtureSession) closeHandles() error {
	var err error
	for _, p := range s.parents {
		err = errors.Join(err, s.closeFile(p.file), s.closeRoot(p.root))
	}
	for f, info := range s.fdinfo {
		err = errors.Join(err, info.Close())
		delete(s.fdinfo, f)
	}
	return err
}

// close adopts release only after successful construction. It removes no names.
// Its first result is sticky, including on repeated close; Lock.Close retains
// its existing public first-call error/idempotence semantics.
func (s *fixtureSession) close() error {
	if s == nil {
		return fixtureRefused
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.closeErr
	}
	s.closed = true
	if s.lock == nil {
		s.closeErr = fixtureRefused
		return s.closeErr
	}
	s.closeErr = errors.Join(s.closeHandles(), s.lock.Close())
	return s.closeErr
}
func (s *fixtureSession) syncParent(key string, checks ...func() error) error {
	return s.syncDir(key, false, checks...)
}

// publishSync is syncParent for the directory sync a link or replace makes
// after it published. Inside a deferred window (LGL-V0-016 proposed) the
// boundary and every check run now, in the same order, and only the sync
// call is queued for flushPublishSyncs.
func (s *fixtureSession) publishSync(key string, checks ...func() error) error {
	return s.syncDir(key, true, checks...)
}

func (s *fixtureSession) syncDir(key string, deferrable bool, checks ...func() error) error {
	if err := s.boundary("sync:" + key); err != nil {
		return err
	}
	if err := s.check(); err != nil {
		return err
	}
	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	if deferrable && s.publishSyncsDeferred {
		s.pendingSyncs = append(s.pendingSyncs, s.parents[key].file)
		return nil
	}
	return s.timedSync(func() error { return syncDirectory(s.parents[key].file) })
}

// flushPublishSyncs closes the deferred window and issues every queued
// directory sync concurrently, returning only when each has returned; any
// failure fails the flush. Its boundary is "sync:flush".
func (s *fixtureSession) flushPublishSyncs() error {
	return s.operation(func() error {
		pending := s.pendingSyncs
		s.pendingSyncs, s.publishSyncsDeferred = nil, false
		if err := s.boundary("sync:flush"); err != nil {
			return err
		}
		switch len(pending) {
		case 0:
			return nil
		case 1:
			return s.timedSync(func() error { return syncDirectory(pending[0]) })
		}
		start := time.Now()
		errs := make([]error, len(pending))
		var wg sync.WaitGroup
		for i, f := range pending {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = syncDirectory(f)
			}()
		}
		wg.Wait()
		s.syncTime += time.Since(start)
		s.syncCalls += len(pending)
		return errors.Join(errs...)
	})
}

// syncStaging is syncParent("staging"), except that a session with
// stagingSyncSkipped keeps the boundary and every check but makes no sync
// call (LGL-V0-005 proposed). The staging directory's own entries are never
// published state: each artifact's file sync and its destination's directory
// sync make it durable, and a slot name a crash restores or drops is the
// orphan a killed writer leaves (CAL-V0-019, CTS-V0-026).
func (s *fixtureSession) syncStaging(checks ...func() error) error {
	return s.syncStagingDir(false, checks...)
}

func (s *fixtureSession) syncStagingDir(deferrable bool, checks ...func() error) error {
	if !s.stagingSyncSkipped {
		return s.syncDir("staging", deferrable, checks...)
	}
	if err := s.boundary("sync:staging"); err != nil {
		return err
	}
	if err := s.check(); err != nil {
		return err
	}
	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}
func (s *fixtureSession) absent(key, name string) error {
	_, err := s.parents[key].root.Lstat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fixtureRefused
}
func fixtureSame(a, b os.FileInfo) bool {
	return a != nil && b != nil && a.Mode().IsRegular() && b.Mode().IsRegular() && os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime() == b.ModTime()
}
func fixtureConfirm(f *os.File, info os.FileInfo, size int64, digest wire.Digest) error {
	got, err := f.Stat()
	if err != nil {
		return err
	}
	if !fixtureSame(info, got) || got.Size() != size {
		return fixtureRefused
	}
	// Bound the read independently of a changed file size; no payload-sized allocation.
	h := sha256.New()
	n, err := io.Copy(h, io.NewSectionReader(f, 0, size+1))
	if err != nil {
		return err
	}
	if n != size || fmt.Sprintf("%x", h.Sum(nil)) != string(digest) {
		return fixtureRefused
	}
	after, err := f.Stat()
	if err != nil {
		return err
	}
	if !fixtureSame(info, after) {
		return fixtureRefused
	}
	return nil
}
func (s *fixtureSession) checkedFile(key, name string, info os.FileInfo, size int64, digest wire.Digest) (_ *os.File, err error) {
	p := s.parents[key]
	if p == nil {
		return nil, fixtureRefused
	}
	cur, err := p.root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !fixtureSame(info, cur) {
		return nil, fixtureRefused
	}
	f, err := safeopen.InRoot(p.root, name, os.O_RDONLY, 0, false)
	if err != nil {
		return nil, err
	}
	if err = fixtureConfirm(f, info, size, digest); err != nil {
		return nil, errors.Join(err, s.closeFile(f))
	}
	return f, nil
}

func (s *fixtureSession) prepare(slot fixtureSlot, role fixtureRole, data []byte) (*fixtureStage, error) {
	stages, err := s.prepareAll([]fixturePrepare{{slot: slot, role: role, data: data}})
	if err != nil {
		return nil, err
	}
	return stages[0], nil
}

type fixturePrepare struct {
	slot fixtureSlot
	role fixtureRole
	data []byte
}

// prepareAll stages every item in one operation. Each file is created,
// written and confirmed in item order with prepare's boundaries and checks;
// then every file is synced, concurrently when there are several, before any
// of them is closed, rechecked or registered (LGL-V0-013 proposed). A single
// item takes exactly prepare's path. No staged file is visible outside
// staging until a later link or replace, and each is synced before that, so
// syncing them together leaves every publication as durable as before.
//
// On any failure every file this call created is closed and cleaned up, as
// prepare cleans up its one file, and the error reports that a stage was
// created. Nothing is registered unless every item succeeds.
func (s *fixtureSession) prepareAll(items []fixturePrepare) (stages []*fixtureStage, err error) {
	if len(items) == 0 {
		return nil, fixtureRefused
	}
	for _, it := range items {
		limit, err := fixtureStageLimit(it.slot, it.role)
		if err != nil {
			return nil, err
		}
		if len(it.data) > limit || (len(it.data) == 0 && it.role != fixtureEvidence) {
			return nil, fixtureRefused
		}
		if it.role == fixtureVersion && string(it.data) != "taskman-state/0\n" {
			return nil, fixtureRefused
		}
	}
	err = s.operation(func() (err error) {
		p := s.parents["staging"]
		if p == nil {
			return fixtureRefused
		}
		owned := make([]*fixtureStage, 0, len(items))
		files := make([]*os.File, 0, len(items))
		defer func() {
			// The real close always runs, even when a deterministic close fault fires.
			for i, f := range files {
				if f != nil {
					err = errors.Join(err, s.closeFile(f))
					files[i] = nil
				}
			}
			if err != nil && len(owned) > 0 {
				for _, stage := range owned {
					err = errors.Join(err, s.cleanupStage(stage))
				}
				err = fixtureAfter("stage create", err)
			}
		}()
		for _, it := range items {
			if err = s.boundary("create"); err != nil {
				return err
			}
			if err = s.check(); err != nil {
				return err
			}
			f, err := safeopen.InRoot(p.root, string(it.slot), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600, false)
			if err != nil {
				return err
			}
			stage := &fixtureStage{owner: s, slot: it.slot, role: it.role, digest: wire.Sum(nil)}
			owned, files = append(owned, stage), append(files, f)
			if err = s.writeStage(stage, f, it.data); err != nil {
				return err
			}
		}
		if err = s.syncStages(files); err != nil {
			return err
		}
		for i, stage := range owned {
			f := files[i]
			if err = s.syncStaging(func() error { return s.checkStageFile(stage, f) }); err != nil {
				return err
			}
			files[i] = nil
			if err = errors.Join(s.boundary("stage-close"), s.closeFile(f)); err != nil {
				return err
			}
			if err = s.check(); err != nil {
				return err
			}
			check, err := s.checkedFile("staging", string(stage.slot), stage.info, stage.size, stage.digest)
			if err != nil {
				return err
			}
			if err = s.closeFile(check); err != nil {
				return err
			}
		}
		for _, stage := range owned {
			s.stages[stage] = true
		}
		stages = owned
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stages, nil
}

// writeStage writes one freshly created stage file and confirms it, up to
// and including the check that precedes its file sync.
func (s *fixtureSession) writeStage(owned *fixtureStage, f *os.File, data []byte) (err error) {
	if owned.info, err = f.Stat(); err != nil {
		return err
	}
	if err = s.boundary("write"); err != nil {
		return err
	}
	if err = s.checkStageFile(owned, f); err != nil {
		return err
	}
	n, writeErr := s.write(f, data)
	if n < 0 || n > len(data) {
		return errors.Join(fixtureRefused, writeErr)
	}
	owned.size, owned.digest = int64(n), wire.Sum(data[:n])
	owned.info, err = f.Stat()
	err = errors.Join(err, writeErr)
	if n != len(data) {
		err = errors.Join(err, io.ErrShortWrite)
	}
	if err != nil {
		return err
	}
	if err = fixtureConfirm(f, owned.info, owned.size, owned.digest); err != nil {
		return err
	}
	if err = s.boundary("file-sync"); err != nil {
		return err
	}
	return s.checkStageFile(owned, f)
}

// syncStages syncs every staged file. Several files are synced concurrently,
// so a filesystem that groups commits (ext4's journal, for one) pays about
// one sync latency for all of them; the call returns only when every sync
// has returned, and any failure fails the whole preparation.
func (s *fixtureSession) syncStages(files []*os.File) error {
	if len(files) == 1 {
		return s.timedSync(func() error { return syncFile(files[0]) })
	}
	start := time.Now()
	errs := make([]error, len(files))
	var wg sync.WaitGroup
	for i, f := range files {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = syncFile(f)
		}()
	}
	wg.Wait()
	s.syncTime += time.Since(start)
	s.syncCalls += len(files)
	return errors.Join(errs...)
}

// Cleanup is conditional on the exact known bytes and inode, even for a partial
// write. Unknown/foreign preparation remains visible; never recursively clean.
func (s *fixtureSession) cleanupStage(stage *fixtureStage) (err error) {
	performed := false
	defer func() {
		if performed {
			err = fixtureAfter("stage unlink", err)
		}
	}()
	if err = s.boundary("cleanup"); err != nil {
		return err
	}
	if err = s.check(); err != nil {
		return err
	}
	f, err := s.checkedFile("staging", string(stage.slot), stage.info, stage.size, stage.digest)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.closeFile(f)) }()
	if err = s.boundary("cleanup-unlink"); err != nil {
		return err
	}
	if err = s.checkStageFile(stage, f); err != nil {
		return err
	}
	if err = s.boundary("native:unlink"); err != nil {
		return err
	}
	if err = fixtureUnlink(s.parents["staging"].file, string(stage.slot), f); err != nil {
		return err
	}
	performed = true
	return s.syncStaging(func() error { return s.absent("staging", string(stage.slot)) })
}
func (s *fixtureSession) checkStageFile(stage *fixtureStage, f *os.File) error {
	if err := s.check(); err != nil {
		return err
	}
	cur, err := s.parents["staging"].root.Lstat(string(stage.slot))
	if err != nil {
		return err
	}
	if !fixtureSame(stage.info, cur) {
		return fixtureRefused
	}
	return fixtureConfirm(f, stage.info, stage.size, stage.digest)
}

func (s *fixtureSession) source(stage *fixtureStage, t fixtureTarget) (*os.File, string, string, error) {
	key, name, limit, err := t.location()
	if err != nil {
		return nil, "", "", err
	}
	if stage == nil || stage.owner != s || !s.stages[stage] || stage.size > int64(limit) {
		return nil, "", "", fixtureRefused
	}
	if stage.role == fixtureDescriptor && t.role != fixtureDescriptor {
		return nil, "", "", fixtureRefused
	}
	if stage.role != t.role && t.role != fixtureEvidence {
		return nil, "", "", fixtureRefused
	}
	if t.role == fixtureEvidence || t.role == fixturePin {
		if strings.TrimSuffix(name, ".json") != string(stage.digest) {
			return nil, "", "", fixtureRefused
		}
	}
	if t.role == fixtureDescriptor && stage.slot != "active.json.tmp" {
		return nil, "", "", fixtureRefused
	}
	p, err := s.pinShard(key)
	if err != nil {
		return nil, "", "", err
	}
	if p.mount != s.parents["staging"].mount {
		return nil, "", "", fixtureRefused
	}
	f, err := s.checkedFile("staging", string(stage.slot), stage.info, stage.size, stage.digest)
	return f, key, name, err
}

// pinShard returns the retained parent for key, pinning an existing request
// shard the first time a target under it is used (CAL-V0-234). The caller
// holds mu and the lock. The pin observes the shard through the retained
// requests/ root exactly as construction would have, and every later check
// re-verifies it with the other parents; a shard that does not exist is not
// created here (mkdir owns that) and refuses as before.
func (s *fixtureSession) pinShard(key string) (*fixtureParent, error) {
	if p := s.parents[key]; p != nil {
		return p, nil
	}
	if !strings.HasPrefix(key, "requests/") {
		return nil, fixtureRefused
	}
	if err := s.pinExisting(key); err != nil {
		return nil, err
	}
	if p := s.parents[key]; p != nil {
		return p, nil
	}
	return nil, fixtureRefused
}
func (s *fixtureSession) link(stage *fixtureStage, t fixtureTarget) error {
	return s.operation(func() (err error) {
		f, key, name, err := s.source(stage, t)
		if err != nil {
			return err
		}
		performed := false
		defer func() {
			err = errors.Join(err, s.closeFile(f))
			if performed {
				err = fixtureAfter("link", err)
			}
		}()
		if err = s.boundary("link"); err != nil {
			return err
		}
		if err = s.checkStageFile(stage, f); err != nil {
			return err
		}
		// linkat(flags=0) is exclusive, including on existing symlink/FIFO/dir.
		if err = s.boundary("native:link"); err != nil {
			return err
		}
		if err = fixtureLink(s.parents["staging"].file, string(stage.slot), s.parents[key].file, name, f); err != nil {
			return err
		}
		performed = true
		if err = s.publishSync(key, func() error { return s.checkStageFile(stage, f) }, func() error { return s.checkCurrent(key, name, f, stage.info, stage.digest) }); err != nil {
			return err
		}
		if t.role == fixtureDescriptor {
			active := *stage
			active.slot = "active.json"
			s.stages[&active] = true
		}
		return nil
	})
}

// replace covers private projections only. expected == nil means absent-only
// exclusive publication. Equal post is already applied, but still requires a
// directory sync (and a file sync) before clean success. No CAS is claimed.
func (s *fixtureSession) replace(stage *fixtureStage, t fixtureTarget, expected *wire.Digest) error {
	switch t.role {
	case fixtureHead, fixtureBarrier, fixtureReservations, fixturePools, fixturePrograms, fixtureAttempt:
	case fixtureQueue, fixturePolicy, fixtureImportMap, fixtureTicket, fixtureRelease, fixtureVersion:
		// An intent projection is replaced only under an explicit expected
		// digest. The §5.2 redo rule decides what the destination must
		// currently hold, and an unconditional overwrite of a projection
		// would discard a divergent edit instead of reporting it.
		if expected == nil {
			return fixtureRefused
		}
	default:
		return fixtureRefused
	}
	if expected == nil {
		return s.link(stage, t)
	}
	if _, err := wire.ParseDigest("", string(*expected)); err != nil {
		return err
	}
	return s.operation(func() (err error) {
		source, key, name, err := s.source(stage, t)
		if err != nil {
			return err
		}
		performed := false
		defer func() {
			err = errors.Join(err, s.closeFile(source))
			if performed {
				err = fixtureAfter("rename", err)
			}
		}()
		dest, info, digest, err := s.readCurrent(key, name, int(stageLimitForTarget(t)))
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, s.closeFile(dest)) }()
		if digest == stage.digest {
			if err = s.boundary("already-file-sync"); err != nil {
				return err
			}
			if err = s.checkCurrent(key, name, dest, info, digest); err != nil {
				return err
			}
			if err = s.timedSync(func() error { return syncFile(dest) }); err != nil {
				return err
			}
			return s.publishSync(key, func() error { return s.checkCurrent(key, name, dest, info, digest) })
		}
		if digest != *expected {
			return fixtureRefused
		}
		if err = s.boundary("rename"); err != nil {
			return err
		}
		if err = s.checkStageFile(stage, source); err != nil {
			return err
		}
		if err = s.checkCurrent(key, name, dest, info, digest); err != nil {
			return err
		}
		if err = s.boundary("native:rename"); err != nil {
			return err
		}
		if err = fixtureRename(s.parents["staging"].file, string(stage.slot), s.parents[key].file, name, source, dest); err != nil {
			return err
		}
		performed = true
		delete(s.stages, stage)
		// Both are attempted, even if the first fails. Rename consumed the slot.
		checkPost := func() error { return s.checkCurrent(key, name, source, stage.info, stage.digest) }
		checkSourceAbsent := func() error { return s.absent("staging", string(stage.slot)) }
		return errors.Join(s.publishSync(key, checkPost), s.syncStagingDir(true, checkPost, checkSourceAbsent))
	})
}
func stageLimitForTarget(t fixtureTarget) int { _, _, limit, _ := t.location(); return limit }
func (s *fixtureSession) readCurrent(key, name string, limit int) (_ *os.File, info os.FileInfo, digest wire.Digest, err error) {
	p := s.parents[key]
	if p == nil {
		return nil, nil, "", fixtureRefused
	}
	info, err = p.root.Lstat(name)
	if err != nil {
		return nil, nil, "", err
	}
	if !info.Mode().IsRegular() || info.Size() > int64(limit) {
		return nil, nil, "", fixtureRefused
	}
	f, err := safeopen.InRoot(p.root, name, os.O_RDONLY, 0, false)
	if err != nil {
		return nil, nil, "", err
	}
	h := sha256.New()
	n, err := io.Copy(h, io.NewSectionReader(f, 0, int64(limit)+1))
	digest = wire.Digest(fmt.Sprintf("%x", h.Sum(nil)))
	if err == nil && n != info.Size() {
		err = fixtureRefused
	}
	if err == nil {
		err = s.checkCurrent(key, name, f, info, digest)
	}
	if err != nil {
		return nil, nil, "", errors.Join(err, s.closeFile(f))
	}
	return f, info, digest, nil
}
func (s *fixtureSession) checkCurrent(key, name string, f *os.File, info os.FileInfo, digest wire.Digest) error {
	if err := s.check(); err != nil {
		return err
	}
	cur, err := s.parents[key].root.Lstat(name)
	if err != nil {
		return err
	}
	if !fixtureSame(cur, info) {
		return fixtureRefused
	}
	return fixtureConfirm(f, info, info.Size(), digest)
}
func (s *fixtureSession) removeBarrier(t fixtureTarget, expected wire.Digest) error {
	if t.role != fixtureBarrier {
		return fixtureRefused
	}
	key, name, limit, err := t.location()
	if err != nil {
		return err
	}
	if _, err = wire.ParseDigest("", string(expected)); err != nil {
		return err
	}
	return s.operation(func() (err error) {
		f, info, digest, err := s.readCurrent(key, name, limit)
		if os.IsNotExist(err) {
			return s.syncParent(key, func() error { return s.absent(key, name) })
		}
		if err != nil {
			return err
		}
		performed := false
		defer func() {
			err = errors.Join(err, s.closeFile(f))
			if performed {
				err = fixtureAfter("unlink", err)
			}
		}()
		if digest != expected {
			return fixtureRefused
		}
		if err = s.boundary("unlink"); err != nil {
			return err
		}
		if err = s.checkCurrent(key, name, f, info, digest); err != nil {
			return err
		}
		if err = s.boundary("native:unlink"); err != nil {
			return err
		}
		if err = fixtureUnlink(s.parents[key].file, name, f); err != nil {
			return err
		}
		performed = true
		return s.syncParent(key, func() error { return s.absent(key, name) })
	})
}
func (s *fixtureSession) removeStage(slot fixtureSlot) error {
	return s.operation(func() error {
		for stage := range s.stages {
			if stage.slot == slot {
				err := s.cleanupStage(stage)
				// Any removal attempt invalidates the token, including incomplete durability.
				delete(s.stages, stage)
				return err
			}
		}
		return fixtureRefused
	})
}
func (s *fixtureSession) removeOrphanStages() (removed int, err error) {
	err = s.operation(func() error {
		p := s.parents["staging"]
		if p == nil {
			return nil
		}
		if len(s.stages) != 0 {
			return fixtureRefused
		}
		// A staging descriptor marks active staging, not orphans: it is left
		// for the journal audit to refuse.
		for _, name := range []string{"active.json", "active.json.tmp"} {
			if _, err := p.root.Lstat(name); !os.IsNotExist(err) {
				return err
			}
		}
		for i := 0; i <= 10; i++ {
			name := fmt.Sprintf("a%02d", i)
			_, err := p.root.Lstat(name)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			if err = s.unlinkOrphan(name); err != nil {
				return err
			}
			removed++
		}
		return nil
	})
	return removed, err
}
func (s *fixtureSession) unlinkOrphan(name string) (err error) {
	f, info, digest, err := s.readCurrent("staging", name, wire.MaxEvidenceBlobBytes)
	if err != nil {
		return err
	}
	performed := false
	defer func() {
		err = errors.Join(err, s.closeFile(f))
		if performed {
			err = fixtureAfter("unlink", err)
		}
	}()
	if err = s.boundary("orphan-unlink"); err != nil {
		return err
	}
	if err = s.checkCurrent("staging", name, f, info, digest); err != nil {
		return err
	}
	if err = s.boundary("native:unlink"); err != nil {
		return err
	}
	if err = fixtureUnlink(s.parents["staging"].file, name, f); err != nil {
		return err
	}
	performed = true
	return s.syncStaging(func() error { return s.absent("staging", name) })
}
func (s *fixtureSession) mkdir(key string) error {
	parent, name, ok := fixtureDirectory(key)
	if !ok || key == "common" {
		return fixtureRefused
	}
	parent = s.parentKey(key, parent)
	return s.operation(func() (err error) {
		p := s.parents[parent]
		if p == nil || s.parents[key] != nil {
			return fixtureRefused
		}
		if err = s.boundary("mkdir"); err != nil {
			return err
		}
		if err = s.check(); err != nil {
			return err
		}
		if err = s.boundary("native:mkdir"); err != nil {
			return err
		}
		if err = fixtureMkdir(p.file, name); err != nil {
			return err
		}
		defer func() { err = fixtureAfter("mkdir", err) }()
		// No recursive cleanup, even when retaining or syncing the new child fails.
		if err = s.pinExisting(key); err != nil {
			return errors.Join(err, s.syncParent(parent))
		}
		if s.parents[key] == nil {
			return errors.Join(fixtureRefused, s.syncParent(parent))
		}
		return errors.Join(s.syncParent(key), s.syncParent(parent))
	})
}

// timedSync runs one durability call and adds its wall time and one call to
// the session's issue 494 diagnostic sums. The caller holds mu.
func (s *fixtureSession) timedSync(sync func() error) error {
	start := time.Now()
	err := sync()
	s.syncTime += time.Since(start)
	s.syncCalls++
	return err
}
