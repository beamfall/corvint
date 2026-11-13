// SPDX-License-Identifier: Apache-2.0

package runevidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// countReads records every byte readBounded consumes for the rest of the test.
func countReads(t *testing.T) *int64 {
	t.Helper()
	var total int64
	saved := readCounted
	readCounted = func(n int64) { total += n }
	t.Cleanup(func() { readCounted = saved })
	return &total
}

// RVD-V1-010: a damaged store whose log names many records that are oversized and corrupt is read
// within the byte bound. An oversized file is skipped unread, so 600 sparse 9 MiB files cost no
// bytes where each used to cost a whole MaxRecordBytes read. They are listed as corrupt, the newest
// valid record is still read, and the total stays within the writer's cap plus one record.
func TestRVDV1010OversizedCorruptRecordsStayWithinTheReadBudget(t *testing.T) {
	s := testStore(t)
	validID, err := s.Append(receipt(commitA, treeA, "go:m/a=PASS"))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.Dir, recordsDir)
	var log bytes.Buffer
	const oversized = 600
	for index := range oversized {
		id := fmt.Sprintf("%064x", index+1)
		file, err := os.Create(filepath.Join(dir, id+".json"))
		if err != nil {
			t.Fatal(err)
		}
		// Sparse: the size is past MaxRecordBytes but the disk holds no data.
		if err := file.Truncate(MaxRecordBytes + 1<<20); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		line, err := json.Marshal(Entry{ID: id, Profile: ProfileReceipt, Instances: []string{}, Tests: []string{"go:m/x"}})
		if err != nil {
			t.Fatal(err)
		}
		log.Write(append(line, '\n'))
	}
	valid, err := json.Marshal(readLog(t, s)[0])
	if err != nil {
		t.Fatal(err)
	}
	log.Write(append(valid, '\n'))
	if err := os.WriteFile(filepath.Join(s.Dir, logName), log.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	read := countReads(t)
	snapshot, err := s.Read()
	if err != nil || len(snapshot.Records) != 1 || snapshot.Records[0].Entry.ID != validID ||
		len(snapshot.Corrupt) != oversized || !slices.Equal(snapshot.Reasons, []string{ReasonCorrupt}) {
		t.Fatalf("records %d corrupt %d reasons %v err %v", len(snapshot.Records), len(snapshot.Corrupt), snapshot.Reasons, err)
	}
	// The log itself is read too; the records cost only the one valid record.
	logBytes := int64(log.Len())
	if *read > logBytes+int64(len(receipt(commitA, treeA, "go:m/a=PASS")))+1 {
		t.Fatalf("read %d bytes for a log of %d and one record", *read, logBytes)
	}
	if *read > s.capBytes()+readSlack+maxLogBytes {
		t.Fatalf("read %d bytes, over the bound", *read)
	}
}

// RVD-V1-010: readBounded hands back what a failed read consumed, at most limit+1 bytes, so the
// reader charges a file that grew past its limit after the stat against its budget.
func TestRVDV1010FailedReadReportsWhatItConsumed(t *testing.T) {
	name := filepath.Join(t.TempDir(), "grown.json")
	if err := os.WriteFile(name, bytes.Repeat([]byte("x"), 100), 0o600); err != nil {
		t.Fatal(err)
	}
	read := countReads(t)
	data, err := readBounded(name, 10)
	if err == nil || len(data) != 11 || *read != 11 {
		t.Fatalf("data %d bytes, counted %d, err %v", len(data), *read, err)
	}
}
