// Package main is an isolated reader-format experiment for AT-04: it never
// imports internal/contextindex (a separate module cannot import it, and
// this package does not attempt to). It generates a synthetic corpus shaped
// like internal/contextindex.Index -- paths, blob ids, symbols, imports,
// document markers, a vocabulary table -- and benchmarks three ways to
// encode and re-read it. See README.md for the commands and results, and
// for what this experiment does NOT establish.
package main

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
)

const (
	symbolsPerFile = 8
	importsPerFile = 5
	markersPerFile = 3
	vocabSize      = 50_000
)

// Symbol is a synthetic stand-in for internal/contextindex.Symbol.
type Symbol struct {
	Name string
	Line int
}

// Marker is a synthetic stand-in for internal/contextindex.Marker.
type Marker struct {
	Line int
	Text string
}

// Record is one file's worth of extracted facts: the per-path unit every
// encoding stores and every "one-path" read must isolate.
type Record struct {
	Path     string
	BlobHash string
	Symbols  []Symbol
	Imports  []string
	Markers  []Marker
}

// Corpus is the synthetic whole-snapshot payload, shaped after the map-of-
// path-to-record and shared vocabulary table in internal/contextindex.Index.
type Corpus struct {
	Files      map[string]Record
	Paths      []string // deterministic generation order
	Vocabulary []string
}

// GenerateCorpus builds a deterministic synthetic corpus of fileCount files.
// It does not model ranking, symbol resolution, or any other Corvint
// behavior -- it exists only to give the three encodings a corpus of
// realistic shape and scale.
func GenerateCorpus(fileCount int, seed int64) *Corpus {
	rng := rand.New(rand.NewSource(seed))
	c := &Corpus{
		Files: make(map[string]Record, fileCount),
		Paths: make([]string, 0, fileCount),
	}

	pkgNames := make([]string, 200)
	for i := range pkgNames {
		pkgNames[i] = fmt.Sprintf("pkg%03d", i)
	}

	for i := 0; i < fileCount; i++ {
		path := fmt.Sprintf("%s/file%06d.go", pkgNames[i%len(pkgNames)], i)
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", path, seed)))
		blobHash := fmt.Sprintf("%x", sum)

		symbols := make([]Symbol, symbolsPerFile)
		for s := range symbols {
			symbols[s] = Symbol{Name: fmt.Sprintf("Symbol%d_%d", i, s), Line: rng.Intn(2000) + 1}
		}

		imports := make([]string, importsPerFile)
		for m := range imports {
			imports[m] = pkgNames[rng.Intn(len(pkgNames))]
		}

		markers := make([]Marker, markersPerFile)
		for d := range markers {
			markers[d] = Marker{Line: rng.Intn(2000) + 1, Text: fmt.Sprintf("marker-%d-%d", i, d)}
		}

		c.Files[path] = Record{Path: path, BlobHash: blobHash, Symbols: symbols, Imports: imports, Markers: markers}
		c.Paths = append(c.Paths, path)
	}

	c.Vocabulary = make([]string, vocabSize)
	for i := range c.Vocabulary {
		c.Vocabulary[i] = fmt.Sprintf("term%06d", i)
	}
	return c
}
