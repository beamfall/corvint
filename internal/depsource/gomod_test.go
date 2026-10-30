package depsource

import "testing"

// DSE-V0-002: a quoted module path or version, a column-aligned directive,
// and a trailing "//" comment must each tokenize correctly instead of being
// split or truncated the way strings.Fields would split them.
func TestParseGoModQuotedAndAlignedDirectives(t *testing.T) {
	content := []byte(`module example.com/host

go 1.27.0

require "example.com/single path" v3.0.0 // single-line form
require(
	"example.com/quoted path"   v1.2.3 // inline comment
	example.com/plain           v0.1.0
)
replace "example.com/quoted path" v1.2.3 => "./local path" // single-line form
replace(
	example.com/plain => ` + "`../local path`" + `
)
`)
	parsed, err := parseGoMod(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Requires) != 3 {
		t.Fatalf("expected 3 requirements, got %+v", parsed.Requires)
	}
	if parsed.Requires[0] != (requirement{"example.com/single path", "v3.0.0"}) {
		t.Fatalf("single-line directive misparsed: %+v", parsed.Requires[0])
	}
	if parsed.Requires[1] != (requirement{"example.com/quoted path", "v1.2.3"}) {
		t.Fatalf("quoted, aligned, commented directive misparsed: %+v", parsed.Requires[1])
	}
	if parsed.Requires[2] != (requirement{"example.com/plain", "v0.1.0"}) {
		t.Fatalf("aligned directive misparsed: %+v", parsed.Requires[2])
	}
	if len(parsed.Replaces) != 2 || parsed.Replaces[0].NewPath != "./local path" ||
		parsed.Replaces[1].NewPath != "../local path" {
		t.Fatalf("replace forms misparsed: %+v", parsed.Replaces)
	}
}

// DSE-V0-002: exclude directives are modeled, both single-line and block form.
func TestParseGoModExclude(t *testing.T) {
	content := []byte(`module example.com/host

go 1.27.0

exclude example.com/dep v1.0.0

exclude (
	example.com/other v2.0.0
)
`)
	parsed, err := parseGoMod(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Excludes) != 2 {
		t.Fatalf("expected 2 exclusions, got %+v", parsed.Excludes)
	}
	if !parsed.excluded("example.com/dep", "v1.0.0") {
		t.Fatal("expected the single-line exclude to be recognized")
	}
	if !parsed.excluded("example.com/other", "v2.0.0") {
		t.Fatal("expected the block-form exclude to be recognized")
	}
	if parsed.excluded("example.com/dep", "v1.0.1") {
		t.Fatal("exclude must not match a different version")
	}
}

// DSE-V0-002: a line the restricted grammar cannot tokenize is refused. The
// prior reader consumed an unterminated literal to end of line, so a malformed
// require silently vanished and the module afterwards read as unrequired.
func TestParseGoModRefusesMalformedLiterals(t *testing.T) {
	for name, line := range map[string]string{
		"unterminated-quote":     `require "example.com/dep v1.0.0`,
		"unterminated-backquote": "require `example.com/dep v1.0.0",
		"literal-then-text":      `require "example.com/dep"tail v1.0.0`,
		"bad-escape":             `require "example.com/\q" v1.0.0`,
		"quoted-directive":       `"require" example.com/dep v1.0.0`,
		"quoted-block-close":     "require (\n\t\")\"",
		"quoted-replace-arrow":   `replace example.com/dep "=>" ./local`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseGoMod([]byte("module example.com/host\n\n" + line + "\n")); err == nil {
				t.Fatalf("expected a parse refusal for %q", line)
			}
		})
	}
}

// DSE-V0-002: strconv.Unquote decodes the literal, so a backquoted path and an
// escape sequence both resolve to the bytes the committed line names.
func TestParseGoModDecodesLiterals(t *testing.T) {
	content := []byte("module example.com/host\n\n" +
		"require `example.com/raw path` v1.0.0\n" +
		"require \"example.com/esc\\u00e9\" v2.0.0\n")
	parsed, err := parseGoMod(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Requires) != 2 {
		t.Fatalf("expected 2 requirements, got %+v", parsed.Requires)
	}
	if parsed.Requires[0] != (requirement{"example.com/raw path", "v1.0.0"}) {
		t.Fatalf("backquoted path misparsed: %+v", parsed.Requires[0])
	}
	if parsed.Requires[1] != (requirement{"example.com/escé", "v2.0.0"}) {
		t.Fatalf("escape sequence misparsed: %+v", parsed.Requires[1])
	}
}

// DSE-V0-002: unsupported directives, malformed directive shapes, and
// unbalanced blocks must fail closed instead of letting partial fields become
// resolution evidence.
func TestParseGoModRefusesUnsupportedSyntax(t *testing.T) {
	for name, content := range map[string]string{
		"unsupported-directive": "module example.com/host\nretract v1.0.0\n",
		"short-require":         "module example.com/host\nrequire example.com/dep\n",
		"long-exclude":          "module example.com/host\nexclude example.com/dep v1.0.0 extra\n",
		"malformed-replace":     "module example.com/host\nreplace example.com/dep v1.0.0 example.com/fork v1.0.1\n",
		"conflicting-replace":   "module example.com/host\nreplace example.com/dep v1.0.0 => example.com/one v1.0.1\nreplace example.com/dep v1.0.0 => example.com/two v1.0.2\n",
		"unterminated-block":    "module example.com/host\nrequire (\nexample.com/dep v1.0.0\n",
		"stray-block-close":     "module example.com/host\n)\n",
		"unsupported-block":     "module example.com/host\nretract (\nv1.0.0\n)\n",
		"nested-block":          "module example.com/host\nrequire (\nrequire (\nexample.com/dep v1.0.0\n)\n",
		"nested-declaration":    "module example.com/host\nrequire (\nmodule example.com/dep\n)\n",
		"block-comment":         "module example.com/host\n/* unsupported */\nrequire example.com/dep v1.0.0\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseGoMod([]byte(content)); err == nil {
				t.Fatalf("expected unsupported syntax to be refused:\n%s", content)
			}
		})
	}
}
