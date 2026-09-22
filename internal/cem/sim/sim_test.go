package sim

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/patch"
)

type memSource map[string][]byte

func (m memSource) BaseBlob(path string) ([]byte, string, bool, error) {
	data, ok := m[path]
	return data, "100644", ok, nil
}

func interopBase(t *testing.T) memSource {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "interop", "cem-0.1", "repository", "base"))
	if err != nil {
		t.Fatal(err)
	}
	source := memSource{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		source[filepath.ToSlash(relative)] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func parseFixture(t *testing.T, name string) *patch.Patch {
	t.Helper()
	path := filepath.Join("..", "..", "..", "interop", "cem-0.1", "patches", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := patch.Parse(data)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return parsed
}

// TestFrozenValidPatchesSimulate proves every frozen valid patch against the
// frozen base tree bytes.
func TestFrozenValidPatchesSimulate(t *testing.T) {
	source := interopBase(t)
	for _, fixture := range []string{
		"supported.patch", "topology.patch", "whitespace.patch",
		"line-ending.patch", "bytes.patch", "invalid-line-join.patch",
	} {
		if err := Simulate(parseFixture(t, fixture), source); err != nil {
			t.Errorf("%s: %v", fixture, err)
		}
	}
}

// TestInventedOldBytesRejected proves a valid-looking patch cannot pair with a
// base whose bytes disagree: the exact failure the rejected build accepted.
func TestInventedOldBytesRejected(t *testing.T) {
	source := interopBase(t)
	source["src/app.py"] = []byte("def allowed():\n    return None\n")
	err := Simulate(parseFixture(t, "supported.patch"), source)
	if err == nil || cemcode.CodeOf(err) != cemcode.BaseMismatch {
		t.Fatalf("got %v, want base-mismatch", err)
	}
}

func mustParse(t *testing.T, text string) *patch.Patch {
	t.Helper()
	parsed, err := patch.Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestTopologyRejections(t *testing.T) {
	source := interopBase(t)
	cases := []struct {
		name  string
		input string
	}{
		{"create-existing", "diff --git a/src/app.py b/src/app.py\nnew file mode 100644\n--- /dev/null\n+++ b/src/app.py\n@@ -0,0 +1 @@\n+x\n"},
		{"delete-absent", "diff --git a/src/ghost.txt b/src/ghost.txt\ndeleted file mode 100644\n--- a/src/ghost.txt\n+++ /dev/null\n@@ -1 +0,0 @@\n-x\n"},
		{"delete-not-empty", "diff --git a/src/app.py b/src/app.py\ndeleted file mode 100644\n--- a/src/app.py\n+++ /dev/null\n@@ -1 +0,0 @@\n-def allowed():\n"},
		{"rename-absent-old", "diff --git a/src/ghost.txt b/src/other.txt\nsimilarity index 50%\nrename from src/ghost.txt\nrename to src/other.txt\n--- a/src/ghost.txt\n+++ b/src/other.txt\n@@ -1 +1 @@\n-x\n+y\n"},
		{"rename-existing-destination", "diff --git a/src/oldname.txt b/src/app.py\nsimilarity index 50%\nrename from src/oldname.txt\nrename to src/app.py\n--- a/src/oldname.txt\n+++ b/src/app.py\n@@ -1 +1 @@\n-before\n+after\n"},
		{"beyond-eof", "--- a/src/style.txt\n+++ b/src/style.txt\n@@ -9,1 +9,1 @@\n-x\n+y\n"},
		{"insertion-beyond-eof", "--- a/src/style.txt\n+++ b/src/style.txt\n@@ -2,0 +3,1 @@\n+y\n"},
		{"context-mismatch", "--- a/src/style.txt\n+++ b/src/style.txt\n@@ -1 +1 @@\n-wrong context\n+y\n"},
	}
	for _, testCase := range cases {
		err := Simulate(mustParse(t, testCase.input), source)
		if err == nil {
			t.Errorf("%s: accepted", testCase.name)
			continue
		}
		if cemcode.CodeOf(err) != cemcode.BaseMismatch {
			t.Errorf("%s: code %q (%v)", testCase.name, cemcode.CodeOf(err), err)
		}
	}
}

// TestDeleteMustDrainWholeFile proves the delete-empty rule fires on the exact
// frozen delete fixture when the base gains an extra line.
func TestDeleteMustDrainWholeFile(t *testing.T) {
	source := interopBase(t)
	source["src/doomed.txt"] = []byte("gone\nextra\n")
	err := Simulate(parseFixture(t, "topology.patch"), source)
	if err == nil || cemcode.CodeOf(err) != cemcode.BaseMismatch {
		t.Fatalf("got %v, want base-mismatch", err)
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("unexpected failure shape: %v", err)
	}
}

// TestBlobTokenizerBound proves the LF record bound applies to base blobs.
func TestBlobTokenizerBound(t *testing.T) {
	source := interopBase(t)
	oversized := make([]byte, 0, (patch.MaxLFRecords+1)*2)
	for range patch.MaxLFRecords + 1 {
		oversized = append(oversized, 'x', '\n')
	}
	source["src/style.txt"] = append(oversized, []byte("value = 1\n")...)
	err := Simulate(parseFixture(t, "whitespace.patch"), source)
	if err == nil || cemcode.CodeOf(err) != cemcode.TooManyLines {
		t.Fatalf("got %v, want too-many-lines", err)
	}
}

// TestMultiHunkCreateAndDeleteSimulate covers ALGORITHMS.md step 4 for creates
// and deletes: contiguous hunks compose to the created bytes or to empty bytes.
func TestMultiHunkCreateAndDeleteSimulate(t *testing.T) {
	source := interopBase(t)
	source["src/four.txt"] = []byte("a\nb\nc\nd\n")
	create := "diff --git a/src/new.txt b/src/new.txt\nnew file mode 100644\n--- /dev/null\n+++ b/src/new.txt\n@@ -0,0 +1,2 @@\n+a\n+b\n@@ -0,0 +3 @@\n+c\n"
	remove := "diff --git a/src/four.txt b/src/four.txt\ndeleted file mode 100644\n--- a/src/four.txt\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n@@ -3,2 +0,0 @@\n-c\n-d\n"
	for name, input := range map[string]string{"create": create, "delete": remove} {
		if err := Simulate(mustParse(t, input), source); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestDeleteModeBoundToBaseMode applies decision 0192: a delete's declared
// mode must equal the base tree entry mode it removes.
func TestDeleteModeBoundToBaseMode(t *testing.T) {
	source := interopBase(t)
	input := "diff --git a/src/doomed.txt b/src/doomed.txt\ndeleted file mode 100755\n--- a/src/doomed.txt\n+++ /dev/null\n@@ -1 +0,0 @@\n-gone\n"
	err := Simulate(mustParse(t, input), source)
	if cemcode.CodeOf(err) != cemcode.DiffMetadataMismatch {
		t.Fatalf("got %v, want diff-metadata-mismatch", err)
	}
}
