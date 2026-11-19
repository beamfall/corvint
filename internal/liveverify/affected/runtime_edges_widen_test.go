// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func runtimeWidenFixture(t *testing.T, declaration string) (string, fake) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, RuntimeEdgesPath), []byte(declaration), 0600); err != nil {
		t.Fatal(err)
	}
	return root, fake{name: "g", units: []Unit{
		{ID: "g:holder", Sources: []string{"holder.g"}, Tests: []string{"holder_test.g"}},
		{ID: "g:testuser", Sources: []string{"testuser.g"}, Tests: []string{"testuser_test.g"}, TestImports: []string{"g:holder"}},
		{ID: "g:downstream", Sources: []string{"downstream.g"}, Tests: []string{"downstream_test.g"}, Imports: []string{"g:testuser"}},
		{ID: "g:runner", Tests: []string{"runner_test.g"}, Execs: []string{"g:downstream"}, TestOnlyExecs: []string{"g:downstream"}},
		{ID: "g:reader", Sources: []string{"reader.g"}, Tests: []string{"reader_test.g"}, PathTokens: []string{"holder.g"}},
		{ID: "g:readeruser", Tests: []string{"readeruser_test.g"}, Imports: []string{"g:reader"}},
		{ID: "g:unrelated", Sources: []string{"unrelated.g"}, Tests: []string{"unrelated_test.g"}},
	}}
}

const unresolvedRuntimeDeclaration = `{"profile":"corvint-runtime-edges/0","edges":[{"tests":"holder_test.g","runs":"new:target","reason":"the holder drives a newly analyzable tool"}]}`

func TestRuntimeEdgeWidening_CINV1940(t *testing.T) {
	root, language := runtimeWidenFixture(t, unresolvedRuntimeDeclaration)
	if _, err := Build(context.Background(), root, language); !IsUnknownRuntimeUnit(err) {
		t.Fatalf("strict compatibility refusal = %v", err)
	}
	graph, err := BuildWithRuntimeEdgeWidening(context.Background(), root, language)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"g:downstream", "g:holder", "g:reader", "g:readeruser", "g:runner", "g:testuser"}
	// CIN-V1-941: unknown target, changed holder source and changed holder test
	// all retain the complete area, including users past terminal test/exec edges.
	for _, dirty := range []string{"new/target.x", "holder.g", "holder_test.g"} {
		t.Run(dirty, func(t *testing.T) {
			plan := Select(graph.WithChangeStatus(map[string]ChangeStatus{dirty: ChangeModified}), []string{dirty})
			got := slices.Sorted(maps.Keys(witnesses(plan)))
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("selected=%v want=%v", got, want)
			}
			if plan.Scope != ScopeUnknown {
				t.Fatalf("scope=%s", plan.Scope)
			}
			found := false
			for _, unknown := range plan.Unknown {
				if unknown.Reason == UnknownLanguageFrontier && strings.Contains(unknown.Detail, `tests="holder_test.g" runs="new:target": runs names no unit`) {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing degradation: %+v", plan.Unknown)
			}
			for _, selected := range plan.Selected {
				if len(selected.Tests) != 1 || selected.Witness.Kind != WitnessUnboundedReader || selected.Witness.Cause == nil {
					t.Fatalf("not whole conservative selection: %+v", selected)
				}
			}
		})
	}
	// The no-edge control selects only the actual changed holder test.
	plainRoot, plainLanguage := runtimeWidenFixture(t, `{"profile":"corvint-runtime-edges/0","edges":[]}`)
	plain, err := Build(context.Background(), plainRoot, plainLanguage)
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Keys(witnesses(Select(plain, []string{"unrelated.g"})))); !reflect.DeepEqual(got, []string{"g:unrelated"}) {
		t.Fatalf("control=%v", got)
	}
	partial, err := BuildPartial(context.Background(), root, language)
	if err != nil || graph.Digest() == partial.Digest() {
		t.Fatalf("partial and widened graph identities must differ: %v", err)
	}
	t.Logf("strict fallback: 7 whole units; widened: %d whole units; unrelated excluded: 1", len(want))
	again, err := BuildWithRuntimeEdgeWidening(context.Background(), root, language)
	if err != nil || graph.Digest() != again.Digest() || graph.Digest() == plain.Digest() {
		t.Fatalf("digest binding: %v", err)
	}
}

func TestRuntimeEdgeWideningRefusals_CINV1940(t *testing.T) {
	for name, declaration := range map[string]string{
		"no holder":            strings.ReplaceAll(unresolvedRuntimeDeclaration, "holder_test.g", "absent_test.g"),
		"unknown field":        strings.Replace(unresolvedRuntimeDeclaration, `"reason":`, `"extra":true,"reason":`, 1),
		"blank reason":         strings.ReplaceAll(unresolvedRuntimeDeclaration, "the holder drives a newly analyzable tool", ""),
		"trailing JSON":        unresolvedRuntimeDeclaration + `{}`,
		"unsorted":             `{"profile":"corvint-runtime-edges/0","edges":[{"tests":"holder_test.g","runs":"new:target","reason":"r"},{"tests":"a","runs":"g:holder","reason":"r"}]}`,
		"later invalid holder": `{"profile":"corvint-runtime-edges/0","edges":[{"tests":"holder_test.g","runs":"new:target","reason":"r"},{"tests":"missing.g","runs":"g:holder","reason":"r"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			root, language := runtimeWidenFixture(t, declaration)
			if _, err := BuildWithRuntimeEdgeWidening(context.Background(), root, language); err == nil {
				t.Fatal("malformed declaration admitted")
			}
		})
	}
}

func TestRuntimeEdgeWideningDeclaredOwners_CINV1941(t *testing.T) {
	root, language := runtimeWidenFixture(t, unresolvedRuntimeDeclaration)
	language.units = append(language.units,
		Unit{ID: "g:integration", Sources: []string{"integration.g"},
			Tests: []string{"integration_test.g"}},
		Unit{ID: "g:owneruser", Tests: []string{"owneruser_test.g"},
			Imports: []string{"g:integration"}},
		Unit{ID: "g:secondowner", Tests: []string{"secondowner_test.g"}},
	)
	const owners = `{"profile":"corvint-test-selection/0","owners":[
		{"paths":"holder.g","tests":["integration_test.g"]},
		{"paths":"integration.g","tests":["secondowner_test.g"]}
	]}`
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, DeclaredSelectionPath), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(owners)
	graph, err := BuildWithRuntimeEdgeWidening(t.Context(), root, language)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"g:downstream", "g:holder", "g:integration", "g:owneruser", "g:reader",
		"g:readeruser", "g:runner", "g:secondowner", "g:testuser"}
	// Neither dirty path matches an owner pattern. Unknown source forms must
	// retain the same closure, including owners discovered in later rounds.
	for _, dirty := range []string{"new/target.x", "future/input.unrecognized"} {
		plan := Select(graph, []string{dirty})
		if got := slices.Sorted(maps.Keys(witnesses(plan))); !slices.Equal(got, want) {
			t.Fatalf("%s: closure = %v, want %v", dirty, got, want)
		}
		if plan.Scope != ScopeUnknown {
			t.Fatalf("%s: scope = %s", dirty, plan.Scope)
		}
		for _, selected := range plan.Selected {
			if selected.Witness.Kind != WitnessUnboundedReader || selected.Witness.Cause == nil {
				t.Fatalf("owner closure lost conservative witness: %+v", selected)
			}
		}
	}
	// The declaration profile is closed: a future owner form must refuse,
	// never silently omit a relation from a supposedly complete closure.
	write(strings.Replace(owners, `"paths":"holder.g"`,
		`"paths":"holder.g","futureRelation":true`, 1))
	if _, err := BuildWithRuntimeEdgeWidening(t.Context(), root, language); err == nil {
		t.Fatal("unknown owner form admitted")
	}
}
