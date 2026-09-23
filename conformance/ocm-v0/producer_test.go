// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"bytes"
	"testing"
)

// TestFrozenVectorsMatchTheRealProducer rebuilds each valid vector's seed
// universe and drives the real `ocm prepare`, `ocm link`, and `ocm mark`
// producers over it. The bytes they write must equal the frozen vector: that
// proves the vectors are genuine producer output and that the producers are
// deterministic. A difference here is a wire change and must be recorded in
// the OCM spec before the vector is refrozen.
func TestFrozenVectorsMatchTheRealProducer(t *testing.T) {
	vectors, err := LoadStructuralVectors(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		if len(v.Universe) == 0 {
			continue
		}
		t.Run(v.ID, func(t *testing.T) {
			u, cleanup, err := BuildUniverse(v.Universe)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			if !bytes.Equal(u.OCMRaw, []byte(v.Raw)) {
				t.Fatalf("producer bytes drifted from the frozen vector:\n got %s\nwant %s", u.OCMRaw, v.Raw)
			}
			verdict, err := Verify(u, u.OCMRaw, u.Target)
			if err != nil {
				t.Fatal(err)
			}
			if verdict.State != "ready-for-review" {
				t.Fatalf("produced map verifies as %q (%s), want ready-for-review", verdict.State, verdict.Code)
			}
		})
	}
}
