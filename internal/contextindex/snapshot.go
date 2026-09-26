package contextindex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
)

// The index snapshot (index-snapshot-v0) is the compiled Index of one
// committed tree, written once by `corvint index` and read by the packet
// verbs in place of a rebuild. Every table is a pure function of the tree
// (DIRTY-CACHE-001: sources come only from `HEAD^{tree}`), so the file is
// keyed by object format, tree OID, and the digest of the binary that
// compiled it; the two fields that depend on the worktree, DirtyPaths and
// StatusSHA256, are left out and filled in at load time from `git status`.
// Nothing in the file depends on the worktree, so it lives under the Git
// common directory and every linked worktree at that tree reads the one copy
// (DIRTY-CACHE-013).
const (
	snapshotFormat = "corvint-index-snapshot/1"
	snapshotKeep   = 8
	// snapshotKeepCap caps the shared store's scaled entry bound.
	snapshotKeepCap = 64
	// sharedSnapshotSubpath is the store under the Git common directory;
	// snapshotSubpath is the per-worktree fallback for a root whose common
	// directory does not resolve from its `.git` metadata.
	sharedSnapshotSubpath = "corvint/index"
	snapshotSubpath       = ".corvint/index"
	// snapshotTemporaryStaleAfter is how long a writer's temporary may live.
	// A writer holds one only while it encodes and syncs an index it already
	// built, seconds for the largest snapshot, so an older one is a crashed
	// writer's orphan (IDX-SNAP-V0-007).
	snapshotTemporaryStaleAfter = 10 * time.Minute
	// snapshotStoreBytes bounds the published gob snapshots in one store
	// beside the entry bound, which alone lets 64 large snapshots pile up.
	snapshotStoreBytes = 1 << 30
	corvintIgnore      = "/.gitignore\n/index/\n/self-observations.jsonl\n/.self-observations.*\n"
)

type snapshotHeader struct {
	Format, ObjectFormat, Tree, Engine string
}

// eventSnapshot is the subset of Index read by Impact and compact-session
// rehydration. Gob skips the snapshot's query-only Tracked and Vocabulary
// fields while retaining the same on-disk format.
type eventSnapshot struct {
	Root, ObjectFormat, CommitRevision, Revision, ProfileID, Module string
	StatusSHA256                                                    string
	Sources                                                         map[string]Source
	Exclusions                                                      []Exclusion
	UnsupportedSuffixCount                                          int
	DirtyPaths                                                      []string
	Features, Scenarios, Documents                                  map[string]Record
	Markers                                                         map[string][]Marker
	Symbols                                                         []Symbol
	Imports                                                         map[string]map[string]struct{}
	Unparsed                                                        []Unparsed
	ApproximateImports                                              int
	ExtractionNotes                                                 []ExtractionNote
}

func (snapshot *eventSnapshot) index() *Index {
	return &Index{
		Root: snapshot.Root, ObjectFormat: snapshot.ObjectFormat,
		CommitRevision: snapshot.CommitRevision, Revision: snapshot.Revision,
		ProfileID: snapshot.ProfileID, Module: snapshot.Module, StatusSHA256: snapshot.StatusSHA256,
		Sources: snapshot.Sources, Exclusions: snapshot.Exclusions, DirtyPaths: snapshot.DirtyPaths,
		UnsupportedSuffixCount: snapshot.UnsupportedSuffixCount, Features: snapshot.Features,
		Scenarios: snapshot.Scenarios, Documents: snapshot.Documents,
		Markers: snapshot.Markers, Symbols: snapshot.Symbols, Imports: snapshot.Imports,
		Unparsed: snapshot.Unparsed, ApproximateImports: snapshot.ApproximateImports,
		ExtractionNotes: snapshot.ExtractionNotes,
	}
}

type compactEventSnapshot struct{ ProfileID string }

// SnapshotReceipt is what `corvint index` reports about the file it wrote.
type SnapshotReceipt struct {
	Path                 string
	Bytes                int64
	Tree, Commit, Engine string
	Sources, Symbols     int
	Evicted              int
	// SectionedPath and SectionedBytes name the experimental sectioned file
	// written beside the gob snapshot under CORVINT_SNAPSHOT_FORMAT=sectioned.
	SectionedPath  string
	SectionedBytes int64
	// PackPath and PackBytes name the experimental pack (`.aip`) written
	// beside the gob snapshot under CORVINT_SNAPSHOT_FORMAT=pack (proposed
	// IDX-SNAP-V0-015).
	PackPath  string
	PackBytes int64
}

// SnapshotProbe identifies a matching snapshot without decoding its index.
type SnapshotProbe struct {
	Path, Tree, Commit, Engine string
}

func init() {
	gob.Register([]any{})
	gob.Register(map[string]any{})
}

var (
	engineOnce   sync.Once
	engineDigest string
)

// engine is the digest of the running binary, so a rebuilt Corvint never reads
// a snapshot an older extractor wrote. Empty when the executable cannot be
// read, which disables snapshots rather than risking a stale table.
func engine() string {
	engineOnce.Do(func() {
		engineDigest = digestExecutable()
	})
	return engineDigest
}

func digestExecutable() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	file, err := os.Open(executable)
	if err != nil {
		return ""
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return ""
	}
	return hex.EncodeToString(digest.Sum(nil))[:16]
}

// LoadedEngineID is the engine id loadSnapshot computes to name the snapshot
// file it opens: a digest of the running binary, taken with no child process
// (SBQ-V0-010(f)). It is the load-path id, not SnapshotReceipt.Engine, which is
// write-side only, and it spawns nothing, unlike ProbeSnapshot's Git identity
// read. Empty when the executable cannot be read, which disables snapshots.
func LoadedEngineID() string { return engine() }

// SnapshotDirectory is where a repository's snapshots live: `corvint/index`
// under the Git common directory, shared by every linked worktree, or the
// worktree's own `.corvint/index` when the common directory cannot be resolved
// from `.git` metadata without a Git process (DIRTY-CACHE-013).
func SnapshotDirectory(root string) string {
	return locateSnapshotStore(root).directory
}

// snapshotStore is one resolution of a root's store. An operation resolves it
// once and passes it down, so its symlink checks and its opens name the same
// directory. base anchors the store's no-follow walks: the Git common
// directory, or root on the fallback, where shared is false.
type snapshotStore struct {
	base, directory string
	shared          bool
}

func locateSnapshotStore(root string) snapshotStore {
	common, err := gitstatus.CommonDirectory(root)
	if err != nil {
		return snapshotStore{base: root, directory: filepath.Join(root, filepath.FromSlash(snapshotSubpath))}
	}
	return snapshotStore{base: common, directory: filepath.Join(common, filepath.FromSlash(sharedSnapshotSubpath)), shared: true}
}

// bound is the store's entry bound (DIRTY-CACHE-007). A shared store serves
// every linked worktree, so it keeps snapshotKeep per worktree -- the main one
// plus each entry under `<common>/worktrees/` -- up to snapshotKeepCap; eight
// in all would let worktrees on different trees evict each other's snapshot on
// every write. The per-worktree fallback keeps snapshotKeep (DIRTY-CACHE-013).
func (store snapshotStore) bound() int {
	if !store.shared {
		return snapshotKeep
	}
	worktrees, _ := os.ReadDir(filepath.Join(store.base, "worktrees"))
	return min(snapshotKeep*(1+len(worktrees)), snapshotKeepCap)
}

// refuseLinkedSnapshotDirectory rejects a worktree `.corvint`, or either
// component of the snapshot directory, that exists as anything but a real
// directory. A repository can commit `.corvint` as a symlink, and following it,
// or a linked store component, would write and evict outside the store
// (IDX-SNAP-V0-005). A component that does not exist yet is fine.
func refuseLinkedSnapshotDirectory(root, directory string) error {
	for _, component := range []string{filepath.Join(root, ".corvint"), filepath.Dir(directory), directory} {
		info, err := os.Lstat(component)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return &Error{Message: "index snapshot refused: " + component + " must be a real directory, not a symlink or file"}
		}
	}
	return nil
}

func snapshotPath(directory, objectFormat, tree, engineID string) string {
	return filepath.Join(directory, objectFormat+"-"+tree+"-"+engineID+".gob")
}

// firstError reports the encode failure over the close failure when both
// occur, since the encode error explains the close error's likely cause
// (e.g. a write that failed already flushed nothing on Close). A close
// failure alone still refuses: a buffer that never reached disk must not be
// renamed into place as if it were complete.
func firstError(encodeErr, closeErr error) error {
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}

// syncAndClose flushes a writer's temporary file to stable storage, then
// closes it, so the rename that follows cannot publish a name whose blocks
// were never written. After a crash the gob decoder would accept a zeroed
// range inside a source body as that body (decision 0210); a sync failure
// refuses the write as a close failure does.
func syncAndClose(file *os.File) error {
	return firstError(file.Sync(), file.Close())
}

// WriteSnapshot persists index for its tree, atomically, and keeps the
// directory to the store's entry bound (DIRTY-CACHE-007, snapshotStore.bound).
// Writers from several worktrees take no lock: each publishes a complete,
// synced file by rename onto the same key, the last rename wins, and a reader
// holds whichever complete file it opened (DIRTY-CACHE-013).
func WriteSnapshot(index *Index) (SnapshotReceipt, error) {
	engineID := engine()
	if engineID == "" {
		return SnapshotReceipt{}, &Error{Message: "index snapshot disabled: the running binary cannot be digested"}
	}
	store := locateSnapshotStore(index.Root)
	directory := store.directory
	corvintDirectory := filepath.Join(index.Root, ".corvint")
	if err := refuseLinkedSnapshotDirectory(index.Root, directory); err != nil {
		return SnapshotReceipt{}, err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return SnapshotReceipt{}, err
	}
	if err := os.MkdirAll(corvintDirectory, 0o755); err != nil {
		return SnapshotReceipt{}, err
	}
	if err := refuseLinkedSnapshotDirectory(index.Root, directory); err != nil {
		return SnapshotReceipt{}, err
	}
	if err := writeCorvintIgnore(corvintDirectory); err != nil {
		return SnapshotReceipt{}, err
	}
	// The worktree fallback store ignores itself, so a repository with no rule
	// for it stays clean in `git status` and a snapshot never becomes a dirty
	// path. Git never tracks the shared store under the common directory, so
	// it gets no ignore file.
	if !store.shared {
		if err := writeSnapshotGitIgnore(directory); err != nil {
			return SnapshotReceipt{}, err
		}
	}
	target := snapshotPath(directory, index.ObjectFormat, index.Revision, engineID)
	temporary, err := os.CreateTemp(directory, "snapshot-*.tmp")
	if err != nil {
		return SnapshotReceipt{}, err
	}
	written, encodeErr := encodeSnapshot(temporary, index, engineID)
	if err := firstError(encodeErr, syncAndClose(temporary)); err != nil {
		os.Remove(temporary.Name())
		return SnapshotReceipt{}, err
	}
	if err := os.Rename(temporary.Name(), target); err != nil {
		os.Remove(temporary.Name())
		return SnapshotReceipt{}, err
	}
	receipt := SnapshotReceipt{
		Path: target, Bytes: written, Tree: index.Revision, Commit: index.CommitRevision, Engine: engineID,
		Sources: len(index.Sources), Symbols: len(index.Symbols),
	}
	if sectionedEnabled() {
		sectionedTarget := sectionedPath(directory, index.ObjectFormat, index.Revision, engineID)
		sectionedBytes, err := writeSectionedSnapshot(directory, sectionedTarget, index, engineID)
		if err != nil {
			return SnapshotReceipt{}, err
		}
		receipt.SectionedPath, receipt.SectionedBytes = sectionedTarget, sectionedBytes
	}
	if packEnabled() {
		packEngineID := analyzerEngine()
		packTarget := packPath(directory, index.ObjectFormat, index.Revision, packEngineID)
		packBytes, err := writePackSnapshot(directory, packTarget, index, packEngineID)
		if err != nil {
			return SnapshotReceipt{}, err
		}
		receipt.PackPath, receipt.PackBytes = packTarget, packBytes
	}
	if blobShardsEnabled() {
		if err := writeBlobShards(index, store, analyzerEngine()); err != nil {
			return SnapshotReceipt{}, err
		}
	}
	bound := store.bound()
	receipt.Evicted = evictSnapshots(directory, target, bound)
	if packEnabled() {
		receipt.Evicted += evictAnalyzerPacks(directory, receipt.PackPath, bound)
	}
	return receipt, nil
}

func writeSectionedSnapshot(directory, target string, index *Index, engineID string) (int64, error) {
	temporary, err := os.CreateTemp(directory, "snapshot-*.tmp")
	if err != nil {
		return 0, err
	}
	written, encodeErr := encodeSectionedSnapshot(temporary, index, engineID)
	if err := firstError(encodeErr, syncAndClose(temporary)); err != nil {
		os.Remove(temporary.Name())
		return 0, err
	}
	if err := os.Rename(temporary.Name(), target); err != nil {
		os.Remove(temporary.Name())
		return 0, err
	}
	return written, nil
}

// writePackSnapshot publishes the pack beside the gob file the same way:
// temporary file, then rename. The co-change history it carries is read
// here, on the write path, so no read verb spawns for it.
func writePackSnapshot(directory, target string, index *Index, engineID string) (int64, error) {
	history, historyCommit := packHistoryForWrite(context.Background(), index)
	temporary, err := os.CreateTemp(directory, "snapshot-*.tmp")
	if err != nil {
		return 0, err
	}
	written, encodeErr := encodePackSnapshot(temporary, index, engineID, history, historyCommit)
	if err := firstError(encodeErr, syncAndClose(temporary)); err != nil {
		os.Remove(temporary.Name())
		return 0, err
	}
	if err := os.Rename(temporary.Name(), target); err != nil {
		os.Remove(temporary.Name())
		return 0, err
	}
	return written, nil
}

func writeCorvintIgnore(corvintDirectory string) error {
	ignorePath := filepath.Join(corvintDirectory, ".gitignore")
	file, err := os.OpenFile(ignorePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := file.WriteString(corvintIgnore); err != nil {
		file.Close()
		os.Remove(ignorePath)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(ignorePath)
		return err
	}
	return nil
}

func writeSnapshotGitIgnore(directory string) error {
	path := filepath.Join(directory, ".gitignore")
	content := []byte("*\n")
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == string(content) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".gitignore-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func encodeSnapshot(file *os.File, index *Index, engineID string) (int64, error) {
	portable := *index
	portable.Root, portable.DirtyPaths, portable.StatusSHA256 = "", nil, ""
	digest := sha256.New()
	encoder := gob.NewEncoder(io.MultiWriter(file, digest))
	if err := encoder.Encode(snapshotHeader{Format: snapshotFormat, ObjectFormat: index.ObjectFormat, Tree: index.Revision, Engine: engineID}); err != nil {
		return 0, err
	}
	if err := encoder.Encode(&portable); err != nil {
		return 0, err
	}
	if _, err := file.Write(digest.Sum(nil)); err != nil {
		return 0, err
	}
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// evictSnapshots removes stale writer temporaries and the published files
// beyond bound or snapshotStoreBytes, other engines' first and then the
// oldest, never the snapshot just written.
func evictSnapshots(directory, keep string, bound int) int {
	return evictSnapshotsAt(directory, keep, bound, time.Now())
}

func evictSnapshotsAt(directory, keep string, bound int, now time.Time) int {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0
	}
	type aged struct {
		path    string
		when    int64
		bytes   int64
		current bool
	}
	currentEngine := snapshotEngineOf(keep)
	files := make([]aged, 0, len(entries))
	staleTemporaryCutoff := now.Add(-snapshotTemporaryStaleAfter)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		temporary, _ := filepath.Match("snapshot-*.tmp", entry.Name())
		if temporary && !info.ModTime().After(staleTemporaryCutoff) {
			_ = os.Remove(path)
			continue
		}
		if filepath.Ext(entry.Name()) != ".gob" {
			continue
		}
		files = append(files, aged{path, info.ModTime().UnixNano(), info.Size(), snapshotEngineOf(path) == currentEngine})
	}
	// Snapshots the writing binary can read come first, newest first; one
	// another binary wrote is evicted before an older readable one.
	sort.Slice(files, func(left, right int) bool {
		if files[left].current != files[right].current {
			return files[left].current
		}
		return files[left].when > files[right].when
	})
	evicted := 0
	var keptBytes int64
	for position, file := range files {
		if file.path == keep || position < bound && keptBytes+file.bytes <= snapshotStoreBytes {
			keptBytes += file.bytes
			continue
		}
		if os.Remove(file.path) == nil {
			evicted++
		}
		_ = os.Remove(strings.TrimSuffix(file.path, ".gob") + sectionedExtension)
		// No current writer uses this name: packs are keyed by the analyzer
		// engine (evictAnalyzerPacks). It removes executable-keyed packs that
		// binaries before decision 0074 wrote.
		_ = os.Remove(strings.TrimSuffix(file.path, ".gob") + packExtension)
	}
	return evicted
}

// snapshotEngineOf is the engine segment of a snapshotPath name.
func snapshotEngineOf(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), ".gob")
	return name[strings.LastIndexByte(name, '-')+1:]
}

// LoadSnapshot returns the snapshot of the repository's current tree with the
// worktree's dirty paths applied, or ok=false when no snapshot matches. It
// reads identity and status as a build's opening observation does, closes the
// hit with one identity re-read after status, and writes nothing.
func LoadSnapshot(ctx context.Context, root string) (*Index, bool, error) {
	index, hit, _, err := loadSnapshot(ctx, root, loadFull)
	return index, hit, err
}

// LoadQuerySnapshot is LoadSnapshot for the repository query path: the same
// read and the same hit, plus the loader's paired observation on a hit so the
// shared query bracket (CORVINT_QUERY_SHARED_OBSERVATION=1) can open on it. The
// observation is nil on a miss that spawned nothing or straddled two revisions.
func LoadQuerySnapshot(ctx context.Context, root string) (*Index, bool, *LoaderObservation, error) {
	return loadSnapshot(ctx, root, loadFull)
}

// LoadSharedQuerySnapshot is LoadQuerySnapshot for a caller whose shared query
// bracket (CORVINT_QUERY_SHARED_OBSERVATION=1, proposed GPK-V0-065) closes on a
// full observation and compares it to the index. That closing is the identity
// re-read this loader otherwise makes after the decode, so a hit is returned
// without it, as LoadEventSnapshotObserved does for the harness window
// (GPK-V0-058); an identity that moves between the opening pair and the decode
// is refused at the bracket's closing instead of read as a miss here. The context
// also lets the closing status scan reuse the opening's metadata probes.
func LoadSharedQuerySnapshot(ctx context.Context, root string) (*Index, bool, *LoaderObservation, error) {
	return loadSnapshot(WithSharedQueryObservation(ctx), root, loadFull, false)
}

// WithSharedQueryObservation is the context of one shared query bracket: the
// isolated status scans it spawns answer their metadata probes from the first
// one's over unchanged bytes (gitstatus.WithProbeReuse).
func WithSharedQueryObservation(ctx context.Context) context.Context {
	return gitstatus.WithProbeReuse(ctx)
}

// ProbeSnapshot reads only the current snapshot's header. It reports a miss
// for a missing, corrupt, or mismatched file and never writes.
func ProbeSnapshot(ctx context.Context, root string) (SnapshotProbe, bool, error) {
	var engineID string
	engineReady := make(chan struct{})
	go func() {
		defer close(engineReady)
		engineID = engine()
	}()
	identity, err := readIdentity(ctx, root)
	<-engineReady
	if err != nil {
		return SnapshotProbe{}, false, err
	}
	if engineID == "" {
		return SnapshotProbe{}, false, nil
	}
	directory, present := snapshotDirectoryPresent(root)
	if !present {
		return SnapshotProbe{}, false, nil
	}
	if packEnabled() {
		return probeAnalyzerPack(directory, identity)
	}
	path := snapshotPath(directory, identity.objectFormat, identity.treeRevision, engineID)
	file, err := os.Open(path)
	if err != nil {
		return SnapshotProbe{}, false, nil
	}
	defer file.Close()
	// A header alone would call a torn body fresh, and `index --if-stale`
	// would then never repair the file every loader misses on.
	if _, err := decodeCompactEventSnapshot(file, identity, engineID); err != nil {
		return SnapshotProbe{}, false, nil
	}
	if sectionedEnabled() {
		if _, err := readSectionedSnapshot(sectionedPath(directory, identity.objectFormat, identity.treeRevision, engineID), identity, engineID, loadCompact); err != nil {
			return SnapshotProbe{}, false, nil
		}
	}
	return SnapshotProbe{
		Path: path, Tree: identity.treeRevision, Commit: identity.commitRevision, Engine: engineID,
	}, true, nil
}

// LoadEventSnapshot reads only the tables used by file-change and compact
// session-start. A clean compact event needs only the profile identifier; a
// dirty one reopens the immutable file for the impact tables it must rehydrate.
func LoadEventSnapshot(ctx context.Context, root string, compact bool) (*Index, bool, error) {
	load := loadEvent
	if compact {
		load = loadCompact
	}
	index, hit, _, err := loadSnapshot(ctx, root, load)
	return index, hit, err
}

// snapshotEngineID names the running binary for the load path. It is a
// variable only so a test can hold the engine goroutine open and prove the
// load joins it before snapshotPath is taken; production always binds engine.
var snapshotEngineID = engine

// loadSnapshot is the one read body behind LoadSnapshot, LoadEventSnapshot and
// LoadContextSnapshot. A miss carries the loader's paired observation when the
// pair still describes the tree, so a build that follows can open its stability
// window on it instead of spawning the pair again; the observation is nil when
// the loader spawned nothing or when HEAD moved during the load. Callers that
// only read the snapshot drop it.
func loadSnapshot(ctx context.Context, root string, load snapshotLoad, closing ...bool) (*Index, bool, *LoaderObservation, error) {
	// A repository that has never run `index` has no directory, and asking Git
	// for the tree OID that names the file would cost that repository two
	// process spawns per read to learn nothing. The stat is the miss.
	directory, present := snapshotDirectoryPresent(root)
	if !present {
		return nil, false, nil, nil
	}
	var engineID string
	engineReady := make(chan struct{})
	go func() {
		defer close(engineReady)
		engineID = snapshotEngineID()
	}()
	var observation repositoryObservation
	statusReady := make(chan struct{})
	go func() {
		defer close(statusReady)
		observation.dirty, observation.statusSHA256, observation.statusErr = readStatusSnapshot(ctx, root)
	}()
	observation.identity, observation.identityErr = readIdentity(ctx, root)
	<-engineReady
	// No engine digest names no snapshot file, so the read is a miss with
	// nothing decoded; the observation still stands, and a build opened on it
	// reports whatever the pair carries.
	if engineID == "" {
		<-statusReady
		return nil, false, &LoaderObservation{observation}, nil
	}
	if observation.identityErr != nil {
		<-statusReady
		return nil, false, nil, observation.identityErr
	}
	// Identity names the immutable snapshot file. Decode it while status is
	// still scanning the worktree; neither result depends on the other. A
	// potential hit is returned only after status joins and identity is stable.
	identity := observation.identity
	compact := load.tables() == loadCompact
	index, err := readSnapshotIndex(directory, identity, engineID, load)
	<-statusReady
	if observation.statusErr != nil {
		return nil, false, nil, observation.statusErr
	}
	if err != nil {
		return nil, false, &LoaderObservation{observation}, nil
	}
	// closingRead is false only for a caller whose own bracket closes on a
	// full observation and compares it to the index (LoadSharedQuerySnapshot).
	if len(closing) == 0 || closing[0] {
		closingIdentity, identityErr := readIdentity(ctx, root)
		if identityErr != nil {
			return nil, false, nil, identityErr
		}
		if identity != closingIdentity {
			return nil, false, nil, nil
		}
	}
	if compact && len(observation.dirty) != 0 {
		index, err = readSnapshotIndex(directory, identity, engineID, loadEvent|load&loadDeferredBodies)
		if err != nil {
			return nil, false, nil, nil
		}
	}
	dirty := make(map[string]struct{}, len(observation.dirty))
	for _, item := range observation.dirty {
		dirty[item] = struct{}{}
	}
	index.Root, index.DirtyPaths, index.StatusSHA256 = root, keys(dirty), observation.statusSHA256
	// The file is keyed by tree, so a later commit with the same tree hits it;
	// the stored commit is the writer's, and a hit reports the live HEAD.
	index.CommitRevision = identity.commitRevision
	return index, true, &LoaderObservation{observation}, nil
}

// readSnapshotIndex decodes the tables load names. Under the pack or
// sectioned opt-in it reads that file first and falls back to the gob
// snapshot when the file is absent or refused, so the accepted format
// always answers.
func readSnapshotIndex(directory string, identity repositoryIdentity, engineID string, load snapshotLoad) (*Index, error) {
	if packEnabled() {
		packEngineID := analyzerEngine()
		index, err := readPackSnapshot(packPath(directory, identity.objectFormat, identity.treeRevision, packEngineID), identity, packEngineID, load)
		if err == nil {
			return index, nil
		}
	}
	load = load.tables()
	if sectionedEnabled() {
		index, err := readSectionedSnapshot(sectionedPath(directory, identity.objectFormat, identity.treeRevision, engineID), identity, engineID, load)
		if err == nil {
			return index, nil
		}
	}
	file, err := os.Open(snapshotPath(directory, identity.objectFormat, identity.treeRevision, engineID))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	switch load {
	case loadCompact:
		return decodeCompactEventSnapshot(file, identity, engineID)
	case loadEvent:
		return decodeEventSnapshot(file, identity, engineID)
	}
	return decodeSnapshot(file, identity, engineID)
}

func decodeSnapshot(file *os.File, identity repositoryIdentity, engineID string) (*Index, error) {
	index := &Index{}
	if err := decodeSnapshotValue(file, identity, engineID, index); err != nil {
		return nil, err
	}
	if err := index.checkSymbolWindows(); err != nil {
		return nil, err
	}
	if err := index.Vocabulary.check(); err != nil {
		return nil, err
	}
	return index, nil
}

func decodeEventSnapshot(file *os.File, identity repositoryIdentity, engineID string) (*Index, error) {
	portable := &eventSnapshot{}
	if err := decodeSnapshotValue(file, identity, engineID, portable); err != nil {
		return nil, err
	}
	return portable.index(), nil
}

func decodeCompactEventSnapshot(file *os.File, identity repositoryIdentity, engineID string) (*Index, error) {
	portable := &compactEventSnapshot{}
	if err := decodeSnapshotValue(file, identity, engineID, portable); err != nil {
		return nil, err
	}
	return &Index{ProfileID: portable.ProfileID}, nil
}

// LoadEventSnapshotObserved is LoadEventSnapshot for a caller whose own
// bracket already holds the opening observation and will take the closing one
// (CORVINT_HARNESS_SHARED_OBSERVATION=1, proposed GPK-V0-058). It spawns no Git
// process: the observation names the file and supplies the dirty set, and the
// caller's closing observation is the identity re-read this loader otherwise
// makes. The Observation is the one Observe returns: the identity that names
// the snapshot file and the status read whose sorted paths and digest a hit
// applies (IDX-SNAP-V0-010). Everything else -- the stat miss, the engine check, the header
// check, the compact and dirty decode choice -- is loadSnapshot's.
func LoadEventSnapshotObserved(root string, compact bool, observation Observation) (*Index, bool, error) {
	directory, present := snapshotDirectoryPresent(root)
	if !present {
		return nil, false, nil
	}
	engineID := engine()
	if engineID == "" {
		return nil, false, nil
	}
	identity := repositoryIdentity{objectFormat: observation.ObjectFormat, commitRevision: observation.CommitRevision, treeRevision: observation.Revision}
	decode := decodeEventSnapshot
	if compact && len(observation.DirtyPaths) == 0 {
		decode = decodeCompactEventSnapshot
	}
	file, err := os.Open(snapshotPath(directory, identity.objectFormat, identity.treeRevision, engineID))
	if err != nil {
		return nil, false, nil
	}
	index, err := decode(file, identity, engineID)
	file.Close()
	if err != nil {
		return nil, false, nil
	}
	dirty := make(map[string]struct{}, len(observation.DirtyPaths))
	for _, item := range observation.DirtyPaths {
		dirty[item] = struct{}{}
	}
	index.Root, index.DirtyPaths, index.StatusSHA256 = root, keys(dirty), observation.StatusSHA256
	return index, true, nil
}

// decodeSnapshotValue decodes the header and the index message into value
// and then checks the SHA-256 trailer over every byte before it. Gob cannot
// tell a body overwritten with the same number of bytes from the real one,
// so without the trailer such a file would load as a hit serving text its
// blob does not contain (IDX-SNAP-V0-003, decision 0398). Every loader and
// ProbeSnapshot reads the whole message anyway, so the check adds a hash of
// bytes already read, not another read of the file.
func decodeSnapshotValue(file *os.File, identity repositoryIdentity, engineID string, value any) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	payloadBytes := info.Size() - sha256.Size
	if payloadBytes <= 0 {
		return errors.New("snapshot shorter than its digest")
	}
	digest := sha256.New()
	payload := io.TeeReader(io.NewSectionReader(file, 0, payloadBytes), digest)
	decoder := gob.NewDecoder(payload)
	if err := decodeSnapshotHeader(decoder, identity, engineID); err != nil {
		return err
	}
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if _, err := io.Copy(io.Discard, payload); err != nil {
		return err
	}
	var trailer [sha256.Size]byte
	if _, err := file.ReadAt(trailer[:], payloadBytes); err != nil {
		return err
	}
	if !bytes.Equal(digest.Sum(nil), trailer[:]) {
		return errors.New("snapshot digest mismatch")
	}
	return nil
}

func decodeSnapshotHeader(decoder *gob.Decoder, identity repositoryIdentity, engineID string) error {
	var header snapshotHeader
	if err := decoder.Decode(&header); err != nil {
		return err
	}
	expected := snapshotHeader{Format: snapshotFormat, ObjectFormat: identity.objectFormat, Tree: identity.treeRevision, Engine: engineID}
	if header != expected {
		return errors.New("snapshot header mismatch")
	}
	return nil
}

// snapshotDirectoryPresent reports whether the snapshot directory exists as a
// real directory under a real parent, with no linked worktree `.corvint`.
// Readers refuse what the writer refuses: a symlink would otherwise serve a
// snapshot from outside the store, so a linked component is the IDX-SNAP-V0-003
// miss, as absence is. It returns the store directory it checked, which the
// reader then opens, so both use one resolution.
func snapshotDirectoryPresent(root string) (string, bool) {
	directory := locateSnapshotStore(root).directory
	if refuseLinkedSnapshotDirectory(root, directory) != nil {
		return "", false
	}
	_, err := os.Stat(directory)
	return directory, err == nil
}
