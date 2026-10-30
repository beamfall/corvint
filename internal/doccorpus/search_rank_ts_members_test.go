package doccorpus

import (
	"slices"
	"testing"
)

var tsMemberSources = map[string]string{
	"src/cart.ts":   "export class Cart {\n  syncWithPos(): void {}\n}\nexport function totalDue() { return 1; }\n",
	"src/readme.md": "# Cart\n\nTotals the order.\n",
}

// DCS-V1-571 (V1-2548, GitHub #1665): a method declared inside a TypeScript
// class body is a native declaration, whether the native provider owns the
// source or only a records provider claims it. Its subject and :declaration
// claim carry the method's line, and docs_search for the method name ranks it
// first.
func TestDCSV1571TypeScriptClassMembersAreDeclarations(t *testing.T) {
	t.Parallel()
	const subject = "native:symbol:src/cart.ts:2:syncWithPos"
	for name, owned := range map[string][]string{"native-owned": {"src/readme.md"}, "records-owned": {"src/cart.ts"}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, m := ownedSourceCorpus(t, tsMemberSources, owned, false)
			a := buildOwned(t, root, m)
			var ids []string
			for _, s := range subjectsWithPrefix(a, "native:symbol:src/cart.ts:") {
				ids = append(ids, s.ID)
			}
			want := []string{"native:symbol:src/cart.ts:1:Cart", subject, "native:symbol:src/cart.ts:4:totalDue"}
			if !slices.Equal(ids, want) {
				t.Fatalf("declarations = %v, want %v", ids, want)
			}
			s := subjectsWithPrefix(a, subject)[0]
			if s.Kind != "symbol" || s.Name != "syncWithPos" || len(s.Evidence.Anchors) != 1 || s.Evidence.Anchors[0].Symbol != "syncWithPos" || s.Evidence.Anchors[0].Start != 2 {
				t.Fatalf("member declaration subject: %+v", s)
			}
			var claim *Claim
			for i := range a.Claims {
				if a.Claims[i].ID == subject+":declaration" {
					claim = &a.Claims[i]
				}
			}
			if claim == nil || claim.Text != "  syncWithPos(): void {}" {
				t.Fatalf("member declaration claim: %+v", claim)
			}
			if order := searchOrders(t, a, Request{Operation: "search", Query: "syncWithPos"}); len(order) == 0 || order[0] != subject {
				t.Fatalf("the member declaration should rank first: %v", order)
			}
		})
	}
}

// DCS-V1-571: a getter and setter of one name on one line are one declaration,
// so the build never holds two subjects with one ID.
func TestDCSV1571AccessorPairOnOneLineIsOneDeclaration(t *testing.T) {
	t.Parallel()
	got := withClassMembers(nil, "src/c.ts", "class C { get v() { return 1 } set v(x) {} }\n")
	if len(got) != 1 || got[0].Name != "v" || got[0].Line != 1 {
		t.Fatalf("declarations = %+v, want one v at line 1", got)
	}
}
