package contextindex

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func objectKeyNames(keys []Symbol) []string {
	var out []string
	for _, k := range keys {
		out = append(out, fmt.Sprintf("%d:%s", k.Line, k.Name))
	}
	return out
}

// DCS-V1-583 (V1-3073, GitHub #1975): the top-level keys of an exported
// object literal are named with their lines, for each export form; nested
// keys, spreads, computed keys and unexported objects are not.
func TestDCSV1583WebExportedObjectKeys(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		text string
		want []string
	}{
		"default call": {
			"import { defineConfig } from 'vite'\nexport default defineConfig({\n  root: 'src',\n  server: { port: 1, host: true },\n  ...base,\n  [dynamic]: 1,\n  'build-dir': 'out',\n  plugins,\n  async setup() {},\n  get mode() { return 1 },\n})\n",
			[]string{"3:root", "4:server", "7:build-dir", "8:plugins", "9:setup", "10:mode"},
		},
		"default literal": {"export default {\n  test: { globals: true },\n}\n", []string{"2:test"}},
		"named typed": {
			"type C = { a: string }\nexport const config: Record<string, { a: string }> = {\n  alpha: 1,\n  beta: () => 2,\n}\nconst hidden = { gamma: 1 }\n",
			[]string{"3:alpha", "4:beta"},
		},
		"default name":           {"const config = defineConfig({\n  delta: 1,\n})\nexport default config\n", []string{"2:delta"}},
		"default name semicolon": {"const config = {\n  delta: 1,\n}\nexport default config;\nconst other = 2\n", []string{"2:delta"}},
		"default name next line": {"const config = {\n  delta: 1,\n}\nexport default config\nconst other = 2\n", []string{"2:delta"}},
		// V1-3075: a bare name that only starts the exported expression is
		// not the exported value.
		"default conditional": {"const config = { secretOption: 1 }; export default config ? {} : {};\n", nil},
		"default binary":      {"const config = { secretOption: 1 }\nexport default config || fallback\n", nil},
		"default member":      {"const config = { secretOption: 1 }\nexport default config.inner\n", nil},
		"default continued":   {"const config = { secretOption: 1 }\nexport default config\n  as unknown\n", nil},
		// V1-3076: ASI ends the statement before a later line that starts
		// with `{`, `!` or `++`; a template literal there is a tagged call.
		"default name asi block":      {"const config = { delta: 1 }\nexport default config\n{}\n", []string{"1:delta"}},
		"default name asi not":        {"const config = { delta: 1 }\nexport default config\n!false\n", []string{"1:delta"}},
		"default name asi inc":        {"const config = { delta: 1 }\nexport default config\n++n\n", []string{"1:delta"}},
		"default name binary plus":    {"const config = { secretOption: 1 }\nexport default config\n+ +n\n", nil},
		"default tagged template":     {"const config = { secretOption: 1 }\nexport default config\n`tpl`\n", nil},
		"default tagged substitution": {"const config = { secretOption: 1 }\nexport default config\n`a${b}`\n", nil},
		"default name next string":    {"const config = { delta: 1 }\nexport default config\n'use strict'\n", []string{"1:delta"}},
		// V1-3089: `!=`/`!==` and a `+` before a string continue the value.
		"default strict inequality": {"const config = { secretOption: 1 };\nexport default config\n!== null;\n", nil},
		"default loose inequality":  {"const config = { secretOption: 1 };\nexport default config\n!= null;\n", nil},
		"default plus string":       {"const config = { secretOption: 1 };\nexport default config\n+\"+\";\n", nil},
		"module exports":            {"module.exports = {\n  epsilon: 1,\n}\n", []string{"2:epsilon"}},
		"not exported":              {"const config = { zeta: 1 }\nfunction f() { return { eta: 1 } }\n", nil},
		"export function":           {"export default function f() { return { theta: 1 } }\n", nil},
	} {
		keys, ok := WebExportedObjectKeys("vite.config.ts", tc.text)
		if !ok || !slices.Equal(objectKeyNames(keys), tc.want) {
			t.Errorf("%s: keys = %v ok=%v, want %v", name, objectKeyNames(keys), ok, tc.want)
		}
		for _, k := range keys {
			if k.Kind != "property" || k.Path != "vite.config.ts" {
				t.Errorf("%s: key %+v", name, k)
			}
		}
	}
	if keys, ok := WebExportedObjectKeys("config.go", "export default { a: 1 }\n"); !ok || keys != nil {
		t.Fatalf("a non-web path yields keys: %v", keys)
	}
	if keys, ok := WebExportedObjectKeys("c.ts", "export default { a: 1 \n"); ok || keys != nil {
		t.Fatalf("an unclosed object yields keys: %v ok=%v", keys, ok)
	}
}

// DCS-V1-583: keys are bounded per object, per file and in length.
func TestDCSV1583WebExportedObjectKeysAreBounded(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	for n := range 3 {
		fmt.Fprintf(&b, "export const o%d = {\n", n)
		for k := range webObjectKeysPerObject + 10 {
			fmt.Fprintf(&b, "  k%d_%d: 1,\n", n, k)
		}
		b.WriteString("}\n")
	}
	keys, ok := WebExportedObjectKeys("c.ts", b.String())
	if !ok || len(keys) != webObjectKeysPerFile {
		t.Fatalf("keys = %d, want %d", len(keys), webObjectKeysPerFile)
	}
	if keys[webObjectKeysPerObject-1].Name != fmt.Sprintf("k0_%d", webObjectKeysPerObject-1) || keys[webObjectKeysPerObject].Name != "k1_0" {
		t.Fatalf("per-object bound not applied: %v", objectKeyNames(keys[webObjectKeysPerObject-1:webObjectKeysPerObject+1]))
	}
	long := strings.Repeat("x", webObjectKeyBytes+1)
	if keys, _ := WebExportedObjectKeys("c.ts", "export default { '"+long+"': 1, 'a b': 2, ok: 3 }\n"); !slices.Equal(objectKeyNames(keys), []string{"1:ok"}) {
		t.Fatalf("long or spaced string keys named: %v", objectKeyNames(keys))
	}
}
