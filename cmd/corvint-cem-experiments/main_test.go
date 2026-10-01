package main

import (
	"bytes"
	"context"
	"testing"
)

// CEX-V0-001 and CEX-V0-009: admission failures emit no success payload.
func TestCLIAdmissionAndUnknownVerb(t *testing.T) {
	for _, args := range [][]string{nil, {"wat"}, {"run", "--repo", "/missing"}, {"run", "--experimental", "--trusted-local", "--approve", "wrong", "--repo", "/missing"}, {"verify", "extra"}} {
		var out, errout bytes.Buffer
		if command(context.Background(), args, &out, &errout) == 0 {
			t.Fatalf("admitted %v", args)
		}
		if out.Len() != 0 {
			t.Fatal("failure printed success payload")
		}
	}
}
