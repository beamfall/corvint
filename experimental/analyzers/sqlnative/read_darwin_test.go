//go:build darwin

package sqlnative

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestReadNoFollowAnchorsDescriptorsAcrossSwapAndCWDChange(t *testing.T) {
	root := t.TempDir()
	victim := filepath.Join(root, "victim")
	attacker := filepath.Join(root, "attacker")
	if err := os.MkdirAll(filepath.Join(victim, "internal", "store"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(victim, "internal", "store", "query.sql"), []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	authority := testReadAuthority(t, victim)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(victim); err != nil {
		t.Fatal(err)
	}

	original := openAt
	calls := 0
	openAt = func(dirfd int, name string, flags int, mode uint32) (int, error) {
		fd, err := original(dirfd, name, flags, mode)
		calls++
		if calls != 1 || err != nil {
			return fd, err
		}
		if err := os.Rename(filepath.Join(victim, "internal"), filepath.Join(victim, "held-internal")); err != nil {
			_ = syscall.Close(fd)
			return 0, err
		}
		if err := os.MkdirAll(filepath.Join(victim, "internal", "store"), 0700); err != nil {
			_ = syscall.Close(fd)
			return 0, err
		}
		if err := os.WriteFile(filepath.Join(victim, "internal", "store", "query.sql"), []byte("attacker"), 0600); err != nil {
			_ = syscall.Close(fd)
			return 0, err
		}
		if err := os.MkdirAll(attacker, 0700); err != nil {
			_ = syscall.Close(fd)
			return 0, err
		}
		if err := os.Chdir(attacker); err != nil {
			_ = syscall.Close(fd)
			return 0, err
		}
		return fd, nil
	}
	defer func() { openAt = original }()

	got, err := ReadNoFollow(context.Background(), authority, "internal/store/query.sql", 32)
	if err == nil && string(got) != "safe" {
		t.Fatalf("swap escaped descriptor root: read=%q", got)
	}
	if err != nil && err.Error() != "input identity changed" {
		t.Fatalf("unexpected swap result read=%q err=%v", got, err)
	}
}

func TestReadNoFollowCancellationInterruptsRead(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "store"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "store", "query.sql"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	authority := testReadAuthority(t, root)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	original := newReadFile
	started := make(chan struct{})
	newReadFile = func(fd uintptr, _ string) *os.File {
		_ = syscall.Close(int(fd))
		close(started)
		return reader
	}
	defer func() { newReadFile = original }()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := ReadNoFollow(ctx, authority, "internal/store/query.sql", 32)
		result <- err
	}()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("read cancellation=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt read")
	}
}

func TestReadNoFollowClosesHeldDescriptorsOnOpenFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "store"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("query.sql", filepath.Join(root, "internal", "store", "linked.sql")); err != nil {
		t.Fatal(err)
	}
	authority := testReadAuthority(t, root)
	before, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 128; i++ {
		if _, err := ReadNoFollow(context.Background(), authority, "internal/store/linked.sql", 32); err == nil {
			t.Fatal("followed symlink")
		}
	}
	after, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) > len(before)+4 {
		t.Fatalf("descriptor growth before=%d after=%d", len(before), len(after))
	}
}
