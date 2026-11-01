package intent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCTSV0532_SymlinkRefusalNamesTheResolvedPath (V1-3558): a symlinked
// component (macOS /tmp -> /private/tmp) is refused UNSUPPORTED_FILESYSTEM as
// a path shape, and the detail names the symlink-free path to use instead;
// that resolved path itself passes.
func TestCTSV0532_SymlinkRefusalNamesTheResolvedPath(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	err = intent.CheckNoSymlink(filepath.Join(link, "repo", ".taskman"))
	want := filepath.Join(real, "repo", ".taskman")
	if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "use "+want) {
		t.Fatalf("symlink refusal must name %s: %v", want, err)
	}
	if err := intent.CheckNoSymlink(want); err != nil {
		t.Fatalf("the named resolved path must pass: %v", err)
	}
}
