// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

// Command ocm-v0 reports what this conformance suite covers and validates its
// own data. It runs no implementation.
//
//	go run ./conformance/ocm-v0            # coverage report
//	go run ./conformance/ocm-v0 -check     # validate vectors, fixtures, and manifest
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	check := flag.Bool("check", false, "validate every vector and fixture and exit non-zero on a defect")
	dir := flag.String("dir", "", "suite directory (default: the directory of this source tree)")
	flag.Parse()
	root := *dir
	if root == "" {
		root = defaultDir()
	}
	if err := run(root, *check); err != nil {
		fmt.Fprintln(os.Stderr, "ocm-v0:", err)
		os.Exit(1)
	}
}

func defaultDir() string {
	if _, err := os.Stat("manifest.json"); err == nil {
		return "."
	}
	return filepath.Join("conformance", "ocm-v0")
}

func run(dir string, check bool) error {
	vectors, err := LoadStructuralVectors(dir)
	if err != nil {
		return err
	}
	fixtures, err := LoadFixtures(dir)
	if err != nil {
		return err
	}
	manifest, err := LoadManifest(dir)
	if err != nil {
		return err
	}
	cases := 0
	for _, f := range fixtures {
		cases += len(f.Cases)
	}
	fmt.Printf("%s: %d structural vectors, %d fixtures (%d cases)\n", manifest.Profile, len(vectors), len(fixtures), cases)
	if !check {
		return nil
	}
	if err := ValidateVectors(vectors); err != nil {
		return err
	}
	for _, f := range fixtures {
		if err := f.Validate(); err != nil {
			return err
		}
	}
	if err := manifest.Validate(vectors, fixtures); err != nil {
		return err
	}
	return manifest.ValidateArtifacts(dir)
}
