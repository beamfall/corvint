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

// This audit deliberately names every production source of contextindex and
// its extraction dependencies, across platforms; additions are discovered, not
// an enumerated filename allowlist. Review new imports too. It pins no digest:
// an input edit moves no pin unless TestAnalyzerSchemaOutput's golden output
// moves, so two branches editing one input do not conflict on a pin line
// (IDX-SNAP-V0-890). script/analyzer-schema.sh reads the directory list below
// to name the changed inputs.
func TestAnalyzerSchemaInputs(t *testing.T) {
	t.Run("IDX-SNAP-V0-017", func(t *testing.T) {
		root := filepath.Join("..", "..")
		paths := []string{"go.mod"}
		if _, err := os.Stat(filepath.Join(root, "go.sum")); err == nil {
			paths = append(paths, "go.sum")
		}
		for _, directory := range []string{"contextindex", "gitstatus", "projectprofile", "pythongrammar", "pythonsyntax", "rubyautoload", "runtimeenv", "secretscreen"} {
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
		t.Logf("analyzer inputs SHA256=%x over %d files", digest.Sum(nil), len(paths))
	})
}

// TestAnalyzerSchemaOutput pins the analyzer's output next to its schema
// number: the SHA-256 of the pack a fixed multi-language corpus compiles to,
// under a fixed engine name so the schema number itself is not part of it, and
// the SHA-256 of the blob-shard facts the same corpus yields. An input edit that
// leaves both unchanged keeps its schema number and moves no pin
// (IDX-SNAP-V0-107, IDX-SNAP-V0-890, IDX-SNAP-V0-892, V1-2289).
func TestAnalyzerSchemaOutput(t *testing.T) {
	t.Run("IDX-SNAP-V0-107", func(t *testing.T) {
		const auditedSchema = "corvint-analyzer/150"
		const auditedOutputSHA256 = "edd4d3a241552c95c49e65dff4ef2ffea3355f5b8d4ea32984c165711fe77261"
		const auditedShardSHA256 = "d7e6c60b732d28b544c25b1d3bffea24ee374bb3bd87051e432eb6f6b30f5554"
		got, shards := analyzerGoldenOutput(t, nil)
		t.Logf("analyzer output SHA256=%s", got)
		t.Logf("analyzer shard SHA256=%s", shards)
		if analyzerSchemaID != auditedSchema || got != auditedOutputSHA256 || shards != auditedShardSHA256 {
			t.Fatalf("analyzer output drift: schema=%s output SHA256=%s shard SHA256=%s; the golden corpus compiles to other facts, so the facts or their encoding changed: run script/analyzer-schema.sh bump, which moves the schema number past origin/main's and rewrites the pins (IDX-SNAP-V0-107)", analyzerSchemaID, got, shards)
		}
	})
}

// TestAnalyzerSchemaOutputCoversMarkersAndShards: the golden corpus yields a
// marker, and a change to the markers alone, or to one shard fact alone, moves
// the pinned digests (IDX-SNAP-V0-892, V1-2356).
func TestAnalyzerSchemaOutputCoversMarkersAndShards(t *testing.T) {
	t.Run("IDX-SNAP-V0-892", func(t *testing.T) {
		var markers int
		pack, shards := analyzerGoldenOutput(t, func(index *Index) {
			for _, list := range index.Markers {
				markers += len(list)
			}
		})
		if markers == 0 {
			t.Fatal("the golden corpus yields no marker")
		}
		markerPack, markerShards := analyzerGoldenOutput(t, func(index *Index) {
			for key, list := range index.Markers {
				moved := slices.Clone(list)
				moved[0].Line++
				index.Markers[key] = moved
			}
		})
		if markerPack == pack {
			t.Fatal("a marker-only change left the output digest unchanged")
		}
		if markerShards != shards {
			t.Fatal("an index-only marker change moved the shard digest")
		}
		_, sourceShards := analyzerGoldenOutput(t, func(index *Index) {
			source := index.Sources["cache/split.go"]
			source.Data = append(slices.Clone(source.Data), []byte("\nfunc Extra() {}\n")...)
			index.Sources["cache/split.go"] = source
		})
		if sourceShards == shards {
			t.Fatal("a changed shard fact left the shard digest unchanged")
		}
	})
}

// analyzerGoldenCorpus exercises every extractor family the pack carries:
// Go, Python, Ruby (with a Rake file), the web languages and a template, Java, a stylesheet and an
// AsciiDoc page as searchable text, record ledgers, decisions, specs, instructions, a marker and a
// non-text blob.
var analyzerGoldenCorpus = map[string]string{
	"go.mod":                       "module example.test/golden\n\ngo 1.27.0\n",
	"AGENTS.md":                    "# Agent contract\n\nRead the cache spec first.\n\n## Verify\n\nRun the tests.\n",
	"cache/cache.go":               "package cache\n\n// Demux splits a key. TODO: bound it.\nfunc Demux(key string) string { return Split(key) }\n\ntype Store struct{ Name string }\n\nfunc (s *Store) Get(key string) string { return s.Name + key }\n",
	"cache/split.go":               "package cache\n\nfunc Split(key string) string { return key }\n",
	"cache/cache_test.go":          "package cache\n\nimport \"testing\"\n\nfunc TestDemux(t *testing.T) { _ = Demux(\"k\") }\n",
	"server/server.go":             "package server\n\nimport \"example.test/golden/cache\"\n\nfunc Run() string { return cache.Demux(\"r\") }\n",
	"tools/report.py":              "import os\nfrom cache import client\n\n\nclass Report:\n    def render(self, rows):\n        return [os.path.join(r) for r in rows]\n\n\ndef main():\n    return Report().render([])\n",
	"tools/cache/client.py":        "def fetch(key):\n    return key\n",
	"app/models/user.rb":           "class User < ApplicationRecord\n  def name\n    Profile.new.name\n  end\nend\n",
	"app/models/profile.rb":        "class Profile\n  def name\n    \"p\"\n  end\nend\n",
	"lib/tasks/seed.rake":          "# feature:seed-demo\ndef seed_profiles\n  Profile.new\nend\n",
	"app/views/home.html.erb":      "<p><%= User.new.name %></p>\n",
	"legacy/DelimiterRules.java":   "package legacy;\n\npublic class DelimiterRules {\n  static boolean validateDelimiterRules(String input) { return input.isEmpty(); }\n}\n",
	"web/styles/site.css":          ".report { color: #333; }\n",
	"docs/guide.adoc":              "= Cache guide\n\nThe cache demuxes keys before splitting them.\n",
	"web/src/main.ts":              "import { render } from './view';\nexport function start(): string { return render('x'); }\n",
	"web/src/view.js":              "export function render(value) { return `<p>${value}</p>`; }\n",
	"testing/features.yaml":        "features:\n  - id: F-CACHE\n    title: \"Cache demux\"\n    tags: ['cache', \"split\", True, None]\n    paths: [cache/cache.go, cache/split.go]\n    owner: core\n  - id: F-REPORT\n    title: Report rendering\n    enabled: true\n",
	"testing/scenarios.yaml":       "scenarios:\n  - id: S-DEMUX\n    feature: F-CACHE\n    steps: ['demux a key', 'split it']\n",
	"docs/adr/0001-cache-split.md": "---\nstatus: accepted\n---\n# Split cache keys\n\nWe split keys before demux.\n\n## Consequences\n\nKeys stay short.\n",
	"docs/specs/cache-spec.md":     "# Cache spec\n\nThe cache demuxes keys.\n\n## Requirements\n\n- CACHE-001 Demux splits keys.\n",
	"docs/notes.md":                "# Notes\n\nPlain prose about reports.\n",
	"assets/logo.bin":              "\x00\x01\x02binary\xff",
	// DIRTY-CACHE-019: a sealed CEM is excluded from sources by the path screen.
	".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json": "{\"note\":\"cache demux evidence\"}\n",
}

// TestAnalyzerSchemaOutputDetectsAnalyzerMutation is the mutant check of the
// output pin (IDX-SNAP-V0-890, IDX-SNAP-V0-955): an analyzer that extracts one
// more fact from the same corpus encodes to another pack, so the output pin
// fails and a bump is required.
func TestAnalyzerSchemaOutputDetectsAnalyzerMutation(t *testing.T) {
	plain, _ := analyzerGoldenOutput(t, nil)
	mutant, _ := analyzerGoldenOutput(t, func(index *Index) {
		index.Symbols = append(index.Symbols, Symbol{Path: "cache/split.go", Name: "Mutant", Kind: "function", Line: 3})
	})
	if plain == mutant {
		t.Fatalf("a mutated analyzer kept the golden output digest %s; the output pin would not catch an analyzer change", plain)
	}
}

// analyzerGoldenOutput commits the corpus with fixed identities and dates,
// compiles it, applies mutate (when not nil) to the built index, and returns
// the SHA-256 of its pack and of its blob-shard facts, each shard hashed as
// "path NUL payload NUL" in path order under the fixed engine.
func analyzerGoldenOutput(t *testing.T, mutate func(*Index)) (string, string) {
	t.Helper()
	for name, value := range map[string]string{
		"GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME": "Golden", "GIT_AUTHOR_EMAIL": "golden@example.test", "GIT_AUTHOR_DATE": "2026-01-01T00:00:00Z",
		"GIT_COMMITTER_NAME": "Golden", "GIT_COMMITTER_EMAIL": "golden@example.test", "GIT_COMMITTER_DATE": "2026-01-01T00:00:00Z",
	} {
		t.Setenv(name, value)
	}
	root := t.TempDir()
	testGit(t, root, "init", "-q", "-b", "main")
	for path, content := range analyzerGoldenCorpus {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-qm", "golden corpus")
	ctx := context.Background()
	index, err := Build(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(index)
	}
	shards := sha256.New()
	tokeniser := newTokeniser()
	for _, path := range blobSourcePaths(index.Sources) {
		payload, ok, err := blobShardPayload(index.ObjectFormat, "golden", path, index.Sources[path], tokeniser)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			fmt.Fprintf(shards, "%s\x00%s\x00", path, payload)
		}
	}
	history, historyCommit := packHistoryForWrite(ctx, index)
	file, err := os.Create(filepath.Join(t.TempDir(), "golden.aip"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := encodePackSnapshot(file, index, "golden", history, historyCommit); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(content)), fmt.Sprintf("%x", shards.Sum(nil))
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
