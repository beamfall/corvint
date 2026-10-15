package parentverify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/godiscovery"
)

// V1-1429: scanTree hashes regular files on a bounded worker pool. Every file's digest must land
// in its own walk-order row, and the snapshot must not depend on how many hashers ran.
func TestScanTreeConcurrentHashingMatchesOneHasher(t *testing.T) {
	root := resolved(t, t.TempDir())
	contents := map[string]string{}
	for directory := range 12 {
		for file := range 25 {
			relative := filepath.ToSlash(filepath.Join(fmt.Sprintf("d%02d", directory), fmt.Sprintf("sub%d", file%3), fmt.Sprintf("f%02d.go", file)))
			body := fmt.Sprintf("package p // %d/%d %s", directory, file, string(make([]byte, directory*file*97)))
			path := filepath.Join(root, filepath.FromSlash(relative))
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			contents[relative] = body
		}
	}
	scan := func(procs int) treeSnapshot {
		t.Helper()
		defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(procs))
		snapshot, err := scanTree(t.Context(), root, godiscovery.NamespaceSource, ".", false)
		if err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	one := scan(1)
	many := scan(maxTreeHashers)
	if one.digest != many.digest || one.metadataDigest != many.metadataDigest {
		t.Fatalf("snapshot depends on hasher count: %s/%s vs %s/%s", one.digest, one.metadataDigest, many.digest, many.metadataDigest)
	}
	if len(many.inputs) != len(contents) {
		t.Fatalf("inputs = %d, want %d", len(many.inputs), len(contents))
	}
	for _, input := range many.inputs {
		want := sha256.Sum256([]byte(contents[input.logical]))
		if input.RawSHA256 != hex.EncodeToString(want[:]) {
			t.Fatalf("%s carries digest %s, want its own content's", input.logical, input.RawSHA256)
		}
	}
}

// V1-1429: finish reports exactly what the sequential walk met first: a queued file's read
// failure, then the cumulative MaxTreeBytes bound, and only then the walk's own failure, which
// the walk can meet only after every file it queued.
func TestTreeHashersFinishReportsFirstFailureInWalkOrder(t *testing.T) {
	walkErr := errors.New("walk failure after every queued file")
	for _, test := range []struct {
		name  string
		tasks []fileTask
		want  error
	}{
		{"walk failure after clean files", []fileTask{{count: 1}, {count: 2}}, walkErr},
		{"earliest file failure wins", []fileTask{{count: 1}, {err: ErrDrift}, {err: ErrUnavailable}}, ErrDrift},
		{"byte bound before a later file failure", []fileTask{{count: MaxTreeBytes}, {count: 1}, {err: ErrDrift}}, ErrLimit},
		{"file failure before the byte bound", []fileTask{{count: 1}, {err: ErrUnavailable}, {count: MaxTreeBytes}}, ErrUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			hashing := startTreeHashers(context.Background())
			var done sync.WaitGroup
			for index := range test.tasks {
				task := test.tasks[index]
				task.done = &done
				hashing.tasks = append(hashing.tasks, &task)
			}
			if err := hashing.finish(walkErr, nil, nil); !errors.Is(err, test.want) {
				t.Fatalf("finish = %v, want %v", err, test.want)
			}
		})
	}
}
