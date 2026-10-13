// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

// Command frontier-v0 reports what this conformance suite holds and validates
// its own data. It runs no implementation. The frontier/0 fixture tier was
// retired with the cem/0.x wire (decision 0525 §1, CEM-V1-217); the codec and
// identity vectors remain.
//
//	go run ./conformance/frontier-v0            # report
//	go run ./conformance/frontier-v0 -check     # validate vectors and the ledger
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	check := flag.Bool("check", false, "validate every vector and the manifest ledger and exit non-zero on a defect")
	dir := flag.String("dir", "", "suite directory (default: the directory of this source tree)")
	flag.Parse()

	root := *dir
	if root == "" {
		root = defaultDir()
	}
	if err := run(root, *check); err != nil {
		fmt.Fprintln(os.Stderr, "frontier-v0:", err)
		os.Exit(1)
	}
}

func defaultDir() string {
	if _, err := os.Stat("manifest.json"); err == nil {
		return "."
	}
	return filepath.Join("conformance", "frontier-v0")
}

func run(dir string, check bool) error {
	m, err := LoadManifest(dir)
	if err != nil {
		return err
	}
	cv, err := LoadCodecVectors(dir)
	if err != nil {
		return err
	}
	iv, err := LoadIdentityVectors(dir)
	if err != nil {
		return err
	}
	modes := map[string]int{}
	for _, v := range cv.Vectors {
		modes[v.Mode]++
	}
	retiredCases := 0
	for _, f := range m.Retired.Fixtures {
		retiredCases += len(f.Cases)
	}

	fmt.Printf("suite      %s\n", m.Schema)
	fmt.Printf("spec       %s\n", m.Spec)
	fmt.Printf("profile    %s (errors: %s)\n", m.Profile, m.ErrorProfil)
	fmt.Printf("codec      %d vectors (serialize %d, invalid %d, verify %d, document %d)\n",
		len(cv.Vectors), modes["serialize"], modes["invalid"], modes["verify"], modes["document"])
	fmt.Printf("identity   %d universes, %d items, 1 frontier id\n", len(iv.Universes), len(iv.Items))
	fmt.Printf("retired    %d fixtures, %d cases (decision %s, %s)\n",
		len(m.Retired.Fixtures), retiredCases, m.Retired.Decision, m.Retired.Requirement)

	if !check {
		return nil
	}
	for _, v := range cv.Vectors {
		if err := checkVector(v); err != nil {
			return err
		}
	}
	for _, u := range iv.Universes {
		want := "frontier-universe:sha256:" + DomainHash(u.Domain, []byte(u.PreimageCodec))
		if want != u.UniverseID {
			return fmt.Errorf("universe %s: recorded ID does not reproduce", u.ID)
		}
	}
	for _, it := range iv.Items {
		want := "frontier-item:sha256:" + DomainHash(it.Domain, []byte(it.PreimageCodec))
		if want != it.ItemID {
			return fmt.Errorf("item %s: recorded ID does not reproduce", it.ID)
		}
	}
	if err := m.ValidateRetired(); err != nil {
		return err
	}
	if err := m.ValidateArtifacts(dir); err != nil {
		return err
	}
	fmt.Println("\nall vectors and the retirement ledger validate")
	return nil
}

func checkVector(v CodecVector) error {
	in, err := v.InputBytes()
	if err != nil {
		return err
	}
	switch v.Mode {
	case "serialize":
		want, err := v.ExpectBytes()
		if err != nil {
			return err
		}
		val, err := Parse(in)
		if err != nil {
			return fmt.Errorf("%s: %w", v.ID, err)
		}
		if got := string(Codec(val)); got != string(want) {
			return fmt.Errorf("%s: canonical bytes differ", v.ID)
		}
	case "invalid":
		if _, err := Parse(in); err == nil {
			return fmt.Errorf("%s: invalid input was accepted", v.ID)
		} else if Reason(err) != v.Reason {
			return fmt.Errorf("%s: rejected for %q, want %q", v.ID, Reason(err), v.Reason)
		}
	case "verify":
		ok, err := IsCanonical(in)
		if err != nil {
			return fmt.Errorf("%s: %w", v.ID, err)
		}
		if ok != v.Canonical {
			return fmt.Errorf("%s: canonicality mismatch", v.ID)
		}
	case "document":
		ok, err := IsCompleteDocument(in)
		if err != nil {
			return fmt.Errorf("%s: %w", v.ID, err)
		}
		if ok != v.Canonical {
			return fmt.Errorf("%s: document framing mismatch", v.ID)
		}
	default:
		return fmt.Errorf("%s: unknown mode %q", v.ID, v.Mode)
	}
	return nil
}
