// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
)

// AZU-V0-8905: adding the optional driver cannot change frozen graph
// identity or output when the tree has no driver input.
func TestEmptyDriverPreservesGraphIdentity_AZU_V0_8905(t *testing.T) {
	root := t.TempDir()
	g := fake{name: "g", units: []Unit{{ID: "g:a", Sources: []string{"a.g"}, Tests: []string{"a_test.g"}}}}
	before, err := Build(context.Background(), root, g)
	if err != nil {
		t.Fatal(err)
	}
	after, err := Build(context.Background(), root, g, Driver{})
	if err != nil {
		t.Fatal(err)
	}
	if before.Digest() != after.Digest() {
		t.Fatalf("empty driver changed graph identity: %s -> %s", before.Digest(), after.Digest())
	}
}

// AZU-V0-8905: the registered provider retains its historical namespace even
// when the optional driver would have no observed content.
func TestRegisteredDriverRetainsEmptyIdentity_AZU_V0_8905(t *testing.T) {
	graph, err := Build(context.Background(), t.TempDir(), RegisteredDriver{})
	if err != nil {
		t.Fatal(err)
	}
	body, err := graph.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	var canonical struct {
		Languages []string `json:"languages"`
	}
	if err := json.Unmarshal(body, &canonical); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(canonical.Languages, []string{DriverNamespace}) {
		t.Fatalf("registered driver namespace missing: %s", body)
	}
}
