package pythongrammar

import "testing"

// TestTypeSoftKeywordOnlyOpensAnAlias pins the reported false positive together
// with the siblings that already passed, so a future routing change cannot
// regress one without the others. `type` is a soft keyword: CPython commits to
// `type_alias: "type" NAME [type_params] '=' expression` only on that shape, and
// routing every statement that merely opens with `type` into the alias rule cost
// three workspace test modules every symbol they define. Every case here is
// accepted by CPython 3.12 ast.parse.
func TestTypeSoftKeywordOnlyOpensAnAlias(t *testing.T) {
	for _, source := range []string{
		"type(self).calls += 1\n",   // the reported false positive
		"type(self).reads += 1\n",   // the same shape, second corpus file
		"type(self).calls = 1\n",    // plain assignment through the same target
		"type(self).calls\n",        // bare attribute of a call
		"type(x)\n",                 // the builtin called as a statement
		"type(a) == type(b)\n",      // two calls in one comparison
		"type.mro(x)\n",             // attribute of the builtin itself
		"type = 5\n",                // rebinding the builtin
		"obj.attr().field += 1\n",   // passing sibling: call is not the first element
		"self.calls += 1\n",         // passing sibling: no call in the target
		"type Alias = int\n",        // the real alias must still parse
		"type Alias[T] = list[T]\n", // the real alias with type parameters
	} {
		if reason := ParsePython312Subset("probe.py", []byte(source), discardFacts{}); reason != "" {
			t.Errorf("ParsePython312Subset(%q) = %q, want accepted", source, reason)
		}
	}
}

// TestTypeAliasStillRejectsAKeywordName guards the other side: narrowing the
// route must not stop the alias rule from judging the aliases it does own.
func TestTypeAliasStillRejectsAKeywordName(t *testing.T) {
	if reason := ParsePython312Subset("probe.py", []byte("type if = 3\n"), discardFacts{}); reason == "" {
		t.Error("ParsePython312Subset(\"type if = 3\") = accepted, want rejected")
	}
}
