package typescript

import "testing"

// TJAA-V0-101: a declared dirty path is named by its whole path, a trailing part closed by a
// quote, or the path or an ancestor directory opened by a quote, `./`, `../` or `}/`; a route, a
// different directory or a longer sibling path is not a name of it.
func TestDeclaredPathNamed_TJAA_V0_101(t *testing.T) {
	const declared = "docs/contracts/_setup.json"
	for text, want := range map[string]string{
		`readFileSync("docs/contracts/_setup.json")`:          declared,
		`import data from "../../docs/contracts/_setup.json"`: declared,
		`join(dir, '_setup.json')`:                            "_setup.json",
		"readdirSync(`docs/contracts`)":                       "docs/contracts",
		`readdirSync("./docs")`:                               "docs",
		"glob(`${root}/docs/**/*.json`)":                      "docs",
		`readFileSync("docs/" + name)`:                        "docs",
		`page.goto("/docs/contracts")`:                        "",
		`const other = "src/docs"`:                            "",
		`readFileSync("docs/other/file.json")`:                "",
		`readFileSync("docs/contracts/_setup.json.bak")`:      "",
		`// docs/contracts/_setup.json without quotes`:        "",
	} {
		offset, spelling := declaredPathNamed(text, declared)
		if spelling != want || (want == "") != (offset < 0) {
			t.Errorf("%s: offset=%d spelling=%q want %q", text, offset, spelling, want)
		}
	}
}

// TJAA-V0-101: string escapes are decoded before matching, an escaped quote neither opens nor
// closes a spelling, and a quote closes only the spelling its own delimiter opened. The offset maps
// back to the raw byte the spelling starts at.
func TestDeclaredPathNamedDecodesEscapes_TJAA_V0_101(t *testing.T) {
	const declared = "docs/contracts/_setup.json"
	for text, want := range map[string]struct {
		spelling string
		offset   int
	}{
		`x; readFileSync("docs/contracts/_setup\u002ejson")`:  {declared, 17},
		`x; readFileSync("docs/contracts/_setup\x2ejson")`:    {declared, 17},
		`x; readFileSync("docs/contracts/_setup\u{2e}json")`:  {declared, 17},
		`x; readFileSync("\u0064ocs/contracts/_setup.json")`:  {declared, 17},
		`x; readFileSync("docs\/contracts\/_setup.json")`:     {declared, 17},
		`x; readFileSync('docs/contracts/_setup.json".bak')`:  {"", -1},
		`x; readFileSync("docs/contracts/_setup.json\".bak")`: {"", -1},
		`x; readFileSync('docs/contracts/_setup.json\'.bak')`: {"", -1},
		"x; readFileSync(\"docs/contracts/\\\n_setup.json\")": {declared, 17},
	} {
		decoded, raw := decodeStringEscapes(text)
		offset, spelling := declaredPathNamed(decoded, declared)
		if offset >= 0 {
			offset = raw(offset)
		}
		if spelling != want.spelling || offset != want.offset {
			t.Errorf("%s: offset=%d spelling=%q want %+v", text, offset, spelling, want)
		}
	}
}
