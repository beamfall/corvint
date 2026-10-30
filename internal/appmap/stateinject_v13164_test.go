package appmap

import (
	"strings"
	"testing"
)

// AMAP-V0-650 (V1-3164): a plain function or arrow that spreads a parameter into a literal it returns
// can end that literal in a plain element (exempt by AMAP-V0-384), so a caller that pops the element
// and pushes a function registers an annotation rebuilt from its argument. Each witness keeps
// computed-annotation, as the method path does under AMAP-V0-426.
func TestAMAPV0650PlainFunctionSpreadRebuildBlocks(t *testing.T) {
	const caller = "const annotation = go(data);\nannotation.pop();\nannotation.push(function (n) { n.HOME = '/changed'; });\nregister(annotation);\n"
	for name, src := range map[string]string{
		"function declaration": "export function go(i) { return [...i, 0] }\n" + caller,
		"unexported function":  "function go(i) { return [...i, 0] }\n" + caller,
		"const arrow body":     "export const go = (i) => [...i, 0];\n" + caller,
		"const arrow block":    "export const go = (i) => { return [...i, 0]; };\n" + caller,
		"function expression":  "export const go = function (i) { return [...i, 0]; };\n" + caller,
		"later parameter":      "export function go(a, i) { return [a, ...i, 0] }\nconst annotation = go(1, data);\nannotation.pop();\nregister(annotation);\n",
		"nested arrow":         "export function go(i) { const f = () => [...i, 0]; return f() }\n" + caller,
		"parameter member":     "export function go(t) { return [...t.deps, 0] }\nconst annotation = go(data);\nannotation.pop();\nregister(annotation);\n",
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := gh1994Routed(t, src, "")
			if d := diDetails(m); len(d) == 0 || !strings.Contains(strings.Join(d, ";"), ":computed-annotation") {
				t.Fatalf("a plain-function spread rebuild resolved: %+v", m.Unknowns)
			}
		})
	}
	// A plain-ended literal with no spread, and one spreading a module const rather than a parameter,
	// stay quiet as before.
	for name, src := range map[string]string{
		"no spread":           "export function go(i) { return [i.a, 0] }\ngo(data);\n",
		"module const spread": "const PIPES = [PipeA, PipeB];\nexport function go(i) { return { list: [...PIPES, 'x'], i } }\ngo(data);\n",
	} {
		t.Run(name, func(t *testing.T) {
			m, details := gh1994Routed(t, src, "")
			gh1869Resolved(t, name, m, details)
		})
	}
}
