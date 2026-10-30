package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// PackBlock locates one independently checksummed, independently decodable
// region of data.bin: DNIP-IDX-005 requires every packed block to carry its
// own checksum and every query to verify the exact touched byte range
// before decoding it.
type PackBlock struct {
	Offset int64
	Length int64
	SHA256 [32]byte
}

// PackManifest is the manifest written last, after every data block, so a
// reader always sees either the previous complete manifest or the new
// complete one (DNIP-IDX-008). Paths is stored once and shared by every
// kind's parallel block list, instead of repeating each path string once
// per kind. DataFile names the section file this manifest's offsets are
// relative to: BuildPack gives every publish its own unique data file (via
// os.CreateTemp) instead of overwriting a fixed name, so a republish can
// never truncate the sections a still-published old manifest points at.
type PackManifest struct {
	Paths      []string
	Kinds      map[string][]PackBlock // kind -> blocks parallel to Paths
	Vocabulary PackBlock
	DataFile   string
}

const (
	kindFilesMeta = "files_meta"
	kindSymbols   = "symbols"
	kindImports   = "imports"
	kindMarkers   = "markers"
)

var kindOrder = []string{kindFilesMeta, kindSymbols, kindImports, kindMarkers}

func dataPath(dir, name string) string { return filepath.Join(dir, name) }
func manifestPath(dir string) string   { return filepath.Join(dir, "manifest.gob") }

func gobEncode(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BuildPack writes the sectioned pack for c into dir: a freshly named data
// file (via os.CreateTemp, never a reused fixed name) holds every per-path,
// per-section-name (per-kind) block plus the vocabulary block, each
// independently gob-encoded and checksummed (DNIP-IDX-004/005); the
// manifest is written to a temp file and renamed into place last, the
// crash-atomic publication step DNIP-IDX-008 requires. Because the data
// file name is unique per publish, a republish never truncates or
// overwrites the section file an already-published manifest still points
// at -- a failed republish (new sections written, new manifest never
// renamed) leaves the previous snapshot fully readable.
func BuildPack(dir string, c *Corpus) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "data-*.bin")
	if err != nil {
		return err
	}
	defer f.Close()
	dataFileName := filepath.Base(f.Name())

	sortedPaths := append([]string(nil), c.Paths...)
	sort.Strings(sortedPaths)

	manifest := PackManifest{Paths: sortedPaths, Kinds: make(map[string][]PackBlock, len(kindOrder)), DataFile: dataFileName}
	var offset int64

	writeBlock := func(payload []byte) (PackBlock, error) {
		n, err := f.Write(payload)
		if err != nil {
			return PackBlock{}, err
		}
		blk := PackBlock{Offset: offset, Length: int64(n), SHA256: sha256.Sum256(payload)}
		offset += int64(n)
		return blk, nil
	}

	for _, kind := range kindOrder {
		blocks := make([]PackBlock, 0, len(sortedPaths))
		for _, p := range sortedPaths {
			rec := c.Files[p]
			var payload []byte
			var err error
			switch kind {
			case kindFilesMeta:
				payload, err = gobEncode(rec.BlobHash)
			case kindSymbols:
				payload, err = gobEncode(rec.Symbols)
			case kindImports:
				payload, err = gobEncode(rec.Imports)
			case kindMarkers:
				payload, err = gobEncode(rec.Markers)
			}
			if err != nil {
				return err
			}
			blk, err := writeBlock(payload)
			if err != nil {
				return err
			}
			blocks = append(blocks, blk)
		}
		manifest.Kinds[kind] = blocks
	}

	vocabPayload, err := gobEncode(c.Vocabulary)
	if err != nil {
		return err
	}
	if manifest.Vocabulary, err = writeBlock(vocabPayload); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	manifestBytes, err := gobEncode(manifest)
	if err != nil {
		return err
	}
	tmp := manifestPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, manifestBytes, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, manifestPath(dir))
}

// CountingReaderAt wraps an io.ReaderAt and accumulates the bytes returned
// by ReadAt, so a benchmark can report bytes actually read from the file
// instead of bytes present on disk.
type CountingReaderAt struct {
	R     io.ReaderAt
	bytes int64
}

func (c *CountingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, err := c.R.ReadAt(p, off)
	c.bytes += int64(n)
	return n, err
}

// BytesRead returns the running total of bytes returned by ReadAt.
func (c *CountingReaderAt) BytesRead() int64 { return c.bytes }

// errNoSnapshot is returned when the manifest file is absent: DNIP-IDX-008
// means a missing manifest is "no snapshot", never a partial read of
// whatever data blocks happen to be on disk underneath it.
var errNoSnapshot = errors.New("snapshot-reader: manifest missing, no snapshot")

// errChecksumMismatch is returned by readBlock when a touched byte range's
// SHA-256 does not match the manifest -- a distinct, matchable sentinel so
// a test can assert the checksum path specifically fired, not merely that
// some error (e.g. a gob decode failure) occurred.
var errChecksumMismatch = errors.New("snapshot-reader: checksum mismatch")

// PackReader opens a pack for reading. The manifest is loaded once; every
// data read after that goes through a bounded ReaderAt (DNIP-IDX-006) with
// the exact touched range's checksum verified before decoding
// (DNIP-IDX-005), and no read decodes more sections than the caller asked
// for (DNIP-IDX-007).
type PackReader struct {
	manifest          PackManifest
	pathIdx           map[string]int
	ra                *CountingReaderAt
	file              *os.File
	manifestBytesRead int64 // bytes read for manifest.gob at Open, folded into BytesRead
}

// readFullAt reads exactly len(buf) bytes at offset 0 through ra, treating
// an io.EOF that still filled buf as success (some io.ReaderAt
// implementations, including *os.File, may return it on the final read).
func readFullAt(ra *CountingReaderAt, buf []byte) error {
	n, err := ra.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return err
	}
	if n != len(buf) {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// OpenPack opens the pack rooted at dir. It returns errNoSnapshot if the
// manifest file has not been published yet. The manifest read itself goes
// through a CountingReaderAt (folded into BytesRead) so bytes-read totals
// are comparable across operations instead of silently excluding the
// manifest.
func OpenPack(dir string) (*PackReader, error) {
	mf, err := os.Open(manifestPath(dir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errNoSnapshot
		}
		return nil, err
	}
	info, err := mf.Stat()
	if err != nil {
		mf.Close()
		return nil, err
	}
	manifestRA := &CountingReaderAt{R: mf}
	manifestBytes := make([]byte, info.Size())
	readErr := readFullAt(manifestRA, manifestBytes)
	mf.Close()
	if readErr != nil {
		return nil, readErr
	}
	var manifest PackManifest
	if err := gob.NewDecoder(bytes.NewReader(manifestBytes)).Decode(&manifest); err != nil {
		return nil, err
	}
	f, err := os.Open(dataPath(dir, manifest.DataFile))
	if err != nil {
		return nil, err
	}
	pathIdx := make(map[string]int, len(manifest.Paths))
	for i, p := range manifest.Paths {
		pathIdx[p] = i
	}
	return &PackReader{
		manifest:          manifest,
		pathIdx:           pathIdx,
		ra:                &CountingReaderAt{R: f},
		file:              f,
		manifestBytesRead: manifestRA.BytesRead(),
	}, nil
}

// Close releases the underlying file handle.
func (r *PackReader) Close() error { return r.file.Close() }

// BytesRead returns the running total of file bytes read since Open,
// including the one-time manifest read -- comparable to gob's
// whole-file bytes/op instead of excluding manifest overhead.
func (r *PackReader) BytesRead() int64 { return r.manifestBytesRead + r.ra.BytesRead() }

func (r *PackReader) readBlock(blk PackBlock, out interface{}) error {
	buf := make([]byte, blk.Length)
	if _, err := r.ra.ReadAt(buf, blk.Offset); err != nil {
		return err
	}
	if sha256.Sum256(buf) != blk.SHA256 {
		return fmt.Errorf("%w: offset %d length %d", errChecksumMismatch, blk.Offset, blk.Length)
	}
	return gob.NewDecoder(bytes.NewReader(buf)).Decode(out)
}

// ReadWhole decodes every block in the pack: the "whole" read.
func (r *PackReader) ReadWhole() (*Corpus, error) {
	c := &Corpus{Files: make(map[string]Record, len(r.manifest.Paths))}
	filesMeta := r.manifest.Kinds[kindFilesMeta]
	symbols := r.manifest.Kinds[kindSymbols]
	imports := r.manifest.Kinds[kindImports]
	markers := r.manifest.Kinds[kindMarkers]
	for i, path := range r.manifest.Paths {
		var blobHash string
		if err := r.readBlock(filesMeta[i], &blobHash); err != nil {
			return nil, err
		}
		var syms []Symbol
		if err := r.readBlock(symbols[i], &syms); err != nil {
			return nil, err
		}
		var imps []string
		if err := r.readBlock(imports[i], &imps); err != nil {
			return nil, err
		}
		var mks []Marker
		if err := r.readBlock(markers[i], &mks); err != nil {
			return nil, err
		}
		c.Files[path] = Record{Path: path, BlobHash: blobHash, Symbols: syms, Imports: imps, Markers: mks}
		c.Paths = append(c.Paths, path)
	}
	var vocab []string
	if err := r.readBlock(r.manifest.Vocabulary, &vocab); err != nil {
		return nil, err
	}
	c.Vocabulary = vocab
	return c, nil
}

// ReadOneTable decodes only the imports section, across every path
// (DNIP-IDX-004's tree-scoped packed section), without touching files_meta,
// symbols, markers, or the vocabulary block.
func (r *PackReader) ReadOneTable() (map[string][]string, error) {
	blocks := r.manifest.Kinds[kindImports]
	out := make(map[string][]string, len(blocks))
	for i, path := range r.manifest.Paths {
		var imps []string
		if err := r.readBlock(blocks[i], &imps); err != nil {
			return nil, err
		}
		out[path] = imps
	}
	return out, nil
}

// ReadOnePath decodes only the blocks for one path, across every kind --
// DNIP-IDX-007's bound against deserializing the whole corpus for a single
// lookup.
func (r *PackReader) ReadOnePath(target string) (Record, error) {
	i, ok := r.pathIdx[target]
	if !ok {
		return Record{}, fmt.Errorf("snapshot-reader: path %q not found", target)
	}
	var blobHash string
	if err := r.readBlock(r.manifest.Kinds[kindFilesMeta][i], &blobHash); err != nil {
		return Record{}, err
	}
	var syms []Symbol
	if err := r.readBlock(r.manifest.Kinds[kindSymbols][i], &syms); err != nil {
		return Record{}, err
	}
	var imps []string
	if err := r.readBlock(r.manifest.Kinds[kindImports][i], &imps); err != nil {
		return Record{}, err
	}
	var mks []Marker
	if err := r.readBlock(r.manifest.Kinds[kindMarkers][i], &mks); err != nil {
		return Record{}, err
	}
	return Record{Path: target, BlobHash: blobHash, Symbols: syms, Imports: imps, Markers: mks}, nil
}
