// SPDX-License-Identifier: Apache-2.0
package testevidence

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sort"
)

// Union retains every observed edge on identical blobs (RSP-V0-010,
// RSP-V0-055). Blob drift refuses instead of replacing an edge without a
// verified newer collection. The caller must recollect the changed blobs.
func Union(a, b Record) (Record, error) {
	if err := a.Validate(); err != nil {
		return Record{}, err
	}
	if err := b.Validate(); err != nil {
		return Record{}, err
	}
	ka, _ := a.TargetKey()
	kb, _ := b.TargetKey()
	if ka != kb || a.Runner != b.Runner || a.Execution != b.Execution {
		return Record{}, errors.New("union-identity-drift")
	}
	blobs := map[PathKey]string{}
	for _, r := range []Record{a, b} {
		add := func(blob Blob) bool {
			for p, oid := range map[string]string{blob.Path.Resolved: blob.OID, blob.Path.Lexical: lexicalOID(blob)} {
				k := PathKey{blob.Path.Repository, p}
				if old, ok := blobs[k]; ok && old != oid {
					return false
				}
				blobs[k] = oid
			}
			return true
		}
		var bs []Blob
		for _, f := range r.Executed.Items {
			bs = append(bs, f.Blob)
		}
		for _, f := range r.Regions.Items {
			bs = append(bs, f.Blob)
		}
		bs = append(bs, r.Loaded.Items...)
		bs = append(bs, r.Reads.Items...)
		for _, e := range r.Execs.Items {
			bs = append(bs, e.Blob)
		}
		for _, blob := range bs {
			if !add(blob) {
				return Record{}, errors.New("union-blob-drift")
			}
		}
	}
	sources := map[string]Producer{}
	for _, p := range a.Sources {
		sources[p.Run] = p
	}
	for _, p := range b.Sources {
		if old, ok := sources[p.Run]; ok && old != p {
			return Record{}, errors.New("union-collection-conflict")
		}
		sources[p.Run] = p
	}
	// Semantic set comparison, not artifact ordering, determines disagreement.
	disagrees := !reflect.DeepEqual(canonicalExecuted(a.Executed), canonicalExecuted(b.Executed)) || !reflect.DeepEqual(canonical(a.Reads), canonical(b.Reads)) || !reflect.DeepEqual(canonical(a.Listings), canonical(b.Listings))
	// Replaying the same collection with conflicting evidence is malformed.
	if len(sources) == len(a.Sources) && len(sources) == len(b.Sources) && disagrees {
		return Record{}, errors.New("union-collection-conflict")
	}
	out := a
	out.Sources = nil
	for _, p := range sources {
		out.Sources = append(out.Sources, p)
	}
	sort.Slice(out.Sources, func(i, j int) bool { return out.Sources[i].Run < out.Sources[j].Run })
	out.CollectionCount = uint64(len(out.Sources))
	out.Nondeterministic = a.Nondeterministic || b.Nondeterministic || disagrees
	out.Executed = canonicalExecuted(join(a.Executed, b.Executed))
	out.Regions = join(a.Regions, b.Regions)
	out.Loaded = join(a.Loaded, b.Loaded)
	// RSP-V0-050: agreeing file-only collections retain their explicit capability.
	// Unknown or mixed capabilities still flow through the incomplete join.
	if a.SelectionCapability() == FileSelection && b.SelectionCapability() == FileSelection {
		out.Executed, out.Regions, out.Loaded = a.Executed, a.Regions, a.Loaded
	}
	out.Reads = join(a.Reads, b.Reads)
	out.Listings = join(a.Listings, b.Listings)
	out.Lookups = join(a.Lookups, b.Lookups)
	out.Writes = join(a.Writes, b.Writes)
	out.Execs = join(a.Execs, b.Execs)
	out.External = join(a.External, b.External)
	// These audit counts are upper bounds over collections. Saturating keeps an
	// adversarial count from wrapping into a misleading small number.
	sum := func(x, y uint64) uint64 {
		if math.MaxUint64-x < y {
			return math.MaxUint64
		}
		return x + y
	}
	out.Outside = OutsideCounts{sum(a.Outside.Toolchain, b.Outside.Toolchain), sum(a.Outside.Cache, b.Outside.Cache), sum(a.Outside.Temporary, b.Outside.Temporary), sum(a.Outside.Other, b.Outside.Other)}
	if err := out.Validate(); err != nil {
		return Record{}, err
	}
	return out, nil
}
func lexicalOID(b Blob) string {
	if b.Path.Lexical == b.Path.Resolved {
		return b.OID
	}
	return b.LexicalOID
}

func canonical[T any](c Component[T]) Component[T] {
	set := map[string]T{}
	for _, item := range c.Items {
		b, _ := json.Marshal(item)
		set[string(b)] = item
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	c.Items = nil
	for _, k := range keys {
		c.Items = append(c.Items, set[k])
	}
	return c
}
func join[T any](a, b Component[T]) Component[T] {
	out := Component[T]{State: Present, Items: append(append([]T(nil), a.Items...), b.Items...)}
	if a.State != Present || b.State != Present {
		out.State = Incomplete
		out.Reason = "collection-component-incomplete"
	}
	return canonical(out)
}
func canonicalExecuted(c Component[ExecutedFile]) Component[ExecutedFile] {
	files := map[Blob]ExecutedFile{}
	for _, f := range c.Items {
		old := files[f.Blob]
		old.Blob = f.Blob
		old.Executed = append(old.Executed, f.Executed...)
		old.Unexecuted = append(old.Unexecuted, f.Unexecuted...)
		files[f.Blob] = old
	}
	c.Items = nil
	for _, f := range files {
		f.Executed = rangeUnion(f.Executed)
		f.Unexecuted = subtract(rangeUnion(f.Unexecuted), f.Executed)
		c.Items = append(c.Items, f)
	}
	return canonical(c)
}

type position struct{ line, col int32 }

func before(a, b position) bool { return a.line < b.line || a.line == b.line && a.col < b.col }
func start(r Range) position    { return position{r.Start, r.StartColumn} }
func end(r Range) position      { return position{r.End, r.EndColumn} }
func span(a, b position) Range  { return Range{a.line, a.col, b.line, b.col} }
func rangeUnion(rs []Range) []Range {
	rs = append([]Range(nil), rs...)
	sort.Slice(rs, func(i, j int) bool { return before(start(rs[i]), start(rs[j])) })
	var out []Range
	for _, r := range rs {
		if len(out) == 0 || before(end(out[len(out)-1]), start(r)) {
			out = append(out, r)
			continue
		}
		prev := &out[len(out)-1]
		if before(end(*prev), end(r)) {
			prev.End, prev.EndColumn = r.End, r.EndColumn
		}
	}
	return out
}
func subtract(unexecuted, executed []Range) []Range {
	var out []Range
	for _, u := range unexecuted {
		lo, hi := start(u), end(u)
		for _, e := range executed {
			if !before(lo, hi) {
				break
			}
			if !before(lo, end(e)) || !before(start(e), hi) {
				continue
			}
			if before(lo, start(e)) {
				out = append(out, span(lo, start(e)))
			}
			if before(lo, end(e)) {
				lo = end(e)
			}
		}
		if before(lo, hi) {
			out = append(out, span(lo, hi))
		}
	}
	return out
}

// CollectionReady reports only the collection-quality prerequisite, never an
// exclusion. The boolean must come from admitted runner qualification.
func (r Record) CollectionReady(singleCollectionQualified bool) (bool, string) {
	if err := r.Validate(); err != nil {
		return false, err.Error()
	}
	if len(r.MissingSelectionComponents()) != 0 {
		return false, "collection-components-missing"
	}
	if singleCollectionQualified {
		return true, ""
	}
	if r.CollectionCount < 2 {
		return false, "collection-count-insufficient"
	}
	if r.Nondeterministic {
		return false, "nondeterministic"
	}
	return true, ""
}
