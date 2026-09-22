package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const sharedQueryRequest = `{"operations":[{"id":"q","verb":"query","task":"fix the demux key split in the cache reader"},{"id":"c","verb":"context","task":"fix the demux key split","subject":"cache/demux.go"}]}`

func runQueryForSharedTest(t *testing.T, root string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	arguments := []string{"--root", root, "query", "--task", "fix the demux key split in the cache reader"}
	if code := runContext(context.Background(), arguments, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("query exit %d: %s", code, stderr.String())
	}
	return stdout.Bytes()
}

// TestQuerySharedObservationEmitsByteIdenticalOutput pins the proposed shared
// query bracket: with a snapshot present, standalone query and batch emit the
// same bytes with CORVINT_QUERY_SHARED_OBSERVATION set as without it, on a clean
// and on a mixed worktree, and the shared path spawns fewer Git processes: on a
// clean worktree the learn stage's opening pair and the trace read's two pairs
// (six processes) fold into the loader's, and standalone query's loader takes
// no closing identity read (the bracket's closing is that read; batch keeps
// it for its context and impact operations); on a mixed worktree the trace
// read is blocked before any bracket, so only the learn stage's pair and that
// closing read fold. The fixture's simple metadata spawns no status probes, so
// their reuse is pinned in gitstatus instead.
func TestQuerySharedObservationEmitsByteIdenticalOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test shim uses a POSIX shell")
	}
	for name, test := range map[string]struct {
		dirty                                                bool
		queryDefault, queryShared, batchDefault, batchShared int
	}{
		"clean":          {false, 11, 6, 12, 8},
		"mixed-worktree": {true, 8, 5, 9, 7},
	} {
		t.Run(name, func(t *testing.T) {
			root := batchRepository(t)
			runIndexForTest(t, root, false)
			if test.dirty {
				if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("scratch\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			path, spawns := countingGit(t)
			t.Setenv("PATH", strings.TrimPrefix(path, "PATH="))
			t.Setenv("CORVINT_QUERY_SHARED_OBSERVATION", "")
			defaultQuery := runQueryForSharedTest(t, root)
			defaultQuerySpawns := spawns()
			_, defaultBatch, _ := runBatchForTest(t, root, sharedQueryRequest)
			defaultBatchSpawns := spawns()
			t.Setenv("CORVINT_QUERY_SHARED_OBSERVATION", "1")
			sharedQuery := runQueryForSharedTest(t, root)
			sharedQuerySpawns := spawns()
			_, sharedBatch, _ := runBatchForTest(t, root, sharedQueryRequest)
			sharedBatchSpawns := spawns()
			if !bytes.Equal(defaultQuery, sharedQuery) {
				t.Fatalf("query output differs\ndefault: %s\nshared:  %s", defaultQuery, sharedQuery)
			}
			if !bytes.Equal(defaultBatch, sharedBatch) {
				t.Fatalf("batch output differs\ndefault: %s\nshared:  %s", defaultBatch, sharedBatch)
			}
			if !bytes.Contains(defaultQuery, []byte(`"ok":true`)) || !bytes.Contains(defaultBatch, []byte(`"ok":true`)) {
				t.Fatalf("query or batch did not succeed\nquery: %s\nbatch: %s", defaultQuery, defaultBatch)
			}
			if defaultQuerySpawns != test.queryDefault || sharedQuerySpawns != test.queryShared ||
				defaultBatchSpawns != test.batchDefault || sharedBatchSpawns != test.batchShared {
				t.Fatalf("Git spawns query default %d (want %d) shared %d (want %d), batch default %d (want %d) shared %d (want %d)",
					defaultQuerySpawns, test.queryDefault, sharedQuerySpawns, test.queryShared,
					defaultBatchSpawns, test.batchDefault, sharedBatchSpawns, test.batchShared)
			}
		})
	}
}
