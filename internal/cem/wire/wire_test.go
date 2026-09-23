package wire

import (
	"encoding/json"
	"os"
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

func readFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestFrozenValidMapsParse(t *testing.T) {
	root := repoRoot(t)
	fixtures := []string{
		filepath.Join(root, "conformance", "cem-0.1", "valid.cem.json"),
		filepath.Join(root, "interop", "cem-0.1", "maps", "valid", "supported-sha1.json"),
		filepath.Join(root, "interop", "cem-0.1", "maps", "valid", "supported-sha256.json"),
		filepath.Join(root, "interop", "cem-0.1", "maps", "valid", "topology.json"),
		filepath.Join(root, "interop", "cem-0.1", "maps", "valid", "whitespace.json"),
		filepath.Join(root, "interop", "cem-0.1", "maps", "valid", "line-ending.json"),
		filepath.Join(root, "interop", "cem-0.1", "maps", "valid", "bytes.json"),
		filepath.Join(root, "interop", "cem-0.1", "maps", "valid", "overlap-sha1.json"),
	}
	for _, fixture := range fixtures {
		parsed, err := ParseMap(readFixture(t, fixture))
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(fixture), err)
			continue
		}
		if parsed.Spec != Spec01 {
			t.Errorf("%s: spec %q", filepath.Base(fixture), parsed.Spec)
		}
	}
}

// TestGoldenEvidenceIdentities recomputes every evidence ID in the frozen valid
// maps from its own fields; these are the mandatory canonical-encoding vectors.
func TestGoldenEvidenceIdentities(t *testing.T) {
	root := repoRoot(t)
	pattern := filepath.Join(root, "interop", "cem-0.1", "maps", "valid", "*.json")
	fixtures, err := filepath.Glob(pattern)
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("no valid map fixtures: %v", err)
	}
	total := 0
	for _, fixture := range fixtures {
		parsed, err := ParseMap(readFixture(t, fixture))
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(fixture), err)
		}
		for _, record := range parsed.Evidence {
			derived := EvidenceIdentity(record.BlobOid, record.Path, record.Span, record.SpanSha256)
			if derived != record.ID {
				t.Errorf("%s: evidence ID drift: derived %s recorded %s", filepath.Base(fixture), derived, record.ID)
			}
			total++
		}
	}
	if total == 0 {
		t.Fatal("no evidence identities exercised")
	}
}

func TestStructuralInvalidMapsReject(t *testing.T) {
	root := repoRoot(t)
	cases := []struct {
		fixture string
		code    string
	}{
		{"duplicate-key.json", cemcode.InvalidJSON},
		{"unknown-field.json", cemcode.UnknownField},
		// unknown-spec.json carries spec cem/0.2; an upgraded consumer rejects it
		// as a malformed 0.2 document (missing excludedPath), which is a valid
		// rejection under the advisory-code rule.
		{"unknown-spec.json", cemcode.MissingField},
		{"path-traversal.json", cemcode.PathTraversal},
		{"basis.json", cemcode.UnsupportedWithoutBasis},
		{"orphan.json", cemcode.OrphanEvidence},
		{"resource-integer.json", ""},
		{"resource-path.json", ""},
		// range.json mutates oldRange only; its rejection is hunk-identity
		// recomputation during verification, not wire shape.
	}
	for _, testCase := range cases {
		data := readFixture(t, filepath.Join(root, "interop", "cem-0.1", "maps", "invalid", testCase.fixture))
		_, err := ParseMap(data)
		if err == nil {
			t.Errorf("%s: accepted", testCase.fixture)
			continue
		}
		if testCase.code != "" && cemcode.CodeOf(err) != testCase.code {
			t.Errorf("%s: code %q, want %q (%v)", testCase.fixture, cemcode.CodeOf(err), testCase.code, err)
		}
	}
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

func TestSpec02ExcludedPath(t *testing.T) {
	base := `{"spec":"cem/0.2","baseRevision":"4ca153370afd9bd8c6034ad73acc3925150ab681",` +
		`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c",` +
		`"excludedPath":%s,"evidence":[],"hunks":[{"id":"hunk:sha256:07461a992e03e064986720e365dc4bb477da7cefe73da853f51bcb70e2c3100c",` +
		`"path":"src/app.py","oldRange":{"start":1,"count":2},"newRange":{"start":1,"count":2},` +
		`"disposition":"unknown","reason":"no-evidence","basis":[]}]}`
	render := func(excluded string) []byte {
		document := base
		return []byte(replaceOnce(document, "%s", excluded))
	}
	if _, err := ParseMap(render(`".corvint/change.cem.json"`)); err != nil {
		t.Fatalf("exact excludedPath rejected: %v", err)
	}
	rejects := map[string]string{
		`"custom.json"`:                cemcode.InvalidExcludedPath,
		`".Corvint/change.cem.json"`:   cemcode.InvalidExcludedPath,
		`".corvint/*.json"`:            cemcode.InvalidExcludedPath,
		`[".corvint/change.cem.json"]`: cemcode.InvalidExcludedPath,
		`null`:                         cemcode.InvalidExcludedPath,
	}
	for excluded, code := range rejects {
		_, err := ParseMap(render(excluded))
		if err == nil || cemcode.CodeOf(err) != code {
			t.Errorf("excludedPath %s: got %v, want code %s", excluded, err, code)
		}
	}
	missing := `{"spec":"cem/0.2","baseRevision":"4ca153370afd9bd8c6034ad73acc3925150ab681",` +
		`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c",` +
		`"evidence":[],"hunks":[]}`
	_, err := ParseMap([]byte(missing))
	if err == nil || cemcode.CodeOf(err) != cemcode.MissingField {
		t.Errorf("missing excludedPath: got %v, want missing-field", err)
	}
}

// TestMissingSpecPrecedence proves a missing spec fails before unknown-field
// evaluation, per the frozen stage-2 precedence.
func TestMissingSpecPrecedence(t *testing.T) {
	document := `{"surplus":true,"baseRevision":"4ca153370afd9bd8c6034ad73acc3925150ab681"}`
	_, err := ParseMap([]byte(document))
	if err == nil || cemcode.CodeOf(err) != cemcode.MissingField {
		t.Fatalf("got %v, want missing-field for spec", err)
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

// TestConformance02CasesConsumed drives the structural 0.2 mutation suite
// directly from the frozen conformance/cem-0.2/cases.json fixture, so corpus
// drift or a schema regression fails this test rather than going unnoticed.
func TestConformance02CasesConsumed(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "conformance", "cem-0.2", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var suite struct {
		ValidAssurance string `json:"validAssurance"`
		Invalid        []struct {
			Name          string `json:"name"`
			Operation     string `json:"operation"`
			Value         any    `json:"value"`
			ReferenceCode string `json:"referenceCode"`
		} `json:"invalid"`
	}
	if err := json.Unmarshal(raw, &suite); err != nil {
		t.Fatal(err)
	}
	// The frozen structural assurance label is bound by CEM-CB-014; the
	// workflow envelope tests assert the same literal on the wire.
	if suite.ValidAssurance != "structural-only" {
		t.Fatalf("validAssurance drifted: %q", suite.ValidAssurance)
	}
	// Closed vector list (CEM-CB-006/-021/-EX-003/-EX-004): pinned by name and
	// count so a dropped or renamed vector fails here instead of leaving the
	// loop below silently short one case.
	expectedNames := []string{
		"missing-excluded-path", "custom-excluded-path", "glob-excluded-path",
		"array-excluded-path", "surplus-field",
	}
	if len(suite.Invalid) != len(expectedNames) {
		t.Fatalf("invalid vector count = %d, want %d %v", len(suite.Invalid), len(expectedNames), expectedNames)
	}
	for index, vector := range suite.Invalid {
		if vector.Name != expectedNames[index] {
			t.Fatalf("invalid vector %d = %q, want %q", index, vector.Name, expectedNames[index])
		}
	}
	base := func() map[string]any {
		return map[string]any{
			"spec":         "cem/0.2",
			"baseRevision": "4ca153370afd9bd8c6034ad73acc3925150ab681",
			"patchSha256":  "dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c",
			"excludedPath": ".corvint/change.cem.json",
			"evidence":     []any{},
			"hunks": []any{map[string]any{
				"id":          "hunk:sha256:07461a992e03e064986720e365dc4bb477da7cefe73da853f51bcb70e2c3100c",
				"path":        "src/app.py",
				"oldRange":    map[string]any{"start": 1, "count": 2},
				"newRange":    map[string]any{"start": 1, "count": 2},
				"disposition": "unknown",
				"reason":      "no-evidence",
				"basis":       []any{},
			}},
		}
	}
	valid, err := json.Marshal(base())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseMap(valid); err != nil {
		t.Fatalf("unmutated base document rejected: %v", err)
	}
	for _, vector := range suite.Invalid {
		document := base()
		switch vector.Operation {
		case "remove":
			delete(document, "excludedPath")
		case "set":
			document["excludedPath"] = vector.Value
		case "surplus":
			document["surplus"] = true
		default:
			t.Fatalf("%s: unknown operation %q", vector.Name, vector.Operation)
		}
		encoded, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		_, parseErr := ParseMap(encoded)
		if cemcode.CodeOf(parseErr) != vector.ReferenceCode {
			t.Errorf("%s: got %v, want code %s", vector.Name, parseErr, vector.ReferenceCode)
		}
	}
}

// TestSpec03StructuralReasons pins the additive cem/0.3 vocabulary: 0.3
// accepts the structural reasons, 0.2 and 0.1 still reject them, and 0.3
// keeps the 0.2 excludedPath obligation (CEM-SM-001).
func TestSpec03StructuralReasons(t *testing.T) {
	render := func(spec, reason, excluded string) []byte {
		return []byte(`{"spec":"` + spec + `","baseRevision":"4ca153370afd9bd8c6034ad73acc3925150ab681",` +
			`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c",` + excluded +
			`"evidence":[],"hunks":[{"id":"hunk:sha256:07461a992e03e064986720e365dc4bb477da7cefe73da853f51bcb70e2c3100c",` +
			`"path":"src/a.go","oldRange":{"start":1,"count":2},"newRange":{"start":1,"count":2},` +
			`"disposition":"mechanical","reason":"` + reason + `","basis":[]}]}`)
	}
	excluded := `"excludedPath":".corvint/change.cem.json",`
	for reason := range StructuralReasons {
		if _, err := ParseMap(render(Spec03, reason, excluded)); err != nil {
			t.Errorf("cem/0.3 %s: %v", reason, err)
		}
		for _, spec := range []string{Spec01, Spec02} {
			path := excluded
			if spec == Spec01 {
				path = ""
			}
			_, err := ParseMap(render(spec, reason, path))
			if err == nil || cemcode.CodeOf(err) != cemcode.InvalidField {
				t.Errorf("%s %s: got %v, want invalid-field", spec, reason, err)
			}
		}
	}
	if _, err := ParseMap(render(Spec03, "whitespace-only", excluded)); err != nil {
		t.Errorf("cem/0.3 whitespace-only: %v", err)
	}
	_, err := ParseMap(render(Spec03, "rename", ""))
	if err == nil || cemcode.CodeOf(err) != cemcode.MissingField {
		t.Errorf("cem/0.3 without excludedPath: got %v, want missing-field", err)
	}
	if !MechanicalReason(Spec03, "move") || MechanicalReason(Spec02, "move") || !MechanicalReason(Spec01, "whitespace-only") {
		t.Error("MechanicalReason vocabulary is not gated by spec")
	}
}
