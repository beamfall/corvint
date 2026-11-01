package doccorpus

import (
	"fmt"
	"strings"
	"testing"
)

// TestDCSV1976OneArtifactEncodePerCompile: a compile encodes its artifact
// once, the digest's, and applies the output bound to the digested length
// plus the digest, which is exactly the final encoding's length.
func TestDCSV1976OneArtifactEncodePerCompile(t *testing.T) {
	// Not parallel: BuildsForTest and ArtifactEncodesForTest count the whole package.
	root, m := ownedSourceCorpus(t, map[string]string{"src/app.py": "def run():\n    return 1\n", "src/readme.md": "# App\n\nRuns.\n"}, []string{"src/app.py"}, false)
	builds, encodes := BuildsForTest(), ArtifactEncodesForTest()
	a := buildOwned(t, root, m)
	if compiles, got := BuildsForTest()-builds, ArtifactEncodesForTest()-encodes; compiles != 1 || got != 1 {
		t.Fatalf("%d compiles encoded the artifact %d times, want 1 and 1", compiles, got)
	}
	unsigned := *a
	unsigned.SHA256 = ""
	_, size, err := hashSized(&unsigned)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(data)) != size+int64(len(a.SHA256)) {
		t.Fatalf("final encoding is %d bytes, want the digested %d plus the %d-byte digest", len(data), size, len(a.SHA256))
	}

	var big strings.Builder
	for i := range MaxRecords + 8 {
		fmt.Fprintf(&big, "def f%05d():\n    return %d\n", i, i)
	}
	root, m = ownedSourceCorpus(t, map[string]string{"src/big.py": big.String(), "src/readme.md": "# Big\n\nMany functions.\n"}, []string{"src/big.py"}, false)
	builds, encodes = BuildsForTest(), ArtifactEncodesForTest()
	buildOwned(t, root, m)
	if compiles, got := BuildsForTest()-builds, ArtifactEncodesForTest()-encodes; compiles != 2 || got != 2 {
		t.Fatalf("over-bound build: %d compiles encoded the artifact %d times, want 2 and 2", compiles, got)
	}
}
