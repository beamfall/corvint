package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The verified audit cache (CAL-V0-026) lets a §5.2 writer reuse the last
// full audit instead of repeating it. It is derived state beside the store
// lock, outside the closed state-dir layout: deleting or corrupting it only
// costs the next writer one full audit. Only writers holding the store lock
// write it; read commands never read or write it.
//
// A cached view is used only when all of these hold for the current store:
//   - head.json has the cached head digest;
//   - the intent tree digest, computed from the bytes on disk, is the cached
//     one, so every intent byte is exactly the audited one;
//   - the private state dir lists exactly the cached paths, and each file has
//     the cached mode, size, mtime, change time, inode and device. A file
//     whose change time falls within racyWindow of the moment its content
//     was last checked is re-read and re-hashed, because a change within the
//     same timestamp tick would leave its stamp unmoved.
//
// The audit is a function of those bytes and the reader's queue, primary
// worktree and limits, which are bound too. So a cached view reports what a
// full strict audit of the current store would report.
const (
	auditCacheName    = "taskman.audit-cache"
	auditCacheVersion = 1
)

// racyWindow is a var only so tests can show a stamp change is caught
// without the re-hash.
var racyWindow = 2 * time.Second

// fullAudits counts full writer audits, so tests can show a cache hit.
var fullAudits atomic.Int64

type stamp struct {
	Mode               uint32
	Size, Mtime, Ctime int64
	Ino, Dev           uint64
}

// stateEntry is one state-dir path. Checked is the time, in Unix
// nanoseconds, at or before which the file's bytes were known to hash to
// Sha256; zero means never checked. Directories carry only their presence.
type stateEntry struct {
	Dir     bool        `json:"d,omitempty"`
	Stamp   stamp       `json:"s"`
	Sha256  wire.Digest `json:"h,omitempty"`
	Bytes   wire.Size   `json:"b,omitempty"`
	Checked int64       `json:"c,omitempty"`
}

type cacheFile struct {
	Version          int
	StateDir         string
	PrimaryWorktree  string
	QueueID          string
	HeadSha256       wire.Digest
	IntentTreeSha256 wire.Digest
	State            map[string]stateEntry
}

// auditView is what a writer plans against: the audited identity, the
// transaction inventory, and the canonical intent records.
type auditView struct {
	identity journal.Identity
	queueID  string
	staging  bool // the audit saw staging; the caller refuses
	cached   bool // validated from the cache, not audited
	inv      *transaction.Inventory
	records  map[string][]byte // intent path -> canonical bytes
	tickets  [][]byte
	releases [][]byte
	// state and files are nil when the view cannot be cached.
	state map[string]stateEntry
	files map[string]intent.File // intent tree, store-relative paths
}

func (v *auditView) raw(path string) []byte { return v.records[path] }

// replayed is a request already in the journal and its receipt's target.
type replayed struct {
	entry  mutation.IndexEntry
	ticket string
}

// writerView looks the request up and returns the view to plan against. On a
// verified cache hit the lookup is a direct read of the request file, which
// the view vouches for. Otherwise the request-index audit and the full audit
// run as before, and a strict result refreshes the cache. relaxed selects
// the UNPAUSE barrier-removal audit, which a strict cached view satisfies.
func writerView(repo *intent.Repository, head *snapshot.Head, requestID string, relaxed bool) (*auditView, *replayed, error) {
	if v := cachedView(repo, head); v != nil {
		r, err := v.lookup(repo, requestID)
		if err != nil || r != nil {
			return nil, r, err
		}
		return v, nil, nil
	}
	reader := journalReader(repo, head)
	index := journal.RequestIndex{Reader: reader}
	entry, found, err := index.Lookup(requestID)
	if err != nil {
		return nil, nil, err
	}
	if found {
		return nil, &replayed{entry, index.TicketID}, nil
	}
	v, err := freshView(repo, reader, relaxed)
	if err != nil {
		return nil, nil, err
	}
	v.save(repo)
	return v, nil, nil
}

// storeView is the view with no request lookup: the verified cache, or else
// one full strict audit, which refreshes the cache.
func storeView(repo *intent.Repository, head *snapshot.Head) (*auditView, error) {
	if v := cachedView(repo, head); v != nil {
		return v, nil
	}
	v, err := freshView(repo, journalReader(repo, head), false)
	if err != nil {
		return nil, err
	}
	v.save(repo)
	return v, nil
}

// recheckPosts confirms that every destination a plan posts still holds what
// the view recorded, or is still absent. The head is checked by its digest.
func (v *auditView) recheckPosts(repo *intent.Repository, plan *transaction.Plan) error {
	for _, a := range plan.Artifacts() {
		if a.Target == "head.json" {
			continue
		}
		e, known := v.state[a.Target]
		full, rel, want := filepath.Join(repo.StateDir, filepath.FromSlash(a.Target)), a.Target, e.Sha256
		if rest, ok := strings.CutPrefix(a.Target, "intent/"); ok {
			f, ok := v.files[rest]
			full, rel, want, known = filepath.Join(repo.PrimaryWorktree, intent.Dir, filepath.FromSlash(rest)), rest, f.Sha256, ok
		}
		entry, present, err := fileEntry(full, rel)
		if err != nil {
			return err
		}
		if present != known || (present && entry.Sha256 != want) {
			return wire.Errorf(wire.CodeSnapshotMoved, a.Target, "validated destination changed")
		}
	}
	return nil
}

// freshView takes the full audit. The state dir is stamped before anything
// is read, so a change made while the audit reads leaves a stamp that no
// longer matches and the cache misses.
func freshView(repo *intent.Repository, reader journal.Reader, relaxed bool) (*auditView, error) {
	fullAudits.Add(1)
	checked := time.Now().UnixNano()
	state, stateErr := statState(repo.StateDir)
	inv, err := inventory(repo)
	if err != nil {
		return nil, err
	}
	paths := []string{"intent/queue.json", "intent/policy.json"}
	for _, file := range inv.Files() {
		if strings.HasPrefix(file.Path, "intent/tickets/") || strings.HasPrefix(file.Path, "intent/releases/") {
			paths = append(paths, file.Path)
		}
	}
	audit := reader.Audit
	if relaxed {
		audit = reader.BarrierRemoval
	}
	proof, err := audit(paths...)
	if err != nil {
		return nil, err
	}
	v := &auditView{identity: proof.Identity, queueID: reader.QueueID.Raw, staging: proof.StagingPresent, inv: inv, records: map[string][]byte{}}
	for _, path := range paths {
		v.records[path] = proof.Records[path].Raw
	}
	v.split()
	if relaxed || proof.Pending || proof.StagingPresent || stateErr != nil {
		return v, nil
	}
	tree, err := intent.TreeDigest(repo.PrimaryWorktree)
	if err != nil || tree.Sha256 != proof.Identity.IntentTreeSha256 {
		return v, nil
	}
	for _, f := range inv.Files() {
		if strings.HasPrefix(f.Path, "intent/") {
			continue
		}
		e, ok := state[f.Path]
		if !ok || e.Dir || e.Stamp.Size != int64(f.Bytes.Uint64()) {
			return v, nil
		}
		e.Sha256, e.Bytes = f.Sha256, f.Bytes
		state[f.Path] = e
	}
	for p, e := range state {
		e.Checked = checked
		state[p] = e
	}
	v.state, v.files = state, treeFiles(tree.Files)
	return v, nil
}

// cachedView validates the cache against the store and returns nil on any
// mismatch or error, so the caller takes the full audit instead.
func cachedView(repo *intent.Repository, head *snapshot.Head) *auditView {
	c := loadCache(repo)
	if c == nil || c.QueueID != head.QueueID.Raw {
		return nil
	}
	headRaw, err := intent.ReadFile(filepath.Join(repo.StateDir, "head.json"), 4096)
	if err != nil || wire.Sum(headRaw) != c.HeadSha256 {
		return nil
	}
	walked := time.Now().UnixNano()
	state, err := statState(repo.StateDir)
	if err != nil || len(state) != len(c.State) {
		return nil
	}
	for p, cached := range c.State {
		now, ok := state[p]
		if !ok || now.Dir != cached.Dir || (!cached.Dir && now.Stamp != cached.Stamp) {
			return nil
		}
		if cached.Dir || (cached.Checked != 0 && cached.Stamp.Ctime < cached.Checked-racyWindow.Nanoseconds()) {
			continue
		}
		if cached.Sha256 == "" {
			return nil
		}
		entry, ok, err := fileEntry(filepath.Join(repo.StateDir, filepath.FromSlash(p)), p)
		if err != nil || !ok || entry.Sha256 != cached.Sha256 || entry.Bytes != cached.Bytes {
			return nil
		}
		cached.Checked = walked
		c.State[p] = cached
	}
	tree, err := intent.TreeDigest(repo.PrimaryWorktree)
	if err != nil || tree.Sha256 != c.IntentTreeSha256 {
		return nil
	}
	v := &auditView{identity: journal.Identity{HeadSha256: c.HeadSha256, IntentTreeSha256: c.IntentTreeSha256}, queueID: c.QueueID, cached: true, state: c.State, files: treeFiles(tree.Files)}
	if v.build() != nil {
		return nil
	}
	return v
}

// lookup reads the request file a verified view lists. The audit the view
// stands for bound it to its receipt, so it is the request index's answer.
func (v *auditView) lookup(repo *intent.Repository, requestID string) (*replayed, error) {
	p, err := snapshot.RequestPath(requestID)
	if err != nil {
		return nil, err
	}
	e, ok := v.state[p]
	if !ok {
		return nil, nil
	}
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, filepath.FromSlash(p)), wire.MaxEvidenceBlobBytes)
	if err != nil {
		return nil, err
	}
	if wire.Sum(raw) != e.Sha256 {
		return nil, wire.Errorf(wire.CodeSnapshotMoved, p, "validated request changed")
	}
	req, err := snapshot.DecodeRequest(raw)
	if err != nil {
		return nil, err
	}
	receipt, found, err := receiptBytes(repo, req.Seq.Uint64())
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, wire.Errorf(wire.CodeSnapshotMoved, p, "validated receipt changed")
	}
	rc, err := snapshot.DecodeReceipt(receipt)
	if err != nil {
		return nil, err
	}
	out := &replayed{entry: req.Entry}
	if rc.TicketID != nil {
		out.ticket = rc.TicketID.Raw
	}
	return out, nil
}

// advance moves a cacheable view past its own committed plan: the written
// state files are re-stamped and marked unchecked, so the next validation
// re-hashes them, and the intent tree takes the posted bytes. Anything else
// that moved is caught by the next validation.
func (v *auditView) advance(repo *intent.Repository, plan *transaction.Plan) {
	if v.state == nil {
		return
	}
	v.state, v.files = v.advanced(repo, plan)
	if v.state == nil || v.build() != nil {
		v.state, v.files = nil, nil
	}
}

// recheck confirms, just before apply, that the store is still the one the
// view describes: a cached view is validated again, an audited one is
// audited again and must have the same identity.
func (v *auditView) recheck(repo *intent.Repository, head *snapshot.Head, audit func(...string) (*journal.Result, error)) error {
	if v.cached {
		if again := cachedView(repo, head); again == nil || again.identity != v.identity {
			return wire.Errorf(wire.CodeSnapshotMoved, "barrier", "validated snapshot changed")
		}
		return nil
	}
	current, err := audit()
	if err != nil {
		return err
	}
	if current.StagingPresent || current.Identity != v.identity {
		return wire.Errorf(wire.CodeSnapshotMoved, "barrier", "validated snapshot changed")
	}
	return nil
}

// commit saves the view advanced past its applied plan. v itself keeps the
// records the plan was modelled on; only its cache maps move.
func (v *auditView) commit(repo *intent.Repository, plan *transaction.Plan) {
	next := *v
	next.advance(repo, plan)
	next.save(repo)
}

func (v *auditView) advanced(repo *intent.Repository, plan *transaction.Plan) (map[string]stateEntry, map[string]intent.File) {
	receipt, err := snapshot.DecodeReceipt(plan.Receipt())
	if err != nil {
		return nil, nil
	}
	state, files := v.state, v.files
	for _, post := range receipt.Post {
		if post.Sha256 == nil {
			delete(state, post.Path)
			delete(files, strings.TrimPrefix(post.Path, "intent/"))
		}
	}
	touched := []string{"staging"}
	for _, a := range plan.Artifacts() {
		if rest, ok := strings.CutPrefix(a.Target, "intent/"); ok {
			files[rest] = intent.File{Path: rest, Sha256: a.Sha256, Bytes: len(a.Data), Raw: a.Data}
			continue
		}
		info, err := os.Lstat(filepath.Join(repo.StateDir, filepath.FromSlash(a.Target)))
		if err != nil {
			return nil, nil
		}
		s, ok := fileStamp(info)
		if !ok {
			return nil, nil
		}
		state[a.Target] = stateEntry{Stamp: s, Sha256: a.Sha256, Bytes: a.Bytes}
		for dir := filepath.ToSlash(filepath.Dir(a.Target)); dir != "."; dir = filepath.ToSlash(filepath.Dir(dir)) {
			touched = append(touched, dir)
		}
	}
	for _, dir := range touched {
		if info, err := os.Lstat(filepath.Join(repo.StateDir, filepath.FromSlash(dir))); err == nil && info.IsDir() {
			state[dir] = stateEntry{Dir: true}
		} else if dir == "staging" {
			delete(state, dir)
		}
	}
	all := make([]intent.File, 0, len(files))
	for _, f := range files {
		all = append(all, f)
	}
	v.identity = journal.Identity{HeadSha256: wire.Sum(plan.Head()), IntentTreeSha256: intent.DigestOfFiles(all)}
	return state, files
}

// build derives the inventory and canonical records of a cached view from
// its stamps and intent tree, as inventory() would scan them.
func (v *auditView) build() error {
	var entries []archive.FileEntry
	var dirs []string
	for p, e := range v.state {
		if !scanned(p) {
			continue
		}
		if e.Dir {
			dirs = append(dirs, p)
			continue
		}
		entries = append(entries, archive.FileEntry{Path: p, Sha256: e.Sha256, Bytes: e.Bytes})
	}
	v.records = map[string][]byte{}
	for rel, f := range v.files {
		if rel == intent.ImportMapFile {
			continue
		}
		p := "intent/" + rel
		entries = append(entries, archive.FileEntry{Path: p, Sha256: f.Sha256, Bytes: wire.SizeOf(uint64(f.Bytes))})
		v.records[p] = f.Raw
	}
	slices.Sort(dirs)
	archive.SortFiles(entries)
	inv, err := transaction.NewInventory(entries, dirs)
	if err != nil {
		return err
	}
	v.inv = inv
	v.split()
	return nil
}

// split lists canonical tickets and releases in inventory order.
func (v *auditView) split() {
	v.tickets, v.releases = nil, nil
	for _, f := range v.inv.Files() {
		switch {
		case strings.HasPrefix(f.Path, "intent/tickets/"):
			v.tickets = append(v.tickets, v.records[f.Path])
		case strings.HasPrefix(f.Path, "intent/releases/"):
			v.releases = append(v.releases, v.records[f.Path])
		}
	}
}

// scanned reports whether scan() inventories a state-dir path.
func scanned(p string) bool {
	if slices.Contains(stateFiles, p) {
		return true
	}
	for _, dir := range scanDirectories {
		if p == dir || strings.HasPrefix(p, dir+"/") {
			return true
		}
	}
	return false
}

func treeFiles(list []intent.File) map[string]intent.File {
	out := make(map[string]intent.File, len(list))
	for _, f := range list {
		out[f.Path] = f
	}
	return out
}

// statState stamps every state-dir path the journal audit observes: all of
// it except the contents of worktrees/.
func statState(stateDir string) (map[string]stateEntry, error) {
	out := map[string]stateEntry{}
	pending := []string{"."}
	for len(pending) > 0 {
		dir := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		entries, err := os.ReadDir(filepath.Join(stateDir, filepath.FromSlash(dir)))
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			p := e.Name()
			if dir != "." {
				p = dir + "/" + p
			}
			if e.IsDir() {
				out[p] = stateEntry{Dir: true}
				if p != "worktrees" {
					pending = append(pending, p)
				}
				continue
			}
			info, err := e.Info()
			if err != nil {
				return nil, err
			}
			s, ok := fileStamp(info)
			if !ok {
				return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, p, "no change time")
			}
			out[p] = stateEntry{Stamp: s}
		}
	}
	return out, nil
}

// save writes a cacheable view beside the store lock: the body, then a
// newline and the hex SHA-256 of the body. Failure only costs a later miss.
func (v *auditView) save(repo *intent.Repository) {
	if v.state == nil {
		return
	}
	body, err := json.Marshal(cacheFile{Version: auditCacheVersion, StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree, QueueID: v.queueID, HeadSha256: v.identity.HeadSha256, IntentTreeSha256: v.identity.IntentTreeSha256, State: v.state})
	if err != nil {
		return
	}
	sum := sha256.Sum256(body)
	path := filepath.Join(repo.CommonDir, auditCacheName)
	tmp := path + ".tmp"
	if os.WriteFile(tmp, append(append(body, '\n'), hex.EncodeToString(sum[:])...), 0o600) != nil {
		_ = os.Remove(tmp)
		return
	}
	if os.Rename(tmp, path) != nil {
		_ = os.Remove(tmp)
	}
}

func loadCache(repo *intent.Repository) *cacheFile {
	raw, err := os.ReadFile(filepath.Join(repo.CommonDir, auditCacheName))
	if err != nil {
		return nil
	}
	i := bytes.LastIndexByte(raw, '\n')
	if i < 0 {
		return nil
	}
	sum := sha256.Sum256(raw[:i])
	if string(raw[i+1:]) != hex.EncodeToString(sum[:]) {
		return nil
	}
	c := &cacheFile{}
	if json.Unmarshal(raw[:i], c) != nil || c.Version != auditCacheVersion || c.StateDir != repo.StateDir || c.PrimaryWorktree != repo.PrimaryWorktree || c.State == nil {
		return nil
	}
	return c
}
