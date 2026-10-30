package pythongrammar

import "testing"

// TestLambdaTakesTheDefParameterList pins the reported false positive together
// with the siblings that already passed. A lambda's parameter list is CPython's
// `lambda_params`, which differs from a def's `params` only in carrying no
// annotation; admitting bare names alone rejected every keyword-argument
// callback in six workspace test modules. Only the call-argument path reaches
// this rule, so the siblings outside it must keep parsing too. Every case here
// is accepted by CPython 3.12 ast.parse.
func TestLambdaTakesTheDefParameterList(t *testing.T) {
	for _, source := range []string{
		"f(lambda x=1: x)\n",                  // the reported false positive
		"f(lambda *_args: invoked.touch())\n", // the shape the corpus actually carries
		"f(lambda x: x)\n",                    // passing sibling: one bare name
		"f(lambda: 1)\n",                      // passing sibling: no parameters
		"g = lambda x=1: x\n",                 // passing sibling: assignment, not an argument
		"[lambda x=1: x]\n",                   // passing sibling: list display
		"{lambda x=1: x}\n",                   // passing sibling: set display
		"f(lambda **k: 1)\n",                  // keyword-argument collector
		"f(lambda x, y=2: x)\n",               // a default after a plain parameter
		"f(lambda x, /, y, *, z: x)\n",        // positional-only and keyword-only markers
		"f(lambda x=1, *a, y=2, **k: x)\n",    // every parameter kind at once
		"f(sorted(v, key=lambda p=1: p))\n",   // nested one call deeper
	} {
		if reason := ParsePython312Subset("probe.py", []byte(source), discardFacts{}); reason != "" {
			t.Errorf("ParsePython312Subset(%q) = %q, want accepted", source, reason)
		}
	}
}

// TestLambdaParameterListStaysClosed guards the other side of the widening: the
// def parameter rule judges a lambda's list, so the orderings CPython refuses
// must still be refused here.
func TestLambdaParameterListStaysClosed(t *testing.T) {
	for _, source := range []string{
		"f(lambda x=: x)\n",      // default with no value
		"f(lambda *: x)\n",       // bare star with no keyword-only parameter
		"f(lambda **: x)\n",      // collector with no name
		"f(lambda 1: x)\n",       // a number is not a parameter
		"f(lambda x y: x)\n",     // two names, no comma
		"f(lambda *a, *b: x)\n",  // two variadic collectors
		"f(lambda **a, b: x)\n",  // a parameter after the keyword collector
		"f(lambda x=1, y: x)\n",  // a plain parameter after a defaulted one
		"f(lambda /, x: x)\n",    // positional-only marker with nothing before it
		"f(lambda x, /, /: x)\n", // two positional-only markers
	} {
		if reason := ParsePython312Subset("probe.py", []byte(source), discardFacts{}); reason == "" {
			t.Errorf("ParsePython312Subset(%q) = accepted, want rejected", source)
		}
	}
}
