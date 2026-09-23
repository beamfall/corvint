// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
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
	// States pins each hostile evidence state to the cases that freeze its
	// decoded outcome, or names why the state does not apply.
	States []StateCover `json:"states"`
	// ArtifactSHA256 is the SHA-256 of every file under vectors/ and
	// fixtures/, keyed by suite-relative slash path.
	ArtifactSHA256 map[string]string `json:"artifactSha256"`
}

// StateCover ties one evidence state to its cases or to a not-applicable reason.
type StateCover struct {
	State         string   `json:"state"`
	Cases         []string `json:"cases,omitempty"`
	NotApplicable string   `json:"notApplicable,omitempty"`
}

// hostileStates is the closed state vocabulary every frozen proof profile pins.
var hostileStates = []string{"stable", "relocated", "stale", "ambiguous", "deleted", "unknown"}

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
	return m.validateStates(known)
}

// validateStates requires the six states in order, each with known cases or
// a not-applicable reason, never both.
func (m Manifest) validateStates(known map[string]bool) error {
	if len(m.States) != len(hostileStates) {
		return fmt.Errorf("manifest.json: states lists %d entries, want %d", len(m.States), len(hostileStates))
	}
	for index, cover := range m.States {
		if cover.State != hostileStates[index] {
			return fmt.Errorf("manifest.json: state %d is %q, want %q", index, cover.State, hostileStates[index])
		}
		if (len(cover.Cases) == 0) == (cover.NotApplicable == "") {
			return fmt.Errorf("manifest.json: state %q needs cases or a notApplicable reason", cover.State)
		}
		for _, id := range cover.Cases {
			if !known[id] {
				return fmt.Errorf("manifest.json: state %q names unknown case %q", cover.State, id)
			}
		}
	}
	return nil
}

// ValidateArtifacts requires artifactSha256 to name exactly the files under
// vectors/ and fixtures/ with their current digests, so any byte drift in a
// frozen vector fails.
func (m Manifest) ValidateArtifacts(dir string) error {
	actual, err := artifactDigests(dir)
	if err != nil {
		return err
	}
	if len(actual) != len(m.ArtifactSHA256) {
		return fmt.Errorf("manifest.json: artifactSha256 lists %d files, %d present", len(m.ArtifactSHA256), len(actual))
	}
	for path, digest := range actual {
		if m.ArtifactSHA256[path] != digest {
			return fmt.Errorf("manifest.json: artifact %s digest %s, pinned %q", path, digest, m.ArtifactSHA256[path])
		}
	}
	return nil
}

func artifactDigests(dir string) (map[string]string, error) {
	digests := map[string]string{}
	for _, tree := range []string{"vectors", "fixtures"} {
		err := filepath.WalkDir(filepath.Join(dir, tree), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(raw)
			digests[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return digests, nil
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
