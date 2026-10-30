package source

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadDeterministicStableResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "trace.json")
	if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := []byte(`{"ok":true}`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	root := mustOpenRoot(t, dir)

	first := Read(root, "nested/trace.json", KindLocalTrace, VerifierLocalTraceV1, NewBudget(MaxAggregateBytes), nil)
	second := Read(root, "nested/trace.json", KindLocalTrace, VerifierLocalTraceV1, NewBudget(MaxAggregateBytes), nil)
	wantDigest := sha256.Sum256(contents)
	wantDigestText := "sha256:" + hex.EncodeToString(wantDigest[:])

	for _, got := range []Result{first, second} {
		if got.Validity != ValidityStable || got.Issue != IssueNone || got.Bytes != uint64(len(contents)) {
			t.Fatalf("unexpected result: %#v", got)
		}
		if got.SHA256 == nil || *got.SHA256 != wantDigestText {
			t.Fatalf("digest = %v, want %s", got.SHA256, wantDigestText)
		}
		if got.sourceID == "" || strings.Contains(got.sourceID, "trace.json") {
			t.Fatalf("internal acquisition ID is absent or discloses path: %q", got.sourceID)
		}
	}
	if first.sourceID != second.sourceID || *first.SHA256 != *second.SHA256 {
		t.Fatalf("success is not deterministic: %#v != %#v", first, second)
	}
	after, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("read changed source metadata")
	}
}

func TestRejectsInvalidRegistration(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := mustOpenRoot(t, dir)
	got := Read(root, "source", Kind("unregistered"), VerifierID("unregistered/9"), NewBudget(1), nil)
	assertFailure(t, got, IssueInvalidRegistration)
	if got.Kind != KindUnknown || got.VerifierID != VerifierUnknown {
		t.Fatalf("unregistered identifiers escaped result: %#v", got)
	}
}

func TestRejectsInvalidPaths(t *testing.T) {
	root := mustOpenRoot(t, t.TempDir())
	paths := []string{
		"", ".", "..", "../secret", "a/../secret", "a/./secret", "a//secret",
		"/absolute", `C:\secret`, `a\secret`, "control\nname", strings.Repeat("a", MaxPathBytes+1),
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			got := Read(root, path, KindLocalTrace, VerifierLocalTraceV1, NewBudget(MaxAggregateBytes), nil)
			assertFailure(t, got, IssueInvalidPath)
		})
	}
}

func TestRejectsSymlinkLeaf(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "real"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(dir, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got := Read(mustOpenRoot(t, dir), "linked", KindLocalTrace, VerifierLocalTraceV1, NewBudget(1), nil)
	assertFailure(t, got, IssueSourceSymlink)
}

func TestRejectsSymlinkParent(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "real"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "real", "source"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(dir, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got := Read(mustOpenRoot(t, dir), "linked/source", KindLocalTrace, VerifierLocalTraceV1, NewBudget(1), nil)
	assertFailure(t, got, IssueSourceSymlink)
}

func TestRejectsHardLink(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original")
	linked := filepath.Join(dir, "linked")
	if err := os.WriteFile(original, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(original, linked); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	info, err := os.Lstat(linked)
	if err != nil {
		t.Fatal(err)
	}
	multiple, known := multipleLinks(info)
	if !known || !multiple {
		t.Skip("portable file metadata does not expose multiple links")
	}
	got := Read(mustOpenRoot(t, dir), "linked", KindLocalTrace, VerifierLocalTraceV1, NewBudget(1), nil)
	assertFailure(t, got, IssueSourceHardLinked)
}

func TestRejectsDirectoryAndMissingSource(t *testing.T) {
	dir := t.TempDir()
	root := mustOpenRoot(t, dir)
	assertFailure(t, Read(root, "missing", KindLocalTrace, VerifierLocalTraceV1, NewBudget(1), nil), IssueSourceUnavailable)
	if err := os.Mkdir(filepath.Join(dir, "directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	assertFailure(t, Read(root, "directory", KindLocalTrace, VerifierLocalTraceV1, NewBudget(1), nil), IssueSourceNotRegular)
}

func TestRejectsClosedRootWithoutErrorDisclosure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "private-token"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := mustOpenRoot(t, dir)
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	got := Read(root, "private-token", KindLocalTrace, VerifierLocalTraceV1, NewBudget(64), nil)
	assertFailure(t, got, IssueSourceUnreadable)
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private-token", dir, "closed", "secret"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("result disclosed %q: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(string(encoded), `"sha256":null`) {
		t.Fatalf("failure digest is not serialized as null: %s", encoded)
	}
}

func TestHostileFilenameIsRedacted(t *testing.T) {
	hostile := "credential=top-secret\nprivate/path"
	got := Read(mustOpenRoot(t, t.TempDir()), hostile, KindLocalTrace, VerifierLocalTraceV1, NewBudget(1), nil)
	assertFailure(t, got, IssueInvalidPath)
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "credential") || strings.Contains(string(encoded), "top-secret") || strings.Contains(string(encoded), "private/path") {
		t.Fatalf("hostile path escaped closed result: %s", encoded)
	}
}

func TestRejectsReplacementAfterPreflight(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := mustOpenRoot(t, dir)
	got := read(root, "source", KindLocalTrace, VerifierLocalTraceV1, NewBudget(64), nil, func() {
		if err := os.Rename(path, filepath.Join(dir, "old-source")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("new"), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	assertFailure(t, got, IssueSourceUnstable)
}

func TestSourceAndAggregateLimits(t *testing.T) {
	dir := t.TempDir()
	exact := filepath.Join(dir, "exact")
	over := filepath.Join(dir, "over")
	if err := os.WriteFile(exact, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(exact, int64(MaxSourceBytes)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(over, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(over, int64(MaxSourceBytes+1)); err != nil {
		t.Fatal(err)
	}
	root := mustOpenRoot(t, dir)
	exactResult := Read(root, "exact", KindLocalTrace, VerifierLocalTraceV1, NewBudget(MaxSourceBytes), nil)
	if exactResult.Validity != ValidityStable || exactResult.Bytes != MaxSourceBytes || exactResult.SHA256 == nil {
		t.Fatalf("exact limit rejected: %#v", exactResult)
	}
	overResult := Read(root, "over", KindLocalTrace, VerifierLocalTraceV1, NewBudget(MaxAggregateBytes), nil)
	assertFailure(t, overResult, IssueSourceTooLarge)
	if overResult.Bytes != MaxSourceBytes+1 {
		t.Fatalf("oversize byte count = %d", overResult.Bytes)
	}

	if err := os.WriteFile(filepath.Join(dir, "small"), []byte("1234"), 0o600); err != nil {
		t.Fatal(err)
	}
	budget := NewBudget(3)
	aggregateResult := Read(root, "small", KindLocalTrace, VerifierLocalTraceV1, budget, nil)
	assertFailure(t, aggregateResult, IssueAggregateBudgetExceeded)
	if budget.Used() != 0 {
		t.Fatalf("rejected reservation consumed budget: %d", budget.Used())
	}
	assertFailure(t, Read(root, "small", KindLocalTrace, VerifierLocalTraceV1, nil, nil), IssueAggregateBudgetExceeded)

	if err := os.WriteFile(filepath.Join(dir, "two-a"), []byte("12"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "two-b"), []byte("34"), 0o600); err != nil {
		t.Fatal(err)
	}
	cumulative := NewBudget(MaxSourceBytes)
	if got := ReadOrdinal(root, "two-a", 0, KindLocalTrace, VerifierLocalTraceV1, cumulative, nil); got.Validity != ValidityStable {
		t.Fatalf("first aggregate source rejected: %#v", got)
	}
	assertFailure(t, ReadOrdinal(root, "two-b", 1, KindLocalTrace, VerifierLocalTraceV1, cumulative, nil), IssueAggregateBudgetExceeded)
	if cumulative.Used() != MaxSourceBytes {
		t.Fatalf("cumulative budget = %d, want %d", cumulative.Used(), MaxSourceBytes)
	}
}

func TestBudgetCapsAtAggregateMaximum(t *testing.T) {
	budget := NewBudget(MaxAggregateBytes + 1)
	if !budget.reserve(MaxAggregateBytes) || budget.reserve(1) || budget.Used() != MaxAggregateBytes {
		t.Fatalf("aggregate cap not enforced: used=%d", budget.Used())
	}
}

func TestRetryReservationChargesCompiledBoundOnceAcrossReplacementGrowth(t *testing.T) {
	grown := []byte("replacement-is-larger")
	dir := t.TempDir()
	filePath := filepath.Join(dir, "source")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	budget := NewBudget(MaxSourceBytes)
	got := readOrdinal(
		mustOpenRoot(t, dir), "source", 0, KindLocalTrace, VerifierLocalTraceV1,
		budget, ProcessClock(), nil, acquisitionHooks{afterStat1: func(attempt int) {
			if attempt == 0 {
				if err := os.WriteFile(filePath, grown, 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}},
	)
	if got.Validity != ValidityStable || got.Issue != IssueNone || got.Bytes != uint64(len(grown)) {
		t.Fatalf("grown retry result = %#v", got)
	}
	if budget.Used() != MaxSourceBytes || budget.count != 1 {
		t.Fatalf("reservation ledger used=%d count=%d, want used=%d count=1", budget.Used(), budget.count, MaxSourceBytes)
	}
}

func assertFailure(t *testing.T, got Result, want IssueCode) {
	t.Helper()
	if got.Validity == ValidityStable || got.Issue != want || got.SHA256 != nil {
		t.Fatalf("result = %#v, want failed %q with nil digest", got, want)
	}
	if got.sourceID == "" {
		t.Fatal("failure omitted internal acquisition ID")
	}
}

func TestStableConsumerUsesAdmittedBytesWithoutReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source")
	original := []byte("admitted bytes")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	called := 0
	var captured StableContent
	var retainedReader io.Reader
	got := ReadOrdinal(mustOpenRoot(t, dir), "source", 37, KindLocalTrace, VerifierLocalTraceV1, NewBudget(MaxAggregateBytes), func(content StableContent) {
		called++
		if content.Len() != uint64(len(original)) {
			t.Fatalf("callback length = %d", content.Len())
		}
		if err := os.WriteFile(path, []byte("replacement"), 0o600); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			readBack, err := io.ReadAll(content.Reader())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(readBack, original) {
				t.Fatalf("callback reopened or changed bytes: %q", readBack)
			}
		}
		digest := sha256.Sum256(original)
		if content.SHA256() != "sha256:"+hex.EncodeToString(digest[:]) {
			t.Fatalf("callback digest = %s", content.SHA256())
		}
		captured = content
		retainedReader = content.Reader()
	})
	if called != 1 || got.Validity != ValidityStable || got.SHA256 == nil || *got.SHA256 != contentDigest(original) {
		t.Fatalf("callback/result mismatch: called=%d result=%#v", called, got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "source_id") || strings.Contains(string(encoded), got.sourceID) {
		t.Fatalf("internal acquisition ID serialized: %s", encoded)
	}
	if got.Evidence == nil || got.Evidence.ConfiguredOrdinal() != 37 {
		t.Fatalf("configured ordinal evidence = %#v", got.Evidence)
	}
	if got.Evidence.Start().After(got.Evidence.End()) || got.Evidence.Start().Location() != time.UTC || got.Evidence.End().Location() != time.UTC {
		t.Fatalf("invalid observation interval: %s..%s", got.Evidence.Start(), got.Evidence.End())
	}
	if got.Evidence.FileIdentityBefore() != got.Evidence.FileIdentityAfter() {
		t.Fatalf("successful identities differ: %#v != %#v", got.Evidence.FileIdentityBefore(), got.Evidence.FileIdentityAfter())
	}
	if got.Evidence.FileIdentityBefore().PlatformIdentity().LinkCount != 1 {
		t.Fatalf("unqualified stable identity: %#v", got.Evidence.FileIdentityBefore())
	}
	if captured.Len() != 0 || captured.SHA256() != "" {
		t.Fatal("stable content outlived synchronous callback")
	}
	if _, err := io.ReadAll(retainedReader); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("retained reader remained live: %v", err)
	}
}

func TestStableConsumerNotCalledOnFailure(t *testing.T) {
	called := false
	got := Read(mustOpenRoot(t, t.TempDir()), "missing", KindLocalTrace, VerifierLocalTraceV1, NewBudget(1), func(StableContent) {
		called = true
	})
	assertFailure(t, got, IssueSourceUnavailable)
	if called {
		t.Fatal("consumer called for rejected source")
	}
}

func TestReadOrdinalFixedClockBindsExactEvidenceInterval(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("fixed"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixedTime := time.Date(2026, time.August, 23, 12, 34, 56, 123456789, time.UTC)
	clock, err := FixedClock(fixedTime)
	if err != nil {
		t.Fatal(err)
	}
	got := ReadOrdinalWithClock(mustOpenRoot(t, dir), "source", 4, KindLocalTrace, VerifierLocalTraceV1, NewBudget(MaxAggregateBytes), clock, nil)
	if got.Validity != ValidityStable || got.Evidence == nil || got.Evidence.Start() != fixedTime || got.Evidence.End() != fixedTime {
		t.Fatalf("fixed-clock result = %#v", got)
	}
	if got.Evidence.Start().Format("2006-01-02T15:04:05.000000000Z") != "2026-08-23T12:34:56.123456789Z" {
		t.Fatalf("fixed-clock shape = %s", got.Evidence.Start())
	}
}

func TestClockConstructionIsClosedAndRejectsInvalidValues(t *testing.T) {
	if _, err := FixedClock(time.Date(2026, 8, 23, 12, 0, 0, 0, time.FixedZone("not-utc", 0))); err == nil {
		t.Fatal("accepted non-UTC fixed clock")
	}
	called := false
	got := ReadOrdinalWithClock(mustOpenRoot(t, t.TempDir()), "source", 0, KindLocalTrace, VerifierLocalTraceV1, NewBudget(1), Clock{}, func(StableContent) {
		called = true
	})
	assertFailure(t, got, IssueInvalidRegistration)
	if called {
		t.Fatal("invalid clock reached consumer")
	}
}

func contentDigest(contents []byte) string {
	digest := sha256.Sum256(contents)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func mustOpenRoot(t *testing.T, dir string) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}
