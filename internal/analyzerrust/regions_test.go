// SPDX-License-Identifier: Apache-2.0

package analyzerrust

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testevidence"
)

func TestRSPV0005RustDialectExecutableBodies(t *testing.T) {
	source := []byte("/* fn bogus() { /* nested */ } */\nimpl Thing {\n  pub fn outer<'a>(&'a self) -> bool {\n    let text = r###\"fn bogus() { }\"###;\n    fn first() { true; } fn second() { false; }\n    true\n  }\n}\n")
	bodies, err := ExecutableBodies(source)
	if err != nil || len(bodies) != 3 {
		t.Fatalf("%+v %v", bodies, err)
	}
	if bodies[0].Range.Start != 3 || bodies[0].Range.End != 7 || bodies[0].Parent != "" || bodies[1].Parent != bodies[0].ID || bodies[2].Parent != bodies[0].ID || bodies[1].ID == bodies[2].ID {
		t.Fatalf("%+v", bodies)
	}
	// Same-line siblings need exact byte columns, not line-only nesting.
	if bodies[1].Range.StartColumn >= bodies[1].Range.EndColumn || bodies[1].Range.EndColumn >= bodies[2].Range.StartColumn {
		t.Fatal(bodies)
	}
	for _, source := range []string{"", "struct S;", "trait T { fn required(&self); }", "fn empty() {}"} {
		bodies, err := ExecutableBodies([]byte(source))
		if err != nil || len(bodies) != 0 {
			t.Fatalf("%q: %+v %v", source, bodies, err)
		}
	}
	// Reuse the record validator for parent containment and valid ranges.
	r := testevidence.New("repo", "test", strings.Repeat("a", 40))
	r.Regions = testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present, Items: []testevidence.RegionsFile{{Blob: testevidence.Blob{Path: testevidence.Path{Repository: "repo", Lexical: "lib.rs", Resolved: "lib.rs"}, OID: strings.Repeat("a", 40)}, Bodies: bodies}}}
	if err := r.ValidateComponents(); err != nil {
		t.Fatal(err)
	}
}

func TestRSPV0005RustDialectUnsupportedRegions(t *testing.T) {
	for _, source := range []string{
		"fn truncated() {", "fn text() { let s = r##\"unterminated; }", "fn text() { /* unterminated }",
		"#[test] fn test() { true; }", "fn expansion() { include_str!(\"data\"); }", "fn closure() { let f = || true; }",
		"fn generic<const N: usize>() { true; }", "fn nonascii() { let 名 = true; }", strings.Repeat(" ", MaxSourceBytes+1),
	} {
		if bodies, err := ExecutableBodies([]byte(source)); err == nil || len(bodies) != 0 {
			t.Fatalf("unsupported input emitted regions: %+v %v", bodies, err)
		}
	}
}

// RSP-V0-005: braces in return-type const arguments are not executable bodies.
func TestRSPV0005RustReturnTypeConstExpressionRefused(t *testing.T) {
	for _, source := range []string{
		"fn value() -> S<{\n    1 + 1\n}> {\n    S\n}\n",
		"fn value() -> Outer<Inner<{\n    2\n}>> {\n    value\n}\n",
		"fn value() -> S<fn() -> u8, {\n    2\n}> {\n    value\n}\n",
	} {
		bodies, err := ExecutableBodies([]byte(source))
		if err == nil || len(bodies) != 0 || !strings.Contains(err.Error(), "signature-unsupported") {
			t.Fatalf("signature expression emitted executable bodies: %+v %v", bodies, err)
		}
	}
}
