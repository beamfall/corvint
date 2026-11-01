package affected

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// GraphCacheProfile names the graph cache entry format (AFP-V0-391). An entry
// written under any other profile is never trusted.
const GraphCacheProfile = "corvint-affected-graph-cache/0"

const (
	// graphCacheMaxBytes bounds one entry read or written.
	graphCacheMaxBytes = 256 << 20
)

// GraphCacheRacyWindow is how recently an observed path may have changed and
// still be pinned by its stamp: a change inside the window could share a
// timestamp with the bytes the build read, so no entry is written. Tests over
// freshly written fixtures shorten it.
var GraphCacheRacyWindow = 2 * time.Second

// CacheOutcome reports what BuildCached did with the cache. It never reaches a
// receipt: a plan is byte-identical whatever the outcome (AFP-V0-393).
type CacheOutcome struct {
	// Hit is true when the graph was restored from a verified entry.
	Hit bool
	// Reason names why an entry was not used, when Hit is false.
	Reason string
	// Stored is true when a fresh entry was written after a full build.
	Stored bool
	// NotStored names why no entry was written after a full build.
	NotStored string
}

// pathStamp is the stat identity of one observed path. Any write to a file
// sets its change time to the current time, so a pinned change time older than
// the build start proves the bytes the build read are the bytes still there.
type pathStamp struct {
	Present bool   `json:"present"`
	Mode    uint32 `json:"mode,omitzero"`
	Size    int64  `json:"size,omitzero"`
	Mtime   int64  `json:"mtime,omitzero"`
	Ctime   int64  `json:"ctime,omitzero"`
	Inode   uint64 `json:"inode,omitzero"`
	Device  uint64 `json:"device,omitzero"`
}

type observedPath struct {
	relative string
	follow   bool
}

// recording is what one disk observation read: every directory a source walk
// listed, and every path a plugin read or statted, each with the stamp taken
// just before the read. A directory's modification and change time move
// whenever an entry is added, removed, renamed or replaced in it, so its stamp
// pins the listing the walk saw.
type recording struct {
	mu          sync.Mutex
	paths       map[observedPath]pathStamp
	uncacheable string
}

func newRecording() *recording {
	return &recording{paths: map[observedPath]pathStamp{}}
}

func (r *recording) path(root, relative string, follow bool) {
	if r == nil || !fs.ValidPath(relative) {
		return
	}
	key := observedPath{relative: relative, follow: follow}
	r.mu.Lock()
	_, seen := r.paths[key]
	r.mu.Unlock()
	if seen {
		return
	}
	stamp, ok := stampPath(root, relative, follow)
	r.mu.Lock()
	defer r.mu.Unlock()
	if !ok && r.uncacheable == "" {
		r.uncacheable = "unstampable path " + relative
	}
	if _, seen := r.paths[key]; !seen {
		r.paths[key] = stamp
	}
}

// refuse marks the observation as one no entry may pin.
func (r *recording) refuse(reason string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.uncacheable == "" {
		r.uncacheable = reason
	}
}

// failed marks the observation uncacheable when a read of relative failed for
// a reason its stamp does not pin. An absent path, a path that is not a
// regular file, and a file over its bound are fixed by the stamp's presence,
// mode and size; any other failure (an I/O error, a permission or a transient
// fault) could clear while the stamp stays the same, and pinning it would
// reuse an observation with that file missing (AFP-V0-391, AFP-V0-392).
func (r *recording) failed(relative string, err error) {
	if r == nil || err == nil || errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrInvalidUnit) || errors.Is(err, ErrWalkLimit) {
		return
	}
	r.refuse("unreadable path " + relative)
}

// subtree records a plugin walk of a directory below root. Only an absent
// directory has a stamp; a present one is not pinned, so the entry is not
// written.
func (r *recording) subtree(root, relative string) {
	if r == nil {
		return
	}
	stamp, ok := stampPath(root, relative, false)
	r.mu.Lock()
	defer r.mu.Unlock()
	if (!ok || stamp.Present) && r.uncacheable == "" {
		r.uncacheable = "walked subtree " + relative
		return
	}
	r.paths[observedPath{relative: relative}] = stamp
}

// stampPath stats one repository-relative path; an absent path is a valid
// stamp, any other failure is not.
func stampPath(root, relative string, follow bool) (pathStamp, bool) {
	name := filepath.Join(root, filepath.FromSlash(relative))
	var info fs.FileInfo
	var err error
	if follow {
		info, err = os.Stat(name)
	} else {
		info, err = os.Lstat(name)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return pathStamp{}, true
	}
	if err != nil {
		return pathStamp{}, false
	}
	return statStamp(info)
}

// cachedLanguage is one plugin's observed result, as build admitted it.
type cachedLanguage struct {
	Name       string       `json:"name"`
	Units      []cachedUnit `json:"units"`
	Frontier   []string     `json:"frontier"`
	Unobserved []string     `json:"unobserved"`
	Runnable   []string     `json:"runnable"`
}

// cachedUnit is Unit with every member always written, and a nil list written
// as null, so a restored unit is the observed one exactly, nil lists included.
type cachedUnit struct {
	ID                   string   `json:"id"`
	Sources              []string `json:"sources"`
	Tests                []string `json:"tests"`
	Imports              []string `json:"imports"`
	TestImports          []string `json:"testImports"`
	Execs                []string `json:"execs"`
	AdvisoryImports      []string `json:"advisoryImports"`
	RouteDispatchImports []string `json:"routeDispatchImports"`
	PathTokens           []string `json:"pathTokens"`
	PathGlobs            []string `json:"pathGlobs"`
	PathTokensBounded    bool     `json:"pathTokensBounded"`
	PlacesFileNames      bool     `json:"placesFileNames"`
	Embeds               bool     `json:"embeds"`
	EmbedFiles           []string `json:"embedFiles"`
	UnboundedReads       string   `json:"unboundedReads"`
	LocatesRoot          bool     `json:"locatesRoot"`
	ReadScoped           bool     `json:"readScoped"`
	ReadScope            []string `json:"readScope"`
	Frontier             []string `json:"frontier"`
	FrontierBears        string   `json:"frontierBears"`
	Callers              []string `json:"callers"`
	ExecConsumers        []string `json:"execConsumers"`
}

type cachedStamp struct {
	Path   string    `json:"path"`
	Follow bool      `json:"follow,omitzero"`
	Stamp  pathStamp `json:"stamp"`
}

// cacheEntry is the body of one entry file (AFP-V0-391).
type cacheEntry struct {
	Profile       string           `json:"profile"`
	Root          string           `json:"root"`
	Identity      string           `json:"identity"`
	Languages     []cachedLanguage `json:"languages"`
	Documentation []string         `json:"documentation"`
	// RuntimeReaders is the AFP-V0-602 declaration the graph was built with.
	RuntimeReaders []RuntimeReader `json:"runtimeReaders,omitempty"`
	// RuntimeEdges is the AFP-V0-819 declaration the graph was built with.
	RuntimeEdges []RuntimeEdge `json:"runtimeEdges,omitempty"`
	// ExistenceReaders is the AFP-V0-980 declaration, with its manifest names.
	ExistenceReaders []ExistenceReader `json:"existenceReaders,omitempty"`
	Digest           string            `json:"digest"`
	Stamps           []cachedStamp     `json:"stamps"`
}

// GraphCacheEntryPath names the one entry for root and language set inside
// dir. The program identity is checked inside the entry, not named here, so
// a new corvint build replaces the entry rather than adding one beside it and
// the directory holds at most one entry per repository root.
func GraphCacheEntryPath(dir, root string, languages []Language) string {
	sum := sha256.Sum256([]byte(GraphCacheProfile + "\x00" + root + "\x00" + strings.Join(languageNames(languages), "\x00")))
	return filepath.Join(dir, "affected-graph-"+hex.EncodeToString(sum[:16])+".json")
}

func languageNames(languages []Language) []string {
	names := make([]string, 0, len(languages))
	for _, language := range languages {
		names = append(names, language.Name())
	}
	return names
}

// BuildCached is Build that first tries the verified graph cache entry in dir
// and, on any miss, builds in full and writes a fresh entry (AFP-V0-390 to
// AFP-V0-393). identity names the exact program doing the observation; an
// empty identity never hits and never stores. A cache failure is never an
// error: only the full build can fail.
func BuildCached(ctx context.Context, root, dir, identity string, languages ...Language) (*Graph, CacheOutcome, error) {
	return BuildCachedExcluding(ctx, root, dir, nil, identity, languages...)
}

// BuildCachedExcluding is BuildCached with further directories, the
// repository's Git and common directories, that the opened cache directory
// may not be at or under, as root may not (AFP-V0-390).
func BuildCachedExcluding(ctx context.Context, root, dir string, excluded []string, identity string, languages ...Language) (*Graph, CacheOutcome, error) {
	outcome := CacheOutcome{}
	entryPath := GraphCacheEntryPath(dir, root, languages)
	if identity == "" {
		outcome.Reason = "no program identity"
	} else if graph, reason := loadGraphCache(ctx, entryPath, root, excluded, identity, languages); graph != nil {
		graph.root = root
		return graph, CacheOutcome{Hit: true}, nil
	} else {
		outcome.Reason = reason
	}
	if err := ctx.Err(); err != nil {
		return nil, outcome, err
	}
	started := time.Now()
	reader := DiskSourceContext(ctx, root)
	reader.record = newRecording()
	graph, results, err := buildDisk(ctx, reader, languages, false)
	if err != nil {
		return nil, outcome, err
	}
	if identity == "" {
		outcome.NotStored = outcome.Reason
		return graph, outcome, nil
	}
	entry, reason := newCacheEntry(root, identity, languageNames(languages), graph, results, reader.record, started)
	if reason == "" {
		reason = writeGraphCache(entryPath, root, excluded, entry)
	}
	outcome.Stored, outcome.NotStored = reason == "", reason
	return graph, outcome, nil
}

func newCacheEntry(root, identity string, names []string, graph *Graph, results []Result, record *recording, started time.Time) (cacheEntry, string) {
	if len(results) != len(names) {
		return cacheEntry{}, "incomplete language results"
	}
	record.mu.Lock()
	defer record.mu.Unlock()
	if record.uncacheable != "" {
		return cacheEntry{}, record.uncacheable
	}
	entry := cacheEntry{Profile: GraphCacheProfile, Root: root, Identity: identity, Documentation: graph.documentation, RuntimeReaders: graph.runtimeReaders, RuntimeEdges: graph.runtimeEdges, ExistenceReaders: graph.existenceReaders, Digest: graph.digest}
	for index, result := range results {
		units := make([]cachedUnit, 0, len(result.Units))
		for _, unit := range result.Units {
			units = append(units, cachedUnit(unit))
		}
		entry.Languages = append(entry.Languages, cachedLanguage{Name: names[index], Units: units, Frontier: result.Frontier, Unobserved: result.Unobserved, Runnable: result.Runnable})
	}
	horizon := started.Add(-GraphCacheRacyWindow).UnixNano()
	for key, before := range record.paths {
		after, ok := stampPath(root, key.relative, key.follow)
		if !ok || after != before {
			return cacheEntry{}, "path changed during the build: " + key.relative
		}
		if after.Present && (after.Ctime >= horizon || after.Mtime >= horizon) {
			return cacheEntry{}, "path changed too recently to pin: " + key.relative
		}
		entry.Stamps = append(entry.Stamps, cachedStamp{Path: key.relative, Follow: key.follow, Stamp: after})
	}
	sort.Slice(entry.Stamps, func(i, j int) bool {
		if entry.Stamps[i].Path != entry.Stamps[j].Path {
			return entry.Stamps[i].Path < entry.Stamps[j].Path
		}
		return !entry.Stamps[i].Follow && entry.Stamps[j].Follow
	})
	return entry, ""
}

// writeGraphCache writes entry as one checksummed file, through a temporary
// file in the same directory renamed over the entry, so a reader sees the old
// entry or the new one, never a torn one. Both go through the one opened
// directory handle, so the pathname cannot be redirected between the
// placement check and the write (AFP-V0-390).
func writeGraphCache(entryPath, root string, excluded []string, entry cacheEntry) string {
	body, err := json.Marshal(entry, json.Deterministic(true), json.FormatNilSliceAsNull(true))
	if err != nil {
		return "encode: " + err.Error()
	}
	sum := sha256.Sum256(body)
	encoded := append([]byte(GraphCacheProfile+" "+hex.EncodeToString(sum[:])+"\n"), body...)
	if len(encoded) > graphCacheMaxBytes {
		return fmt.Sprintf("entry over the %d-byte bound", graphCacheMaxBytes)
	}
	directory, reason := openCacheDirectory(filepath.Dir(entryPath), root, excluded, true)
	if reason != "" {
		return reason
	}
	defer directory.Close()
	graphCacheHook("write")
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "cache write: " + err.Error()
	}
	name := ".affected-graph-" + hex.EncodeToString(nonce[:]) + ".tmp"
	temporary, err := directory.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "cache write: " + err.Error()
	}
	_, writeErr := temporary.Write(encoded)
	closeErr := temporary.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = directory.Remove(name)
		return "cache write: " + err.Error()
	}
	if err := directory.Rename(name, filepath.Base(entryPath)); err != nil {
		_ = directory.Remove(name)
		return "cache write: " + err.Error()
	}
	evictGraphCache(directory, filepath.Base(entryPath))
	return ""
}

// loadGraphCache returns the graph of a verified entry, or nil and the reason
// it is not trusted. Every check fails closed to a full build (AFP-V0-392).
func loadGraphCache(ctx context.Context, entryPath, root string, excluded []string, identity string, languages []Language) (*Graph, string) {
	directory, reason := openCacheDirectory(filepath.Dir(entryPath), root, excluded, false)
	if reason != "" {
		return nil, reason
	}
	defer directory.Close()
	graphCacheHook("entry")
	// Nonblocking, so a FIFO in place of the entry cannot hang the open; the
	// descriptor, not the pathname, is then held to a regular file.
	file, err := directory.OpenFile(filepath.Base(entryPath), os.O_RDONLY|openNonblocking, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "no entry"
	}
	if err != nil {
		return nil, "entry is not a readable regular file"
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, "entry is not a readable regular file"
	}
	if info.Size() > graphCacheMaxBytes {
		return nil, "entry over the size bound"
	}
	raw, err := io.ReadAll(io.LimitReader(file, graphCacheMaxBytes+1))
	if err != nil || len(raw) > graphCacheMaxBytes {
		return nil, "entry unreadable"
	}
	header, body, found := bytes.Cut(raw, []byte("\n"))
	profile, checksum, _ := strings.Cut(string(header), " ")
	sum := sha256.Sum256(body)
	if !found || profile != GraphCacheProfile || checksum != hex.EncodeToString(sum[:]) {
		return nil, "entry checksum or profile mismatch"
	}
	var entry cacheEntry
	if err := json.Unmarshal(body, &entry, json.RejectUnknownMembers(true), json.FormatNilSliceAsNull(true)); err != nil {
		return nil, "entry undecodable"
	}
	names := languageNames(languages)
	if entry.Profile != GraphCacheProfile || entry.Root != root || entry.Identity != identity || len(entry.Languages) != len(names) {
		return nil, "entry key mismatch"
	}
	if reason := completeStamps(entry); reason != "" {
		return nil, reason
	}
	if reason := verifyObservation(ctx, root, entry); reason != "" {
		return nil, reason
	}
	graph, err := restoreGraph(ctx, entry, languages)
	if err != nil {
		return nil, "entry graph invalid: " + err.Error()
	}
	if graph.digest != entry.Digest {
		return nil, "entry graph digest mismatch"
	}
	return graph, ""
}

// completeStamps refuses a stamp list that cannot be the one newCacheEntry
// wrote for this entry's graph: out of order or repeated, or missing the
// root listing, the documentation declaration, any unit's source or test
// file, or any directory above one. The checksum is not a signature, so
// these are the stamps a forged or stripped list can be held to without
// observing again: a stale graph whose stamps were removed or emptied would
// otherwise restat nothing and pass (AFP-V0-392).
func completeStamps(entry cacheEntry) string {
	const incomplete = "entry stamps incomplete"
	stamps := make(map[observedPath]pathStamp, len(entry.Stamps))
	for index, recorded := range entry.Stamps {
		if !fs.ValidPath(recorded.Path) {
			return incomplete
		}
		if index > 0 {
			previous := entry.Stamps[index-1]
			if previous.Path > recorded.Path || previous.Path == recorded.Path && (previous.Follow || !recorded.Follow) {
				return incomplete
			}
		}
		stamps[observedPath{relative: recorded.Path, follow: recorded.Follow}] = recorded.Stamp
	}
	if stamp, ok := stamps[observedPath{relative: "."}]; !ok || !stamp.Present || !fs.FileMode(stamp.Mode).IsDir() {
		return incomplete
	}
	for _, declaration := range []string{DocumentationPathsPath, RuntimeReadersPath, RuntimeEdgesPath, UnboundedReaderExclusionsPath, ExistenceReadersPath} {
		if _, ok := stamps[observedPath{relative: declaration}]; !ok {
			return incomplete
		}
	}
	directories := map[string]bool{}
	for _, language := range entry.Languages {
		for _, unit := range language.Units {
			for _, list := range [][]string{unit.Sources, unit.Tests} {
				for _, relative := range list {
					if stamp, ok := stamps[observedPath{relative: relative}]; !ok || !stamp.Present || !fs.FileMode(stamp.Mode).IsRegular() {
						return incomplete
					}
					for parent := path.Dir(relative); parent != "." && !directories[parent]; parent = path.Dir(parent) {
						if stamp, ok := stamps[observedPath{relative: parent}]; !ok || !stamp.Present || !fs.FileMode(stamp.Mode).IsDir() {
							return incomplete
						}
						directories[parent] = true
					}
				}
			}
		}
	}
	return ""
}

// verifyObservation restats every directory the build listed and every path
// it read, by a bounded pool: each stamp must be exactly the recorded one. The
// first failing stamp in entry order is the reason, so the outcome does not
// depend on scheduling.
func verifyObservation(ctx context.Context, root string, entry cacheEntry) string {
	reasons := make([]string, len(entry.Stamps))
	var pending sync.WaitGroup
	var next atomic.Int64
	for range min(verifyWorkers, len(entry.Stamps)) {
		pending.Go(func() {
			for index := int(next.Add(1)) - 1; index < len(entry.Stamps); index = int(next.Add(1)) - 1 {
				recorded := entry.Stamps[index]
				if err := ctx.Err(); err != nil {
					reasons[index] = err.Error()
					continue
				}
				if current, ok := stampPath(root, recorded.Path, recorded.Follow); !ok || current != recorded.Stamp {
					reasons[index] = "path changed: " + recorded.Path
				}
			}
		})
	}
	pending.Wait()
	for _, reason := range reasons {
		if reason != "" {
			return reason
		}
	}
	return ""
}

// verifyWorkers bounds the paths restatted at once.
const verifyWorkers = 8

// restoreGraph rebuilds the graph from the recorded plugin results through the
// same admission, reverse edges and digest a full build uses.
func restoreGraph(ctx context.Context, entry cacheEntry, languages []Language) (*Graph, error) {
	index := 0
	return build(ctx, languages, func() (declarations, error) {
		return declarations{documentation: entry.Documentation, runtimeReaders: entry.RuntimeReaders, runtimeEdges: entry.RuntimeEdges, existenceReaders: entry.ExistenceReaders}, nil
	}, func(language Language) (Result, error) {
		cached := entry.Languages[index]
		index++
		if cached.Name != language.Name() {
			return Result{}, fmt.Errorf("%w: cached language %q", ErrInvalidLanguage, cached.Name)
		}
		units := make([]Unit, 0, len(cached.Units))
		for _, unit := range cached.Units {
			units = append(units, Unit(unit))
		}
		if cached.Units == nil {
			units = nil
		}
		return Result{Units: units, Frontier: cached.Frontier, Unobserved: cached.Unobserved, Runnable: cached.Runnable}, nil
	})
}
