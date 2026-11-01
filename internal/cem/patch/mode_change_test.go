package patch

import (
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestCEMV1930ModeChangeGrammar pins CEM-V1-930 (V1-3404): a modification or
// rename may carry one old mode/new mode pair of different regular modes and
// records both; every other placement or shape refuses with its named code.
func TestCEMV1930ModeChangeGrammar(t *testing.T) {
	t.Parallel()
	const hunk = "--- a/m.sh\n+++ b/m.sh\n@@ -1 +1 @@\n-a\n+b\n"
	const index = "index 1111111..2222222\n"
	accepted := []struct{ name, input, oldMode, newMode string }{
		{"modify", "diff --git a/m.sh b/m.sh\nold mode 100644\nnew mode 100755\n" + index + hunk, "100644", "100755"},
		{"modify-drop-exec", "diff --git a/m.sh b/m.sh\nold mode 100755\nnew mode 100644\n" + index + hunk, "100755", "100644"},
		{"rename", "diff --git a/m.sh b/n.sh\nold mode 100644\nnew mode 100755\nsimilarity index 80%\nrename from m.sh\nrename to n.sh\n" + index +
			"--- a/m.sh\n+++ b/n.sh\n@@ -1 +1 @@\n-a\n+b\n", "100644", "100755"},
	}
	for _, tc := range accepted {
		parsed, err := Parse([]byte(tc.input))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		group := parsed.Groups[0]
		if group.OldMode != tc.oldMode || group.NewMode != tc.newMode || len(group.Hunks) != 1 {
			t.Fatalf("%s: modes %q/%q with %d hunks", tc.name, group.OldMode, group.NewMode, len(group.Hunks))
		}
	}
	plain, err := Parse([]byte("diff --git a/m.sh b/m.sh\n" + index + hunk))
	if err != nil || plain.Groups[0].OldMode != "" || plain.Groups[0].NewMode != "" {
		t.Fatalf("a group without a mode change: %v, %+v", err, plain)
	}
	refused := []struct{ name, input, code string }{
		{"index-mode-suffix", "diff --git a/m.sh b/m.sh\nold mode 100644\nnew mode 100755\nindex 1111111..2222222 100755\n" + hunk, cemcode.DiffMetadataMismatch},
		{"create", "diff --git a/m.sh b/m.sh\nold mode 100644\nnew mode 100755\nnew file mode 100755\n--- /dev/null\n+++ b/m.sh\n@@ -0,0 +1 @@\n+b\n", cemcode.DiffMetadataMismatch},
		{"delete", "diff --git a/m.sh b/m.sh\nold mode 100644\nnew mode 100755\ndeleted file mode 100644\n--- a/m.sh\n+++ /dev/null\n@@ -1 +0,0 @@\n-a\n", cemcode.DiffMetadataMismatch},
		{"lone-old", "diff --git a/m.sh b/m.sh\nold mode 100644\n" + index + hunk, cemcode.MalformedPatch},
		{"lone-new", "diff --git a/m.sh b/m.sh\nnew mode 100755\n" + index + hunk, cemcode.MalformedPatch},
		{"repeated", "diff --git a/m.sh b/m.sh\nold mode 100644\nold mode 100644\nnew mode 100755\n" + index + hunk, cemcode.MalformedPatch},
		{"equal", "diff --git a/m.sh b/m.sh\nold mode 100644\nnew mode 100644\n" + index + hunk, cemcode.MalformedPatch},
		{"symlink", "diff --git a/m.sh b/m.sh\nold mode 100644\nnew mode 120000\n" + index + hunk, cemcode.MalformedPatch},
		{"binary", "diff --git a/m.bin b/m.bin\nold mode 100644\nnew mode 100755\nindex " + repeat('1', 40) + ".." + repeat('2', 40) + "\nBinary files a/m.bin and b/m.bin differ\n" + "diff --git a/m.sh b/m.sh\n" + index + hunk, cemcode.BinaryPatch},
	}
	for _, tc := range refused {
		_, err := Parse([]byte(tc.input))
		if got := cemcode.CodeOf(err); got != tc.code {
			t.Errorf("%s: code = %q (%v), want %q", tc.name, got, err, tc.code)
		}
	}
}

func repeat(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}
