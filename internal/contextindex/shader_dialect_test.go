// SPDX-License-Identifier: Apache-2.0

package contextindex

import "testing"

// RSP-V0-005: the core resolves the registered shader dialect by suffix.
func TestRSPV0005ShaderDialectRoute(t *testing.T) {
	for _, name := range []string{"a.metal", "a.hlsl", "a.glsl"} {
		if SymbolRegionLanguage(name) != "shader" {
			t.Fatalf("missing dialect for %s", name)
		}
		if _, ok := NormalizedCodeLines(name, []byte("void main() {}")); !ok {
			t.Fatalf("missing normalizer for %s", name)
		}
		if _, ok := NormalizedCodeLines(name, []byte("void main() {")); ok {
			t.Fatalf("partial normalizer for %s", name)
		}
	}
}

// RSP-V0-005: unknown shader extraction never reports a complete empty walk.
func TestRSPV0005ShaderSymbolRoute(t *testing.T) {
	for _, name := range []string{"a.metal", "a.hlsl", "a.glsl"} {
		source := Source{Path: name}
		symbols, notes, handled := languageSymbols(source, "void main()\n{ }\n")
		if !handled || len(notes) != 0 || len(symbols) != 1 || symbols[0].Name != "main" || symbols[0].Line != 1 {
			t.Fatalf("%s %+v %+v", name, symbols, notes)
		}
		symbols, notes, handled = languageSymbols(source, "#if FEATURE\nvoid main() {}\n#endif")
		if !handled || len(symbols) != 0 || len(notes) != 1 || notes[0].Reason != "shader-directive-unsupported" {
			t.Fatalf("partial shader: %+v %+v", symbols, notes)
		}
	}
}
