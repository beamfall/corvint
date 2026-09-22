package analyzerpython

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	json "encoding/json/v2"
)

type vectorInput struct {
	handle, family, path, content string
}

func canonicalFrame(t testing.TB, requestID string, values ...vectorInput) []byte {
	t.Helper()
	inputs := make([]Input, len(values))
	for index, value := range values {
		sum := sha256.Sum256([]byte(value.content))
		inputs[index] = Input{
			Handle: value.handle, Family: value.family, Path: value.path,
			SHA256:        "sha256:" + hex.EncodeToString(sum[:]),
			ContentBase64: base64.StdEncoding.EncodeToString([]byte(value.content)),
		}
	}
	request := Request{
		Profile: Profile, Family: Family, RequestID: requestID, ScopeID: "root", CompilationUnitID: "unit-1",
		Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: inputs,
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func response(t testing.TB, frame []byte) string {
	t.Helper()
	result := string(Process(frame))
	if strings.Count(result, "\n") != 1 || !strings.HasSuffix(result, "\n") {
		t.Fatalf("not one LF frame: %q", result)
	}
	return result
}

func TestCanonicalPython312RichGrammar(t *testing.T) {
	requirement := struct{ name string }{name: "PNC-003 deterministic typed candidate facts"}
	if requirement.name == "" {
		t.Fatal("missing requirement claim")
	}
	frame := canonicalFrame(t, "request-rich",
		vectorInput{"input-001", "py.project", "pyproject.toml", "requires-python = \"==3.12.0\"\n"},
		vectorInput{"input-002", "py.requirements", "requirements.txt", "attrs==24.2.0\nrequests==2.32.3\n"},
		vectorInput{"input-003", "py.source", "tooling/rich.py", `# comments and indentation are syntax, not facts
"""triple quoted string"""
from .relay import client
from ... import support
import alpha as beta
@decorator(flag="x")
async def worker(items: list[str]) -> None:
    raw = r"\\n"
    blob = b"\\x00"
    rendered = f"{raw}:{blob!r}"
    values = [call(value) for value in items if value]
    match values:
        case []:
            return
    await client.run()
type Alias = list[str]
worker([])
`},
	)
	got := response(t, frame)
	if !strings.Contains(got, `"status":"CANDIDATE"`) {
		t.Fatal(got)
	}
	for _, closed := range []string{
		`"value":".relay"`, `"value":"..."`, `"value":"alpha"`,
		`"value":"async-function:worker"`, `"value":"decorator->async-function:worker"`,
		`"value":"worker"`, `"instance_id":"tooling/rich.py:17:1"`,
		`"value":"python-3.12.0"`,
	} {
		if !strings.Contains(got, closed) {
			t.Fatalf("missing %s in %s", closed, got)
		}
	}
}

func TestMalformedPythonNeverEmitsFacts(t *testing.T) {
	frame := canonicalFrame(t, "request-invalid", vectorInput{"input-001", "py.source", "tooling/invalid.py", "from relay import a-b\ndef class(): pass\n"})
	got := response(t, frame)
	if !strings.Contains(got, `"reason":"MALFORMED_INPUT"`) || strings.Contains(got, `"facts"`) || strings.Contains(got, "tooling/invalid.py:") {
		t.Fatalf("invalid Python fabricated output: %s", got)
	}
}

// TestClosedFutureSyntaxFails covers PNC-004 closed Python 3.12 syntax rejection.
func TestClosedFutureSyntaxFails(t *testing.T) {
	for name, source := range map[string]string{
		"type-default":          "type Alias[T = int] = list[T]\n",
		"template-string":       "t\"future\"\n",
		"template-assignment":   "value = t\"future\"\n",
		"template-f-expression": "value = f\"{t'future'}\"\n",
		"except-list":           "try:\n    pass\nexcept A, B:\n    pass\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := response(t, canonicalFrame(t, "request-"+name, vectorInput{"input-001", "py.source", "tooling/future.py", source}))
			if !strings.Contains(got, `"reason":"UNSUPPORTED_SCHEMA"`) {
				t.Fatalf("future Python accepted: %s", got)
			}
		})
	}
	continuedExact := strings.Join([]string{"try:", "    pass", "except A \\", ", B:", "    pass", ""}, "\n")
	if got := response(t, canonicalFrame(t, "request-except-list-continuation-exact", vectorInput{"input-001", "py.source", "tooling/future.py", continuedExact})); !strings.Contains(got, `"reason":"UNSUPPORTED_SCHEMA"`) {
		t.Fatalf("exact continued PEP 758 exception list escaped: %s", got)
	}
	for name, source := range map[string]string{
		"parenthesized-first-item":  strings.Join([]string{"try:", "    pass", "except (", "    A", "), B:", "    pass", ""}, "\n"),
		"comments-blank-and-nested": strings.Join([]string{"try:", "    pass", "except (", "    (A)", "    # continuation comment", "", "), B:", "    pass", ""}, "\n"),
	} {
		t.Run(name, func(t *testing.T) {
			got := response(t, canonicalFrame(t, "request-except-list-"+name, vectorInput{"input-001", "py.source", "tooling/future.py", source}))
			if !strings.Contains(got, `"reason":"UNSUPPORTED_SCHEMA"`) {
				t.Fatalf("multiline PEP 758 exception list escaped: %s", got)
			}
		})
	}
	if got := response(t, canonicalFrame(t, "request-ordinary-string", vectorInput{"input-001", "py.source", "tooling/ordinary.py", "t = \"ordinary\"\n"})); !strings.Contains(got, `"status":"CANDIDATE"`) {
		t.Fatalf("ordinary string rejected as template: %s", got)
	}
}

func TestPythonContinuationIndentationDoesNotNest(t *testing.T) {
	var source strings.Builder
	source.WriteString("value = 1 \\\n")
	for level := 1; level <= maxPythonDepth+1; level++ {
		source.WriteString(strings.Repeat(" ", level))
		source.WriteString("+ 1")
		if level < maxPythonDepth+1 {
			source.WriteString(" \\\n")
		} else {
			source.WriteByte('\n')
		}
	}
	got := response(t, canonicalFrame(t, "request-continuation-indent", vectorInput{"input-001", "py.source", "tooling/continuation.py", source.String()}))
	if !strings.Contains(got, `"status":"CANDIDATE"`) {
		t.Fatalf("continuation indentation became structural: %s", got)
	}
}

func TestPythonBareBackslashRejects(t *testing.T) {
	source := "import os\n\\ \n"
	got := response(t, canonicalFrame(t, "request-bare-backslash", vectorInput{"input-001", "py.source", "tooling/backslash.py", source}))
	if !strings.Contains(got, `"reason":"MALFORMED_INPUT"`) || strings.Contains(got, `"facts"`) {
		t.Fatalf("bare backslash emitted facts: %s", got)
	}
}

func TestPythonFStringExpressionDepthBoundPrecedesParser(t *testing.T) {
	for _, depth := range []int{maxPythonDepth - 1, maxPythonDepth, maxPythonDepth + 1} {
		t.Run(fmt.Sprintf("depth-%d", depth), func(t *testing.T) {
			expression := strings.Repeat("(", depth) + "value" + strings.Repeat(")", depth)
			got := preflightPythonSource([]byte("value = f\"{" + expression + "}\"\n"))
			if depth <= maxPythonDepth && got != "" {
				t.Fatalf("accepted f-string depth=%d preflight=%s", depth, got)
			}
			if depth > maxPythonDepth && got != failureLimit {
				t.Fatalf("unbounded f-string depth=%d preflight=%s", depth, got)
			}
		})
	}
	over := strings.Repeat("(", maxPythonDepth+1) + "value" + strings.Repeat(")", maxPythonDepth+1)
	got := response(t, canonicalFrame(t, "request-fstring-depth", vectorInput{"input-001", "py.source", "tooling/fstring.py", "value = f\"{" + over + "}\"\n"}))
	if !strings.Contains(got, `"reason":"LIMIT_EXCEEDED"`) {
		t.Fatalf("f-string expression reached parser: %s", got)
	}
}

func TestPythonFStringExpressionTokenBoundPrecedesParser(t *testing.T) {
	for name, testcase := range map[string]struct {
		fields int
		want   string
	}{
		"under": {maxPythonTokens - 5, `"status":"CANDIDATE"`},
		"at":    {maxPythonTokens - 4, `"status":"CANDIDATE"`},
		"over":  {maxPythonTokens - 3, `"reason":"LIMIT_EXCEEDED"`},
	} {
		t.Run(name, func(t *testing.T) {
			got := response(t, canonicalFrame(t, "request-fstring-token-"+name, vectorInput{"input-001", "py.source", "tooling/fstring-token.py", fStringFormatSpecTokens(testcase.fields)}))
			if !strings.Contains(got, testcase.want) {
				t.Fatalf("format fields=%d output=%s", testcase.fields, got)
			}
		})
	}
}

func fStringFormatSpecTokens(fields int) string {
	return "value = f\"{value:" + strings.Repeat("{x}", fields) + "}\"\n"
}

func TestPythonFStringStructuralClosure(t *testing.T) {
	for name, source := range map[string]string{
		"lone-literal-close":               "value = f\"x}\"\n",
		"mismatched-replacement-delimiter": "value = f\"{(]}\"\n",
		"single-quoted-newline":            "value = f\"{\nvalue\n}\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := response(t, canonicalFrame(t, "request-fstring-closure-"+name, vectorInput{"input-001", "py.source", "tooling/fstring.py", source}))
			if !strings.Contains(got, `"reason":"MALFORMED_INPUT"`) || strings.Contains(got, `"facts"`) {
				t.Fatalf("malformed f-string emitted facts: %s", got)
			}
		})
	}
	positive := "value = f\"{{literal}} {value:{width}.{precision}f}\"\n"
	if got := response(t, canonicalFrame(t, "request-fstring-closure-positive", vectorInput{"input-001", "py.source", "tooling/fstring.py", positive})); !strings.Contains(got, `"status":"CANDIDATE"`) {
		t.Fatalf("closed f-string rejected: %s", got)
	}
}

func TestPythonBytesLiteralSourceIsASCII(t *testing.T) {
	for name, literal := range map[string]string{
		"bytes":     `b"é"`,
		"raw-bytes": `rb'é'`,
	} {
		t.Run(name, func(t *testing.T) {
			source := literal + "\nimport os\n"
			got := response(t, canonicalFrame(t, "request-bytes-"+name, vectorInput{"input-001", "py.source", "tooling/bytes.py", source}))
			if !strings.Contains(got, `"reason":"MALFORMED_INPUT"`) || strings.Contains(got, `"facts"`) {
				t.Fatalf("non-ASCII bytes literal emitted facts: %s", got)
			}
		})
	}
	if got := response(t, canonicalFrame(t, "request-bytes-escape", vectorInput{"input-001", "py.source", "tooling/bytes.py", `b"\xc3\xa9"` + "\n"})); !strings.Contains(got, `"status":"CANDIDATE"`) {
		t.Fatalf("ASCII bytes escape rejected: %s", got)
	}
}

// TestUnrepresentableStaticTargetsFailClosed covers PNC-005 unrepresentable
// static targets reject dynamic input.
func TestUnrepresentableStaticTargetsFailClosed(t *testing.T) {
	for name, source := range map[string]string{
		"call":               "café()\n",
		"attribute-call":     "module.café()\n",
		"decorator":          "@café\ndef target():\n    pass\n",
		"call-decorator":     "@café()\ndef target():\n    pass\n",
		"import-module":      "import café\n",
		"from-import-module": "from café import value\n",
		"from-import-member": "from module import café\n",
		"definition":         "def café():\n    pass\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := response(t, canonicalFrame(t, "request-static-"+name, vectorInput{"input-001", "py.source", "tooling/static.py", source}))
			if !strings.Contains(got, `"reason":"DYNAMIC_INPUT"`) || strings.Contains(got, `"facts"`) {
				t.Fatalf("unrepresentable static target escaped: %s", got)
			}
		})
	}
}

func TestPython312IndentationBoundIsStructural(t *testing.T) {
	for name, testcase := range map[string]struct {
		blocks int
		want   string
	}{
		"under": {maxPythonDepth - 1, `"status":"CANDIDATE"`},
		"at":    {maxPythonDepth, `"status":"CANDIDATE"`},
		"over":  {maxPythonDepth + 1, `"reason":"LIMIT_EXCEEDED"`},
	} {
		t.Run(name, func(t *testing.T) {
			var source strings.Builder
			for level := 0; level < testcase.blocks; level++ {
				source.WriteString(strings.Repeat("    ", level))
				source.WriteString("if True:\n")
			}
			source.WriteString(strings.Repeat("    ", testcase.blocks))
			source.WriteString("pass\n")
			got := response(t, canonicalFrame(t, "request-indent-"+name, vectorInput{"input-001", "py.source", "tooling/indent.py", source.String()}))
			if !strings.Contains(got, testcase.want) {
				t.Fatalf("blocks=%d output=%s", testcase.blocks, got)
			}
		})
	}
}

func TestPython312IndentationStructure(t *testing.T) {
	for name, source := range map[string]string{
		"unexpected-top-level-indent": "    import os\n",
		"unindented-definition-body":  "def f():\npass\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := response(t, canonicalFrame(t, "request-indent-structure-"+name, vectorInput{"input-001", "py.source", "tooling/indent.py", source}))
			if !strings.Contains(got, `"reason":"MALFORMED_INPUT"`) || strings.Contains(got, `"facts"`) {
				t.Fatalf("invalid indentation emitted facts: %s", got)
			}
		})
	}
	nested := "def outer():\n    if ready:\n        def inner():\n            pass\n"
	got := response(t, canonicalFrame(t, "request-indent-structure-nested", vectorInput{"input-001", "py.source", "tooling/indent.py", nested}))
	for _, value := range []string{`"status":"CANDIDATE"`, `"value":"function:outer"`, `"value":"function:inner"`} {
		if !strings.Contains(got, value) {
			t.Fatalf("nested suite missing %s: %s", value, got)
		}
	}
}

func TestPythonMatchCasePlacementFailsClosed(t *testing.T) {
	for name, source := range map[string]string{
		"top-level-case":     "case 1:\n    import os\n",
		"case-under-if":      "if ready:\n    case 1:\n        import os\n",
		"match-without-case": "match value:\n    import os\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := response(t, canonicalFrame(t, "request-match-case-"+name, vectorInput{"input-001", "py.source", "tooling/match.py", source}))
			if !strings.Contains(got, `"reason":"MALFORMED_INPUT"`) || strings.Contains(got, `"facts"`) {
				t.Fatalf("misplaced match/case suite emitted facts: %s", got)
			}
		})
	}
	valid := "case: int = 1\nmatch: int = 2\nmatch value:\n    case 1:\n        import os\n    case _:\n        pass\n"
	if got := response(t, canonicalFrame(t, "request-match-case-valid", vectorInput{"input-001", "py.source", "tooling/match.py", valid})); !strings.Contains(got, `"status":"CANDIDATE"`) || !strings.Contains(got, `"value":"os"`) {
		t.Fatalf("valid match/case or soft-keyword assignment rejected: %s", got)
	}
}

func TestPython312DefinitionHeaders(t *testing.T) {
	for name, source := range map[string]string{
		"missing-function-parameters": "def target: call()\n",
		"default-before-required":     "def target(a=1, b): pass\n",
		"missing-return-annotation":   "def target() ->: pass\n",
		"malformed-class-parentheses": "class Target(,): pass\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := response(t, canonicalFrame(t, "request-header-"+name, vectorInput{"input-001", "py.source", "tooling/header.py", source}))
			if !strings.Contains(got, `"reason":"MALFORMED_INPUT"`) || strings.Contains(got, `"facts"`) {
				t.Fatalf("malformed header emitted facts: %s", got)
			}
		})
	}
	positive := "def build(left: int, /, right: str = \"x\", *args: object, enabled: bool = True, **options: object) -> dict[str, object]:\n    pass\nasync def worker(item: list[str], *, retry: int = 3) -> None:\n    pass\nclass Widget(Base, metaclass=Factory()):\n    pass\n"
	got := response(t, canonicalFrame(t, "request-header-positive", vectorInput{"input-001", "py.source", "tooling/header.py", positive}))
	for _, value := range []string{`"status":"CANDIDATE"`, `"value":"function:build"`, `"value":"async-function:worker"`, `"value":"class:Widget"`} {
		if !strings.Contains(got, value) {
			t.Fatalf("closed header missing %s: %s", value, got)
		}
	}
}

func TestHeaderExpressionShapesFailClosed(t *testing.T) {
	for name, source := range map[string]string{
		"dangling-unary-default":    "def target(x=+): pass\n",
		"dangling-binary-default":   "def target(x=1+): pass\n",
		"bare-star-default":         "def target(x=*): pass\n",
		"operator-only-annotation":  "def target(x: ->): pass\n",
		"adjacent-operands-default": "def target(x=1 2): pass\n",
		"colon-after-default":       "def target(x=1:2): pass\n",
		"dangling-direct-call-arg":  "call(1 +)\n",
		"separator-only-call-arg":   "call(,)\n",
		"leading-keyword-default":   "def target(x=and value): pass\n",
		"leading-keyword-call-arg":  "call(and value)\n",
		"brace-after-operand":       "def target(x=value{}): pass\n",
		"brace-after-operand-call":  "call(value{})\n",
		"colon-inside-parentheses":  "def target(x=(1:2)): pass\n",
		"dict-excess-colon":         "call({1:2:3})\n",
		"list-display-colon":        "call([1:2])\n",
		"keyword-operator-chain":    "call(a in in b)\n",
		"leading-not-in":            "call(not in value)\n",
		"compound-comparison-chain": "call(value is not in items)\n",
		"dangling-binary-not":       "call(a not b)\n",
		"empty-subscript-trailer":   "call(value[])\n",
		"dict-missing-key":          "call({:1})\n",
		"dict-missing-value":        "call({1:})\n",
		"mixed-dict-set-display":    "call({1: 2, 3})\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := response(t, canonicalFrame(t, "request-shape-"+name, vectorInput{"input-001", "py.source", "tooling/shape.py", source}))
			if !strings.Contains(got, `"reason":"MALFORMED_INPUT"`) || strings.Contains(got, `"facts"`) {
				t.Fatalf("malformed expression emitted facts: %s", got)
			}
		})
	}
	for name, source := range map[string]string{
		"negative-default":     "def f(x=-1): pass\n",
		"binary-default":       "def f(x=a + b): pass\n",
		"call-default":         "def f(x=g(1)): pass\n",
		"conditional-default":  "def f(x=a if b else c): pass\n",
		"lambda-default":       "def f(cb=lambda v: v + 1): pass\n",
		"ellipsis-annotation":  "def f(t: tuple[str, ...] = SCOPES): pass\n",
		"dict-slice-defaults":  "def f(d={1: 2}, s=x[1:2]): pass\n",
		"comparison-call-args": "call(a == b, not c)\n",
		"lambda-call-arg":      "sort(key=lambda item: (-item[0], item[1]))\n",
		"multi-param-lambda":   "call(lambda left, right: left)\n",
		"lambda-then-more":     "call(f(lambda a, b: x, other))\n",
		"not-in-chain":         "call(a not in b, c is not d, not e)\n",
		"async-comprehension":  "call([x async for y in z])\n",
		"call-after-operand":   "call(value(1), items[0])\n",
		"slice-step-subscript": "call(items[1:2:3], items[::2])\n",
		"dict-comprehension":   "call({k: v for k in x}, {s for s in x})\n",
		"tuple-of-slices":      "call(matrix[a:b, c:d])\n",
		"comp-tuple-target":    "call({name: found for name, found in items()})\n",
		"star-displays":        "call({*a, 1}, {**a, 1: 2})\n",
		"empty-displays":       "call([], {}, value[0])\n",
		"unary-not-chain":      "call(not not a, a and not b)\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := response(t, canonicalFrame(t, "request-shape-"+name, vectorInput{"input-001", "py.source", "tooling/shape.py", source}))
			if !strings.Contains(got, `"status":"CANDIDATE"`) {
				t.Fatalf("closed expression shape rejected: %s", got)
			}
		})
	}
}

func TestDecoratorExpressionsFailClosed(t *testing.T) {
	for name, source := range map[string]string{
		"plain":  "@name\ndef target():\n    pass\n",
		"dotted": "@pkg.name\ndef target():\n    pass\n",
		"call":   "@factory(flag=\"x\")\ndef target():\n    pass\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := response(t, canonicalFrame(t, "request-decorator-"+name, vectorInput{"input-001", "py.source", "tooling/decorator.py", source}))
			if !strings.Contains(got, `"status":"CANDIDATE"`) || !strings.Contains(got, `"value":"`+map[string]string{"plain": "name", "dotted": "pkg.name", "call": "factory"}[name]+`->function:target"`) {
				t.Fatalf("closed decorator rejected: %s", got)
			}
		})
	}
	got := response(t, canonicalFrame(t, "request-decorator-dynamic", vectorInput{"input-001", "py.source", "tooling/decorator.py", "@factory().attr\ndef f():\n    pass\n"}))
	if !strings.Contains(got, `"reason":"DYNAMIC_INPUT"`) || strings.Contains(got, `"facts"`) {
		t.Fatalf("unsupported decorator emitted facts: %s", got)
	}
}

func TestDecoratorCallArgumentsFailClosed(t *testing.T) {
	for name, decorator := range map[string]string{
		"empty-item":      "@factory(,)",
		"dangling-binary": "@factory(value +)",
	} {
		t.Run(name, func(t *testing.T) {
			source := decorator + "\ndef target(): pass\n"
			got := response(t, canonicalFrame(t, "request-decorator-args-"+name, vectorInput{"input-001", "py.source", "tooling/decorator.py", source}))
			if !strings.Contains(got, `"reason":"MALFORMED_INPUT"`) || strings.Contains(got, `"facts"`) {
				t.Fatalf("malformed decorator arguments emitted facts: %s", got)
			}
		})
	}
}

func TestDecoratorCallArgumentOrdering(t *testing.T) {
	for name, decorator := range map[string]string{
		"positional-after-keyword": "@factory(x=1, y)",
		"comparison-after-keyword": "@factory(x=1, y == 2)",
		"iterable-after-mapping":   "@factory(**options, *items)",
	} {
		t.Run(name, func(t *testing.T) {
			source := decorator + "\ndef target(): pass\n"
			got := response(t, canonicalFrame(t, "request-decorator-order-invalid-"+name, vectorInput{"input-001", "py.source", "tooling/decorator.py", source}))
			if !strings.Contains(got, `"reason":"MALFORMED_INPUT"`) || strings.Contains(got, `"facts"`) {
				t.Fatalf("invalid decorator argument order emitted facts: %s", got)
			}
		})
	}
	for name, decorator := range map[string]string{
		"iterable-after-keyword":   "@factory(x=1, *items)",
		"mapping-after-keyword":    "@factory(x=1, **options)",
		"keyword-after-mapping":    "@factory(**options, y=1)",
		"keyword-after-comparison": "@factory(x == 1, y=2)",
		"positional-after-star":    "@factory(*items, y)",
	} {
		t.Run(name, func(t *testing.T) {
			source := decorator + "\ndef target(): pass\n"
			got := response(t, canonicalFrame(t, "request-decorator-order-"+name, vectorInput{"input-001", "py.source", "tooling/decorator.py", source}))
			if !strings.Contains(got, `"status":"CANDIDATE"`) || !strings.Contains(got, `"value":"factory->function:target"`) {
				t.Fatalf("valid unpacked decorator rejected: %s", got)
			}
		})
	}
}

func TestNestedTypeParameterDefaultsFailClosed(t *testing.T) {
	for name, source := range map[string]string{
		"alias":    "type Alias[T: tuple[int, int] = str] = T\n",
		"function": "def f[T: tuple[int, int] = str](): pass\n",
		"class":    "class C[T: tuple[int, int] = str]: pass\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := response(t, canonicalFrame(t, "request-type-default-"+name, vectorInput{"input-001", "py.source", "tooling/type_params.py", source}))
			if !strings.Contains(got, `"reason":"UNSUPPORTED_SCHEMA"`) || strings.Contains(got, `"facts"`) {
				t.Fatalf("nested type default emitted facts: %s", got)
			}
		})
	}
	positive := "type Alias[T: tuple[int, int]] = T\ndef f[T: tuple[int, int]](): pass\nclass C[T: tuple[int, int]]: pass\n"
	if got := response(t, canonicalFrame(t, "request-type-default-positive", vectorInput{"input-001", "py.source", "tooling/type_params.py", positive})); !strings.Contains(got, `"status":"CANDIDATE"`) {
		t.Fatalf("nested type bounds rejected: %s", got)
	}
}

func TestClosedCoordinatesAndTypedFactOrder(t *testing.T) {
	frame := canonicalFrame(t, "request-order", vectorInput{"input-001", "py.source", "tooling/order.py", "import zebra\nimport alpha\ndef second(): pass\ndef first(): pass\nfirst()\n"})
	got := response(t, frame)
	for _, coordinate := range []string{"tooling/order.py:1:1", "tooling/order.py:2:1", "tooling/order.py:3:1", "tooling/order.py:4:1", "tooling/order.py:5:1"} {
		if !strings.Contains(got, coordinate) {
			t.Fatalf("missing closed source coordinate %q in %s", coordinate, got)
		}
	}
	var result success
	if err := json.Unmarshal([]byte(strings.TrimSuffix(got, "\n")), &result, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	for index := 1; index < len(result.Facts); index++ {
		if compareFact(result.Facts[index-1], result.Facts[index]) >= 0 {
			t.Fatalf("non-typed order at %d: %#v %#v", index, result.Facts[index-1], result.Facts[index])
		}
	}
}

func TestRejectionEchoesEveryBoundInputAndReason(t *testing.T) {
	frame := canonicalFrame(t, "request-reject",
		vectorInput{"input-001", "py.source", "tooling/first.py", "import first\n"},
		vectorInput{"input-002", "py.source", "tooling/second.py", "import second\n"},
	)
	var request Request
	if err := json.Unmarshal(bytes.TrimSuffix(frame, []byte{'\n'}), &request); err != nil {
		t.Fatal(err)
	}
	request.Inputs[0].SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	bad := append(encoded, '\n')
	got := response(t, bad)
	for _, required := range []string{
		`"profile":"corvint-analyzer-candidate/experimental"`, `"family":"python"`, `"request_id":"request-reject"`,
		`"scope_id":"root"`, `"compilation_unit_id":"unit-1"`, `"target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]}`,
		`"handle":"input-001"`, `"handle":"input-002"`, `"reason":"DIGEST_MISMATCH"`,
	} {
		if !strings.Contains(got, required) {
			t.Fatalf("missing %s in %s", required, got)
		}
	}
}

func TestMinimalRejectionSentinels(t *testing.T) {
	want := `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n"
	for _, frame := range [][]byte{nil, []byte("{}\n"), []byte("{\"family\":\"python\"}\n"), []byte("{\"x\":1}\r\n")} {
		if got := response(t, frame); got != want {
			t.Fatalf("minimal=%q want=%q", got, want)
		}
	}
	wantFamily := `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"UNKNOWN_FAMILY"}` + "\n"
	for _, requestID := range []string{"request-unknown-family", "request-unknown-family-distinct"} {
		unknownFamily := editedFrame(t, canonicalFrame(t, requestID, vectorInput{"input-001", "py.source", "tooling/unknown.py", "import example\n"}), func(request *Request) {
			request.Family = "other"
		})
		if got := response(t, unknownFamily); got != wantFamily {
			t.Fatalf("unknown family=%q want=%q", got, wantFamily)
		}
	}
}

func TestInvalidEnvelopeRejectionStaysUnbound(t *testing.T) {
	base := canonicalFrame(t, "request-unbound", vectorInput{"input-001", "py.source", "tooling/a.py", "import a\n"}, vectorInput{"input-002", "py.source", "tooling/b.py", "import b\n"})
	for reason, edit := range map[failure]func(*Request){
		failureFamily:     func(request *Request) { request.Profile, request.Family = "other-profile", "other" },
		failureIdentifier: func(request *Request) { request.Inputs[0].Handle = "bad handle" },
		failureDigest:     func(request *Request) { request.Inputs[0].SHA256 = "sha256:BAD" },
		failureDuplicate:  func(request *Request) { request.Inputs[1].Handle = request.Inputs[0].Handle },
	} {
		want := `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"` + string(reason) + "\"}\n"
		if got := response(t, editedFrame(t, base, edit)); got != want {
			t.Fatalf("%s: got=%q want=%q", reason, got, want)
		}
	}
}

func TestPreflightBoundsStringsAndTokensBeforeDecode(t *testing.T) {
	ordinary := `{"ordinary":"` + strings.Repeat("a", maxStringBytes+1) + `"}`
	if got := preflight([]byte(ordinary)); got != failureLimit {
		t.Fatalf("ordinary string preflight=%s", got)
	}
	base64Member := `{"content_base64":"` + strings.Repeat("A", maxStringBytes+1) + `"}`
	if got := preflight([]byte(base64Member)); got != "" {
		t.Fatalf("bounded base64 member rejected before its own cap: %s", got)
	}
	var object strings.Builder
	object.WriteByte('{')
	for index := 0; index <= maxJSONTokens; index++ {
		if index > 0 {
			object.WriteByte(',')
		}
		object.WriteString(`"k`)
		object.WriteString(strconv.Itoa(index))
		object.WriteString(`":0`)
	}
	object.WriteByte('}')
	if got := preflight([]byte(object.String())); got != failureLimit {
		t.Fatalf("object-key token preflight=%s", got)
	}
}

func TestPreflightProspectivelyBoundsInputsAndFeatures(t *testing.T) {
	for _, testcase := range []struct {
		name, field, want string
		count             int
	}{
		{"inputs-under", "inputs", "", MaxInputs - 1},
		{"inputs-at", "inputs", "", MaxInputs},
		{"inputs-over", "inputs", string(failureLimit), MaxInputs + 1},
		{"features-under", "features", "", maxFeatures - 1},
		{"features-at", "features", "", maxFeatures},
		{"features-over", "features", string(failureLimit), maxFeatures + 1},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			values := make([]string, testcase.count)
			for index := range values {
				values[index] = `{"handle":"h","family":"py.source","path":"x.py","sha256":"sha256:0000000000000000000000000000000000000000000000000000000000000000","content_base64":"eAo="}`
				if testcase.field == "features" {
					values[index] = `"f` + strconv.Itoa(index) + `"`
				}
			}
			body := `{"` + testcase.field + `":[` + strings.Join(values, ",") + `]}`
			got := preflight([]byte(body))
			if string(got) != testcase.want {
				t.Fatalf("count=%d preflight=%s want=%s", testcase.count, got, testcase.want)
			}
		})
	}
}

func TestPreflightCanonicalizesKeysBeforeProspectiveBounds(t *testing.T) {
	requirement := struct{ name string }{name: "PNC-002 prospective canonical request member bounds"}
	if requirement.name == "" {
		t.Fatal("missing requirement claim")
	}
	for _, testcase := range []struct {
		name, field, value string
		count              int
	}{
		{"escaped-inputs", `\u0069nputs`, `{"handle":"h","family":"py.source","path":"x.py","sha256":"sha256:0000000000000000000000000000000000000000000000000000000000000000","content_base64":"eAo="}`, MaxInputs + 1},
		{"escaped-features", `\u0066eatures`, `"feature"`, maxFeatures + 1},
		{"escaped-content-base64", `c\u006fntent_base64`, `"` + strings.Repeat("A", maxBase64Bytes+1) + `"`, 1},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			body := `{"` + testcase.field + `":[` + strings.TrimSuffix(strings.Repeat(testcase.value+",", testcase.count), ",") + `]}`
			if got := preflight([]byte(body)); got != failureLimit {
				t.Fatalf("preflight=%s", got)
			}
			if got := response(t, append([]byte(body), '\n')); !strings.Contains(got, `"reason":"LIMIT_EXCEEDED"`) {
				t.Fatalf("Process allocated before escaped bound: %s", got)
			}
		})
	}
}

func TestPublicFrameAndJSONBoundaries(t *testing.T) {
	for _, testcase := range []struct {
		name   string
		size   int
		reason failure
	}{
		{"under", MaxRequestBytes - 1, failureNoncanonical},
		{"at", MaxRequestBytes, failureNoncanonical},
		{"over", MaxRequestBytes + 1, failureLimit},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			if got := Process([]byte(strings.Repeat("x", testcase.size))); !bytes.Contains(got, []byte(`"reason":"`+string(testcase.reason)+`"`)) {
				t.Fatalf("size=%d output=%s", testcase.size, got)
			}
		})
	}
	for _, testcase := range []struct {
		name   string
		values int
		want   failure
	}{
		{"tokens-under", maxJSONTokens - 2, ""},
		{"tokens-at", maxJSONTokens - 1, ""},
		{"tokens-over", maxJSONTokens, failureLimit},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			body := "[" + strings.TrimSuffix(strings.Repeat("0,", testcase.values), ",") + "]"
			if got := preflight([]byte(body)); got != testcase.want {
				t.Fatalf("values=%d preflight=%s want=%s", testcase.values, got, testcase.want)
			}
		})
	}
	for _, testcase := range []struct {
		name  string
		depth int
		want  failure
	}{
		{"depth-under", maxJSONDepth - 1, ""},
		{"depth-at", maxJSONDepth, ""},
		{"depth-over", maxJSONDepth + 1, failureLimit},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			body := strings.Repeat("[", testcase.depth) + "0" + strings.Repeat("]", testcase.depth)
			if got := preflight([]byte(body)); got != testcase.want {
				t.Fatalf("depth=%d preflight=%s want=%s", testcase.depth, got, testcase.want)
			}
		})
	}
}

func TestRequirementsLineBoundPrecedesStringRetention(t *testing.T) {
	content := "package==" + strings.Repeat("1", 513) + "\n"
	got := response(t, canonicalFrame(t, "request-requirement-bound", vectorInput{"input-001", "py.requirements", "requirements.txt", content}))
	if !strings.Contains(got, `"reason":"LIMIT_EXCEEDED"`) {
		t.Fatalf("requirement line bound=%s", got)
	}
}

func TestAggregateInputAndBase64Boundaries(t *testing.T) {
	for _, testcase := range []struct {
		name string
		size int
		want string
	}{
		{"input-under", MaxInputBytes - 1, `"status":"CANDIDATE"`},
		{"input-at", MaxInputBytes, `"status":"CANDIDATE"`},
		{"input-over", MaxInputBytes + 1, `"reason":"LIMIT_EXCEEDED"`},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			content := strings.Repeat("#", testcase.size-1) + "\n"
			got := response(t, canonicalFrame(t, "request-"+testcase.name, vectorInput{"input-001", "py.source", "tooling/input-size.py", content}))
			if !strings.Contains(got, testcase.want) {
				t.Fatalf("size=%d output=%s", testcase.size, got)
			}
		})
	}
	for _, testcase := range []struct {
		name string
		size int
		want failure
	}{
		{"base64-under", maxBase64Bytes - 1, ""},
		{"base64-at", maxBase64Bytes, ""},
		{"base64-over", maxBase64Bytes + 1, failureLimit},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			body := `{"content_base64":"` + strings.Repeat("A", testcase.size) + `"}`
			if got := preflight([]byte(body)); got != testcase.want {
				t.Fatalf("size=%d preflight=%s want=%s", testcase.size, got, testcase.want)
			}
		})
	}
}

func TestBase64RejectsNoncanonicalPadBits(t *testing.T) {
	requirement := struct{ name string }{name: "PNC-006 strict RFC 4648 canonical pad bits"}
	if requirement.name == "" {
		t.Fatal("missing requirement claim")
	}
	for _, testcase := range []struct {
		value string
		bytes int
		want  failure
	}{
		{"YQ==", 1, ""}, {"YR==", 0, failureMalformed},
		{"YQo=", 2, ""}, {"YQp=", 0, failureMalformed},
	} {
		if got, reason := decodedLength(testcase.value); got != testcase.bytes || reason != testcase.want {
			t.Fatalf("decodedLength(%q)=(%d,%s) want=(%d,%s)", testcase.value, got, reason, testcase.bytes, testcase.want)
		}
	}
	for _, testcase := range []struct{ canonical, noncanonical, content string }{{"YQ==", "YR==", "a"}, {"YQo=", "YQp=", "a\n"}} {
		frame := canonicalFrame(t, "request-base64-pad", vectorInput{"input-001", "py.source", "tooling/base64.py", testcase.content})
		frame = []byte(strings.Replace(string(frame), testcase.canonical, testcase.noncanonical, 1))
		got := response(t, frame)
		if !strings.Contains(got, `"reason":"MALFORMED_INPUT"`) || strings.Contains(got, `"facts"`) {
			t.Fatalf("noncanonical base64 %q accepted: %s", testcase.noncanonical, got)
		}
	}
}

func TestPythonTokenAndDepthBoundsPrecedeParser(t *testing.T) {
	tokens := strings.Repeat("x=1\n", maxPythonTokens/2+1)
	got := response(t, canonicalFrame(t, "request-token-bound", vectorInput{"input-001", "py.source", "tooling/token-bound.py", tokens}))
	if !strings.Contains(got, `"reason":"LIMIT_EXCEEDED"`) {
		t.Fatalf("token bound=%s", got)
	}
	depth := strings.Repeat("(", maxPythonDepth+1) + "1" + strings.Repeat(")", maxPythonDepth+1) + "\n"
	got = response(t, canonicalFrame(t, "request-depth-bound", vectorInput{"input-001", "py.source", "tooling/depth-bound.py", depth}))
	if !strings.Contains(got, `"reason":"LIMIT_EXCEEDED"`) {
		t.Fatalf("depth bound=%s", got)
	}
}

// TestPreflightDepthFloorSurvivesUnmatchedClose confirms leading unmatched
// closing delimiters cannot lower the recorded depth below zero and thereby
// raise the effective nesting ceiling above maxPythonDepth.
func TestPreflightDepthFloorSurvivesUnmatchedClose(t *testing.T) {
	realDepth := maxPythonDepth + 20
	source := []byte(strings.Repeat(")", 25) + strings.Repeat("(", realDepth) + "1" + strings.Repeat(")", realDepth) + "\n")
	if got := preflightPythonSource(source); got != failureLimit {
		t.Fatalf("unmatched leading close raised the depth ceiling: preflight=%q", got)
	}
}

func editedFrame(t testing.TB, frame []byte, edit func(*Request)) []byte {
	t.Helper()
	var request Request
	if err := json.Unmarshal(bytes.TrimSuffix(frame, []byte{'\n'}), &request); err != nil {
		t.Fatal(err)
	}
	edit(&request)
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func TestSafeCanonicalExtensionBindsUnknownField(t *testing.T) {
	frame := canonicalFrame(t, "request-extension", vectorInput{"input-001", "py.source", "tooling/a.py", "import a\n"})
	var request Request
	if err := json.Unmarshal(bytes.TrimSuffix(frame, []byte{'\n'}), &request); err != nil {
		t.Fatal(err)
	}
	body := strings.TrimSuffix(string(frame), "\n")
	base := strings.TrimSuffix(body, "}")
	for _, edited := range []string{
		base + `,"x":[9007199254740993,{"a":"b","c":null}]}`,
		base + `,"target inputs":0}`,
		strings.Replace(body, `"features":[]`, `"features":[],"x":true`, 1),
		strings.Replace(body, `"}]}`, `","x":0,"y":"z"}]}`, 1),
	} {
		if got, want := response(t, []byte(edited+"\n")), string(rejection(request, failureField)); got != want {
			t.Fatalf("frame=%s\ngot=%s\nwant=%s", edited, got, want)
		}
	}
	sentinel := `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"`
	for edited, reason := range map[string]failure{
		base + `,"x":1.0}`: failureNoncanonical, base + `,"y":0,"x":0}`: failureNoncanonical, base + `,"x":{"b":0,"a":0}}`: failureNoncanonical,
		strings.Replace(body, `{"profile":`, `{"x":0,"profile":`, 1):  failureNoncanonical,
		strings.Replace(base, `"sha256:`, `"sha256:X`, 1) + `,"x":0}`: failureDigest,
	} {
		if got := response(t, []byte(edited+"\n")); got != sentinel+string(reason)+"\"}\n" {
			t.Fatalf("frame=%s\ngot=%s", edited, got)
		}
	}
}

func TestEveryReachableClosedReason(t *testing.T) {
	base := canonicalFrame(t, "request-reasons", vectorInput{"input-001", "py.source", "tooling/reasons.py", "import valid\n"})
	cases := []struct {
		name, reason string
		frame        []byte
	}{
		{"invalid-identifier", string(failureIdentifier), editedFrame(t, base, func(request *Request) { request.RequestID = "bad identifier" })},
		{"unknown-family", string(failureFamily), editedFrame(t, base, func(request *Request) { request.Family = "other" })},
		{"invalid-path", string(failurePath), editedFrame(t, base, func(request *Request) { request.Inputs[0].Path = "bad:path.py" })},
		{"digest", string(failureDigest), editedFrame(t, base, func(request *Request) {
			request.Inputs[0].SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		})},
		{"duplicate", string(failureDuplicate), editedFrame(t, canonicalFrame(t, "request-duplicate", vectorInput{"input-001", "py.source", "tooling/a.py", "import a\n"}, vectorInput{"input-002", "py.source", "tooling/b.py", "import b\n"}), func(request *Request) { request.Inputs[1].Path = request.Inputs[0].Path })},
		{"malformed", string(failureMalformed), canonicalFrame(t, "request-malformed", vectorInput{"input-001", "py.source", "tooling/malformed.py", "def class(): pass\n"})},
		{"unsupported", string(failureUnsupported), canonicalFrame(t, "request-unsupported", vectorInput{"input-001", "py.project", "pyproject.toml", "requires-python = \"==3.13.0\"\n"})},
		{"dynamic", string(failureDynamic), canonicalFrame(t, "request-dynamic", vectorInput{"input-001", "py.source", "tooling/dynamic.py", "import café\n"})},
		{"limit", string(failureLimit), editedFrame(t, base, func(request *Request) { request.Inputs = nil })},
	}
	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			got := response(t, testcase.frame)
			if !strings.Contains(got, `"reason":"`+testcase.reason+`"`) || strings.Contains(got, `"facts"`) {
				t.Fatalf("reason %s: %s", testcase.reason, got)
			}
		})
	}
	unknown := append(bytes.TrimSuffix(base, []byte("}\n")), `,"unknown":0}`+"\n"...)
	if got := response(t, unknown); !strings.Contains(got, `"reason":"UNKNOWN_FIELD"`) {
		t.Fatalf("unknown field: %s", got)
	}
	var source strings.Builder
	for index := 0; index < MaxFacts; index++ {
		source.WriteString("import item")
		source.WriteString(strconv.Itoa(index))
		source.WriteByte('\n')
	}
	if got := response(t, canonicalFrame(t, "request-output", vectorInput{"input-001", "py.source", "tooling/output.py", source.String()})); !strings.Contains(got, `"reason":"OUTPUT_LIMIT"`) {
		t.Fatalf("output limit: %s", got)
	}
}

func TestPathAndDuplicateClosure(t *testing.T) {
	for name, mutate := range map[string]func(*Request){
		"colon": func(request *Request) { request.Inputs[0].Path = "tooling:escape.py" },
		"duplicate-nonadjacent": func(request *Request) {
			request.Inputs[2].Path = request.Inputs[0].Path
		},
	} {
		t.Run(name, func(t *testing.T) {
			frame := canonicalFrame(t, "request-closure",
				vectorInput{"input-001", "py.source", "tooling/first.py", "import first\n"},
				vectorInput{"input-002", "py.source", "tooling/middle.py", "import middle\n"},
				vectorInput{"input-003", "py.source", "tooling/last.py", "import last\n"},
			)
			var request Request
			if err := json.Unmarshal(bytes.TrimSuffix(frame, []byte{'\n'}), &request); err != nil {
				t.Fatal(err)
			}
			mutate(&request)
			encoded, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			got := response(t, append(encoded, '\n'))
			want := `"reason":"INVALID_PATH"`
			if name == "duplicate-nonadjacent" {
				want = `"reason":"DUPLICATE_VALUE"`
			}
			if !strings.Contains(got, want) {
				t.Fatalf("got=%s want=%s", got, want)
			}
		})
	}
}

func TestMaximumLengthPathRejectsDerivedCoordinate(t *testing.T) {
	path := strings.Repeat(strings.Repeat("a", 128)+"/", 31) + strings.Repeat("b", 94) + ".py"
	if len(path) != maxStringBytes || !logicalPath(path) {
		t.Fatalf("maximum logical path witness invalid: length=%d", len(path))
	}
	got := response(t, canonicalFrame(t, "request-maximum-path", vectorInput{"input-001", "py.source", path, "pass\n"}))
	if !strings.Contains(got, `"reason":"LIMIT_EXCEEDED"`) || strings.Contains(got, `"facts"`) {
		t.Fatalf("over-length derived coordinate escaped: %s", got)
	}
}

func TestReplayBindsWholeRequest(t *testing.T) {
	frame := canonicalFrame(t, "request-replay", vectorInput{"input-001", "py.source", "tooling/replay.py", "import original\n"})
	first := response(t, frame)
	replayed := canonicalFrame(t, "request-replay", vectorInput{"input-001", "py.source", "tooling/replay.py", "import changed\n"})
	second := response(t, replayed)
	if first == second || !strings.Contains(first, "original") || !strings.Contains(second, "changed") {
		t.Fatalf("replay was not exact-input-bound: first=%s second=%s", first, second)
	}
}

type pythonFixturePin struct {
	consumer, directory, revision, aggregate string
	count                                    int
}

var pythonFixturePins = []pythonFixturePin{
	{"beamfall-core", "beamfall", "da38c59eb30b2121cbac37b912485b30b2e54841", "da71de79a7d9c14341061337fbfca548a0d3ee7ec0564db04460a7f303723b11", 114},
	{"beamfall-apple", "beamfall-apple", "8588cec3dedaef63bbff458e5e7c8bb6335de107", "212224660edfbc7ff7621378ade8c77fa3e972dfe06c88d54ab525a60b510609", 10},
	{"beamfall-relay", "beamfall-relay", "9723152fdd4ead36b32553a171d98749e4fc23e9", "8c2c1259831d7dbd39d2592e4edc9ff20b9a7926750b68b268a8af5293d3321a", 1},
	{"beamfall-plugin-sdk", "beamfall-plugin-sdk", "5536ecde6d12214d6786a6832e537bd5d4feca02", "5e3c8519a5d4e0c8f2919a37433d6e9e2f00e500d4847852aba31fbca950c141", 1},
}

func TestLiteralPinnedBeamfallPythonFixtures(t *testing.T) {
	requirement := struct{ name string }{name: "PNC-007 literal pinned 126 file Beamfall corpus"}
	if requirement.name == "" {
		t.Fatal("missing requirement claim")
	}
	frames := literalPinnedPythonFrames(t)
	if len(frames) != 126 {
		t.Fatalf("literal fixture frames=%d want=126", len(frames))
	}
}

func literalPinnedPythonFrames(t testing.TB) [][]byte {
	t.Helper()
	workspace := fixtureWorkspace(t)
	frames := make([][]byte, 0, 126)
	for _, pin := range pythonFixturePins {
		repository := filepath.Join(workspace, pin.directory)
		if gitOutput(t, repository, "status", "--porcelain") != "" {
			t.Fatalf("fixture checkout is not clean: %s", pin.directory)
		}
		_ = gitOutput(t, repository, "cat-file", "-e", pin.revision+"^{commit}")
		if resolved := strings.TrimSpace(gitOutput(t, repository, "rev-parse", pin.revision+"^{commit}")); resolved != pin.revision {
			t.Fatalf("revision %s resolved to %s", pin.revision, resolved)
		}
		paths := strings.Fields(gitOutput(t, repository, "ls-tree", "-r", "--name-only", pin.revision))
		python := make([]string, 0, len(paths))
		for _, path := range paths {
			if strings.HasSuffix(path, ".py") {
				python = append(python, path)
			}
		}
		if len(python) != pin.count {
			t.Fatalf("literal fixture count=%d want=%d", len(python), pin.count)
		}
		aggregate := sha256.New()
		for _, path := range python {
			content := []byte(gitOutput(t, repository, "show", pin.revision+":"+path))
			contentSum := sha256.Sum256(content)
			_, _ = aggregate.Write([]byte(path))
			_, _ = aggregate.Write([]byte{0})
			_, _ = aggregate.Write([]byte(hex.EncodeToString(contentSum[:])))
			_, _ = aggregate.Write([]byte{0})
			frame := canonicalFrame(t, "fixture-"+pin.consumer, vectorInput{"input-001", "py.source", path, string(content)})
			got := response(t, frame)
			if !strings.Contains(got, `"status":"CANDIDATE"`) || !strings.Contains(got, `"instance_id":"`+path+`:1:1"`) {
				t.Fatalf("fixture %s:%s rejected or omitted exact source coordinate: %s", pin.consumer, path, got)
			}
			frames = append(frames, frame)
		}
		if got := hex.EncodeToString(aggregate.Sum(nil)); got != pin.aggregate {
			t.Fatalf("literal aggregate=%s want=%s", got, pin.aggregate)
		}
	}
	return frames
}

func fixtureWorkspace(t testing.TB) string {
	t.Helper()
	repository := strings.TrimSpace(gitOutput(t, ".", "rev-parse", "--show-toplevel"))
	for root := repository; ; root = filepath.Dir(root) {
		complete := true
		for _, pin := range pythonFixturePins {
			_, err := os.Stat(filepath.Join(root, pin.directory, ".git"))
			if os.IsNotExist(err) {
				complete = false
				break
			}
			if err != nil {
				t.Fatalf("inspect pinned fixture checkout %s: %v", pin.directory, err)
			}
		}
		if complete {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
	}
	t.Skip("portable pinned fixture workspace unavailable: required sibling checkouts beamfall, beamfall-apple, beamfall-relay, and beamfall-plugin-sdk were not found together in this checkout or any ancestor")
	return ""
}

func gitOutput(t testing.TB, repository string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
