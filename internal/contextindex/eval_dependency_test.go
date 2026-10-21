package contextindex

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// evalConfidentPaths returns the confident selection for one task: the result
// selectors in order, and each one's relation block where it carries one.
func evalConfidentSelection(t *testing.T, root, text string, limit int) []evalSymbolCandidate {
	t.Helper()
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	queryText := pythonLower(text)
	intent := evalInferIntent(queryText)
	queryTerms := evalRelevanceTerms(terms(queryText), intent)
	ranked, frequency := evalRankSymbols(index, queryText, queryTerms, intent)
	return evalConfidentSymbols(index, ranked, frequency, queryText, queryTerms, limit)
}

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

// TestEvalConfidentSymbolsPromotesTheRootDeclarationsDependencies pins the
// dependency-link stage: where the task names the strongest declaration, the
// declarations it calls take the next ranks -- carrying the relation and the
// import and call sites that bind them -- and the same-vocabulary neighbour
// that the frontier alone would have promoted is dropped.
func TestEvalConfidentSymbolsPromotesTheRootDeclarationsDependencies(t *testing.T) {
	confident := evalConfidentSelection(t, evalDependencyRepository(t), "resolve option value", 6)
	selectors := make([]string, 0, len(confident))
	for _, candidate := range confident {
		selectors = append(selectors, candidate.symbol.symbol.Path+":"+candidate.symbol.symbol.Name)
	}
	want := []string{"lib/root.js:resolveOptionValue", "lib/option.js:optionValue"}
	if len(selectors) != len(want) || selectors[0] != want[0] || selectors[1] != want[1] {
		t.Fatalf("confident = %v, want exactly %v", selectors, want)
	}
	relation, carried := confident[1].result["relation"].(map[string]any)
	if !carried {
		t.Fatalf("linked result carries no relation: %v", confident[1].result)
	}
	if relation["kind"] != "calls-imported" || relation["depth"] != 1 ||
		relation["from"] != "symbol:lib/root.js:resolveOptionValue" {
		t.Fatalf("relation = %v, want a depth-1 calls-imported edge from the root", relation)
	}
	authorities := make([]string, 0, 3)
	for _, item := range anySlice(confident[1].result["evidence"]) {
		authorities = append(authorities, stringValue(item.(map[string]any)["authority"]))
	}
	if len(authorities) != 3 || authorities[1] != "syntax-import" || authorities[2] != "syntax-call" {
		t.Fatalf("evidence authorities = %v, want the declaration plus its import and call sites", authorities)
	}
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

// TestEvalConfidentSymbolsAnchorsOnTheIndexWideTermFrequency pins the table the
// anchor term is chosen from: the frequency of every indexed symbol, test
// declarations included, not the frequency across the admitted candidates
// alone. The two tables disagree here, and only the index-wide one keeps the
// frontier on the path the task's rarest word actually names.
func TestEvalConfidentSymbolsAnchorsOnTheIndexWideTermFrequency(t *testing.T) {
	t.Parallel()
	confident := evalConfidentSelection(t, evalAnchorRepository(t), "resolve mode descriptor", 6)
	paths := make(map[string]bool, len(confident))
	for _, candidate := range confident {
		paths[candidate.symbol.symbol.Path] = true
	}
	if !paths["lib/mode/registry.js"] || !paths["lib/mode/store.js"] {
		t.Fatalf("frontier paths = %v, want both lib/mode paths: \"mode\" is the rarer term index-wide", keysOf(paths))
	}
	if paths["lib/descriptor/lookup.js"] {
		t.Fatalf("frontier paths = %v, want no lib/descriptor path: it is anchored on the commoner term", keysOf(paths))
	}
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

// TestEvalLinkedSymbolsDropsTheFrontierOnlyForAMultiPartNamedRoot pins the
// GPK-V0-047 amendment (decision 0157) on evalLinkedSymbols itself, handed the
// whole ranked list as its frontier: the later fully-named fill would re-add a
// dropped multi-part neighbour and hide the rule. Both tasks contain the root's
// compact name and the root links to a helper. A one-part name such as `kill`
// is an ordinary task word, so the frontier is kept; a multi-part identifier is
// a name, so everything past the promoted link is dropped.
func TestEvalLinkedSymbolsDropsTheFrontierOnlyForAMultiPartNamedRoot(t *testing.T) {
	for _, tc := range []struct {
		name, rootName, task string
		want                 []string
	}{
		{"one-part root keeps the frontier", "kill", "terminate and kill", []string{"lib/terminate/kill.js:kill", "lib/terminate/signal.js:killSignal", "lib/terminate/timeout.js:killTimeout", "lib/terminate/timeout.js:killOnTimeout"}},
		{"multi-part root drops the frontier", "killProcess", "terminate and kill process", []string{"lib/terminate/kill.js:killProcess", "lib/terminate/signal.js:killSignal"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index := evalNamedRootRepository(t, tc.rootName)
			queryText := pythonLower(tc.task)
			intent := evalInferIntent(queryText)
			ranked, _ := evalRankSymbols(index, queryText, evalRelevanceTerms(terms(queryText), intent), intent)
			selectors := make([]string, 0, len(ranked))
			for _, candidate := range evalLinkedSymbols(index, ranked, ranked, queryText, 6) {
				selectors = append(selectors, candidate.symbol.symbol.Path+":"+candidate.symbol.symbol.Name)
			}
			if strings.Join(selectors, " ") != strings.Join(tc.want, " ") {
				t.Fatalf("linked selection = %v, want %v", selectors, tc.want)
			}
		})
	}
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
