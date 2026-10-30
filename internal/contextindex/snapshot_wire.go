package contextindex

import (
	"encoding/gob"
	"errors"
	"io"
	"slices"
)

// Gob writes a map in Go's randomised iteration order, so the same index
// encoded twice gave different snapshot bytes (V1-2290). The snapshot and the
// pack's gob sections therefore carry every map as key-sorted pairs: the same
// decoded index always encodes to the same bytes (IDX-SNAP-V0-106). Present
// keeps a nil map apart from an empty one, which gob also kept apart for a
// struct field, so a decoded index equals the one the old encoding returned.

// wirePairs is a map[string]V as parallel key-sorted slices.
type wirePairs[V any] struct {
	Present bool
	Keys    []string
	Values  []V
}

// wireSet is a map[string]struct{} as its sorted keys.
type wireSet struct {
	Present bool
	Keys    []string
}

// wireRecord is a Record whose Fields are key-sorted pairs.
type wireRecord struct {
	Kind, ID, Path, BlobHash string
	Line                     int
	Fields                   wirePairs[any]
}

// wireAnyMap is a map[string]any held in an interface, such as a nested
// front-matter mapping in Record.Fields.
type wireAnyMap wirePairs[any]

var errWirePairs = errors.New("snapshot map pairs are malformed")

func pairsOf[V, W any](values map[string]V, convert func(V) W) wirePairs[W] {
	if values == nil {
		return wirePairs[W]{}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	converted := make([]W, len(keys))
	for position, key := range keys {
		converted[position] = convert(values[key])
	}
	return wirePairs[W]{Present: true, Keys: keys, Values: converted}
}

func mapOf[W, V any](pairs wirePairs[W], convert func(W) (V, error)) (map[string]V, error) {
	if len(pairs.Keys) != len(pairs.Values) || (!pairs.Present && len(pairs.Keys) != 0) {
		return nil, errWirePairs
	}
	if !pairs.Present {
		return nil, nil
	}
	values := make(map[string]V, len(pairs.Keys))
	for position, key := range pairs.Keys {
		value, err := convert(pairs.Values[position])
		if err != nil {
			return nil, err
		}
		values[key] = value
	}
	return values, nil
}

func wireSetOf(values map[string]struct{}) wireSet {
	if values == nil {
		return wireSet{}
	}
	return wireSet{Present: true, Keys: sortedSet(values)}
}

func (set wireSet) set() (map[string]struct{}, error) {
	if !set.Present {
		if len(set.Keys) != 0 {
			return nil, errWirePairs
		}
		return nil, nil
	}
	values := make(map[string]struct{}, len(set.Keys))
	for _, key := range set.Keys {
		values[key] = struct{}{}
	}
	return values, nil
}

func sortedSet(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func same[V any](value V) V { return value }

func sameOK[V any](value V) (V, error) { return value, nil }

func wireRecords(records map[string]Record) wirePairs[wireRecord] {
	return pairsOf(records, func(record Record) wireRecord {
		return wireRecord{Kind: record.Kind, ID: record.ID, Path: record.Path, BlobHash: record.BlobHash, Line: record.Line, Fields: pairsOf(record.Fields, wireValue)}
	})
}

func recordsOf(pairs wirePairs[wireRecord]) (map[string]Record, error) {
	return mapOf(pairs, func(record wireRecord) (Record, error) {
		fields, err := mapOf(record.Fields, indexValue)
		return Record{Kind: record.Kind, ID: record.ID, Path: record.Path, BlobHash: record.BlobHash, Line: record.Line, Fields: fields}, err
	})
}

// wireValue rewrites the maps inside a field value; a value without one is
// returned as it is.
func wireValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return wireAnyMap(pairsOf(typed, wireValue))
	case []any:
		if !slices.ContainsFunc(typed, holdsMap) {
			return typed
		}
		converted := make([]any, len(typed))
		for position, item := range typed {
			converted[position] = wireValue(item)
		}
		return converted
	}
	return value
}

func holdsMap(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		return true
	case []any:
		return slices.ContainsFunc(typed, holdsMap)
	}
	return false
}

// indexValue reverses wireValue on a freshly decoded value, in place.
func indexValue(value any) (any, error) {
	switch typed := value.(type) {
	case wireAnyMap:
		return mapOf(wirePairs[any](typed), indexValue)
	case []any:
		for position, item := range typed {
			converted, err := indexValue(item)
			if err != nil {
				return nil, err
			}
			typed[position] = converted
		}
	}
	return value, nil
}

func markersOf(pairs wirePairs[[]Marker]) (map[string][]Marker, error) {
	return mapOf(pairs, sameOK[[]Marker])
}

func wireImports(imports map[string]map[string]struct{}) wirePairs[[]string] {
	return pairsOf(imports, sortedSet)
}

// importsOf rebuilds every inner set, empty ones included, as gob decoded an
// inner map: never nil.
func importsOf(pairs wirePairs[[]string]) (map[string]map[string]struct{}, error) {
	return mapOf(pairs, func(keys []string) (map[string]struct{}, error) {
		return wireSet{Present: true, Keys: keys}.set()
	})
}

// snapshotWire is the portable index as the gob snapshot carries it. Root,
// DirtyPaths and StatusSHA256 describe one working tree, never the file.
type snapshotWire struct {
	ObjectFormat, CommitRevision, Revision, ProfileID, Module string
	Sources                                                   wirePairs[Source]
	Tracked, Skipped                                          wireSet
	Exclusions                                                []Exclusion
	UnsupportedSuffixCount                                    int
	Features, Scenarios, Documents                            wirePairs[wireRecord]
	Markers                                                   wirePairs[[]Marker]
	Symbols                                                   []Symbol
	Fields                                                    []Symbol
	Imports                                                   wirePairs[[]string]
	Unparsed                                                  []Unparsed
	ApproximateImports                                        int
	ExtractionNotes                                           []ExtractionNote
	Vocabulary                                                *TermTable
}

func snapshotWireOf(index *Index) *snapshotWire {
	return &snapshotWire{
		ObjectFormat: index.ObjectFormat, CommitRevision: index.CommitRevision, Revision: index.Revision,
		ProfileID: index.ProfileID, Module: index.Module,
		Sources: pairsOf(index.Sources, same[Source]),
		Tracked: wireSetOf(index.Tracked), Skipped: wireSetOf(index.Skipped),
		Exclusions: index.Exclusions, UnsupportedSuffixCount: index.UnsupportedSuffixCount,
		Features: wireRecords(index.Features), Scenarios: wireRecords(index.Scenarios), Documents: wireRecords(index.Documents),
		Markers: pairsOf(index.Markers, same[[]Marker]), Symbols: index.Symbols, Fields: index.Fields, Imports: wireImports(index.Imports),
		Unparsed: index.Unparsed, ApproximateImports: index.ApproximateImports,
		ExtractionNotes: index.ExtractionNotes, Vocabulary: index.Vocabulary,
	}
}

func (wire *snapshotWire) index() (*Index, error) {
	event := eventSnapshot{
		ObjectFormat: wire.ObjectFormat, CommitRevision: wire.CommitRevision, Revision: wire.Revision,
		ProfileID: wire.ProfileID, Module: wire.Module, Sources: wire.Sources,
		Exclusions: wire.Exclusions, UnsupportedSuffixCount: wire.UnsupportedSuffixCount,
		Features: wire.Features, Scenarios: wire.Scenarios, Documents: wire.Documents,
		Markers: wire.Markers, Symbols: wire.Symbols, Fields: wire.Fields, Imports: wire.Imports,
		Unparsed: wire.Unparsed, ApproximateImports: wire.ApproximateImports, ExtractionNotes: wire.ExtractionNotes,
	}
	index, err := event.index()
	if err != nil {
		return nil, err
	}
	if index.Tracked, err = wire.Tracked.set(); err != nil {
		return nil, err
	}
	if index.Skipped, err = wire.Skipped.set(); err != nil {
		return nil, err
	}
	index.Vocabulary = wire.Vocabulary
	return index, nil
}

// warmGobTypes encodes one value of every type the snapshot and the pack
// send, in a fixed order, before any other gob use in the package. Gob
// numbers a type the first time the process meets it and writes that number
// into the stream, so without this the bytes would depend on which loads and
// writes the process happened to run first (IDX-SNAP-V0-106).
func warmGobTypes() {
	sample := &Index{
		Sources: map[string]Source{"": {}}, Tracked: map[string]struct{}{"": {}}, Skipped: map[string]struct{}{},
		Exclusions: []Exclusion{{}}, Markers: map[string][]Marker{"": {{}}}, Symbols: []Symbol{{}}, Fields: []Symbol{{}},
		Imports: map[string]map[string]struct{}{"": {"": {}}}, Unparsed: []Unparsed{{}}, ExtractionNotes: []ExtractionNote{{}},
		Features: map[string]Record{"": {Fields: map[string]any{
			"a": "", "b": []any{"", true, nil}, "c": []string{""}, "d": true, "e": map[string]any{"": ""},
		}}},
		Vocabulary: &TermTable{IdentGraph: &identGraph{}},
	}
	encoder := gob.NewEncoder(io.Discard)
	_ = encoder.Encode(snapshotHeader{})
	_ = encoder.Encode(snapshotWireOf(sample))
	for _, section := range packSectionWriters(sample, nil) {
		_ = section.write(io.Discard)
	}
	_, _ = encodePackHeader(packHeader{Sections: []packSectionEntry{{}}})
}
