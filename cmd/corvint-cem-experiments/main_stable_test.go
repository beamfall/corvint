package main

import (
	"bytes"
	"context"
	"testing"
)

// TestCEMV1139StableFlagRefusesWithoutPayload (CEM-V1-139): every phase given
// --stable-artifact-root takes the stable entry and, on refusal, prints no
// success payload; the flag cannot be repeated.
func TestCEMV1139StableFlagRefusesWithoutPayload(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"plan", "--stable-artifact-root", root, "--request", "/missing/request.json"},
		{"run", "--stable-artifact-root", root, "--experimental", "--trusted-local", "--approve", "wrong", "--repo", "/missing"},
		{"verify", "--stable-artifact-root", root, "--repo", "/missing"},
		{"gate", "--stable-artifact-root", root, "--stable-artifact-root", root},
	} {
		var out, errout bytes.Buffer
		if command(context.Background(), args, &out, &errout) == 0 || out.Len() != 0 {
			t.Fatalf("admitted %v: %q", args, out.String())
		}
	}
}
