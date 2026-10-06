package contextindex

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// The trailer digest runs beside the decode on its own reread of the file.
// It must still refuse what the inline digest refused, report a decode
// refusal rather than the stopped digest, and decode the same value.
func TestDecodeSnapshotValueDigestBesideTheDecode(t *testing.T) {
	index := taskContextFixture(t)
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	identity := repositoryIdentity{objectFormat: index.ObjectFormat, treeRevision: index.Revision}
	decode := func(path string, engineID string) (*Index, error) {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		decoded := &Index{}
		return decoded, decodeSnapshotValue(file, identity, engineID, decoded)
	}
	decoded, err := decode(receipt.Path, engine())
	if err != nil {
		t.Fatal(err)
	}
	again, err := decode(receipt.Path, engine())
	if err != nil || !reflect.DeepEqual(decoded, again) {
		t.Fatalf("repeated decode differs: err=%v", err)
	}
	if _, err := decode(receipt.Path, "other-engine"); err == nil || err.Error() != "snapshot header mismatch" {
		t.Fatalf("header refusal err=%v, want snapshot header mismatch", err)
	}
	data, err := os.ReadFile(receipt.Path)
	if err != nil {
		t.Fatal(err)
	}
	forged := bytes.Replace(data, []byte("The cache demuxes keys."), []byte("The cache ignores keys."), 1)
	if bytes.Equal(forged, data) {
		t.Fatal("fixture body is missing from the snapshot")
	}
	path := t.TempDir() + "/forged.gob"
	if err := os.WriteFile(path, forged, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := decode(path, engine()); err == nil || err.Error() != "snapshot digest mismatch" {
		t.Fatalf("same-length overwrite err=%v, want snapshot digest mismatch", err)
	}
}

// BenchmarkDecodeSnapshotValue times one full snapshot decode with its
// trailer digest over 400 files of 40 documented functions each, so the
// decode is many small values as in a real repository, not a few blobs.
func BenchmarkDecodeSnapshotValue(b *testing.B) {
	root := testRepository(b)
	for file := 0; file < 400; file++ {
		var content strings.Builder
		content.WriteString("package corpus\n")
		for function := 0; function < 40; function++ {
			fmt.Fprintf(&content, "\n// Handle%d%d routes request %d through cache shard %d.\nfunc Handle%d%d(key string) string { return key + %q }\n",
				file, function, function, file, file, function, strconv.Itoa(file*function))
		}
		writeTestFile(b, root, fmt.Sprintf("internal/corpus%02d/file%03d.go", file%20, file), content.String())
	}
	testGit(b, root, "add", ".")
	testGit(b, root, "commit", "-qm", "large corpus")
	built, err := Build(context.Background(), root)
	if err != nil {
		b.Fatal(err)
	}
	receipt, err := WriteSnapshot(built)
	if err != nil {
		b.Fatal(err)
	}
	identity := repositoryIdentity{objectFormat: built.ObjectFormat, treeRevision: built.Revision}
	engineID := engine()
	b.ReportAllocs()
	b.SetBytes(receipt.Bytes)
	for b.Loop() {
		file, err := os.Open(receipt.Path)
		if err != nil {
			b.Fatal(err)
		}
		if err := decodeSnapshotValue(file, identity, engineID, &Index{}); err != nil {
			b.Fatal(err)
		}
		file.Close()
	}
}
