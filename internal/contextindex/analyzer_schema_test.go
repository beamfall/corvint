package contextindex

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// This audit deliberately includes every production source of contextindex
// and its extraction dependencies, across platforms. It over-invalidates
// consumer-only changes rather than risk reusing stale facts. Additions are
// discovered, not an enumerated filename allowlist. Review new imports too.
func TestAnalyzerSchemaInputs(t *testing.T) {
	t.Run("IDX-SNAP-V0-017", func(t *testing.T) {
		const auditedSchema = "corvint-analyzer/85"
		const auditedSHA256 = "f230a79f9ab317691058c96c83ed0a2634c0b075414aab5bf9feeb34e0008a1d"
		root := filepath.Join("..", "..")
		paths := []string{"go.mod"}
		if _, err := os.Stat(filepath.Join(root, "go.sum")); err == nil {
			paths = append(paths, "go.sum")
		}
		for _, directory := range []string{"contextindex", "gitstatus", "projectprofile", "pythongrammar", "pythonsyntax", "runtimeenv", "secretscreen"} {
			matches, err := filepath.Glob(filepath.Join(root, "internal", directory, "*.go"))
			if err != nil || len(matches) == 0 {
				t.Fatalf("analyzer inputs %s: %v", directory, err)
			}
			for _, match := range matches {
				if strings.HasSuffix(match, "_test.go") {
					continue
				}
				path, err := filepath.Rel(root, match)
				if err != nil {
					t.Fatal(err)
				}
				paths = append(paths, path)
			}
		}
		slices.Sort(paths)
		digest := sha256.New()
		for _, path := range paths {
			content, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(digest, "%s\x00%s\x00", filepath.ToSlash(path), content)
		}
		got := fmt.Sprintf("%x", digest.Sum(nil))
		if analyzerSchemaID != auditedSchema || got != auditedSHA256 {
			t.Fatalf("analyzer inputs drift: schema=%s SHA256=%s; review extraction/encoding changes, bump analyzerSchemaID and update both audit pins", analyzerSchemaID, got)
		}
	})
}

func TestAnalyzerPackEngineIgnoresExecutableIdentity(t *testing.T) {
	t.Run("IDX-SNAP-V0-017", func(t *testing.T) {
		index, receipt := packFixture(t)
		if receipt.Engine != engine() {
			t.Fatal("gob receipt no longer carries the executable engine")
		}
		if !strings.Contains(receipt.PackPath, "-"+analyzerEngine()+".aip") {
			t.Fatal("pack path does not carry analyzer engine")
		}
		// The same pack must answer a reader with no executable-key gob.
		loaded, err := readSnapshotIndex(SnapshotDirectory(index.Root), fixtureIdentity(index), "rebuilt-executable", loadFull)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := packHistory(loaded); !ok {
			t.Fatal("read fell through to a gob instead of reusing the pack")
		}
		probe, hit, err := ProbeSnapshot(context.Background(), index.Root)
		if err != nil || !hit || probe.Path != receipt.PackPath || probe.Engine != analyzerEngine() {
			t.Fatalf("pack probe = %+v hit=%v err=%v", probe, hit, err)
		}
		if _, err := readPackSnapshot(receipt.PackPath, fixtureIdentity(index), "changed-analyzer", loadFull); err == nil {
			t.Fatal("changed analyzer accepted old facts")
		}
		t.Setenv(snapshotFormatEnv, "")
		if _, err := readSnapshotIndex(SnapshotDirectory(index.Root), fixtureIdentity(index), "rebuilt-executable", loadFull); err == nil {
			t.Fatal("default gob accepted a different executable")
		}
	})
}

// TestAnalyzerPackProbeMissesACorruptBody: a same-length flipped byte in a
// section past identity is refused by every loader, so the probe must miss
// and `index --if-stale` must rewrite it (IDX-SNAP-V0-011, IDX-SNAP-V0-017).
func TestAnalyzerPackProbeMissesACorruptBody(t *testing.T) {
	index, receipt := packFixture(t)
	rewritePack(t, receipt.PackPath, func(content []byte, header packHeader) []byte {
		for _, entry := range header.Sections {
			if entry.Name == packSectionBodies {
				content[entry.Offset] ^= 0xff
			}
		}
		return content
	})
	// A rebuilt executable: no gob answers, only the analyzer-keyed pack.
	if err := os.Remove(receipt.Path); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, hit, err := LoadSnapshot(ctx, index.Root); err != nil || hit {
		t.Fatalf("corrupt pack loaded: hit=%v err=%v", hit, err)
	}
	if probe, fresh, err := ProbeSnapshot(ctx, index.Root); err != nil || fresh {
		t.Fatalf("corrupt pack probed fresh: %+v err=%v", probe, err)
	}
}
