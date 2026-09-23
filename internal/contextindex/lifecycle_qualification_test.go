package contextindex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// canonicalSnapshotBytes is the canonical JSON of the index a snapshot hit
// serves for root, with the three worktree fields cleared as the writer
// clears them. JSON orders map keys, so equal indexes give equal bytes; the
// gob file itself does not, because gob encodes maps in iteration order
// (IDX-SNAP-V0-022).
func canonicalSnapshotBytes(t *testing.T, root string) []byte {
	t.Helper()
	loaded, hit, err := LoadSnapshot(context.Background(), root)
	if err != nil || !hit {
		t.Fatalf("snapshot load at %s: hit=%v err=%v", root, hit, err)
	}
	portable := *loaded
	portable.Root, portable.DirtyPaths, portable.StatusSHA256 = "", nil, ""
	encoded, err := json.Marshal(&portable)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func writeTreeSnapshot(t *testing.T, root string) (*Index, SnapshotReceipt) {
	t.Helper()
	built, err := BuildForSnapshot(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := WriteSnapshot(built)
	if err != nil {
		t.Fatal(err)
	}
	return built, receipt
}

func cloneAtRevision(t *testing.T, source, revision string) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "clone", "-q", "--no-checkout", source, ".")
	testGit(t, root, "checkout", "-q", "--detach", revision)
	return root
}

// qualifyColdAndIncremental indexes target in a fresh clone (cold) and in a
// clone that first indexed prior and then moved to target (incremental), and
// requires the two served indexes to be byte-identical.
func qualifyColdAndIncremental(t *testing.T, source, prior, target string) {
	t.Helper()
	cold := cloneAtRevision(t, source, target)
	writeTreeSnapshot(t, cold)
	incremental := cloneAtRevision(t, source, prior)
	writeTreeSnapshot(t, incremental)
	testGit(t, incremental, "checkout", "-q", "--detach", target)
	if _, fresh, err := ProbeSnapshot(context.Background(), incremental); err != nil || fresh {
		t.Fatalf("IDX-SNAP-V0-011: moved-ahead repository probed fresh=%v err=%v", fresh, err)
	}
	writeTreeSnapshot(t, incremental)
	coldBytes := canonicalSnapshotBytes(t, cold)
	incrementalBytes := canonicalSnapshotBytes(t, incremental)
	if !bytes.Equal(coldBytes, incrementalBytes) {
		t.Fatalf("IDX-SNAP-V0-022: cold %x and incremental %x indexes of %s differ", sha256.Sum256(coldBytes), sha256.Sum256(incrementalBytes), target)
	}
	t.Logf("IDX-SNAP-V0-022: %s after %s: canonical index %d bytes sha256 %x", target, prior, len(coldBytes), sha256.Sum256(coldBytes))
}

// qualifyDefaultPath clears the opt-in writer settings so an exported shard or
// snapshot-format setting cannot move these tests off the default gob path.
func qualifyDefaultPath(t *testing.T) {
	t.Setenv("CORVINT_INDEX_SHARDS", "")
	t.Setenv("CORVINT_SNAPSHOT_FORMAT", "")
}

func TestColdAndIncrementalSnapshotsAreByteIdentical(t *testing.T) {
	qualifyDefaultPath(t)
	t.Run("IDX-SNAP-V0-022 fixture", func(t *testing.T) {
		source := impactRepositoryWithFiles(t, map[string]string{
			"go.mod":             "module example.test/lifecycle\n\ngo 1.27.0\n",
			"app/app.go":         "package app\n\nfunc Run() string { return \"first\" }\n",
			"app/app_test.go":    "package app\n\nfunc TestRun() { _ = Run() }\n",
			"old/removed.go":     "package old\n\nfunc Removed() {}\n",
			"docs/guide.md":      "# Guide\n\nRun `app/app.go`.\n",
			"lib/renamed.py":     "def renamed():\n    return 1\n",
			"testing/scenes.txt": "scenario one\n",
		})
		prior := testGit(t, source, "rev-parse", "HEAD")
		writeTestFile(t, source, "app/app.go", "package app\n\nfunc Run() string { return \"second\" }\n")
		writeTestFile(t, source, "app/added.go", "package app\n\nfunc Added() {}\n")
		testGit(t, source, "rm", "-q", "old/removed.go")
		testGit(t, source, "mv", "lib/renamed.py", "lib/moved.py")
		testGit(t, source, "add", ".")
		testGit(t, source, "commit", "-qm", "move ahead")
		qualifyColdAndIncremental(t, source, prior, testGit(t, source, "rev-parse", "HEAD"))
	})
	t.Run("IDX-SNAP-V0-022 corpus", func(t *testing.T) {
		source := os.Getenv("CORVINT_LIFECYCLE_CORPUS")
		if source == "" {
			t.Skip("CORVINT_LIFECYCLE_CORPUS names no qualified corpus repository")
		}
		qualifyColdAndIncremental(t, source, testGit(t, source, "rev-parse", "HEAD~1"), testGit(t, source, "rev-parse", "HEAD"))
	})
}

func snapshotDirectoryListing(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(SnapshotDirectory(root))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func requireSnapshotMiss(t *testing.T, root, state string) {
	t.Helper()
	if index, hit, err := LoadSnapshot(context.Background(), root); err != nil || hit || index != nil {
		t.Fatalf("IDX-SNAP-V0-023 %s: load hit=%v err=%v", state, hit, err)
	}
	if _, fresh, err := ProbeSnapshot(context.Background(), root); err != nil || fresh {
		t.Fatalf("IDX-SNAP-V0-023 %s: probe fresh=%v err=%v", state, fresh, err)
	}
	if _, hit, err := LoadEventSnapshot(context.Background(), root, true); err != nil || hit {
		t.Fatalf("IDX-SNAP-V0-023 %s: compact event load hit=%v err=%v", state, hit, err)
	}
}

// TestSnapshotLifecycleHostileStatesHaveBoundedOutcomes pins the lifecycle
// matrix of IDX-SNAP-V0-023: unsupported input, corruption, staleness, dirty
// state and rollback each end in one asserted outcome, never a panic.
func TestSnapshotLifecycleHostileStatesHaveBoundedOutcomes(t *testing.T) {
	qualifyDefaultPath(t)
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":          "module example.test/hostile\n\ngo 1.27.0\n",
		"app/app.go":      "package app\n\nfunc Run() string { return \"first\" }\n",
		"docs/guide.md":   "# Guide\n\nRun the app.\n",
		"assets/logo.png": "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR",
		"data/blob.txt":   "a\x00b",
		"large/large.go":  "package large\n\nvar Payload = \"" + strings.Repeat("x", maxSourceBytes) + "\"\n",
	})
	prior := testGit(t, root, "rev-parse", "HEAD")
	built, receipt := writeTreeSnapshot(t, root)
	priorBytes := canonicalSnapshotBytes(t, root)

	t.Run("unsupported input", func(t *testing.T) {
		if want := []Exclusion{{"large/large.go", "source exceeds size bound"}}; !reflect.DeepEqual(built.Exclusions, want) {
			t.Fatalf("IDX-SNAP-V0-023: exclusions = %v, want %v", built.Exclusions, want)
		}
		if built.UnsupportedSuffixCount != 1 {
			t.Fatalf("IDX-SNAP-V0-023: unsupported suffix count = %d, want 1 (assets/logo.png)", built.UnsupportedSuffixCount)
		}
		for _, path := range []string{"assets/logo.png", "large/large.go"} {
			if _, ok := built.Sources[path]; ok {
				t.Fatalf("IDX-SNAP-V0-023: %s entered Sources", path)
			}
			if _, ok := built.Tracked[path]; !ok {
				t.Fatalf("IDX-SNAP-V0-023: %s missing from Tracked", path)
			}
		}
		if _, valid, loaded := built.Sources["data/blob.txt"].Text(); !loaded || valid {
			t.Fatalf("IDX-SNAP-V0-023: binary admitted source loaded=%v valid=%v, want loaded and not text", loaded, valid)
		}
		loaded, hit, err := LoadSnapshot(context.Background(), root)
		if err != nil || !hit || !reflect.DeepEqual(loaded, built) {
			t.Fatalf("IDX-SNAP-V0-023: unsupported inputs did not round-trip: hit=%v err=%v", hit, err)
		}
	})

	t.Run("corruption and file rollback", func(t *testing.T) {
		original, err := os.ReadFile(receipt.Path)
		if err != nil {
			t.Fatal(err)
		}
		garbled := bytes.Clone(original)
		copy(garbled, bytes.Repeat([]byte{0xff}, 32))
		for _, mutation := range []struct {
			name  string
			bytes []byte
		}{
			{"empty file", nil},
			{"torn header", original[:16]},
			{"torn body", original[:len(original)/2]},
			{"one byte short", original[:len(original)-1]},
			{"garbled header", garbled},
		} {
			if err := os.WriteFile(receipt.Path, mutation.bytes, 0o644); err != nil {
				t.Fatal(err)
			}
			requireSnapshotMiss(t, root, mutation.name)
			if err := os.WriteFile(receipt.Path, original, 0o644); err != nil {
				t.Fatal(err)
			}
			if got := canonicalSnapshotBytes(t, root); !bytes.Equal(got, priorBytes) {
				t.Fatalf("IDX-SNAP-V0-023 %s: restored snapshot serves a different index", mutation.name)
			}
		}
	})

	t.Run("stale", func(t *testing.T) {
		writeTestFile(t, root, "app/app.go", "package app\n\nfunc Run() string { return \"second\" }\n")
		testGit(t, root, "commit", "-qam", "move ahead")
		requireSnapshotMiss(t, root, "stale")
		writeTreeSnapshot(t, root)
	})

	t.Run("dirty", func(t *testing.T) {
		before := snapshotDirectoryListing(t, root)
		committed, hit, err := LoadSnapshot(context.Background(), root)
		if err != nil || !hit {
			t.Fatalf("clean load: hit=%v err=%v", hit, err)
		}
		writeTestFile(t, root, "app/app.go", "package app\n\nfunc Run() string { return \"dirty\" }\n")
		dirty, hit, err := LoadSnapshot(context.Background(), root)
		if err != nil || !hit {
			t.Fatalf("IDX-SNAP-V0-023 dirty: hit=%v err=%v", hit, err)
		}
		if !reflect.DeepEqual(dirty.DirtyPaths, []string{"app/app.go"}) || !bytes.Equal(dirty.Sources["app/app.go"].Data, committed.Sources["app/app.go"].Data) {
			t.Fatalf("IDX-SNAP-V0-023 dirty: paths %v body %q", dirty.DirtyPaths, dirty.Sources["app/app.go"].Data)
		}
		if after := snapshotDirectoryListing(t, root); !reflect.DeepEqual(after, before) {
			t.Fatalf("IDX-SNAP-V0-023 dirty: read changed the snapshot directory %v -> %v", before, after)
		}
		testGit(t, root, "checkout", "--", "app/app.go")
	})

	t.Run("rollback", func(t *testing.T) {
		testGit(t, root, "reset", "-q", "--hard", prior)
		if got := canonicalSnapshotBytes(t, root); !bytes.Equal(got, priorBytes) {
			t.Fatal("IDX-SNAP-V0-023 rollback: the retained prior snapshot serves a different index")
		}
		if _, fresh, err := ProbeSnapshot(context.Background(), root); err != nil || !fresh {
			t.Fatalf("IDX-SNAP-V0-023 rollback: probe fresh=%v err=%v", fresh, err)
		}
	})
}
