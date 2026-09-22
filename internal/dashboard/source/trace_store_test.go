package source

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanTraceStoreStableBoundedMembers(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	revisionA := strings.Repeat("a", 40)
	revisionB := strings.Repeat("b", 40)
	if err := os.WriteFile(filepath.Join(store, revisionB+".jsonl"), []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, "ignored"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, revisionA+".jsonl"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}

	var revisions []string
	var captured StableContent
	result := ScanTraceStore(mustOpenRoot(t, dir), "traces", 9, ObjectFormatSHA1, NewBudget(MaxSourceBytes), func(revision string, content StableContent, evidence Evidence) {
		revisions = append(revisions, revision)
		if evidence.ConfiguredOrdinal() != 9 || evidence.AdapterID() != "local-trace-v1" || evidence.Validity() != ValidityStable {
			t.Fatalf("member evidence = %#v", evidence)
		}
		body, err := io.ReadAll(content.Reader())
		if err != nil || string(body) != string(revision[0]) {
			t.Fatalf("member body = %q, err=%v", body, err)
		}
		captured = content
	})
	if result.Validity != ValidityStable || result.Issue != IssueNone || result.Limit != 0 || len(result.Members) != 2 {
		t.Fatalf("store result = %#v", result)
	}
	if strings.Join(revisions, ",") != revisionA+","+revisionB {
		t.Fatalf("callback order = %v", revisions)
	}
	if result.Evidence == nil || result.Evidence.ConfiguredOrdinal() != 9 || result.Evidence.EntryCount() != 3 {
		t.Fatalf("store evidence = %#v", result.Evidence)
	}
	if result.Evidence.Start().After(result.Evidence.End()) || result.Evidence.DirectoryBefore() != result.Evidence.DirectoryAfter() {
		t.Fatalf("unstable store evidence = %#v", result.Evidence)
	}
	if captured.Len() != 0 {
		t.Fatal("member content escaped callback")
	}
}

func TestScanTraceStoreRetriesWholeStoreBeforeCallbacks(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("c", 40)
	if err := os.WriteFile(filepath.Join(store, revision+".jsonl"), []byte("c"), 0o600); err != nil {
		t.Fatal(err)
	}
	callbacks := 0
	result := scanTraceStore(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), ProcessClock(), func(string, StableContent, Evidence) {
		callbacks++
	}, func(attempt int) {
		if attempt == 0 {
			if err := os.WriteFile(filepath.Join(store, "appeared"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	})
	if result.Validity != ValidityStable || callbacks != 1 || result.Evidence == nil || result.Evidence.EntryCount() != 2 {
		t.Fatalf("retry result = %#v, callbacks=%d", result, callbacks)
	}
}

func TestScanTraceStoreChangedTwiceFailsClosed(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	callbacks := 0
	result := scanTraceStore(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), ProcessClock(), func(string, StableContent, Evidence) {
		callbacks++
	}, func(attempt int) {
		name := filepath.Join(store, "change-"+string(rune('0'+attempt)))
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	if result.Validity != ValidityInvalid || result.Issue != IssueStoreChanged || result.Limit != 0 || callbacks != 0 || result.Evidence != nil || len(result.Members) != 0 {
		t.Fatalf("changed store result = %#v, callbacks=%d", result, callbacks)
	}
}

func TestTraceStoreBoundReportsExactEntryLimit(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	for index := 0; index <= MaxTraceStoreEntries; index++ {
		name := filepath.Join(store, fmt.Sprintf("entry-%04d", index))
		if err := os.WriteFile(name, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result := ScanTraceStore(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxAggregateBytes), nil)
	if result.Validity != ValidityInvalid || result.Issue != IssueTraceStoreBound || result.Limit != MaxTraceStoreEntries {
		t.Fatalf("entry-bound result = %#v", result)
	}
}

func TestTraceStoreBoundReportsExactByteLimit(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, MaxSourceBytes/2+1)
	for _, revision := range []string{strings.Repeat("1", 40), strings.Repeat("2", 40)} {
		if err := os.WriteFile(filepath.Join(store, revision+".jsonl"), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result := ScanTraceStore(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxAggregateBytes), nil)
	if result.Validity != ValidityInvalid || result.Issue != IssueTraceStoreBound || result.Limit != MaxSourceBytes {
		t.Fatalf("byte-bound result = %#v", result)
	}
}

func TestTraceStoreMemberOversizedPrecedesAggregateBound(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("5", 40)
	member := filepath.Join(store, revision+".jsonl")
	file, err := os.OpenFile(member, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(int64(MaxSourceBytes + 1)); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	result := ScanTraceStore(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), nil)
	if result.Validity != ValidityStable || result.Issue != IssueNone || len(result.Members) != 1 || result.Members[0].Result.Issue != IssueSourceTooLarge {
		t.Fatalf("oversized precedence result = %#v", result)
	}
}

func TestTraceStoreMultilinkPrecedesAggregateByteBound(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	firstRevision := strings.Repeat("6", 40)
	linkedRevision := strings.Repeat("7", 40)
	first := filepath.Join(store, firstRevision+".jsonl")
	linked := filepath.Join(store, linkedRevision+".jsonl")
	firstFile, err := os.OpenFile(first, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := firstFile.Truncate(int64(MaxSourceBytes/2 + 1)); err != nil {
		_ = firstFile.Close()
		t.Fatal(err)
	}
	if err := firstFile.Close(); err != nil {
		t.Fatal(err)
	}
	linkedTarget := filepath.Join(store, "linked-target")
	linkedFile, err := os.OpenFile(linkedTarget, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := linkedFile.Truncate(int64(MaxSourceBytes / 2)); err != nil {
		_ = linkedFile.Close()
		t.Fatal(err)
	}
	if err := linkedFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(linkedTarget, linked); err != nil {
		t.Skipf("hard link unavailable: %v", err)
	}

	result := ScanTraceStore(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), nil)
	if result.Validity != ValidityStable || result.Issue != IssueNone || len(result.Members) != 2 || result.Members[1].Result.Issue != IssueSourceHardLinked {
		t.Fatalf("multilink precedence result = %#v", result)
	}
}

func TestTraceStoreChangedOverridesStagedEntryBound(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	for index := 0; index <= MaxTraceStoreEntries; index++ {
		if err := os.WriteFile(filepath.Join(store, fmt.Sprintf("entry-%04d", index)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result := scanTraceStore(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), ProcessClock(), nil, func(attempt int) {
		if err := os.WriteFile(filepath.Join(store, fmt.Sprintf("changed-%d", attempt)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	})
	if result.Validity != ValidityInvalid || result.Issue != IssueStoreChanged {
		t.Fatalf("store-change precedence result = %#v", result)
	}
}

func TestTraceStoreChangedOverridesMemberPhysicalBound(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("8", 40)
	if err := os.WriteFile(filepath.Join(store, revision+".jsonl"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	budget := NewBudget(MaxSourceBytes)
	budget.physicalLimit = 1
	callbacks := 0
	result := scanTraceStore(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, budget, ProcessClock(), func(string, StableContent, Evidence) {
		callbacks++
	}, func(attempt int) {
		if err := os.WriteFile(filepath.Join(store, fmt.Sprintf("changed-physical-%d", attempt)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	})
	if result.Validity != ValidityInvalid || result.Issue != IssueStoreChanged || result.Evidence != nil || len(result.Members) != 0 || callbacks != 0 {
		t.Fatalf("store-change precedence result = %#v, callbacks=%d", result, callbacks)
	}
}

func TestTraceStoreEntryBoundPrecedesLaterPhysicalExhaustion(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("9", 40)
	if err := os.WriteFile(filepath.Join(store, revision+".jsonl"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < MaxTraceStoreEntries; index++ {
		if err := os.WriteFile(filepath.Join(store, fmt.Sprintf("ignored-%04d", index)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	budget := NewBudget(MaxSourceBytes)
	budget.physicalLimit = 0
	callbacks := 0
	result := ScanTraceStore(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, budget, func(string, StableContent, Evidence) {
		callbacks++
	})
	if result.Validity != ValidityInvalid || result.Issue != IssueTraceStoreBound || result.Limit != MaxTraceStoreEntries ||
		result.Evidence != nil || len(result.Members) != 0 || callbacks != 0 || budget.PhysicalUsed() != 0 {
		t.Fatalf("entry-bound precedence result=%#v callbacks=%d physical=%d", result, callbacks, budget.PhysicalUsed())
	}
}

func TestTraceStoreByteBoundStopsBeforeLaterPhysicalExhaustion(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	sizes := []int64{int64(MaxSourceBytes/2 + 1), int64(MaxSourceBytes / 2), 1}
	for index, size := range sizes {
		path := filepath.Join(store, strings.Repeat(string(rune('a'+index)), 40)+".jsonl")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(size); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	budget := NewBudget(MaxSourceBytes)
	budget.physicalLimit = 2 * uint64(sizes[0]+sizes[1])
	callbacks := 0
	result := ScanTraceStore(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, budget, func(string, StableContent, Evidence) {
		callbacks++
	})
	if result.Validity != ValidityInvalid || result.Issue != IssueTraceStoreBound || result.Limit != MaxSourceBytes ||
		result.Evidence != nil || len(result.Members) != 0 || callbacks != 0 || budget.PhysicalUsed() != budget.physicalLimit {
		t.Fatalf("byte-bound precedence result=%#v callbacks=%d physical=%d/%d", result, callbacks, budget.PhysicalUsed(), budget.physicalLimit)
	}
}

func TestScanTraceStoreCandidateSymlinkRejectedWithoutFollowing(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, "target"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("d", 40)
	if err := os.Symlink("target", filepath.Join(store, revision+".jsonl")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	result := ScanTraceStore(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), nil)
	if result.Validity != ValidityStable || len(result.Members) != 1 || result.Members[0].Result.Issue != IssueSourceSymlink || result.Members[0].Result.SHA256 != nil {
		t.Fatalf("symlink member result = %#v", result)
	}
}

func TestTraceRevisionGrammar(t *testing.T) {
	sha1 := strings.Repeat("0", 40)
	sha256 := strings.Repeat("f", 64)
	for _, test := range []struct {
		name   string
		format ObjectFormat
		ok     bool
	}{
		{sha1 + ".jsonl", ObjectFormatSHA1, true},
		{sha256 + ".jsonl", ObjectFormatSHA256, true},
		{strings.Repeat("A", 40) + ".jsonl", ObjectFormatSHA1, false},
		{sha1 + ".json", ObjectFormatSHA1, false},
		{sha256 + ".jsonl", ObjectFormatSHA1, false},
	} {
		_, got := traceRevision(test.name, test.format)
		if got != test.ok {
			t.Fatalf("traceRevision(%q, %q) = %v", test.name, test.format, got)
		}
	}
}

func TestWindowsReparseAttributeRejectsEveryTagClass(t *testing.T) {
	if !windowsFileAttributesReparse(windowsFileAttributeReparsePoint) {
		t.Fatal("name-surrogate reparse attribute accepted")
	}
	if !windowsFileAttributesReparse(windowsFileAttributeReparsePoint | 0x00080000) {
		t.Fatal("non-name-surrogate cloud/dedup reparse attribute accepted")
	}
	if windowsFileAttributesReparse(0x00080000) {
		t.Fatal("ordinary non-reparse attribute rejected")
	}
}

func TestScanTraceStoreExpiredReader(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("e", 40)
	if err := os.WriteFile(filepath.Join(store, revision+".jsonl"), []byte("e"), 0o600); err != nil {
		t.Fatal(err)
	}
	var reader io.Reader
	result := ScanTraceStore(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), func(_ string, content StableContent, _ Evidence) {
		reader = content.Reader()
	})
	if result.Validity != ValidityStable {
		t.Fatalf("store result = %#v", result)
	}
	if _, err := io.ReadAll(reader); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("retained reader remained valid: %v", err)
	}
}

func TestScanTraceStoreFixedClockBindsAggregateAndMemberIntervals(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("f", 40)
	if err := os.WriteFile(filepath.Join(store, revision+".jsonl"), []byte("f"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixedTime := time.Date(2026, time.August, 23, 12, 34, 56, 987654321, time.UTC)
	clock, err := FixedClock(fixedTime)
	if err != nil {
		t.Fatal(err)
	}
	callbacks := 0
	result := ScanTraceStoreWithClock(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), clock, func(_ string, _ StableContent, evidence Evidence) {
		callbacks++
		if evidence.Start() != fixedTime || evidence.End() != fixedTime {
			t.Fatalf("member interval = %s..%s", evidence.Start(), evidence.End())
		}
	})
	if result.Validity != ValidityStable || result.Evidence == nil || callbacks != 1 || result.Evidence.Start() != fixedTime || result.Evidence.End() != fixedTime {
		t.Fatalf("fixed aggregate = %#v, callbacks=%d", result, callbacks)
	}
}

func TestScanTraceStoreKeepsRenamedDirectoryDescriptorAndRetriesReplacement(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("1", 40)
	member := filepath.Join(store, revision+".jsonl")
	if err := os.WriteFile(member, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	var bodies []string
	result := scanTraceStoreWithHooks(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), ProcessClock(), func(_ string, content StableContent, _ Evidence) {
		body, err := io.ReadAll(content.Reader())
		if err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, string(body))
	}, traceStoreHooks{afterFirstEnumeration: func(attempt int) {
		if attempt != 0 {
			return
		}
		if err := os.Rename(store, filepath.Join(dir, "renamed-store")); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(store, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(store, revision+".jsonl"), []byte("new"), 0o600); err != nil {
			t.Fatal(err)
		}
	}})
	if result.Validity != ValidityStable || result.Issue != IssueNone || len(bodies) != 1 || bodies[0] != "new" {
		t.Fatalf("replacement result=%#v bodies=%q", result, bodies)
	}
}

func TestScanTraceStoreRejectsDirectorySymlinkReplacementWithoutFollowing(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	external := filepath.Join(dir, "external")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(external, 0o700); err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("2", 40)
	if err := os.WriteFile(filepath.Join(store, revision+".jsonl"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, revision+".jsonl"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	callbacks := 0
	result := scanTraceStoreWithHooks(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), ProcessClock(), func(string, StableContent, Evidence) {
		callbacks++
	}, traceStoreHooks{afterFirstEnumeration: func(attempt int) {
		if attempt != 0 {
			return
		}
		if err := os.Rename(store, filepath.Join(dir, "renamed-store")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("external", store); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
	}})
	if result.Validity != ValidityInvalid || result.Issue != IssueSourceSymlink || callbacks != 0 {
		t.Fatalf("symlink replacement result=%#v callbacks=%d", result, callbacks)
	}
}

func TestScanTraceStoreCandidateReplacementCannotEscapeHeldDirectory(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("3", 40)
	memberName := revision + ".jsonl"
	member := filepath.Join(store, memberName)
	if err := os.WriteFile(member, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	callbacks := 0
	result := scanTraceStoreWithHooks(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), ProcessClock(), func(string, StableContent, Evidence) {
		callbacks++
	}, traceStoreHooks{afterFirstEnumeration: func(attempt int) {
		if attempt != 0 {
			return
		}
		if err := os.Rename(member, filepath.Join(store, "original")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join("..", "secret"), member); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
	}})
	if result.Validity != ValidityStable || result.Issue != IssueNone || len(result.Members) != 1 || result.Members[0].Result.Issue != IssueSourceSymlink || callbacks != 0 {
		t.Fatalf("candidate replacement result=%#v callbacks=%d", result, callbacks)
	}
}

func TestScanTraceStoreUnstableMemberDoesNotSpendOtherMembersOverflowProbes(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	unstableName := strings.Repeat("a", 40) + ".jsonl"
	stableName := strings.Repeat("b", 40) + ".jsonl"
	unstable := filepath.Join(store, unstableName)
	if err := os.WriteFile(unstable, []byte("grow"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, stableName), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	var restore func()
	var bodies []string
	result := scanTraceStoreWithHooks(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), ProcessClock(), func(_ string, content StableContent, _ Evidence) {
		body, err := io.ReadAll(content.Reader())
		if err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, string(body))
	}, traceStoreHooks{beforeMemberRead: func(name string, attempt int) {
		if name == stableName && restore != nil {
			restore()
			return
		}
		if name != unstableName {
			return
		}
		info, err := os.Lstat(unstable)
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(unstable, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte("+")); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if attempt == 1 {
			restore = func() {
				if err := os.Truncate(unstable, info.Size()); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(unstable, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			}
		}
	}})
	if result.Validity != ValidityStable || result.Issue != IssueNone || len(result.Members) != 2 {
		t.Fatalf("store result=%#v", result)
	}
	if result.Members[0].Result.Validity != ValidityInvalid || result.Members[0].Result.Issue != IssueSourceUnstable {
		t.Fatalf("unstable member = %#v", result.Members[0].Result)
	}
	if result.Members[1].Result.Validity != ValidityStable || len(bodies) != 1 || bodies[0] != "keep" {
		t.Fatalf("stable member = %#v bodies=%q", result.Members[1].Result, bodies)
	}
}

func TestScanTraceStoreCandidateIdentityReplacementRetriesWholeStore(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("4", 40)
	member := filepath.Join(store, revision+".jsonl")
	if err := os.WriteFile(member, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	var bodies []string
	result := scanTraceStoreWithHooks(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), ProcessClock(), func(_ string, content StableContent, _ Evidence) {
		body, err := io.ReadAll(content.Reader())
		if err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, string(body))
	}, traceStoreHooks{afterFirstEnumeration: func(attempt int) {
		if attempt != 0 {
			return
		}
		if err := os.Rename(member, filepath.Join(store, "original")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(member, []byte("new"), 0o600); err != nil {
			t.Fatal(err)
		}
	}})
	if result.Validity != ValidityStable || result.Issue != IssueNone || len(bodies) != 1 || bodies[0] != "new" {
		t.Fatalf("identity replacement result=%#v bodies=%q", result, bodies)
	}
}
