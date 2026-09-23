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
	"strings"
)

// Manifest maps every row of the spec's "Conformance and adversarial matrix"
// to the fixtures that cover it. CF-V0-030 keeps delivery experimental and candidate-only until
// canonical vectors, adversarial fixtures, full local gate, and independent
// review exist; this manifest is the ledger for the first two. The separately
// constructed third-party-PR replay remains NOT_RUN until the first release cut.
type Manifest struct {
	Schema      string      `json:"schema"`
	Spec        string      `json:"spec"`
	Profile     string      `json:"profile"`
	ErrorProfil string      `json:"errorProfile"`
	Derivation  string      `json:"derivation"`
	VectorFiles []string    `json:"vectorFiles"`
	Matrix      []MatrixRow `json:"matrix"`
	// States pins each hostile evidence state to the fixture cases or codec
	// vectors that freeze its decoded outcome, and names any gap.
	States []StateCover `json:"states"`
	// ArtifactSHA256 is the SHA-256 of every file under vectors/ and
	// fixtures/, keyed by suite-relative slash path.
	ArtifactSHA256 map[string]string `json:"artifactSha256"`
}

// StateCover ties one evidence state to its cases ("fixture/case" or
// "codec:<vector id>") and to the part of the state no vector reaches yet.
type StateCover struct {
	State string   `json:"state"`
	Cases []string `json:"cases,omitempty"`
	Gap   string   `json:"gap,omitempty"`
}

// hostileStates is the closed state vocabulary every frozen proof profile pins.
var hostileStates = []string{"stable", "relocated", "stale", "ambiguous", "deleted", "unknown"}

// ValidateStates requires the six states in order, each naming existing
// cases or stating its gap.
func (m Manifest) ValidateStates(fixtures []Fixture, codec []CodecVector) error {
	known := map[string]bool{}
	for _, f := range fixtures {
		for _, c := range f.Cases {
			known[f.ID+"/"+c.ID] = true
		}
	}
	for _, v := range codec {
		known["codec:"+v.ID] = true
	}
	if len(m.States) != len(hostileStates) {
		return fmt.Errorf("manifest.json: states lists %d entries, want %d", len(m.States), len(hostileStates))
	}
	for index, cover := range m.States {
		if cover.State != hostileStates[index] {
			return fmt.Errorf("manifest.json: state %d is %q, want %q", index, cover.State, hostileStates[index])
		}
		if len(cover.Cases) == 0 && cover.Gap == "" {
			return fmt.Errorf("manifest.json: state %q needs cases or a stated gap", cover.State)
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

// MatrixRow is one verbatim row of the spec table plus the fixtures covering it.
type MatrixRow struct {
	// MatrixRow is the left cell, verbatim.
	MatrixRow string `json:"matrixRow"`
	// RequiredResult is the right cell, verbatim.
	RequiredResult string `json:"requiredResult"`
	// Fixtures are the fixture ids covering the row. An empty list is an
	// uncovered row and fails the manifest test.
	Fixtures []string `json:"fixtures"`
}

// LoadManifest reads manifest.json relative to dir.
func LoadManifest(dir string) (Manifest, error) {
	var m Manifest
	err := loadJSON(filepath.Join(dir, "manifest.json"), &m)
	return m, err
}

// SpecMatrixRows re-extracts the matrix table from the live specification. The
// manifest is a snapshot; this reader is the guard that makes the snapshot go
// stale loudly. If the spec grows a row, the manifest test fails until a
// fixture covers it.
func SpecMatrixRows(specPath string) ([]MatrixRow, error) {
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}
	text := string(raw)
	const startMarker = "## Conformance and adversarial matrix"
	const endMarker = "## Evaluation and kill criteria"
	i := strings.Index(text, startMarker)
	if i < 0 {
		return nil, fmt.Errorf("%s: no %q heading", specPath, startMarker)
	}
	rest := text[i+len(startMarker):]
	if j := strings.Index(rest, endMarker); j >= 0 {
		rest = rest[:j]
	}
	var out []MatrixRow
	for _, line := range strings.Split(rest, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || strings.HasPrefix(line, "|---") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 2 {
			continue
		}
		left := strings.TrimSpace(cells[0])
		if left == "Fixture" {
			continue
		}
		out = append(out, MatrixRow{MatrixRow: left, RequiredResult: strings.TrimSpace(cells[1])})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: matrix table has no rows", specPath)
	}
	return out, nil
}

// SpecPath is the specification this suite is authored from, relative to the
// repository root.
const SpecPath = "docs/specs/change-frontier-v0.md"

// RepoRelativeSpecPath resolves SpecPath from the conformance directory.
func RepoRelativeSpecPath(dir string) string {
	return filepath.Join(dir, "..", "..", SpecPath)
}
