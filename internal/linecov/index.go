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
	"time"
)

// Format names the index encoding this package writes; a reader refuses any
// other but FormatV2 (LPCV-V0-951, MCJ-V0-003). An index of FormatV2 reads as
// one whole-test segment per test, so it selects exactly as before.
const (
	Format   = "linecov-index/3"
	FormatV2 = "linecov-index/2"
)

// Index is the immutable union of per-test records collected at one commit
// (LPCV-V0-950, LPCV-V0-956, LPCV-V0-958).
type Index struct {
	Format         string
	Module         string
	Commit         string            // the commit every record was collected at
	Runner         string            // Go version and coverage mode; a different runner decides nothing
	TestTimeout    time.Duration     // recorded test deadline; zero is legacy/unknown
	SuiteTimeout   time.Duration     // recorded in-order suite deadline
	WindowFallback map[string]string // packages retaining the in-order union and why
	Tests          []Test            // sorted by Package, Name
	Files          []File            // every instrumented file, sorted by Path
	// Unordered lists, sorted, the packages whose in-order suite run did not
	// pass, so which blocks only that order reaches is unknown: a change
	// reaching any of their tests keeps the whole package (LPCV-V0-962).
	Unordered []string
	// Segments lists every test's segments, grouped by Test in ascending
	// order and, within a test, in run order (MCJ-V0-001). Empty means each
	// test is its own single whole-test segment, as a FormatV2 index or an
	// index with no segmented record stores it.
	Segments []Segment
	// Divergent lists the equivalence classes a shadow miss marked, carried
	// into the next index until a blob in the class reach changes
	// (MCJ-V0-018). A divergent class takes part in no exclusion.
	Divergent []Divergence

	byName   map[string]int32
	byPath   map[string]int
	segs     []Segment // Segments, or one synthesized whole-test segment per test
	segFirst []int32   // per test, its first index into segs; len(Tests)+1 entries
	diverged map[string]bool
	reach    *segReach
}

// Test is one top-level test with a passing record. Package is the
// module-relative directory ("." for the module root). Elapsed is its isolated
// run time, zero in an index collected before it was recorded.
type Test struct {
	Package, Name string
	Elapsed       time.Duration
	// Level is the MCJ-V0-007 level; empty reads as LevelUnknown.
	Level string `json:",omitempty"`
	// Incomplete marks a segment record that is not whole (MCJ-V0-015): the
	// test keeps its test-level reach and joins no equivalence class.
	Incomplete bool `json:",omitempty"`
}

// File is one instrumented file at the index commit: its blob OID and every
// coverage block, executed or not.
type File struct {
	Path   string // module-relative, slash-separated
	OID    string
	Blocks []Block // sorted by Start, StartCol, End, EndCol
}

// Block is one coverage block's span and the tests that executed it. Columns
// are 1-based byte offsets; EndCol is exclusive, as in a cover profile, so two
// blocks sharing a line stay distinct and a line part outside every block is
// visible (LPCV-V0-952).
type Block struct {
	Start, StartCol, End, EndCol int32
	Tests                        []int32 // indexes into Index.Tests, ascending
	// Segs lists, ascending, the indexes into Index.Segments whose reach
	// holds the block. Nil means each test of Tests reached it through its
	// only segment (MCJ-V0-001).
	Segs []int32
}

// Record is one passing run of one top-level test: its `go tool covdata
// textfmt` profile. A failed or timed-out run yields no Record (LPCV-V0-962).
type Record struct {
	Package, Name string
	Profile       []byte
	Elapsed       time.Duration
	Level         string
	// Segments, when present, bound the run (MCJ-V0-001). They are disjoint:
	// a collector closes one before opening the next and reports a method
	// called inside a step as part of that step. Blocks Profile executed
	// outside every segment form one trailing unbound segment, so a change
	// reaching them keeps the test.
	Segments   []SegmentRecord
	Incomplete bool // MCJ-V0-015
}

// InOrder is the Name of a package's record from one run of its whole suite in
// ordinary order. Build keeps it only when that run executed a block no
// isolated record of the package did; a change reaching such a block then
// keeps the whole package, since which test took that path depends on order
// (LPCV-V0-962).
const InOrder = "(in-order)"

// profileLine is one block of a cover profile. Any file extension is read, so
// the JavaScript collector's line blocks (internal/linecov/v8) share the index
// (LPCV-V0-965).
var profileLine = regexp.MustCompile(`^(.+):(\d+)\.(\d+),(\d+)\.(\d+) \d+ (\d+)$`)

// Build unions records into an index. oids maps every module-relative path at
// commit to its blob OID; a profiled file absent from it refuses the build,
// since its blocks could not be bound to immutable content.
func Build(module, commit, runner string, oids map[string]string, records []Record) (*Index, error) {
	type key struct{ start, startCol, end, endCol int32 }
	type hit struct {
		path string
		k    key
		ran  bool
	}
	type blockKey struct {
		path string
		k    key
	}
	// parse reads a profile's blocks and, summed over its lines, the count of
	// each; counted reports a count or atomic mode, whose counts a segmented
	// record needs to see code run outside its segments.
	parse := func(r Record, profile []byte) (out []hit, counts map[blockKey]int64, counted bool, err error) {
		counts = map[blockKey]int64{}
		counted = true
		sc := bufio.NewScanner(bytes.NewReader(profile))
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			if mode, ok := strings.CutPrefix(line, "mode:"); ok {
				if strings.TrimSpace(mode) == "set" {
					counted = false
				}
				continue
			}
			if line == "" {
				continue
			}
			m := profileLine.FindStringSubmatch(line)
			if m == nil {
				return nil, nil, false, fmt.Errorf("linecov: malformed profile line %q of %s.%s", line, r.Package, r.Name)
			}
			path := strings.TrimPrefix(m[1], module+"/")
			if path == m[1] {
				continue // outside the module: not instrumented here
			}
			var span [4]int32
			for j := range span {
				n, err := strconv.ParseInt(m[2+j], 10, 32)
				if err != nil {
					return nil, nil, false, fmt.Errorf("linecov: malformed block %q", line)
				}
				span[j] = int32(n)
			}
			n, err := strconv.ParseInt(m[6], 10, 64)
			if err != nil {
				return nil, nil, false, fmt.Errorf("linecov: malformed count %q", line)
			}
			k := key{span[0], span[1], span[2], span[3]}
			out = append(out, hit{path, k, n != 0})
			counts[blockKey{path, k}] += n
		}
		return out, counts, counted, sc.Err()
	}
	hits := make([][]hit, len(records))
	// segRan[i][j] is the blocks segment j of record i executed; a segmented
	// record's blocks outside every segment become one trailing segment.
	segRan := make([][]map[blockKey]bool, len(records))
	isolated := map[string]map[hit]bool{} // package -> executed blocks
	seen := map[string]bool{}
	segmented := false
	incomplete := make([]bool, len(records))
	for i, r := range records {
		name := r.Package + "\x00" + r.Name
		if seen[name] {
			return nil, fmt.Errorf("linecov: %s.%s recorded twice", r.Package, r.Name)
		}
		seen[name] = true
		h, whole, counted, err := parse(r, r.Profile)
		if err != nil {
			return nil, err
		}
		hits[i] = h
		incomplete[i] = r.Incomplete
		if len(r.Segments) > 0 {
			segmented = true
			inSeg := map[blockKey]int64{}
			for _, sr := range r.Segments {
				sh, counts, segCounted, err := parse(r, sr.Profile)
				if err != nil {
					return nil, err
				}
				counted = counted && segCounted
				hits[i] = append(hits[i], sh...)
				ran := map[blockKey]bool{}
				for bk, n := range counts {
					if n != 0 {
						ran[bk] = true
					}
					inSeg[bk] += n
				}
				segRan[i] = append(segRan[i], ran)
			}
			// A block the whole run executed more often than its segments
			// together also ran outside them, before, between or after.
			rest := map[blockKey]bool{}
			for bk, n := range whole {
				if n > inSeg[bk] {
					rest[bk] = true
				}
			}
			if len(rest) > 0 {
				segRan[i] = append(segRan[i], rest)
			}
			// Segments are disjoint parts of the run. Segments counting a
			// block more often than the whole run overlap, so a later run
			// of that block can hide in the overlap: incomplete.
			for bk, n := range inSeg {
				if n > whole[bk] {
					incomplete[i] = true
					break
				}
			}
			// Without counts, code run both inside and outside a segment is
			// invisible, so the record is incomplete (MCJ-V0-015).
			if !counted {
				incomplete[i] = true
			}
		}
		for _, x := range hits[i] {
			if x.ran && r.Name != InOrder {
				if isolated[r.Package] == nil {
					isolated[r.Package] = map[hit]bool{}
				}
				isolated[r.Package][x] = true
			}
		}
	}
	files := map[string]map[key]map[string]bool{}
	tests := map[string]Test{}
	recordOf := map[string]int{}
	for i, r := range records {
		if r.Name == InOrder {
			sensitive := false
			for _, h := range hits[i] {
				if h.ran && !isolated[r.Package][h] {
					sensitive = true
					break
				}
			}
			if !sensitive {
				continue
			}
		}
		name := r.Package + "\x00" + r.Name
		tests[name] = Test{Package: r.Package, Name: r.Name, Elapsed: r.Elapsed, Level: r.Level, Incomplete: incomplete[i]}
		recordOf[name] = i
		for _, h := range hits[i] {
			if files[h.path] == nil {
				files[h.path] = map[key]map[string]bool{}
			}
			if files[h.path][h.k] == nil {
				files[h.path][h.k] = map[string]bool{}
			}
			if h.ran {
				files[h.path][h.k][name] = true
			}
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
	// segOf maps a block to the segments reaching it, filled only when a
	// record is segmented; an unsegmented test is its own whole segment.
	segOf := map[blockKey][]int32{}
	if segmented {
		for i, n := range names {
			r := records[recordOf[n]]
			if len(r.Segments) == 0 {
				first := int32(len(ix.Segments))
				ix.Segments = append(ix.Segments, Segment{Test: int32(i), Kind: KindTest})
				for _, x := range hits[recordOf[n]] {
					if x.ran {
						bk := blockKey{x.path, x.k}
						segOf[bk] = append(segOf[bk], first)
					}
				}
				continue
			}
			for j, ran := range segRan[recordOf[n]] {
				seg := Segment{Test: int32(i)} // the trailing residual: unbound
				if j < len(r.Segments) {
					sr := r.Segments[j]
					seg = Segment{Test: int32(i), Kind: sr.Kind, ID: sr.ID, Bound: sr.Bound, Prior: sr.Prior, Units: sr.Units, Elapsed: sr.Elapsed}
				}
				id := int32(len(ix.Segments))
				ix.Segments = append(ix.Segments, seg)
				for bk := range ran {
					segOf[bk] = append(segOf[bk], id)
				}
			}
		}
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
			b := Block{Start: k.start, StartCol: k.startCol, End: k.end, EndCol: k.endCol}
			for n := range set {
				b.Tests = append(b.Tests, ids[n])
			}
			sort.Slice(b.Tests, func(i, j int) bool { return b.Tests[i] < b.Tests[j] })
			if segmented && len(b.Tests) > 0 {
				b.Segs = uniqueSorted(segOf[blockKey{p, k}])
			}
			f.Blocks = append(f.Blocks, b)
		}
		f.sortBlocks()
		ix.Files = append(ix.Files, f)
	}
	ix.reindex()
	return ix, nil
}

// sortBlocks orders f's blocks by Start, StartCol, End, EndCol.
func (f *File) sortBlocks() {
	sort.Slice(f.Blocks, func(i, j int) bool {
		a, b := f.Blocks[i], f.Blocks[j]
		if a.Start != b.Start {
			return a.Start < b.Start
		}
		if a.StartCol != b.StartCol {
			return a.StartCol < b.StartCol
		}
		if a.End != b.End {
			return a.End < b.End
		}
		return a.EndCol < b.EndCol
	})
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
	ix.segs = ix.Segments
	if len(ix.segs) == 0 {
		ix.segs = make([]Segment, len(ix.Tests))
		for i := range ix.Tests {
			ix.segs[i] = Segment{Test: int32(i), Kind: KindTest}
		}
	}
	ix.segFirst = make([]int32, len(ix.Tests)+1)
	for i := range ix.segFirst {
		ix.segFirst[i] = int32(len(ix.segs))
	}
	for i := len(ix.segs) - 1; i >= 0; i-- {
		ix.segFirst[ix.segs[i].Test] = int32(i)
	}
	ix.reach = &segReach{}
	ix.diverged = make(map[string]bool, len(ix.Divergent))
	for _, d := range ix.Divergent {
		ix.diverged[d.Class] = true
	}
}

// uniqueSorted sorts ids ascending and drops repeats.
func uniqueSorted(ids []int32) []int32 {
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := ids[:0]
	for i, id := range ids {
		if i == 0 || id != ids[i-1] {
			out = append(out, id)
		}
	}
	return out
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
	switch ix.Format {
	case Format:
	case FormatV2:
		// A /2 index has no segments: each test reads as one whole segment,
		// so selection is unchanged (MCJ-V0-003).
		if len(ix.Segments) > 0 || len(ix.Divergent) > 0 {
			return nil, errors.New("linecov: linecov-index/2 carries segments")
		}
		ix.Format = Format
	default:
		return nil, fmt.Errorf("linecov: index format %q, want %q or %q", ix.Format, Format, FormatV2)
	}
	if ix.Module == "" || ix.Commit == "" || ix.Runner == "" {
		return nil, errors.New("linecov: index identity incomplete")
	}
	if err := ix.checkSegments(); err != nil {
		return nil, err
	}
	ix.reindex()
	return &ix, nil
}

// Merge unions indexes collected at one commit by one runner over disjoint
// package sets, as sharded collection writes them. A package absent from
// every part keeps its tests unrecorded, so Narrow runs it whole. Parts that
// disagree on identity, on a file's blob, or that both record one test
// refuse the merge.
func Merge(parts ...*Index) (*Index, error) {
	if len(parts) == 0 {
		return nil, errors.New("linecov: merge needs at least one index")
	}
	first := parts[0]
	type key struct{ start, startCol, end, endCol int32 }
	tests := map[string]Test{}
	oids := map[string]string{}
	blocks := map[string]map[key]map[string]bool{}
	unordered := map[string]bool{}
	for _, p := range parts {
		if p.Module != first.Module || p.Commit != first.Commit || p.Runner != first.Runner {
			return nil, fmt.Errorf("linecov: merge of %s %s %q with %s %s %q", first.Module, first.Commit, first.Runner, p.Module, p.Commit, p.Runner)
		}
		for _, t := range p.Tests {
			name := t.Package + "\x00" + t.Name
			if _, dup := tests[name]; dup {
				return nil, fmt.Errorf("linecov: merge: %s.%s recorded twice", t.Package, t.Name)
			}
			tests[name] = t
		}
		for _, pkg := range p.Unordered {
			unordered[pkg] = true
		}
		for _, f := range p.Files {
			if oid, ok := oids[f.Path]; ok && oid != f.OID {
				return nil, fmt.Errorf("linecov: merge: %s at blobs %s and %s", f.Path, oid, f.OID)
			}
			oids[f.Path] = f.OID
			if blocks[f.Path] == nil {
				blocks[f.Path] = map[key]map[string]bool{}
			}
			for _, b := range f.Blocks {
				k := key{b.Start, b.StartCol, b.End, b.EndCol}
				if blocks[f.Path][k] == nil {
					blocks[f.Path][k] = map[string]bool{}
				}
				for _, id := range b.Tests {
					t := p.Tests[id]
					blocks[f.Path][k][t.Package+"\x00"+t.Name] = true
				}
			}
		}
	}
	names := make([]string, 0, len(tests))
	for n := range tests {
		names = append(names, n)
	}
	sort.Strings(names)
	ix := &Index{Format: Format, Module: first.Module, Commit: first.Commit, Runner: first.Runner}
	ids := map[string]int32{}
	for i, n := range names {
		ids[n] = int32(i)
		ix.Tests = append(ix.Tests, tests[n])
	}
	for pkg := range unordered {
		ix.Unordered = append(ix.Unordered, pkg)
	}
	sort.Strings(ix.Unordered)
	paths := make([]string, 0, len(blocks))
	for p := range blocks {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		f := File{Path: p, OID: oids[p]}
		for k, set := range blocks[p] {
			b := Block{Start: k.start, StartCol: k.startCol, End: k.end, EndCol: k.endCol}
			for n := range set {
				b.Tests = append(b.Tests, ids[n])
			}
			sort.Slice(b.Tests, func(i, j int) bool { return b.Tests[i] < b.Tests[j] })
			f.Blocks = append(f.Blocks, b)
		}
		f.sortBlocks()
		ix.Files = append(ix.Files, f)
	}
	ix.reindex()
	return ix, nil
}
