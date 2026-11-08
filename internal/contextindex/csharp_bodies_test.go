// SPDX-License-Identifier: Apache-2.0

package contextindex

import (
	"reflect"
	"testing"
)

func TestRSPV0005CSharpDialectBodyInteriors(t *testing.T) {
	text := "namespace Demo;\npublic class A\n{\n    public string F()\n    {\n        return \"{ not a brace }\"; // }\n    }\n    public int G() {\n        return 2;\n    }\n}\n"
	got, ok := CSharpBodies([]byte(text))
	want := []CSharpBody{{Name: "F", Start: 6, End: 7}, {Name: "G", Start: 9, End: 10}}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v %v", got, ok)
	}
}
func TestRSPV0005CSharpUnsupportedBodiesIncomplete(t *testing.T) {
	for _, text := range []string{
		"public class A {\npublic int F() => 1;\n}\n",
		"public class A {\npublic int F() {\nif (true) { return 1; }\n}\n}\n",
		"public class A {\npublic int F() {\nreturn 1;\n",
		"public class A {\n#if X\npublic int F() {\nreturn 1;\n}\n#endif\n}\n",
		"public class A {\npublic int F() {\n/* unterminated\n}\n}\n",
		"public class A {\npublic int F() {\nreturn \"unterminated\n}\n}\n",
	} {
		if bodies, ok := CSharpBodies([]byte(text)); ok {
			t.Fatalf("accepted %q: %+v", text, bodies)
		}
	}
}
