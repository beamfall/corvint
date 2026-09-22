package pythonsyntax

import "testing"

// TestValidatedRouteAcceptsTheReportedFalsePositives pins both false-positive
// classes on the route the index actually walks. The grammar layer owns most of
// each class, but two shapes were refused only here: `type = 5` reached the
// alias validator because it opens with the soft keyword, and a lambda's
// keyword-only `*` marker wore the shape the dangling-star rule refuses. A file
// refused on this route loses EVERY symbol and import it defines, not merely the
// construct that failed. Every case here is accepted by CPython 3.12 ast.parse.
func TestValidatedRouteAcceptsTheReportedFalsePositives(t *testing.T) {
	for _, source := range []string{
		"type(self).calls += 1\n",                    // class A, as the corpus carries it
		"type = 5\n",                                 // class A, refused by the alias validator alone
		"type Alias = int\n",                         // the real alias must still parse
		"f(lambda x=1: x)\n",                         // class B, as reported
		"f(lambda *_args: invoked.touch())\n",        // class B, as the corpus carries it
		"f(lambda a, *, b: a)\n",                     // class B, refused by the dangling-star rule alone
		"@deco(lambda x=1: x)\ndef g():\n    pass\n", // the same list in a decorator argument
	} {
		if refusal := ParseValidated("probe.py", []byte(source), discardFacts{}); refusal != "" {
			t.Errorf("ParseValidated(%q) = %q, want accepted", source, refusal)
		}
	}
}

// TestValidatedRouteStillRefusesDanglingStars guards the widening that let a
// lambda's `*` marker through: a star that really is a dangling expression must
// still be refused.
func TestValidatedRouteStillRefusesDanglingStars(t *testing.T) {
	for _, source := range []string{
		"f(*)\n",
		"f(x, *, y)\n",
		"f(lambda x: *)\n",
		"type X = \n",
		"type X\n",
	} {
		if refusal := ParseValidated("probe.py", []byte(source), discardFacts{}); refusal == "" {
			t.Errorf("ParseValidated(%q) = accepted, want refused", source)
		}
	}
}
