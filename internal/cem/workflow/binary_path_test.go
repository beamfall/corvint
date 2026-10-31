package workflow

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestPrepareBinaryRefusalNamesPath reproduces V1-0301 on real Git commits:
// cem prepare over a binary-only creation, modification or deletion keeps the
// binary-patch refusal code and names the binary path (CEM-V1-037, decision
// 0548). Since V1-2752 a binary change beside a text change prepares instead
// (CEM-V1-980..983, TestCEMV1983PrepareBindsBinaryFixture).
func TestPrepareBinaryRefusalNamesPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, path string
		change     func(t *testing.T, root string)
	}{
		{"binary-creation", "assets/new.bin", func(t *testing.T, root string) {
			writeFile(t, root, "assets/new.bin", "new\x00payload\n")
		}},
		{"binary-modification", "assets/logo.bin", func(t *testing.T, root string) {
			writeFile(t, root, "assets/logo.bin", "logo\x00v2\n")
		}},
		{"binary-deletion", "assets/logo.bin", func(t *testing.T, root string) {
			gitCmd(t, root, "rm", "-q", "assets/logo.bin")
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
			writeFile(t, root, "assets/logo.bin", "logo\x00v1\n")
			gitCmd(t, root, "add", ".")
			gitCmd(t, root, "commit", "-qm", "base")
			base := gitCmd(t, root, "rev-parse", "HEAD")
			tc.change(t, root)
			gitCmd(t, root, "add", "-A")
			gitCmd(t, root, "commit", "-qm", "target")
			target := gitCmd(t, root, "rev-parse", "HEAD")

			_, err = openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target})
			if got := cemcode.CodeOf(err); got != cemcode.BinaryPatch {
				t.Fatalf("Prepare error = %v, want %s", err, cemcode.BinaryPatch)
			}
			want := fmt.Sprintf("binary change to %q", tc.path)
			if !strings.Contains(cemcode.MessageOf(err), want) {
				t.Fatalf("message = %q, want it to name %s", cemcode.MessageOf(err), want)
			}
		})
	}
}
