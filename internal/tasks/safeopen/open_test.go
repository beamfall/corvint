//go:build darwin || linux

package safeopen

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func realTemp(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// All race helpers are in-process, joined before cleanup, and create no descendants.
func bounded(t *testing.T, fifo string, call func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- call() }()
	select {
	case err := <-done:
		return err
	case <-time.After(100 * time.Millisecond):
	}
	// Contain a regressed blocking FIFO open and join before failing the test.
	f, err := os.OpenFile(fifo, os.O_RDWR|syscall.O_NONBLOCK, 0)
	must(t, err)
	defer f.Close()
	select {
	case <-done:
		t.Fatal("opening replacement FIFO blocked")
	case <-time.After(time.Second):
		t.Fatal("FIFO opener did not exit after release")
	}
	return nil
}
func TestTMV0001_AS10_DirectorySwapNoRedirectOrBlock(t *testing.T) {
	for _, target := range []string{"leaf", "ancestor"} {
		for _, replacement := range []string{"symlink", "fifo"} {
			t.Run(target+"-"+replacement, func(t *testing.T) {
				base := realTemp(t)
				must(t, os.MkdirAll(filepath.Join(base, "parent", "leaf"), 0755))
				outside := filepath.Join(base, "outside")
				must(t, os.MkdirAll(filepath.Join(outside, "leaf"), 0755))
				swap := filepath.Join(base, "parent", "leaf")
				if target == "ancestor" {
					swap = filepath.Join(base, "parent")
				}
				hook := beforeOpen
				t.Cleanup(func() { beforeOpen = hook })
				fired := false
				beforeOpen = func(name string) {
					if name != filepath.Base(swap) || fired {
						return
					}
					fired = true
					must(t, os.Rename(swap, swap+"-original"))
					if replacement == "fifo" {
						must(t, syscall.Mkfifo(swap, 0600))
					} else {
						must(t, os.Symlink(outside, swap))
					}
				}
				err := bounded(t, swap, func() error {
					r, err := Root(filepath.Join(base, "parent", "leaf"))
					if r != nil {
						defer r.Close()
						if err == nil {
							return r.WriteFile("redirected", []byte("bad"), 0600)
						}
					}
					return err
				})
				if !fired || err == nil {
					t.Fatalf("swap accepted: fired=%v err=%v", fired, err)
				}
				if _, err := os.Stat(filepath.Join(outside, "redirected")); !os.IsNotExist(err) {
					t.Fatal("redirected publication")
				}
				if _, err := os.Stat(filepath.Join(outside, "leaf", "redirected")); !os.IsNotExist(err) {
					t.Fatal("ancestor redirected publication")
				}
			})
		}
	}
}
func TestTMV0001_AS10_PinnedRootSurvivesPathReplacement(t *testing.T) {
	base := realTemp(t)
	path := filepath.Join(base, "held")
	must(t, os.Mkdir(path, 0755))
	outside := filepath.Join(base, "outside")
	must(t, os.Mkdir(outside, 0755))
	root, err := Root(path)
	must(t, err)
	defer root.Close()
	must(t, os.Rename(path, path+"-original"))
	must(t, os.Symlink(outside, path))
	f, err := InRoot(root, "receipt", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600, false)
	must(t, err)
	_, err = f.Write([]byte("complete"))
	must(t, err)
	must(t, f.Close())
	if _, err := os.Stat(filepath.Join(outside, "receipt")); !os.IsNotExist(err) {
		t.Fatal("pinned root redirected")
	}
	if _, err := os.Stat(filepath.Join(path+"-original", "receipt")); err != nil {
		t.Fatal(err)
	}
}
func TestTMV0008_AS07_ReadReplacementFIFOAndSymlink(t *testing.T) {
	for _, replacement := range []string{"fifo", "symlink"} {
		t.Run(replacement, func(t *testing.T) {
			base := realTemp(t)
			path := filepath.Join(base, "record")
			must(t, os.WriteFile(path, []byte("old"), 0600))
			outside := filepath.Join(base, "secret")
			must(t, os.WriteFile(outside, []byte("secret"), 0600))
			root, err := Root(base)
			must(t, err)
			defer root.Close()
			hook := beforeOpen
			t.Cleanup(func() { beforeOpen = hook })
			fired := false
			beforeOpen = func(name string) {
				if name != "record" || fired {
					return
				}
				fired = true
				must(t, os.Remove(path))
				if replacement == "fifo" {
					must(t, syscall.Mkfifo(path, 0600))
				} else {
					must(t, os.Symlink(outside, path))
				}
			}
			err = bounded(t, path, func() error {
				f, err := InRoot(root, "record", os.O_RDONLY, 0, false)
				if err != nil {
					return err
				}
				defer f.Close()
				st, err := f.Stat()
				if err != nil {
					return err
				}
				if !st.Mode().IsRegular() {
					return syscall.EINVAL
				}
				return nil
			})
			if !fired || err == nil {
				t.Fatalf("unsafe file accepted: %v", err)
			}
		})
	}
}
func TestTMV0001_AS10_DescriptorLifetime(t *testing.T) {
	f, err := os.Open(realTemp(t))
	must(t, err)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Control(f, func(fd uintptr) error {
			close(entered)
			<-release
			var st syscall.Stat_t
			return syscall.Fstat(int(fd), &st)
		})
	}()
	<-entered
	closed := make(chan error, 1)
	go func() { closed <- f.Close() }()
	// Close may return before the last reference is dropped. Control must keep
	// the actual descriptor valid until its callback has finished.
	closeReturned := false
	select {
	case err := <-closed:
		must(t, err)
		closeReturned = true
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	must(t, <-done)
	if !closeReturned {
		must(t, <-closed)
	}
	if err := Control(f, func(uintptr) error { t.Fatal("used closed descriptor"); return nil }); err == nil {
		t.Fatal("closed control succeeded")
	}
}

func TestTMV0001_AS10_DescriptorBridgeFailsClosed(t *testing.T) {
	base := realTemp(t)
	other := realTemp(t)
	old := openBridge
	t.Cleanup(func() { openBridge = old })
	for _, scenario := range []string{"missing", "wrong-directory"} {
		t.Run(scenario, func(t *testing.T) {
			var leaked *os.Root
			openBridge = func(path string) (*os.Root, error) {
				if scenario == "missing" {
					return nil, os.ErrNotExist
				}
				r, err := os.OpenRoot(other)
				leaked = r
				return r, err
			}
			root, err := Root(base)
			if root != nil || err == nil {
				if root != nil {
					root.Close()
				}
				t.Fatal("bridge failure accepted")
			}
			if leaked != nil {
				if _, err := leaked.Stat("."); err == nil {
					t.Fatal("mismatched bridge handle leaked")
				}
			}
		})
	}
}
