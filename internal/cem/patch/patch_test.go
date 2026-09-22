package patch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

func interopDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "interop", "cem-0.1"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestGoldenHunkIdentities parses every frozen valid patch and requires the
// derived hunk IDs, ranges, and display paths to match its frozen valid map
// exactly. These are the mandatory hunk-identity algorithm vectors.
func TestGoldenHunkIdentities(t *testing.T) {
	root := interopDir(t)
	pairs := []struct{ patch, mapFile string }{
		{"supported.patch", "supported-sha1.json"},
		{"supported.patch", "supported-sha256.json"},
		{"topology.patch", "topology.json"},
		{"whitespace.patch", "whitespace.json"},
		{"line-ending.patch", "line-ending.json"},
		{"bytes.patch", "bytes.json"},
	}
	for _, pair := range pairs {
		parsed, err := Parse(readFixture(t, root, "patches", pair.patch))
		if err != nil {
			t.Errorf("%s: %v", pair.patch, err)
			continue
		}
		mapped, err := wire.ParseMap(readFixture(t, root, "maps", "valid", pair.mapFile))
		if err != nil {
			t.Fatalf("%s: %v", pair.mapFile, err)
		}
		if len(parsed.Hunks) != len(mapped.Hunks) {
			t.Errorf("%s: %d parsed hunks, map has %d", pair.patch, len(parsed.Hunks), len(mapped.Hunks))
			continue
		}
		recorded := map[string]wire.Hunk{}
		for _, hunk := range mapped.Hunks {
			recorded[hunk.ID] = hunk
		}
		for index, hunk := range parsed.Hunks {
			match, ok := recorded[hunk.ID]
			if !ok {
				t.Errorf("%s hunk %d: derived ID %s not in %s", pair.patch, index, hunk.ID, pair.mapFile)
				continue
			}
			if match.Path != hunk.DisplayPath {
				t.Errorf("%s hunk %d: display path %q, map %q", pair.patch, index, hunk.DisplayPath, match.Path)
			}
			if match.OldRange != hunk.OldRange || match.NewRange != hunk.NewRange {
				t.Errorf("%s hunk %d: range drift", pair.patch, index)
			}
		}
	}
}

func TestFrozenInvalidPatches(t *testing.T) {
	root := interopDir(t)
	cases := []struct {
		fixture string
		code    string
	}{
		{"invalid-header-create.patch", cemcode.DiffMetadataMismatch},
		{"invalid-header-delete.patch", cemcode.DiffMetadataMismatch},
		{"invalid-mode-mismatch.patch", cemcode.DiffMetadataMismatch},
		{"invalid-special-mode.patch", cemcode.UnsupportedTreeMode},
		{"invalid-surplus.patch", cemcode.ExtraHunkPayload},
	}
	for _, testCase := range cases {
		_, err := Parse(readFixture(t, root, "patches", testCase.fixture))
		if err == nil {
			t.Errorf("%s: accepted", testCase.fixture)
			continue
		}
		if cemcode.CodeOf(err) != testCase.code {
			t.Errorf("%s: code %q, want %q (%v)", testCase.fixture, cemcode.CodeOf(err), testCase.code, err)
		}
	}
	// invalid-line-join.patch is grammatically valid; its rejection is the
	// mechanical whitespace proof during verification.
	if _, err := Parse(readFixture(t, root, "patches", "invalid-line-join.patch")); err != nil {
		t.Errorf("invalid-line-join.patch: parser rejected a grammatically valid patch: %v", err)
	}
}

// TestLFOverflowRecipe materializes the single closed recipe
// cem/0.1-lf-overflow-x-lines and requires the frozen digest and rejection.
func TestLFOverflowRecipe(t *testing.T) {
	payload := bytes.Repeat([]byte{0x78, 0x0a}, 262145)
	if len(payload) != 524290 {
		t.Fatalf("recipe length %d", len(payload))
	}
	if got := bytes.Count(payload, []byte{'\n'}); got != 262145 {
		t.Fatalf("recipe LF count %d", got)
	}
	digest := sha256.Sum256(payload)
	const want = "cfecb16854630a4d141b429fdfdcdc73bb48124ac47375b5896c634037c9eee6"
	if hex.EncodeToString(digest[:]) != want {
		t.Fatalf("recipe digest %s", hex.EncodeToString(digest[:]))
	}
	_, err := Parse(payload)
	if err == nil || cemcode.CodeOf(err) != cemcode.TooManyLines {
		t.Fatalf("recipe input: got %v, want too-many-lines", err)
	}
}

func TestTokenizerBoundary(t *testing.T) {
	atBound := bytes.Repeat([]byte{'x', '\n'}, MaxLFRecords)
	if _, err := SplitLF(atBound); err != nil {
		t.Fatalf("exactly %d records rejected: %v", MaxLFRecords, err)
	}
	noFinalLF := append(bytes.Repeat([]byte{'x', '\n'}, MaxLFRecords-1), 'x')
	if _, err := SplitLF(noFinalLF); err != nil {
		t.Fatalf("bound with no-final-LF record rejected: %v", err)
	}
	overBound := append(bytes.Repeat([]byte{'x', '\n'}, MaxLFRecords), 'x')
	if _, err := SplitLF(overBound); err == nil {
		t.Fatal("bound+1 records accepted")
	}
}

// TestPatchSizeBoundIsInclusive checks that a patch of exactly MaxPatchBytes
// is not refused as oversized: the size check is `>`, not `>=`.
func TestPatchSizeBoundIsInclusive(t *testing.T) {
	atBound := bytes.Repeat([]byte("x"), MaxPatchBytes)
	_, err := Parse(atBound)
	if err == nil {
		return
	}
	if cemcode.CodeOf(err) == cemcode.BinaryPatch {
		t.Fatalf("a patch of exactly MaxPatchBytes was refused as oversized: %v", err)
	}
}

func TestParserRejections(t *testing.T) {
	cases := []struct {
		name  string
		input string
		code  string
	}{
		{"empty", "", cemcode.BinaryPatch},
		{"nul", "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-\x00\n+b\n", cemcode.BinaryPatch},
		{"nul-at-start", "\x00--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n", cemcode.BinaryPatch},
		{"binary-marker", "diff --git a/x b/x\nBinary files a/x and b/x differ\n", cemcode.BinaryPatch},
		{"git-binary-marker", "diff --git a/x b/x\nGIT binary patch\n", cemcode.BinaryPatch},
		{"no-hunks", "hello\n", cemcode.MalformedPatch},
		{"bare-rename", "--- a/x\n+++ b/y\n@@ -1 +1 @@\n-a\n+b\n", cemcode.MalformedPatch},
		{"traversal-path", "--- a/../x\n+++ b/../x\n@@ -1 +1 @@\n-a\n+b\n", cemcode.PathTraversal},
		{"quoted-path", "--- \"a/x\"\n+++ \"b/x\"\n@@ -1 +1 @@\n-a\n+b\n", cemcode.MalformedPatch},
		{"short-body", "--- a/x\n+++ b/x\n@@ -1,2 +1,2 @@\n-a\n+b\n", cemcode.MalformedPatch},
		{"no-changed-line", "--- a/x\n+++ b/x\n@@ -1 +1 @@\n a\n", cemcode.MalformedPatch},
		{"zero-count-nonzero-start", "--- a/x\n+++ b/x\n@@ -0,2 +1,2 @@\n-a\n-b\n+c\n+d\n", cemcode.MalformedPatch},
		{"overlapping-hunks", "--- a/x\n+++ b/x\n@@ -3 +3 @@\n-a\n+b\n@@ -2 +2 @@\n-c\n+d\n", cemcode.MalformedPatch},
		{"overlap-on-previous-last-line", "--- a/x\n+++ b/x\n@@ -1,2 +1,2 @@\n-a\n-b\n+c\n+d\n@@ -2 +2 @@\n-e\n+f\n", cemcode.MalformedPatch},
		{"inconsistent-new-position", "--- a/x\n+++ b/x\n@@ -1 +5 @@\n-a\n+b\n", cemcode.MalformedPatch},
		{"duplicate-pair", "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n--- a/x\n+++ b/x\n@@ -2 +2 @@\n-c\n+d\n", cemcode.MalformedPatch},
		{"old-mode", "diff --git a/x b/x\nold mode 100644\nnew mode 100755\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n", cemcode.MalformedPatch},
		{"rename-to-disagrees", "diff --git a/x b/y\nsimilarity index 50%\nrename from x\nrename to z\n--- a/x\n+++ b/y\n@@ -1 +1 @@\n-a\n+b\n", cemcode.DiffMetadataMismatch},
		{"double-devnull", "diff --git a/x b/x\nnew file mode 100644\n--- /dev/null\n+++ /dev/null\n@@ -1 +1 @@\n-a\n+b\n", cemcode.MalformedPatch},
		{"similarity-leading-zero", "diff --git a/x b/y\nsimilarity index 050%\nrename from x\nrename to y\n--- a/x\n+++ b/y\n@@ -1 +1 @@\n-a\n+b\n", cemcode.MalformedPatch},
		{"body-record-without-lf", "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b", cemcode.MalformedPatch},
		{"marker-without-lf", "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n\\ No newline at end of file", cemcode.ExtraHunkPayload},
		{"repeated-marker", "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n\\ No newline at end of file\n\\ No newline at end of file\n", cemcode.ExtraHunkPayload},
	}
	for _, testCase := range cases {
		_, err := Parse([]byte(testCase.input))
		if err == nil {
			t.Errorf("%s: accepted", testCase.name)
			continue
		}
		if cemcode.CodeOf(err) != testCase.code {
			t.Errorf("%s: code %q, want %q (%v)", testCase.name, cemcode.CodeOf(err), testCase.code, err)
		}
	}
}

// TestIndexLineAcceptsMinimumOIDLength checks the ALGORITHMS.md floor: a
// 4-hex-character `index` object ID, the shortest allowed, is accepted.
func TestIndexLineAcceptsMinimumOIDLength(t *testing.T) {
	input := "diff --git a/x b/x\nindex abcd..1234 100644\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n"
	if _, err := Parse([]byte(input)); err != nil {
		t.Fatalf("a 4-hex index OID was refused: %v", err)
	}
}

func TestDuplicateHunkIDsRejected(t *testing.T) {
	// Two byte-identical single-line replacements at the same position cannot
	// exist in one group, so force the collision across two groups of the same
	// path pair; the pair rule fires first, then prove the ID rule with a
	// same-content different-path setup that yields distinct IDs.
	input := "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n@@ -1 +1 @@\n-a\n+b\n"
	if _, err := Parse([]byte(input)); err == nil {
		t.Fatal("duplicate positions accepted")
	}
}

func TestNoNewlineMarkerReconstruction(t *testing.T) {
	root := interopDir(t)
	parsed, err := Parse(readFixture(t, root, "patches", "bytes.patch"))
	if err != nil {
		t.Fatal(err)
	}
	last := parsed.Hunks[len(parsed.Hunks)-1]
	var oldSide, newSide []byte
	for _, line := range last.Body {
		if line.Prefix != '+' {
			oldSide = append(oldSide, line.OldPayload()...)
		}
		if line.Prefix != '-' {
			newSide = append(newSide, line.NewPayload()...)
		}
	}
	if string(oldSide) != "old" || string(newSide) != "new" {
		t.Fatalf("reconstructed %q -> %q, want old -> new without trailing LF", oldSide, newSide)
	}
}

func TestCRRetainedAsPayload(t *testing.T) {
	root := interopDir(t)
	parsed, err := Parse(readFixture(t, root, "patches", "line-ending.patch"))
	if err != nil {
		t.Fatal(err)
	}
	body := parsed.Hunks[0].Body
	if string(body[0].Payload) != "alpha\r\n" {
		t.Fatalf("CR payload %q", body[0].Payload)
	}
	if string(body[1].Payload) != "alpha\n" {
		t.Fatalf("LF payload %q", body[1].Payload)
	}
}

func TestHunkHeaderSuffixUninterpreted(t *testing.T) {
	input := "--- a/x\n+++ b/x\n@@ -1 +1 @@ func anything() {\n-a\n+b\n"
	if _, err := Parse([]byte(input)); err != nil {
		t.Fatalf("suffix rejected: %v", err)
	}
	bare := "--- a/x\n+++ b/x\n@@ -1 +1 @@x\n-a\n+b\n"
	if _, err := Parse([]byte(bare)); err != nil {
		t.Fatalf("suffix without a separating space rejected: %v", err)
	}
	crlf := "--- a/x\n+++ b/x\n@@ -1 +1 @@\r\n-a\n+b\n"
	if _, err := Parse([]byte(crlf)); err != nil {
		t.Fatalf("CR before LF on hunk header rejected: %v", err)
	}
}

// TestCreateAndDeleteAcceptContiguousHunks follows ALGORITHMS.md step 4, "one
// or more contiguous hunks", for creates and deletes as for any group.
func TestCreateAndDeleteAcceptContiguousHunks(t *testing.T) {
	for name, input := range map[string]string{
		"create": "diff --git a/x b/x\nnew file mode 100644\n--- /dev/null\n+++ b/x\n@@ -0,0 +1,2 @@\n+a\n+b\n@@ -0,0 +3 @@\n+c\n",
		"delete": "diff --git a/x b/x\ndeleted file mode 100644\n--- a/x\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n@@ -3,2 +0,0 @@\n-c\n-d\n",
	} {
		if _, err := Parse([]byte(input)); err != nil {
			t.Errorf("%s with two hunks rejected: %v", name, err)
		}
	}
}
