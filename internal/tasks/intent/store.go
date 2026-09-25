package intent

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Layout of the intent store (§3.1).
const (
	Dir           = ".taskman"
	QueueFile     = "queue.json"
	PolicyFile    = "policy.json"
	ImportMapFile = "import-map.json"
	TicketsDir    = "tickets"
	ReleasesDir   = "releases"
)

// Directory enumeration bounds (§3.1 "Digest preimages", §3.5).
const (
	// DirChunk is the number of names materialized per Readdirnames call
	// while a store directory is enumerated: a flooded directory never
	// allocates more than the bound plus one chunk of names before the
	// count bound fires.
	DirChunk = 256
	// MaxIntentRootEntries is the number of entries the flat, closed
	// intent store root can hold: queue.json, policy.json,
	// import-map.json and tickets/. A fifth entry, whatever its name,
	// exceeds the bound.
	MaxIntentRootEntries = 5
)

// ReadDirNames enumerates an open directory in chunks of DirChunk names and
// fails LIMIT_EXCEEDED as soon as more than max entries have been seen:
// before the listing completes, before the names are sorted or validated,
// and before any entry is stat'ed or opened. Every entry counts, whatever
// its name, so skipped temp files and unexpected names are bounded too.
// Names are returned sorted. It reads nothing but the listing; the caller
// owns and closes d.
func ReadDirNames(d *os.File, full, label string, max int) ([]string, error) {
	var names []string
	for {
		chunk, err := d.Readdirnames(DirChunk)
		names = append(names, chunk...)
		if len(names) > max {
			return nil, wire.Errorf(wire.CodeLimitExceeded, full, "more than %d entries under %s (listing stopped after %d)", max, label, len(names))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, full, "cannot list: %v", err)
		}
	}
	sort.Strings(names)
	return names, nil
}

// File is one intent file: its store-relative path, digest, size and, when
// it was read from disk by TreeDigest, the exact bytes that were hashed. Raw
// is nil for a File rebuilt from an archive manifest.
type File struct {
	Path   string // e.g. "tickets/AT-07.json"
	Sha256 wire.Digest
	Bytes  int
	Raw    []byte
}

// Tree is the inventory of intent files with the intent tree digest.
type Tree struct {
	Files      []File // sorted by path
	TotalBytes int
	Sha256     wire.Digest
}

// Store is the loaded, validated intent store of the primary worktree.
type Store struct {
	Root      string // <primary>/.taskman
	Queue     *Queue
	Policy    *Policy
	ImportMap *ImportMap // nil when absent
	Tickets   []*ticket.Record
	Releases  []*release.Record
	Inventory *ticket.Inventory
	Tree      Tree
	// Digests of the individual files, keyed by store-relative path.
	Digests map[string]wire.Digest
}

// DigestOfFiles computes the intent tree digest frozen in SPEC §3.1 (R3):
// SHA-256 over `path || 0x00 || hex(sha256(bytes)) || 0x0A` for every file in
// byte-sorted path order. The input order does not matter.
func DigestOfFiles(files []File) wire.Digest {
	sorted := make([]File, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	h := sha256.New()
	for _, f := range sorted {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write([]byte(f.Sha256))
		h.Write([]byte{'\n'})
	}
	return wire.Digest(hex.EncodeToString(h.Sum(nil)))
}

// BoundFor returns the §1 byte bound of an admissible store-relative intent
// path and false for any path the flat, closed layout does not admit.
func BoundFor(path string) (int, bool) {
	switch path {
	case QueueFile:
		return wire.MaxQueueFileBytes, true
	case PolicyFile:
		return wire.MaxPolicyFileBytes, true
	case ImportMapFile:
		return wire.MaxImportMapBytes, true
	}
	if !strings.HasPrefix(path, TicketsDir+"/") {
		if !strings.HasPrefix(path, ReleasesDir+"/") {
			return 0, false
		}
		name := path[len(ReleasesDir)+1:]
		if _, ok := releaseName(name); !ok {
			return 0, false
		}
		return wire.MaxReleaseFileBytes, true
	}
	name := path[len(TicketsDir)+1:]
	if _, ok := ticketLocal(name); !ok {
		return 0, false
	}
	return wire.MaxTicketFileBytes, true
}

func releaseName(name string) (string, bool) {
	if !strings.HasSuffix(name, ".json") || strings.Contains(name, "/") {
		return "", false
	}
	id := strings.TrimSuffix(name, ".json")
	if _, err := wire.ParseLabel("", id); err != nil {
		return "", false
	}
	return id, true
}

// ticketLocal returns the local token of a `<local>.json` ticket file name.
func ticketLocal(name string) (string, bool) {
	if !strings.HasSuffix(name, ".json") || strings.Contains(name, "/") {
		return "", false
	}
	local := name[:len(name)-len(".json")]
	if _, err := wire.ParseToken("", local, wire.MaxLocalTokenBytes); err != nil {
		return "", false
	}
	return local, true
}

type plannedFile struct {
	path string
	max  int
}

// TreeDigest reads the intent store of a primary worktree through a
// root-confined descriptor (os.Root) and returns every file's bytes with the
// SPEC §3.1 tree digest. The flat, closed layout, the per-file §1 bounds, the
// ticket count bound and the 256 MiB tree bound are all enforced from
// directory listings and Lstat results before any byte is read or buffered;
// symlinks, nested directories, non-regular entries, empty files and unknown
// names fail closed. Records are not validated here; Load does that from the
// same captured bytes.
func TreeDigest(primaryWorktree string) (Tree, error) {
	rootPath := filepath.Join(primaryWorktree, Dir)
	fi, err := os.Lstat(rootPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Tree{}, wire.Errorf(wire.CodeUninitialized, rootPath, "no intent store (.taskman) in the primary worktree")
		}
		return Tree{}, wire.Errorf(wire.CodeUnsupportedFilesystem, rootPath, "cannot stat: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return Tree{}, wire.Errorf(wire.CodeUnsupportedFilesystem, rootPath, ".taskman is a symlink")
	}
	if !fi.IsDir() {
		return Tree{}, wire.Errorf(wire.CodeMalformed, rootPath, ".taskman is not a directory")
	}
	root, err := safeopen.Root(rootPath)
	if err != nil {
		return Tree{}, wire.Errorf(wire.CodeUnsupportedFilesystem, rootPath, "cannot open the intent store: %v", err)
	}
	defer root.Close()

	// Phase 1: list (bounded per directory), validate names, stat sizes;
	// nothing is read yet.
	var plan []plannedFile
	total := int64(0)
	names, err := listNames(root, rootPath, ".", Dir+"/", MaxIntentRootEntries)
	if err != nil {
		return Tree{}, err
	}
	for _, name := range names {
		full := filepath.Join(rootPath, name)
		switch name {
		case QueueFile, PolicyFile, ImportMapFile:
			max, _ := BoundFor(name)
			size, err := statRegular(root, full, name, max)
			if err != nil {
				return Tree{}, err
			}
			total += size
			plan = append(plan, plannedFile{name, max})
		case TicketsDir, ReleasesDir:
			info, err := root.Lstat(name)
			if err != nil {
				return Tree{}, wire.Errorf(wire.CodeUnsupportedFilesystem, full, "cannot stat: %v", err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return Tree{}, wire.Errorf(wire.CodeUnsupportedFilesystem, full, "symlink inside the intent store")
			}
			if !info.IsDir() {
				return Tree{}, wire.Errorf(wire.CodeMalformed, full, "%s is not a directory", name)
			}
			limit := wire.MaxTicketsPerQueue
			bound := wire.MaxTicketFileBytes
			if name == ReleasesDir {
				limit = wire.MaxReleasesPerQueue
				bound = wire.MaxReleaseFileBytes
			}
			tnames, err := listNames(root, rootPath, name, name+"/", limit)
			if err != nil {
				return Tree{}, err
			}
			for _, tn := range tnames {
				rel := name + "/" + tn
				tfull := filepath.Join(rootPath, name, tn)
				_, ok := ticketLocal(tn)
				if name == ReleasesDir {
					_, ok = releaseName(tn)
				}
				if !ok {
					return Tree{}, wire.Errorf(wire.CodeMalformed, tfull, "unexpected entry in the intent store (%s/ admits only named JSON records)", name)
				}
				size, err := statRegular(root, tfull, rel, bound)
				if err != nil {
					return Tree{}, err
				}
				total += size
				plan = append(plan, plannedFile{rel, bound})
				if total > int64(wire.MaxIntentTreeBytes) {
					return Tree{}, wire.Errorf(wire.CodeLimitExceeded, rootPath, "intent tree larger than %d bytes", wire.MaxIntentTreeBytes)
				}
			}
		default:
			return Tree{}, wire.Errorf(wire.CodeMalformed, full, "unexpected entry in the intent store (only queue.json, policy.json, import-map.json, tickets/ and releases/ are admitted)")
		}
		if total > int64(wire.MaxIntentTreeBytes) {
			return Tree{}, wire.Errorf(wire.CodeLimitExceeded, rootPath, "intent tree larger than %d bytes", wire.MaxIntentTreeBytes)
		}
	}

	// Phase 2: read exactly the planned entries through the root descriptor.
	files := make([]File, 0, len(plan))
	captured := 0
	for _, p := range plan {
		raw, err := readInRoot(root, filepath.Join(rootPath, filepath.FromSlash(p.path)), p.path, p.max)
		if err != nil {
			return Tree{}, err
		}
		captured += len(raw)
		if captured > wire.MaxIntentTreeBytes {
			return Tree{}, wire.Errorf(wire.CodeLimitExceeded, rootPath, "intent tree larger than %d bytes", wire.MaxIntentTreeBytes)
		}
		files = append(files, File{Path: p.path, Sha256: wire.Sum(raw), Bytes: len(raw), Raw: raw})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return Tree{Files: files, TotalBytes: captured, Sha256: DigestOfFiles(files)}, nil
}

// listNames lists a directory inside the store through the root-confined
// descriptor (`.` is the store root itself, so no unchecked path is ever
// opened) with that directory's entry bound, and returns its names sorted.
func listNames(root *os.Root, rootPath, sub, label string, max int) ([]string, error) {
	full := filepath.Join(rootPath, sub)
	d, err := safeopen.InRoot(root, sub, os.O_RDONLY, 0, true)
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, full, "cannot open directory: %v", err)
	}
	defer d.Close()
	st, err := d.Stat()
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, full, "cannot stat the opened directory: %v", err)
	}
	if !st.IsDir() {
		return nil, wire.Errorf(wire.CodeMalformed, full, "not a directory")
	}
	return ReadDirNames(d, full, label, max)
}

// statRegular proves, without opening it, that a store entry is a regular
// non-empty file within its bound, and returns its size.
func statRegular(root *os.Root, full, rel string, max int) (int64, error) {
	info, err := root.Lstat(rel)
	if err != nil {
		return 0, wire.Errorf(wire.CodeUnsupportedFilesystem, full, "cannot stat: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return 0, wire.Errorf(wire.CodeUnsupportedFilesystem, full, "symlink inside the intent store")
	}
	if !info.Mode().IsRegular() {
		return 0, wire.Errorf(wire.CodeMalformed, full, "not a regular file")
	}
	if info.Size() == 0 {
		return 0, wire.Errorf(wire.CodeMalformed, full, "empty file (a canonical document is never empty)")
	}
	if info.Size() > int64(max) {
		return 0, wire.Errorf(wire.CodeLimitExceeded, full, "file larger than %d bytes (%d)", max, info.Size())
	}
	return info.Size(), nil
}

// readInRoot opens a store entry through the root (so no path can escape
// the store, whatever races with the Lstat above), re-checks the opened
// descriptor, and reads it with a hard bound.
func readInRoot(root *os.Root, full, rel string, max int) ([]byte, error) {
	f, err := safeopen.InRoot(root, rel, os.O_RDONLY, 0, false)
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, full, "cannot open: %v", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, full, "cannot stat the opened file: %v", err)
	}
	if !st.Mode().IsRegular() {
		return nil, wire.Errorf(wire.CodeMalformed, full, "not a regular file")
	}
	if st.Size() > int64(max) {
		return nil, wire.Errorf(wire.CodeLimitExceeded, full, "file larger than %d bytes (%d)", max, st.Size())
	}
	raw, err := readAll(f, full, max, int(st.Size()))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, wire.Errorf(wire.CodeMalformed, full, "empty file (a canonical document is never empty)")
	}
	return raw, nil
}

// Load reads and validates the whole intent store of a primary worktree.
// It is a pure read: nothing is created, locked or modified. Every record is
// decoded from the same captured bytes that produced the tree digest.
func Load(primaryWorktree string) (*Store, error) {
	return LoadExpecting(primaryWorktree, "")
}

// LoadExpecting is Load pinned to a tree digest observed earlier under the
// TM-V0-008 protocol: when the captured tree's digest differs from want the
// load fails SNAPSHOT_MOVED before decoding, so a reader can never decode
// bytes from one snapshot under the digest of another. An empty want pins
// nothing.
func LoadExpecting(primaryWorktree string, want wire.Digest) (*Store, error) {
	tree, err := TreeDigest(primaryWorktree)
	if err != nil {
		return nil, err
	}
	if want != "" && tree.Sha256 != want {
		return nil, wire.Errorf(wire.CodeSnapshotMoved, filepath.Join(primaryWorktree, Dir), "intent tree digest %s differs from the pinned snapshot %s", tree.Sha256, want)
	}
	return decodeTree(filepath.Join(primaryWorktree, Dir), tree)
}

func decodeTree(root string, tree Tree) (*Store, error) {
	st := &Store{Root: root, Tree: tree, Digests: map[string]wire.Digest{}}
	byPath := map[string]File{}
	for _, f := range tree.Files {
		if _, ok := BoundFor(f.Path); !ok {
			return nil, wire.Errorf(wire.CodeMalformed, filepath.Join(root, f.Path), "unexpected entry in the intent store")
		}
		if f.Raw == nil || wire.Sum(f.Raw) != f.Sha256 {
			return nil, wire.Errorf(wire.CodeSnapshotMoved, filepath.Join(root, f.Path), "captured bytes do not match the tree digest")
		}
		byPath[f.Path] = f
		st.Digests[f.Path] = f.Sha256
	}
	if _, ok := byPath[QueueFile]; !ok {
		return nil, wire.Errorf(wire.CodeUninitialized, filepath.Join(root, QueueFile), "queue.json is absent")
	}
	if _, ok := byPath[PolicyFile]; !ok {
		return nil, wire.Errorf(wire.CodeUninitialized, filepath.Join(root, PolicyFile), "policy.json is absent")
	}
	q, err := DecodeQueue(byPath[QueueFile].Raw)
	if err != nil {
		return nil, prefix(err, QueueFile)
	}
	st.Queue = q
	p, err := DecodePolicy(byPath[PolicyFile].Raw)
	if err != nil {
		return nil, prefix(err, PolicyFile)
	}
	st.Policy = p
	if mf, ok := byPath[ImportMapFile]; ok {
		m, err := DecodeImportMap(mf.Raw)
		if err != nil {
			return nil, prefix(err, ImportMapFile)
		}
		if m.QueueID.Raw != q.QueueID.Raw {
			return nil, wire.Errorf(wire.CodeMalformed, ImportMapFile+"/queueId", "import map queue %s differs from queue.json %s", m.QueueID.Raw, q.QueueID.Raw)
		}
		if q.ImportMapSha256 == nil || *q.ImportMapSha256 != mf.Sha256 {
			return nil, wire.Errorf(wire.CodeMalformed, QueueFile+"/importMapSha256", "does not equal the chain digest of import-map.json")
		}
		st.ImportMap = m
	} else if q.ImportMapSha256 != nil {
		return nil, wire.Errorf(wire.CodeMalformed, QueueFile+"/importMapSha256", "names an import-map.json that is absent")
	}
	gateIDs := p.GateIDs()
	var records []*ticket.Record
	for _, f := range tree.Files {
		if !strings.HasPrefix(f.Path, TicketsDir+"/") {
			continue
		}
		rec, err := ticket.Decode(f.Raw)
		if err != nil {
			return nil, prefix(err, f.Path)
		}
		want := TicketsDir + "/" + rec.TicketID.Local + ".json"
		if f.Path != want {
			return nil, wire.Errorf(wire.CodeMalformed, f.Path, "file name does not match ticketId local token %q", rec.TicketID.Local)
		}
		if rec.TicketID.QueueID() != q.QueueID.Raw {
			return nil, wire.Errorf(wire.CodeMalformed, f.Path+"/ticketId", "ticket %s is outside queue %s", rec.TicketID.Raw, q.QueueID.Raw)
		}
		for i, g := range rec.RequiredGates {
			if !gateIDs[g] {
				return nil, wire.Errorf(wire.CodeGateUnknown, f.Path+"/requiredGates/"+string(wire.CountOf(int64(i))), "gate %q is not defined in policy", g)
			}
		}
		for i, d := range rec.Dependencies {
			if d.GateID != nil && !gateIDs[*d.GateID] {
				return nil, wire.Errorf(wire.CodeGateUnknown, f.Path+"/dependencies/"+string(wire.CountOf(int64(i)))+"/gateId", "gate %q is not defined in policy", *d.GateID)
			}
		}
		records = append(records, rec)
	}
	inv, err := ticket.NewInventory(q.QueueID, records)
	if err != nil {
		return nil, err
	}
	st.Tickets = records
	st.Inventory = inv
	for _, f := range tree.Files {
		if !strings.HasPrefix(f.Path, ReleasesDir+"/") {
			continue
		}
		rec, err := release.Decode(f.Raw)
		if err != nil {
			return nil, prefix(err, f.Path)
		}
		want := ReleasesDir + "/" + rec.ReleaseID + ".json"
		if f.Path != want || rec.QueueID != q.QueueID {
			return nil, wire.Errorf(wire.CodeMalformed, f.Path, "release path or queue scope mismatch")
		}
		for _, g := range rec.RequiredGates {
			if !gateIDs[g] {
				return nil, wire.Errorf(wire.CodeGateUnknown, f.Path+"/requiredGates", "gate %q is not defined in policy", g)
			}
		}
		st.Releases = append(st.Releases, rec)
	}
	return st, nil
}

// Context returns the eligibility context this store implies (§3.2) with
// the no-evidence oracles of this slice.
func (st *Store) Context() ticket.Context {
	return ticket.Context{CanonicalWriter: st.Queue.CanonicalWriter, SerialFallback: st.Policy.SerialFallback}
}

func prefix(err error, file string) error {
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
