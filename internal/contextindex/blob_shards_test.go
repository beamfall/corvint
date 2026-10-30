package contextindex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBlobShardsReuseCommittedFactsAcrossChanges(t *testing.T) {
	t.Run("IDX-SNAP-V0-016", func(t *testing.T) {
		t.Setenv("CORVINT_INDEX_SHARDS", "1")
		same := "package shared\nfunc Same() {}\n"
		root := impactRepositoryWithFiles(t, map[string]string{
			"go.mod":        "module example.test/shards\n\ngo 1.27.0\n",
			"src/shared.go": same, "copy/shared.go": same, "copy/shared.txt": same,
			"src/shared_test.go": "package shared\nfunc TestSame() { Same() }\n",
			"docs/readme.md":     "# Guide\nSee `new.go` and `src/shared.go`.\n",
			"delete.go":          "package old\nfunc Old() {}\n",
			"invalid.py":         "def broken(:\n",
			"invalid.go":         "package broken\nfunc bad(\n",
			"binary.txt":         "a\x00b",
		})
		before, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := WriteSnapshot(before); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(root, "delete.go")); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, root, "new.go", "package shared\nfunc New() {}\n")
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "change manifest")
		writeTestFile(t, root, "src/shared.go", "dirty contents must not enter evidence")
		shardsBefore := shardFiles(t, root)
		hitsBefore := blobBatchEntries.Load()
		loaded, err := buildWithBlobShards(context.Background(), root, nil, (*Index).compile)
		if err != nil {
			t.Fatal(err)
		}
		if got := blobBatchEntries.Load() - hitsBefore; got != 1 {
			t.Fatalf("fetched %d blobs, want only new.go", got)
		}
		if len(loaded.blobFacts) != len(before.Sources)-1 {
			t.Fatalf("reused %d sources, before %d", len(loaded.blobFacts), len(before.Sources))
		}
		loaded.blobFacts = nil
		full, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(loaded, full) {
			t.Fatalf("IDX-SNAP-V0-016: full and shard indexes differ")
		}
		if !reflect.DeepEqual(shardsBefore, shardFiles(t, root)) {
			t.Fatal("read changed shard bytes")
		}
		for _, subject := range []string{"", "src/shared.go", "invalid.py", "invalid.go"} {
			got, err := buildWithBlobShards(context.Background(), root, nil, contextCompile(subject))
			if err != nil {
				t.Fatal(err)
			}
			got.blobFacts = nil
			want, err := BuildContext(context.Background(), root, subject)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("context subject %q tables differ", subject)
			}
		}

	})
}

func TestBlobShardCorruptionRefusesAccelerationAndReadFallsBack(t *testing.T) {
	t.Setenv("CORVINT_INDEX_SHARDS", "1")
	built := taskContextFixture(t)
	if _, err := WriteSnapshot(built); err != nil {
		t.Fatal(err)
	}
	source := built.Sources["cache/demux.go"]
	target := blobShardPath(SnapshotDirectory(built.Root), built.ObjectFormat, analyzerEngine(), source.BlobHash, source.Path)
	original, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	corruptBody := func() []byte {
		var fact blobFacts
		if err := json.Unmarshal(original[sha256.Size:], &fact); err != nil {
			t.Fatal(err)
		}
		fact.Source.Data[0] ^= 1
		payload, err := json.Marshal(fact)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(payload)
		return append(digest[:], payload...)
	}
	flipped := append([]byte(nil), original...)
	flipped[len(flipped)-1] ^= 1
	for label, data := range map[string][]byte{"digest": flipped, "truncated": original[:12], "body-oid": corruptBody()} {
		t.Run(label, func(t *testing.T) {
			if err := os.WriteFile(target, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := buildWithBlobShards(context.Background(), built.Root, nil, contextCompile("")); err == nil {
				t.Fatal("corruption accepted")
			}
			got, err := BuildContextObserved(context.Background(), built.Root, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			want, err := BuildContext(context.Background(), built.Root, "")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatal("fallback differs")
			}
			after, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, after) {
				t.Fatal("read repaired corrupted shard")
			}
		})
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(built.Root, "go.mod"), target); err != nil {
		t.Fatal(err)
	}
	if _, err := buildWithBlobShards(context.Background(), built.Root, nil, contextCompile("")); err == nil {
		t.Fatal("symlink accepted")
	}
}

func shardFiles(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	result := make(map[string][32]byte)
	err := filepath.WalkDir(SnapshotDirectory(root), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[path] = sha256.Sum256(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestBlobShardAggregateReadBudgetRefusesAcceleration(t *testing.T) {
	t.Setenv("CORVINT_INDEX_SHARDS", "1")
	built := taskContextFixture(t)
	if _, err := WriteSnapshot(built); err != nil {
		t.Fatal(err)
	}
	source := built.Sources["cache/demux.go"]
	entry := treeEntry{source.Path, source.BlobHash, source.Mode, len(source.Data), false}
	var used blobReadBudget
	used.bytes.Store(maxBlobShardReadBytes)
	if _, err := readBlobFact(snapshotBase(built.Root), SnapshotDirectory(built.Root), built.ObjectFormat, analyzerEngine(), entry, &used); err == nil {
		t.Fatal("aggregate byte bound ignored")
	}
}

func TestBlobShardRefusesForgedUnboundedFacts(t *testing.T) {
	t.Setenv("CORVINT_INDEX_SHARDS", "1")
	built := taskContextFixture(t)
	if _, err := WriteSnapshot(built); err != nil {
		t.Fatal(err)
	}
	source := built.Sources["cache/demux.go"]
	entry := treeEntry{source.Path, source.BlobHash, source.Mode, len(source.Data), false}
	target := blobShardPath(SnapshotDirectory(built.Root), built.ObjectFormat, analyzerEngine(), source.BlobHash, source.Path)
	for label, payload := range map[string][]byte{
		"depth":  []byte(strings.Repeat("[", 33) + strings.Repeat("]", 33)),
		"tokens": []byte("[" + strings.Repeat("0,", maxBlobShardTokens) + "0]"),
	} {
		t.Run(label, func(t *testing.T) {
			digest := sha256.Sum256(payload)
			if err := os.WriteFile(target, append(digest[:], payload...), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := readBlobFact(snapshotBase(built.Root), SnapshotDirectory(built.Root), built.ObjectFormat, analyzerEngine(), entry, nil); err == nil {
				t.Fatal("forged facts accepted")
			}
		})
	}
	file, err := os.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxBlobShardBytes + 1); err != nil {
		t.Fatal(err)
	}
	file.Close()
	if _, err := readBlobFact(snapshotBase(built.Root), SnapshotDirectory(built.Root), built.ObjectFormat, analyzerEngine(), entry, nil); err == nil {
		t.Fatal("oversized shard accepted")
	}
}

// snapshotBase is the directory that anchors the store's no-follow walks.
func snapshotBase(root string) string {
	return locateSnapshotStore(root).base
}
