package archive

import (
	"os"

	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/evidencestore"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sourceFile is one file the export copies: its archive path, its absolute
// source path and the §1 bound that applies to it. raw, when non-nil, is the
// snapshot's captured copy of the bytes (intent files) and is used instead
// of re-reading the path.
type sourceFile struct {
	path string
	abs  string
	max  int
	raw  []byte
	life *archiveRead
}

// read returns the file's bytes: the captured copy when present, else a
// bounded read of the source path.
func (f sourceFile) read() ([]byte, error) {
	if f.raw != nil {
		return f.raw, nil
	}
	if f.life != nil {
		return f.life.Read(f.path, f.max)
	}
	return intent.ReadFile(f.abs, f.max)
}

// scan counts every directory entry the export enumerates against one
// cumulative scan bound (wire.MaxArchiveScanEntries in Export, §3.5). Each
// directory is listed in bounded chunks with the remaining budget as its
// per-directory bound, so a flood of entries anywhere under the state dir
// is refused during the listing, before any entry is stat'ed, opened or
// read. Directories, skipped temp entries and unexpected names count
// against this scan bound; the exported-file count is a separate budget
// (wire.MaxArchiveFiles) that Export checks once the layout is known.
type scan struct {
	seen         int
	max          int
	life         *archiveRead
	info         map[string]os.FileInfo
	stage        *snapshot.StageObservation
	stageDigests []stageDigest
	stageErr     error
	// segmented records a listed segments/ directory.
	segmented bool
}

// list opens a directory the caller has already Lstat'ed as a non-symlink
// directory (info), proves the opened descriptor is that same directory,
// and returns its names under the remaining budget.
func (s *scan) list(dir, label string, dirInfo os.FileInfo) ([]string, error) {
	rel, err := filepath.Rel(s.life.native.StateDir, dir)
	if err != nil {
		return nil, err
	}
	max := s.max - s.seen
	if rel == "staging" && max > snapshot.MaxStageChildren {
		max = snapshot.MaxStageChildren
	}
	listing, err := s.life.List(filepath.ToSlash(rel), max)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(dirInfo, listing.DirectoryInfo) {
		return nil, archiveMoved(dir)
	}
	s.info[dir] = listing.DirectoryInfo
	names := make([]string, 0, len(listing.Entries))
	for _, e := range listing.Entries {
		names = append(names, e.Name)
		s.info[filepath.Join(dir, e.Name)] = e.Info
	}
	s.seen += len(names)
	return names, nil
}

// stateLayout lists the state-dir content an archive carries (TM-V0-022):
// VERSION, head.json, barrier.json, receipts, the requests index, attempts,
// the reservation set, effect records with their bootstrap and
// acknowledgement files, pinned documents, evidence blobs and the witness
// receipt store (TOL-V0-139). Temp files
// (`receipts/*.tmp-*`, `head.json.tmp`, `effects/*.tmp-*`) are not store
// content and are skipped; `worktrees/` holds lane checkouts and is not
// archived; any other entry fails closed so history is never silently
// dropped. Every directory entry seen, directories and skipped ones
// included, counts against maxScan (wire.MaxArchiveScanEntries in Export);
// the count of entries scanned is returned beside the files. The number of
// files returned is bounded separately by the caller against
// wire.MaxArchiveFiles together with the intent tree.
func stateLayout(stateDir string, maxScan int) (outFiles []sourceFile, count int, retErr error) {
	life := newArchiveRead(archiveNative{StateDir: stateDir, PrimaryWorktree: filepath.Dir(stateDir)})
	defer func() { retErr = joinReadClose(retErr, life.close()) }()
	files, sc, err := stateLayoutRead(stateDir, maxScan, life)
	for i := range files {
		files[i].life = nil
	}
	return files, sc.seen, err
}

func stateLayoutRead(stateDir string, maxScan int, life *archiveRead) ([]sourceFile, *scan, error) {
	sc := &scan{max: maxScan, life: life, info: map[string]os.FileInfo{}}
	var out []sourceFile
	root, err := life.parent(".")
	if err != nil {
		return nil, sc, err
	}
	rootInfo, err := root.Stat(".")
	if err != nil {
		return nil, sc, wire.Errorf(wire.CodeUnsupportedFilesystem, stateDir, "cannot stat: %v", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return nil, sc, wire.Errorf(wire.CodeUnsupportedFilesystem, stateDir, "state dir is not a directory")
	}
	entries, err := sc.list(stateDir, "the state dir", rootInfo)
	if err != nil {
		return nil, sc, err
	}
	for _, name := range entries {
		abs := filepath.Join(stateDir, name)
		info := sc.info[abs]
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, sc, wire.Errorf(wire.CodeUnsupportedFilesystem, abs, "symlink inside the state dir")
		}
		switch {
		case name == "staging" && info.IsDir():
			names, err := sc.list(abs, "staging/", info)
			if err != nil {
				return nil, sc, err
			}
			for _, n := range names {
				if !snapshot.StageName(n) {
					return nil, sc, wire.Errorf(wire.CodeMalformed, n, "unexpected staging child")
				}
				if !sc.info[filepath.Join(abs, n)].Mode().IsRegular() {
					return nil, sc, wire.Errorf(wire.CodeUnsupportedFilesystem, n, "stage child is not regular")
				}
			}
		case name == "VERSION" && info.Mode().IsRegular():
			out = append(out, sourceFile{path: "VERSION", abs: abs, max: 64})
		case name == "head.json" && info.Mode().IsRegular():
			out = append(out, sourceFile{path: "head.json", abs: abs, max: wire.MaxJournalHeadBytes})
		case name == "barrier.json" && info.Mode().IsRegular():
			out = append(out, sourceFile{path: "barrier.json", abs: abs, max: wire.MaxBarrierBytes})
		case name == "programs.json" && info.Mode().IsRegular():
			out = append(out, sourceFile{path: "programs.json", abs: abs, max: 1 << 20})
		case name == "pools.json" && info.Mode().IsRegular():
			out = append(out, sourceFile{path: "pools.json", abs: abs, max: snapshot.MaxPoolStateBytes})
		case name == "reservations.json" && info.Mode().IsRegular():
			out = append(out, sourceFile{path: "reservations.json", abs: abs, max: wire.MaxReservationSetBytes})
		case name == "head.json.tmp" && info.Mode().IsRegular():
			// a crashed head rename (§5.2); recovery deletes it, reads ignore it
		case name == "worktrees" && info.IsDir():
			// lane worktrees are git checkouts, not archive content
		case name == "receipts" && info.IsDir():
			files, err := flatDir(sc, abs, info, "receipts", wire.MaxReceiptFileBytes, func(n string) bool {
				return strings.HasSuffix(n, ".json") && !strings.Contains(n, ".tmp-")
			}, func(n string) bool { return strings.Contains(n, ".tmp-") })
			if err != nil {
				return nil, sc, err
			}
			out = append(out, files...)
		case name == "attempts" && info.IsDir():
			files, err := flatDir(sc, abs, info, "attempts", wire.MaxAttemptRecordBytes, func(n string) bool { return strings.HasSuffix(n, ".json") }, nil)
			if err != nil {
				return nil, sc, err
			}
			out = append(out, files...)
		case name == "effects" && info.IsDir():
			files, err := flatDir(sc, abs, info, "effects", wire.MaxAttemptRecordBytes, func(n string) bool {
				return !strings.Contains(n, ".tmp-") && (strings.HasSuffix(n, ".json") || strings.HasSuffix(n, ".boot") || strings.HasSuffix(n, ".ack"))
			}, func(n string) bool { return strings.Contains(n, ".tmp-") })
			if err != nil {
				return nil, sc, err
			}
			out = append(out, files...)
		case name == "pinned" && info.IsDir():
			files, err := flatDir(sc, abs, info, "pinned", wire.MaxPinnedBytes, func(n string) bool { return validContentName("pinned/" + n) }, nil)
			if err != nil {
				return nil, sc, err
			}
			out = append(out, files...)
		case name == "evidence" && info.IsDir():
			files, err := flatDir(sc, abs, info, "evidence", wire.MaxEvidenceBlobBytes, func(n string) bool { return validContentName("evidence/" + n) }, nil)
			if err != nil {
				return nil, sc, err
			}
			out = append(out, files...)
		case name == evidencestore.Dir && info.IsDir():
			// The witness receipt store (TOL-V0-139): every stored report,
			// named by its digest; a put in progress is skipped.
			files, err := flatDir(sc, abs, info, evidencestore.Dir, evidencestore.MaxBlobBytes, func(n string) bool { return validContentName(evidencestore.Dir + "/" + n) }, func(n string) bool { return strings.HasPrefix(n, evidencestore.TempPrefix) })
			if err != nil {
				return nil, sc, err
			}
			out = append(out, files...)
		case name == snapshot.SegmentsDir && info.IsDir():
			// Sealed history (JSG-V0-012): Export serves its receipts and
			// requests by their logical paths through sealedSources; the
			// segment files themselves are not archive content.
			sealed, err := flatDir(sc, abs, info, snapshot.SegmentsDir, snapshot.MaxSegmentBytes, func(string) bool { return true }, nil)
			if err != nil {
				return nil, sc, err
			}
			sc.segmented = len(sealed) != 0
		case name == "requests" && info.IsDir():
			shards, err := sc.list(abs, "requests/", info)
			if err != nil {
				return nil, sc, err
			}
			for _, sh := range shards {
				shAbs := filepath.Join(abs, sh)
				shInfo := sc.info[shAbs]
				if shInfo.Mode()&os.ModeSymlink != 0 || !shInfo.IsDir() || len(sh) != 2 {
					return nil, sc, wire.Errorf(wire.CodeMalformed, shAbs, "requests/ must contain two-character shard directories")
				}
				files, err := flatDir(sc, shAbs, shInfo, "requests/"+sh, wire.MaxAttemptRecordBytes, func(n string) bool { return strings.HasSuffix(n, ".json") }, nil)
				if err != nil {
					return nil, sc, err
				}
				out = append(out, files...)
			}
		default:
			return nil, sc, wire.Errorf(wire.CodeMalformed, abs, "unexpected entry in the state dir; refusing to export an incomplete or unknown layout")
		}
	}
	for i := range out {
		out[i].life = life
	}
	return out, sc, nil
}

// flatDir lists one flat state-dir directory under the scan's cumulative
// bound; every name is Lstat'ed (never opened) and must be a regular file
// the accept rule admits, unless the skip rule names it as a temp entry.
func flatDir(sc *scan, dir string, dirInfo os.FileInfo, rel string, max int, accept func(string) bool, skip func(string) bool) ([]sourceFile, error) {
	entries, err := sc.list(dir, rel+"/", dirInfo)
	if err != nil {
		return nil, err
	}
	var out []sourceFile
	for _, name := range entries {
		abs := filepath.Join(dir, name)
		info := sc.info[abs]
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, abs, "symlink inside the state dir")
		}
		if skip != nil && skip(name) {
			continue
		}
		if !info.Mode().IsRegular() || !accept(name) {
			return nil, wire.Errorf(wire.CodeMalformed, abs, "unexpected entry in the state dir")
		}
		out = append(out, sourceFile{path: rel + "/" + name, abs: abs, max: max})
	}
	return out, nil
}

// boundFor returns the §1 byte bound that verify applies to an archive
// path.
func boundFor(path string) int {
	switch {
	case path == "VERSION":
		return 64
	case path == "head.json":
		return wire.MaxJournalHeadBytes
	case path == "barrier.json":
		return wire.MaxBarrierBytes
	case path == "pools.json":
		return snapshot.MaxPoolStateBytes
	case path == "programs.json":
		return 1 << 20
	case path == "reservations.json":
		return wire.MaxReservationSetBytes
	case strings.HasPrefix(path, "receipts/"):
		return wire.MaxReceiptFileBytes
	case strings.HasPrefix(path, "pinned/"):
		return wire.MaxPinnedBytes
	case strings.HasPrefix(path, "evidence/"):
		return wire.MaxEvidenceBlobBytes
	case strings.HasPrefix(path, evidencestore.Dir+"/"):
		return evidencestore.MaxBlobBytes
	case path == "intent/"+intent.QueueFile:
		return wire.MaxQueueFileBytes
	case path == "intent/"+intent.PolicyFile:
		return wire.MaxPolicyFileBytes
	case path == "intent/"+intent.ImportMapFile:
		return wire.MaxImportMapBytes
	case strings.HasPrefix(path, "intent/tickets/"):
		return wire.MaxTicketFileBytes
	}
	return wire.MaxAttemptRecordBytes
}

func sortSources(files []sourceFile) {
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
}

func contentName(p string) (string, bool) {
	if strings.HasPrefix(p, "evidence/") {
		return strings.TrimPrefix(p, "evidence/"), true
	}
	if strings.HasPrefix(p, evidencestore.Dir+"/") {
		return strings.TrimPrefix(p, evidencestore.Dir+"/"), true
	}
	if strings.HasPrefix(p, "pinned/") {
		name := strings.TrimPrefix(p, "pinned/")
		if !strings.HasSuffix(name, ".json") {
			return "", true
		}
		return strings.TrimSuffix(name, ".json"), true
	}
	return "", false
}
func validContentName(p string) bool {
	name, ok := contentName(p)
	if !ok {
		return true
	}
	_, err := wire.ParseDigest(p, name)
	return err == nil
}
func checkContentName(p string, digest wire.Digest) error {
	name, ok := contentName(p)
	if !ok {
		return nil
	}
	if !validContentName(p) || name != string(digest) {
		return wire.Errorf(wire.CodeMalformed, p, "content-addressed name differs from consumed-byte digest")
	}
	return nil
}

// sealedSources returns the archive's logical view of a segmented store
// (JSG-V0-012): the head rendered as taskman-journal-head/0 without its
// segment binding, and receipts 1..S with the request afterimages they
// posted, served from the segments the head binds once the manifest's and
// each segment's digest check. The archive is therefore the archive of the
// unsegmented store, and archive verify and import are unchanged. Sealed
// bytes are held in memory for the export attempt.
func sealedSources(life *archiveRead, head *snapshot.Head, headSha wire.Digest, files []sourceFile) ([]sourceFile, wire.Digest, error) {
	raw, err := life.Read("head.json", wire.MaxJournalHeadBytes)
	if err != nil {
		return nil, "", err
	}
	if wire.Sum(raw) != headSha {
		return nil, "", archiveMoved("head.json")
	}
	through := head.Segments.Through.Uint64()
	name := snapshot.SegmentManifestName(through)
	mraw, err := life.Read(name, snapshot.MaxSegmentManifestBytes)
	if err != nil {
		return nil, "", err
	}
	if wire.Sum(mraw) != head.Segments.ManifestSha256 {
		return nil, "", archiveMoved(name)
	}
	m, err := snapshot.DecodeSegmentManifest(mraw, 1)
	if err != nil {
		return nil, "", err
	}
	if m.Through != through || m.QueueID != head.QueueID {
		return nil, "", wire.Errorf(wire.CodeJournalForked, name, "segment manifest differs from the head")
	}
	seen := make(map[string]bool, len(files))
	for _, f := range files {
		if strings.HasPrefix(f.path, "receipts/") {
			if seq, ok := parseReceiptName(strings.TrimPrefix(f.path, "receipts/")); ok && seq <= through {
				return nil, "", wire.Errorf(wire.CodeJournalForked, f.path, "a sealed receipt is still linked")
			}
		}
		seen[f.path] = true
	}
	for _, e := range m.Segments {
		sname := snapshot.SegmentName(e.First, e.Last)
		sraw, err := life.Read(sname, snapshot.MaxSegmentBytes)
		if err != nil {
			return nil, "", err
		}
		if uint64(len(sraw)) != e.Bytes || wire.Sum(sraw) != e.Sha256 {
			return nil, "", wire.Errorf(wire.CodeJournalForked, sname, "segment differs from the digest the manifest binds")
		}
		seg, err := snapshot.DecodeSegment(sraw, e.First, e.Last)
		if err != nil {
			return nil, "", err
		}
		abs := filepath.Join(life.native.StateDir, filepath.FromSlash(sname))
		for _, group := range [][]snapshot.SegmentFile{seg.Receipts, seg.Requests} {
			for _, f := range group {
				if seen[f.Path] {
					return nil, "", wire.Errorf(wire.CodeJournalForked, f.Path, "sealed file is also linked or sealed twice")
				}
				seen[f.Path] = true
				files = append(files, sourceFile{path: f.Path, abs: abs, max: boundFor(f.Path), raw: f.Raw})
			}
		}
	}
	plain := *head
	plain.Segments = nil
	hraw := wire.EncodeFile(plain.Value())
	if _, err := snapshot.DecodeHead(hraw); err != nil {
		return nil, "", err
	}
	for i := range files {
		if files[i].path == "head.json" {
			files[i].raw = hraw
		}
	}
	sortSources(files)
	return files, wire.Sum(hraw), nil
}
