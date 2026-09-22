//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package source

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRejectsFIFO(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o600); err != nil {
		t.Skipf("FIFO unavailable: %v", err)
	}
	got := Read(mustOpenRoot(t, dir), "pipe", KindLocalTrace, VerifierLocalTraceV1, NewBudget(1), nil)
	assertFailure(t, got, IssueSourceNotRegular)
}

func TestRejectsDevice(t *testing.T) {
	if _, err := os.Lstat("/dev/null"); err != nil {
		t.Skipf("device unavailable: %v", err)
	}
	got := Read(mustOpenRoot(t, "/dev"), "null", KindLocalTrace, VerifierLocalTraceV1, NewBudget(1), nil)
	assertFailure(t, got, IssueSourceNotRegular)
}

// TestRejectsFIFOSwappedInAfterPreflight covers the window between the
// component Lstat and the open: a FIFO renamed over the source there must be
// refused by the descriptor check, never block the open.
func TestRejectsFIFOSwappedInAfterPreflight(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source")
	if err := os.WriteFile(sourcePath, []byte("stable bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	pipePath := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(pipePath, 0o600); err != nil {
		t.Skipf("FIFO unavailable: %v", err)
	}
	var swapErr error
	swap := func(attempt int) {
		if attempt == 0 {
			swapErr = os.Rename(pipePath, sourcePath)
		}
	}
	done := make(chan Result, 1)
	go func() {
		done <- readOrdinal(
			mustOpenRoot(t, dir), "source", 0, KindLocalTrace, VerifierLocalTraceV1,
			NewBudget(MaxAggregateBytes), ProcessClock(), nil, acquisitionHooks{afterPreflight: swap},
		)
	}()
	select {
	case got := <-done:
		if swapErr != nil {
			t.Fatal(swapErr)
		}
		assertFailure(t, got, IssueSourceNotRegular)
	case <-time.After(5 * time.Second):
		t.Fatal("acquisition blocked opening a FIFO swapped in after preflight")
	}
}

func TestRejectsTransientLinkCountAtEveryDescriptorStat(t *testing.T) {
	for _, test := range []struct {
		name      string
		configure func(create, remove func(int)) acquisitionHooks
		wantIssue IssueCode
	}{
		{
			name: "first",
			configure: func(create, remove func(int)) acquisitionHooks {
				return acquisitionHooks{afterPreflight: create, afterStat1: remove}
			},
			wantIssue: IssueSourceHardLinked,
		},
		{
			name: "second",
			configure: func(create, remove func(int)) acquisitionHooks {
				return acquisitionHooks{afterStat1: create, afterStat2: remove}
			},
			wantIssue: IssueSourceUnstable,
		},
		{
			name: "third",
			configure: func(create, remove func(int)) acquisitionHooks {
				return acquisitionHooks{afterStat2: create, afterStat3: remove}
			},
			wantIssue: IssueSourceUnstable,
		},
		{
			name: "fourth",
			configure: func(create, remove func(int)) acquisitionHooks {
				return acquisitionHooks{afterStat3: create, afterStat4: remove}
			},
			wantIssue: IssueSourceUnstable,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			sourcePath := filepath.Join(dir, "source")
			linkPath := filepath.Join(dir, "transient-link")
			if err := os.WriteFile(sourcePath, []byte("stable bytes"), 0o600); err != nil {
				t.Fatal(err)
			}
			create := func(int) {
				if err := os.Link(sourcePath, linkPath); err != nil {
					t.Skipf("hard links unavailable: %v", err)
				}
			}
			remove := func(int) {
				if err := os.Remove(linkPath); err != nil {
					t.Fatal(err)
				}
			}
			got := readOrdinal(
				mustOpenRoot(t, dir), "source", 0, KindLocalTrace, VerifierLocalTraceV1,
				NewBudget(MaxAggregateBytes), ProcessClock(), nil, test.configure(create, remove),
			)
			assertFailure(t, got, test.wantIssue)
			if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
				t.Fatalf("transient link survived test: %v", err)
			}
		})
	}
}

func TestRejectsPermissionDeniedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unreadable")
	if err := os.WriteFile(path, []byte("x"), 0); err != nil {
		t.Fatal(err)
	}
	got := Read(mustOpenRoot(t, dir), "unreadable", KindLocalTrace, VerifierLocalTraceV1, NewBudget(1), nil)
	if got.Validity == ValidityStable {
		t.Skip("test process can read mode-000 files")
	}
	assertFailure(t, got, IssueSourceUnreadable)
}

func TestScanTraceStoreIgnoresBackslashNonCandidateName(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "traces")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("a", 40)
	for _, name := range []string{revision + ".jsonl", `notes\draft.txt`} {
		if err := os.WriteFile(filepath.Join(store, name), []byte("a"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result := ScanTraceStore(mustOpenRoot(t, dir), "traces", 0, ObjectFormatSHA1, NewBudget(MaxSourceBytes), nil)
	if result.Validity != ValidityStable || result.Issue != IssueNone || len(result.Members) != 1 || result.Evidence == nil || result.Evidence.EntryCount() != 2 {
		t.Fatalf("store result = %#v", result)
	}
}
