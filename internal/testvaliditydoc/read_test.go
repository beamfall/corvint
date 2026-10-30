package testvaliditydoc

import (
	"os"
	"path/filepath"
	"testing"
)

// MTV-V0-003/LPCV-V0-051: an opened replacement cannot inherit the old identity.
func TestReceiptRejectsReplacedFile(t *testing.T) {
	for _, replacement := range []string{"regular", "symlink-to-original"} {
		t.Run(replacement, func(t *testing.T) {
			directory := t.TempDir()
			name := filepath.Join(directory, "receipt")
			if err := os.WriteFile(name, []byte("before"), 0600); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			before, err := root.Lstat("receipt")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(name, name+".old"); err != nil {
				t.Fatal(err)
			}
			if replacement == "regular" {
				err = os.WriteFile(name, []byte("after"), 0600)
			} else {
				err = os.Symlink(name+".old", name)
			}
			if err != nil {
				t.Fatal(err)
			}
			if data, err := readRegular(root, "receipt", before); err == nil || len(data) != 0 {
				t.Fatalf("data=%q err=%v", data, err)
			}
		})
	}
}

// MTV-V0-006/LPCV-V0-051: enforce the bound on fd bytes even after size growth.
func TestReceiptBoundsOpenedFile(t *testing.T) {
	directory := t.TempDir()
	name := filepath.Join(directory, "receipt")
	if err := os.WriteFile(name, nil, 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	before, err := root.Lstat("receipt")
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int64{MaxInputBytes, MaxInputBytes + 1} {
		if err := os.Truncate(name, size); err != nil {
			t.Fatal(err)
		}
		data, err := readRegular(root, "receipt", before)
		if size == MaxInputBytes && (err != nil || len(data) != MaxInputBytes) {
			t.Fatalf("len=%d err=%v", len(data), err)
		}
		if size > MaxInputBytes && (err == nil || len(data) != 0) {
			t.Fatalf("len=%d err=%v", len(data), err)
		}
	}
}

// MTV-V0-003: os.Root follows an in-root directory symlink despite O_NOFOLLOW,
// so a resolved component that became a symlink must still be refused.
func TestReceiptRejectsInRootDirectorySymlink(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "real"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "real", "receipt"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(directory, "link")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if data, err := ReadFile(root, filepath.Join("link", "receipt")); err == nil || len(data) != 0 {
		t.Fatalf("data=%q err=%v", data, err)
	}
}
