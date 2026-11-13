package contextindex

import (
	"reflect"
	"testing"
)

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
