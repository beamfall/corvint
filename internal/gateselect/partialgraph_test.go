// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"context"
	"errors"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// countPartialBuilds wraps buildPartialGraph for the rest of the test and
// returns the number of builds it has run.
func countPartialBuilds(t *testing.T) *int {
	t.Helper()
	builds := new(int)
	previous := buildPartialGraph
	buildPartialGraph = func(ctx context.Context, root string) (*affected.Graph, error) {
		*builds++
		return previous(ctx, root)
	}
	t.Cleanup(func() { buildPartialGraph = previous })
	return builds
}

// TestPartialGraphObservedOncePerIndex_AFPV011134 pins the memo's rules: one
// successful build per index and root, a failed build kept by no one and
// retried, another root built fresh without replacing the kept graph, and a
// nil memo (no index) building every time.
func TestPartialGraphObservedOncePerIndex_AFPV011134(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"go.mod":       "module " + fixtureModule + "\n\ngo 1.27\n",
		"a/a.go":       "package a\n",
		"a/a_test.go":  "package a\n",
		"other/go.mod": "module example.test/other\n\ngo 1.27\n",
		"other/b.go":   "package other\n",
	})
	builds := countPartialBuilds(t)
	fail := true
	previous := buildPartialGraph
	buildPartialGraph = func(ctx context.Context, root string) (*affected.Graph, error) {
		if fail {
			fail = false
			*builds++
			return nil, errors.New("injected")
		}
		return previous(ctx, root)
	}
	t.Cleanup(func() { buildPartialGraph = previous })
	ctx := context.Background()
	var memo partialGraph
	if _, err := memo.observe(ctx, root); err == nil {
		t.Fatal("injected failure not returned")
	}
	first, err := memo.observe(ctx, root)
	if err != nil || first == nil {
		t.Fatalf("retry after failure: %v", err)
	}
	again, err := memo.observe(ctx, root)
	if err != nil || again != first {
		t.Fatalf("second observation rebuilt or failed: %v", err)
	}
	if *builds != 2 || memo.built != 1 {
		t.Fatalf("builds %d (kept %d), want 2 attempts and 1 kept", *builds, memo.built)
	}
	other := root + "/other"
	fresh, err := memo.observe(ctx, other)
	if err != nil || fresh == first {
		t.Fatalf("another root reused the kept graph: %v", err)
	}
	if kept, _ := memo.observe(ctx, root); kept != first || *builds != 3 {
		t.Fatalf("another root replaced the kept graph or the root rebuilt: builds %d", *builds)
	}
	var none *partialGraph
	if _, err := none.observe(ctx, root); err != nil {
		t.Fatal(err)
	}
	if _, err := none.observe(ctx, root); err != nil {
		t.Fatal(err)
	}
	if *builds != 5 {
		t.Fatalf("nil memo builds %d, want 5 in all", *builds)
	}
	if partialGraphOf(nil) != nil {
		t.Fatal("nil index has a memo")
	}
}
