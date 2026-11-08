// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProposeBatchUsesTwoProcessesPerReadSet(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process counter uses a POSIX shell wrapper; parity tests are portable")
	}
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	files, names := map[string]string{}, []string{}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("dir%d/evidence.txt", i)
		files[name] = fmt.Sprintf("evidence %d\n", i)
		names = append(names, name)
	}
	claimsCommit(t, root, "base", files)
	base := testGit(t, root, "rev-parse", "HEAD")
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	wrapper := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + quote(log) + "\nexec " + quote(git) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	p := &proposal{flow: flow{ctx: context.Background(), root: root, stderr: &bytes.Buffer{}}, blobs: map[string][]byte{}}
	p.prefetchBlobs(base, names)
	for _, name := range names {
		if data, present := p.blob(base, name); !present || string(data) != files[name] {
			t.Fatalf("prefetched %s = %v %q", name, present, data)
		}
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(data, []byte{'\n'}) != 2 || !bytes.Contains(data, []byte("ls-tree")) || !bytes.Contains(data, []byte("cat-file --batch")) {
		t.Fatalf("20 blobs should use one listing and one batch, got:\n%s", data)
	}
}

// CKC-V0-012 / V1-4852: compare both public flows with per-file reads,
// including Git versions that cannot use the objectmode batch reader.
func TestProposeDraftAndRefreshBatchParity(t *testing.T) {
	root, base := claimsRepo(t)
	oldPrefetch, oldBatch := prefetchBlobReads, batchBlobReads
	t.Cleanup(func() { prefetchBlobReads, batchBlobReads = oldPrefetch, oldBatch })
	batchBlobReads = false
	compare := func(refresh string) string {
		t.Helper()
		prefetchBlobReads = false
		wantCode, wantOut, wantErr := claimsRefresh(t, root, base, refresh)
		prefetchBlobReads = true
		gotCode, gotOut, gotErr := claimsRefresh(t, root, base, refresh)
		if wantCode != 0 || gotCode != wantCode || gotOut != wantOut || gotErr != wantErr {
			t.Fatalf("refresh=%q: batch %d %s %s; per-file %d %s %s", refresh, gotCode, gotOut, gotErr, wantCode, wantOut, wantErr)
		}
		return gotOut
	}
	draft := compare("")
	compare(writeInput(t, draft))
	// Evidence from distinct unchanged paths bypasses anchor-search prefetch.
	// Both commits are prefetched before any of these paths are consumed.
	p := &proposal{flow: flow{ctx: context.Background(), root: root, base: base, target: testGit(t, root, "rev-parse", "HEAD"), stderr: &bytes.Buffer{}}, blobs: map[string][]byte{}}
	p.refreshRows = []refreshRow{{rest: "docs/spec.md\t3:3\tspecification"}, {rest: "a.go\t1:1\timplementation"}, {rest: "absent.txt\t1:1\tspecification"}}
	p.prefetchRefreshBlobs()
	for _, commit := range []string{p.base, p.target} {
		for _, name := range []string{"docs/spec.md", "a.go", "absent.txt"} {
			if _, cached := p.blobs[commit+":"+name]; !cached {
				t.Errorf("refresh did not prefetch %s:%s", commit, name)
			}
		}
	}
}

func TestProposeBatchRejectsMalformedRecords(t *testing.T) {
	first := "aaa blob 2\na\n\n"
	for _, tail := range []string{"", "bbb missing\n", "bbb blob 2\nb", "bbb blob 2\nb\nx", "aaa blob 2\na\n\n", "ccc blob 2\nc\n\n", "bbb tree 2\nb\n\n", "bbb blob 9223372036854775807\nb\n\n"} {
		p := &proposal{blobs: map[string][]byte{}}
		p.cacheBlobBatch("base", []byte(first+tail), map[string][]string{"aaa": {"a"}, "bbb": {"b"}})
		if len(p.blobs) != 0 {
			t.Errorf("malformed tail %q published a partial batch: %v", tail, p.blobs)
		}
	}
	p := &proposal{blobs: map[string][]byte{}}
	p.cacheBlobBatch("base", []byte(first+"bbb blob 0\n\n"), map[string][]string{"aaa": {"a", "alias"}, "bbb": {"b"}})
	if string(p.blobs["base:a"]) != "a\n" || string(p.blobs["base:alias"]) != "a\n" || p.blobs["base:b"] == nil {
		t.Fatalf("valid batch lost bytes, alias or empty file: %v", p.blobs)
	}
}

func TestProposeBatchMissingAndCorruptBlobFailClosed(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprint(corrupt), func(t *testing.T) {
			root := t.TempDir()
			testGit(t, root, "init", "-q")
			claimsCommit(t, root, "base", map[string]string{"evidence.txt": "unique evidence bytes\n"})
			base := testGit(t, root, "rev-parse", "HEAD")
			oid := testGit(t, root, "rev-parse", base+":evidence.txt")
			object := filepath.Join(root, ".git", "objects", oid[:2], oid[2:])
			if err := os.Remove(object); err != nil {
				t.Fatal(err)
			}
			if corrupt {
				if err := os.WriteFile(object, []byte("corrupt object\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, batch := range []bool{false, true} {
				p := &proposal{flow: flow{ctx: context.Background(), root: root, base: base, stderr: &bytes.Buffer{}}, blobs: map[string][]byte{}, batchBlobs: batch}
				if batch {
					p.prefetchBlobs(base, []string{"evidence.txt"})
				}
				data, present := p.blob(base, "evidence.txt")
				p.closeBlobBatch()
				if present || data != nil {
					t.Fatalf("batch=%v admitted damaged evidence: %v %q", batch, present, data)
				}
				if got := p.citeRefusal("evidence.txt", "1:1", nil, nil); !strings.Contains(got, "is absent at BASE") {
					t.Fatalf("damaged evidence lacks a named refusal: %q", got)
				}
			}
		})
	}
}
