// SPDX-License-Identifier: Apache-2.0

package shaderdialect

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/langplugin/dialect"
)

// RSP-V0-005: native structural evidence for all three shader dialects.
func TestRSPV0005FunctionsAndKernels(t *testing.T) {
	for name, text := range map[string]string{
		"metal": "#include \"common.h\"\nkernel void shade(uint i [[thread_position_in_grid]]) {\n if (i > 0) { i++; }\n}\n",
		"hlsl":  "[numthreads(8, 8, 1)]\nvoid shade(uint3 i : SV_DispatchThreadID) { if (i.x > 0) { return; } }\n",
		"glsl":  "#version 450\nfloat helper(float x) { return x * 2.0; }\nvoid main() { helper(1.0); }\n",
	} {
		t.Run(name, func(t *testing.T) {
			s := Parse(text)
			if s.Reason != "" || len(s.Bodies) == 0 {
				t.Fatalf("%+v", s)
			}
			if name == "metal" && (len(s.Includes) != 1 || s.Includes[0].Name != "common.h" || s.Bodies[0].Range.Start != 2 || s.Bodies[0].Range.End != 4 || s.Bodies[0].Range.EndColumn != 2) {
				t.Fatalf("%+v", s)
			}
			if name == "glsl" && len(s.Bodies) != 2 {
				t.Fatalf("%+v", s)
			}
			if x := Symbols(text, 100, 10); x.Stop != "" || len(x.Declarations) != len(s.Bodies) {
				t.Fatalf("%+v", x)
			}
		})
	}
}

// RSP-V0-005: no unsupported/malformed source becomes complete empty regions.
func TestRSPV0005MalformedAndUnsupported(t *testing.T) {
	for _, text := range []string{
		"main() {}", "if (x) {}", "void main() {", "void main() { ] }", "void main() {} /*", "void main() { \"x; }",
		"#include HEADER\nvoid main() {}", "#if ACTIVE\nvoid main() {}\n#endif", "#define F(x) x\nvoid main() {}",
		"#include \"a.h\" extra\n", "#include\n", "#include <a b.h>\n", "void main() {}\nmissing", "struct Data { float x; };",
		"void ma\\\nin() {}", "void main() {\x00}", string([]byte{255}), strings.Repeat(" ", MaxBytes+1),
	} {
		if s := Parse(text); s.Reason == "" || len(s.Bodies) != 0 {
			t.Fatalf("partial success: %q %+v", text, s)
		}
		if _, ok := Normalize(text); ok {
			t.Fatalf("normalized partial source %q", text)
		}
	}
	if s := Symbols("void main() {}", 0, 10); s.Stop != dialect.LineCap {
		t.Fatal(s)
	}
	if s := Symbols("void a() {} void b() {}", 10, 1); s.Stop != dialect.SymbolCap {
		t.Fatal(s)
	}
	if s := Parse("void main() {" + strings.Repeat("x;", MaxTokens) + "}"); s.Reason != "shader-token-cap" {
		t.Fatal(s.Reason)
	}
	if s := Parse(strings.Repeat("void a() {}\n", MaxBodies+1)); s.Reason != "shader-body-cap" {
		t.Fatal(s.Reason)
	}
}

func TestRSPV0005NormalizationAndIncludes(t *testing.T) {
	a, ok := Normalize("void main() { float x = 1; } // tail")
	b, ok2 := Normalize("  void   main ( ) { float x=1; } /* comment */")
	if !ok || !ok2 || !reflect.DeepEqual(a, b) {
		t.Fatalf("%v %v", a, b)
	}
	x, _ := Normalize("void main() { probe(\"//a\"); }")
	y, _ := Normalize("void main() { probe(\"//b\"); }")
	if reflect.DeepEqual(x, y) {
		t.Fatal("literal edit hidden")
	}
	s := Parse("// #include \"ignored.h\"\n/* #include <other> */\n#include <lib/common.h>\nvoid main() {}")
	if s.Reason != "" || !reflect.DeepEqual(s.Includes, []Include{{Name: "lib/common.h", Angled: true}}) {
		t.Fatalf("%+v", s)
	}
	hook, ok := dialect.LookupRegion("shader")
	if !ok || hook.Normalize == nil || hook.Symbols == nil {
		t.Fatal("dialect unregistered")
	}
}

// RSP-V0-005: operator and numeric token boundaries remain significant.
func TestRSPV0005TokenAdjacency(t *testing.T) {
	for _, pair := range [][2]string{
		{"void main() { x++; }", "void main() { x+ +; }"},
		{"void main() { x = 1.0; }", "void main() { x = 1 . 0; }"},
		{"void main() { x >>= 2; }", "void main() { x > >= 2; }"},
	} {
		a, ok := Normalize(pair[0])
		b, ok2 := Normalize(pair[1])
		if ok && ok2 && reflect.DeepEqual(a, b) {
			t.Fatalf("token change hidden: %v", pair)
		}
	}
}
