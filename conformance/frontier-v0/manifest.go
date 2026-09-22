// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"fmt"
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
