package contextindex

import (
	"context"
	"reflect"
	"testing"
)

// evalDependencyRepository is a Git repository whose strongest declaration
// calls an imported helper, beside an unrelated declaration that shares the
// task's vocabulary but nothing else.
func evalDependencyRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "lib/root.js", "import {optionValue} from \"./option.js\";\n\nexport const resolveOptionValue = (input) => optionValue(input);\n")
	writeTestFile(t, root, "lib/option.js", "export const optionValue = (input) => input;\n")
	writeTestFile(t, root, "lib/zother.js", "export const resolveOptionDefault = (input) => input;\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "option resolution")
	return root
}

// evalAnchorRepository is a Git repository where "descriptor" is the rarer
// term among the declarations the ranker admits and the commoner term across
// the whole symbol universe, because four test-only declarations carry it.
func evalAnchorRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "lib/mode/descriptor.js", "export const resolveEntry = (input) => input;\n")
	writeTestFile(t, root, "lib/mode/registry.js", "export const resolveRecord = (input) => input;\n")
	writeTestFile(t, root, "lib/mode/store.js", "export const resolveTable = (input) => input;\n")
	writeTestFile(t, root, "lib/descriptor/lookup.js", "export const resolveColumn = (input) => input;\n")
	for _, name := range []string{"one", "two", "three"} {
		writeTestFile(t, root, "test/descriptor-"+name+".test.js", "export const fixture = 1;\n")
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "descriptor validation")
	return root
}

func keysOf(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	return result
}

// evalNamedRootRepository is a Git repository whose strongest declaration,
// rootName, calls an imported helper beside two same-vocabulary declarations
// it does not call.
func evalNamedRootRepository(t *testing.T, rootName string) *Index {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "lib/terminate/kill.js", "import {killSignal} from \"./signal.js\";\n\nexport const "+rootName+" = (input) => killSignal(input);\n")
	writeTestFile(t, root, "lib/terminate/signal.js", "export const killSignal = (input) => input;\n")
	writeTestFile(t, root, "lib/terminate/timeout.js", "export const killTimeout = (input) => input;\nexport const killOnTimeout = (input) => input; // terminate\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "process termination")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

// TestWebImportBindingsReadsEveryStatementOnALine: GPK-V0-087 (V1-2128). Two
// static named imports on one line, joined by `;`, both bind; the first-only
// oracle reading is known-divergent DR-0044.
func TestWebImportBindingsReadsEveryStatementOnALine(t *testing.T) {
	got := webImportBindings("import { a } from './a'; import { b as c } from './b';\nimport { d } from './d'\n")
	want := []webBinding{{"a", "./a", "a", 1}, {"c", "./b", "b", 1}, {"d", "./d", "d", 2}}
	if len(got) != len(want) {
		t.Fatalf("bindings = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("bindings = %+v, want %+v", got, want)
		}
	}
}

// TestWebImportBindingsIgnoresSemicolonsOutsideCode: GPK-V0-087 (V1-2128). A
// `;` inside a string, template literal or comment opens no statement, so a
// fake import there neither rebinds a real one nor swallows the real statement
// after it; and a second statement never rebinds a local already bound.
func TestWebImportBindingsIgnoresSemicolonsOutsideCode(t *testing.T) {
	for _, test := range []struct {
		name, text string
		want       []webBinding
	}{
		{"string after a real import", "import { a } from './a'; const note = \"; import { a } from './b'\";\n",
			[]webBinding{{"a", "./a", "a", 1}}},
		{"partial fake import before a real one", "const s = \"; import { z }\"; import { b } from './b';\n",
			[]webBinding{{"b", "./b", "b", 1}}},
		{"template literal", "run(); const t = `; import { w } from './w'`;\n", []webBinding{}},
		{"line comment", "run(); // ; import { y } from './y'\n", []webBinding{}},
		{"block comment", "run(); /* ; import { y } from './y' */\n", []webBinding{}},
		{"second statement never rebinds", "import { a } from './a'; import { a } from './b';\n",
			[]webBinding{{"a", "./a", "a", 1}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := webImportBindings(test.text); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("bindings = %+v, want %+v", got, test.want)
			}
		})
	}
}
