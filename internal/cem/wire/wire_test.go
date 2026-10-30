package wire

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestStrictJSONRejections(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"duplicate-key", `{"a":1,"a":2}`},
		{"nested-duplicate-key", `{"a":{"b":1,"b":2}}`},
		{"float", `{"a":1.5}`},
		{"exponent", `{"a":1e3}`},
		{"negative", `{"a":-1}`},
		{"leading-zero", `{"a":01}`},
		{"overflow", `{"a":9007199254740992}`},
		{"lone-high-surrogate", `{"a":"\ud800"}`},
		{"lone-low-surrogate", `{"a":"\udc00"}`},
		{"raw-control", "{\"a\":\"\x01\"}"},
		{"raw-nul", "{\"a\":\"\x00\"}"},
		{"high-surrogate-before-non-low", `{"a":"\ud800\u0041"}`},
		{"trailing", `{} `},
		{"trailing-garbage", `{}x`},
		{"bom", "\xef\xbb\xbf{}"},
		{"invalid-utf8", "{\"a\":\"\xff\"}"},
		{"empty", ``},
	}
	for _, testCase := range cases {
		if testCase.name == "trailing" {
			// Trailing whitespace alone is legal JSON; only non-space bytes reject.
			if _, err := Parse([]byte(testCase.input)); err != nil {
				t.Errorf("%s: rejected legal trailing whitespace: %v", testCase.name, err)
			}
			continue
		}
		if _, err := Parse([]byte(testCase.input)); err == nil {
			t.Errorf("%s: accepted", testCase.name)
		}
	}
}

func TestSurrogatePairAccepted(t *testing.T) {
	value, err := Parse([]byte(`{"a":"😀"}`))
	if err != nil {
		t.Fatal(err)
	}
	member, _ := value.Obj.Get("a")
	if member.Str != "😀" {
		t.Fatalf("decoded %q", member.Str)
	}
}

// TestMaxWireIntegerAccepted pins the inclusive top of the wire integer range
// 0..9007199254740991 from ALGORITHMS.md; "overflow" rejects the next value.
func TestMaxWireIntegerAccepted(t *testing.T) {
	value, err := Parse([]byte(`{"a":9007199254740991}`))
	if err != nil {
		t.Fatal(err)
	}
	member, _ := value.Obj.Get("a")
	if member.Kind != KindInt || member.Int != MaxWireInteger {
		t.Fatalf("decoded %+v", member)
	}
}

func TestDepthBound(t *testing.T) {
	deep := ""
	for range 70 {
		deep += "["
	}
	for range 70 {
		deep += "]"
	}
	if _, err := Parse([]byte(deep)); err == nil {
		t.Fatal("accepted 70-deep nesting")
	}
	shallow := `{"a":[[[[[[1]]]]]]}`
	if _, err := Parse([]byte(shallow)); err != nil {
		t.Fatalf("rejected shallow nesting: %v", err)
	}
	// The bound is inclusive: nesting of exactly maxDepth brackets around a
	// leaf value must still parse.
	atBound := strings.Repeat("[", maxDepth) + "1" + strings.Repeat("]", maxDepth)
	if _, err := Parse([]byte(atBound)); err != nil {
		t.Fatalf("rejected nesting of exactly maxDepth: %v", err)
	}
}

func TestPathGrammar(t *testing.T) {
	valid := []string{"a", "a/b", "docs/rule.txt", "src/café.txt", ".corvint/change.cem.json", "...", "a..b"}
	for _, path := range valid {
		if err := ValidatePath(path); err != nil {
			t.Errorf("%q rejected: %v", path, err)
		}
	}
	invalid := []string{"", "/a", "a//b", "a/", "./a", "a/./b", "..", "a/../b", "a\\b", "a\x00b", "a\x1fb", "a\x7fb"}
	for _, path := range invalid {
		if err := ValidatePath(path); err == nil {
			t.Errorf("%q accepted", path)
		}
	}
}

func TestCanonicalStringEscapes(t *testing.T) {
	cases := map[string]string{
		"plain":       `"plain"`,
		"café":        `"café"`,
		"a/b":         `"a/b"`,
		"q\"w":        `"q\"w"`,
		"back\\slash": `"back\\slash"`,
		"tab\there":   `"tab\there"`,
		"nl\nhere":    `"nl\nhere"`,
		"\x01":        `"\u0001"`,
		"\x1f":        `"\u001f"`,
	}
	for input, want := range cases {
		if got := CanonicalString(input); got != want {
			t.Errorf("CanonicalString(%q) = %s, want %s", input, got, want)
		}
	}
}

func replaceOnce(text, marker, replacement string) string {
	for index := 0; index+len(marker) <= len(text); index++ {
		if text[index:index+len(marker)] == marker {
			return text[:index] + replacement + text[index+len(marker):]
		}
	}
	return text
}

// parseMechanics runs the shared validators a cem/1.0 map inherits from the
// retired 0.x wire (ParseStable calls the same pair) without the stable
// envelope; fullWitnesses false is the candidate profile's admission.
func parseMechanics(data []byte, fullWitnesses bool) (*Map, error) {
	root, err := Parse(data)
	if err != nil {
		return nil, err
	}
	result := &Map{Spec: StableSpec}
	if err := validateCommonCapabilities(root.Obj, result, true, fullWitnesses); err != nil {
		return nil, err
	}
	if err := validateReferences(result); err != nil {
		return nil, err
	}
	return result, nil
}

// TestInheritedExcludedPath pins the excludedPath literal the inherited
// mechanics require.
func TestInheritedExcludedPath(t *testing.T) {
	base := `{"baseRevision":"4ca153370afd9bd8c6034ad73acc3925150ab681",` +
		`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c",` +
		`"excludedPath":%s,"evidence":[],"hunks":[{"id":"hunk:sha256:07461a992e03e064986720e365dc4bb477da7cefe73da853f51bcb70e2c3100c",` +
		`"path":"src/app.py","oldRange":{"start":1,"count":2},"newRange":{"start":1,"count":2},` +
		`"disposition":"unknown","reason":"no-evidence","basis":[]}]}`
	render := func(excluded string) []byte { return []byte(replaceOnce(base, "%s", excluded)) }
	if _, err := parseMechanics(render(`".corvint/change.cem.json"`), true); err != nil {
		t.Fatalf("exact excludedPath rejected: %v", err)
	}
	for _, excluded := range []string{`"custom.json"`, `".Corvint/change.cem.json"`, `".corvint/*.json"`, `[".corvint/change.cem.json"]`, `null`} {
		_, err := parseMechanics(render(excluded), true)
		if cemcode.CodeOf(err) != cemcode.InvalidExcludedPath {
			t.Errorf("excludedPath %s: got %v, want code %s", excluded, err, cemcode.InvalidExcludedPath)
		}
	}
}

// TestInheritedStructuralReasons pins the structural mechanical vocabulary
// (CEM-SM-001): the cem/1.0 mechanics accept it and the candidate profile,
// which admits no witnesses, rejects it.
func TestInheritedStructuralReasons(t *testing.T) {
	render := func(reason string) []byte {
		return []byte(`{"baseRevision":"4ca153370afd9bd8c6034ad73acc3925150ab681",` +
			`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c","excludedPath":".corvint/change.cem.json",` +
			`"evidence":[],"hunks":[{"id":"hunk:sha256:07461a992e03e064986720e365dc4bb477da7cefe73da853f51bcb70e2c3100c",` +
			`"path":"src/a.go","oldRange":{"start":1,"count":2},"newRange":{"start":1,"count":2},` +
			`"disposition":"mechanical","reason":"` + reason + `","basis":[]}]}`)
	}
	for reason := range StructuralReasons {
		if _, err := parseMechanics(render(reason), true); err != nil {
			t.Errorf("%s: %v", reason, err)
		}
		if _, err := parseMechanics(render(reason), false); cemcode.CodeOf(err) != cemcode.InvalidField {
			t.Errorf("candidate %s: got %v, want invalid-field", reason, err)
		}
		if !MechanicalReason(reason) {
			t.Errorf("MechanicalReason(%s) = false", reason)
		}
	}
	if _, err := parseMechanics(render("whitespace-only"), false); err != nil {
		t.Errorf("whitespace-only: %v", err)
	}
	if MechanicalReason("no-evidence") || !MechanicalReason("line-ending-only") {
		t.Error("MechanicalReason vocabulary drifted")
	}
}
