// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

// Command frontier-v0 reports what this conformance suite covers and validates
// its own data. It runs no implementation: the suite is deliberately readable
// and checkable before internal/frontier exports an entry point.
//
//	go run ./conformance/frontier-v0            # coverage report
//	go run ./conformance/frontier-v0 -check     # validate vectors and fixtures
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	fx, err := LoadFixtures(dir)
	if err != nil {
		return err
	}

	modes := map[string]int{}
	for _, v := range cv.Vectors {
		modes[v.Mode]++
	}
	cases := 0
	for _, f := range fx {
		cases += len(f.Cases)
	}

	fmt.Printf("suite      %s\n", m.Schema)
	fmt.Printf("spec       %s\n", m.Spec)
	fmt.Printf("profile    %s (errors: %s)\n", m.Profile, m.ErrorProfil)
	fmt.Printf("codec      %d vectors (serialize %d, invalid %d, verify %d, document %d)\n",
		len(cv.Vectors), modes["serialize"], modes["invalid"], modes["verify"], modes["document"])
	fmt.Printf("identity   %d universes, %d items, 1 frontier id\n", len(iv.Universes), len(iv.Items))
	fmt.Printf("fixtures   %d fixtures, %d cases\n", len(fx), cases)
	fmt.Printf("matrix     %d rows\n\n", len(m.Matrix))

	uncovered := 0
	for _, r := range m.Matrix {
		mark := "ok  "
		if len(r.Fixtures) == 0 {
			mark = "GAP "
			uncovered++
		}
		fmt.Printf("%s %-78s %v\n", mark, truncate(r.MatrixRow, 78), r.Fixtures)
	}
	if uncovered > 0 {
		return fmt.Errorf("%d matrix rows are uncovered", uncovered)
	}
	reportSatisfiability(fx)

	if !check {
		return nil
	}
	for _, f := range fx {
		if err := f.Validate(); err != nil {
			return err
		}
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
	if err := m.ValidateStates(fx, cv.Vectors); err != nil {
		return err
	}
	if err := m.ValidateArtifacts(dir); err != nil {
		return err
	}
	fmt.Println("\nall vectors and fixtures validate")
	return nil
}

// reportSatisfiability prints how many fixture cases the bound runner can
// actually execute and, for the rest, which capability puts their declared
// universe out of reach. It is here rather than only in the test log because
// the honest number is the point: a suite that skips is reporting a result, and
// the result is worthless unless the reason is named and countable.
func reportSatisfiability(fixtures []Fixture) {
	if registeredRunner == nil {
		fmt.Println("\nrunner     unbound: every fixture case skips")
		return
	}
	blocked := map[Capability]int{}
	executable := 0
	for _, f := range fixtures {
		for _, c := range f.Cases {
			missing := Capability("")
			for _, capability := range RequiredCapabilities(f, c) {
				if !registeredRunner.Supports(capability) {
					missing = capability
					break
				}
			}
			if missing == "" {
				executable++
				continue
			}
			blocked[missing]++
		}
	}
	fmt.Printf("\nrunner     %d cases execute against the implementation\n", executable)
	names := make([]string, 0, len(blocked))
	for capability := range blocked {
		names = append(names, string(capability))
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Printf("skip       %-28s %d case(s)\n", name, blocked[Capability(name)])
	}
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
