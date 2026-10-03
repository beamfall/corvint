//go:build darwin || linux

// SPDX-License-Identifier: AGPL-3.0-or-later
// Derived from internal/tasks/safeopen/open_test.go at
// 29a6db884ed795f7694c316433896d190e1ab508; kept private for decision 0397.
package postmergeworkflow

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func nativeOpenTemp(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func nativeOpenMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// All race helpers are in-process, joined before cleanup, and create no descendants.
func nativeOpenBounded(t *testing.T, fifo string, call func() error) error {
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
	nativeOpenMust(t, err)
	defer f.Close()
	select {
	case <-done:
		t.Fatal("opening replacement FIFO blocked")
	case <-time.After(time.Second):
		t.Fatal("FIFO opener did not exit after release")
	}
	return nil
}
func TestNativeOpenDirectorySwapNoRedirectOrBlock(t *testing.T) {
	for _, target := range []string{"leaf", "ancestor"} {
		for _, replacement := range []string{"symlink", "fifo"} {
			t.Run(target+"-"+replacement, func(t *testing.T) {
				base := nativeOpenTemp(t)
				nativeOpenMust(t, os.MkdirAll(filepath.Join(base, "parent", "leaf"), 0755))
				outside := filepath.Join(base, "outside")
				nativeOpenMust(t, os.MkdirAll(filepath.Join(outside, "leaf"), 0755))
				swap := filepath.Join(base, "parent", "leaf")
				if target == "ancestor" {
					swap = filepath.Join(base, "parent")
				}
				hook := nativeBeforeOpen
				t.Cleanup(func() { nativeBeforeOpen = hook })
				fired := false
				nativeBeforeOpen = func(name string) {
					if name != filepath.Base(swap) || fired {
						return
					}
					fired = true
					nativeOpenMust(t, os.Rename(swap, swap+"-original"))
					if replacement == "fifo" {
						nativeOpenMust(t, syscall.Mkfifo(swap, 0600))
					} else {
						nativeOpenMust(t, os.Symlink(outside, swap))
					}
				}
				err := nativeOpenBounded(t, swap, func() error {
					r, err := nativeOpenRoot(filepath.Join(base, "parent", "leaf"))
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
func TestNativeOpenPinnedRootSurvivesPathReplacement(t *testing.T) {
	base := nativeOpenTemp(t)
	path := filepath.Join(base, "held")
	nativeOpenMust(t, os.Mkdir(path, 0755))
	outside := filepath.Join(base, "outside")
	nativeOpenMust(t, os.Mkdir(outside, 0755))
	root, err := nativeOpenRoot(path)
	nativeOpenMust(t, err)
	defer root.Close()
	nativeOpenMust(t, os.Rename(path, path+"-original"))
	nativeOpenMust(t, os.Symlink(outside, path))
	f, err := nativeOpenInRoot(root, "receipt", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600, false)
	nativeOpenMust(t, err)
	_, err = f.Write([]byte("complete"))
	nativeOpenMust(t, err)
	nativeOpenMust(t, f.Close())
	if _, err := os.Stat(filepath.Join(outside, "receipt")); !os.IsNotExist(err) {
		t.Fatal("pinned root redirected")
	}
	if _, err := os.Stat(filepath.Join(path+"-original", "receipt")); err != nil {
		t.Fatal(err)
	}
}
func TestNativeOpenReadReplacementFIFOAndSymlink(t *testing.T) {
	for _, replacement := range []string{"fifo", "symlink"} {
		t.Run(replacement, func(t *testing.T) {
			base := nativeOpenTemp(t)
			path := filepath.Join(base, "record")
			nativeOpenMust(t, os.WriteFile(path, []byte("old"), 0600))
			outside := filepath.Join(base, "secret")
			nativeOpenMust(t, os.WriteFile(outside, []byte("secret"), 0600))
			root, err := nativeOpenRoot(base)
			nativeOpenMust(t, err)
			defer root.Close()
			hook := nativeBeforeOpen
			t.Cleanup(func() { nativeBeforeOpen = hook })
			fired := false
			nativeBeforeOpen = func(name string) {
				if name != "record" || fired {
					return
				}
				fired = true
				nativeOpenMust(t, os.Remove(path))
				if replacement == "fifo" {
					nativeOpenMust(t, syscall.Mkfifo(path, 0600))
				} else {
					nativeOpenMust(t, os.Symlink(outside, path))
				}
			}
			err = nativeOpenBounded(t, path, func() error {
				f, err := nativeOpenInRoot(root, "record", os.O_RDONLY, 0, false)
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
func TestNativeOpenDescriptorLifetime(t *testing.T) {
	f, err := os.Open(nativeOpenTemp(t))
	nativeOpenMust(t, err)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- nativeOpenControl(f, func(fd uintptr) error {
			close(entered)
			<-release
			var st syscall.Stat_t
			return syscall.Fstat(int(fd), &st)
		})
	}()
	<-entered
	closed := make(chan error, 1)
	go func() { closed <- f.Close() }()
	// Close may return before the last reference is dropped. nativeOpenControl nativeOpenMust keep
	// the actual descriptor valid until its callback has finished.
	closeReturned := false
	select {
	case err := <-closed:
		nativeOpenMust(t, err)
		closeReturned = true
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	nativeOpenMust(t, <-done)
	if !closeReturned {
		nativeOpenMust(t, <-closed)
	}
	if err := nativeOpenControl(f, func(uintptr) error { t.Fatal("used closed descriptor"); return nil }); err == nil {
		t.Fatal("closed control succeeded")
	}
}

func TestNativeOpenDescriptorBridgeFailsClosed(t *testing.T) {
	base := nativeOpenTemp(t)
	other := nativeOpenTemp(t)
	old := nativeOpenBridge
	t.Cleanup(func() { nativeOpenBridge = old })
	for _, scenario := range []string{"missing", "wrong-directory"} {
		t.Run(scenario, func(t *testing.T) {
			var leaked *os.Root
			nativeOpenBridge = func(path string) (*os.Root, error) {
				if scenario == "missing" {
					return nil, os.ErrNotExist
				}
				r, err := os.OpenRoot(other)
				leaked = r
				return r, err
			}
			root, err := nativeOpenRoot(base)
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

// PMR-V1-001: exclusive retention must not overwrite an existing name or alias.
func TestNativeOpenExclusiveCreation(t *testing.T) {
	base := nativeOpenTemp(t)
	target := filepath.Join(base, "original")
	nativeOpenMust(t, os.WriteFile(target, []byte("retained"), 0600))
	nativeOpenMust(t, os.Link(target, filepath.Join(base, "hardlink")))
	nativeOpenMust(t, os.Symlink(target, filepath.Join(base, "symlink")))
	root, err := nativeOpenRoot(base)
	nativeOpenMust(t, err)
	defer root.Close()
	for _, name := range []string{"original", "hardlink", "symlink"} {
		f, err := nativeOpenInRoot(root, name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600, false)
		if f != nil {
			f.Close()
		}
		if err == nil {
			t.Fatalf("existing %s admitted", name)
		}
	}
	data, err := os.ReadFile(target)
	nativeOpenMust(t, err)
	if string(data) != "retained" {
		t.Fatal("existing evidence changed")
	}
}
