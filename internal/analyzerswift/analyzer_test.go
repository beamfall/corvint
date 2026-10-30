package analyzerswift

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func dogfoodBodies(t testing.TB) (coordinate, source, uiPackage []byte) {
	t.Helper()
	coordinate = mustRead(t, "testdata/beamfall-apple-8588/coordinates.txt")
	source = mustRead(t, "testdata/beamfall-apple-8588/Sources/BeamfallA11y/A11yID.swift")
	uiPackage = mustRead(t, "testdata/beamfall-apple-ui-830a/Package.swift")
	return coordinate, source, uiPackage
}

func mustRead(t testing.TB, path string) []byte {
	t.Helper()
	value, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fixtureRequest(t testing.TB, requestID string) Request {
	t.Helper()
	coordinate, source, uiPackage := dogfoodBodies(t)
	return Request{
		Profile: Profile, Family: Family, RequestID: requestID, ScopeID: "beamfall-apple", CompilationUnitID: "a11y",
		Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}},
		Inputs: []Input{
			fixtureInput("apple-coordinates", "swift.apple.coordinates", "fixtures/beamfall-apple-8588cec3/coordinates.txt", coordinate),
			fixtureInput("apple-source", "swift.source", "Sources/BeamfallA11y/A11yID.swift", source),
			fixtureInput("apple-ui", "swift.apple-ui.package", "Package.swift", uiPackage),
		},
	}
}

func fixtureInput(handle, family, path string, content []byte) Input {
	sum := sha256.Sum256(content)
	return Input{Handle: handle, Family: family, Path: path, SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString(content)}
}

func requestBytes(t testing.TB, request Request) []byte {
	t.Helper()
	value, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(value, '\n')
}

func TestAnalyzePinnedBeamfallAppleVector(t *testing.T) {
	request := fixtureRequest(t, "swift-apple-8588")
	got := Analyze(bytes.NewReader(requestBytes(t, request)))
	var result output
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "CANDIDATE" || len(result.Facts) != 8 {
		t.Fatalf("result=%s", got)
	}
	if result.Facts[0].Kind != "apple.project.revision" || result.Facts[0].Value != appleRevision {
		t.Fatalf("project fact=%+v", result.Facts[0])
	}
	var importFact, declarationFact Fact
	for _, fact := range result.Facts {
		switch fact.Kind {
		case "swift.import.static":
			importFact = fact
		case "swift.symbol.declaration":
			declarationFact = fact
		}
	}
	if importFact.Value != "SwiftUI" {
		t.Fatalf("import fact=%+v", importFact)
	}
	if declarationFact.Value != "A11yID" {
		t.Fatalf("declaration fact=%+v", declarationFact)
	}
	var extensionFact Fact
	for _, fact := range result.Facts {
		if fact.Kind == "swift.symbol.extension" {
			extensionFact = fact
		}
	}
	if extensionFact.Predicate != "extends" || extensionFact.Value != "View" {
		t.Fatalf("extension fact=%+v", extensionFact)
	}
	if got[len(got)-1] != '\n' {
		t.Fatalf("missing LF: %q", got)
	}
	const outputSHA256 = "f63f9eaae4481d69f1978083693e49bdde91c76ed68f1b45d50b95a756457d6c"
	if actual := fmt.Sprintf("%x", sha256.Sum256(got)); actual != outputSHA256 {
		t.Fatalf("pinned output SHA-256=%s", actual)
	}
}

func TestFixtureBindsGitBlobAndAppleUIBytes(t *testing.T) {
	coordinate, source, uiPackage := dogfoodBodies(t)
	if string(coordinate) != coordinateVector || gitBlob(source) != a11yBlob || gitBlob(uiPackage) != uiPackageBlob {
		t.Fatal("fixture binding drift")
	}
	if len(source) != 7673 || len(uiPackage) != 6189 {
		t.Fatalf("fixture sizes source=%d ui=%d", len(source), len(uiPackage))
	}
}

func TestAnalyzeRejectsSourceBindingDriftScoped(t *testing.T) {
	request := fixtureRequest(t, "binding-drift")
	request = cloneRequest(request)
	request.Inputs[1] = fixtureInput("apple-source", "swift.source", "Sources/BeamfallA11y/A11yID.swift", []byte("import SwiftUI\n"))
	assertScopedReason(t, Analyze(bytes.NewReader(requestBytes(t, request))), request, "EXACT_BINDING_UNAVAILABLE")
}

func cloneRequest(request Request) Request {
	if request.Target.Features != nil {
		features := request.Target.Features
		request.Target.Features = make([]string, len(features))
		copy(request.Target.Features, features)
	}
	request.Inputs = append([]Input(nil), request.Inputs...)
	return request
}

func TestAnalyzeUsesSentinelBeforeEnvelope(t *testing.T) {
	request := fixtureRequest(t, "pre-envelope")
	request.Target.Features = nil
	if got := Analyze(bytes.NewReader(requestBytes(t, request))); !bytes.Equal(got, sentinel) {
		t.Fatalf("features=null=%s", got)
	}
	request = fixtureRequest(t, "pre-envelope")
	request.Inputs[2].Path = "a:b"
	if got := Analyze(bytes.NewReader(requestBytes(t, request))); !bytes.Equal(got, sentinel) {
		t.Fatalf("colon path=%s", got)
	}
	request = fixtureRequest(t, "pre-envelope")
	request.Inputs[2].Handle = request.Inputs[1].Handle
	if got := Analyze(bytes.NewReader(requestBytes(t, request))); !bytes.Equal(got, sentinel) {
		t.Fatalf("duplicate handle=%s", got)
	}
}

func TestAnalyzeScopedReasonsAndLiteralSentinel(t *testing.T) {
	request := fixtureRequest(t, "digest-mismatch")
	request.Inputs[2].SHA256 = "sha256:" + strings.Repeat("0", 64)
	assertScopedReason(t, Analyze(bytes.NewReader(requestBytes(t, request))), request, "DIGEST_MISMATCH")
	for _, value := range [][]byte{[]byte("{\"family\":\"swift-apple\"}\n"), append(requestBytes(t, fixtureRequest(t, "extra-bytes")), 'x'), []byte("{\n")} {
		if got := Analyze(bytes.NewReader(value)); !bytes.Equal(got, sentinel) {
			t.Fatalf("sentinel=%q", got)
		}
	}
}

func TestSafeCanonicalExtensionBindsUnknownField(t *testing.T) {
	request := fixtureRequest(t, "extension")
	base := strings.TrimSuffix(string(requestBytes(t, request)), "}\n")
	for _, edited := range []string{
		base + `,"x":[9007199254740993,{"a":"b","c":null}]}`,
		strings.Replace(base, `"features":[]`, `"features":[],"x":true`, 1) + "}",
		strings.Replace(base, `"}]`, `","x":0,"y":"z"}]`, 1) + "}",
		base + `,"target inputs":0}`,
	} {
		if got, want := string(Analyze(strings.NewReader(edited+"\n"))), string(reject(request, echoes(request.Inputs), "UNKNOWN_FIELD")); got != want {
			t.Fatalf("frame=%.200s\ngot=%s\nwant=%s", edited, got, want)
		}
	}
	for _, edited := range []string{
		base + `,"x":1.0}`, base + `,"y":0,"x":0}`, base + `,"x":{"b":0,"a":0}}`,
		strings.Replace(base, `{"profile":`, `{"x":0,"profile":`, 1) + "}",
		strings.Replace(base, `"a11y"`, `"other"`, 1) + `,"x":0}`,
	} {
		if got := Analyze(strings.NewReader(edited + "\n")); !bytes.Equal(got, sentinel) {
			t.Fatalf("frame=%.200s\ngot=%s", edited, got)
		}
	}
}

func assertScopedReason(t testing.TB, got []byte, request Request, reason string) {
	t.Helper()
	var rejected rejection
	if err := json.Unmarshal(got, &rejected); err != nil {
		t.Fatal(err)
	}
	if rejected.Reason != reason || rejected.Profile != Profile || rejected.Family != Family || rejected.RequestID != request.RequestID || rejected.ScopeID != request.ScopeID || rejected.CompilationUnitID != request.CompilationUnitID || got[len(got)-1] != '\n' {
		t.Fatalf("reason=%s got=%s", reason, got)
	}
	if rejected.Status != "REJECTED" || len(rejected.InputEchoes) != maxInputs {
		t.Fatalf("echoes=%+v", rejected.InputEchoes)
	}
	gotTarget, err := json.Marshal(rejected.Target)
	if err != nil {
		t.Fatal(err)
	}
	wantTarget, err := json.Marshal(request.Target)
	if err != nil || !bytes.Equal(gotTarget, wantTarget) {
		t.Fatalf("target=%s want=%s", gotTarget, wantTarget)
	}
	wantEchoes := echoes(request.Inputs)
	for index := range wantEchoes {
		if rejected.InputEchoes[index] != wantEchoes[index] {
			t.Fatalf("echo[%d]=%+v want=%+v", index, rejected.InputEchoes[index], wantEchoes[index])
		}
	}
	canonical, err := json.Marshal(rejected)
	if err != nil || !bytes.Equal(append(canonical, '\n'), got) {
		t.Fatalf("noncanonical scoped rejection=%q", got)
	}
}

// TestParseSourceRejectionReasonsAreExact pins the reason code for every
// construct the closed grammar refuses, in both the lexical and the top-level
// grammar phases. It replaces a table that only asserted "some reason", which
// let a wrong code -- every attribute and every pound construct reported as
// DYNAMIC_INPUT -- pass unnoticed and misdirect coverage triage.
//
// The family's rule: DYNAMIC_INPUT means the bytes' meaning depends on a value
// that is not in the input (string interpolation). UNSUPPORTED_SCHEMA means the
// construct is real Swift outside the closed matrix. MALFORMED_INPUT means the
// bytes are not lexable Swift at all. LIMIT_EXCEEDED is a bounded-resource
// refusal and never a statement about the grammar.
func TestParseSourceRejectionReasonsAreExact(t *testing.T) {
	cases := map[string]struct{ source, reason string }{
		// Conditional compilation: the declaration set depends on a build
		// configuration absent from the input. Not dynamic; unimplemented.
		"conditional-top-level": {"#if DEBUG\nimport SwiftUI\n#endif\n", "UNSUPPORTED_SCHEMA"},
		"conditional-nested":    {"import SwiftUI\npublic enum A {\n#if DEBUG\ncase x\n#endif\n}\n", "UNSUPPORTED_SCHEMA"},
		"conditional-else":      {"#if os(macOS)\n#else\n#endif\n", "UNSUPPORTED_SCHEMA"},
		"source-location":       {"#sourceLocation(file: \"a.swift\", line: 1)\n", "UNSUPPORTED_SCHEMA"},
		// Freestanding macros and pound literals: the expansion is not present
		// in the bytes.
		"macro-preview":   {"#Preview {\n}\n", "UNSUPPORTED_SCHEMA"},
		"macro-external":  {"#externalMacro\n", "UNSUPPORTED_SCHEMA"},
		"macro-expect":    {"public struct A {\nfunc f() {\n#expect(x)\n}\n}\n", "UNSUPPORTED_SCHEMA"},
		"pound-available": {"public struct A {\nfunc f() {\nif #available(iOS 17.0, *) {}\n}\n}\n", "UNSUPPORTED_SCHEMA"},
		"pound-filepath":  {"public struct A {\nlet p = #filePath\n}\n", "UNSUPPORTED_SCHEMA"},
		"pound-selector":  {"public struct A {\nlet s = #selector(f)\n}\n", "UNSUPPORTED_SCHEMA"},
		"pound-warning":   {"#warning(\"x\")\n", "UNSUPPORTED_SCHEMA"},
		// Interpolation is the only construct this family calls dynamic.
		"interpolation":            {"import SwiftUI\npublic enum A {\nlet x = \"\\(value)\"\n}\n", "DYNAMIC_INPUT"},
		"interpolation-multiline":  {"public enum A {\nlet x = \"\"\"\n\\(value)\n\"\"\"\n}\n", "DYNAMIC_INPUT"},
		"interpolation-raw-string": {"let text = #\"\\#(value)\"#\n", "DYNAMIC_INPUT"},
		// Attribute grammar boundaries.
		"attribute-bare":           {"@\n", "MALFORMED_INPUT"},
		"attribute-space":          {"@ MainActor\npublic enum A {}\n", "MALFORMED_INPUT"},
		"attribute-digit":          {"@1Thing\npublic enum A {}\n", "MALFORMED_INPUT"},
		"attribute-without-decl":   {"@MainActor\n", "UNSUPPORTED_SCHEMA"},
		"attribute-before-brace":   {"@MainActor\n{\n}\n", "UNSUPPORTED_SCHEMA"},
		"attribute-unbalanced":     {"@available(*, deprecated\npublic enum A {}\n", "UNSUPPORTED_SCHEMA"},
		"attribute-brace-argument": {"@Wrapper({ x })\npublic enum A {}\n", "UNSUPPORTED_SCHEMA"},
		// Top-level grammar the matrix has no tuple for.
		"import-keyword":                 {"import class\n", "UNSUPPORTED_SCHEMA"},
		"unknown-modifier":               {"pub struct A {}\n", "UNSUPPORTED_SCHEMA"},
		"operator-decl":                  {"operator infix +\n", "UNSUPPORTED_SCHEMA"},
		"top-level-function":             {"public func f() {}\n", "UNSUPPORTED_SCHEMA"},
		"top-level-binding":              {"let value = 1\n", "UNSUPPORTED_SCHEMA"},
		"generic-parameter":              {"public struct Box<T> {}\n", "UNSUPPORTED_SCHEMA"},
		"duplicate-modifier":             {"public public struct A {}\n", "UNSUPPORTED_SCHEMA"},
		"inheritance-attribute-only":     {"public struct A: @unchecked {}\n", "UNSUPPORTED_SCHEMA"},
		"inheritance-trailing-comma":     {"public struct A: Sendable, {}\n", "UNSUPPORTED_SCHEMA"},
		"inheritance-attribute-no-ident": {"public struct A: @unchecked, Sendable {}\n", "UNSUPPORTED_SCHEMA"},
		// Not lexable Swift.
		"unterminated-string":  {"import SwiftUI\npublic enum A {\nlet x = \"unterminated\n}\n", "MALFORMED_INPUT"},
		"unterminated-comment": {"/* outer /* inner */\nimport SwiftUI\n", "MALFORMED_INPUT"},
		"stray-comment-close":  {"*/\nimport SwiftUI\n", "MALFORMED_INPUT"},
		"carriage-return":      {"import SwiftUI\r\n", "MALFORMED_INPUT"},
		"tab":                  {"import\tSwiftUI\n", "MALFORMED_INPUT"},
		"nul":                  {"import SwiftUI\x00\n", "MALFORMED_INPUT"},
		"no-trailing-newline":  {"import SwiftUI", "MALFORMED_INPUT"},
		"pound-escape":         {"#\\\"raw\\\"#\n", "MALFORMED_INPUT"},
		"pound-bare":           {"#\n", "MALFORMED_INPUT"},
		"empty":                {"", "MALFORMED_INPUT"},
	}
	request := fixtureRequest(t, "reasons")
	for name, testCase := range cases {
		facts, reason := parseSource(request, request.Inputs[1], []byte(testCase.source))
		if reason != testCase.reason {
			t.Errorf("%s: reason=%q want=%q source=%q", name, reason, testCase.reason, testCase.source)
		}
		if len(facts) != 0 {
			t.Errorf("%s: rejected source produced %d facts", name, len(facts))
		}
	}

	invalidUTF8 := append([]byte("import SwiftUI\n// "), 0xff)
	invalidUTF8 = append(invalidUTF8, '\n')
	if _, reason := parseSource(request, request.Inputs[1], invalidUTF8); reason != "MALFORMED_INPUT" {
		t.Errorf("invalid utf8: reason=%q want MALFORMED_INPUT", reason)
	}
}

// TestParseSourceAdmitsAttributedDeclarations pins the constructs this change
// newly admits. An attribute is not fact-bearing under the frozen tuple matrix,
// so each case asserts the exact facts the underlying declaration or import
// yields -- admitting the attribute must add no fact of its own.
func TestParseSourceAdmitsAttributedDeclarations(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{"leading-attribute", "@MainActor\npublic enum A {}\n", []string{"declares-enum=A"}},
		{"inline-attribute", "@objc final class B {}\n", []string{"declares-class=B"}},
		{"attribute-arguments", "@available(iOS 17.0, macOS 14.0, *)\npublic struct C {}\n", []string{"declares-struct=C"}},
		{"attribute-identifier-argument", "@objc(BFThing)\npublic protocol D {}\n", []string{"declares-protocol=D"}},
		{"underscored-attribute-import", "@_exported import SwiftUI\n", []string{"imports=SwiftUI"}},
		{"attributed-import-and-actor", "@preconcurrency import Foundation\npublic actor E {}\n", []string{"imports=Foundation", "declares-actor=E"}},
		{"stacked-attributes", "@Observable\n@MainActor\nfinal class F {}\n", []string{"declares-class=F"}},
		{"attributed-extension", "@MainActor\nextension G: View {}\n", []string{"extends=G"}},
		{"attribute-inside-body", "import SwiftUI\npublic struct H {\n@State private var count = 0\n@Environment(\\.dismiss) private var dismiss\n}\n", []string{"imports=SwiftUI", "declares-struct=H"}},
		{"nested-attribute-arguments", "@available(*, deprecated, message: \"use I2\")\npublic enum I {}\n", []string{"declares-enum=I"}},
		{"attributed-inheritance", "public struct J: @unchecked Sendable {}\n", []string{"declares-struct=J"}},
		{"attributed-inheritance-list", "public final class K: NSObject, @unchecked Sendable {}\n", []string{"declares-class=K"}},
		{"attributed-retroactive-extension", "extension L: @retroactive Equatable {}\n", []string{"extends=L"}},
	}
	request := fixtureRequest(t, "attributes")
	for _, testCase := range cases {
		facts, reason := parseSource(request, request.Inputs[1], []byte(testCase.source))
		if reason != "" {
			t.Errorf("%s: rejected with %s: %q", testCase.name, reason, testCase.source)
			continue
		}
		got := make([]string, 0, len(facts))
		for _, fact := range facts {
			got = append(got, fact.Predicate+"="+fact.Value)
		}
		if strings.Join(got, ",") != strings.Join(testCase.want, ",") {
			t.Errorf("%s: facts=%v want=%v", testCase.name, got, testCase.want)
		}
	}
}

// TestTokenOffsetsHoldAtMaximalSource pins that a token at the far end of a
// maximally sized body still reports its own bytes. The offset fields were
// uint16 while maxSourceSize is 65,536: in range only because a body must end
// in LF, so one byte more of source would have wrapped a start offset to zero
// and produced a valid-looking slice of the wrong bytes rather than a
// rejection. This asserts the fact, not the ceiling.
func TestTokenOffsetsHoldAtMaximalSource(t *testing.T) {
	const declaration = "public enum TailSentinel {}\n"
	// The padding is spaces, which emit no token: maxSwiftTokens (4,096) binds
	// long before maxSourceSize (65,536), so no token-bearing filler can reach
	// the source ceiling at all.
	body := append(bytes.Repeat([]byte{' '}, maxSourceSize-len(declaration)), []byte(declaration)...)
	if len(body) != maxSourceSize {
		t.Fatalf("body=%d want=%d", len(body), maxSourceSize)
	}
	request := fixtureRequest(t, "maximal")
	facts, reason := parseSource(request, request.Inputs[1], body)
	if reason != "" {
		t.Fatalf("maximal source rejected: %s", reason)
	}
	if len(facts) != 1 || facts[0].Value != "TailSentinel" {
		t.Fatalf("facts=%+v", facts)
	}
	if _, reason := lexSwift(append(body, '\n')); reason != "LIMIT_EXCEEDED" {
		t.Fatalf("over-ceiling reason=%q want LIMIT_EXCEEDED", reason)
	}

	// The margin between the token offset type and maxSourceSize is exactly one
	// byte, and only the required trailing LF supplies it: the last addressable
	// index is 65,535, so a start offset never reaches 65,536. Raising
	// maxSourceSize without widening the offset type makes a wrap reachable,
	// and a wrapped start produces a valid-looking slice of the wrong bytes
	// rather than a rejection. This asserts the coupling rather than the value,
	// so it fails on either half of the change.
	var probe swiftToken
	probe.start = ^probe.start
	if maxSourceSize > int(probe.start)+1 {
		t.Fatalf("maxSourceSize=%d exceeds the %d offsets a swiftToken addresses plus its trailing LF: a start offset can wrap", maxSourceSize, int(probe.start)+1)
	}
}

// TestSwiftTokenCapIsEnforced pins maxSwiftTokens in both directions. The cap
// binds well before maxSourceSize on real Swift -- 4,096 tokens is roughly a
// 20KB file -- so it is a live rejection cause, not a theoretical one, and it
// must report LIMIT_EXCEEDED rather than any grammar reason.
func TestSwiftTokenCapIsEnforced(t *testing.T) {
	atCap := append(bytes.Repeat([]byte{'\n'}, maxSwiftTokens-1), '\n')
	if tokens, reason := lexSwift(atCap); reason != "" || len(tokens) != maxSwiftTokens {
		t.Fatalf("at cap tokens=%d reason=%q", len(tokens), reason)
	}
	overCap := append(bytes.Repeat([]byte{'\n'}, maxSwiftTokens), '\n')
	if _, reason := lexSwift(overCap); reason != "LIMIT_EXCEEDED" {
		t.Fatalf("over cap reason=%q want LIMIT_EXCEEDED", reason)
	}
	if _, reason := lexSwift([]byte("let text = " + strings.Repeat("#", maxRawHashes+1) + "\"x\"" + strings.Repeat("#", maxRawHashes+1) + "\n")); reason != "LIMIT_EXCEEDED" {
		t.Fatalf("raw-hash cap reason=%q want LIMIT_EXCEEDED", reason)
	}
}

func TestLexerRawStringsAndControlBytes(t *testing.T) {
	valid := []string{
		"let text = #\"// not comment\"#\n",
		"let text = ##\"\\#(notInterpolation)\"##\n",
		"let text = #\"\"\"line one\nline two\"\"\"#\n",
	}
	for _, source := range valid {
		if _, reason := lexSwift([]byte(source)); reason != "" {
			t.Fatalf("raw string rejected %q: %s", source, reason)
		}
	}
	invalid := []string{
		"let text = #\"\\#(value)\"#\n",
		"/*\x00*/\nimport SwiftUI\n",
		"//\x00\nimport SwiftUI\n",
		"/*\r*/\nimport SwiftUI\n",
	}
	for _, source := range invalid {
		if _, reason := lexSwift([]byte(source)); reason == "" {
			t.Fatalf("unsafe lexical state accepted %q", source)
		}
	}
}

func TestLexerHandlesNestedCommentsAndStringsWithoutFacts(t *testing.T) {
	source := []byte(`/* outer /* inner */ outer */
import SwiftUI
public enum A {
let text = "plain"
let multiline = """ok"""
}
`)
	request := fixtureRequest(t, "lexical-control")
	facts, reason := parseSource(request, request.Inputs[1], source)
	if reason != "" || len(facts) != 2 {
		t.Fatalf("facts=%+v reason=%s", facts, reason)
	}
}

func TestBoundsAndTypedOrdering(t *testing.T) {
	if _, reason := lexSwift(append(bytes.Repeat([]byte{' '}, maxSourceSize-1), '\n')); reason != "" {
		t.Fatalf("limit source=%s", reason)
	}
	if _, reason := lexSwift(append(bytes.Repeat([]byte{' '}, maxSourceSize), '\n')); reason == "" {
		t.Fatal("accepted source over limit")
	}
	left := Fact{Kind: "a", InputHandle: "b\\x00z", RelatedHandle: "-", Subject: "s", Predicate: "p", Value: "v", InstanceID: "i", EvidenceSHA256: "x"}
	right := Fact{Kind: "a\\x00b", InputHandle: "z", RelatedHandle: "-", Subject: "s", Predicate: "p", Value: "v", InstanceID: "i", EvidenceSHA256: "x"}
	if compareFact(left, right) == 0 {
		t.Fatal("typed comparison collapsed delimiter-bearing fields")
	}
	duplicate := left
	if compareFact(left, duplicate) != 0 {
		t.Fatal("duplicate identity included non-tuple state")
	}
	request := fixtureRequest(t, "facts")
	budgetFacts := make([]Fact, maxFacts)
	for index := range budgetFacts {
		budgetFacts[index] = Fact{Kind: "k", InputHandle: "i", RelatedHandle: "-", Subject: "s", Predicate: "p", Value: fmt.Sprintf("v-%d", index), InstanceID: "x", EvidenceSHA256: "sha256:" + strings.Repeat("a", 64)}
	}
	if !factsFitOutput(request, echoes(request.Inputs), budgetFacts) {
		t.Fatal("rejected fact-count boundary")
	}
	if factsFitOutput(request, echoes(request.Inputs), append(budgetFacts, budgetFacts[0])) {
		t.Fatal("accepted fact count over limit")
	}
}

func TestEnvelopeAndProspectiveBounds(t *testing.T) {
	request := fixtureRequest(t, "bounds")
	wire := requestBytes(t, request)
	parsed, content, err := parseCanonicalRequest(wire)
	if err != nil || parsed.Inputs[0].ContentBase64 != "" || len(content) != maxInputs {
		t.Fatalf("parse err=%v inputs=%+v content=%d", err, parsed.Inputs, len(content))
	}
	if _, _, err := parseCanonicalRequest(append([]byte(" "), wire...)); err == nil {
		t.Fatal("accepted noncanonical leading whitespace")
	}
	body := bytes.Repeat([]byte{'a'}, maxData)
	input := fixtureInput("apple-ui", "swift.apple-ui.package", "Package.swift", body)
	encoded := []byte(input.ContentBase64)
	if len(encoded) != 1_398_104 {
		t.Fatalf("encoded=%d", len(encoded))
	}
	if decoded, reason := decodeInput(input, encoded, maxData); reason != "" || len(decoded) != maxData {
		t.Fatalf("boundary reason=%s len=%d", reason, len(decoded))
	}
	if _, reason := decodeInput(input, encoded, maxData-1); reason != "LIMIT_EXCEEDED" {
		t.Fatalf("over-limit reason=%s", reason)
	}
	budget, ok := newOutputBudget(request, echoes(request.Inputs))
	if !ok || !budget.add(Fact{Kind: "k", InputHandle: "i", RelatedHandle: "-", Subject: "s", Predicate: "p", Value: "v", InstanceID: "x", EvidenceSHA256: "sha256:" + strings.Repeat("a", 64)}) {
		t.Fatal("rejected bounded fact")
	}
	if budget.add(Fact{Kind: strings.Repeat("x", 4097), InputHandle: "i", RelatedHandle: "-", Subject: "s", Predicate: "p", Value: "v", InstanceID: "x", EvidenceSHA256: "sha256:" + strings.Repeat("a", 64)}) {
		t.Fatal("accepted unbounded fact")
	}
}

func TestThousandUniqueInMemoryReplays(t *testing.T) {
	seen := make(map[[32]byte]struct{}, 1_000)
	for index := 0; index < 1_000; index++ {
		request := fixtureRequest(t, fmt.Sprintf("replay-%04d", index))
		wire := requestBytes(t, request)
		first := Analyze(bytes.NewReader(wire))
		second := Analyze(bytes.NewReader(wire))
		if !bytes.Equal(first, second) || first[len(first)-1] != '\n' {
			t.Fatalf("replay=%d mismatch", index)
		}
		digest := sha256.Sum256(first)
		if _, duplicate := seen[digest]; duplicate {
			t.Fatalf("replay=%d duplicate output", index)
		}
		seen[digest] = struct{}{}
	}
}

func TestSentinelReplayIsImmutable(t *testing.T) {
	first := Analyze(strings.NewReader("{\n"))
	first[0] = '!'
	second := Analyze(strings.NewReader("{\n"))
	if !bytes.Equal(second, sentinel) {
		t.Fatalf("mutable sentinel=%q", second)
	}
}

type shortWriter struct{}

func (shortWriter) Write(value []byte) (int, error) { return len(value) - 1, nil }

func TestRunRejectsShortWrite(t *testing.T) {
	if got := Run([]string{"--request-file", "missing"}, shortWriter{}); got != 1 {
		t.Fatalf("exit=%d", got)
	}
	if err := writeFull(shortWriter{}, sentinelBytes()); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write=%v", err)
	}
}

func TestDescriptorPathIsRequired(t *testing.T) {
	var output bytes.Buffer
	if got := Run(nil, &output); got != 1 || !bytes.Equal(output.Bytes(), sentinel) {
		t.Fatalf("run=%d output=%q", got, output.Bytes())
	}
}

func TestFixturePathsAreRepositoryLocal(t *testing.T) {
	for _, path := range []string{"testdata/beamfall-apple-8588/Sources/BeamfallA11y/A11yID.swift", "testdata/beamfall-apple-ui-830a/Package.swift"} {
		if filepath.IsAbs(path) {
			t.Fatal(path)
		}
	}
}

func BenchmarkAnalyzeCandidate(b *testing.B) {
	input := requestBytes(b, fixtureRequest(b, "benchmark-pinned-beamfall-apple"))
	want := Analyze(bytes.NewReader(input))
	if !bytes.Contains(want, []byte(`"status":"CANDIDATE"`)) {
		b.Fatalf("pinned response=%q", want)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got := Analyze(bytes.NewReader(input))
		if !bytes.Equal(got, want) {
			b.Fatal("pinned response drift")
		}
	}
}
