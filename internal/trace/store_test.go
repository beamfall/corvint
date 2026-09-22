package trace

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreReadAbsentAndAppendRoundTrip(t *testing.T) {
	root := t.TempDir()
	revisions := map[string]Revision{testRevision: {TreeRevision: strings.Repeat("a", 40), TrackedPaths: []string{"a.go", "z.go"}}}
	store := newTestStore(t, root, revisions, func() error { return nil })
	records, state, err := store.Read()
	if err != nil || state != StateAbsent || len(records) != 0 {
		t.Fatalf("Read() = (%+v, %q, %v)", records, state, err)
	}
	record := mustRecord(t, Input{Revision: testRevision, Task: "café ☃ 😀", OpenedPaths: []string{"z.go", "a.go"}, ChangedPaths: []string{"a.go"}, Verification: []string{"go test ./...", "git diff --check"}, Outcome: "passed"}, []string{"a.go", "z.go"})
	written, err := store.Append(record)
	if err != nil || !written {
		t.Fatalf("Append() = (%v, %v)", written, err)
	}
	written, err = store.Append(record)
	if err != nil || written {
		t.Fatalf("idempotent Append() = (%v, %v)", written, err)
	}
	records, state, err = store.Read()
	if err != nil || state != StateReady || len(records) != 1 || records[0].TraceID != record.TraceID {
		t.Fatalf("Read() = (%+v, %q, %v)", records, state, err)
	}
	want := "{\"changed_paths\":[\"a.go\"],\"opened_paths\":[\"a.go\",\"z.go\"],\"outcome\":\"passed\",\"revision\":\"0123456789abcdef0123456789abcdef01234567\",\"schema_version\":1,\"task\":\"caf\\u00e9 \\u2603 \\ud83d\\ude00\",\"trace_id\":\"5be94b69b7df0706b2aa769fcfc9d46d1fd439d7845023521681cd22ce0951bf\",\"verification\":[\"git diff --check\",\"go test ./...\"]}\n"
	got, err := os.ReadFile(StorePath(root, testRevision))
	if err != nil || string(got) != want {
		t.Fatalf("stored bytes = %q, error = %v", got, err)
	}
	assertMode(t, filepath.Join(root, ".context-corvint"), 0o700)
	assertMode(t, filepath.Join(root, ".context-corvint", "traces"), 0o700)
	assertMode(t, StorePath(root, testRevision), 0o600)
}

func TestNewStoreRejectsInvalidObjectIdentitySets(t *testing.T) {
	tests := []struct {
		name      string
		revisions map[string]Revision
	}{
		{"intermediate length", map[string]Revision{strings.Repeat("a", 41): {TreeRevision: strings.Repeat("b", 41)}}},
		{"mixed object format", map[string]Revision{
			strings.Repeat("a", 40): {TreeRevision: strings.Repeat("b", 40)},
			strings.Repeat("c", 64): {TreeRevision: strings.Repeat("d", 64)},
		}},
		{"commit tree mismatch", map[string]Revision{strings.Repeat("a", 40): {TreeRevision: strings.Repeat("b", 64)}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewStore(t.TempDir(), test.revisions, func() error { return nil }); err == nil {
				t.Fatal("invalid object identity set accepted")
			}
		})
	}
}

func TestStoreResolvesSymlinkedRepositoryRoot(t *testing.T) {
	realRoot := t.TempDir()
	linkParent := t.TempDir()
	linkedRoot := filepath.Join(linkParent, "repository")
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Fatal(err)
	}
	realRoot, err := filepath.EvalSymlinks(realRoot)
	if err != nil {
		t.Fatal(err)
	}
	store := newTestStore(t, linkedRoot, nil, func() error { return nil })
	if store.root != realRoot {
		t.Fatalf("store root = %q, want %q", store.root, realRoot)
	}
	want := filepath.Join(realRoot, ".context-corvint", "traces", zeroRevision+".jsonl")
	if got := StorePath(linkedRoot, zeroRevision); got != want {
		t.Fatalf("StorePath() = %q, want %q", got, want)
	}
}

func TestStoreReadSortsAndUsesFrozenRevisionSnapshot(t *testing.T) {
	root := t.TempDir()
	firstRevision := strings.Repeat("1", 40)
	secondRevision := strings.Repeat("2", 40)
	revisions := map[string]Revision{
		firstRevision:  {TreeRevision: strings.Repeat("a", 40), TrackedPaths: []string{"a.go"}},
		secondRevision: {TreeRevision: strings.Repeat("b", 40), TrackedPaths: []string{"b.go"}},
	}
	store := newTestStore(t, root, revisions, func() error { return nil })
	revisions[firstRevision] = Revision{TreeRevision: "invalid"}
	first := mustRecord(t, Input{Revision: firstRevision, Task: "z task", ChangedPaths: []string{"a.go"}, Outcome: "passed"}, []string{"a.go"})
	second := mustRecord(t, Input{Revision: secondRevision, Task: "a task", ChangedPaths: []string{"b.go"}, Outcome: "failed"}, []string{"b.go"})
	if _, err := store.Append(first); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(second); err != nil {
		t.Fatal(err)
	}
	records, _, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].TraceID > records[1].TraceID {
		t.Fatalf("records not sorted by trace_id: %+v", records)
	}
}

func TestStoreReadTypesCandidateAndFileChangesAsDrift(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{"candidate set", func(t *testing.T, root string) {
			writeTraceFixture(t, root, strings.Repeat("c", 40), nil)
		}},
		{"file bytes", func(t *testing.T, root string) {
			path := StorePath(root, testRevision)
			file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.WriteString("\n"); err != nil {
				file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			revisions := map[string]Revision{testRevision: {TreeRevision: strings.Repeat("a", 40), TrackedPaths: []string{"a.go"}}}
			store := newTestStore(t, root, revisions, func() error { return nil })
			record := mustRecord(t, Input{Revision: testRevision, Task: "task", ChangedPaths: []string{"a.go"}, Outcome: "passed"}, []string{"a.go"})
			if written, err := store.Append(record); err != nil || !written {
				t.Fatalf("Append() = (%v, %v)", written, err)
			}
			store.hooks.duringRead = func() { test.mutate(t, root) }
			_, _, err := store.Read()
			if err == nil || !IsDrift(err) {
				t.Fatalf("Read() error = %v, drift=%v", err, IsDrift(err))
			}
		})
	}
}

func TestStoreReadRejectsWholeStoreViolations(t *testing.T) {
	canonicalRevision := strings.Repeat("b", 40)
	legacyRevision := strings.Repeat("a", 40)
	tree := legacyRevision
	revisions := map[string]Revision{
		canonicalRevision: {TreeRevision: tree, TrackedPaths: []string{"a.go"}},
		legacyRevision:    {TreeRevision: tree, Legacy: true, TrackedPaths: []string{"a.go"}},
	}
	tests := []struct {
		name    string
		prepare func(*testing.T, string)
		wantErr string
	}{
		{
			name: "duplicate ids",
			prepare: func(t *testing.T, root string) {
				record := mustRecord(t, Input{Revision: canonicalRevision, Task: "task", ChangedPaths: []string{"a.go"}, Outcome: "passed"}, []string{"a.go"})
				row, _ := Encode(record)
				writeTraceFixture(t, root, canonicalRevision, append(append([]byte(nil), row...), row...))
			},
			wantErr: "duplicate trace ids",
		},
		{
			name: "incomplete legacy migration",
			prepare: func(t *testing.T, root string) {
				for _, revision := range []string{legacyRevision, canonicalRevision} {
					record := mustRecord(t, Input{Revision: revision, Task: "same task", ChangedPaths: []string{"a.go"}, Outcome: "passed"}, []string{"a.go"})
					row, _ := Encode(record)
					writeTraceFixture(t, root, revision, row)
				}
			},
			wantErr: "incomplete legacy migration",
		},
		{
			name: "unreachable candidate",
			prepare: func(t *testing.T, root string) {
				writeTraceFixture(t, root, strings.Repeat("c", 40), nil)
			},
			wantErr: "unreachable revision",
		},
		{
			name: "directory entry bound",
			prepare: func(t *testing.T, root string) {
				directory := filepath.Join(root, ".context-corvint", "traces")
				if err := os.MkdirAll(directory, 0o700); err != nil {
					t.Fatal(err)
				}
				for index := 0; index <= MaxTraceFiles; index++ {
					if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("junk-%04d", index)), nil, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			},
			wantErr: "directory entries",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			test.prepare(t, root)
			store := newTestStore(t, root, revisions, func() error { return nil })
			_, _, err := store.Read()
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestStoreRejectsSymlinkAndHardlinkCandidates(t *testing.T) {
	revision := strings.Repeat("c", 40)
	revisions := map[string]Revision{revision: {TreeRevision: strings.Repeat("d", 40)}}
	tests := []struct {
		name    string
		prepare func(*testing.T, string, string)
	}{
		{"symlink", func(t *testing.T, target, outside string) {
			if err := os.WriteFile(outside, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, target); err != nil {
				t.Fatal(err)
			}
		}},
		{"hardlink", func(t *testing.T, target, outside string) {
			if err := os.WriteFile(outside, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Link(outside, target); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, ".context-corvint", "traces")
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			test.prepare(t, filepath.Join(directory, revision+".jsonl"), filepath.Join(root, "outside"))
			store := newTestStore(t, root, revisions, func() error { return nil })
			_, _, err := store.Read()
			if err == nil || !strings.Contains(err.Error(), "unsafe local trace") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestAppendEnforcesGlobalRowBound(t *testing.T) {
	root := t.TempDir()
	revision := strings.Repeat("d", 40)
	rows := make([]byte, 0, MaxTraces*260)
	for index := 0; index < MaxTraces; index++ {
		record := mustRecord(t, Input{Revision: revision, Task: fmt.Sprintf("task %d", index), Outcome: "passed"}, nil)
		row, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row...)
	}
	writeTraceFixture(t, root, revision, rows)
	store := newTestStore(t, root, map[string]Revision{revision: {TreeRevision: strings.Repeat("e", 40)}}, func() error { return nil })
	next := mustRecord(t, Input{Revision: revision, Task: "one too many", Outcome: "passed"}, nil)
	written, err := store.Append(next)
	if err == nil || written || !strings.Contains(err.Error(), "exceeds 1000 rows") {
		t.Fatalf("Append() = (%v, %v)", written, err)
	}
}

// LTA-V0-003.
func TestAppendEvictsWholeFilesInRetentionOrder(t *testing.T) {
	root := t.TempDir()
	target := strings.Repeat("a", 40)
	unreachable := strings.Repeat("b", 40)
	noPassedFar := strings.Repeat("c", 40)
	noPassedNear := strings.Repeat("d", 40)
	passedFar := strings.Repeat("e", 40)
	passedNear := strings.Repeat("f", 40)
	revisions := map[string]Revision{
		target:       {TreeRevision: strings.Repeat("1", 40), AncestryDistance: 0},
		noPassedFar:  {TreeRevision: strings.Repeat("2", 40), AncestryDistance: 4},
		noPassedNear: {TreeRevision: strings.Repeat("3", 40), AncestryDistance: 1},
		passedFar:    {TreeRevision: strings.Repeat("4", 40), AncestryDistance: 3},
		passedNear:   {TreeRevision: strings.Repeat("5", 40), AncestryDistance: 2},
	}
	rows := make([]byte, 0, 995*260)
	for index := 0; index < 995; index++ {
		record := mustRecord(t, Input{Revision: target, Task: fmt.Sprintf("target task %d", index), Outcome: "passed"}, nil)
		row, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row...)
	}
	writeTraceFixture(t, root, target, rows)
	for _, fixture := range []struct {
		revision string
		outcome  string
	}{
		{noPassedFar, "failed"},
		{noPassedNear, "blocked"},
		{passedFar, "passed"},
		{passedNear, "passed"},
	} {
		record := mustRecord(t, Input{Revision: fixture.revision, Task: "candidate " + fixture.revision, Outcome: fixture.outcome}, nil)
		row, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		writeTraceFixture(t, root, fixture.revision, row)
	}
	store := newTestStore(t, root, revisions, func() error { return nil })
	fill := mustRecord(t, Input{Revision: target, Task: "fill cap", Outcome: "passed"}, nil)
	if written, err := store.Append(fill); err != nil || !written {
		t.Fatalf("fill Append() = (%v, %v)", written, err)
	}
	for _, revision := range []string{noPassedFar, noPassedNear, passedFar, passedNear} {
		if _, err := os.Stat(StorePath(root, revision)); err != nil {
			t.Fatalf("append below cap evicted %s: %v", revision, err)
		}
	}
	if err := os.WriteFile(StorePath(root, target), rows, 0o600); err != nil {
		t.Fatal(err)
	}
	unreachableRecord := mustRecord(t, Input{
		Revision: unreachable, Task: "unreachable candidate", ChangedPaths: []string{"historical.go"}, Outcome: "passed",
	}, []string{"historical.go"})
	unreachableRow, err := Encode(unreachableRecord)
	if err != nil {
		t.Fatal(err)
	}
	writeTraceFixture(t, root, unreachable, unreachableRow)
	for index, revision := range []string{unreachable, noPassedFar, noPassedNear, passedFar, passedNear} {
		next := mustRecord(t, Input{Revision: target, Task: fmt.Sprintf("reclaim %d", index), Outcome: "passed"}, nil)
		if written, err := store.Append(next); err != nil || !written {
			t.Fatalf("reclaim Append() = (%v, %v)", written, err)
		}
		if _, err := os.Stat(StorePath(root, revision)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("retention did not evict whole file %s: %v", revision, err)
		}
	}
	if _, err := os.Stat(StorePath(root, target)); err != nil {
		t.Fatalf("retention evicted append target: %v", err)
	}
	overflow := mustRecord(t, Input{Revision: target, Task: "target alone overflow", Outcome: "passed"}, nil)
	written, err := store.Append(overflow)
	if err == nil || written || !strings.Contains(err.Error(), "record at a newer revision or remove the target trace file") {
		t.Fatalf("target-only Append() = (%v, %v)", written, err)
	}
}

// LTA-V0-003.
func TestAppendRetentionPreservesDirectoryEntryCap(t *testing.T) {
	root := t.TempDir()
	revisions := make(map[string]Revision, MaxTraceFiles)
	for distance := 1; distance < MaxTraceFiles; distance++ {
		revision := fmt.Sprintf("%040x", distance)
		revisions[revision] = Revision{TreeRevision: strings.Repeat("1", 40), AncestryDistance: distance}
		writeTraceFixture(t, root, revision, nil)
	}
	target := fmt.Sprintf("%040x", MaxTraceFiles)
	revisions[target] = Revision{TreeRevision: strings.Repeat("1", 40)}
	lock := filepath.Join(root, ".context-corvint", "traces", ".trace-operation.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	store := newTestStore(t, root, revisions, func() error { return nil })
	record := mustRecord(t, Input{Revision: target, Task: "new append target", Outcome: "passed"}, nil)
	if written, err := store.Append(record); err != nil || !written {
		t.Fatalf("Append() = (%v, %v)", written, err)
	}
	farthest := fmt.Sprintf("%040x", MaxTraceFiles-1)
	if _, err := os.Stat(StorePath(root, farthest)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("farthest trace was not evicted: %v", err)
	}
	records, state, err := store.Read()
	if err != nil || state != StateReady || len(records) != 1 || records[0].TraceID != record.TraceID {
		t.Fatalf("Read() = (%+v, %q, %v)", records, state, err)
	}
}

// LTA-V0-003.
func TestAppendRetentionPublishFailureRestoresCandidateFiles(t *testing.T) {
	root := t.TempDir()
	target := strings.Repeat("a", 40)
	failed := strings.Repeat("b", 40)
	passed := strings.Repeat("c", 40)
	revisions := map[string]Revision{
		target: {TreeRevision: strings.Repeat("1", 40)},
		failed: {TreeRevision: strings.Repeat("2", 40), AncestryDistance: 2},
		passed: {TreeRevision: strings.Repeat("3", 40), AncestryDistance: 1},
	}
	rows := make([]byte, 0, 998*260)
	for index := 0; index < 998; index++ {
		record := mustRecord(t, Input{Revision: target, Task: fmt.Sprintf("target task %d", index), Outcome: "passed"}, nil)
		row, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row...)
	}
	writeTraceFixture(t, root, target, rows)
	for revision, outcome := range map[string]string{failed: "failed", passed: "passed"} {
		record := mustRecord(t, Input{Revision: revision, Task: "candidate " + revision, Outcome: outcome}, nil)
		row, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		writeTraceFixture(t, root, revision, row)
	}
	before := make(map[string][]byte, len(revisions))
	for revision := range revisions {
		data, err := os.ReadFile(StorePath(root, revision))
		if err != nil {
			t.Fatal(err)
		}
		before[revision] = data
	}
	store := newTestStore(t, root, revisions, func() error { return nil })
	store.hooks.rename = func(root *os.Root, oldName, newName string) error {
		if oldName == ".record-"+target+".tmp" && newName == target+".jsonl" {
			return errors.New("injected publish failure")
		}
		return root.Rename(oldName, newName)
	}
	next := mustRecord(t, Input{Revision: target, Task: "must not publish", Outcome: "passed"}, nil)
	if written, err := store.Append(next); err == nil || written {
		t.Fatalf("Append() = (%v, %v)", written, err)
	}
	for revision, want := range before {
		got, err := os.ReadFile(StorePath(root, revision))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("rollback changed %s: error=%v\n got=%q\nwant=%q", revision, err, got, want)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, ".context-corvint", "traces"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".evict-") {
			t.Fatalf("staged eviction survived rollback: %s", entry.Name())
		}
	}
	collisionRecord := mustRecord(t, Input{Revision: target, Task: "collision must fail closed", Outcome: "passed"}, nil)
	collisionName := stagedEvictionName(target, collisionRecord.TraceID, failed+".jsonl")
	collisionPath := filepath.Join(root, ".context-corvint", "traces", collisionName)
	collisionBytes := []byte("pre-existing collision")
	if err := os.WriteFile(collisionPath, collisionBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if written, err := store.Append(collisionRecord); err == nil || written || !strings.Contains(err.Error(), "rollback collision") {
		t.Fatalf("collision Append() = (%v, %v)", written, err)
	}
	if got, err := os.ReadFile(collisionPath); err != nil || !bytes.Equal(got, collisionBytes) {
		t.Fatalf("collision changed: error=%v got=%q", err, got)
	}
	if got, err := os.ReadFile(StorePath(root, failed)); err != nil || !bytes.Equal(got, before[failed]) {
		t.Fatalf("collision changed candidate: error=%v got=%q", err, got)
	}
}

// LTA-V0-003.
func TestAppendRejectsMalformedUnreachableRows(t *testing.T) {
	root := t.TempDir()
	target := strings.Repeat("a", 40)
	unreachable := strings.Repeat("b", 40)
	writeTraceFixture(t, root, unreachable, []byte("{}\n"))
	store := newTestStore(t, root, map[string]Revision{
		target: {TreeRevision: strings.Repeat("1", 40)},
	}, func() error { return nil })
	record := mustRecord(t, Input{Revision: target, Task: "must reject malformed unreachable row", Outcome: "passed"}, nil)
	written, err := store.Append(record)
	if err == nil || written || !strings.Contains(err.Error(), "invalid local trace fields") {
		t.Fatalf("Append() = (%v, %v)", written, err)
	}
	if got, err := os.ReadFile(StorePath(root, unreachable)); err != nil || string(got) != "{}\n" {
		t.Fatalf("malformed unreachable file changed: error=%v bytes=%q", err, got)
	}
}

// LTA-V0-003.
func TestAppendRecoversInterruptedRetention(t *testing.T) {
	for _, published := range []bool{false, true} {
		t.Run(fmt.Sprintf("published=%t", published), func(t *testing.T) {
			root := t.TempDir()
			target := strings.Repeat("a", 40)
			candidate := strings.Repeat("b", 40)
			revisions := map[string]Revision{
				target:    {TreeRevision: strings.Repeat("1", 40)},
				candidate: {TreeRevision: strings.Repeat("2", 40), AncestryDistance: 1},
			}
			old := mustRecord(t, Input{Revision: target, Task: "old target row", Outcome: "passed"}, nil)
			oldRow, err := Encode(old)
			if err != nil {
				t.Fatal(err)
			}
			interrupted := mustRecord(t, Input{Revision: target, Task: "interrupted row", Outcome: "passed"}, nil)
			interruptedRow, err := Encode(interrupted)
			if err != nil {
				t.Fatal(err)
			}
			candidateRecord := mustRecord(t, Input{Revision: candidate, Task: "eviction candidate", Outcome: "failed"}, nil)
			candidateRow, err := Encode(candidateRecord)
			if err != nil {
				t.Fatal(err)
			}
			targetRows := append([]byte(nil), oldRow...)
			if published {
				targetRows = append(targetRows, interruptedRow...)
			}
			writeTraceFixture(t, root, target, targetRows)
			hidden := stagedEvictionName(target, interrupted.TraceID, candidate+".jsonl")
			if err := os.WriteFile(filepath.Join(root, ".context-corvint", "traces", hidden), candidateRow, 0o600); err != nil {
				t.Fatal(err)
			}
			if !published {
				stagedTarget := append(append([]byte(nil), oldRow...), interruptedRow...)
				if err := os.WriteFile(filepath.Join(root, ".context-corvint", "traces", ".record-"+target+".tmp"), stagedTarget, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			store := newTestStore(t, root, revisions, func() error { return nil })
			if _, _, err := store.Read(); !errors.Is(err, errStagedEvictionResidue) {
				t.Fatalf("Read() error=%v, want staged residue refusal", err)
			}
			retry := mustRecord(t, Input{Revision: target, Task: "recovery append", Outcome: "passed"}, nil)
			if written, err := store.Append(retry); err != nil || !written {
				t.Fatalf("Append() = (%v, %v)", written, err)
			}
			if _, err := os.Stat(filepath.Join(root, ".context-corvint", "traces", hidden)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("staged eviction survived recovery: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, ".context-corvint", "traces", ".record-"+target+".tmp")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("staged target survived recovery: %v", err)
			}
			_, candidateErr := os.Stat(StorePath(root, candidate))
			if published && !errors.Is(candidateErr, os.ErrNotExist) {
				t.Fatalf("published eviction was not cleared: %v", candidateErr)
			}
			if !published {
				got, err := os.ReadFile(StorePath(root, candidate))
				if err != nil || !bytes.Equal(got, candidateRow) {
					t.Fatalf("unpublished eviction was not restored: error=%v got=%q", err, got)
				}
			}
			records, state, err := store.Read()
			if err != nil || state != StateReady {
				t.Fatalf("Read() after recovery = (%+v, %q, %v)", records, state, err)
			}
			ids := make(map[string]struct{}, len(records))
			for _, record := range records {
				ids[record.TraceID] = struct{}{}
			}
			if _, ok := ids[retry.TraceID]; !ok {
				t.Fatal("recovery append was not published")
			}
			_, interruptedPresent := ids[interrupted.TraceID]
			if interruptedPresent != published {
				t.Fatalf("interrupted row present=%t, want %t", interruptedPresent, published)
			}
		})
	}
}

// LTA-V0-003.
func TestAppendRecoversLoneStagedTarget(t *testing.T) {
	root := t.TempDir()
	revision := strings.Repeat("a", 40)
	staged := mustRecord(t, Input{Revision: revision, Task: "staged before eviction", Outcome: "passed"}, nil)
	stagedRow, err := Encode(staged)
	if err != nil {
		t.Fatal(err)
	}
	writeTraceFixture(t, root, revision, nil)
	temporary := filepath.Join(root, ".context-corvint", "traces", ".record-"+revision+".tmp")
	if err := os.WriteFile(temporary, stagedRow, 0o600); err != nil {
		t.Fatal(err)
	}
	store := newTestStore(t, root, map[string]Revision{
		revision: {TreeRevision: strings.Repeat("1", 40)},
	}, func() error { return nil })
	if _, _, err := store.Read(); !errors.Is(err, errStagedEvictionResidue) {
		t.Fatalf("Read() error=%v, want staged residue refusal", err)
	}
	if got, err := os.ReadFile(temporary); err != nil || !bytes.Equal(got, stagedRow) {
		t.Fatalf("Read() mutated staged target: error=%v got=%q", err, got)
	}
	retry := mustRecord(t, Input{Revision: revision, Task: "retry after lone staging", Outcome: "passed"}, nil)
	if written, err := store.Append(retry); err != nil || !written {
		t.Fatalf("Append() = (%v, %v)", written, err)
	}
	if _, err := os.Stat(temporary); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged target survived recovery: %v", err)
	}
}

// LTA-V0-003: interrupted-append recovery is a trace-store mutation and must
// revalidate repository stability before removing staged bytes.
func TestAppendRepositoryDriftPreventsInterruptedRecoveryMutation(t *testing.T) {
	root := t.TempDir()
	revision := strings.Repeat("a", 40)
	staged := mustRecord(t, Input{Revision: revision, Task: "staged before drift", Outcome: "passed"}, nil)
	stagedRow, err := Encode(staged)
	if err != nil {
		t.Fatal(err)
	}
	writeTraceFixture(t, root, revision, nil)
	temporary := filepath.Join(root, ".context-corvint", "traces", ".record-"+revision+".tmp")
	if err := os.WriteFile(temporary, stagedRow, 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	store := newTestStore(t, root, map[string]Revision{
		revision: {TreeRevision: strings.Repeat("1", 40)},
	}, func() error {
		calls++
		if calls >= 2 {
			return errors.New("injected repository drift")
		}
		return nil
	})
	retry := mustRecord(t, Input{Revision: revision, Task: "retry after drift", Outcome: "passed"}, nil)
	if written, err := store.Append(retry); err == nil || written {
		t.Fatalf("Append() = (%v, %v), want drift refusal", written, err)
	}
	if got, err := os.ReadFile(temporary); err != nil || !bytes.Equal(got, stagedRow) {
		t.Fatalf("drifted append mutated staged bytes: error=%v got=%q", err, got)
	}
}

// LTA-V0-003.
func TestAppendInterruptedRecoveryRemainsRetryable(t *testing.T) {
	root := t.TempDir()
	target := strings.Repeat("a", 40)
	candidate := strings.Repeat("b", 40)
	old := mustRecord(t, Input{Revision: target, Task: "old target row", Outcome: "passed"}, nil)
	oldRow, err := Encode(old)
	if err != nil {
		t.Fatal(err)
	}
	interrupted := mustRecord(t, Input{Revision: target, Task: "interrupted row", Outcome: "passed"}, nil)
	interruptedRow, err := Encode(interrupted)
	if err != nil {
		t.Fatal(err)
	}
	candidateRecord := mustRecord(t, Input{Revision: candidate, Task: "eviction candidate", Outcome: "failed"}, nil)
	candidateRow, err := Encode(candidateRecord)
	if err != nil {
		t.Fatal(err)
	}
	writeTraceFixture(t, root, target, oldRow)
	hidden := stagedEvictionName(target, interrupted.TraceID, candidate+".jsonl")
	if err := os.WriteFile(filepath.Join(root, ".context-corvint", "traces", hidden), candidateRow, 0o600); err != nil {
		t.Fatal(err)
	}
	temporary := filepath.Join(root, ".context-corvint", "traces", ".record-"+target+".tmp")
	if err := os.WriteFile(temporary, append(append([]byte(nil), oldRow...), interruptedRow...), 0o600); err != nil {
		t.Fatal(err)
	}
	store := newTestStore(t, root, map[string]Revision{
		target:    {TreeRevision: strings.Repeat("1", 40)},
		candidate: {TreeRevision: strings.Repeat("2", 40), AncestryDistance: 1},
	}, func() error { return nil })
	store.hooks.afterRecoveryTargetCleanup = func() error { return errors.New("injected recovery interruption") }
	retry := mustRecord(t, Input{Revision: target, Task: "retry interrupted recovery", Outcome: "passed"}, nil)
	if written, err := store.Append(retry); err == nil || written || !strings.Contains(err.Error(), "injected recovery interruption") {
		t.Fatalf("faulted Append() = (%v, %v)", written, err)
	}
	if _, err := os.Stat(temporary); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged target survived cleanup boundary: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, ".context-corvint", "traces", hidden)); err != nil || !bytes.Equal(got, candidateRow) {
		t.Fatalf("recovery marker lost at cleanup boundary: error=%v got=%q", err, got)
	}
	if _, err := os.Stat(StorePath(root, candidate)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("candidate restored before retry: %v", err)
	}

	store.hooks.afterRecoveryTargetCleanup = func() error { return nil }
	if written, err := store.Append(retry); err != nil || !written {
		t.Fatalf("retry Append() = (%v, %v)", written, err)
	}
	if got, err := os.ReadFile(StorePath(root, candidate)); err != nil || !bytes.Equal(got, candidateRow) {
		t.Fatalf("candidate not restored on retry: error=%v got=%q", err, got)
	}
	if _, err := os.Stat(filepath.Join(root, ".context-corvint", "traces", hidden)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recovery marker survived retry: %v", err)
	}
}

// LTA-V0-003.
func TestAppendRetentionPostPublishSyncFailureRecoversOnRetry(t *testing.T) {
	root := t.TempDir()
	target := strings.Repeat("a", 40)
	candidate := strings.Repeat("b", 40)
	revisions := map[string]Revision{
		target:    {TreeRevision: strings.Repeat("1", 40)},
		candidate: {TreeRevision: strings.Repeat("2", 40), AncestryDistance: 1},
	}
	rows := make([]byte, 0, 999*260)
	for index := 0; index < 999; index++ {
		record := mustRecord(t, Input{Revision: target, Task: fmt.Sprintf("target task %d", index), Outcome: "passed"}, nil)
		row, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row...)
	}
	writeTraceFixture(t, root, target, rows)
	candidateRecord := mustRecord(t, Input{Revision: candidate, Task: "eviction candidate", Outcome: "failed"}, nil)
	candidateRow, err := Encode(candidateRecord)
	if err != nil {
		t.Fatal(err)
	}
	writeTraceFixture(t, root, candidate, candidateRow)
	store := newTestStore(t, root, revisions, func() error { return nil })
	store.hooks.publishSync = func(*os.File) error { return errors.New("injected post-publish sync failure") }
	next := mustRecord(t, Input{Revision: target, Task: "published before sync failure", Outcome: "passed"}, nil)
	written, err := store.Append(next)
	if err == nil || !written || !strings.Contains(err.Error(), "cannot sync local trace directory") {
		t.Fatalf("Append() = (%v, %v)", written, err)
	}
	nextRow, err := Encode(next)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(StorePath(root, target)); err != nil || !bytes.Equal(got, append(append([]byte(nil), rows...), nextRow...)) {
		t.Fatalf("published target mismatch: error=%v", err)
	}
	hidden := stagedEvictionName(target, next.TraceID, candidate+".jsonl")
	if got, err := os.ReadFile(filepath.Join(root, ".context-corvint", "traces", hidden)); err != nil || !bytes.Equal(got, candidateRow) {
		t.Fatalf("recovery residue mismatch: error=%v got=%q", err, got)
	}
	if _, _, err := store.Read(); !errors.Is(err, errStagedEvictionResidue) {
		t.Fatalf("Read() error=%v, want staged residue refusal", err)
	}
	written, err = store.Append(next)
	if err != nil || written {
		t.Fatalf("retry Append() = (%v, %v)", written, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".context-corvint", "traces", hidden)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retry did not clear recovery residue: %v", err)
	}
	records, state, err := store.Read()
	if err != nil || state != StateReady || len(records) != MaxTraces {
		t.Fatalf("Read() after retry = (%d records, %q, %v)", len(records), state, err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".context-corvint", "traces"))
	if err != nil || len(entries) > MaxTraceFiles {
		t.Fatalf("final directory entries=%d error=%v", len(entries), err)
	}
}

func TestAppendRollbackPreservesOldBytes(t *testing.T) {
	tests := []struct {
		name  string
		hook  func(*Store)
		check func() error
	}{
		{
			name: "partial write failure",
			hook: func(store *Store) {
				calls := 0
				store.hooks.write = func(file *os.File, data []byte) (int, error) {
					calls++
					if calls == 1 {
						return file.Write(data[:len(data)/2])
					}
					return 0, errors.New("injected write failure")
				}
			},
			check: func() error { return nil },
		},
		{
			name: "fsync failure",
			hook: func(store *Store) {
				calls := 0
				store.hooks.sync = func(file *os.File) error {
					calls++
					if calls == 1 {
						return errors.New("injected sync failure")
					}
					return file.Sync()
				}
			},
			check: func() error { return nil },
		},
		{
			name: "repository drift",
			hook: func(*Store) {},
			check: func() func() error {
				calls := 0
				return func() error {
					calls++
					if calls == 3 {
						return errors.New("injected drift")
					}
					return nil
				}
			}(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			revision := strings.Repeat("f", 40)
			revisions := map[string]Revision{revision: {TreeRevision: strings.Repeat("a", 40)}}
			initialStore := newTestStore(t, root, revisions, func() error { return nil })
			first := mustRecord(t, Input{Revision: revision, Task: "first", Outcome: "passed"}, nil)
			if written, err := initialStore.Append(first); err != nil || !written {
				t.Fatalf("initial Append() = (%v, %v)", written, err)
			}
			before, err := os.ReadFile(StorePath(root, revision))
			if err != nil {
				t.Fatal(err)
			}
			store := newTestStore(t, root, revisions, test.check)
			test.hook(store)
			second := mustRecord(t, Input{Revision: revision, Task: "second", Outcome: "failed"}, nil)
			if written, err := store.Append(second); err == nil || written {
				t.Fatalf("faulted Append() = (%v, %v)", written, err)
			}
			after, err := os.ReadFile(StorePath(root, revision))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, before) {
				t.Fatalf("rollback changed bytes\n got: %q\nwant: %q", after, before)
			}
		})
	}
}

func TestAppendRejectsOperationLockPathReplacement(t *testing.T) {
	root := t.TempDir()
	revision := strings.Repeat("9", 40)
	revisions := map[string]Revision{revision: {TreeRevision: strings.Repeat("8", 40)}}
	store := newTestStore(t, root, revisions, func() error { return nil })
	first := mustRecord(t, Input{Revision: revision, Task: "first", Outcome: "passed"}, nil)
	if written, err := store.Append(first); err != nil || !written {
		t.Fatalf("initial Append() = (%v, %v)", written, err)
	}
	before, err := os.ReadFile(StorePath(root, revision))
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, ".context-corvint", "traces")
	store.hooks.beforeCommit = func() {
		lock := filepath.Join(directory, ".trace-operation.lock")
		moved := filepath.Join(directory, ".trace-operation.replaced")
		if err := os.Rename(lock, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lock, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	second := mustRecord(t, Input{Revision: revision, Task: "second", Outcome: "failed"}, nil)
	written, err := store.Append(second)
	if err == nil || written || !strings.Contains(err.Error(), "pathname changed") {
		t.Fatalf("Append() = (%v, %v)", written, err)
	}
	after, err := os.ReadFile(StorePath(root, revision))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("lock replacement changed target\n got: %q\nwant: %q", after, before)
	}
}

func TestAppendFreshValidationFailureRemovesCreatedArtifacts(t *testing.T) {
	root := t.TempDir()
	revision := strings.Repeat("7", 40)
	calls := 0
	store := newTestStore(t, root, map[string]Revision{
		revision: {TreeRevision: strings.Repeat("6", 40)},
	}, func() error {
		calls++
		if calls == 3 {
			return errors.New("injected drift")
		}
		return nil
	})
	record := mustRecord(t, Input{Revision: revision, Task: "task", Outcome: "passed"}, nil)
	if written, err := store.Append(record); err == nil || written {
		t.Fatalf("Append() = (%v, %v)", written, err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".context-corvint")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed validation left trace artifacts: %v", err)
	}
}

func TestAppendRenameFailurePreservesOldBytesAndRemovesTemporary(t *testing.T) {
	root := t.TempDir()
	revision := strings.Repeat("6", 40)
	revisions := map[string]Revision{revision: {TreeRevision: strings.Repeat("5", 40)}}
	store := newTestStore(t, root, revisions, func() error { return nil })
	first := mustRecord(t, Input{Revision: revision, Task: "first", Outcome: "passed"}, nil)
	if written, err := store.Append(first); err != nil || !written {
		t.Fatalf("initial Append() = (%v, %v)", written, err)
	}
	before, err := os.ReadFile(StorePath(root, revision))
	if err != nil {
		t.Fatal(err)
	}
	store.hooks.rename = func(*os.Root, string, string) error { return errors.New("injected rename failure") }
	second := mustRecord(t, Input{Revision: revision, Task: "second", Outcome: "failed"}, nil)
	if written, err := store.Append(second); err == nil || written {
		t.Fatalf("faulted Append() = (%v, %v)", written, err)
	}
	after, err := os.ReadFile(StorePath(root, revision))
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("target changed: error=%v\n got=%q\nwant=%q", err, after, before)
	}
	temporary := filepath.Join(root, ".context-corvint", "traces", ".record-"+revision+".tmp")
	if _, err := os.Lstat(temporary); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary survived rename failure: %v", err)
	}
}

func TestAppendRejectsTemporarySymlinkWithoutOutsideWrite(t *testing.T) {
	root := t.TempDir()
	revision := strings.Repeat("5", 40)
	revisions := map[string]Revision{revision: {TreeRevision: strings.Repeat("4", 40)}}
	store := newTestStore(t, root, revisions, func() error { return nil })
	first := mustRecord(t, Input{Revision: revision, Task: "first", Outcome: "passed"}, nil)
	if written, err := store.Append(first); err != nil || !written {
		t.Fatalf("initial Append() = (%v, %v)", written, err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	wantOutside := []byte("outside bytes")
	if err := os.WriteFile(outside, wantOutside, 0o600); err != nil {
		t.Fatal(err)
	}
	temporary := filepath.Join(root, ".context-corvint", "traces", ".record-"+revision+".tmp")
	if err := os.Symlink(outside, temporary); err != nil {
		t.Fatal(err)
	}
	second := mustRecord(t, Input{Revision: revision, Task: "second", Outcome: "failed"}, nil)
	if written, err := store.Append(second); err == nil || written {
		t.Fatalf("Append() = (%v, %v)", written, err)
	}
	gotOutside, err := os.ReadFile(outside)
	if err != nil || !bytes.Equal(gotOutside, wantOutside) {
		t.Fatalf("outside changed: error=%v got=%q want=%q", err, gotOutside, wantOutside)
	}
}

// os.Root follows an in-root leaf symlink despite O_NOFOLLOW, so a writable
// member open must refuse it before creating or re-permissioning its target.
func TestOpenRegularRejectsInRootSymlinkWithoutTargetMutation(t *testing.T) {
	root := t.TempDir()
	traces := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(traces, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(traces, "victim")
	if err := os.WriteFile(victim, []byte("victim"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(victim, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("victim", filepath.Join(traces, "existing")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("created", filepath.Join(traces, "dangling")); err != nil {
		t.Fatal(err)
	}
	directory, err := openTraceDirectory(root, false)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.close()
	for name, flags := range map[string]int{"existing": os.O_RDWR, "dangling": os.O_RDWR | os.O_CREATE} {
		if file, err := directory.openRegular(name, flags, 0o600); err == nil {
			file.Close()
			t.Fatalf("openRegular(%s) accepted an in-root symlink", name)
		}
	}
	if info, err := os.Stat(victim); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("symlink target re-permissioned: info=%v err=%v", info, err)
	}
	if _, err := os.Lstat(filepath.Join(traces, "created")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dangling symlink target created: %v", err)
	}
}

func newTestStore(t *testing.T, root string, revisions map[string]Revision, check func() error) *Store {
	t.Helper()
	store, err := NewStore(root, revisions, check)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func mustRecord(t *testing.T, input Input, tracked []string) Record {
	t.Helper()
	record, err := NewRecord(input, tracked)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func writeTraceFixture(t *testing.T, root, revision string, data []byte) {
	t.Helper()
	directory := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, revision+".jsonl"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertMode(t *testing.T, name string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", name, got, want)
	}
}
