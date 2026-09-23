// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"testing"
)

// TestFixturesAgainstRealVerifier builds each fixture's seed universe once,
// then offers every case's perturbed map to the real `ocm status` reader
// (adapter.go, Verify) and asserts the exact verdict the case declares.
func TestFixturesAgainstRealVerifier(t *testing.T) {
	fixtures, err := LoadFixtures(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.ID, func(t *testing.T) {
			if err := f.Validate(); err != nil {
				t.Fatal(err)
			}
			u, cleanup, err := BuildUniverse(f.Obligations)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			for _, c := range f.Cases {
				t.Run(c.ID, func(t *testing.T) { runCase(t, u, c) })
			}
		})
	}
}

func runCase(t *testing.T, u *Universe, c Case) {
	t.Helper()
	perturbed, err := c.Apply(u.OCMRaw)
	if err != nil {
		t.Fatal(err)
	}
	verdict, err := Verify(u, perturbed, callerTarget(u, c.Expect))
	if err != nil {
		t.Fatalf("verifier returned a non-refusal error: %v", err)
	}
	assertVerdict(t, c.Expect, verdict)
}

func callerTarget(u *Universe, e Expect) string {
	if e.Target == "successor" {
		return u.Successor
	}
	return u.Target
}

func assertVerdict(t *testing.T, e Expect, v Verdict) {
	t.Helper()
	if v.Refusal != e.Refusal {
		t.Fatalf("refusal %q, want %q", v.Refusal, e.Refusal)
	}
	if v.State != e.State || v.Code != e.Code {
		t.Fatalf("verdict %s/%s, want %s/%s", v.State, v.Code, e.State, e.Code)
	}
	if e.Linked != nil && v.Counts["linked"] != *e.Linked {
		t.Fatalf("linked %d, want %d", v.Counts["linked"], *e.Linked)
	}
	if e.Unknown != nil && v.Counts["unknown"] != *e.Unknown {
		t.Fatalf("unknown %d, want %d", v.Counts["unknown"], *e.Unknown)
	}
}

// TestSuiteDataIsSelfConsistent runs the suite's own `-check` validation so a
// fixture, vector, or manifest defect fails here before any implementation
// is consulted.
func TestSuiteDataIsSelfConsistent(t *testing.T) {
	if err := run(".", true); err != nil {
		t.Fatal(err)
	}
}
