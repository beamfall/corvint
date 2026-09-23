// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Structural vectors freeze the OCM wire at the byte level. Each vector carries
// the exact map bytes as `raw` and one expected outcome from the structural
// parser (see adapter.go, DecodeStructural):
//
//   - valid:    the map parses; re-marking `obligation` to the reason it
//               already carries republishes byte-identical output, which is the
//               canonical round-trip identity the wire profile promises.
//   - produced: the map parses and the mark of `obligation` to `reason` is
//               applied; no identity holds because the marked row changes.
//   - invalid:  the parser refuses with exactly `code`.
//   - oversize: `raw` is padded with trailing spaces to one byte past the
//               map limit before it is offered; the reader refuses with `code`.
//   - count:    `raw`'s obligations are replaced by `obligations` synthetic
//               `unknown`/`unassessed` rows and re-canonicalized before they are
//               offered; the parser refuses with `code`.
//
// The `valid` and `produced` raws are the genuine bytes `ocm prepare`,
// `ocm link`, and `ocm mark` wrote for the seed universe their `universe`
// member declares (universe.go); producer_test.go proves that on every run.

// MaxMapBytes is the OCM map size limit the wire profile publishes.
const MaxMapBytes = 1 << 20

// StructuralVector is one frozen byte-level case.
type StructuralVector struct {
	ID   string `json:"id"`
	Note string `json:"note"`
	Mode string `json:"mode"`
	Raw  string `json:"raw"`
	// Universe declares the seed obligations whose real production wrote Raw.
	Universe []DeclaredObligation `json:"universe,omitempty"`
	// Obligation and Reason name the `ocm mark` a valid vector round-trips
	// through; the reason must be the one the row already carries.
	Obligation string `json:"obligation,omitempty"`
	Reason     string `json:"reason,omitempty"`
	// Code is the expected refusal for every mode but `valid`.
	Code string `json:"code,omitempty"`
	// Obligations is the synthetic row count for `count` vectors.
	Obligations int `json:"obligations,omitempty"`
}

type structuralFile struct {
	Schema  string             `json:"schema"`
	Vectors []StructuralVector `json:"vectors"`
}

const structuralSchema = "corvint.ocm-v0-conformance.structural/1"

// LoadStructuralVectors reads vectors/structural.json under dir.
func LoadStructuralVectors(dir string) ([]StructuralVector, error) {
	var file structuralFile
	if err := loadJSON(filepath.Join(dir, "vectors", "structural.json"), &file); err != nil {
		return nil, err
	}
	if file.Schema != structuralSchema {
		return nil, fmt.Errorf("vectors/structural.json: schema %q, want %q", file.Schema, structuralSchema)
	}
	return file.Vectors, nil
}

// ValidateVectors checks the data invariants every vector must hold before it
// is offered to the implementation.
func ValidateVectors(vectors []StructuralVector) error {
	seen := map[string]bool{}
	for _, v := range vectors {
		if err := v.validate(); err != nil {
			return err
		}
		if seen[v.ID] {
			return fmt.Errorf("vector %q: duplicate ID", v.ID)
		}
		seen[v.ID] = true
	}
	return nil
}

func (v StructuralVector) validate() error {
	if v.ID == "" || v.Note == "" || v.Raw == "" {
		return fmt.Errorf("vector %q: id, note, and raw are required", v.ID)
	}
	switch v.Mode {
	case "valid":
		if v.Obligation == "" || v.Reason == "" || v.Code != "" || len(v.Universe) == 0 {
			return fmt.Errorf("vector %q: valid needs obligation, reason, and universe and no code", v.ID)
		}
	case "produced":
		if v.Code != "" || len(v.Universe) == 0 {
			return fmt.Errorf("vector %q: produced needs a universe and no code", v.ID)
		}
	case "invalid", "oversize":
		if v.Code == "" {
			return fmt.Errorf("vector %q: %s needs a code", v.ID, v.Mode)
		}
	case "count":
		if v.Code == "" || v.Obligations == 0 {
			return fmt.Errorf("vector %q: count needs a code and an obligation count", v.ID)
		}
	default:
		return fmt.Errorf("vector %q: unknown mode %q", v.ID, v.Mode)
	}
	return nil
}

// Input is the exact bytes offered to the implementation.
func (v StructuralVector) Input() ([]byte, error) {
	switch v.Mode {
	case "oversize":
		return padToOversize(v.Raw), nil
	case "count":
		return expandObligations(v.Raw, v.Obligations)
	}
	return []byte(v.Raw), nil
}

// padToOversize keeps the JSON value intact and appends whitespace until the
// document is one byte over the limit, so only the size bound can refuse it.
func padToOversize(raw string) []byte {
	return []byte(raw + strings.Repeat(" ", MaxMapBytes+1-len(raw)))
}

// expandObligations replaces the obligations array with n `unknown` rows and
// re-canonicalizes, so only the row bound can refuse the result.
func expandObligations(raw string, n int) ([]byte, error) {
	root, err := wire.Parse([]byte(raw))
	if err != nil {
		return nil, err
	}
	rows := make([]wire.Value, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, unknownRow(fmt.Sprintf("CONF-V0-%03d", i+1)))
	}
	setMember(root.Obj, "obligations", wire.Value{Kind: wire.KindArray, Arr: rows})
	return append(wire.CanonicalValue(root), '\n'), nil
}

func unknownRow(id string) wire.Value {
	empty := wire.Value{Kind: wire.KindArray, Arr: []wire.Value{}}
	row := &wire.Object{Values: map[string]wire.Value{}}
	setMember(row, "claimIds", empty)
	setMember(row, "disposition", stringValue("unknown"))
	setMember(row, "hunkIds", empty)
	setMember(row, "id", stringValue(id))
	setMember(row, "reason", stringValue("unassessed"))
	return wire.Value{Kind: wire.KindObject, Obj: row}
}

func loadJSON(path string, into any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
