//go:build darwin || linux

package safeopen

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// CAL-V0-070: one component opened beneath a PinDir descriptor is the same
// file, or the same error text, as InRoot opening it, including after the
// directory path itself is replaced.
func TestCALV0070_PinnedDirOpensMatchInRoot(t *testing.T) {
	base := realTemp(t)
	dir := filepath.Join(base, "d")
	must(t, os.Mkdir(dir, 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "file.json"), []byte("{}\n"), 0o600))
	must(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "sub", "inner"), nil, 0o600))
	must(t, os.Symlink(filepath.Join(dir, "file.json"), filepath.Join(dir, "link")))
	must(t, os.Symlink(filepath.Join(dir, "absent"), filepath.Join(dir, "dangling")))
	must(t, syscall.Mkfifo(filepath.Join(dir, "fifo"), 0o600))
	root, err := Root(dir)
	must(t, err)
	defer root.Close()
	pin, err := PinDir(root)
	must(t, err)
	defer pin.Close()

	// The pinned descriptor outlives the path: both forms keep reading the
	// original directory after it is renamed and replaced.
	must(t, os.Rename(dir, filepath.Join(base, "moved")))
	must(t, os.Mkdir(dir, 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "file.json"), []byte("replacement\n"), 0o600))

	for _, name := range []string{"file.json", "absent", "link", "dangling", "fifo", "sub", ".", "..", "", "/abs", "x\\y", "./file.json"} {
		want, wantErr := InRoot(root, name, os.O_RDONLY, 0, false)
		got, gotErr := InDir(pin, name, os.O_RDONLY, 0)
		if (wantErr == nil) != (gotErr == nil) || (wantErr != nil && wantErr.Error() != gotErr.Error()) {
			t.Fatalf("%q: InDir error %v, InRoot error %v", name, gotErr, wantErr)
		}
		if want == nil {
			continue
		}
		a, errA := want.Stat()
		b, errB := got.Stat()
		must(t, errA)
		must(t, errB)
		if !os.SameFile(a, b) {
			t.Fatalf("%q: InDir opened a different file", name)
		}
		must(t, want.Close())
		must(t, got.Close())
	}
	// InRoot descends; a pinned directory never does.
	if f, err := InDir(pin, "sub/inner", os.O_RDONLY, 0); err == nil {
		f.Close()
		t.Fatal("pinned directory descended")
	}
}
