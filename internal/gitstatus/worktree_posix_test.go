//go:build darwin || linux

package gitstatus

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"testing"
)

// inputsOf lists names' ignore and attributes inputs in worktreeInputs' order.
func inputsOf(names ...string) []string {
	directories := map[string]bool{".": true}
	inputs := []string{".gitignore", ".gitattributes"}
	for _, name := range names {
		for directory := path.Dir(name); !directories[directory]; directory = path.Dir(directory) {
			directories[directory] = true
			inputs = append(inputs, directory+"/.gitignore", directory+"/.gitattributes")
		}
	}
	return inputs
}

// naiveInputModes is the oracle: one worktree Lstat per input.
func naiveInputModes(worktree *os.Root, inputs []string) map[string]os.FileMode {
	modes := map[string]os.FileMode{}
	for _, input := range inputs {
		if info, err := worktree.Lstat(input); err == nil {
			modes[input] = info.Mode()
		}
	}
	return modes
}

// The per-directory traversal reports exactly the modes one worktree Lstat
// per input reports, through in-parent, escaping and out-of-root symlinks,
// missing directories, files in place of directories and FIFOs at any depth,
// and refuses the first blocking input in worktreeInputs' order.
func TestWorktreeInputModesMatchRootLstat(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	for _, directory := range []string{"a/b/c/d/e", "x/y", "a-b", "a.b", "z"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeTest(t, filepath.Join(root, "a", "file"), "not a directory\n")
	writeTest(t, filepath.Join(outside, ".gitignore"), "*.log\n")
	writeTest(t, filepath.Join(root, "a", "b", ".gitignore"), "*.tmp\n")
	// Computed, so no literal here climbs out of this package (AFP-V0-012).
	climb, err := filepath.Rel(filepath.Join(root, "a", "b", "c"), filepath.Join(root, "x", "y"))
	if err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		"a/inlink":       "b",                            // stays in its parent
		"a/b/up":         "../../x",                      // leaves its parent, stays in the root
		"a/b/c/climb":    climb,                          // leaves three parents
		"a/out":          outside,                        // leaves the root
		"a/b/dangling":   "missing",                      // resolves nowhere
		"x/y/.gitignore": filepath.Join(outside, "fifo"), // symlinked input
		"a/b/c/loop":     "loop",                         // never resolves
	} {
		if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(link))); err != nil {
			t.Fatal(err)
		}
	}
	for _, fifo := range []string{"a/b/c/d/.gitattributes", "x/.gitignore", "a-b/.gitignore", "z/.gitignore"} {
		if err := syscall.Mkfifo(filepath.Join(root, filepath.FromSlash(fifo)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := syscall.Mkfifo(filepath.Join(outside, "fifo"), 0o600); err != nil {
		t.Fatal(err)
	}
	names := []string{
		"z/first.go", "a/b/c/d/e/deep.go", "a/inlink/c/d/via.go", "a/b/up/y/up.go", "a/b/c/climb/climb.go",
		"a/out/out.go", "a/file/under.go", "a/b/dangling/d.go", "a/b/c/loop/l.go", "gone/missing/m.go",
		"../escape.go", "/abs/path.go", "a-b/dash.go", "a.b/dot.go", "x/y/z.go", "a/b/../b/c/dotted.go",
	}
	worktree, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer worktree.Close()
	for _, order := range [][]string{names, reversed(names)} {
		inputs := inputsOf(order...)
		got, err := worktreeInputModes(context.Background(), worktree, inputs)
		if err != nil {
			t.Fatal(err)
		}
		want := naiveInputModes(worktree, inputs)
		if !maps.Equal(got, want) {
			t.Fatalf("modes differ\n got %v\nwant %v", got, want)
		}
		if want["a/inlink/c/d/.gitattributes"]&os.ModeNamedPipe == 0 || want["a/b/up/y/.gitignore"]&os.ModeSymlink == 0 {
			t.Fatalf("fixture lost its symlinked FIFO cases: %v", want)
		}
	}
	for _, order := range [][]string{names, reversed(names)} {
		index := testIndexOf(t, order)
		got := worktreeInputsOpen(context.Background(), root, index)
		want := naiveInputsOpen(t, root, index)
		if RefusalMessage(got) != RefusalMessage(want) || RefusalClass(got) != RefusalClass(want) || (got == nil) != (want == nil) {
			t.Fatalf("refusal %v, want %v", got, want)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := worktreeInputModes(ctx, worktree, inputsOf(names...)); err != context.Canceled {
		t.Fatalf("cancelled traversal err=%v", err)
	}
}

// naiveInputsOpen is the V1-0388 refusal before the per-directory traversal.
func naiveInputsOpen(t *testing.T, root string, index []byte) error {
	worktree, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer worktree.Close()
	for _, name := range worktreeInputs(index) {
		info, err := worktree.Lstat(name)
		if err == nil && info.Mode()&blocking != 0 {
			return unsupported(classMetadataUnreadable, "worktree file "+path.Base(name)+" "+irregular(info.Mode()))
		}
	}
	return nil
}

// testIndexOf frames names as a v2 SHA-1 index with zeroed stat data.
func testIndexOf(t *testing.T, names []string) []byte {
	t.Helper()
	index := []byte("DIRC\x00\x00\x00\x02")
	index = append(index, byte(len(names)>>24), byte(len(names)>>16), byte(len(names)>>8), byte(len(names)))
	for _, name := range names {
		entry := make([]byte, 60, 64+len(name))
		entry = append(entry, byte(len(name)>>8), byte(len(name)))
		entry = append(entry, name...)
		for padding := 8 - len(entry)%8; padding > 0; padding-- {
			entry = append(entry, 0)
		}
		index = append(index, entry...)
	}
	if got := worktreeInputs(index); len(got) != len(inputsOf(names...)) {
		t.Fatalf("test index frames %d inputs, want %d", len(got), len(inputsOf(names...)))
	}
	return index
}

func reversed(names []string) []string {
	out := make([]string, len(names))
	for offset, name := range names {
		out[len(names)-1-offset] = name
	}
	return out
}

// BenchmarkWorktreeInputsOpen times the V1-0388 refusal over a tree of 512
// directories four levels deep, the shape a status read pays per call.
func BenchmarkWorktreeInputsOpen(b *testing.B) {
	root := b.TempDir()
	var names []string
	for first := range 8 {
		for second := range 8 {
			for third := range 8 {
				directory := filepath.Join(root, fmt.Sprintf("p%d", first), fmt.Sprintf("q%d", second), fmt.Sprintf("r%d", third))
				if err := os.MkdirAll(directory, 0o700); err != nil {
					b.Fatal(err)
				}
				names = append(names, fmt.Sprintf("p%d/q%d/r%d/file.go", first, second, third))
			}
		}
	}
	index := []byte("DIRC\x00\x00\x00\x02")
	index = append(index, byte(len(names)>>24), byte(len(names)>>16), byte(len(names)>>8), byte(len(names)))
	for _, name := range names {
		entry := make([]byte, 60, 64+len(name))
		entry = append(entry, byte(len(name)>>8), byte(len(name)))
		entry = append(entry, name...)
		for padding := 8 - len(entry)%8; padding > 0; padding-- {
			entry = append(entry, 0)
		}
		index = append(index, entry...)
	}
	b.ResetTimer()
	for b.Loop() {
		if err := worktreeInputsOpen(context.Background(), root, index); err != nil {
			b.Fatal(err)
		}
	}
}
