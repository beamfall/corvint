// SPDX-License-Identifier: Apache-2.0

package runevidence

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// Store layout, bounds and reason codes (design decisions 1, 6, 7 and 8).
const (
	// RetainPerTest is how many of the newest records containing a test the writer keeps
	// (RVD-V1-009).
	RetainPerTest = 20
	// DefaultCapBytes caps the total size of the stored records (RVD-V1-009; the proposed 64 MiB of
	// the change's Open item 1).
	DefaultCapBytes = 64 << 20
	// MaxRecordBytes bounds one record: a receipt is at most 1 MiB and an ingested document at most
	// the flow-input bound of 5 MiB.
	MaxRecordBytes = 8 << 20
	// maxLogBytes bounds log.jsonl as a reader reads it.
	maxLogBytes = 64 << 20
	// MaxReadRecords bounds how many log entries, or record files when the log is unreadable, one
	// read takes; a store the writer keeps holds far fewer, so more means a damaged store.
	MaxReadRecords = 4096
	// DefaultLockWait bounds how long an append waits for the store lock before it fails as
	// run-evidence-write-failed (RVD-V1-010).
	DefaultLockWait = 5 * time.Second
	// lockRetry is the pause between two attempts to take the store lock.
	lockRetry = 25 * time.Millisecond

	// ReasonCorrupt names a skipped record or an unreadable log (RVD-V1-010).
	ReasonCorrupt = "run-evidence-corrupt"
	// ReasonWriteFailed names an append that did not complete (RVD-V1-010).
	ReasonWriteFailed = "run-evidence-write-failed"

	recordsDir = "records"
	logName    = "log.jsonl"
	lockName   = "log.lock"
)

var recordName = regexp.MustCompile(`^[0-9a-f]{64}\.json$`)

// readSlack is what a read allows past the cap: the newest record, which the writer keeps even
// when it alone exceeds the cap. A variable so a test can bound a read without 64 MiB of records.
var readSlack int64 = MaxRecordBytes

// ErrLockBusy is an append that could not take the store lock within its wait (RVD-V1-010).
var ErrLockBusy = errors.New("the run evidence store lock is held by another writer")

// Locate is the store directory of a repository whose absolute Git directory is gitDir:
// <git-dir>/corvint/evidence, never inside the worktree (RVD-V1-001).
func Locate(gitDir string) string {
	return filepath.Join(gitDir, "corvint", "evidence")
}

// ID is the record ID of data: the lowercase hex SHA-256 of the bytes, the same digest
// `corvint-tasks receipt put FILE` stores a witness receipt under (RVD-V1-003).
func ID(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Entry is one line of log.jsonl (RVD-V1-002). Tests is the retention keys of the record's rows,
// so a reader can name the tests of a record that no longer hashes to its name (RVD-V1-010).
type Entry struct {
	ID        string   `json:"id"`
	Profile   string   `json:"profile"`
	Tree      string   `json:"tree"`
	Commit    string   `json:"commit"`
	Instances []string `json:"instances"`
	Tests     []string `json:"tests"`
	Bytes     int64    `json:"bytes"`
	WrittenAt string   `json:"writtenAt"`
}

// Store is one repository's run evidence store.
type Store struct {
	// Dir is <git-dir>/corvint/evidence.
	Dir string
	// CapBytes caps the stored record bytes; zero is DefaultCapBytes.
	CapBytes int64
	// Keep is the per-test retention; zero is RetainPerTest.
	Keep int
	// Now is the write clock; nil is time.Now.
	Now func() time.Time
	// LockWait bounds the wait for the store lock; zero is DefaultLockWait.
	LockWait time.Duration
}

func (s Store) lockWait() time.Duration {
	if s.LockWait > 0 {
		return s.LockWait
	}
	return DefaultLockWait
}

// lockFile takes the store's exclusive lock, retrying without blocking until wait has passed, and
// returns the release. A lock still held at the deadline is ErrLockBusy, so a stuck writer never
// stalls the run that appends (RVD-V1-010).
func lockFile(name string, wait time.Duration) (func(), error) {
	file, err := openLock(name)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		held, err := tryLock(file)
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		if held {
			return func() {
				unlockFile(file)
				_ = file.Close()
			}, nil
		}
		if !time.Now().Before(deadline) {
			_ = file.Close()
			return nil, fmt.Errorf("%w after %s", ErrLockBusy, wait)
		}
		time.Sleep(lockRetry)
	}
}

func (s Store) clock() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s Store) capBytes() int64 {
	if s.CapBytes > 0 {
		return s.CapBytes
	}
	return DefaultCapBytes
}

func (s Store) keep() int {
	if s.Keep > 0 {
		return s.Keep
	}
	return RetainPerTest
}

// Append stores data as one record and lists it in log.jsonl under an exclusive lock, then prunes
// to the retention and the cap (RVD-V1-001, RVD-V1-002, RVD-V1-009). It returns the record ID. A
// document that is not a run document is refused before anything is written. The store is
// private: directories 0700, files 0600.
func (s Store) Append(data []byte) (string, error) {
	parsed, err := Parse(data)
	if err != nil {
		return "", err
	}
	id := ID(data)
	records := filepath.Join(s.Dir, recordsDir)
	if err := makeStoreDirs(filepath.Dir(s.Dir), s.Dir, records); err != nil {
		return "", err
	}
	unlock, err := lockFile(filepath.Join(s.Dir, lockName), s.lockWait())
	if err != nil {
		return "", err
	}
	defer unlock()
	if err := writeOnce(filepath.Join(records, id+".json"), id, data); err != nil {
		return "", err
	}
	entry := Entry{
		ID: id, Profile: parsed.Profile, Tree: parsed.Tree, Commit: parsed.Commit, Instances: parsed.Instances,
		Tests: parsed.Tests(), Bytes: int64(len(data)), WrittenAt: s.clock().UTC().Format(time.RFC3339Nano),
	}
	entries, intact, torn := s.loadLog()
	if !intact {
		// RVD-V1-010: the writer rebuilds an unreadable log from records/.
		entries = s.rebuild(id)
	}
	entries = append(entries, entry)
	kept := s.retain(entries)
	// A torn tail (a writer died mid-line) would join the new line to the partial one, so the
	// log is rewritten whole instead of appended to.
	if intact && !torn && len(kept) == len(entries) {
		line, err := json.Marshal(entry)
		if err != nil {
			return "", err
		}
		return id, appendLine(filepath.Join(s.Dir, logName), line)
	}
	if err := s.writeLog(kept); err != nil {
		return "", err
	}
	s.removeUnreferenced(kept)
	return id, nil
}

// makeStoreDirs creates each directory, in order, 0700. A linked <git-dir>/corvint or store
// directory could carry the write anywhere, tracked state included, so each must be a real
// directory before anything is created inside it; a link is refused, never followed.
func makeStoreDirs(dirs ...string) error {
	if err := os.MkdirAll(filepath.Dir(dirs[0]), 0o755); err != nil {
		return err
	}
	for _, dir := range dirs {
		info, err := os.Lstat(dir)
		if errors.Is(err, fs.ErrNotExist) {
			if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}
			info, err = os.Lstat(dir)
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory of the store", dir)
		}
	}
	return nil
}

// writeOnce writes a record through a temporary file and a rename. An existing record of the same
// name that still hashes to it is kept as it is; a record is never edited (RVD-V1-001).
func writeOnce(name, id string, data []byte) error {
	if existing, err := readBounded(name, MaxRecordBytes); err == nil && ID(existing) == id {
		return nil
	}
	return replace(name, data)
}

func replace(name string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(name), "."+filepath.Base(name)+".*")
	if err != nil {
		return err
	}
	_, writeErr := temporary.Write(data)
	syncErr := temporary.Sync()
	closeErr := temporary.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	if err := os.Chmod(temporary.Name(), 0o600); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	if err := os.Rename(temporary.Name(), name); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	return nil
}

// appendLine appends one line to the log, never through a final symlink, and syncs it before the
// append reports success.
func appendLine(name string, line []byte) error {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_APPEND|os.O_CREATE|noFollow, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(line, '\n'))
	syncErr := file.Sync()
	return errors.Join(writeErr, syncErr, file.Close())
}

func (s Store) writeLog(entries []Entry) error {
	var buffer bytes.Buffer
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		buffer.Write(line)
		buffer.WriteByte('\n')
	}
	return replace(filepath.Join(s.Dir, logName), buffer.Bytes())
}

// loadLog reads log.jsonl. intact is false when the log exists but cannot be read whole: a line
// that does not parse or names a malformed ID. A missing log with no records is an empty, intact
// store; a missing log beside records is not intact. torn is set when a non-empty log does not
// end in a newline: under the lock that is a line a dead writer left unfinished.
func (s Store) loadLog() (entries []Entry, intact, torn bool) {
	raw, err := readBounded(filepath.Join(s.Dir, logName), maxLogBytes)
	if errors.Is(err, fs.ErrNotExist) {
		names, _ := listRecords(filepath.Join(s.Dir, recordsDir))
		return nil, len(names) <= 1, false
	}
	if err != nil {
		return nil, false, false
	}
	entries, intact = decodeLog(raw)
	return entries, intact, len(raw) != 0 && raw[len(raw)-1] != '\n'
}

// decodeLog parses whole lines. A final line without a newline is an append in progress and is
// not read; it does not make the log unreadable.
func decodeLog(raw []byte) ([]Entry, bool) {
	entries := []Entry{}
	for len(raw) != 0 {
		line, rest, complete := bytes.Cut(raw, []byte{'\n'})
		if !complete {
			break
		}
		raw = rest
		var entry Entry
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&entry) != nil || !recordName.MatchString(entry.ID+".json") {
			return nil, false
		}
		entries = append(entries, entry)
	}
	return entries, true
}

// rebuild lists every intact record in records/ by file time, then name, as the log a writer
// writes after an unreadable one (RVD-V1-010). The record being appended is left out; the caller
// appends it last.
func (s Store) rebuild(appending string) []Entry {
	dir := filepath.Join(s.Dir, recordsDir)
	names, _ := listRecords(dir)
	entries := []Entry{}
	for _, name := range names {
		id := strings.TrimSuffix(name, ".json")
		if id == appending {
			continue
		}
		data, err := readBounded(filepath.Join(dir, name), MaxRecordBytes)
		if err != nil || ID(data) != id {
			continue
		}
		parsed, err := Parse(data)
		if err != nil {
			continue
		}
		info, _ := os.Lstat(filepath.Join(dir, name))
		written := ""
		if info != nil {
			written = info.ModTime().UTC().Format(time.RFC3339Nano)
		}
		entries = append(entries, Entry{ID: id, Profile: parsed.Profile, Tree: parsed.Tree, Commit: parsed.Commit,
			Instances: parsed.Instances, Tests: parsed.Tests(), Bytes: int64(len(data)), WrittenAt: written})
	}
	return entries
}

// listRecords returns the record file names in records/ ordered by modification time, then name.
// It reads at most maxListEntries directory entries, so a flooded directory cannot exhaust memory.
func listRecords(dir string) ([]string, error) {
	items, err := readDirBounded(dir, maxListEntries)
	if err != nil {
		return nil, err
	}
	type named struct {
		name string
		at   time.Time
	}
	var found []named
	for _, item := range items {
		if !item.Type().IsRegular() || !recordName.MatchString(item.Name()) {
			continue
		}
		info, err := item.Info()
		if err != nil {
			continue
		}
		found = append(found, named{item.Name(), info.ModTime()})
	}
	sort.SliceStable(found, func(i, j int) bool {
		if !found[i].at.Equal(found[j].at) {
			return found[i].at.Before(found[j].at)
		}
		return found[i].name < found[j].name
	})
	names := make([]string, 0, len(found))
	for _, item := range found {
		names = append(names, item.name)
	}
	return names, nil
}

// maxListEntries bounds the directory entries listRecords reads.
const maxListEntries = 4 * MaxReadRecords

func readDirBounded(dir string, limit int) ([]fs.DirEntry, error) {
	file, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	items, err := file.ReadDir(limit)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return items, err
}

// retain applies RVD-V1-009 to entries in write order and returns those kept, in write order.
// Walking newest first, an entry is kept while one of its tests has fewer than Keep newer kept
// entries; the newest entry is always kept. Then the oldest kept entries go until the distinct
// record bytes fit the cap, never the newest.
func (s Store) retain(entries []Entry) []Entry {
	keep := make([]bool, len(entries))
	counts := map[string]int{}
	for index := len(entries) - 1; index >= 0; index-- {
		needed := index == len(entries)-1
		for _, test := range entries[index].Tests {
			if counts[test] < s.keep() {
				needed = true
			}
		}
		if !needed {
			continue
		}
		keep[index] = true
		for _, test := range entries[index].Tests {
			counts[test]++
		}
	}
	size := func() int64 {
		total, seen := int64(0), map[string]bool{}
		for index, entry := range entries {
			if keep[index] && !seen[entry.ID] {
				seen[entry.ID] = true
				total += entry.Bytes
			}
		}
		return total
	}
	for index := 0; index < len(entries)-1 && size() > s.capBytes(); index++ {
		keep[index] = false
	}
	kept := []Entry{}
	for index, entry := range entries {
		if keep[index] {
			kept = append(kept, entry)
		}
	}
	return kept
}

// removeUnreferenced deletes the record files no kept entry names, after the log naming only the
// kept entries is in place, so a reader of the old log at worst finds a record missing.
func (s Store) removeUnreferenced(kept []Entry) {
	referenced := map[string]bool{}
	for _, entry := range kept {
		referenced[entry.ID+".json"] = true
	}
	dir := filepath.Join(s.Dir, recordsDir)
	names, _ := listRecords(dir)
	for _, name := range names {
		if !referenced[name] {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

// Record is one stored record as a reader sees it.
type Record struct {
	Entry Entry
	Data  []byte
	Rows  []Row
}

// Corrupt is one record a reader skipped, with the tests its log line names (RVD-V1-010).
type Corrupt struct {
	ID    string   `json:"id"`
	Tests []string `json:"tests"`
}

// Snapshot is what one read of the store found: the intact records in write order, the skipped
// ones, and the reasons a report prints.
type Snapshot struct {
	Records []Record
	Corrupt []Corrupt
	// LogUnreadable is set when log.jsonl could not be read and the records were listed by file
	// time instead (RVD-V1-010).
	LogUnreadable bool
	Reasons       []string
}

// Present reports whether the read found anything worth reporting.
func (s Snapshot) Present() bool {
	return len(s.Records) != 0 || len(s.Corrupt) != 0 || len(s.Reasons) != 0
}

// CorruptTests is the sorted retention keys of every skipped record's tests.
func (s Snapshot) CorruptTests() []string {
	var tests []string
	for _, corrupt := range s.Corrupt {
		tests = append(tests, corrupt.Tests...)
	}
	slices.Sort(tests)
	return slices.Compact(tests)
}

// Read reads the store without writing anything: no directory, lock, log or record is created,
// changed or removed (RVD-V1-012, product invariant 4). A missing store is an empty snapshot. A
// record that does not hash to its name or does not parse is skipped and named; a record the log
// names that is missing was pruned by a concurrent writer and is passed over.
//
// A read is bounded: at most MaxReadRecords entries and, counting each record once, at most the
// writer's cap plus one record of bytes. The writer never keeps more, so a store past either bound
// is damaged: the newest records within the bounds are read and run-evidence-corrupt is reported.
func (s Store) Read() (Snapshot, error) {
	snapshot := Snapshot{}
	if _, err := os.Lstat(s.Dir); errors.Is(err, fs.ErrNotExist) {
		return snapshot, nil
	} else if err != nil {
		return snapshot, err
	}
	dir := filepath.Join(s.Dir, recordsDir)
	raw, err := readBounded(filepath.Join(s.Dir, logName), maxLogBytes)
	entries, intact := []Entry{}, true
	switch {
	case errors.Is(err, fs.ErrNotExist):
		names, _ := listRecords(dir)
		intact = len(names) == 0
	case err != nil:
		intact = false
	default:
		entries, intact = decodeLog(raw)
	}
	corrupt := func() {
		if !slices.Contains(snapshot.Reasons, ReasonCorrupt) {
			snapshot.Reasons = append(snapshot.Reasons, ReasonCorrupt)
		}
	}
	if !intact {
		snapshot.LogUnreadable = true
		corrupt()
		names, _ := listRecords(dir)
		entries = entries[:0]
		for _, name := range names {
			entries = append(entries, Entry{ID: strings.TrimSuffix(name, ".json"), Tests: []string{}})
		}
	}
	if len(entries) > MaxReadRecords {
		entries = entries[len(entries)-MaxReadRecords:]
		corrupt()
	}
	// Newest first, so the bounds keep the newest records; the result is put back in write order.
	budget := s.capBytes() + readSlack
	read := map[string][]byte{}
	var records []Record
	var skipped []Corrupt
	for index := len(entries) - 1; index >= 0; index-- {
		entry := entries[index]
		data, seen := read[entry.ID]
		var err error
		if !seen {
			name := filepath.Join(dir, entry.ID+".json")
			info, statErr := os.Lstat(name)
			if errors.Is(statErr, fs.ErrNotExist) {
				continue
			}
			if statErr == nil && info.Size() > budget && info.Size() <= MaxRecordBytes {
				corrupt()
				break
			}
			data, err = readBounded(name, min(MaxRecordBytes, budget))
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err == nil {
				budget -= int64(len(data))
				read[entry.ID] = data
			}
		}
		var parsed Parsed
		if err == nil && ID(data) == entry.ID {
			parsed, err = Parse(data)
		} else if err == nil {
			err = errors.New("record bytes do not hash to the record's name")
		}
		if err != nil {
			skipped = append(skipped, Corrupt{ID: entry.ID, Tests: nonNil(entry.Tests)})
			corrupt()
			continue
		}
		if snapshot.LogUnreadable {
			entry = Entry{ID: entry.ID, Profile: parsed.Profile, Tree: parsed.Tree, Commit: parsed.Commit,
				Instances: parsed.Instances, Tests: parsed.Tests(), Bytes: int64(len(data))}
		}
		records = append(records, Record{Entry: entry, Data: data, Rows: parsed.Rows})
	}
	slices.Reverse(records)
	slices.Reverse(skipped)
	snapshot.Records, snapshot.Corrupt = records, skipped
	return snapshot, nil
}

// Latest returns, for each retention key, the row of the newest record that contains it
// (RVD-V1-007), keyed by Row.Key.
func (s Snapshot) Latest() map[string]Row {
	latest := map[string]Row{}
	for _, record := range s.Records {
		for _, row := range record.Rows {
			latest[row.Key()] = row
		}
	}
	return latest
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// readBounded reads at most limit bytes of a regular file without following a final symlink and
// without blocking on a FIFO: the name is checked with Lstat before the open and the descriptor
// after it.
func readBounded(name string, limit int64) ([]byte, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", filepath.Base(name))
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if opened, err := file.Stat(); err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("%s changed while it was opened", filepath.Base(name))
	}
	data, err := io.ReadAll(bufio.NewReader(io.LimitReader(file, limit+1)))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", filepath.Base(name), limit)
	}
	return data, nil
}
