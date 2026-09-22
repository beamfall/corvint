package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// corpusDigest returns a content digest of a corpus, canonical over every
// record's fields -- not over c.Paths order or the generation seed, since
// gob preserves generation order while pack sorts paths and SQLite's table
// scans make no read-order guarantee. Every per-path slice (Symbols,
// Imports, Markers) is sorted before hashing for the same reason.
func corpusDigest(c *Corpus) []byte {
	paths := make([]string, 0, len(c.Files))
	for p := range c.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	h := sha256.New()
	for _, p := range paths {
		rec := c.Files[p]
		fmt.Fprintf(h, "PATH:%s\nBLOB:%s\n", rec.Path, rec.BlobHash)

		syms := append([]Symbol(nil), rec.Symbols...)
		sort.Slice(syms, func(i, j int) bool {
			if syms[i].Name != syms[j].Name {
				return syms[i].Name < syms[j].Name
			}
			return syms[i].Line < syms[j].Line
		})
		for _, s := range syms {
			fmt.Fprintf(h, "SYM:%s:%d\n", s.Name, s.Line)
		}

		imps := append([]string(nil), rec.Imports...)
		sort.Strings(imps)
		for _, imp := range imps {
			fmt.Fprintf(h, "IMP:%s\n", imp)
		}

		mks := append([]Marker(nil), rec.Markers...)
		sort.Slice(mks, func(i, j int) bool {
			if mks[i].Line != mks[j].Line {
				return mks[i].Line < mks[j].Line
			}
			return mks[i].Text < mks[j].Text
		})
		for _, m := range mks {
			fmt.Fprintf(h, "MK:%d:%s\n", m.Line, m.Text)
		}
	}

	vocab := append([]string(nil), c.Vocabulary...)
	sort.Strings(vocab)
	for _, term := range vocab {
		fmt.Fprintf(h, "VOCAB:%s\n", term)
	}
	return h.Sum(nil)
}

// TestAllEncodingsRoundTripTheSameCorpus writes one corpus through all four
// encodings, reads the full corpus back from each, and asserts their
// content digests agree. This is a correctness check independent of the
// benchmarks: a bug that silently dropped or reordered records in one
// encoding's whole-read path would previously go unnoticed.
func TestAllEncodingsRoundTripTheSameCorpus(t *testing.T) {
	dir := t.TempDir()
	corpus := GenerateCorpus(500, 123)
	want := corpusDigest(corpus)

	gobPath := filepath.Join(dir, "snapshot.gob")
	if err := WriteGobSnapshot(gobPath, corpus); err != nil {
		t.Fatalf("write gob snapshot: %v", err)
	}
	packDir := filepath.Join(dir, "pack")
	if err := BuildPack(packDir, corpus); err != nil {
		t.Fatalf("build pack: %v", err)
	}
	sqliteDir := filepath.Join(dir, "sqlite")
	if err := os.MkdirAll(sqliteDir, 0o755); err != nil {
		t.Fatalf("mkdir sqlite dir: %v", err)
	}
	if err := BuildSQLite(sqliteDir, corpus); err != nil {
		t.Fatalf("build sqlite: %v", err)
	}

	gobCorpus, _, err := LoadGobWhole(gobPath)
	if err != nil {
		t.Fatalf("load gob: %v", err)
	}
	if got := corpusDigest(gobCorpus); !bytes.Equal(got, want) {
		t.Fatal("gob whole-read corpus digest does not match the source corpus")
	}

	packReader, err := OpenPack(packDir)
	if err != nil {
		t.Fatalf("open pack: %v", err)
	}
	defer packReader.Close()
	packCorpus, err := packReader.ReadWhole()
	if err != nil {
		t.Fatalf("pack read whole: %v", err)
	}
	if got := corpusDigest(packCorpus); !bytes.Equal(got, want) {
		t.Fatal("pack whole-read corpus digest does not match the source corpus")
	}

	packV2Dir := filepath.Join(dir, "packv2")
	if err := BuildPackV2(packV2Dir, corpus); err != nil {
		t.Fatalf("build pack v2: %v", err)
	}
	packV2Reader, err := OpenPackV2(packV2Dir)
	if err != nil {
		t.Fatalf("open pack v2: %v", err)
	}
	defer packV2Reader.Close()
	packV2Corpus, err := packV2Reader.ReadWhole()
	if err != nil {
		t.Fatalf("pack v2 read whole: %v", err)
	}
	if got := corpusDigest(packV2Corpus); !bytes.Equal(got, want) {
		t.Fatal("pack v2 whole-read corpus digest does not match the source corpus")
	}

	db, err := OpenSQLiteReadOnly(sqliteDir)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	sqliteCorpus, err := SQLiteReadWhole(db)
	if err != nil {
		t.Fatalf("sqlite read whole: %v", err)
	}
	if got := corpusDigest(sqliteCorpus); !bytes.Equal(got, want) {
		t.Fatal("sqlite whole-read corpus digest does not match the source corpus")
	}
}

// TestPackV2OnePathLookupFindsEveryPathAndRefusesAbsentOnes builds a corpus
// large enough for a three-level tree, then asserts tree descent returns
// every stored record exactly and reports absent keys (before the first,
// between two, after the last, longer than the key width) as not found.
func TestPackV2OnePathLookupFindsEveryPathAndRefusesAbsentOnes(t *testing.T) {
	dir := t.TempDir()
	corpus := GenerateCorpus(3000, 21)
	if err := BuildPackV2(dir, corpus); err != nil {
		t.Fatalf("build pack v2: %v", err)
	}
	r, err := OpenPackV2(dir)
	if err != nil {
		t.Fatalf("open pack v2: %v", err)
	}
	defer r.Close()
	if r.manifest.Height != 3 {
		t.Fatalf("height = %d, want 3 so interior descent is exercised", r.manifest.Height)
	}
	for _, p := range corpus.Paths {
		got, err := r.ReadOnePath(p)
		if err != nil {
			t.Fatalf("read one path %q: %v", p, err)
		}
		one := &Corpus{Files: map[string]Record{p: got}}
		want := &Corpus{Files: map[string]Record{p: corpus.Files[p]}}
		if !bytes.Equal(corpusDigest(one), corpusDigest(want)) {
			t.Fatalf("record for %q differs from the source corpus", p)
		}
	}
	for _, absent := range []string{"", "a", "pkg000/file000000.go0", "zzz", "pkg000/file000000.go-longer-than-any-stored-key"} {
		if _, err := r.ReadOnePath(absent); !errors.Is(err, errPathNotFound) {
			t.Fatalf("read absent path %q: want errPathNotFound, got %v", absent, err)
		}
	}
}
