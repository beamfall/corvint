// Package linecov is the Go line-level coverage selection of
// docs/specs/live-proof-carrying-verification-v0.md (LPCV-V0-950..963): a
// per-test record of the executed coverage blocks, kept as one immutable index,
// and the analysis that maps a change to the tests it can affect.
package linecov

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Format names the index encoding; a reader refuses any other (LPCV-V0-951).
const Format = "linecov-index/1"

// Index is the immutable union of per-test records collected at one commit
// (LPCV-V0-950, LPCV-V0-956, LPCV-V0-958).
type Index struct {
	Format string
	Module string
	Commit string // the commit every record was collected at
	Runner string // Go version and coverage mode; a different runner decides nothing
	Tests  []Test // sorted by Package, Name
	Files  []File // every instrumented file, sorted by Path

	byName map[string]int32
	byPath map[string]int
}

// Test is one top-level test with a passing record. Package is the
// module-relative directory ("." for the module root).
type Test struct {
	Package, Name string
}

// File is one instrumented file at the index commit: its blob OID and every
// coverage block, executed or not.
type File struct {
	Path   string // module-relative, slash-separated
	OID    string
	Blocks []Block // sorted by Start, End
}

// Block is one coverage block's line span and the tests that executed it.
type Block struct {
	Start, End int32
	Tests      []int32 // indexes into Index.Tests, ascending
}

// Record is one passing run of one top-level test: its `go tool covdata
// textfmt` profile. A failed or timed-out run yields no Record (LPCV-V0-962).
type Record struct {
	Package, Name string
	Profile       []byte
}

var profileLine = regexp.MustCompile(`^(.+\.go):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$`)

// Build unions records into an index. oids maps every module-relative path at
// commit to its blob OID; a profiled file absent from it refuses the build,
// since its blocks could not be bound to immutable content.
func Build(module, commit, runner string, oids map[string]string, records []Record) (*Index, error) {
	type key struct{ start, end int32 }
	files := map[string]map[key]map[string]bool{}
	tests := map[string]Test{}
	for _, r := range records {
		name := r.Package + "\x00" + r.Name
		tests[name] = Test{r.Package, r.Name}
		sc := bufio.NewScanner(bytes.NewReader(r.Profile))
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "mode:") || line == "" {
				continue
			}
			m := profileLine.FindStringSubmatch(line)
			if m == nil {
				return nil, fmt.Errorf("linecov: malformed profile line %q of %s.%s", line, r.Package, r.Name)
			}
			path := strings.TrimPrefix(m[1], module+"/")
			if path == m[1] {
				continue // outside the module: not instrumented here
			}
			s, err1 := strconv.Atoi(m[2])
			e, err2 := strconv.Atoi(m[3])
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("linecov: malformed block %q", line)
			}
			k := key{int32(s), int32(e)}
			if files[path] == nil {
				files[path] = map[key]map[string]bool{}
			}
			if files[path][k] == nil {
				files[path][k] = map[string]bool{}
			}
			if m[4] != "0" {
				files[path][k][name] = true
			}
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}
	names := make([]string, 0, len(tests))
	for n := range tests {
		names = append(names, n)
	}
	sort.Strings(names)
	ix := &Index{Format: Format, Module: module, Commit: commit, Runner: runner}
	ids := map[string]int32{}
	for i, n := range names {
		ids[n] = int32(i)
		ix.Tests = append(ix.Tests, tests[n])
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		oid := oids[p]
		if oid == "" {
			return nil, fmt.Errorf("linecov: profiled file %s has no blob at %s", p, commit)
		}
		f := File{Path: p, OID: oid}
		for k, set := range files[p] {
			b := Block{Start: k.start, End: k.end}
			for n := range set {
				b.Tests = append(b.Tests, ids[n])
			}
			sort.Slice(b.Tests, func(i, j int) bool { return b.Tests[i] < b.Tests[j] })
			f.Blocks = append(f.Blocks, b)
		}
		sort.Slice(f.Blocks, func(i, j int) bool {
			a, b := f.Blocks[i], f.Blocks[j]
			return a.Start < b.Start || a.Start == b.Start && a.End < b.End
		})
		ix.Files = append(ix.Files, f)
	}
	ix.reindex()
	return ix, nil
}

func (ix *Index) reindex() {
	ix.byName = make(map[string]int32, len(ix.Tests))
	for i, t := range ix.Tests {
		ix.byName[t.Package+"\x00"+t.Name] = int32(i)
	}
	ix.byPath = make(map[string]int, len(ix.Files))
	for i, f := range ix.Files {
		ix.byPath[f.Path] = i
	}
}

// Encode writes the index.
func (ix *Index) Encode(w io.Writer) error {
	return gob.NewEncoder(w).Encode(ix)
}

// Decode reads an index and refuses another format or an inconsistent body,
// so a damaged index decides nothing.
func Decode(r io.Reader) (*Index, error) {
	var ix Index
	if err := gob.NewDecoder(bufio.NewReader(r)).Decode(&ix); err != nil {
		return nil, fmt.Errorf("linecov: decode: %w", err)
	}
	if ix.Format != Format {
		return nil, fmt.Errorf("linecov: index format %q, want %q", ix.Format, Format)
	}
	if ix.Module == "" || ix.Commit == "" || ix.Runner == "" {
		return nil, errors.New("linecov: index identity incomplete")
	}
	for _, f := range ix.Files {
		for _, b := range f.Blocks {
			for _, t := range b.Tests {
				if t < 0 || int(t) >= len(ix.Tests) {
					return nil, errors.New("linecov: block names an absent test")
				}
			}
		}
	}
	ix.reindex()
	return &ix, nil
}
