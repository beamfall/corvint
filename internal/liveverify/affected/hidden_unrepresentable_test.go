// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AZU-V0-005 / AZU-V0-9900: the Go plugin's hidden-directory walk must not
// refuse a name no unit can claim (retained metadata below a dot directory),
// while an unrepresentable Go source there still refuses rather than
// silently narrowing the unit set.
func TestHiddenUnrepresentableNames_AZU_V0_005(t *testing.T) {
	base := fstest.MapFS{
		"go.mod":                   {Data: []byte("module example.test/m\n\ngo 1.27.3\n")},
		".github/helper/helper.go": {Data: []byte("package helper\n")},
	}
	unclaimed := fstest.MapFS{".metadata/name\xff.txt": {Data: []byte("x\n")}}
	for name, file := range base {
		unclaimed[name] = file
	}
	graph, err := affected.BuildFS(unclaimed, golang.New())
	if err != nil {
		t.Fatalf("unclaimed hidden name refused: %v", err)
	}
	if _, ok := graph.Unit("go:example.test/m/.github/helper"); !ok {
		t.Fatalf("hidden helper not indexed: %v", graph.UnitIDs())
	}
	claimed := fstest.MapFS{".metadata/name\xff.go": {Data: []byte("package metadata\n")}}
	for name, file := range base {
		claimed[name] = file
	}
	if _, err := affected.BuildFS(claimed, golang.New()); !errors.Is(err, affected.ErrWalkUnrepresentable) {
		t.Fatalf("claimed unrepresentable Go source: error %v, want ErrWalkUnrepresentable", err)
	}
}
