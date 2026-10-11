package affected_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// walkCancelContext cancels only after the real walker has visited entries.
// The stack check confines injection to the actual traversal, not fixture setup.
type walkCancelContext struct {
	context.Context
	cancel context.CancelFunc
	visits atomic.Int32
}

func (c *walkCancelContext) Err() error {
	pcs := make([]uintptr, 16)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, "affected.walkSourceFilesContext.func") {
			if c.visits.Add(1) == 10 {
				c.cancel()
			}
			break
		}
		if !more {
			break
		}
	}
	return c.Context.Err()
}
func cancellationRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for i := 0; i < 30; i++ {
		dir := filepath.Join(root, fmt.Sprintf("dir%02d", i))
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "source.txt"), []byte("source"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// AFU-V1-059: every adapter reaches a cancellable disk traversal.
func TestAFUV1059AdaptersCancel(t *testing.T) {
	root := cancellationRoot(t)
	for _, language := range languages.All() {
		t.Run(language.Name(), func(t *testing.T) {
			if _, err := language.Units(context.Background(), root); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			injected := &walkCancelContext{Context: ctx, cancel: cancel}
			_, err := language.Units(injected, root)
			if injected.visits.Load() != 10 {
				t.Fatalf("walk visits=%d", injected.visits.Load())
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

// AFU-V1-059: cancellation returns no partial graph; later builds are healthy.
func TestAFUV1059BuildCancellation(t *testing.T) {
	root := cancellationRoot(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	injected := &walkCancelContext{Context: ctx, cancel: cancel}
	graph, err := affected.Build(injected, root, languages.All()...)
	if injected.visits.Load() != 10 || graph != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("visits=%d graph=%v error=%v", injected.visits.Load(), graph, err)
	}
	if _, err := affected.Build(context.Background(), root, languages.All()...); err != nil {
		t.Fatal(err)
	}
}

// AFU-V1-059: direct adapter walks and each source operation honor cancellation.
func TestAFUV1059SourceWalkCancellation(t *testing.T) {
	root := cancellationRoot(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	source := affected.DiskSourceContext(ctx, root)
	if _, err := source.Files(func(string) bool { return true }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := source.Read("dir00/source.txt"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := source.Stat("dir00"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := source.Lstat("dir00"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

// AFU-V1-059: context-aware adapters preserve the absolute-root contract.
func TestAFUV1059CancellationPreservesRootValidation(t *testing.T) {
	for _, language := range languages.All() {
		for _, root := range []string{"", "relative"} {
			if _, err := language.Units(context.Background(), root); !errors.Is(err, affected.ErrInvalidLanguage) {
				t.Fatalf("%s root %q: %v", language.Name(), root, err)
			}
		}
	}
}

// AFU-V1-059: opted-in skipped subtrees and direct Source.Walk stop too.
func TestAFUV1059IncludedAndDirectWalkCancellation(t *testing.T) {
	root := cancellationRoot(t)
	if err := os.Rename(filepath.Join(root, "dir00"), filepath.Join(root, "build")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		if err := os.WriteFile(filepath.Join(root, "build", fmt.Sprintf("file%02d.txt", i)), []byte("source"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	injected := &walkCancelContext{Context: ctx, cancel: cancel}
	files, _, err := affected.DiskSourceContext(injected, root).FilesIncluding(func(string) bool { return true }, "build")
	if injected.visits.Load() != 10 || files != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("visits=%d files=%v error=%v", injected.visits.Load(), files, err)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	source := affected.DiskSourceContext(ctx2, root)
	visits := 0
	err = source.Walk("build", func(_ string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		visits++
		if visits == 10 {
			cancel2()
		}
		return nil
	})
	if visits != 10 || !errors.Is(err, context.Canceled) {
		t.Fatalf("direct visits=%d error=%v", visits, err)
	}
	if _, err := source.ReadBounded("build/source.txt", 100); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
