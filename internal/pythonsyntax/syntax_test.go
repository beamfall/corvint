package pythonsyntax

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GPK-V0-014 requires Go claim consumers to abstain outside frozen Python syntax.
func TestGPKV0014SourceSyntaxValidIsNeverMorePermissiveThanPython(t *testing.T) {
	requirement := struct{ name string }{name: "GPK-V0-014 conservative Python syntax validity"}
	if requirement.name == "" {
		t.Fatal("missing requirement claim")
	}
	cases := []struct {
		name, source string
		pythonValid  bool
		goValid      bool
		minMinor     int
	}{
		{name: "assignment", source: "x = 1\n", pythonValid: true, goValid: true},
		{name: "negative", source: "x = -1\n", pythonValid: true, goValid: true},
		{name: "double-unary", source: "x = +-1\n", pythonValid: true, goValid: true},
		{name: "unary-then-binary-operator", source: "x = +/ 2\n"},
		{name: "star-argument", source: "target(*items)\n", pythonValid: true, goValid: true},
		{name: "star-import", source: "from package import *\n", pythonValid: true, goValid: true},
		{name: "function", source: "def ok():\n    pass\n", pythonValid: true, goValid: true},
		{name: "missing-final-newline", source: "x = 1", pythonValid: true, goValid: true},
		{name: "missing-final-newline-suite", source: "def f():\n    return 1", pythonValid: true, goValid: true},
		{name: "missing-final-newline-nested-suite", source: "if ready:\n    if other:\n        pass", pythonValid: true, goValid: true},
		{name: "missing-final-newline-leading-slash", source: "x = / 2"},
		{name: "missing-final-newline-adjacent-operands", source: "x = 1 2"},
		{name: "missing-final-newline-unclosed-group", source: "x = (1"},
		{name: "carriage-return", source: "x = 1\r\n", pythonValid: true},
		{name: "dangling-unary", source: "x = +\n"},
		{name: "nested-dangling", source: "x = [+]\n"},
		{name: "keyword-dangling", source: "return +\n"},
		{name: "invalid-class", source: "class 3bad:\n    pass\n"},
		{name: "invalid-function", source: "def f(:\n"},
		{name: "unclosed-group", source: "x = (1\n"},
		{name: "leading-slash", source: "x = / 2\n"},
		{name: "nested-leading-operator", source: "x = (/ 2)\n"},
		{name: "dangling-span-expression", source: "x = (value +)\n"},
		{name: "missing-span-comma", source: "x = (first second)\n"},
		{name: "dictionary-missing-colon", source: "x = {'key' value}\n"},
		{name: "mixed-bytes-and-string-literals", source: "x = [b'bytes' 'text']\n"},
		{name: "keyword-value-walrus", source: "call(key=value := 1)\n"},
		{name: "call-positional-after-keyword", source: "call(key=1, 2)\n"},
		{name: "nested-leading-slash", source: "if ready:\n    x = / 2\n"},
		{name: "inline-definition-leading-slash", source: "def invalid(): x = / 2\n"},
		{name: "leading-keyword-operator", source: "x = and value\n"},
		{name: "leading-not", source: "x = not y\n", pythonValid: true, goValid: true},
		{name: "leading-dot-number", source: "x = .5\n", pythonValid: true, goValid: true},
		{name: "starred-rhs", source: "x = *items,\n", pythonValid: true, goValid: true},
		{name: "adjacent-operands", source: "x = 1 2\n"},
		{name: "adjacent-group-operand", source: "x = (1) 2\n"},
		{name: "operand-brace-trailer", source: "x = 1 {2}\n"},
		{name: "valid-call-trailer", source: "x = value(1)\n", pythonValid: true, goValid: true},
		{name: "adjacent-strings", source: "x = 'a' 'b'\n", pythonValid: true, goValid: true},
		{name: "malformed-return", source: "return 1 2\n"},
		{name: "valid-return", source: "return 1\n", pythonValid: true, goValid: true},
		{name: "leading-comma-group", source: "return (,)\n"},
		{name: "valid-trailing-comma-group", source: "return (1,)\n", pythonValid: true, goValid: true},
		{name: "malformed-pass", source: "pass value\n"},
		{name: "malformed-break", source: "break value\n"},
		{name: "malformed-yield-from", source: "yield from\n"},
		{name: "malformed-raise-from", source: "raise Error from\n"},
		{name: "valid-global", source: "global first, second\n", pythonValid: true, goValid: true},
		{name: "valid-yield-from", source: "yield from items\n", pythonValid: true, goValid: true},
		{name: "valid-raise-from", source: "raise Error from cause\n", pythonValid: true, goValid: true},
		{name: "empty-if-header", source: "if:\n    pass\n"},
		{name: "adjacent-if-header", source: "if first second:\n    pass\n"},
		{name: "valid-if-header", source: "if value:\n    pass\n", pythonValid: true, goValid: true},
		{name: "missing-for-in", source: "for value:\n    pass\n"},
		{name: "invalid-for-target", source: "for x + y in values:\n    pass\n"},
		{name: "valid-for-target-tuple", source: "for a, b in pairs:\n    pass\n", pythonValid: true, goValid: true},
		{name: "valid-for-target-attribute", source: "for obj.attr in values:\n    pass\n", pythonValid: true, goValid: true},
		{name: "malformed-with-as", source: "with resource as:\n    pass\n"},
		{name: "malformed-except-header", source: "try:\n    pass\nexcept Error Other:\n    pass\n"},
		{name: "valid-compounds", source: "try:\n    pass\nexcept Error as error:\n    pass\nfinally:\n    pass\nfor value in values:\n    pass\nwith resource as value:\n    pass\n", pythonValid: true, goValid: true},
		{name: "soft-match-assignment", source: "match = 1\ncase = 2\n", pythonValid: true, goValid: true},
		{name: "malformed-decorator-arguments", source: "@decorator(,)\ndef f():\n    pass\n"},
		{name: "malformed-decorator-keyword", source: "@decorator(value=)\ndef f():\n    pass\n"},
		{name: "malformed-decorator-lambda", source: "@decorator(lambda:)\ndef f():\n    pass\n"},
		{name: "valid-decorator-arguments", source: "@decorator(x=1, *args, **kwargs)\ndef f():\n    pass\n", pythonValid: true, goValid: true},
		{name: "decorator-positional-after-keyword", source: "@decorator(x=1, 2)\ndef f():\n    pass\n"},
		{name: "valid-decorator-walrus", source: "@decorator((value := 1))\ndef f():\n    pass\n", pythonValid: true, goValid: true},
		{name: "valid-decorator-generator", source: "@decorator(value for value in values)\ndef f():\n    pass\n", pythonValid: true, goValid: true},
		{name: "malformed-type-alias-rhs", source: "type Alias = / 2\n"},
		{name: "nested-type-alias-rhs", source: "type Alias = (/ 2)\n"},
		{name: "adjacent-type-alias-rhs", source: "type Alias = first second\n"},
		{name: "valid-type-alias-rhs", source: "type Alias = list[int]\n", pythonValid: true, goValid: true, minMinor: 12},
		{name: "dangling-if-else", source: "type Alias = first if else\n"},
		{name: "valid-ternary", source: "x = 1 if True else 2\n", pythonValid: true, goValid: true},
		{name: "malformed-fstring-field", source: "x = f'{1 2}'\n"},
		{name: "leading-fstring-field", source: "x = f'{/ 2}'\n"},
		{name: "empty-fstring-field", source: "x = f'{}'\n"},
		{name: "malformed-fstring-conversion", source: "x = f'{value!}'\n"},
		{name: "dangling-fstring-operator", source: "x = f'{value +}'\n"},
		{name: "valid-fstring-field", source: "x = f'{value!r:>10}'\n", pythonValid: true, goValid: true},
		{name: "valid-fstring-debug-field", source: "x = f'{value=}'\n", pythonValid: true, goValid: true, minMinor: 8},
		{name: "valid-fstring-walrus", source: "x = f'{(value := 1)}'\n", pythonValid: true, goValid: true, minMinor: 8},
		{name: "valid-fstring-string-close", source: "x = f\"{'}'}\"\n", pythonValid: true, goValid: true, minMinor: 12},
		{name: "walrus-if-header", source: "if match := pattern.match(line):\n    pass\n", pythonValid: true, goValid: true, minMinor: 8},
		{name: "walrus-while-header", source: "while chunk := stream.read(8):\n    pass\n", pythonValid: true, goValid: true, minMinor: 8},
		{name: "walrus-call-argument", source: "total(count := 1)\n", pythonValid: true, goValid: true, minMinor: 8},
		{name: "walrus-subscript", source: "values[index := 1]\n", pythonValid: true, goValid: true, minMinor: 12},
		{name: "walrus-empty-value", source: "if value := :\n    pass\n"},
		{name: "walrus-empty-target", source: "if := value:\n    pass\n"},
		{name: "walrus-starred-value", source: "g(value := *values)\n"},
		{name: "bare-walrus-statement", source: "value := 1\n"},
		{name: "fstring-nested-mapping", source: "x = f\"array of {render(schema, schema.get('items', {}))}\"\n", pythonValid: true, goValid: true},
		{name: "fstring-nested-set", source: "x = f\"{ {1, 2} }\"\n", pythonValid: true, goValid: true},
		{name: "fstring-nested-display-field", source: "x = f\"{render({1 2})}\"\n"},
		{name: "fstring-spec-nested-field", source: "x = f\"{value:{1 2}}\"\n"},
		{name: "fstring-format-spec-field", source: "x = f\"{1 2:}\"\n"},
		{name: "fstring-unparenthesized-lambda", source: "x = f\"{lambda value: value}\"\n"},
		{name: "fstring-parenthesized-lambda", source: "x = f\"{(lambda value: value)}\"\n", pythonValid: true, goValid: true},
		{name: "fstring-unclosed-field", source: "x = f\"{value\"\n"},
		{name: "fstring-align-spec", source: "x = f\"{value:=^10}\"\n", pythonValid: true, goValid: true},
		{name: "fstring-nested-same-quote", source: "x = f\"{d[\"k\"]}\"\n", pythonValid: true, minMinor: 12},
		{name: "fstring-lone-literal-close", source: "x = f\"literal } brace\"\n"},
		{name: "fstring-doubled-literal-close", source: "x = f\"literal }} brace\"\n", pythonValid: true, goValid: true},
		{name: "fstring-nested-spec-fields", source: "x = f\"{value:>{width}.{precision}f}\"\n", pythonValid: true, goValid: true},
		{name: "fstring-conversion-nested-spec", source: "x = f\"{value!s:{width}}\"\n", pythonValid: true, goValid: true},
		{name: "fstring-literal-spec-bracket", source: "x = f\"{throughput:(.1f}\"\n", pythonValid: true, goValid: true},
		{name: "fstring-nested-displays", source: "x = f\"{ {'k': [1, {2}]} }\"\n", pythonValid: true, goValid: true},
		{name: "walrus-parenthesized-target", source: "g((v) := v)\n"},
		{name: "walrus-attribute-target", source: "g(a.b := 1)\n"},
		{name: "walrus-tuple-target", source: "if first, second := value:\n    pass\n"},
		{name: "walrus-leading-slash-value", source: "if value := / 2:\n    pass\n"},
		{name: "walrus-argument-after-comma", source: "g(a, b := 1)\n", pythonValid: true, goValid: true, minMinor: 8},
		{name: "walrus-list-element", source: "x = [total := 1, total ** 2]\n", pythonValid: true, goValid: true, minMinor: 8},
		{name: "walrus-parenthesized-comparison", source: "if (count := len(items)) > 10:\n    pass\n", pythonValid: true, goValid: true, minMinor: 8},
		{name: "valid-slice", source: "x = values[1:]\n", pythonValid: true, goValid: true},
		{name: "malformed-number", source: "x = 1__0\n"},
		{name: "malformed-empty-hex", source: "x = 0x\n"},
		{name: "malformed-binary-number", source: "x = 0b102\n"},
		{name: "malformed-leading-zero", source: "x = 01\n"},
		{name: "malformed-exponent", source: "x = 1e\n"},
		{name: "malformed-float-underscore", source: "x = 1._0\n"},
		{name: "malformed-imaginary", source: "x = 1j2\n"},
		{name: "malformed-number-suffix", source: "x = 123abc\n"},
		{name: "malformed-double-exponent", source: "x = 1e2e3\n"},
		{name: "malformed-double-dot", source: "x = 1.2.3\n"},
		{name: "valid-number", source: "x = 1_000\n", pythonValid: true, goValid: true},
		{name: "valid-based-number", source: "x = 0x_FF\n", pythonValid: true, goValid: true},
		{name: "valid-zero-number", source: "x = 00\n", pythonValid: true, goValid: true},
		{name: "valid-signed-exponent", source: "x = 1e+2\n", pythonValid: true, goValid: true},
		{name: "dot-after-exponent", source: "x = 1e2.3\n"},
		{name: "valid-dot-before-exponent", source: "x = 1.5e10\n", pythonValid: true, goValid: true},
		{name: "valid-float-attribute", source: "x = 1..real\n", pythonValid: true, goValid: true},
	}
	conservative := false
	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			pythonValid := testcase.pythonValid
			goValid := SourceSyntaxValid("claim.py", []byte(testcase.source))
			if goValid && !pythonValid {
				t.Fatalf("Go accepted Python-rejected source %q", testcase.source)
			}
			if goValid != testcase.goValid {
				t.Fatalf("Go validity=%t want=%t for %q", goValid, testcase.goValid, testcase.source)
			}
			conservative = conservative || pythonValid && !goValid
		})
	}
	if !conservative {
		t.Fatal("corpus did not exercise conservative Go rejection")
	}
}

// TestGPKV0014ASTParityFixtures walks testdata/ast-parity and checks
// SourceSyntaxValid against a verdict recorded ahead of time by running
// `ast.parse` (see EXPECTED.tsv) — it does not invoke Python itself. Each
// fixture is named accept-*.py or reject-*.py; the name and the recorded
// verdict must agree, and Go must match the recorded verdict too.
func TestGPKV0014ASTParityFixtures(t *testing.T) {
	root := "testdata/ast-parity"
	expectedPath := filepath.Join(root, "EXPECTED.tsv")
	raw, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read %s: %v", expectedPath, err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) == 0 {
		t.Fatalf("%s is empty", expectedPath)
	}
	seen := 0
	for _, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) != 2 {
			t.Fatalf("malformed EXPECTED.tsv line %q", line)
		}
		name, want := fields[0], fields[1]
		if want != "ACCEPT" && want != "REJECT" {
			t.Fatalf("malformed verdict %q for %s", want, name)
		}
		wantFromName := "REJECT"
		if strings.HasPrefix(name, "accept-") {
			wantFromName = "ACCEPT"
		} else if !strings.HasPrefix(name, "reject-") {
			t.Fatalf("fixture %s must be named accept-*.py or reject-*.py", name)
		}
		if want != wantFromName {
			t.Fatalf("fixture %s name says %s but EXPECTED.tsv says %s", name, wantFromName, want)
		}
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			goValid := SourceSyntaxValid(name, source)
			got := "REJECT"
			if goValid {
				got = "ACCEPT"
			}
			if got != want {
				t.Fatalf("SourceSyntaxValid=%s want=%s (ast.parse verdict recorded in EXPECTED.tsv)", got, want)
			}
		})
		seen++
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read dir %s: %v", root, err)
	}
	fixtures := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".py" {
			fixtures++
		}
	}
	if fixtures != seen {
		t.Fatalf("EXPECTED.tsv covers %d fixtures but %s holds %d .py files", seen, root, fixtures)
	}
	if fixtures > 18 {
		t.Fatalf("ast-parity fixture directory holds %d files, want <= 18", fixtures)
	}
}
