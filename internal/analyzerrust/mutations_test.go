// SPDX-License-Identifier: Apache-2.0

package analyzerrust

import (
	"strings"
	"testing"
)

func TestRSPV0005RustBooleanMutationOperator(t *testing.T) {
	source := []byte("const GLOBAL: bool = true; fn f() -> bool { let text = r#\"true\"#; /* false */ let r#true = 1; true }\n")
	mutations, err := BooleanMutations(source)
	if err != nil || len(mutations) != 1 {
		t.Fatalf("%+v %v", mutations, err)
	}
	m := mutations[0]
	if m.Offset != strings.LastIndex(string(source), "true") || m.Before != "true" || m.After != "false" || m.Body == "" {
		t.Fatal(m)
	}
	copy := append([]byte{}, source[:m.Offset]...)
	copy = append(copy, []byte(m.After)...)
	copy = append(copy, source[m.Offset+len(m.Before):]...)
	if bodies, err := ExecutableBodies(copy); err != nil || len(bodies) != 1 {
		t.Fatalf("operator broke dialect: %+v %v", bodies, err)
	}
}

func TestRSPV0005RustMutationOperatorRefusesUnknown(t *testing.T) {
	for _, source := range []string{"fn f() { true;", "fn f() { include!(\"input\"); true }", "#[test] fn f() { false }"} {
		if mutations, err := BooleanMutations([]byte(source)); err == nil || len(mutations) != 0 {
			t.Fatalf("unknown source emitted candidates: %+v %v", mutations, err)
		}
	}
	mutations, err := BooleanMutations([]byte("const GLOBAL: bool = false; fn f() {}"))
	if err != nil || len(mutations) != 0 {
		t.Fatalf("declaration mutated as a body: %+v %v", mutations, err)
	}
	if mutations, err := BooleanMutations([]byte("fn f() {" + strings.Repeat("true;", MaxFacts+1) + "}")); err == nil || len(mutations) != 0 {
		t.Fatal("capped operator emitted a partial candidate set")
	}
}
