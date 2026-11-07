// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// AFP-V0-9014 (V1-4710): the real dependency closure selects the frozen
// profiles for graph-digest inputs, and source slicing cannot omit the test
// after an exclusion-digest implementation change. No golden is rewritten.
func TestCoreFreezeDigestSelection_AFPV09014(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	const pkg = "github.com/Beamfall/corvint/cmd/corvint"
	const frozen = "TestCoreVerbsEmitTheFrozenProfiles"
	for _, dirty := range []string{
		"internal/liveverify/affected/graph.go",
		"internal/liveverify/affected/select.go",
		"internal/liveverify/affected/golang/golang.go",
		"cmd/corvint/affected_compact.go",
	} {
		t.Run(dirty, func(t *testing.T) {
			plan := affected.Select(graph, []string{dirty})
			selected := false
			for _, s := range plan.Selected {
				if s.UnitID == "go:"+pkg {
					selected = true
				}
			}
			if !selected {
				t.Fatalf("frozen profile package omitted for %s: %+v", dirty, plan)
			}
			// For dependency changes cmd/corvint runs whole. For its own
			// source, inject a real digest-changing base edit below.
			if !strings.HasPrefix(dirty, "cmd/corvint/") {
				slice := testGoSlices(context.Background(), plan, graph, root, "", typedReachExecutor{environ: os.Environ()})[pkg]
				if slice.Pattern != "" && !strings.Contains(slice.Pattern, frozen) {
					t.Fatalf("digest input sliced away frozen profiles: %+v", slice)
				}
				return
			}
			source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(dirty)))
			if err != nil {
				t.Fatal(err)
			}
			base := strings.Replace(string(source), "sum := sha256.Sum256(encoded)", "sum := sha256.Sum256(append(encoded, 'x'))", 1)
			if base == string(source) {
				t.Fatal("digest edit no longer matches implementation")
			}
			candidate := testSourceCandidate{pkg: pkg, fallback: "whole", own: []string{dirty}}
			slice := testSourceSlice(root, candidate, map[string]testBaseBlob{dirty: {content: []byte(base)}})
			if slice.Pattern != "" && !strings.Contains(slice.Pattern, frozen) {
				t.Fatalf("exclusion digest edit sliced away frozen profiles: %+v", slice)
			}
			t.Logf("selected frozen profiles: pattern=%q reason=%s", slice.Pattern, slice.Reason)
		})
	}
}
