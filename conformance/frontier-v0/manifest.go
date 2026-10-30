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

// Manifest is the suite's ledger. The frontier/0 fixture tier was retired with
// the cem/0.x wire (decision 0525 §1, CEM-V1-217): Retired records every
// retired fixture case with its matrix rows and the reason. What remains are the
// independently authored CF-V0-019 codec and CF-V0-006/007 identity vectors,
// pinned by ArtifactSHA256.
type Manifest struct {
	Schema      string   `json:"schema"`
	Spec        string   `json:"spec"`
	Profile     string   `json:"profile"`
	ErrorProfil string   `json:"errorProfile"`
	Derivation  string   `json:"derivation"`
	VectorFiles []string `json:"vectorFiles"`
	Retired     Retired  `json:"retired"`
	// ArtifactSHA256 is the SHA-256 of every file under vectors/, keyed by
	// suite-relative slash path.
	ArtifactSHA256 map[string]string `json:"artifactSha256"`
}

// Retired is the record of the retired fixture tier.
type Retired struct {
	Decision    string           `json:"decision"`
	Requirement string           `json:"requirement"`
	Reason      string           `json:"reason"`
	Fixtures    []RetiredFixture `json:"fixtures"`
}

// RetiredFixture names one retired fixture, the spec matrix rows it covered
// and each of its retired cases.
type RetiredFixture struct {
	Fixture    string   `json:"fixture"`
	MatrixRows []string `json:"matrixRows"`
	Cases      []string `json:"cases"`
}

// ValidateRetired requires the retirement record to name its decision, its
// requirement and its reason, and every retired fixture to name its rows and
// cases, so a retirement can never be silent.
func (m Manifest) ValidateRetired() error {
	r := m.Retired
	if r.Decision == "" || r.Requirement == "" || r.Reason == "" {
		return fmt.Errorf("manifest.json: retired needs decision, requirement and reason")
	}
	if len(r.Fixtures) == 0 {
		return fmt.Errorf("manifest.json: retired names no fixtures")
	}
	seen := map[string]bool{}
	for _, f := range r.Fixtures {
		if f.Fixture == "" || seen[f.Fixture] || len(f.MatrixRows) == 0 || len(f.Cases) == 0 {
			return fmt.Errorf("manifest.json: retired fixture %q needs a unique id, matrix rows and cases", f.Fixture)
		}
		seen[f.Fixture] = true
	}
	return nil
}

// ValidateArtifacts requires artifactSha256 to name exactly the files under
// vectors/ with their current digests, so any byte drift in a
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
	for _, tree := range []string{"vectors"} {
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

// LoadManifest reads manifest.json relative to dir.
func LoadManifest(dir string) (Manifest, error) {
	var m Manifest
	err := loadJSON(filepath.Join(dir, "manifest.json"), &m)
	return m, err
}
