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

// V1-1429 review: a file queued before the position whose prefix crossed MaxTreeBytes is still
// read, so its own failure is the one reported, while a file queued after that position is
// skipped. The hashes run in the adversarial order: the later file crosses the bound first.
func TestTreeHashersReadEveryFileBeforeTheLimitPosition(t *testing.T) {
	directory := resolved(t, t.TempDir())
	for name, body := range map[string]string{"drifted": "first", "other": "second", "small": "0123456789"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	info := func(name string) os.FileInfo {
		t.Helper()
		value, err := os.Lstat(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	var done sync.WaitGroup
	done.Add(3)
	earlier := &fileTask{root: root, name: "drifted", info: info("other"), seq: 0, done: &done}
	crossing := &fileTask{root: root, name: "small", info: info("small"), seq: 1, done: &done}
	later := &fileTask{root: root, name: "absent", info: info("small"), seq: 2, done: &done}
	hashing := &treeHashers{ctx: context.Background(), work: make(chan *fileTask), tasks: []*fileTask{earlier, crossing, later}}
	hashing.total = MaxTreeBytes - 5
	hashing.hash(crossing)
	hashing.hash(earlier)
	hashing.hash(later)
	if !errors.Is(earlier.err, ErrDrift) {
		t.Fatalf("file before the limit position = %v, want its own ErrDrift", earlier.err)
	}
	if !errors.Is(later.err, ErrLimit) {
		t.Fatalf("file after the limit position = %v, want skipped with ErrLimit", later.err)
	}
	if err := hashing.finish(nil, nil, nil); !errors.Is(err, ErrDrift) {
		t.Fatalf("finish = %v, want the earlier file's ErrDrift", err)
	}
}
