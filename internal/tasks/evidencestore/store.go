// Package evidencestore is the write-once, content-addressed store that
// keeps the bytes behind a witness digest (TOL-V0-133..140, beamfall/corvint#1837).
//
// A Playwright report witness records only the report's SHA-256 in the
// obligation ledger. This store keeps the report itself under
// <state directory>/witness-receipts/<sha256>, so the digest can be resolved
// back to the bytes it names. It is a local, bounded directory beside the
// journal, not part of it: no receipt names its files, the journal audit
// does not fold them, and writing one never changes the journal, intent or
// checkpoint bytes. Every read verifies the file's digest against its name.
package evidencestore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const (
	// Dir is the state-dir child that holds the store (TOL-V0-133).
	Dir = "witness-receipts"
	// MaxBlobBytes bounds one stored file; a Playwright report is read
	// under the same 64 MiB bound.
	MaxBlobBytes = wire.MaxEvidenceBlobBytes
	// MaxFiles bounds the number of stored files.
	MaxFiles = 100000
	// MaxStoreBytes bounds the total stored bytes.
	MaxStoreBytes = wire.MaxEvidenceStoreBytes
	// TempPrefix starts the name of a put in progress; readers skip it.
	TempPrefix = ".tmp-"
	// PruneGrace is how recently a file must have been put (or put again)
	// for prune to keep it although no witness cites it yet, so a
	// concurrent witness between its put and its ledger write is not
	// raced (TOL-V0-140).
	PruneGrace = 15 * time.Minute
)

// Put outcomes.
const (
	StoredNew      = "NEW"
	StoredExisting = "EXISTING"
)

// Check states (TOL-V0-138).
const (
	StatePresent    = "PRESENT"
	StateMissing    = "RECEIPT_MISSING"
	StateMismatch   = "RECEIPT_MISMATCH"
	StateUnreadable = "RECEIPT_UNREADABLE"
)

// PutResult reports one stored file.
type PutResult struct {
	Sha256 wire.Digest
	Path   string
	Bytes  int64
	Stored string
}

// Entry is one stored file a listing found.
type Entry struct {
	Sha256  wire.Digest
	Bytes   int64
	ModTime time.Time
}

// Path is the absolute path of the file that holds digest d.
func Path(stateDir string, d wire.Digest) string {
	return filepath.Join(stateDir, Dir, string(d))
}

// stateRoot requires the state directory to exist as a real directory; the
// store never creates it.
func stateRoot(stateDir string) error {
	info, err := os.Lstat(stateDir)
	if err != nil {
		return wire.Errorf(wire.CodeUninitialized, stateDir, "no Tasks state directory: %v", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, stateDir, "state directory is not a directory")
	}
	return nil
}

// openStore opens the store directory as an os.Root, so no name can escape
// it through a symlink. create makes it (0700) when absent; a read never
// does, and reports absent as (nil, nil).
func openStore(stateDir string, create bool) (*os.Root, error) {
	if err := stateRoot(stateDir); err != nil {
		return nil, err
	}
	dir := filepath.Join(stateDir, Dir)
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		if !create {
			return nil, nil
		}
		if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, dir, "cannot create the witness receipt store: %v", err)
		}
		info, err = os.Lstat(dir)
	}
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, dir, "cannot stat the witness receipt store: %v", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, dir, "the witness receipt store is not a directory")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, dir, "cannot open the witness receipt store: %v", err)
	}
	return root, nil
}

// Put stores raw under its SHA-256 and never replaces a file: an existing
// file with the same name is verified and reported EXISTING (its mtime is
// refreshed so a concurrent prune keeps it for PruneGrace); an existing file
// whose bytes differ from its name is refused, never overwritten. A new file
// is written to a temp name, synced and hard-linked to its final name, so a
// reader never sees a partial file under a digest name (TOL-V0-134).
func Put(stateDir string, raw []byte) (*PutResult, error) {
	if len(raw) > MaxBlobBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, Dir, "a witness receipt is larger than %d bytes", MaxBlobBytes)
	}
	d := wire.Sum(raw)
	root, err := openStore(stateDir, true)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	res := &PutResult{Sha256: d, Path: Path(stateDir, d), Bytes: int64(len(raw)), Stored: StoredExisting}
	if ok, err := existing(root, d); err != nil {
		return nil, err
	} else if ok {
		return res, nil
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir, "cannot name a temp file: %v", err)
	}
	tmp := TempPrefix + hex.EncodeToString(nonce[:])
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir, "cannot create a temp file: %v", err)
	}
	defer root.Remove(tmp)
	_, werr := f.Write(raw)
	if werr == nil {
		werr = f.Sync()
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir, "cannot write a temp file: %v", werr)
	}
	// The bounds are checked after this put's temp file exists and count
	// every temp file, so of two concurrent puts the later check sees the
	// other's bytes and the store never exceeds them (TOL-V0-133).
	entries, temps, err := scan(root)
	if err != nil {
		return nil, err
	}
	total := temps.bytes
	for _, e := range entries {
		total += e.Bytes
	}
	if len(entries)+temps.files > MaxFiles || total > MaxStoreBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, Dir, "the witness receipt store would exceed %d files or %d bytes; prune uncited receipts first", MaxFiles, int64(MaxStoreBytes))
	}
	if err := root.Link(tmp, string(d)); err != nil {
		if errors.Is(err, fs.ErrExist) {
			// A concurrent put of the same bytes won; verify what it wrote.
			if _, err := existing(root, d); err != nil {
				return nil, err
			}
			return res, nil
		}
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir, "cannot link the stored file: %v", err)
	}
	syncDir(filepath.Join(stateDir, Dir))
	res.Stored = StoredNew
	return res, nil
}

// existing reports whether d is already stored with matching bytes; a stored
// file whose bytes differ is refused.
func existing(root *os.Root, d wire.Digest) (bool, error) {
	_, state, err := read(root, d)
	switch state {
	case StateMissing:
		return false, nil
	case StatePresent:
		now := time.Now()
		_ = root.Chtimes(string(d), now, now)
		return true, nil
	}
	return false, err
}

// Get returns the verified bytes stored under d. An absent store or file is
// MISSING_EVIDENCE; a file whose bytes differ from its name is MALFORMED.
// It never creates or changes anything (TOL-V0-136).
func Get(stateDir string, d wire.Digest) ([]byte, error) {
	root, err := openStore(stateDir, false)
	if err != nil {
		return nil, err
	}
	if root == nil {
		return nil, missing(d)
	}
	defer root.Close()
	raw, _, err := read(root, d)
	return raw, err
}

// Check classifies d as PRESENT, RECEIPT_MISSING, RECEIPT_MISMATCH or
// RECEIPT_UNREADABLE. The error explains every state but PRESENT.
func Check(stateDir string, d wire.Digest) (string, error) {
	root, err := openStore(stateDir, false)
	if err != nil {
		return StateUnreadable, err
	}
	if root == nil {
		return StateMissing, missing(d)
	}
	defer root.Close()
	_, state, err := read(root, d)
	return state, err
}

func missing(d wire.Digest) error {
	return wire.Errorf(wire.CodeMissingEvidence, Dir+"/"+string(d), "no witness receipt is stored under this digest")
}

func read(root *os.Root, d wire.Digest) ([]byte, string, error) {
	if _, err := wire.ParseDigest("sha256", string(d)); err != nil {
		return nil, StateUnreadable, err
	}
	name := string(d)
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, StateMissing, missing(d)
	}
	if err != nil {
		return nil, StateUnreadable, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir+"/"+name, "cannot stat: %v", err)
	}
	if !info.Mode().IsRegular() {
		return nil, StateUnreadable, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir+"/"+name, "a stored witness receipt is not a regular file")
	}
	if info.Size() > MaxBlobBytes {
		return nil, StateUnreadable, wire.Errorf(wire.CodeLimitExceeded, Dir+"/"+name, "a stored witness receipt is larger than %d bytes", MaxBlobBytes)
	}
	// O_NONBLOCK keeps a file swapped for a FIFO after the Lstat from
	// blocking the open; the descriptor must still be the regular file the
	// Lstat saw.
	f, err := root.OpenFile(name, os.O_RDONLY|openNonblock, 0)
	if err != nil {
		return nil, StateUnreadable, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir+"/"+name, "cannot open: %v", err)
	}
	defer f.Close()
	if opened, err := f.Stat(); err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, StateUnreadable, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir+"/"+name, "a stored witness receipt changed while it was opened")
	}
	raw, err := io.ReadAll(io.LimitReader(f, MaxBlobBytes+1))
	if err != nil {
		return nil, StateUnreadable, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir+"/"+name, "cannot read: %v", err)
	}
	if len(raw) > MaxBlobBytes {
		return nil, StateUnreadable, wire.Errorf(wire.CodeLimitExceeded, Dir+"/"+name, "a stored witness receipt is larger than %d bytes", MaxBlobBytes)
	}
	if wire.Sum(raw) != d {
		return nil, StateMismatch, wire.Errorf(wire.CodeMalformed, Dir+"/"+name, "stored bytes differ from the digest that names them")
	}
	return raw, StatePresent, nil
}

// List returns the stored files by digest, skipping puts in progress. Any
// other entry is refused, so an unknown file is never silently ignored.
func List(stateDir string) ([]Entry, error) {
	root, err := openStore(stateDir, false)
	if err != nil || root == nil {
		return nil, err
	}
	defer root.Close()
	return list(root)
}

func list(root *os.Root) ([]Entry, error) {
	entries, _, err := scan(root)
	return entries, err
}

// tempUse is what the in-progress temp files of a listing hold.
type tempUse struct {
	files int
	bytes int64
}

func scan(root *os.Root) ([]Entry, tempUse, error) {
	var temps tempUse
	f, err := root.Open(".")
	if err != nil {
		return nil, temps, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir, "cannot list: %v", err)
	}
	defer f.Close()
	names, err := f.Readdirnames(MaxFiles + 2)
	if err != nil && err != io.EOF {
		return nil, temps, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir, "cannot list: %v", err)
	}
	if len(names) > MaxFiles+1 {
		return nil, temps, wire.Errorf(wire.CodeLimitExceeded, Dir, "the witness receipt store holds more than %d files", MaxFiles)
	}
	sort.Strings(names)
	var out []Entry
	for _, n := range names {
		if strings.HasPrefix(n, TempPrefix) {
			if info, err := root.Lstat(n); err == nil {
				temps.files++
				temps.bytes += info.Size()
			}
			continue
		}
		d, err := wire.ParseDigest(Dir+"/"+n, n)
		if err != nil {
			return nil, temps, wire.Errorf(wire.CodeMalformed, Dir+"/"+n, "unexpected entry in the witness receipt store")
		}
		info, err := root.Lstat(n)
		if err != nil {
			return nil, temps, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir+"/"+n, "cannot stat: %v", err)
		}
		if !info.Mode().IsRegular() {
			return nil, temps, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir+"/"+n, "a stored witness receipt is not a regular file")
		}
		out = append(out, Entry{Sha256: d, Bytes: info.Size(), ModTime: info.ModTime()})
	}
	return out, temps, nil
}

// Remove deletes the file stored under d unless it was put again after
// cutoff, and reports whether it removed it; an absent file is not an
// error. Only prune calls it, for a digest no live witness cites
// (TOL-V0-140): the re-check keeps a file a concurrent put refreshed.
func Remove(stateDir string, d wire.Digest, cutoff time.Time) (bool, error) {
	if _, err := wire.ParseDigest("sha256", string(d)); err != nil {
		return false, err
	}
	root, err := openStore(stateDir, false)
	if err != nil || root == nil {
		return false, err
	}
	defer root.Close()
	info, err := root.Lstat(string(d))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir+"/"+string(d), "cannot stat: %v", err)
	}
	if info.ModTime().After(cutoff) {
		return false, nil
	}
	if err := root.Remove(string(d)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir+"/"+string(d), "cannot remove: %v", err)
	}
	return true, nil
}

// RemoveStaleTemps deletes temp files last written before cutoff: a put
// that died before its link leaves one, and it would count against the
// bounds forever. It reports how many it removed.
func RemoveStaleTemps(stateDir string, cutoff time.Time) (int64, error) {
	root, err := openStore(stateDir, false)
	if err != nil || root == nil {
		return 0, err
	}
	defer root.Close()
	f, err := root.Open(".")
	if err != nil {
		return 0, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir, "cannot list: %v", err)
	}
	names, err := f.Readdirnames(MaxFiles + 2)
	_ = f.Close()
	if err != nil && err != io.EOF {
		return 0, wire.Errorf(wire.CodeUnsupportedFilesystem, Dir, "cannot list: %v", err)
	}
	var removed int64
	for _, n := range names {
		if !strings.HasPrefix(n, TempPrefix) {
			continue
		}
		info, err := root.Lstat(n)
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		if err := root.Remove(n); err == nil {
			removed++
		}
	}
	return removed, nil
}

// syncDir makes a link durable where the platform allows a directory sync.
func syncDir(dir string) {
	if f, err := os.Open(dir); err == nil {
		_ = f.Sync()
		_ = f.Close()
	}
}
