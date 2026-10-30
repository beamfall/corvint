package affected

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TJAA-V0-100 (GitHub #1750): a held exclusion skips ignored files and wholly ignored directories
// in every disk walk of its root, a conflicting hold refuses, and release restores the walk.
func TestHoldIgnoredExclusionSkipsWalk_TJAA_V0_100(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"src/a.ts", "report/trace/b.js", "report-keep/c.js", "out/d.js", "out/e.js"} {
		full := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	all := []string{"out/d.js", "out/e.js", "report-keep/c.js", "report/trace/b.js", "src/a.ts"}
	walk := func() []string {
		t.Helper()
		plain, err := SourceFiles(root, acceptEveryName)
		if err != nil {
			t.Fatal(err)
		}
		files, err := DiskSourceContext(context.Background(), root).Files(acceptEveryName)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(plain, files) {
			t.Fatalf("disk walks disagree: %v %v", plain, files)
		}
		return files
	}
	if got := walk(); !slices.Equal(got, all) {
		t.Fatalf("unheld walk=%v", got)
	}
	release, err := HoldIgnoredExclusion(root, []string{"report/", "out/e.js", "report/"})
	if err != nil {
		t.Fatal(err)
	}
	if got := walk(); !slices.Equal(got, []string{"out/d.js", "report-keep/c.js", "src/a.ts"}) {
		t.Fatalf("held walk=%v", got)
	}
	again, err := HoldIgnoredExclusion(root, []string{"out/e.js", "report/"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := HoldIgnoredExclusion(root, []string{"report/"}); !errors.Is(err, ErrIgnoredHoldConflict) {
		t.Fatalf("conflicting hold err=%v", err)
	}
	if match := HeldIgnoredMatcher(root); match == nil || !match("report/trace/x.js") || match("report-keep/c.js") {
		t.Fatal("matcher does not follow the hold")
	}
	again()
	again()
	if got := walk(); len(got) != 3 {
		t.Fatalf("walk after one of two releases=%v", got)
	}
	release()
	if got := walk(); !slices.Equal(got, all) || HeldIgnoredMatcher(root) != nil {
		t.Fatalf("released walk=%v", got)
	}
	// An entry Git decoded lossily, or that is no valid relative path, excludes nothing.
	lossy, err := HoldIgnoredExclusion(root, []string{"out/\uFFFD/", "../out/", "out\x00/"})
	if err != nil {
		t.Fatal(err)
	}
	defer lossy()
	if got := walk(); !slices.Equal(got, all) || HeldIgnoredMatcher(root) != nil {
		t.Fatalf("lossy hold walk=%v", got)
	}
}
