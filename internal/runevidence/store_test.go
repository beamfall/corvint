// SPDX-License-Identifier: Apache-2.0

package runevidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	treeA   = "1111111111111111111111111111111111111111"
	treeB   = "2222222222222222222222222222222222222222"
	commitA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	commitB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// receipt is a corvint-test-run/0 receipt with one Go package outcome per unit, in the shape
// sessiontest.Encode writes; extra names a member a newer Corvint might add.
func receipt(commit, tree string, outcomes ...string) []byte {
	var rows []string
	for _, outcome := range outcomes {
		unit, word, _ := strings.Cut(outcome, "=")
		rows = append(rows, fmt.Sprintf(`{"durationGranularity":"file","durationMs":12,"durationSource":"go-test2json","outcome":%q,"slice":"whole","unit":%q}`, word, unit))
	}
	return []byte(fmt.Sprintf(`{"base":%q,"checks":[],"graphDigest":"","incomplete":[],"invocations":[{"adapter":"go","adapterManifest":"m","argv":["go","test"],"elapsedMilliseconds":40,"env":[],"envDigest":"","exit":0,"outcomes":[%s],"runner":"/usr/bin/go","version":"go1.27.3"}],"manifestDigest":"","partitionDigest":"","placement":"universe","planDigest":"","profile":"corvint-test-run/0","reason":"","result":"PASS","revision":%q,"runnerProfile":"local","scope":"NARROWED","shardCount":1,"shardIndex":0,"source":{"head":%q,"paths":[],"tree":%q},"sourceDigest":"","summary":"","tool":"corvint test"}`+"\n",
		commit, strings.Join(rows, ","), commit, commit, tree))
}

// ingested is test-run-evidence/0 JSONL, one record per test, in the shape flows ingest prints.
func ingested(commit, tree string, tests map[string][]string) []byte {
	keys := make([]string, 0, len(tests))
	for key := range tests {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var out bytes.Buffer
	for _, key := range keys {
		var attempts []string
		for index, outcome := range tests[key] {
			attempts = append(attempts, fmt.Sprintf(`{"ordinal":%d,"outcome":%q,"duration_ms":5,"assertion_anchors":[],"attachments":[]}`, index+1, outcome))
		}
		fmt.Fprintf(&out, `{"schema":"test-run-evidence/0","authority":"INGESTED","run_id":"r1","runner":{"name":"playwright","version":"1.50.0"},"source":{"commit":%q,"tree":%q,"clean":true},"build_artifact_digest":"%s","environment":{"id":"e","digest":"%s"},"fixture":{"id":"f","digest":"%s"},"test_key":%q,"attempts":[%s],"cleanup":"not-declared","negative_controls":[]}`+"\n",
			commit, tree, strings.Repeat("c", 64), strings.Repeat("d", 64), strings.Repeat("e", 64), key, strings.Join(attempts, ","))
	}
	return out.Bytes()
}

func testStore(t *testing.T) Store {
	t.Helper()
	return Store{Dir: Locate(filepath.Join(t.TempDir(), ".git"))}
}

func readLog(t *testing.T, s Store) []Entry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.Dir, logName))
	if err != nil {
		t.Fatal(err)
	}
	entries, ok := decodeLog(raw)
	if !ok {
		t.Fatalf("log unreadable:\n%s", raw)
	}
	return entries
}

// RVD-V1-001, RVD-V1-002, RVD-V1-003: a record is stored once under the SHA-256 of its bytes, the
// log names it with its header, a second run leaves the first record's bytes alone, and the store is
// private under the Git directory.
func TestRVDV1001HashNamedRecordsAndLog(t *testing.T) {
	s := testStore(t)
	first := receipt(commitA, treeA, "go:m/a=PASS")
	clock := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return clock }
	id, err := s.Append(first)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(first)
	if id != hex.EncodeToString(sum[:]) || id != ID(first) {
		t.Fatalf("record ID %s is not the SHA-256 of the bytes", id)
	}
	stored, err := os.ReadFile(filepath.Join(s.Dir, recordsDir, id+".json"))
	if err != nil || !bytes.Equal(stored, first) {
		t.Fatalf("stored record differs: %v", err)
	}
	second := receipt(commitB, treeB, "go:m/a=FAIL")
	if _, err := s.Append(second); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(filepath.Join(s.Dir, recordsDir, id+".json")); !bytes.Equal(again, first) {
		t.Fatal("the first record's bytes changed")
	}
	entries := readLog(t, s)
	want := Entry{ID: id, Profile: ProfileReceipt, Tree: treeA, Commit: commitA, Instances: []string{"go"}, Tests: []string{"go:m/a"}, Bytes: int64(len(first)), WrittenAt: "2026-11-11T12:00:00Z"}
	if len(entries) != 2 || fmt.Sprint(entries[0]) != fmt.Sprint(want) || entries[1].Tree != treeB {
		t.Fatalf("log %+v", entries)
	}
	if runtime.GOOS != "windows" {
		for _, name := range []string{s.Dir, filepath.Join(s.Dir, recordsDir)} {
			if info, err := os.Stat(name); err != nil || info.Mode().Perm() != 0o700 {
				t.Fatalf("%s is not private: %v %v", name, info.Mode(), err)
			}
		}
		for _, name := range []string{filepath.Join(s.Dir, recordsDir, id+".json"), filepath.Join(s.Dir, logName)} {
			if info, err := os.Stat(name); err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("%s is not private: %v %v", name, info.Mode(), err)
			}
		}
	}
	if !strings.HasSuffix(filepath.ToSlash(s.Dir), ".git/corvint/evidence") {
		t.Fatalf("store %s is not under the Git directory", s.Dir)
	}
	if _, err := s.Append([]byte(`{"profile":"something-else/0"}`)); err == nil {
		t.Fatal("a document that is not a run document was stored")
	}
}

// RVD-V1-002: writers that finish together each append one whole log line and keep their record.
func TestRVDV1002ConcurrentAppends(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("the append lock needs flock")
	}
	s := testStore(t)
	const writers = 8
	var wait sync.WaitGroup
	errs := make(chan error, writers)
	ids := make([]string, writers)
	for index := range writers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			id, err := s.Append(receipt(commitA, treeA, fmt.Sprintf("go:m/p%d=PASS", index)))
			ids[index] = id
			errs <- err
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	entries := readLog(t, s)
	if len(entries) != writers {
		t.Fatalf("log has %d lines, want %d", len(entries), writers)
	}
	for _, id := range ids {
		if _, err := os.Stat(filepath.Join(s.Dir, recordsDir, id+".json")); err != nil {
			t.Fatalf("record %s missing: %v", id, err)
		}
		if !slices.ContainsFunc(entries, func(e Entry) bool { return e.ID == id }) {
			t.Fatalf("record %s not logged", id)
		}
	}
}

// RVD-V1-006: a receipt gives one file-granularity row per unit with its tree, commit, instance
// and duration; an ingested document gives one row per test with the final classification and the
// retry count, a retried pass being flaky.
func TestRVDV1006RowsFromBothProfiles(t *testing.T) {
	parsed, err := Parse(receipt(commitA, treeA, "go:m/a=PASS", "go:m/b=BUILD_FAILED"))
	if err != nil {
		t.Fatal(err)
	}
	twelve := int64(12)
	want := Row{TestID: "go:m/a", Granularity: GranularityFile, Outcome: "passed", NativeOutcome: "PASS", Instance: "go",
		RunnerVersion: "go1.27.3", Tree: treeA, Commit: commitA, Clean: true, DurationMs: &twelve, DurationSource: "go-test2json", DurationGranularity: "file"}
	if parsed.Profile != ProfileReceipt || len(parsed.Rows) != 2 || fmt.Sprint(*parsed.Rows[0].DurationMs) != "12" ||
		fmt.Sprintf("%+v", stripDuration(parsed.Rows[0])) != fmt.Sprintf("%+v", stripDuration(want)) || parsed.Rows[1].Outcome != "failed" {
		t.Fatalf("receipt rows %+v", parsed.Rows)
	}
	// A receipt with testId (change canonical-test-id) is keyed by it; without, by its unit.
	withID := bytes.Replace(receipt(commitA, treeA, "go:m/a=PASS"), []byte(`"unit":"go:m/a"`), []byte(`"testId":"go:m/a::","unit":"go:m/a"`), 1)
	if parsed, err := Parse(withID); err != nil || parsed.Rows[0].TestID != "go:m/a::" {
		t.Fatalf("testId not read: %v %+v", err, parsed.Rows)
	}
	parsed, err = Parse(ingested(commitB, treeB, map[string][]string{"e2e/a.spec.ts > signs in": {"failed", "passed"}, "e2e/b.spec.ts > lists": {"passed"}}))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Profile != ProfileIngested || parsed.Tree != treeB || parsed.Commit != commitB || len(parsed.Rows) != 2 ||
		parsed.Rows[0].Outcome != "flaky" || parsed.Rows[0].Retries != 1 || parsed.Rows[0].Granularity != GranularityTest ||
		parsed.Rows[1].Outcome != "passed" || parsed.Rows[1].Retries != 0 || !slices.Equal(parsed.Instances, []string{"playwright"}) {
		t.Fatalf("ingested rows %+v", parsed)
	}
	for _, bad := range []string{"", "{}", `{"schema":"test-run-evidence/0"}`, "not json"} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Fatalf("%q parsed", bad)
		}
	}
}

func stripDuration(row Row) Row {
	row.DurationMs = nil
	return row
}

// RVD-V1-009: with twenty records each holding only test T, the twenty-first removes the oldest;
// a record another test still needs stays.
func TestRVDV1009RetentionAt21Runs(t *testing.T) {
	s := testStore(t)
	var ids []string
	for run := range 21 {
		data := receipt(commitA, fmt.Sprintf("%040d", run), "go:m/t=PASS")
		if run == 1 {
			data = receipt(commitA, fmt.Sprintf("%040d", run), "go:m/t=PASS", "go:m/rare=PASS")
		}
		id, err := s.Append(data)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	entries := readLog(t, s)
	if len(entries) != 20 || entries[0].ID != ids[1] {
		t.Fatalf("log keeps %d records starting %s, want 20 starting %s", len(entries), entries[0].ID, ids[1])
	}
	if _, err := os.Stat(filepath.Join(s.Dir, recordsDir, ids[0]+".json")); !os.IsNotExist(err) {
		t.Fatalf("the oldest record was not removed: %v", err)
	}
	// A 22nd run of T removes nothing: record 1 is still needed by "rare", and T keeps its newest 20.
	if _, err := s.Append(receipt(commitA, fmt.Sprintf("%040d", 21), "go:m/t=PASS")); err != nil {
		t.Fatal(err)
	}
	entries = readLog(t, s)
	if len(entries) != 21 || entries[0].ID != ids[1] || entries[1].ID != ids[2] {
		t.Fatalf("record needed by another test was pruned: %d entries, first %s second %s", len(entries), entries[0].ID, entries[1].ID)
	}
}

// RVD-V1-009: when the records would exceed the cap the oldest go first and the newest stays.
func TestRVDV1009SizeCap(t *testing.T) {
	s := testStore(t)
	one := int64(len(receipt(commitA, treeA, "go:m/a=PASS")))
	s.CapBytes = 3*one + one/2
	var ids []string
	for run := range 5 {
		id, err := s.Append(receipt(commitA, fmt.Sprintf("%040d", run), fmt.Sprintf("go:m/%c=PASS", 'a'+run)))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	entries := readLog(t, s)
	if len(entries) != 3 || entries[0].ID != ids[2] || entries[2].ID != ids[4] {
		t.Fatalf("cap kept %+v", entries)
	}
	names, _ := listRecords(filepath.Join(s.Dir, recordsDir))
	if len(names) != 3 {
		t.Fatalf("records/ holds %v", names)
	}
	// A cap smaller than one record still keeps the newest.
	s.CapBytes = 1
	if _, err := s.Append(receipt(commitA, treeB, "go:m/z=PASS")); err != nil {
		t.Fatal(err)
	}
	if entries := readLog(t, s); len(entries) != 1 {
		t.Fatalf("cap below one record kept %d", len(entries))
	}
}

// RVD-V1-010: a record edited by hand is skipped and named with its tests; the others still read;
// reading changes nothing.
func TestRVDV1010CorruptRecordSkipped(t *testing.T) {
	s := testStore(t)
	good, err := s.Append(receipt(commitA, treeA, "go:m/a=PASS"))
	if err != nil {
		t.Fatal(err)
	}
	bad, err := s.Append(receipt(commitA, treeB, "go:m/b=FAIL"))
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(s.Dir, recordsDir, bad+".json")
	edited := bytes.Replace(receipt(commitA, treeB, "go:m/b=FAIL"), []byte("FAIL"), []byte("PASS"), 1)
	if err := os.WriteFile(name, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	before := treeDigest(t, s.Dir)
	snapshot, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Records) != 1 || snapshot.Records[0].Entry.ID != good || len(snapshot.Corrupt) != 1 ||
		snapshot.Corrupt[0].ID != bad || !slices.Equal(snapshot.CorruptTests(), []string{"go:m/b"}) ||
		!slices.Equal(snapshot.Reasons, []string{ReasonCorrupt}) || snapshot.LogUnreadable {
		t.Fatalf("snapshot %+v", snapshot)
	}
	if after := treeDigest(t, s.Dir); after != before {
		t.Fatal("a read changed the store")
	}
	// A record that does not parse is skipped the same way.
	if err := os.WriteFile(name, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if snapshot, _ := s.Read(); len(snapshot.Corrupt) != 1 {
		t.Fatalf("unparsable record not skipped: %+v", snapshot)
	}
}

// RVD-V1-010: an unreadable log makes readers list records by file time and print the reason, and
// the next writer rebuilds it from records/.
func TestRVDV1010UnreadableLogRebuilt(t *testing.T) {
	s := testStore(t)
	first, err := s.Append(receipt(commitA, treeA, "go:m/a=PASS"))
	if err != nil {
		t.Fatal(err)
	}
	// Distinct file times keep the fallback order deterministic.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(s.Dir, recordsDir, first+".json"), old, old); err != nil {
		t.Fatal(err)
	}
	second, err := s.Append(receipt(commitB, treeB, "go:m/a=FAIL"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir, logName), []byte("not a log line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.LogUnreadable || !slices.Equal(snapshot.Reasons, []string{ReasonCorrupt}) || len(snapshot.Records) != 2 ||
		snapshot.Records[0].Entry.ID != first || snapshot.Records[1].Entry.ID != second || snapshot.Latest()["go:m/a"].Outcome != "failed" {
		t.Fatalf("fallback snapshot %+v", snapshot)
	}
	third, err := s.Append(receipt(commitB, treeA, "go:m/c=PASS"))
	if err != nil {
		t.Fatal(err)
	}
	entries := readLog(t, s)
	if len(entries) != 3 || entries[0].ID != first || entries[1].ID != second || entries[2].ID != third || entries[0].Tree != treeA {
		t.Fatalf("rebuilt log %+v", entries)
	}
	if snapshot, _ := s.Read(); snapshot.LogUnreadable || len(snapshot.Reasons) != 0 {
		t.Fatalf("rebuilt log still unreadable: %+v", snapshot)
	}
	// A line being appended (no newline yet) is not read and is no corruption.
	if err := os.WriteFile(filepath.Join(s.Dir, logName), append(mustRead(t, filepath.Join(s.Dir, logName)), []byte(`{"id":"`)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if snapshot, _ := s.Read(); snapshot.LogUnreadable || len(snapshot.Records) != 3 {
		t.Fatalf("partial last line broke the read: %+v", snapshot)
	}
}

// RVD-V1-012: reading a missing store creates nothing.
func TestRVDV1012ReadMissingStoreWritesNothing(t *testing.T) {
	gitDir := filepath.Join(t.TempDir(), ".git")
	s := Store{Dir: Locate(gitDir)}
	snapshot, err := s.Read()
	if err != nil || snapshot.Present() {
		t.Fatalf("missing store read %+v %v", snapshot, err)
	}
	if _, err := os.Lstat(gitDir); !os.IsNotExist(err) {
		t.Fatal("reading a missing store created a directory")
	}
}

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// treeDigest names every file under dir with its mode, size, time and bytes.
func treeDigest(t *testing.T, dir string) string {
	t.Helper()
	var out strings.Builder
	err := filepath.Walk(dir, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fmt.Fprintf(&out, "%s %v %d %d\n", name, info.Mode(), info.Size(), info.ModTime().UnixNano())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			fmt.Fprintf(&out, "%s\n", ID(data))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// RVD-V1-002, RVD-V1-010: a writer that died mid-line leaves a log without a final newline. The
// next append rewrites the log rather than joining its line to the torn one, so the log stays
// readable and lists the new record.
func TestRVDV1002AppendAfterTornTail(t *testing.T) {
	s := testStore(t)
	first, err := s.Append(receipt(commitA, treeA, "go:m/a=PASS"))
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(s.Dir, logName)
	torn := append(mustRead(t, logPath), []byte(`{"id":"`+strings.Repeat("f", 20))...)
	if err := os.WriteFile(logPath, torn, 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := s.Append(receipt(commitB, treeB, "go:m/a=FAIL"))
	if err != nil {
		t.Fatal(err)
	}
	raw := mustRead(t, logPath)
	if !bytes.HasSuffix(raw, []byte("\n")) {
		t.Fatalf("log still torn:\n%s", raw)
	}
	entries := readLog(t, s)
	if len(entries) != 2 || entries[0].ID != first || entries[1].ID != second {
		t.Fatalf("log after torn tail %+v", entries)
	}
	snapshot, err := s.Read()
	if err != nil || snapshot.LogUnreadable || len(snapshot.Reasons) != 0 || len(snapshot.Records) != 2 ||
		snapshot.Latest()["go:m/a"].Outcome != "failed" {
		t.Fatalf("read after torn tail %+v %v", snapshot, err)
	}
}

// RVD-V1-010: an append waits a bounded time for a lock another writer holds, then fails with
// ErrLockBusy (the caller prints run-evidence-write-failed) and writes no record.
func TestRVDV1010LockWaitIsBounded(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("the advisory lock exists on linux and darwin only")
	}
	s := testStore(t)
	s.LockWait = 150 * time.Millisecond
	if err := makeStoreDirs(filepath.Dir(s.Dir), s.Dir, filepath.Join(s.Dir, recordsDir)); err != nil {
		t.Fatal(err)
	}
	unlock, err := lockFile(filepath.Join(s.Dir, lockName), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err = s.Append(receipt(commitA, treeA, "go:m/a=PASS"))
	waited := time.Since(started)
	unlock()
	if !errors.Is(err, ErrLockBusy) {
		t.Fatalf("append under a held lock returned %v", err)
	}
	if waited < s.LockWait || waited > s.LockWait+2*time.Second {
		t.Fatalf("append waited %s for a %s lock wait", waited, s.LockWait)
	}
	if names, _ := listRecords(filepath.Join(s.Dir, recordsDir)); len(names) != 0 {
		t.Fatalf("a refused append wrote records %v", names)
	}
	if _, err := s.Append(receipt(commitA, treeA, "go:m/a=PASS")); err != nil {
		t.Fatalf("append after release: %v", err)
	}
}

// RVD-V1-002: the writer opens neither the log nor the lock through a final symlink, so a linked
// file cannot carry the append outside the store.
func TestRVDV1002LinkedLogAndLockRefused(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("O_NOFOLLOW and the advisory lock exist on linux and darwin only")
	}
	for _, name := range []string{logName, lockName} {
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			if _, err := s.Append(receipt(commitA, treeA, "go:m/a=PASS")); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(t.TempDir(), "outside")
			if err := os.WriteFile(outside, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(s.Dir, name)
			if err := os.Remove(target); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, target); err != nil {
				t.Fatal(err)
			}
			_, err := s.Append(receipt(commitB, treeB, "go:m/b=PASS"))
			if name == lockName && err == nil {
				t.Fatal("append took a linked lock file")
			}
			if got := mustRead(t, outside); len(got) != 0 {
				t.Fatalf("append wrote through the linked %s: %q", name, got)
			}
		})
	}
}

// RVD-V1-010: a read of a damaged store is bounded. A log naming more than MaxReadRecords entries
// is read only for its newest MaxReadRecords; records past the byte budget are not read. Either is
// reported as run-evidence-corrupt, and the newest records are the ones kept.
func TestRVDV1010ReadIsBounded(t *testing.T) {
	t.Run("records", func(t *testing.T) {
		s := testStore(t)
		id, err := s.Append(receipt(commitA, treeA, "go:m/a=PASS"))
		if err != nil {
			t.Fatal(err)
		}
		line := mustRead(t, filepath.Join(s.Dir, logName))
		if err := os.WriteFile(filepath.Join(s.Dir, logName), bytes.Repeat(line, MaxReadRecords+5), 0o600); err != nil {
			t.Fatal(err)
		}
		snapshot, err := s.Read()
		if err != nil || len(snapshot.Records) != MaxReadRecords || snapshot.Records[0].Entry.ID != id ||
			!slices.Equal(snapshot.Reasons, []string{ReasonCorrupt}) {
			t.Fatalf("records %d reasons %v err %v", len(snapshot.Records), snapshot.Reasons, err)
		}
	})
	t.Run("bytes", func(t *testing.T) {
		saved := readSlack
		readSlack = 0
		t.Cleanup(func() { readSlack = saved })
		s := testStore(t)
		var ids []string
		for index := range 4 {
			id, err := s.Append(receipt(commitA, treeA, fmt.Sprintf("go:m/t%d=PASS", index)))
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		one := int64(len(receipt(commitA, treeA, "go:m/t0=PASS")))
		bounded := Store{Dir: s.Dir, CapBytes: 2*one + one/2}
		snapshot, err := bounded.Read()
		if err != nil || len(snapshot.Records) != 2 || snapshot.Records[0].Entry.ID != ids[2] || snapshot.Records[1].Entry.ID != ids[3] ||
			len(snapshot.Corrupt) != 0 || !slices.Equal(snapshot.Reasons, []string{ReasonCorrupt}) {
			t.Fatalf("bounded read %+v %v", snapshot, err)
		}
	})
	t.Run("unreadable log", func(t *testing.T) {
		s := testStore(t)
		if _, err := s.Append(receipt(commitA, treeA, "go:m/a=PASS")); err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(s.Dir, recordsDir)
		for index := range maxListEntries + 10 {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("junk-%d", index)), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(s.Dir, logName), []byte("not a log line\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		snapshot, err := s.Read()
		if err != nil || !snapshot.LogUnreadable || len(snapshot.Records) > 1 {
			t.Fatalf("fallback read %d records %v", len(snapshot.Records), err)
		}
	})
}
