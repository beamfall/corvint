// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

// Package conformance holds checks shared by the standalone conformance
// runners, which are separate main packages and cannot import each other.
package conformance

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"
)

// decision0357States is the closed hostile-state vocabulary, in the order
// decision 0357 fixes for every OCM and frontier manifest.
var decision0357States = []string{"stable", "relocated", "stale", "ambiguous", "deleted", "unknown"}

// The frontier and OCM state validators each keep a copy of the vocabulary;
// both copies must equal the decision's list so the validators accept the
// same state names (V1-0134).
func TestStateValidatorsShareOneVocabulary(t *testing.T) {
	for _, source := range []string{"frontier-v0/manifest.go", "ocm-v0/manifest.go"} {
		if got := hostileStates(t, source); !slices.Equal(got, decision0357States) {
			t.Errorf("%s hostileStates = %q, want %q", source, got, decision0357States)
		}
	}
}

// hostileStates reads the string literals of the hostileStates variable.
func hostileStates(t *testing.T, source string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), source, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var literal *ast.CompositeLit
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if ok && len(spec.Names) == 1 && spec.Names[0].Name == "hostileStates" && len(spec.Values) == 1 {
			literal, _ = spec.Values[0].(*ast.CompositeLit)
		}
		return literal == nil
	})
	if literal == nil {
		t.Fatalf("%s: no hostileStates composite literal", source)
	}
	states := []string{}
	for _, element := range literal.Elts {
		value, err := strconv.Unquote(element.(*ast.BasicLit).Value)
		if err != nil {
			t.Fatal(err)
		}
		states = append(states, value)
	}
	return states
}
