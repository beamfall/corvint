// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManifestPinsRetirementAndArtifacts requires the retired frontier/0
// fixture tier to be recorded with its decision, requirement, reason, matrix
// rows and cases (CEM-V1-217), and every vector file to match its pinned
// digest.
func TestManifestPinsRetirementAndArtifacts(t *testing.T) {
	m, err := LoadManifest(".")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateRetired(); err != nil {
		t.Fatal(err)
	}
	if m.Retired.Requirement != "CEM-V1-217" || !strings.Contains(m.Retired.Reason, "frontier-stable-view/0") {
		t.Fatalf("retirement record does not name CEM-V1-217 and the stable successor: %+v", m.Retired)
	}
	if err := m.ValidateArtifacts("."); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("fixtures"); !os.IsNotExist(err) {
		t.Fatalf("retired fixtures directory still present: %v", err)
	}
}

// TestArtifactDigestDriftFails edits one byte of a frozen vector in a copy of
// the suite and requires the pinned artifactSha256 check to refuse it.
func TestArtifactDigestDriftFails(t *testing.T) {
	m, err := LoadManifest(".")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for path := range m.ArtifactSHA256 {
		raw, err := os.ReadFile(filepath.FromSlash(path))
		if err != nil {
			t.Fatal(err)
		}
		if path == "vectors/codec.json" {
			raw = append(raw, '\n')
		}
		copyPath := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(copyPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(copyPath, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err = m.ValidateArtifacts(dir)
	if err == nil || !strings.Contains(err.Error(), "vectors/codec.json") {
		t.Fatalf("drifted vector accepted: %v", err)
	}
}
