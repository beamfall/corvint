// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"reflect"
	"testing"
)

// AFP-V0-021: repeated and concurrent plans share the immutable token index,
// including through copies carrying different selection inputs.
func TestPathTokensReuseAcrossPlans(t *testing.T) {
	language := chain()
	language.units[1].PathTokens = []string{"../fixtures/case.txt", "assets/logo.json"}
	language.units[3].PathTokens = []string{"assets/logo.json"}
	graph, err := Build(context.Background(), t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	want := Select(graph, []string{"assets/logo.json"})
	tokens := graph.pathTokens()
	for _, copy := range []*Graph{graph, graph.WithReadScopeBase(map[string][]string{}), graph.WithSparseCheckout()} {
		if got := copy.pathTokens(); len(got) == 0 || &got[0] != &tokens[0] {
			t.Fatal("selection input copy rebuilt the token index")
		}
		if allocations := testing.AllocsPerRun(10, func() { _ = copy.pathTokens() }); allocations != 0 {
			t.Fatalf("repeated token lookup allocated %v times", allocations)
		}
	}
	for range 4 {
		t.Run("concurrent plan", func(t *testing.T) {
			t.Parallel()
			if got := Select(graph, []string{"assets/logo.json"}); !reflect.DeepEqual(got, want) {
				t.Fatal("repeated plan changed")
			}
		})
	}
	// A separately observed graph must not reuse another graph's token index.
	language.units[1].PathTokens = []string{"other/data.json"}
	language.units[3].PathTokens = nil
	other, err := Build(context.Background(), t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	if got := Select(other, []string{"assets/logo.json"}); len(got.Selected) != 0 {
		t.Fatalf("new graph retained old readers: %v", got.Selected)
	}
}
