package gitauth

import (
	"bytes"
	"testing"
)

// TestCAXV0020SplitPatchSectionsCutsOnlyAtHeaders checks the CAX-V0-020
// section cut: a patch splits before each line that begins "diff --git ", a
// content line that merely contains that text is never a cut, and the chunks
// concatenate back to the exact input.
func TestCAXV0020SplitPatchSectionsCutsOnlyAtHeaders(t *testing.T) {
	patch := []byte("diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-diff --git a/y b/y\n+ diff --git a/z b/z\n" +
		"diff --git a/link b/link\nnew file mode 120000\n--- /dev/null\n+++ b/link\n@@ -0,0 +1 @@\n+../outside\n\\ No newline at end of file\n")
	chunks := splitPatchSections(patch)
	if len(chunks) != 2 || !bytes.HasPrefix(chunks[1], []byte("diff --git a/link b/link\n")) || !bytes.Equal(bytes.Join(chunks, nil), patch) {
		t.Fatalf("chunks %q", chunks)
	}
	if got := splitPatchSections(nil); len(got) != 0 {
		t.Fatalf("empty patch split into %q", got)
	}
}
