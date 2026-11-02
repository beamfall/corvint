package python

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// astImports is run by python3 -I: it reads a JSON list of sources and prints,
// for each, the [level, module] pairs of every import statement ast finds.
const astImports = `import ast, json, sys
out = []
for source in json.load(sys.stdin):
    found = []
    try:
        tree = ast.parse(source)
    except SyntaxError:
        out.append(None)
        continue
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            found += [[0, alias.name] for alias in node.names]
        elif isinstance(node, ast.ImportFrom):
            found.append([node.level, node.module or ""])
    out.append(found)
json.dump(out, sys.stdout)
`

// differentialCorpus holds inputs where a line scanner can lose track of
// string, comment and statement boundaries: f-strings, bytes and raw
// prefixes, nested and triple quotes, escapes, line continuations, compound
// bodies and semicolon chains.
var differentialCorpus = []string{
	"import a\n",
	"import a.b as c, d\n",
	"from . import x\n",
	"from ..pkg import (a,\n    b)\n",
	"from x import *\n",
	"x = 1; import semi\n",
	"if True: import cond\n",
	"try:\n    import tried\nexcept ImportError:\n    import fallback\n",
	"s = f\"{';'}\"; import after_fstring\n",
	"s = f'{\"#\"}'; import after_fstring_hash\n",
	"v = b\"\\\\\"; import after_bytes\n",
	"v = rb'''x;'''; import after_rawbytes\n",
	"v = r\"\\\\\"; import after_raw\n",
	"import \\\n    continued\n",
	"from pkg \\\n    import sub\n",
	"x = 1 \\\n    ; import after_continuation\n",
	"print('#'); import after_hash_string\n",
	"# ; import commented\nimport real\n",
	"d = \"\"\"\nimport docstring_only\n\"\"\"; import after_doc\n",
	"d = '''a;\nb'''; import after_multiline\n",
	"s = 'it\\'s'; import after_escaped_quote\n",
	"s = \"a\\\\\\\\\"; import after_double_escape\n",
	"f = lambda: 0; import after_lambda\n",
	"class A: import in_class\n",
	"async def f(): import in_async\n",
	"with open('x') as h: import in_with\n",
	"def f():\n    import nested\n",
	"from __future__ import annotations\n",
	"s = f\"{f'{1}'}\"; import after_nested_fstring\n",
	"s = f'''{1};\n'''; import after_triple_fstring\n",
	"import café\n",
	"if True:\n\timport tabbed\nimport after_tab\n",
	"x = (1,\n     2); import after_paren\n",
	"x = [\n    'import fake',\n]; import after_list\n",
	"if (n := 1): import after_walrus\n",
	"while False: import in_while\n",
	"for i in []: import in_for\n",
	"else_ = 0; from . import rel_after_semi\n",
	"s = u'u;'; import after_unicode_prefix\n",
	"s = Rb\"x\"; import after_upper_prefix\n",
	"t = t'{1};'; import after_tstring\n",
}

// pythonOnly lists corpus inputs that an older python3 may reject, so the
// test skips them there instead of failing.
var pythonOnly = map[string]bool{"t = t'{1};'; import after_tstring\n": true}

// TestScanImportsAgreesWithAst_V13322: every import Python's ast finds in a
// corpus input is read by scanImports, or the file raises a frontier; a
// silent miss fails (AFP-V0-552).
func TestScanImportsAgreesWithAst_V13322(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not installed")
	}
	input, err := json.Marshal(differentialCorpus)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(python, "-I", "-c", astImports)
	command.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("python3 ast: %v: %s", err, stderr.String())
	}
	var want [][][2]any
	if err := json.Unmarshal(output, &want); err != nil {
		t.Fatal(err)
	}
	for index, source := range differentialCorpus {
		if want[index] == nil {
			if !pythonOnly[source] {
				t.Errorf("corpus input %q is not valid Python", source)
			}
			continue
		}
		refs, flags := scanImports("pkg/mod.py", source)
		read := make(map[importRef]bool, len(refs))
		for _, ref := range refs {
			read[importRef{module: ref.module, dots: ref.dots}] = true
		}
		for _, pair := range want[index] {
			ref := importRef{module: pair[1].(string), dots: int(pair[0].(float64))}
			if read[ref] {
				continue
			}
			if len(flags) > 0 {
				t.Logf("frontier %v covers missed %s%s in %q", flags, strings.Repeat(".", ref.dots), ref.module, source)
				continue
			}
			t.Errorf("scanImports misses %s%s in %q with no frontier; read %v", strings.Repeat(".", ref.dots), ref.module, source, refs)
		}
	}
}
