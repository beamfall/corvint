//go:build darwin || linux

package contextindex

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestBlobShardRefusesFIFOWithoutBlocking(t *testing.T) {
	built := taskContextFixture(t)
	source := built.Sources["cache/demux.go"]
	entry := treeEntry{source.Path, source.BlobHash, source.Mode, len(source.Data), false}
	target := blobShardPath(SnapshotDirectory(built.Root), built.ObjectFormat, analyzerEngine(), source.BlobHash, source.Path)
	t.Setenv("CORVINT_INDEX_SHARDS", "1")
	if _, err := WriteSnapshot(built); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(target, 0o600); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := readBlobFact(snapshotBase(built.Root), SnapshotDirectory(built.Root), built.ObjectFormat, analyzerEngine(), entry, nil)
		result <- err
	}()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("FIFO accepted")
		}
	case <-time.After(2 * time.Second):
		// Release a regressed blocking open before failing, leaving no goroutine.
		fd, err := syscall.Open(target, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			syscall.Close(fd)
		}
		select {
		case <-result:
		case <-time.After(time.Second):
		}
		t.Fatal("FIFO blocked blob shard read")
	}
}

func TestBlobShardPublicationPinsDirectoryAcrossSymlinkSwap(t *testing.T) {
	root := t.TempDir()
	relative := filepath.Join(".corvint", "index", "blobs", "schema")
	directory, err := openBlobShardDirectory(root, relative, true)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	original := filepath.Join(root, relative)
	parked := original + ".parked"
	outside := filepath.Join(root, "outside-cache")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(original, parked); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, original); err != nil {
		t.Fatal(err)
	}
	// This is exactly the publication phase after its directory was resolved.
	if err := publishBlobFactAt(directory, "pinned.afs", []byte("facts")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(parked, "pinned.afs")); err != nil {
		t.Fatal(err)
	}
	if err := publishBlobFact(root, filepath.Join(original, "refused.afs"), []byte("facts")); err == nil {
		t.Fatal("static directory symlink accepted")
	}
	files, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("publication escaped cache: %v", files)
	}
}

func TestBlobShardReadRefusesInRootLeafSymlink(t *testing.T) {
	root := t.TempDir()
	relative := filepath.Join(".corvint", "index", "blobs", "schema")
	directory, err := openBlobShardDirectory(root, relative, true)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	if err := publishBlobFactAt(directory, "real.afs", []byte("facts")); err != nil {
		t.Fatal(err)
	}
	// A swap after shardRegularPath's Lstat leaves this link for the open.
	link := filepath.Join(root, relative, "link.afs")
	if err := os.Symlink("real.afs", link); err != nil {
		t.Fatal(err)
	}
	file, err := openBlobShard(root, link)
	if err == nil {
		file.Close()
		t.Fatal("in-root leaf symlink opened")
	}
}
