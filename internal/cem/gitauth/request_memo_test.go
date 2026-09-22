package gitauth

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

func memoFixture(t *testing.T, format string) (*RequestReadMemo, *Repository, string, string, string) {
	t.Helper()
	live, view, base, target := objectViewFixture(t, format)
	immutable := open(t, view)
	if err := immutable.LoadObjectFormat(context.Background()); err != nil {
		t.Fatal(err)
	}
	memo, err := NewRequestReadMemo(immutable)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(memo.Release)
	reader, err := memo.Open(gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	return memo, reader, live, base, target
}

// The shell immediately execs the original Git. Every measured child remains
// inside the existing gitrun containment/cleanup path, with no extra descendant.
func countMemoGit(t *testing.T) func() int {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	body := "#!/bin/sh\nprintf x >>" + quote(log) + "\nexec " + quote(git) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() int {
		raw, err := os.ReadFile(log)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return len(raw)
	}
}

func TestRequestMemoPrimitiveParityAndCopies(t *testing.T) {
	t.Run("PLE-V0-003 immutable primitive successes preserve values and copies", func(t *testing.T) {
		for _, format := range []string{"sha1", "sha256"} {
			t.Run(format, func(t *testing.T) {
				memo, reader, _, base, target := memoFixture(t, format)
				count := countMemoGit(t)
				for _, oid := range []string{base, target} {
					before := count()
					for i := 0; i < 2; i++ {
						got, err := reader.Resolve(context.Background(), oid)
						if err != nil || got != oid {
							t.Fatalf("resolve %s %v", got, err)
						}
					}
					if count()-before != 1 {
						t.Fatal("resolve repeated child")
					}
				}
				var blob string
				for _, path := range []string{"file", "docs/rule.txt", "missing"} {
					before := count()
					first, exists, err := reader.LookupTreeEntry(context.Background(), target, path)
					if err != nil || exists != (path != "missing") {
						t.Fatalf("lookup %v %v", exists, err)
					}
					second, again, err := reader.LookupTreeEntry(context.Background(), target, path)
					// One streamed commit/tree batch verifies every traversed
					// component (CEM-CB-023); the memo hit spawns nothing.
					if err != nil || second != first || exists != again || count()-before != 1 {
						t.Fatalf("lookup path=%s parity=%t exists=%t/%t err=%v children=%d", path, second == first, exists, again, err, count()-before)
					}
					if path == "file" {
						blob = first.OID
					}
				}
				before := count()
				first, err := reader.BlobBytes(context.Background(), blob)
				if err != nil || string(first) != "new\n" {
					t.Fatalf("blob %q %v", first, err)
				}
				first[0] = '!'
				second, err := reader.BlobBytes(context.Background(), blob)
				if err != nil || string(second) != "new\n" {
					t.Fatal("miss bytes alias memo")
				}
				second[0] = '?'
				third, err := reader.BlobBytes(context.Background(), blob)
				if err != nil || string(third) != "new\n" || count()-before != 1 || reader.blobBytes != 4 {
					t.Fatal("hit bytes or distinct budget")
				}
				before = count()
				diff, err := reader.CanonicalDiff(context.Background(), base, target)
				if err != nil || !bytes.Contains(diff, []byte("-old\n+new\n")) {
					t.Fatalf("diff %q %v", diff, err)
				}
				want := bytes.Clone(diff)
				diff[0] = '!'
				// A first diff spawns the lazy format probe, the diff, one tree
				// batch per differing directory level (one here: "file" is at the
				// root), and one blob batch for the proof: 1 + 1 + 1 + 1.
				again, err := reader.CanonicalDiff(context.Background(), base, target)
				if err != nil || !bytes.Equal(again, want) || count()-before != 4 {
					t.Fatal("diff copy, patch proof, or lazy format")
				}
				again[0] = '?'
				other, err := memo.Open(gitrun.NewDefaultBudget())
				if err != nil {
					t.Fatal(err)
				}
				before = count()
				got, err := other.CanonicalDiff(context.Background(), base, target)
				if err != nil || !bytes.Equal(got, want) || count()-before != 1 {
					t.Fatal("second reader lost lazy format or shared diff")
				}
				before = count()
				got, err = other.BlobBytes(context.Background(), blob)
				if err != nil || string(got) != "new\n" || count() != before || other.blobBytes != 4 {
					t.Fatal("independent blob charge")
				}
			})
		}
	})
}

func TestRequestMemoScopesReleaseAndIdentity(t *testing.T) {
	t.Run("PLE-V0-003 memo scope excludes references and released readers", func(t *testing.T) {
		memo, reader, _, base, target := memoFixture(t, "sha1")
		count := countMemoGit(t)
		for i := 0; i < 2; i++ {
			if _, err := reader.Resolve(context.Background(), "HEAD"); err != nil {
				t.Fatal(err)
			}
		}
		if count() != 2 || memo.count != 0 {
			t.Fatal("symbolic revision cached")
		}
		for i := 0; i < 2; i++ {
			if _, _, err := reader.LookupTreeEntry(context.Background(), "HEAD", "file"); err != nil {
				t.Fatal(err)
			}
		}
		if count() != 4 || memo.count != 0 {
			t.Fatalf("symbolic tree: children=%d memoEntries=%d", count(), memo.count)
		}
		if _, err := reader.Resolve(context.Background(), base); err != nil {
			t.Fatal(err)
		}
		for _, mutate := range []func(*Repository){func(r *Repository) { r.Root += "x" }, func(r *Repository) { r.GitDir += "x" }, func(r *Repository) { r.CommonDir += "x" }, func(r *Repository) { r.ObjectFormat = "sha256" }, func(r *Repository) { r.objectView = open(t, memo.root) }} {
			bad := *reader
			mutate(&bad)
			before := count()
			if _, err := bad.Resolve(context.Background(), base); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable || count() != before {
				t.Fatal("changed identity used memo")
			}
		}
		ordinary := open(t, memo.root)
		before := count()
		for i := 0; i < 2; i++ {
			if _, err := ordinary.Resolve(context.Background(), base); err != nil {
				t.Fatal(err)
			}
		}
		if count()-before != 2 {
			t.Fatal("ordinary repository cached")
		}
		freshView := open(t, memo.root)
		if err := freshView.LoadObjectFormat(context.Background()); err != nil {
			t.Fatal(err)
		}
		fresh, err := NewRequestReadMemo(freshView)
		if err != nil {
			t.Fatal(err)
		}
		defer fresh.Release()
		freshReader, err := fresh.Open(gitrun.NewDefaultBudget())
		if err != nil {
			t.Fatal(err)
		}
		before = count()
		if _, err := freshReader.Resolve(context.Background(), base); err != nil || count()-before != 1 {
			t.Fatal("session reused another request")
		}
		memo.Release()
		if memo.count != 0 || memo.payload != 0 || memo.entries != nil {
			t.Fatal("release retained values")
		}
		if _, err := memo.Open(gitrun.NewDefaultBudget()); err == nil {
			t.Fatal("released session opened reader")
		}
		before = count()
		for _, oid := range []string{base, target, base} {
			if _, err := reader.Resolve(context.Background(), oid); err != nil {
				t.Fatal(err)
			}
		}
		if count()-before != 3 || memo.count != 0 || memo.payload != 0 || memo.entries != nil {
			t.Fatal("released reader hit or refilled memo")
		}
	})
}

func TestRequestMemoLogicalBudgetPrecedence(t *testing.T) {
	t.Run("PLE-V0-003 cached reads preserve independent budgets and validation order", func(t *testing.T) {
		memo, reader, _, base, target := memoFixture(t, "sha1")
		ctx := context.Background()
		if _, err := reader.Resolve(ctx, base); err != nil {
			t.Fatal(err)
		}
		entry, _, err := reader.LookupTreeEntry(ctx, target, "file")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := reader.BlobBytes(ctx, entry.OID); err != nil {
			t.Fatal(err)
		}
		if _, err := reader.CanonicalDiff(ctx, base, target); err != nil {
			t.Fatal(err)
		}
		newReader := func(ops int, total time.Duration) *Repository {
			r, e := memo.Open(gitrun.NewBudget(ops, total))
			if e != nil {
				t.Fatal(e)
			}
			return r
		}
		limited := newReader(1, time.Minute)
		if _, err := limited.Resolve(ctx, base); err != nil {
			t.Fatal(err)
		}
		if _, err := limited.Resolve(ctx, base); cemcode.CodeOf(err) != cemcode.GitBudgetExceeded {
			t.Fatalf("hit budget: %v", err)
		}
		if _, err := newReader(1, time.Minute).Resolve(ctx, base); err != nil {
			t.Fatal("shared budget", err)
		}
		invalid := newReader(0, time.Minute)
		if _, err := invalid.Resolve(ctx, "-bad"); cemcode.CodeOf(err) != cemcode.InvalidArguments {
			t.Fatalf("argument precedence: %v", err)
		}
		if _, _, err := invalid.LookupTreeEntry(ctx, target, "../bad"); err == nil || cemcode.CodeOf(err) == cemcode.GitBudgetExceeded {
			t.Fatalf("path precedence: %v", err)
		}
		invalid.blobBytes = MaxTotalBlobBytes
		if _, err := invalid.BlobBytes(ctx, entry.OID); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
			t.Fatalf("blob precheck: %v", err)
		}
		post := newReader(1, time.Minute)
		post.blobBytes = MaxTotalBlobBytes - 1
		if _, err := post.BlobBytes(ctx, entry.OID); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable || !post.chargedOids[entry.OID] || post.blobBytes != MaxTotalBlobBytes+3 {
			t.Fatalf("blob postcharge: %v", err)
		}
		if _, err := post.BlobBytes(ctx, entry.OID); cemcode.CodeOf(err) != cemcode.GitBudgetExceeded {
			t.Fatalf("charged blob budget order: %v", err)
		}
		expired := newReader(5, 0)
		if _, err := expired.Resolve(ctx, base); cemcode.CodeOf(err) != cemcode.GitTimeout {
			t.Fatalf("total timeout: %v", err)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := newReader(5, time.Minute).Resolve(cancelled, base); cemcode.CodeOf(err) != cemcode.GitCancelled {
			t.Fatalf("cancelled hit: %v", err)
		}
		// The lazy format load is not a diff operation: its error stays unmapped.
		if _, err := newReader(5, 0).CanonicalDiff(ctx, base, target); cemcode.CodeOf(err) != cemcode.GitTimeout {
			t.Fatalf("format timeout mapped: %v", err)
		}
		diffReader := newReader(5, 0)
		diffReader.ObjectFormat = "sha1"
		if _, err := diffReader.CanonicalDiff(ctx, base, target); cemcode.CodeOf(err) != cemcode.GitDiffTimeout {
			t.Fatalf("diff timeout unmapped: %v", err)
		}
		if _, err := newReader(1, time.Minute).CanonicalDiff(ctx, base, target); cemcode.CodeOf(err) != cemcode.GitBudgetExceeded {
			t.Fatalf("format operation skipped: %v", err)
		}
	})
}

func TestRequestMemoFailuresAndEviction(t *testing.T) {
	t.Run("PLE-V0-003 errors are uncached and eviction rereads only the view", func(t *testing.T) {
		memo, reader, live, base, target := memoFixture(t, "sha1")
		entry, _, err := reader.LookupTreeEntry(context.Background(), target, "file")
		if err != nil {
			t.Fatal(err)
		}
		count := countMemoGit(t)
		for i := 0; i < 2; i++ {
			if _, err := reader.Resolve(context.Background(), strings.Repeat("f", 40)); cemcode.CodeOf(err) != cemcode.GitReadFailed {
				t.Fatalf("resolution failure: %v", err)
			}
		}
		if count() != 2 {
			t.Fatal("resolution error cached")
		}
		corruptLoose(t, memo.root, entry.OID, "blob", []byte("forged\n"))
		before := count()
		for i := 0; i < 2; i++ {
			if _, err := reader.BlobBytes(context.Background(), entry.OID); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
				t.Fatalf("corrupt blob: %v", err)
			}
		}
		if count()-before != 2 {
			t.Fatal("corruption cached")
		}
		p := filepath.Join(memo.root, ".git", "objects", entry.OID[:2], entry.OID[2:])
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		if _, err := open(t, live).BlobBytes(context.Background(), entry.OID); err != nil {
			t.Fatal("live object missing", err)
		}
		before = count()
		for i := 0; i < 2; i++ {
			if _, err := reader.BlobBytes(context.Background(), entry.OID); err == nil {
				t.Fatal("live fallback")
			}
		}
		if count()-before != 2 {
			t.Fatal("missing error cached")
		}
		if _, err := reader.Resolve(context.Background(), base); err != nil {
			t.Fatal(err)
		}
		// Private storage calls isolate admission-independent FIFO accounting;
		// their synthetic values never become authority or fixture results.
		for i := 0; i < memoMaxEntries; i++ {
			key := memoKey{memoResolve, fmt.Sprintf("%040x", i), ""}
			if err := reader.remember(context.Background(), key, memoValue{oid: key.first}); err != nil {
				t.Fatal(err)
			}
		}
		if memo.count != memoMaxEntries || memo.payload > memoMaxPayload {
			t.Fatal("entry cap")
		}
		before = count()
		if _, err := reader.Resolve(context.Background(), base); err != nil || count()-before != 1 {
			t.Fatal("eviction did not reread view")
		}
		large := make([]byte, memoMaxPayload)
		beforeEntries := memo.count
		if err := reader.remember(context.Background(), memoKey{memoBlob, entry.OID, ""}, memoValue{data: large}); err != nil || memo.count != beforeEntries {
			t.Fatal("oversize narrowed admission")
		}
		for _, key := range []string{"one", "two"} {
			if err := reader.remember(context.Background(), memoKey{memoBlob, key, ""}, memoValue{data: large[:memoMaxPayload/2]}); err != nil {
				t.Fatal(err)
			}
		}
		if memo.payload > memoMaxPayload || memo.count != 1 {
			t.Fatal("payload eviction")
		}
	})
}

func TestRequestMemoCopyCancellation(t *testing.T) {
	t.Run("PLE-V0-003 cached copy honors cancellation and operation timeout", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := copyMemoBytes(ctx, time.Time{}, []byte("x")); cemcode.CodeOf(err) != cemcode.GitCancelled {
			t.Fatal(err)
		}
		if _, err := copyMemoBytes(context.Background(), time.Now().Add(-time.Second), []byte("x")); cemcode.CodeOf(err) != cemcode.GitTimeout {
			t.Fatal(err)
		}
		ctx2 := &memoCancelContext{Context: context.Background(), after: 3}
		if _, err := copyMemoBytes(ctx2, time.Time{}, make([]byte, 3*(64<<10))); cemcode.CodeOf(err) != cemcode.GitCancelled {
			t.Fatal("copy failed to check between chunks", err)
		}
	})
}

type memoCancelContext struct {
	context.Context
	calls, after int
}

func (c *memoCancelContext) Err() error {
	c.calls++
	if c.calls > c.after {
		return context.Canceled
	}
	return nil
}
