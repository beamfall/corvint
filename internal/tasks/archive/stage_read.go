package archive

import (
	"errors"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type archiveNative struct{ StateDir, PrimaryWorktree string }
type archiveEntry struct {
	Name string
	Info os.FileInfo
}
type archiveListing struct {
	DirectoryInfo os.FileInfo
	Entries       []archiveEntry
}

func (n archiveNative) path(p string) (string, error) {
	if !filepath.IsAbs(n.StateDir) || filepath.Clean(n.StateDir) != n.StateDir || !filepath.IsAbs(n.PrimaryWorktree) || filepath.Clean(n.PrimaryWorktree) != n.PrimaryWorktree {
		return "", wire.Errorf(wire.CodeMalformed, p, "native source requires clean absolute roots")
	}
	if p != "." && (!filepath.IsLocal(p) || filepath.ToSlash(filepath.Clean(p)) != p || strings.Contains(p, "\\")) {
		return "", wire.Errorf(wire.CodeMalformed, p, "unclean source path")
	}
	if p == "intent" {
		return filepath.Join(n.PrimaryWorktree, intent.Dir), nil
	}
	if strings.HasPrefix(p, "intent/") {
		return filepath.Join(n.PrimaryWorktree, intent.Dir, strings.TrimPrefix(p, "intent/")), nil
	}
	return filepath.Join(n.StateDir, p), nil
}

// archiveRead lives for exactly one export attempt. It retains every listed
// no-follow parent; file reads never reopen the original absolute parent.
type archiveRead struct {
	native   archiveNative
	roots    map[string]*os.Root
	absent   map[string]bool
	listed   map[string]os.FileInfo
	closeErr error
}

func newArchiveRead(n archiveNative) *archiveRead {
	return &archiveRead{n, map[string]*os.Root{}, map[string]bool{}, map[string]os.FileInfo{}, nil}
}

var afterReadNames func(string) // deterministic enumeration/metadata race witness
var closeReadRoot = (*os.Root).Close
var closeReadFile = (*os.File).Close

func (s *archiveRead) close() error {
	err := s.closeErr
	for p, r := range s.roots {
		err = errors.Join(err, closeReadRoot(r))
		delete(s.roots, p)
	}
	return err
}
func archiveMoved(p string) error {
	return wire.Errorf(wire.CodeSnapshotMoved, p, "previously observed identity disappeared or changed")
}
func (s *archiveRead) parent(p string) (*os.Root, error) {
	if r := s.roots[p]; r != nil {
		return r, nil
	}
	abs, e := s.native.path(p)
	if e != nil {
		return nil, e
	}
	var root *os.Root
	if p == "." || p == "intent" {
		root, e = safeopen.Root(abs)
	} else {
		parent, e2 := s.parent(filepath.ToSlash(filepath.Dir(p)))
		if e2 != nil {
			return nil, e2
		}
		root, e = safeopen.SubRoot(parent, filepath.Base(p))
	}
	if e != nil {
		if os.IsNotExist(e) && s.listed[p] != nil {
			return nil, archiveMoved(p)
		}
		return nil, e
	}
	s.roots[p] = root
	return root, nil
}
func (s *archiveRead) binding(p string, root *os.Root) (err error) {
	abs, e := s.native.path(p)
	if e != nil {
		return e
	}
	named, e := safeopen.Root(abs)
	if os.IsNotExist(e) {
		return archiveMoved(p)
	}
	if e != nil {
		return e
	}
	defer func() { err = s.closed(err, closeReadRoot(named)) }()
	a, e := named.Stat(".")
	if e != nil {
		return e
	}
	b, e := root.Stat(".")
	if e != nil {
		return e
	}
	if !os.SameFile(a, b) {
		return archiveMoved(p)
	}
	return nil
}
func (s *archiveRead) List(p string, max int) (out archiveListing, err error) {
	root, e := s.parent(p)
	if os.IsNotExist(e) {
		s.absent[p] = true
		return out, e
	}
	if e != nil {
		return out, e
	}
	if s.absent[p] {
		return out, archiveMoved(p)
	}
	if e = s.binding(p, root); e != nil {
		return out, e
	}
	dir, e := safeopen.InRoot(root, ".", os.O_RDONLY, 0, true)
	if e != nil {
		return out, e
	}
	defer func() { err = s.closed(err, closeReadFile(dir)) }()
	info, e := dir.Stat()
	if e != nil {
		return out, e
	}
	if old := s.listed[p]; old != nil && !os.SameFile(old, info) {
		return out, archiveMoved(p)
	}
	out.DirectoryInfo = info
	names, e := intent.ReadDirNames(dir, p, p+"/", max)
	if e != nil {
		return out, e
	}
	if afterReadNames != nil {
		afterReadNames(p)
	}
	for _, name := range names {
		info, e := root.Lstat(name)
		if os.IsNotExist(e) {
			return out, archiveMoved(p + "/" + name)
		}
		if e != nil {
			return out, e
		}
		out.Entries = append(out.Entries, archiveEntry{name, info})
		child := name
		if p != "." {
			child = p + "/" + name
		}
		s.listed[child] = info
	}
	return out, nil
}
func (s *archiveRead) Read(p string, max int) (raw []byte, err error) {
	if _, e := s.native.path(p); e != nil {
		return nil, e
	}
	root, e := s.parent(filepath.ToSlash(filepath.Dir(p)))
	if e != nil {
		return nil, e
	}
	f, e := safeopen.InRoot(root, filepath.Base(p), os.O_RDONLY, 0, false)
	if os.IsNotExist(e) && s.listed[p] != nil {
		return nil, archiveMoved(p)
	}
	if e != nil {
		return nil, e
	}
	defer func() { err = s.closed(err, closeReadFile(f)) }()
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, p, "not a regular file")
	}
	if old := s.listed[p]; old != nil && !os.SameFile(old, st) {
		return nil, archiveMoved(p)
	}
	if max < 0 || st.Size() > int64(max) {
		return nil, wire.Errorf(wire.CodeLimitExceeded, p, "file exceeds bound")
	}
	raw, e = io.ReadAll(io.LimitReader(f, int64(max)+1))
	if e != nil {
		return nil, e
	}
	if len(raw) > max {
		return nil, wire.Errorf(wire.CodeLimitExceeded, p, "consumed bytes exceed bound")
	}
	return raw, nil
}

// captureLayout accounts names before opening staging or payload bodies.
func (s *archiveRead) captureLayout(maxScan, maxFiles int) (files []sourceFile, sc *scan, tree intent.Tree, err error) {
	files, sc, err = stateLayoutRead(s.native.StateDir, maxScan, s)
	if err != nil {
		return
	}
	root := filepath.Join(s.native.PrimaryWorktree, intent.Dir)
	var plan []sourceFile
	var total int64
	var collect func(string, int) error
	collect = func(dir string, max int) error {
		listing, e := s.List(dir, max)
		if e != nil {
			return e
		}
		abs := root
		if dir == "intent/tickets" || dir == "intent/releases" {
			abs = filepath.Join(root, strings.TrimPrefix(dir, "intent/"))
		}
		sc.info[abs] = listing.DirectoryInfo
		for _, entry := range listing.Entries {
			p := dir + "/" + entry.Name
			sc.info[filepath.Join(abs, entry.Name)] = entry.Info
			if (p == "intent/tickets" || p == "intent/releases") && entry.Info.IsDir() {
				limit := wire.MaxTicketsPerQueue
				if p == "intent/releases" {
					limit = wire.MaxReleasesPerQueue
				}
				if e = collect(p, limit); e != nil {
					return e
				}
				continue
			}
			bound, ok := intent.BoundFor(strings.TrimPrefix(p, "intent/"))
			if !ok {
				return wire.Errorf(wire.CodeMalformed, p, "unexpected intent path")
			}
			if !entry.Info.Mode().IsRegular() || entry.Info.Size() == 0 {
				return wire.Errorf(wire.CodeMalformed, p, "intent must be regular nonempty bytes")
			}
			if entry.Info.Size() > int64(bound) {
				return wire.Errorf(wire.CodeLimitExceeded, p, "intent file bound")
			}
			total += entry.Info.Size()
			if total > wire.MaxIntentTreeBytes {
				return wire.Errorf(wire.CodeLimitExceeded, p, "intent tree bound")
			}
			plan = append(plan, sourceFile{path: p, max: bound, life: s, abs: filepath.Join(abs, entry.Name)})
		}
		return nil
	}
	if err = collect("intent", intent.MaxIntentRootEntries); err != nil {
		return
	}
	if len(files)+len(plan) > maxFiles {
		err = wire.Errorf(wire.CodeLimitExceeded, "archive", "payload file count")
		return
	}
	for _, f := range plan {
		var raw []byte
		raw, err = f.read()
		if err != nil {
			return
		}
		if len(raw) == 0 {
			err = wire.Errorf(wire.CodeMalformed, f.path, "empty intent")
			return
		}
		tree.TotalBytes += len(raw)
		if tree.TotalBytes > wire.MaxIntentTreeBytes {
			err = wire.Errorf(wire.CodeLimitExceeded, "intent", "tree grew beyond bound")
			return
		}
		tree.Files = append(tree.Files, intent.File{Path: strings.TrimPrefix(f.path, "intent/"), Sha256: wire.Sum(raw), Bytes: len(raw), Raw: raw})
		f.raw = raw
		files = append(files, f)
	}
	tree.Sha256 = intent.DigestOfFiles(tree.Files)
	stageFiles, stageErr, e := s.readStage(sc)
	err = e
	sc.stageErr = stageErr
	if err != nil {
		return
	}
	if sc.stageErr == nil {
		sc.stage, sc.stageErr = snapshot.ObserveStage(stageFiles)
	}
	for _, f := range stageFiles {
		sc.stageDigests = append(sc.stageDigests, stageDigest{f.Name, wire.Sum(f.Raw)})
	}
	sortSources(files)
	return
}

func (s *archiveRead) readStage(sc *scan) (files []snapshot.StageFile, validation error, err error) {
	exists := func(n string) bool { return sc.info[filepath.Join(s.native.StateDir, "staging", n)] != nil }
	read := func(n string, max int) error {
		raw, e := s.Read("staging/"+n, max)
		if e != nil {
			return e
		}
		files = append(files, snapshot.StageFile{Name: n, Raw: raw})
		return nil
	}
	var d *snapshot.StageDescriptor
	if exists("active.json") {
		if e := read("active.json", snapshot.MaxStageDescriptorBytes); e != nil {
			return nil, nil, e
		}
		d, validation = snapshot.DecodeStageDescriptor(files[0].Raw)
	}
	if exists("active.json.tmp") {
		cap := snapshot.MaxStageDescriptorBytes
		if d != nil {
			cap = len(files[0].Raw)
		}
		if e := read("active.json.tmp", cap); e != nil {
			return files, nil, e
		}
	}
	if validation != nil {
		return files, validation, nil
	}
	assigned := map[string]bool{}
	if d != nil {
		for _, a := range d.Artifacts {
			assigned[a.Slot] = true
			if exists(a.Slot) {
				if e := read(a.Slot, int(a.Bytes.Uint64())); e != nil {
					return files, nil, e
				}
			}
		}
	}
	for i := 0; i < 11; i++ {
		n := "a" + string(wire.SizeOf(uint64(i)))
		if i < 10 {
			n = "a0" + string(wire.SizeOf(uint64(i)))
		}
		if exists(n) && !assigned[n] {
			return files, wire.Errorf(wire.CodeMalformed, n, "unassigned stage slot"), nil
		}
	}
	return files, nil, nil
}

func (s *archiveRead) validateStage(sc *scan, snap *snapshot.Snapshot, tree intent.Tree) error {
	if sc.stageErr != nil {
		return sc.stageErr
	}
	stage := sc.stage
	var e error
	if len(stage.Files) == 0 {
		return nil
	}
	b := snapshot.StageBinding{QueueID: snap.Head.QueueID.Raw, HeadRaw: snap.HeadRaw}
	for _, f := range tree.Files {
		if f.Path == intent.QueueFile {
			b.QueueRaw = f.Raw
		}
	}
	if d := stage.Descriptor; d != nil && (d.Base == nil || d.Base.LastSeq != snap.Head.LastSeq) {
		name, _ := snapshot.ReceiptName(snap.Head.LastSeq.Uint64())
		b.ReceiptRaw, e = s.Read("receipts/"+name, wire.MaxReceiptFileBytes)
		if e != nil {
			return e
		}
		rc, e := snapshot.DecodeReceipt(b.ReceiptRaw)
		if e != nil {
			return e
		}
		path, _ := snapshot.RequestPath(d.RequestID)
		for _, p := range rc.Post {
			if p.Path != path {
				continue
			}
			if p.Record != nil {
				b.RequestRaw = wire.EncodeFile(*p.Record)
			} else if p.BlobSha256 != nil {
				bound, e := snapshot.PostBound(p.Path)
				if e != nil {
					return e
				}
				b.RequestRaw, e = s.Read("evidence/"+string(*p.BlobSha256), bound)
				if os.IsNotExist(e) {
					return wire.Errorf(wire.CodeJournalForked, path, "retained request blob absent")
				}
				if e != nil {
					return e
				}
				if wire.Sum(b.RequestRaw) != *p.BlobSha256 {
					return wire.Errorf(wire.CodeJournalForked, path, "request blob identity")
				}
			}
		}
	}
	return stage.Bind(b)
}

func sameLayout(a, b *scan) bool {
	if len(a.info) != len(b.info) || len(a.stageDigests) != len(b.stageDigests) {
		return false
	}
	for p, i := range a.info {
		j := b.info[p]
		if j == nil || !os.SameFile(i, j) || i.Mode() != j.Mode() || i.Size() != j.Size() || !i.ModTime().Equal(j.ModTime()) {
			return false
		}
	}
	for i, f := range a.stageDigests {
		if f != b.stageDigests[i] {
			return false
		}
	}
	return true
}

// One retry owner; the inner header probe is always one-shot.
func readArchive(rd snapshot.Reader, attempt func(*snapshot.Snapshot) error) (*snapshot.Snapshot, error) {
	rd.Retries = -1
	var snap *snapshot.Snapshot
	var err error
	for n := 0; n < 4; n++ {
		snap, err = rd.Read(attempt)
		if wire.CodeOf(err) != wire.CodeSnapshotMoved {
			return snap, err
		}
	}
	return snap, wire.Errorf(wire.CodeSnapshotMoved, "archive", "store moved during all four attempts")
}

func joinReadClose(primary, cleanup error) error {
	if cleanup == nil {
		return primary
	}
	return wire.Errorf(wire.CodeUnsupportedFilesystem, "read lifetime", "%v", errors.Join(primary, cleanup))
}
func (s *archiveRead) closed(primary, cleanup error) error {
	s.closeErr = errors.Join(s.closeErr, cleanup)
	return joinReadClose(primary, cleanup)
}

type stageDigest struct {
	Name string
	Sum  wire.Digest
}
