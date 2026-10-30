package analyzerrust

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func makeInput(handle, family, path, content string) Input {
	sum := sha256.Sum256([]byte(content))
	return Input{
		Handle:        handle,
		Family:        family,
		Path:          path,
		SHA256:        "sha256:" + hex.EncodeToString(sum[:]),
		ContentBase64: base64.StdEncoding.EncodeToString([]byte(content)),
	}
}

func canonicalFrame(t *testing.T, inputs ...Input) []byte {
	t.Helper()
	request := Request{
		Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root",
		CompilationUnitID: "unit-1",
		Target:            Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}},
		Inputs:            inputs,
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func analyzeSuccess(t *testing.T, inputs ...Input) success {
	t.Helper()
	raw := AnalyzeCanonical(canonicalFrame(t, inputs...))
	if raw[len(raw)-1] != '\n' || strings.Count(string(raw), "\n") != 1 {
		t.Fatalf("output is not one LF frame: %q", raw)
	}
	var decoded success
	if err := json.Unmarshal(raw[:len(raw)-1], &decoded); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	if decoded.Status != "CANDIDATE" {
		t.Fatalf("status=%s frame=%s", decoded.Status, raw)
	}
	return decoded
}

func analyzeReason(t *testing.T, inputs ...Input) string {
	t.Helper()
	return reasonOf(t, AnalyzeCanonical(canonicalFrame(t, inputs...)))
}

func reasonOf(t *testing.T, raw []byte) string {
	t.Helper()
	var decoded failure
	if err := json.Unmarshal(raw[:len(raw)-1], &decoded); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	if decoded.Status != "REJECTED" {
		return ""
	}
	return decoded.Reason
}

// factSet renders facts as "kind|predicate|value" for order-independent
// comparison of the extracted set.
func factSet(facts []Fact) []string {
	rendered := make([]string, 0, len(facts))
	for _, f := range facts {
		rendered = append(rendered, f.Kind+"|"+f.Predicate+"|"+f.Value)
	}
	return rendered
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

const manifestSource = `[package]
name = "example"
version = "0.1.0"
edition = "2021"
rust-version = "1.80.0"

[dependencies]
serde = "1.0.0"

[dev-dependencies]
proptest = "1.5.0"
`

// TestManifestFacts pins the closed Cargo.toml subset's complete fact set,
// including the distinct dependency predicates.
func TestManifestFacts(t *testing.T) {
	got := factSet(analyzeSuccess(t, makeInput("input-1", "rust.manifest", "Cargo.toml", manifestSource)).Facts)
	want := []string{
		"rust.package|declares-package|example",
		"rust.package.version|declares-version|0.1.0",
		"rust.edition|declares-edition|2021",
		"rust.language.declaration|declares-language|1.80.0",
		"rust.dependency|requires|1.0.0",
		"rust.dependency|requires-for-development|1.5.0",
	}
	if len(got) != len(want) {
		t.Fatalf("fact count=%d want=%d got=%v", len(got), len(want), got)
	}
	for _, expected := range want {
		if !contains(got, expected) {
			t.Errorf("missing fact %q in %v", expected, got)
		}
	}
}

// TestEvidenceDigestMatchesFrozenGenericConstruction recomputes each digest with
// an independent implementation of the fourteen-field generic construction
// pinned by internal/analyzercap/candidate_profile_vectors_test.go. It is the
// proof that this family reuses the frozen digest rather than inventing one.
func TestEvidenceDigestMatchesFrozenGenericConstruction(t *testing.T) {
	input := makeInput("input-1", "rust.manifest", "Cargo.toml", manifestSource)
	result := analyzeSuccess(t, input)
	target := `{"os":"darwin","architecture":"arm64","abi":"none","features":[]}`
	for _, fact := range result.Facts {
		fields := []string{
			Family, "request-1", "root", "unit-1", target,
			fact.InputHandle, input.SHA256, fact.RelatedHandle, "-", fact.Kind,
			fact.Subject, fact.Predicate, fact.Value, fact.InstanceID,
		}
		hasher := sha256.New()
		hasher.Write([]byte("corvint-analyzer-candidate-evidence/experimental"))
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(fields)))
		hasher.Write(size[:])
		for _, field := range fields {
			binary.BigEndian.PutUint32(size[:], uint32(len(field)))
			hasher.Write(size[:])
			hasher.Write([]byte(field))
		}
		want := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
		if fact.EvidenceSHA256 != want {
			t.Errorf("%s evidence=%s want=%s", fact.Kind, fact.EvidenceSHA256, want)
		}
	}
}

// TestFactsStrictlyIncreasing pins the generic ordering invariant.
func TestFactsStrictlyIncreasing(t *testing.T) {
	result := analyzeSuccess(t,
		makeInput("input-1", "rust.manifest", "Cargo.toml", manifestSource),
		makeInput("input-2", "rust.source", "src/lib.rs", hostileSource),
	)
	for i := 1; i < len(result.Facts); i++ {
		if !factLess(result.Facts[i-1], result.Facts[i]) {
			t.Fatalf("facts not strictly increasing at %d: %#v then %#v",
				i, result.Facts[i-1], result.Facts[i])
		}
	}
}

// hostileSource exercises every construct RAC-005 names. Its comment, string,
// and raw-string bodies each contain source that must never become a fact.
const hostileSource = `
mod real;
/* outer /* inner */ mod hidden_in_nested; */
// mod hidden_in_line;
const RAW: &str = r###"mod fake_in_raw; use fake::path;"###;
const S: &str = "mod fake_in_string;";
const ESCAPED: &str = "he said \"mod not_here;\" loudly";
const BYTES: &[u8] = br#"mod fake_in_byte_raw;"#;
fn borrow<'a>(v: &'a str) -> &'a str { v }
const C: char = 'a';
const B: u8 = b'x';
const NL: char = '\n';
const UNI: char = '\u{1F600}';
static mut COUNTER: u32 = 0;
pub struct Widget;
pub enum Mode { On, Off }
pub trait Draw {}
pub union Bits { a: u32 }
pub type Alias = Widget;
pub const LIMIT: usize = 8;
const fn compute() -> usize { 1 }
fn r#match() {}
mod r#type;
use std::collections::HashMap;
use crate::real::Thing;
use super::super::sibling::Item;
use std::{fmt, io};
use foo::*;
use bar as baz;
extern crate libc;
#[derive(Debug)]
#[cfg(target_os = "linux")]
pub fn gated() {}
#[test]
fn checks_it() {}
`

// TestHostileSourceLexing is the demonstrated-red control for RAC-005: every
// construct that a naive lexer mishandles appears here with source hidden
// inside it, and none of that hidden source may become a fact.
func TestHostileSourceLexing(t *testing.T) {
	got := factSet(analyzeSuccess(t, makeInput("input-1", "rust.source", "src/lib.rs", hostileSource)).Facts)

	forbidden := []string{
		"rust.module|declares-module|hidden_in_nested", // nested block comment
		"rust.module|declares-module|hidden_in_line",   // line comment
		"rust.module|declares-module|fake_in_raw",      // raw string body
		"rust.module|declares-module|fake_in_string",   // string body
		"rust.module|declares-module|not_here",         // escaped quote in string
		"rust.module|declares-module|fake_in_byte_raw", // raw byte string body
		"rust.import.static|imports|fake::path",        // use hidden in raw string
	}
	for _, banned := range forbidden {
		if contains(got, banned) {
			t.Errorf("fact leaked from an inert construct: %q", banned)
		}
	}

	required := []string{
		"rust.module|declares-module|real",
		"rust.module|declares-module|type",         // mod r#type; the reference drops this
		"rust.declaration|declares-fn|match",       // fn r#match; the reference reads "r"
		"rust.declaration|declares-fn|borrow",      // lifetimes did not swallow the file
		"rust.declaration|declares-fn|compute",     // const fn attributed to fn
		"rust.declaration|declares-const|C",        // char literal
		"rust.declaration|declares-const|B",        // byte literal
		"rust.declaration|declares-const|NL",       // escape in char literal
		"rust.declaration|declares-const|UNI",      // unicode escape in char literal
		"rust.declaration|declares-static|COUNTER", // static mut
		"rust.declaration|declares-struct|Widget",
		"rust.declaration|declares-enum|Mode",
		"rust.declaration|declares-trait|Draw",
		"rust.declaration|declares-union|Bits",
		"rust.declaration|declares-type|Alias",
		"rust.extern.crate|links-crate|libc",
		"rust.import.static|imports|std::collections::HashMap",
		"rust.import.static|imports|crate::real::Thing",
		"rust.import.static|imports|super::super::sibling::Item",
		"rust.use.unevaluated|imports-unevaluated|std", // grouped tree
		"rust.use.unevaluated|imports-unevaluated|foo", // glob
		"rust.use.unevaluated|imports-unevaluated|bar", // alias
		"rust.attribute|carries-attribute|derive",
		"rust.attribute|carries-attribute|cfg",
		"rust.cfg.unevaluated|declares-conditional-compilation|cfg",
		"rust.test|declares-test|checks_it",
	}
	for _, expected := range required {
		if !contains(got, expected) {
			t.Errorf("missing fact %q; got %v", expected, got)
		}
	}
}

// TestRepeatedObservationRecordedOnce pins RAC-002: two #[derive] attributes in
// one file are one observation, not a duplicate that rejects the request.
func TestRepeatedObservationRecordedOnce(t *testing.T) {
	source := "#[derive(Debug)]\npub struct A;\n#[derive(Debug)]\npub struct B;\n" +
		"#[cfg(unix)]\nfn u() {}\n#[cfg(windows)]\nfn w() {}\n"
	got := factSet(analyzeSuccess(t, makeInput("input-1", "rust.source", "src/a.rs", source)).Facts)
	derives := 0
	for _, value := range got {
		if value == "rust.attribute|carries-attribute|derive" {
			derives++
		}
	}
	if derives != 1 {
		t.Fatalf("derive observations=%d want 1; got %v", derives, got)
	}
	for _, expected := range []string{"rust.declaration|declares-struct|A", "rust.declaration|declares-struct|B"} {
		if !contains(got, expected) {
			t.Errorf("missing %q in %v", expected, got)
		}
	}
}

// TestSourceRejections covers the lexical fail-closed boundary and the dynamic
// external-input rule.
func TestSourceRejections(t *testing.T) {
	cases := []struct {
		name, source, want string
	}{
		{"include macro", "include!(\"gen.rs\");\n", "DYNAMIC_INPUT"},
		{"include_str macro", "const G: &str = include_str!(\"g.md\");\n", "DYNAMIC_INPUT"},
		{"include_bytes macro", "const G: &[u8] = include_bytes!(\"g.bin\");\n", "DYNAMIC_INPUT"},
		{"unterminated raw string", "const X: &str = r#\"open\n", "MALFORMED_INPUT"},
		{"unterminated block comment", "/* never closed\nfn f() {}\n", "MALFORMED_INPUT"},
		{"unterminated nested block", "/* a /* b */\nfn f() {}\n", "MALFORMED_INPUT"},
		{"unterminated string", "const X: &str = \"open\n", "MALFORMED_INPUT"},
		{"unterminated escaped char", "const C: char = '\\n\n", "MALFORMED_INPUT"},
		{"unterminated non-ident char", "const C: char = '+\n", "MALFORMED_INPUT"},
		{"raw newline in char literal", "const C: char = '\n';\n", "MALFORMED_INPUT"},
		{"unbalanced open brace", "fn f() {\n", "MALFORMED_INPUT"},
		{"unbalanced close brace", "fn f() {}}\n", "MALFORMED_INPUT"},
		{"mismatched delimiter pair", "fn f(] {}\n", "MALFORMED_INPUT"},
		{"mismatched nested delimiter", "fn f() { let x = [1, 2); }\n", "MALFORMED_INPUT"},
		{"carriage return", "fn f() {}\r\n", "MALFORMED_INPUT"},
		{"nul byte", "fn f() {}\x00\n", "MALFORMED_INPUT"},
		{"non-ascii identifier", "fn café() {}\n", "UNSUPPORTED_SCHEMA"},
		{"raw identifier keyword", "mod r#crate;\n", "MALFORMED_INPUT"},
		{"wrong path suffix", "fn f() {}\n", "UNSUPPORTED_SCHEMA"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := "src/a.rs"
			if tc.name == "wrong path suffix" {
				path = "src/a.txt"
			}
			if got := analyzeReason(t, makeInput("input-1", "rust.source", path, tc.source)); got != tc.want {
				t.Fatalf("reason=%s want=%s", got, tc.want)
			}
		})
	}
}

// TestMalformedUsePathIsNotStatic pins RAC-008: a use path whose separators are
// not exactly `::`, or that is not terminated by `;`, is not a closed atom and
// must not become a static import fact.
func TestMalformedUsePathIsNotStatic(t *testing.T) {
	for _, source := range []string{"use a:b;\n", "use a:::b;\n", "use a: :b;\n", "use a::b::;\n", "use a::b"} {
		got := factSet(analyzeSuccess(t, makeInput("input-1", "rust.source", "src/a.rs", source)).Facts)
		if len(got) != 1 || got[0] != "rust.use.unevaluated|imports-unevaluated|a" {
			t.Errorf("source %q facts=%v, want only the unevaluated head", source, got)
		}
	}
}

// TestManifestRejections covers the closed TOML subset boundary.
func TestManifestRejections(t *testing.T) {
	cases := []struct {
		name, content, want string
	}{
		{"caret range", "[package]\nname = \"a\"\n\n[dependencies]\ns = \"^1.0\"\n", "UNSUPPORTED_SCHEMA"},
		{"tilde range", "[package]\nname = \"a\"\n\n[dependencies]\ns = \"~1.0.0\"\n", "UNSUPPORTED_SCHEMA"},
		{"wildcard", "[package]\nname = \"a\"\n\n[dependencies]\ns = \"1.*\"\n", "UNSUPPORTED_SCHEMA"},
		{"inline table", "[package]\nname = \"a\"\n\n[dependencies]\ns = { version = \"1.0.0\" }\n", "UNSUPPORTED_SCHEMA"},
		{"duplicate section", "[package]\nname = \"a\"\n\n[dependencies]\n\n[dependencies]\n", "DUPLICATE_VALUE"},
		{"duplicate key", "[package]\nname = \"a\"\nname = \"b\"\n", "DUPLICATE_VALUE"},
		{"unknown section", "[package]\nname = \"a\"\n\n[lib]\nname = \"a\"\n", "UNSUPPORTED_SCHEMA"},
		{"unknown key", "[package]\nname = \"a\"\nlicense = \"MIT\"\n", "UNSUPPORTED_SCHEMA"},
		{"array of tables", "[package]\nname = \"a\"\n\n[[bin]]\nname = \"a\"\n", "UNSUPPORTED_SCHEMA"},
		{"dotted key", "[package]\nname = \"a\"\nedition.workspace = true\n", "MALFORMED_INPUT"},
		{"target cfg section", "[package]\nname = \"a\"\n\n[target.'cfg(unix)'.dependencies]\nlibc = \"0.2.0\"\n", "UNSUPPORTED_SCHEMA"},
		{"leading zero version", "[package]\nname = \"a\"\nversion = \"01.0.0\"\n", "UNSUPPORTED_SCHEMA"},
		{"two component version", "[package]\nname = \"a\"\nversion = \"1.0\"\n", "UNSUPPORTED_SCHEMA"},
		{"prerelease version", "[package]\nname = \"a\"\nversion = \"1.0.0-rc1\"\n", "UNSUPPORTED_SCHEMA"},
		{"unknown edition", "[package]\nname = \"a\"\nedition = \"2027\"\n", "UNSUPPORTED_SCHEMA"},
		{"indented row", "[package]\n  name = \"a\"\n", "MALFORMED_INPUT"},
		{"noncanonical spacing", "[package]\nname=\"a\"\n", "MALFORMED_INPUT"},
		{"root key before section", "name = \"a\"\n", "UNSUPPORTED_SCHEMA"},
		{"missing package name", "[dependencies]\ns = \"1.0.0\"\n", "UNSUPPORTED_SCHEMA"},
		{"multiline string", "[package]\nname = \"\"\"a\"\"\"\n", "UNSUPPORTED_SCHEMA"},
		{"no trailing newline", "[package]\nname = \"a\"", "MALFORMED_INPUT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := analyzeReason(t, makeInput("input-1", "rust.manifest", "Cargo.toml", tc.content)); got != tc.want {
				t.Fatalf("reason=%s want=%s", got, tc.want)
			}
		})
	}
}

// TestToolchainManifestRule covers the analogue of the ruby.version rule and its
// conflict with a declared rust-version.
func TestToolchainManifestRule(t *testing.T) {
	accepted := []string{"1.80.0", "stable", "beta", "nightly"}
	for _, channel := range accepted {
		content := "[toolchain]\nchannel = \"" + channel + "\"\n"
		result := analyzeSuccess(t, makeInput("input-1", "rust.toolchain", "rust-toolchain.toml", content))
		if len(result.Facts) != 1 || result.Facts[0].Value != channel {
			t.Fatalf("channel %s facts=%#v", channel, result.Facts)
		}
	}
	rejected := map[string]string{
		"[toolchain]\nchannel = \"1.80\"\n":                          "UNSUPPORTED_SCHEMA",
		"[toolchain]\nchannel = \"nightly-2026-01-01\"\n":            "UNSUPPORTED_SCHEMA",
		"[toolchain]\nchannel = \"1.80.0\"\nprofile = \"minimal\"\n": "UNSUPPORTED_SCHEMA",
		"[toolchain]\n":                   "UNSUPPORTED_SCHEMA",
		"[other]\nchannel = \"1.80.0\"\n": "UNSUPPORTED_SCHEMA",
	}
	for content, want := range rejected {
		if got := analyzeReason(t, makeInput("input-1", "rust.toolchain", "rust-toolchain.toml", content)); got != want {
			t.Errorf("content=%q reason=%s want=%s", content, got, want)
		}
	}
}

func TestToolchainConflictsWithDeclaredRustVersion(t *testing.T) {
	manifest := makeInput("input-1", "rust.manifest", "Cargo.toml",
		"[package]\nname = \"a\"\nrust-version = \"1.80.0\"\n")
	older := makeInput("input-2", "rust.toolchain", "rust-toolchain.toml",
		"[toolchain]\nchannel = \"1.70.0\"\n")
	if got := analyzeReason(t, manifest, older); got != "CONFLICTING_VALUE" {
		t.Fatalf("older channel reason=%s want CONFLICTING_VALUE", got)
	}
	newer := makeInput("input-2", "rust.toolchain", "rust-toolchain.toml",
		"[toolchain]\nchannel = \"1.90.0\"\n")
	if result := analyzeSuccess(t, manifest, newer); len(result.Facts) == 0 {
		t.Fatal("newer channel produced no facts")
	}
	named := makeInput("input-2", "rust.toolchain", "rust-toolchain.toml",
		"[toolchain]\nchannel = \"stable\"\n")
	if result := analyzeSuccess(t, manifest, named); len(result.Facts) == 0 {
		t.Fatal("named channel produced no facts")
	}
}

// TestLockfileDeclaredButUnimplemented pins the js.bun-lock-v1 precedent.
func TestLockfileDeclaredButUnimplemented(t *testing.T) {
	content := "version = 4\n\n[[package]]\nname = \"serde\"\nversion = \"1.0.0\"\n"
	if got := analyzeReason(t, makeInput("input-1", "rust.lock", "Cargo.lock", content)); got != "UNSUPPORTED_SCHEMA" {
		t.Fatalf("reason=%s want UNSUPPORTED_SCHEMA", got)
	}
}

// TestEnvelopeRejections covers the pre-envelope sentinel and bound rejection
// boundaries of RAC-001 and RAC-009.
func TestEnvelopeRejections(t *testing.T) {
	valid := makeInput("input-1", "rust.manifest", "Cargo.toml", manifestSource)
	frame := string(canonicalFrame(t, valid))

	cases := []struct {
		name  string
		frame string
		want  string
	}{
		{"missing trailing LF", strings.TrimSuffix(frame, "\n"), "NONCANONICAL_REQUEST"},
		{"two LF", frame + "\n", "NONCANONICAL_REQUEST"},
		{"empty", "", "NONCANONICAL_REQUEST"},
		{"leading whitespace", " " + frame, "NONCANONICAL_REQUEST"},
		{"unknown field", strings.Replace(frame, `"family":"rust"`, `"family":"rust","extra":1`, 1), "UNKNOWN_FIELD"},
		{"duplicate field", strings.Replace(frame, `"family":"rust"`, `"family":"rust","family":"rust"`, 1), "NONCANONICAL_REQUEST"},
		{"wrong family", strings.Replace(frame, `"family":"rust"`, `"family":"go"`, 1), "UNKNOWN_FAMILY"},
		{"wrong profile", strings.Replace(frame, Profile, "corvint-analyzer-candidate/other", 1), "MALFORMED_INPUT"},
		{"bad request id", strings.Replace(frame, `"request_id":"request-1"`, `"request_id":"bad id"`, 1), "INVALID_IDENTIFIER"},
		{"null features", strings.Replace(frame, `"features":[]`, `"features":null`, 1), "MALFORMED_INPUT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := AnalyzeCanonical([]byte(tc.frame))
			if got := reasonOf(t, raw); got != tc.want {
				t.Fatalf("reason=%s want=%s frame=%q", got, tc.want, tc.frame)
			}
			var decoded failure
			if err := json.Unmarshal(raw[:len(raw)-1], &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Family != "unknown" || decoded.RequestID != "unknown" {
				t.Fatalf("pre-envelope failure must use the sentinel: %s", raw)
			}
			if decoded.Target != nil || len(decoded.InputEchoes) != 0 {
				t.Fatalf("sentinel retained request bindings: %s", raw)
			}
		})
	}
}

func TestBoundRejectionsEchoRequest(t *testing.T) {
	bad := makeInput("input-1", "rust.manifest", "Cargo.toml", manifestSource)
	bad.SHA256 = "sha256:" + strings.Repeat("0", 64)
	raw := AnalyzeCanonical(canonicalFrame(t, bad))
	var decoded failure
	if err := json.Unmarshal(raw[:len(raw)-1], &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Reason != "DIGEST_MISMATCH" {
		t.Fatalf("reason=%s want DIGEST_MISMATCH", decoded.Reason)
	}
	if decoded.Family != Family || decoded.RequestID != "request-1" ||
		decoded.ScopeID != "root" || decoded.CompilationUnitID != "unit-1" ||
		decoded.Target == nil || len(decoded.InputEchoes) != 1 ||
		decoded.InputEchoes[0].SHA256 != bad.SHA256 {
		t.Fatalf("bound rejection did not echo the request: %s", raw)
	}
}

func TestInputEnvelopeRejections(t *testing.T) {
	cases := []struct {
		name  string
		input Input
		want  string
	}{
		{"unknown input family", makeInput("input-1", "rust.other", "Cargo.toml", manifestSource), "MALFORMED_INPUT"},
		{"absolute path", makeInput("input-1", "rust.source", "/src/a.rs", "fn f() {}\n"), "INVALID_PATH"},
		{"dot dot segment", makeInput("input-1", "rust.source", "../a.rs", "fn f() {}\n"), "INVALID_PATH"},
		{"backslash path", makeInput("input-1", "rust.source", "src\\a.rs", "fn f() {}\n"), "INVALID_PATH"},
		{"bad handle", makeInput("bad handle", "rust.source", "src/a.rs", "fn f() {}\n"), "INVALID_IDENTIFIER"},
		{"family path disagreement", makeInput("input-1", "rust.manifest", "src/a.rs", manifestSource), "UNSUPPORTED_SCHEMA"},
		{"manifest at wrong name", makeInput("input-1", "rust.manifest", "NotCargo.toml", manifestSource), "UNSUPPORTED_SCHEMA"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := analyzeReason(t, tc.input); got != tc.want {
				t.Fatalf("reason=%s want=%s", got, tc.want)
			}
		})
	}
}

func TestDuplicateHandlesAndPathsReject(t *testing.T) {
	first := makeInput("input-1", "rust.source", "src/a.rs", "fn f() {}\n")
	sameHandle := makeInput("input-1", "rust.source", "src/b.rs", "fn g() {}\n")
	if got := analyzeReason(t, first, sameHandle); got != "DUPLICATE_VALUE" {
		t.Errorf("duplicate handle reason=%s want DUPLICATE_VALUE", got)
	}
	samePath := makeInput("input-2", "rust.source", "src/a.rs", "fn g() {}\n")
	if got := analyzeReason(t, first, samePath); got != "DUPLICATE_VALUE" {
		t.Errorf("duplicate path reason=%s want DUPLICATE_VALUE", got)
	}
}

func TestUnorderedInputsReject(t *testing.T) {
	second := makeInput("input-2", "rust.source", "src/b.rs", "fn g() {}\n")
	first := makeInput("input-1", "rust.source", "src/a.rs", "fn f() {}\n")
	if got := analyzeReason(t, second, first); got != "DUPLICATE_VALUE" {
		t.Fatalf("unordered inputs reason=%s want DUPLICATE_VALUE", got)
	}
}

// TestTargetMatrix pins the closed coordinate set of RAC-007.
func TestTargetMatrix(t *testing.T) {
	input := makeInput("input-1", "rust.source", "src/a.rs", "fn f() {}\n")
	accepted := []Target{
		{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}},
		{OS: "linux", Architecture: "amd64", ABI: "none", Features: []string{}},
	}
	rejected := []Target{
		{OS: "windows", Architecture: "amd64", ABI: "none", Features: []string{}},
		{OS: "darwin", Architecture: "amd64", ABI: "none", Features: []string{}},
		{OS: "darwin", Architecture: "arm64", ABI: "gnu", Features: []string{}},
		{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{"feature-1"}},
	}
	build := func(target Target) []byte {
		encoded, err := json.Marshal(Request{Profile: Profile, Family: Family, RequestID: "request-1",
			ScopeID: "root", CompilationUnitID: "unit-1", Target: target, Inputs: []Input{input}})
		if err != nil {
			t.Fatal(err)
		}
		return append(encoded, '\n')
	}
	for _, target := range accepted {
		if got := reasonOf(t, AnalyzeCanonical(build(target))); got != "" {
			t.Errorf("target %#v rejected with %s", target, got)
		}
	}
	for _, target := range rejected {
		if got := reasonOf(t, AnalyzeCanonical(build(target))); got != "UNSUPPORTED_SCHEMA" {
			t.Errorf("target %#v reason=%s want UNSUPPORTED_SCHEMA", target, got)
		}
	}
}

// TestBounds covers the byte and count ceilings of RAC-009.
func TestBounds(t *testing.T) {
	oversizeSource := strings.Repeat("// filler comment line\n", MaxSourceBytes/22+16)
	if len(oversizeSource) <= MaxSourceBytes {
		t.Fatalf("fixture is not over the source ceiling: %d", len(oversizeSource))
	}
	if got := analyzeReason(t, makeInput("input-1", "rust.source", "src/a.rs", oversizeSource)); got != "LIMIT_EXCEEDED" {
		t.Errorf("oversize source reason=%s want LIMIT_EXCEEDED", got)
	}

	oversizeManifest := "[package]\nname = \"a\"\n" + strings.Repeat("# filler\n", MaxManifestBytes/9+16)
	if got := analyzeReason(t, makeInput("input-1", "rust.manifest", "Cargo.toml", oversizeManifest)); got != "LIMIT_EXCEEDED" {
		t.Errorf("oversize manifest reason=%s want LIMIT_EXCEEDED", got)
	}

	deepComment := strings.Repeat("/*", MaxNestingDepth+2) + strings.Repeat("*/", MaxNestingDepth+2) + "\n"
	if got := analyzeReason(t, makeInput("input-1", "rust.source", "src/a.rs", deepComment)); got != "LIMIT_EXCEEDED" {
		t.Errorf("deep nesting reason=%s want LIMIT_EXCEEDED", got)
	}

	manyHashes := "const X: &str = r" + strings.Repeat("#", MaxRawStringHashes+1) + "\"x\";\n"
	if got := analyzeReason(t, makeInput("input-1", "rust.source", "src/a.rs", manyHashes)); got != "LIMIT_EXCEEDED" {
		t.Errorf("raw hash overflow reason=%s want LIMIT_EXCEEDED", got)
	}

	if got := reasonOf(t, AnalyzeCanonical(make([]byte, MaxRequestBytes+1))); got != "NONCANONICAL_REQUEST" {
		t.Errorf("oversize request reason=%s want NONCANONICAL_REQUEST", got)
	}
}

// TestOutputAccountingIsExact proves the prospective charge equals the emitted
// byte count, which is the invariant analyzeBound asserts before returning.
func TestOutputAccountingIsExact(t *testing.T) {
	raw := AnalyzeCanonical(canonicalFrame(t,
		makeInput("input-1", "rust.manifest", "Cargo.toml", manifestSource),
		makeInput("input-2", "rust.source", "src/lib.rs", hostileSource),
	))
	if got := reasonOf(t, raw); got != "" {
		t.Fatalf("expected success, got %s", got)
	}
	collector, why := newFactCollector(Request{Profile: Profile, Family: Family, RequestID: "request-1",
		ScopeID: "root", CompilationUnitID: "unit-1",
		Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}})
	if why != "" {
		t.Fatal(why)
	}
	if collector.outputSize <= 0 {
		t.Fatal("empty envelope charge must be positive")
	}
}

// TestBase64AndDigestStrictness pins RAC-009's decode discipline.
func TestBase64AndDigestStrictness(t *testing.T) {
	input := makeInput("input-1", "rust.source", "src/a.rs", "fn f() {}\n")
	nonCanonical := input
	nonCanonical.ContentBase64 = "AA=="[:3] + "="
	if got := analyzeReason(t, nonCanonical); got != "MALFORMED_INPUT" && got != "DIGEST_MISMATCH" {
		t.Errorf("bad base64 reason=%s", got)
	}
	unpadded := input
	unpadded.ContentBase64 = strings.TrimRight(input.ContentBase64, "=")
	if got := analyzeReason(t, unpadded); got == "" {
		t.Error("unpadded base64 must not be accepted")
	}
}

// TestSuccessFrameIsCanonical proves the emitted frame round-trips to itself,
// so the output is exactly one canonical object plus LF.
func TestSuccessFrameIsCanonical(t *testing.T) {
	raw := AnalyzeCanonical(canonicalFrame(t, makeInput("input-1", "rust.manifest", "Cargo.toml", manifestSource)))
	var decoded success
	if err := json.Unmarshal(raw[:len(raw)-1], &decoded); err != nil {
		t.Fatal(err)
	}
	reencoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(reencoded)+"\n" != string(raw) {
		t.Fatalf("frame is not canonical:\n got %s\nwant %s", raw, append(reencoded, '\n'))
	}
}

// TestAnalyzeIsPure proves repeated analysis of one frame is byte-identical, so
// no ambient state reaches the result.
func TestAnalyzeIsPure(t *testing.T) {
	frame := canonicalFrame(t,
		makeInput("input-1", "rust.manifest", "Cargo.toml", manifestSource),
		makeInput("input-2", "rust.source", "src/lib.rs", hostileSource),
	)
	first := string(AnalyzeCanonical(frame))
	for i := 0; i < 8; i++ {
		if got := string(AnalyzeCanonical(frame)); got != first {
			t.Fatalf("run %d diverged:\n got %s\nwant %s", i, got, first)
		}
	}
}

// TestEmptySourceIsValid pins that a zero-byte Rust file analyzes to zero facts
// rather than rejecting: an empty crate root is legitimate Rust.
func TestEmptySourceIsValid(t *testing.T) {
	result := analyzeSuccess(t, makeInput("input-1", "rust.source", "src/lib.rs", ""))
	if len(result.Facts) != 0 {
		t.Fatalf("empty source produced facts: %#v", result.Facts)
	}
}

// TestTrailingQuoteFormIsLifetimeNotUnterminatedChar pins the subtle half of the
// char-versus-lifetime rule: `'a` is a complete lifetime token, so a file ending
// in one is lexically well formed even though it is syntactically incomplete.
// This candidate is a lexer, not a parser, and does not claim the file compiles.
func TestTrailingQuoteFormIsLifetimeNotUnterminatedChar(t *testing.T) {
	for _, source := range []string{"const C: char = 'a\n", "fn f<'a>(v: &'a str) {}\n", "'outer: loop {}\n"} {
		if got := analyzeReason(t, makeInput("input-1", "rust.source", "src/a.rs", source)); got != "" {
			t.Errorf("source %q rejected with %s; a lifetime is a complete token", source, got)
		}
	}
}

// TestLifetimeDoesNotSwallowFollowingSource is the regression the reference's
// exact-review round added: treating `'` as a string delimiter made `&'a str`
// consume the rest of the file.
func TestLifetimeDoesNotSwallowFollowingSource(t *testing.T) {
	source := "fn borrow<'a>(v: &'a str) -> &'a str { v }\nmod after_lifetime;\n"
	got := factSet(analyzeSuccess(t, makeInput("input-1", "rust.source", "src/a.rs", source)).Facts)
	if !contains(got, "rust.module|declares-module|after_lifetime") {
		t.Fatalf("source after a lifetime was swallowed: %v", got)
	}
}

// TestCommentsOnlySourceIsValid is the companion to the empty-source case: a
// file whose entire content is inert must still encode an empty facts array.
func TestCommentsOnlySourceIsValid(t *testing.T) {
	source := "// just a comment\n/* and /* a nested */ block */\n"
	result := analyzeSuccess(t, makeInput("input-1", "rust.source", "src/a.rs", source))
	if len(result.Facts) != 0 {
		t.Fatalf("comments-only source produced facts: %#v", result.Facts)
	}
}

// TestBangOperatorIsNotAMacroInvocation pins that a macro call needs a delimiter
// after its bang, so the not-operator cannot be read as a dynamic macro.
func TestBangOperatorIsNotAMacroInvocation(t *testing.T) {
	source := "fn f(include: u32, other: u32) -> bool { include != other }\nmod after;\n"
	got := factSet(analyzeSuccess(t, makeInput("input-1", "rust.source", "src/a.rs", source)).Facts)
	if !contains(got, "rust.module|declares-module|after") {
		t.Fatalf("bang operator disrupted the scan: %v", got)
	}
}

// TestOrdinaryMacroIsInert pins that a non-dynamic macro yields no fact and does
// not disturb the surrounding scan.
func TestOrdinaryMacroIsInert(t *testing.T) {
	source := "fn f() { let v = vec![1, 2]; println!(\"{}\", v.len()); }\nmod after;\n"
	got := factSet(analyzeSuccess(t, makeInput("input-1", "rust.source", "src/a.rs", source)).Facts)
	if !contains(got, "rust.module|declares-module|after") {
		t.Fatalf("macro invocation disrupted the scan: %v", got)
	}
	for _, value := range got {
		if strings.Contains(value, "vec") || strings.Contains(value, "println") {
			t.Errorf("macro name became a fact: %s", value)
		}
	}
}

func TestMacroBodiesAreInert(t *testing.T) {
	source := "macro_rules! declared { () => { mod hidden_definition; use fake::definition; } }\n" +
		"ordinary! { mod hidden_invocation; use fake::invocation; }\nmod visible;\n"
	got := factSet(analyzeSuccess(t, makeInput("input-1", "rust.source", "src/a.rs", source)).Facts)
	for _, value := range []string{"hidden_definition", "fake::definition", "hidden_invocation", "fake::invocation"} {
		for _, fact := range got {
			if strings.Contains(fact, value) {
				t.Errorf("macro body emitted %q: %v", value, got)
			}
		}
	}
	if !contains(got, "rust.module|declares-module|visible") {
		t.Fatalf("source after macros was lost: %v", got)
	}
}

func TestAttributePayloadIsInert(t *testing.T) {
	source := "#[custom(mod hidden; use fake::path;)]\npub struct Visible;\n"
	got := factSet(analyzeSuccess(t, makeInput("input-1", "rust.source", "src/a.rs", source)).Facts)
	for _, value := range []string{"rust.module|declares-module|hidden", "rust.import.static|imports|fake::path"} {
		if contains(got, value) {
			t.Errorf("attribute payload emitted %q: %v", value, got)
		}
	}
	if !contains(got, "rust.declaration|declares-struct|Visible") {
		t.Fatalf("attributed declaration was lost: %v", got)
	}
}

func TestRawStringTerminatorRequiresExactHashRun(t *testing.T) {
	source := "const RAW: &str = r#\"mod hidden;\"##;\nmod visible;\n"
	if got := analyzeReason(t, makeInput("input-1", "rust.source", "src/a.rs", source)); got != "MALFORMED_INPUT" {
		t.Fatalf("reason=%s want MALFORMED_INPUT", got)
	}
}

func TestCharacterEscapeCannotConsumeLineFeed(t *testing.T) {
	source := "const BAD: char = '\\\n';\nmod visible;\n"
	if got := analyzeReason(t, makeInput("input-1", "rust.source", "src/a.rs", source)); got != "MALFORMED_INPUT" {
		t.Fatalf("reason=%s want MALFORMED_INPUT", got)
	}
}

// TestTypePositionConstDeclaresNothing pins that `const` inside a raw pointer
// type or a generic parameter list is not a const item, so neither the pointee
// type nor the const-generic parameter becomes a declaration fact.
func TestTypePositionConstDeclaresNothing(t *testing.T) {
	source := "fn f(p: *const u8) {}\nstruct A<const N: usize, const M: u8>;\nconst LIMIT: u8 = 1;\n"
	got := factSet(analyzeSuccess(t, makeInput("input-1", "rust.source", "src/a.rs", source)).Facts)
	for _, value := range []string{"u8", "N", "M"} {
		if contains(got, "rust.declaration|declares-const|"+value) {
			t.Errorf("type-position const declared %s: %v", value, got)
		}
	}
	if !contains(got, "rust.declaration|declares-const|LIMIT") {
		t.Fatalf("const item was lost: %v", got)
	}
}

// TestTestAttributeBindsOnlyTheNextItem pins that #[test] marks only the item it
// annotates: a non-fn item or a macro-template fn ends the mark instead of
// leaking it onto a later helper function.
func TestTestAttributeBindsOnlyTheNextItem(t *testing.T) {
	for _, source := range []string{
		"#[test]\nmod m {}\nfn helper() {}\n",
		"#[test]\nmod m;\nfn helper() {}\n",
		"#[test]\nuse a::b;\nfn helper() {}\n",
		"#[test]\nordinary! { fn hidden() {} }\nfn helper() {}\n",
		"macro_rules! t { ($n:ident) => { #[test] fn $n() {} } }\nfn helper() {}\n",
	} {
		got := factSet(analyzeSuccess(t, makeInput("input-1", "rust.source", "src/a.rs", source)).Facts)
		if contains(got, "rust.test|declares-test|helper") {
			t.Errorf("%q: #[test] leaked onto helper: %v", source, got)
		}
	}
	source := "#[test]\n#[should_panic]\npub async fn real() {}\nfn helper() {}\n"
	got := factSet(analyzeSuccess(t, makeInput("input-1", "rust.source", "src/a.rs", source)).Facts)
	if !contains(got, "rust.test|declares-test|real") || contains(got, "rust.test|declares-test|helper") {
		t.Fatalf("#[test] did not bind exactly the next fn: %v", got)
	}
}
