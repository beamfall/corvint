//go:build darwin

package authoritystore

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestDarwinLiveMetadataReadsOwnIdentity(t *testing.T) {
	p, err := inspectProcess(uint32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	if p.PID != uint32(os.Getpid()) || p.Parent != uint32(os.Getppid()) || p.Path == "" {
		t.Fatalf("invalid identity metadata")
	}
	hash, err := codeDirectoryHash(uint32(os.Getpid()))
	if err != nil || len(hash) != 40 {
		t.Fatalf("live signed process metadata unavailable: %v", err)
	}
	if _, err = inspectProcess(0xffffffff); err == nil {
		t.Fatal("unknown process admitted")
	}
}

func TestDarwinDescriptorRefusesOwnerModeLinksAndSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("data"), 0444); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	owner := uint32(os.Getuid())
	if err = auditDescriptor(f.Fd(), owner, false, true); err != nil {
		t.Fatal(err)
	}
	if err = auditDescriptor(f.Fd(), owner+1, false, true); err == nil {
		t.Fatal("wrong owner accepted")
	}
	if err = os.Chmod(path, 0666); err != nil {
		t.Fatal(err)
	}
	if err = auditDescriptor(f.Fd(), owner, false, false); err == nil {
		t.Fatal("writable file accepted")
	}
	if err = os.Chmod(path, 0444); err != nil {
		t.Fatal(err)
	}
	if err = os.Link(path, path+"-link"); err != nil {
		t.Fatal(err)
	}
	if err = auditDescriptor(f.Fd(), owner, false, true); err == nil {
		t.Fatal("hardlink accepted")
	}
	if err = os.Symlink("file", path+"-symlink"); err != nil {
		t.Fatal(err)
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if fd, err := openAt(dir.Fd(), "file-symlink", syscall.O_RDONLY|syscall.O_NOFOLLOW); err == nil {
		syscall.Close(fd)
		t.Fatal("symlink followed")
	}
}
