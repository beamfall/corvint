// SPDX-License-Identifier: Apache-2.0

package testreuse

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
)

// TestFilteredDeclarationRuns covers AFP-V0-2321: a clean filter maps a
// widened worktree read-scope declaration back to its committed blob, so Git
// reports it clean while the confinement wrapper, which loads the worktree
// file, would grant scopes the digest never bound. The declaration's raw
// worktree bytes must hash to the blob it was parsed from, or no package is
// eligible (worktree-bytes). The same filter over an unchanged declaration is
// the negative control.
func TestFilteredDeclarationRuns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the clean filter is a POSIX shell script")
	}
	root := fixtureRepo(t)
	filter := filepath.Join(t.TempDir(), "narrow")
	if err := execfile.WriteFile(filter, []byte("#!/bin/sh\nexec sed -e 's|\"b\":\\[\"data/\"\\]|\"b\":[]|'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, root, "config", "filter.narrow.clean", filter)
	writeFixture(t, root, ".gitattributes", ReadScopesPath+" filter=narrow\n")
	commitAll(t, root, "declaration filter")
	record := recordAt(t, root, Source{Kind: SourceLocal}, allPassed)
	wantUnits(t, "filtered but unchanged declaration", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonRecordDigestEqual, "b": ReasonRecordDigestEqual})
	writeFixture(t, root, ReadScopesPath, `{"profile":"corvint-test-read-scopes/0","packages":{"a":["data/a.txt"],"b":["data/"]}}`+"\n")
	git(t, root, "add", ReadScopesPath) // refreshes the index stat; the filtered blob is unchanged
	if status := git(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("the filtered declaration must be clean: %q", status)
	}
	units := decideAt(t, root, localPolicy, record)
	wantUnits(t, "widened worktree declaration", units, map[string]string{"a": ReasonDigestError, "b": ReasonDigestError})
	if !strings.HasPrefix(units["b"].Detail, ErrorWorktreeBytes+": "+ReadScopesPath) {
		t.Errorf("missing worktree-bytes detail: %+v", units["b"])
	}
}

// TestGrantedDirectoryEntriesRun covers AFP-V0-2321: Git reports no empty
// untracked directory and no special file, so a granted directory is bound
// only when its on-disk entries are exactly those the tree holds. A test that
// asserts os.ReadDir results fails after an empty directory appears, so the
// package must run (UNTRACKED_GRANTED_ENTRY), in its own subtree, in a
// granted scope directory, at a scope path the digest records as absent, and
// for a FIFO. The unchanged checkout is the negative control, and the
// undeclared-reader package b is reused throughout.
func TestGrantedDirectoryEntriesRun(t *testing.T) {
	root := fixtureRepo(t)
	writeFixture(t, root, "d/d.go", "package d\n")
	writeFixture(t, root, "d/d_test.go", "package d\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestD(t *testing.T) {\n\tfor _, dir := range []string{\"testdata\", \"../fixtures\"} {\n\t\tentries, err := os.ReadDir(dir)\n\t\tif err != nil || len(entries) != 1 {\n\t\t\tt.Fatalf(\"%s: %v %v\", dir, entries, err)\n\t\t}\n\t}\n}\n")
	writeFixture(t, root, "d/testdata/one.txt", "one\n")
	writeFixture(t, root, "fixtures/one.txt", "one\n")
	writeFixture(t, root, ReadScopesPath, `{"profile":"corvint-test-read-scopes/0","packages":{"a":["data/a.txt"],"b":[],"d":["fixtures/","gone/","missing.txt"]}}`+"\n")
	commitAll(t, root, "directory reader")
	goTest(t, root, true, "./d")
	record := recordAt(t, root, Source{Kind: SourceLocal}, map[string]string{fixtureModule + "/b": "pass", fixtureModule + "/d": "pass"})
	wantUnits(t, "unchanged directories", decideAt(t, root, localPolicy, record), map[string]string{"b": ReasonRecordDigestEqual, "d": ReasonRecordDigestEqual})
	mkdir := func(path string) error { return os.Mkdir(path, 0o755) }
	faults := []struct {
		path     string
		rerunErr bool
		create   func(string) error
	}{
		{"d/testdata/extra", true, mkdir},
		{"fixtures/extra", true, mkdir},
		{"gone", false, mkdir},
		{"missing.txt", false, mkdir},
	}
	if _, err := exec.LookPath("mkfifo"); err == nil && runtime.GOOS != "windows" {
		faults = append(faults, struct {
			path     string
			rerunErr bool
			create   func(string) error
		}{"fixtures/pipe", true, func(path string) error { return exec.Command("mkfifo", path).Run() }})
	}
	for _, fault := range faults {
		full := filepath.Join(root, filepath.FromSlash(fault.path))
		if err := fault.create(full); err != nil {
			t.Fatal(err)
		}
		units := decideAt(t, root, localPolicy, record)
		wantUnits(t, fault.path, units, map[string]string{"b": ReasonRecordDigestEqual, "d": ReasonUntrackedGrantedEntry})
		if !strings.HasPrefix(units["d"].Detail, fault.path+" ") {
			t.Errorf("%s: detail %q", fault.path, units["d"].Detail)
		}
		if fault.rerunErr {
			goTest(t, root, false, "./d")
		}
		if err := os.RemoveAll(full); err != nil {
			t.Fatal(err)
		}
	}
	wantUnits(t, "faults removed", decideAt(t, root, localPolicy, record), map[string]string{"b": ReasonRecordDigestEqual, "d": ReasonRecordDigestEqual})
}

// TestRecheckRevalidatesWorktreeBytes covers AFP-V0-2321: a harness file's
// worktree validation is cached for the observation, and an edit Git's stat
// check misses (same size, restored mtime, core.checkStat=minimal) leaves the
// status unchanged. The recheck rehashes every validated file uncached, so the
// edit is snapshot-drift; the recheck before the edit is the negative control.
func TestRecheckRevalidatesWorktreeBytes(t *testing.T) {
	root := fixtureRepo(t)
	git(t, root, "config", "core.checkStat", "minimal")
	git(t, root, "config", "core.trustctime", "false")
	ctx := context.Background()
	snapshot, err := Observe(ctx, ObserveOptions{Root: root, Env: fixtureEnv()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := observeConfined(ctx, snapshot, "", fixtureEnv(), fixtureArgv, []string{"data/a.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Recheck(ctx); err != nil {
		t.Fatalf("unchanged checkout: %v", err)
	}
	path := filepath.Join(root, "data/a.txt")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Age the index past racy-clean detection, then edit in place.
	old := info.ModTime().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	git(t, root, "update-index", "--refresh")
	snapshot, err = Observe(ctx, ObserveOptions{Root: root, Env: fixtureEnv()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := observeConfined(ctx, snapshot, "", fixtureEnv(), fixtureArgv, []string{"data/a.txt"}); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "data/a.txt", "no\n")
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if status := git(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("the edit must be hidden from status: %q", status)
	}
	if err := snapshot.Recheck(ctx); err == nil || !strings.HasPrefix(err.Error(), ErrorSnapshotDrift+": ") {
		t.Errorf("an edit hidden from status after validation: %v", err)
	}
}
