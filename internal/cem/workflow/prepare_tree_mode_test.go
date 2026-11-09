package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestPrepareNamesEachNonRegularPath pins CEM-V1-655 (V1-2964): a range that
// adds a symbolic link pointing outside the tree still refuses
// unsupported-tree-mode, because cem/1.0 maps only regular files, but the
// refusal names the link path and both modes, and nothing outside the tree is
// read. A link already present at the base is unchanged and maps.
func TestPrepareNamesEachNonRegularPath(t *testing.T) {
	root, base, _ := makeCalcRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../../outside", filepath.Join(root, "web", "escape")); err != nil {
		t.Skipf("cannot create a symlink: %v", err)
	}
	writeFile(t, root, "web/page.txt", "page\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "link")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	_, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target})
	if cemcode.CodeOf(err) != cemcode.UnsupportedTreeMode {
		t.Fatalf("prepare over an added link: %v", err)
	}
	message := cemcode.MessageOf(err)
	if !strings.Contains(message, "1 changed paths are not regular files and cem/1.0 maps only regular files: web/escape (base absent, target 120000)") ||
		strings.Contains(message, "outside") || cemcode.GuidanceOf(err) == "" {
		t.Fatalf("refusal %q guidance %q", message, cemcode.GuidanceOf(err))
	}
	writeFile(t, root, "web/page.txt", "page two\n")
	gitCmd(t, root, "commit", "-qam", "page")
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: target, Target: gitCmd(t, root, "rev-parse", "HEAD")}); err != nil {
		t.Fatalf("a link unchanged since the base must map: %v", err)
	}
}
