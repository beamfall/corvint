package archive

import (
	"archive/tar"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ExportOptions configures one export.
type ExportOptions struct {
	Repo    *intent.Repository
	Staging string    // "" selects the OS temp dir
	Stdout  io.Writer // receives the verified stream; delivery errors may leave a prefix
	// afterProbe is a test hook run inside the snapshot after the head and
	// slot checks and before any file is copied (AS-36 N4d).
	afterCapture  func()       // finite inventory/body movement seam
	afterBody     func() error // stable body-error/recheck seam
	afterProbe    func()
	afterStaged   func(*os.File) // test hook after writing, before verification
	afterVerified func()         // test hook before delivery
	streamLimit   uint64         // tests may lower the encoded-stream budget; never above the profile cap
}

// ExportResult describes a successful export.
type ExportResult struct {
	Manifest *Manifest
	Snapshot *snapshot.Snapshot
	Bytes    uint64 // sum of files-entry payload bytes; excludes manifest and tar framing
}

// SnapshotOnFailure is set by Export on the error path to the partial
// snapshot the failing probe observed (nil when none), so a CLI can still
// report the head it saw. It is a package-level result of the last call and
// is meant for the single-threaded command path only.
var SnapshotOnFailure *snapshot.Snapshot

func resolveReal(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", wire.Errorf(wire.CodeUnsupportedFilesystem, p, "cannot resolve: %v", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", wire.Errorf(wire.CodeUnsupportedFilesystem, p, "cannot resolve: %v", err)
	}
	return filepath.Clean(real), nil
}

func within(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

// checkStaging refuses a staging directory inside the state dir or the
// repository (after resolving symlinks on every side), or one whose own
// final component is a symlink (N4f). Ancestor symlinks such as macOS's
// `/var` → `/private/var` are resolved rather than refused, because the
// default OS temp dir lives behind one; containment is decided on the
// resolved paths.
func checkStaging(staging string, repo *intent.Repository) (*os.Root, error) {
	if staging == "" {
		staging = os.TempDir()
	}
	fi, err := os.Lstat(staging)
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, staging, "staging directory: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, staging, "staging directory is a symlink")
	}
	if !fi.IsDir() {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, staging, "staging path is not a directory")
	}
	real, err := resolveReal(staging)
	if err != nil {
		return nil, err
	}
	stateReal, err := resolveReal(repo.StateDir)
	if err != nil {
		stateReal = filepath.Clean(repo.StateDir)
	}
	repoReal, err := resolveReal(repo.PrimaryWorktree)
	if err != nil {
		return nil, err
	}
	if within(real, stateReal) || within(real, repoReal) {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, staging, "staging directory must be outside the state dir and the repository")
	}
	stagingRepo, err := intent.Resolve(real)
	if err != nil && wire.CodeOf(err) != wire.CodeUninitialized {
		return nil, err
	}
	if stagingRepo != nil {
		same, err := sameCommonDirectory(repo.CommonDir, stagingRepo.CommonDir)
		if err != nil {
			return nil, err
		}
		if same {
			return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, staging, "staging directory must be outside the repository, including linked worktrees")
		}
	}
	root, err := safeopen.Root(real)
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, staging, "cannot pin staging directory: %v", err)
	}
	pinned, err := root.Stat(".")
	if err != nil || !os.SameFile(fi, pinned) {
		return nil, stagingCleanup(wire.Errorf(wire.CodeUnsupportedFilesystem, staging, "staging directory changed during validation: %v", err), root.Close())
	}
	return root, nil
}

// Bound both component walks and retain both no-follow roots until their
// descriptor identities have been compared. Case aliases name the same object.
func sameCommonDirectory(source, candidate string) (same bool, retErr error) {
	var identities [2]os.FileInfo
	for i, path := range []string{source, candidate} {
		if _, err := wire.ParsePathText("common directory", path); err != nil {
			return false, err
		}
		root, err := safeopen.Root(path)
		if err != nil {
			return false, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "cannot inspect common directory identity: %v", err)
		}
		defer func() { retErr = stagingCleanup(retErr, root.Close()) }()
		info, err := root.Stat(".")
		if err != nil {
			return false, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "cannot stat common directory identity: %v", err)
		}
		if !info.IsDir() {
			return false, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "common directory is not a directory")
		}
		identities[i] = info
	}
	return os.SameFile(identities[0], identities[1]), nil
}

// Export captures one TM-V0-008 snapshot of the whole store as a
// taskman-archive/0 stream, stages it, verifies the staged stream and only
// then copies it to Stdout. Validation and staging failures emit zero bytes; delivery failure or interruption may leave a prefix.
func Export(opts ExportOptions) (result *ExportResult, retErr error) {
	limit := uint64(wire.MaxArchiveBytes)
	if opts.streamLimit > 0 && opts.streamLimit < limit {
		limit = opts.streamLimit
	}
	var streamBudget *streamWriter
	var streamExceeded bool
	SnapshotOnFailure = nil
	repo := opts.Repo
	staging, err := checkStaging(opts.Staging, repo)
	if err != nil {
		return nil, err
	}
	defer func() {
		retErr = stagingCleanup(retErr, staging.Close())
		if retErr != nil {
			result = nil
		}
	}()
	var staged *os.File
	var cleanupErr error
	cleanup := func() error {
		if staged != nil {
			cleanupErr = errors.Join(cleanupErr, closeStaging(staged))
			staged = nil
		}
		return cleanupErr
	}
	defer func() {
		retErr = stagingCleanup(retErr, cleanup())
		if retErr != nil {
			result = nil
		}
	}()
	var manifest *Manifest
	var total uint64
	rd := snapshot.Reader{StateDir: repo.StateDir, IntentTree: func() (wire.Digest, error) {
		t, err := intent.TreeDigest(repo.PrimaryWorktree)
		if err != nil {
			return "", err
		}
		return t.Sha256, nil
	}}
	snap, err := readArchive(rd, func(s *snapshot.Snapshot) (bodyErr error) {
		if err := cleanup(); err != nil {
			return stagingCleanup(nil, err)
		}
		streamBudget = nil
		manifest = nil
		total = 0
		defer func() {
			if streamBudget != nil && streamBudget.exceeded {
				streamExceeded = true
			}
		}()
		life := newArchiveRead(archiveNative{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree})
		defer func() {
			if e := life.close(); e != nil {
				cleanupErr = errors.Join(cleanupErr, e)
				bodyErr = stagingCleanup(bodyErr, e)
			}
		}()
		if opts.afterProbe != nil {
			opts.afterProbe()
		}
		if s.Head.PrimaryWorktree != repo.PrimaryWorktree {
			return wire.Errorf(wire.CodeUnsupportedFilesystem, repo.StateDir, "head.primaryWorktree %q differs from the resolved primary worktree %q (repository relocation is unsupported in this preview)", s.Head.PrimaryWorktree, repo.PrimaryWorktree)
		}
		files, before, tree, err := life.captureLayout(wire.MaxArchiveScanEntries, wire.MaxArchiveFiles)
		if err != nil {
			return err
		}
		defer func() {
			if opts.afterBody != nil {
				if e := opts.afterBody(); e != nil {
					if bodyErr == nil {
						bodyErr = e
					} else {
						bodyErr = wire.Errorf(wire.CodeOf(bodyErr), "body", "%v", errors.Join(bodyErr, e))
					}
				}
			}
			_, after, _, e := life.captureLayout(wire.MaxArchiveScanEntries, wire.MaxArchiveFiles)
			if e != nil {
				bodyErr = e
				return
			}
			if !sameLayout(before, after) {
				bodyErr = archiveMoved("inventory")
			}
		}()
		if opts.afterCapture != nil {
			opts.afterCapture()
		}
		if tree.Sha256 != s.IntentTree {
			return archiveMoved("intent")
		}
		if err = life.validateStage(before, s, tree); err != nil {
			return err
		}
		// Pass 1: digest and size every file; check the receipt set.
		m := &Manifest{
			QueueID:           s.Head.QueueID,
			ExportedAtSeq:     s.Head.LastSeq,
			HeadSha256:        s.HeadSha256,
			VersionSha256:     wire.Sum(s.Version),
			PrimaryWorktree:   s.Head.PrimaryWorktree,
			IntentTreeSha256:  s.IntentTree,
			ReceiptCount:      s.Head.LastSeq,
			LastReceiptSha256: *s.Head.LastReceiptSha256,
			HeadGeneration:    s.Head.Generation,
			Complete:          true,
		}
		if s.BarrierRaw != nil {
			d := wire.Sum(s.BarrierRaw)
			m.BarrierSha256 = &d
		}
		digests := make(map[string]wire.Digest, len(files))
		var sum uint64
		receipts := 0
		lastSeq := s.Head.LastSeq.Uint64()
		for _, f := range files {
			raw, err := f.read()
			if err != nil {
				return err
			}
			d := wire.Sum(raw)
			if err := checkContentName(f.path, d); err != nil {
				return err
			}
			digests[f.path] = d
			sum += uint64(len(raw))
			if sum > wire.MaxArchiveBytes {
				return wire.Errorf(wire.CodeLimitExceeded, repo.StateDir, "archive larger than %d bytes", wire.MaxArchiveBytes)
			}
			m.Files = append(m.Files, FileEntry{Path: f.path, Sha256: d, Bytes: wire.SizeOf(uint64(len(raw)))})
			if strings.HasPrefix(f.path, "receipts/") {
				receipts++
				name := strings.TrimPrefix(f.path, "receipts/")
				seq, ok := parseReceiptName(name)
				if !ok {
					return wire.Errorf(wire.CodeMalformed, f.abs, "receipt file name is not <12 digits>.json")
				}
				if seq > lastSeq {
					return wire.Errorf(wire.CodeJournalForked, f.abs, "receipt %d is beyond head.lastSeq %d", seq, lastSeq)
				}
			}
		}
		if uint64(receipts) != lastSeq {
			return wire.Errorf(wire.CodeJournalForked, repo.StateDir, "%d receipt files but head.lastSeq is %d", receipts, lastSeq)
		}
		// Pass 2: stream into the staging file, re-digesting each file.
		f, err := createStaging(staging, &cleanupErr)
		if err != nil {
			return wire.Errorf(wire.CodeUnsupportedFilesystem, "staging", "cannot create staging file: %v", err)
		}
		staged = f
		streamBudget = &streamWriter{w: f, limit: limit}
		tw := tar.NewWriter(streamBudget)
		manifestBytes := m.Encode()
		if err := writeEntry(tw, ManifestName, manifestBytes); err != nil {
			return err
		}
		ordered := make([]FileEntry, len(m.Files))
		copy(ordered, m.Files)
		SortFiles(ordered)
		byPath := make(map[string]sourceFile, len(files))
		for _, sf := range files {
			byPath[sf.path] = sf
		}
		for _, fe := range ordered {
			sf := byPath[fe.Path]
			raw, err := sf.read()
			if err != nil {
				return err
			}
			if wire.Sum(raw) != digests[fe.Path] {
				return wire.Errorf(wire.CodeSnapshotMoved, sf.abs, "file changed between digest and copy")
			}
			if err := writeEntry(tw, fe.Path, raw); err != nil {
				return err
			}
		}
		if err := tw.Close(); err != nil {
			return wire.Errorf(wire.CodeUnsupportedFilesystem, "staging", "cannot finish staging file: %v", err)
		}
		manifest = m
		total = sum
		return nil
	})
	if err != nil {
		SnapshotOnFailure = snap
		if streamExceeded {
			return nil, streamLimitError(limit)
		}
		return nil, err
	}
	if streamExceeded {
		return nil, streamLimitError(limit)
	}
	if cleanupErr != nil {
		return nil, stagingCleanup(nil, cleanupErr)
	}
	SnapshotOnFailure = nil
	// Verify the staged stream by the same procedure as `archive verify`.
	if opts.afterStaged != nil {
		opts.afterStaged(staged)
	}
	if _, err := staged.Seek(0, io.SeekStart); err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, "staging", "cannot seek before verification: %v", err)
	}
	vr, err := Verify(staged)
	if err != nil {
		return nil, err
	}
	if string(vr.Manifest.Encode()) != string(manifest.Encode()) {
		return nil, wire.Errorf(wire.CodeMalformed, "staging", "staged manifest differs from the computed manifest")
	}
	if opts.afterVerified != nil {
		opts.afterVerified()
	}
	if _, err := staged.Seek(0, io.SeekStart); err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, "staging", "cannot seek before delivery: %v", err)
	}
	delivered, err := io.Copy(opts.Stdout, staged)
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, "stdout", "copy failed after %d bytes; stdout may contain a partial archive: %v", delivered, err)
	}
	return &ExportResult{Manifest: manifest, Snapshot: snap, Bytes: total}, nil
}

func writeEntry(tw *tar.Writer, name string, data []byte) error {
	if err := tw.WriteHeader(archiveHeader(name, int64(len(data)))); err != nil {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, name, "tar header: %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, name, "tar body: %v", err)
	}
	return nil
}

// archiveHeader is the one fixed header used by production export and by
// encoding-size measurement (TM-V0-022). Keep its fields byte-identical.
func archiveHeader(name string, size int64) *tar.Header {
	return &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Mode:     0o644,
		Size:     size,
		ModTime:  time.Unix(0, 0).UTC(),
		Format:   tar.FormatPAX,
	}
}

func parseReceiptName(name string) (uint64, bool) {
	if len(name) != 17 || !strings.HasSuffix(name, ".json") {
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
	return n, true
}

// The unlinked descriptor is the only staging identity used after creation.
// O_EXCL and the retained root prevent ordinary pathname replacement from
// redirecting creation; this is not protection against an OS owner moving root.
func createStaging(root *os.Root, cleanupErr *error) (*os.File, error) {
	name := "corvint-tasks-archive-" + rand.Text() + ".tar"
	f, err := safeopen.InRoot(root, name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600, false)
	if err != nil {
		return nil, err
	}
	if err := removeStaging(root, name); err != nil {
		// This is an effect of staging, not a source read error. A moved
		// snapshot may discard the callback error, but never these failures.
		failed := errors.Join(err, closeStaging(f))
		*cleanupErr = errors.Join(*cleanupErr, failed)
		return nil, failed
	}
	return f, nil
}

// Test seams for unlink and close failures; no production option changes this boundary.
var removeStaging = (*os.Root).Remove
var closeStaging = (*os.File).Close

func stagingCleanup(primary, cleanup error) error {
	if cleanup == nil {
		return primary
	}
	if primary == nil {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, "staging", "cleanup failed: %v", cleanup)
	}
	return wire.Errorf(wire.CodeOf(primary), "staging", "%v; cleanup failed: %v", primary, cleanup)
}
