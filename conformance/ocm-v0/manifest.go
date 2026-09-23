// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"fmt"
	"path/filepath"
)

// Manifest is the suite ledger: which spec and profile it freezes, and which
// vectors and fixtures cover which wire clauses.
type Manifest struct {
	Schema     string   `json:"schema"`
	Spec       string   `json:"spec"`
	Profile    string   `json:"profile"`
	Derivation string   `json:"derivation"`
	Vectors    []string `json:"vectors"`
	Fixtures   []string `json:"fixtures"`
	Coverage   []Cover  `json:"coverage"`
}

// Cover ties one wire clause to the vector and fixture-case IDs that freeze it.
type Cover struct {
	Clause string   `json:"clause"`
	Cases  []string `json:"cases"`
}

const manifestSchema = "corvint.ocm-v0-conformance/1"

// LoadManifest reads manifest.json under dir.
func LoadManifest(dir string) (Manifest, error) {
	var m Manifest
	if err := loadJSON(filepath.Join(dir, "manifest.json"), &m); err != nil {
		return m, err
	}
	if m.Schema != manifestSchema {
		return m, fmt.Errorf("manifest.json: schema %q, want %q", m.Schema, manifestSchema)
	}
	return m, nil
}

// Validate checks that the manifest names exactly the loaded vectors and
// fixtures and that every coverage case exists.
func (m Manifest) Validate(vectors []StructuralVector, fixtures []Fixture) error {
	known := map[string]bool{}
	for _, v := range vectors {
		known[v.ID] = true
	}
	for _, f := range fixtures {
		for _, c := range f.Cases {
			known[f.ID+"/"+c.ID] = true
		}
	}
	if err := sameSet("vectors", m.Vectors, vectorIDs(vectors)); err != nil {
		return err
	}
	if err := sameSet("fixtures", m.Fixtures, fixtureIDs(fixtures)); err != nil {
		return err
	}
	for _, cover := range m.Coverage {
		if len(cover.Cases) == 0 {
			return fmt.Errorf("manifest.json: clause %q has no cases", cover.Clause)
		}
		for _, id := range cover.Cases {
			if !known[id] {
				return fmt.Errorf("manifest.json: clause %q names unknown case %q", cover.Clause, id)
			}
		}
	}
	return nil
}

func vectorIDs(vectors []StructuralVector) []string {
	ids := make([]string, 0, len(vectors))
	for _, v := range vectors {
		ids = append(ids, v.ID)
	}
	return ids
}

func fixtureIDs(fixtures []Fixture) []string {
	ids := make([]string, 0, len(fixtures))
	for _, f := range fixtures {
		ids = append(ids, f.ID)
	}
	return ids
}

func sameSet(label string, declared, loaded []string) error {
	want := map[string]bool{}
	for _, id := range loaded {
		want[id] = true
	}
	if len(declared) != len(want) {
		return fmt.Errorf("manifest.json: %s lists %d entries, %d loaded", label, len(declared), len(want))
	}
	for _, id := range declared {
		if !want[id] {
			return fmt.Errorf("manifest.json: %s names unknown %q", label, id)
		}
		delete(want, id)
	}
	return nil
}
