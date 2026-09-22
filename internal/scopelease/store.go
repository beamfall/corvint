package scopelease

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// reap deletes every expired lease document and returns the live remainder and
// the unreadable lease ids, which it never deletes. It runs only under the
// lock, inside acquire, release, and renew.
func reap(directory string, moment time.Time) ([]Lease, []string, error) {
	leases, unreadable, err := load(directory)
	if err != nil {
		return nil, nil, err
	}
	remaining := make([]Lease, 0, len(leases))
	for _, lease := range leases {
		if live(lease, moment) {
			remaining = append(remaining, lease)
			continue
		}
		if err := os.Remove(filepath.Join(directory, lease.ID+".json")); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, nil, fmt.Errorf("scopelease: reap expired lease: %w", err)
		}
	}
	return remaining, unreadable, nil
}

// load reads every lease document, sorted by lease id, and separately names,
// sorted, every lease id whose document cannot be read or parsed, so one torn
// document never hides the others. A missing directory is an empty store,
// never an error, so read commands never create one.
func load(directory string) ([]Lease, []string, error) {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("scopelease: read lease directory: %w", err)
	}
	leases := make([]Lease, 0, len(entries))
	unreadable := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		lease, err := read(filepath.Join(directory, entry.Name()))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			unreadable = append(unreadable, strings.TrimSuffix(entry.Name(), ".json"))
			continue
		}
		leases = append(leases, lease)
	}
	sort.Slice(leases, func(first, second int) bool { return leases[first].ID < leases[second].ID })
	return leases, unreadable, nil
}

// documentLimit bounds one lease document. Acquire refuses a request whose
// document would exceed it, so only a foreign document is ever larger.
const documentLimit = 1 << 20

// widestInstant is the longest instant timeLayout formats for a four-digit year.
const widestInstant = "2006-01-02T15:04:05.999999999Z"

// fitsDocument reports whether a request's lease document, with every field
// Acquire generates at its widest, stays within documentLimit.
func fitsDocument(request Request) bool {
	widest := Lease{
		SchemaVersion: SchemaVersion,
		ID:            strings.Repeat("0", 16),
		Holder:        request.Holder,
		Paths:         request.Paths,
		Ticket:        request.Ticket,
		Note:          request.Note,
		AcquiredAt:    widestInstant,
		ExpiresAt:     widestInstant,
		Revision:      strings.Repeat("0", 64),
	}
	data, err := json.Marshal(widest)
	return err == nil && len(data)+1 <= documentLimit
}

func read(path string) (Lease, error) {
	// Corvint only ever publishes a lease document by renaming a regular file,
	// so anything else is unreadable and is never opened: a committed symlink
	// to a FIFO or device would otherwise block or never end (SCL-V0-011).
	info, err := os.Lstat(path)
	if err != nil {
		return Lease{}, err
	}
	if !info.Mode().IsRegular() {
		return Lease{}, fmt.Errorf("scopelease: lease document %s is not a regular file", filepath.Base(path))
	}
	file, err := os.Open(path)
	if err != nil {
		return Lease{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, documentLimit+1))
	if err != nil {
		return Lease{}, err
	}
	if len(data) > documentLimit {
		return Lease{}, fmt.Errorf("scopelease: lease document %s exceeds %d bytes", filepath.Base(path), documentLimit)
	}
	var lease Lease
	if err := json.Unmarshal(data, &lease); err != nil {
		return Lease{}, fmt.Errorf("scopelease: invalid lease document %s: %w", filepath.Base(path), err)
	}
	// Reaping, release, and renewal act on lease.ID+".json"; a document naming
	// any other id is unreadable so it can never remove or rewrite another lease.
	if lease.ID != strings.TrimSuffix(filepath.Base(path), ".json") {
		return Lease{}, fmt.Errorf("scopelease: lease document %s records lease id %q", filepath.Base(path), lease.ID)
	}
	if lease.Paths == nil {
		lease.Paths = []string{}
	}
	return lease, nil
}

// write is atomic and durable: one synced temporary file in the same
// directory, a rename, then a directory sync.
func write(directory string, lease Lease) error {
	data, err := json.Marshal(lease)
	if err != nil {
		return fmt.Errorf("scopelease: encode lease: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".lease-*.tmp")
	if err != nil {
		return fmt.Errorf("scopelease: create temporary lease: %w", err)
	}
	name := temporary.Name()
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		_ = temporary.Close()
		_ = os.Remove(name)
		return fmt.Errorf("scopelease: write temporary lease: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		_ = os.Remove(name)
		return fmt.Errorf("scopelease: sync temporary lease: %w", err)
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("scopelease: close temporary lease: %w", err)
	}
	if err := os.Rename(name, filepath.Join(directory, lease.ID+".json")); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("scopelease: publish lease: %w", err)
	}
	return syncDirectory(directory)
}

// identifier is a short hex digest of holder, scope, and acquisition nanos.
func identifier(request Request, acquired time.Time) string {
	digest := sha256.New()
	digest.Write([]byte(request.Holder))
	digest.Write([]byte{0})
	digest.Write([]byte(strings.Join(request.Paths, "\x00")))
	digest.Write([]byte{0})
	digest.Write([]byte(request.Ticket))
	digest.Write([]byte{0})
	digest.Write([]byte(strconv.FormatInt(acquired.UnixNano(), 10)))
	return hex.EncodeToString(digest.Sum(nil))[:16]
}

// revision records the tree revision observed at acquisition, for information
// only. It reads Git refs directly, runs no subprocess, and an unreadable or
// absent revision is recorded as the empty string. A symbolic HEAD must name a
// ref under refs/ with no empty or dot-leading segment, as Git itself
// requires, so the read never leaves .git; a ref held only in packed-refs is
// not resolved.
func revision(root string) string {
	value := readRef(filepath.Join(root, ".git", "HEAD"))
	reference, isSymbolic := strings.CutPrefix(value, "ref: ")
	if !isSymbolic {
		return sanitizeRevision(value)
	}
	if !containedReference(reference) {
		return ""
	}
	return sanitizeRevision(readRef(filepath.Join(root, ".git", filepath.FromSlash(reference))))
}

func containedReference(reference string) bool {
	segments := strings.Split(reference, "/")
	if segments[0] != "refs" {
		return false
	}
	for _, segment := range segments[1:] {
		if segment == "" || strings.HasPrefix(segment, ".") || strings.ContainsRune(segment, '\\') {
			return false
		}
	}
	return true
}

// refLimit bounds one ref read; Git reads a loose ref into a buffer this size.
const refLimit = 256

// readRef reads at most refLimit bytes of a regular file, trimmed. Anything
// else, including a FIFO or device that would block or never end, reads as
// empty, since acquire holds the writer lock while it reads.
func readRef(path string) string {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, refLimit))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func sanitizeRevision(value string) string {
	if len(value) < 7 || len(value) > 64 {
		return ""
	}
	if _, err := hex.DecodeString(value); err != nil {
		return ""
	}
	return value
}
