package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestPrepareRefusesMetadataOnlyChanges reproduces V1-0609 on real Git
// commits: cem prepare over an empty-file creation, an empty-file deletion, a
// mode-only change, or a text change mixed with an empty-file creation refuses
// with metadata-only-change-unsupported naming the path and change kind
// (CEM-V1-300), not the earlier misleading malformed-patch.
func TestPrepareRefusesMetadataOnlyChanges(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, kind, path string
		change           func(t *testing.T, root string)
	}{
		{"empty-file-creation", "empty-file-creation", "new.empty", func(t *testing.T, root string) {
			writeFile(t, root, "new.empty", "")
		}},
		{"empty-file-deletion", "empty-file-deletion", "gone.empty", func(t *testing.T, root string) {
			gitCmd(t, root, "rm", "-q", "gone.empty")
		}},
		{"mode-only", "mode-only", "m.sh", func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Join(root, "m.sh"), 0o755); err != nil {
				t.Fatal(err)
			}
			gitCmd(t, root, "update-index", "--chmod=+x", "m.sh")
		}},
		{"mixed-text-and-empty-creation", "empty-file-creation", "new.empty", func(t *testing.T, root string) {
			writeFile(t, root, "t.txt", "b\n")
			writeFile(t, root, "new.empty", "")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			gitCmd(t, root, "init", "-q", "-b", "main")
			writeFile(t, root, "t.txt", "a\n")
			writeFile(t, root, "gone.empty", "")
			writeFile(t, root, "m.sh", "#!/bin/sh\n")
			gitCmd(t, root, "add", ".")
			gitCmd(t, root, "commit", "-qm", "base")
			base := gitCmd(t, root, "rev-parse", "HEAD")
			tc.change(t, root)
			gitCmd(t, root, "add", "-A")
			gitCmd(t, root, "commit", "-qm", "target")
			target := gitCmd(t, root, "rev-parse", "HEAD")

			_, err = openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target})
			if got := cemcode.CodeOf(err); got != cemcode.MetadataOnlyChangeUnsupported {
				t.Fatalf("Prepare error = %v, want %s", err, cemcode.MetadataOnlyChangeUnsupported)
			}
			want := fmt.Sprintf("metadata-only change %s %q", tc.kind, tc.path)
			if !strings.Contains(cemcode.MessageOf(err), want) {
				t.Fatalf("message = %q, want it to name %s", cemcode.MessageOf(err), want)
			}
		})
	}
}
