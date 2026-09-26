package authority

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

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
	fixtureQueue
	fixturePolicy
	fixtureImportMap
	fixtureTicket
	fixtureRelease
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
	// Per-session deterministic syscall boundaries; tests do not swap globals.
	before    func(string) error
	write     func(*os.File, []byte) (int, error)
	closeFile func(*os.File) error
	closeRoot func(*os.Root) error
	observe   func(*os.File, fixtureMount) (fixtureMount, error)
}

func fixtureDirectory(key string) (parent, name string, ok bool) {
	switch key {
	case "common":
		return "primary", ".git", true
	case "state":
		return "common", "taskman", true
	case "intent":
		return "primary", intent.Dir, true
	case "staging", "receipts", "evidence", "pinned", "requests":
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
			return "staging", name, 2422, nil
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
	}
	return "", "", 0, fixtureRefused
}
func fixtureStageLimit(slot fixtureSlot, role fixtureRole) (int, error) {
	if slot == "active.json.tmp" && role == fixtureDescriptor {
		return 2422, nil
	}
	if len(slot) != 3 || slot[0] != 'a' || slot < "a00" || slot > "a10" || slot[1] < '0' || slot[1] > '1' || slot[2] < '0' || slot[2] > '9' {
		return 0, fixtureRefused
	}
	limits := map[fixtureRole]int{
		fixtureReceipt: wire.MaxReceiptFileBytes, fixtureEvidence: wire.MaxEvidenceBlobBytes,
		fixturePin: wire.MaxPinnedBytes, fixtureRequest: wire.MaxAttemptRecordBytes,
		fixtureVersion: len("taskman-state/0\n"), fixtureHead: wire.MaxJournalHeadBytes,
		fixtureBarrier: wire.MaxBarrierBytes, fixtureReservations: wire.MaxReservationSetBytes,
		fixtureQueue: wire.MaxQueueFileBytes, fixturePolicy: wire.MaxPolicyFileBytes,
		fixtureImportMap: wire.MaxImportMapBytes, fixtureTicket: wire.MaxTicketFileBytes, fixtureRelease: wire.MaxReleaseFileBytes,
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
	if again.CommonDir != repo.CommonDir || again.StateDir != repo.StateDir || again.LockPath != repo.LockPath || again.PrimaryWorktree != repo.PrimaryWorktree {
		return nil, fixtureRefused
	}
	s := &fixtureSession{lock: lock, repo: *again, parents: map[string]*fixtureParent{}, stages: map[*fixtureStage]bool{}, write: (*os.File).Write, closeFile: (*os.File).Close, closeRoot: (*os.Root).Close, observe: fixtureObserveMount}
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
	for _, key := range []string{"common", "state", "staging", "receipts", "evidence", "pinned", "requests", "intent", "tickets", "releases"} {
		if err = s.pinExisting(key); err != nil {
			return nil, err
		}
	}
	if s.parents["requests"] != nil {
		for i := 0; i < 256; i++ {
			if err = s.pinExisting(fmt.Sprintf("requests/%02x", i)); err != nil {
				return nil, err
			}
		}
	}
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
func (s *fixtureSession) pinExisting(key string) error {
	parent, name, ok := fixtureDirectory(key)
	if !ok {
		return fixtureRefused
	}
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
func (s *fixtureSession) closeHandles() error {
	var err error
	for _, p := range s.parents {
		err = errors.Join(err, s.closeFile(p.file), s.closeRoot(p.root))
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
	return syncDirectory(s.parents[key].file)
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

func (s *fixtureSession) prepare(slot fixtureSlot, role fixtureRole, data []byte) (stage *fixtureStage, err error) {
	limit, err := fixtureStageLimit(slot, role)
	if err != nil {
		return nil, err
	}
	if len(data) > limit || (len(data) == 0 && role != fixtureEvidence) {
		return nil, fixtureRefused
	}
	if role == fixtureVersion && string(data) != "taskman-state/0\n" {
		return nil, fixtureRefused
	}
	err = s.operation(func() (err error) {
		p := s.parents["staging"]
		if p == nil {
			return fixtureRefused
		}
		if err = s.boundary("create"); err != nil {
			return err
		}
		if err = s.check(); err != nil {
			return err
		}
		f, err := safeopen.InRoot(p.root, string(slot), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600, false)
		if err != nil {
			return err
		}
		owned := &fixtureStage{owner: s, slot: slot, role: role, digest: wire.Sum(nil)}
		owned.info, err = f.Stat()
		closed := false
		defer func() {
			if !closed {
				err = errors.Join(err, s.closeFile(f))
			}
			if err != nil {
				stage = nil
				err = fixtureAfter("stage create", errors.Join(err, s.cleanupStage(owned)))
			}
		}()
		if err != nil {
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
		if err = s.checkStageFile(owned, f); err != nil {
			return err
		}
		if err = syncFile(f); err != nil {
			return err
		}
		if err = s.syncParent("staging", func() error { return s.checkStageFile(owned, f) }); err != nil {
			return err
		}
		// The real close always runs, even when a deterministic close fault fires.
		err = errors.Join(s.boundary("stage-close"), s.closeFile(f))
		closed = true
		if err != nil {
			return err
		}
		if err = s.check(); err != nil {
			return err
		}
		check, err := s.checkedFile("staging", string(slot), owned.info, owned.size, owned.digest)
		if err != nil {
			return err
		}
		if err = s.closeFile(check); err != nil {
			return err
		}
		s.stages[owned] = true
		stage = owned
		return nil
	})
	return stage, err
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
	return s.syncParent("staging", func() error { return s.absent("staging", string(stage.slot)) })
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
	p := s.parents[key]
	if p == nil {
		return nil, "", "", fixtureRefused
	}
	if p.mount != s.parents["staging"].mount {
		return nil, "", "", fixtureRefused
	}
	f, err := s.checkedFile("staging", string(stage.slot), stage.info, stage.size, stage.digest)
	return f, key, name, err
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
		if err = s.syncParent(key, func() error { return s.checkStageFile(stage, f) }, func() error { return s.checkCurrent(key, name, f, stage.info, stage.digest) }); err != nil {
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
	case fixtureHead, fixtureBarrier, fixtureReservations:
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
			if err = syncFile(dest); err != nil {
				return err
			}
			return s.syncParent(key, func() error { return s.checkCurrent(key, name, dest, info, digest) })
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
		return errors.Join(s.syncParent(key, checkPost), s.syncParent("staging", checkPost, checkSourceAbsent))
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
func (s *fixtureSession) mkdir(key string) error {
	parent, name, ok := fixtureDirectory(key)
	if !ok || key == "common" {
		return fixtureRefused
	}
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
