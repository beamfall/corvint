package main

import (
	"bytes"
	"encoding/gob"
	"os"
)

// WriteGobSnapshot writes the whole corpus as a single gob value, matching
// internal/contextindex/snapshot.go's LoadSnapshot style: one encoding/gob
// value holding the entire index.
func WriteGobSnapshot(path string, c *Corpus) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewEncoder(f).Encode(c)
}

// LoadGobWhole reads and decodes the entire gob snapshot, returning the
// number of file bytes read (always the whole file: gob has no random
// access). The read goes through the same CountingReaderAt pack.go uses,
// so gob's and pack's bytes-read totals are measured the same way instead
// of gob reporting os.ReadFile's length while pack reports a narrower
// per-block count.
func LoadGobWhole(path string) (*Corpus, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	ra := &CountingReaderAt{R: f}
	data := make([]byte, info.Size())
	if err := readFullAt(ra, data); err != nil {
		return nil, 0, err
	}
	var c Corpus
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&c); err != nil {
		return nil, 0, err
	}
	return &c, ra.BytesRead(), nil
}

// LoadGobOneTable decodes the whole snapshot -- gob offers no way to reach
// only the imports table -- and then projects it down. This is the behavior
// docs/agent-memory/optimizations.md's 2026-09-04 entry describes: gob must
// read and skip the complete value even when the caller wants one table.
func LoadGobOneTable(path string) (map[string][]string, int64, error) {
	c, n, err := LoadGobWhole(path)
	if err != nil {
		return nil, 0, err
	}
	out := make(map[string][]string, len(c.Files))
	for p, rec := range c.Files {
		out[p] = rec.Imports
	}
	return out, n, nil
}

// LoadGobOnePath decodes the whole snapshot and returns one path's record.
func LoadGobOnePath(path, target string) (Record, int64, error) {
	c, n, err := LoadGobWhole(path)
	if err != nil {
		return Record{}, 0, err
	}
	return c.Files[target], n, nil
}
