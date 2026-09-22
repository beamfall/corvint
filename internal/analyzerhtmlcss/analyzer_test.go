package analyzerhtmlcss

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// These are literal, trailing-LF-pinned source bytes: Core's app-dist placeholder CSS at
// da38c59eb30b2121cbac37b912485b30b2e54841 and Web's app HTML at
// b924fe0ae0d36af08b2a1d50be9383381f809cc0. They deliberately remain rejected by this closed
// candidate grammar; a coordinate is not a source-parser admission claim.
const corePinnedCSS = "html {\n  color-scheme: dark;\n  font-family: system-ui, sans-serif;\n}\n"
const webPinnedHTML = "<!doctype html>\n<html lang=\"en\">\n  <head>\n    <meta charset=\"utf-8\" />\n    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n    <title>Beamfall</title>\n    <link rel=\"stylesheet\" href=\"/styles.css\" />\n  </head>\n  <body>\n    <div id=\"root\"></div>\n    <script type=\"module\" src=\"./main.tsx\"></script>\n  </body>\n</html>\n"

func framedRequest(t *testing.T, inputs ...input) []byte {
	t.Helper()
	r := request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: inputs}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}
func fixtureInput(handle, family, path, content string) input {
	d := sha256.Sum256([]byte(content))
	return input{Handle: handle, Family: family, Path: path, SHA256: "sha256:" + hex.EncodeToString(d[:]), ContentBase64: base64.StdEncoding.EncodeToString([]byte(content))}
}

func TestAnalyzeCanonicalHTMLCSS(t *testing.T) {
	request := framedRequest(t,
		fixtureInput("css-1", "css.stylesheet", "assets/site.css", `@import "theme/base.css";:root{--brand:blue;}.hero{background:url("images/hero.png");}`),
		fixtureInput("html-1", "html.document", "index.html", `<html><head><link rel="stylesheet" href="assets/site.css"></head><body><a href="pages/about.html"></a><script type="module" src="app/main.js"></script><img src="images/logo.png"></body></html>`),
	)
	got := Analyze(request)
	var out success
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatal(err)
	}
	if canonical, _ := json.Marshal(out); string(got) != string(canonical)+"\n" {
		t.Fatalf("noncanonical output: %s", got)
	}
	if len(out.Facts) != 7 {
		t.Fatalf("facts=%d want 7: %s", len(out.Facts), got)
	}
	for i := 1; i < len(out.Facts); i++ {
		if compareFact(out.Facts[i-1], out.Facts[i]) >= 0 {
			t.Fatal("facts not strictly ordered")
		}
	}
	if out.Profile != Profile || out.Family != Family || out.Status != "CANDIDATE" || len(out.InputEchoes) != 2 || out.InputEchoes[0].Family != "css.stylesheet" || out.InputEchoes[0].Path != "assets/site.css" {
		t.Fatalf("bad envelope: %#v", out)
	}
}

func TestPinnedCoreWebBytesStayLiteralAndRejected(t *testing.T) {
	if got := sha256.Sum256([]byte(corePinnedCSS)); hex.EncodeToString(got[:]) != "4291d6dbb81d8a9aa1ac7de1baf733e01aef55f1e4512f38f8bee97dfd513e38" {
		t.Fatalf("Core fixture bytes drifted: %x", got)
	}
	if got := sha256.Sum256([]byte(webPinnedHTML)); hex.EncodeToString(got[:]) != "83ae406fbaae7b63c73cf6f9e1721fb6597e6267e71551844d5487a949fc69e1" {
		t.Fatalf("Web fixture bytes drifted: %x", got)
	}
	for _, tc := range []struct {
		name string
		in   input
	}{
		{"core-css", fixtureInput("css-1", "css.stylesheet", "assets/placeholder.css", corePinnedCSS)},
		{"web-html", fixtureInput("html-1", "html.document", "index.html", webPinnedHTML)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := string(Analyze(framedRequest(t, tc.in)))
			if !strings.Contains(got, `"reason":"UNSUPPORTED_SCHEMA"`) {
				t.Fatalf("pinned bytes unexpectedly admitted: %s", got)
			}
		})
	}
}

func TestDuplicateHandleAndPathRejectBeforeBodyRetention(t *testing.T) {
	first := fixtureInput("same", "css.stylesheet", "a.css", `:root{--x:a;}`)
	second := fixtureInput("same", "html.document", "b.html", `<html><head></head><body></body></html>`)
	got := string(Analyze(framedRequest(t, first, second)))
	if got != `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"DUPLICATE_VALUE"}`+"\n" {
		t.Fatalf("handle duplicate=%s", got)
	}
	second = fixtureInput("other", "html.document", "a.css", `<html><head></head><body></body></html>`)
	got = string(Analyze(framedRequest(t, first, second)))
	if !strings.Contains(got, `"reason":"DUPLICATE_VALUE"`) || strings.Contains(got, `"input_echoes"`) {
		t.Fatalf("path duplicate=%s", got)
	}
}

func TestClosedGrammarAndEnvelopeReasonBoundaries(t *testing.T) {
	valid := fixtureInput("html-1", "html.document", "index.html", `<html><head></head><body></body></html>`)
	cases := []struct {
		name   string
		mutate func(*request)
		want   string
	}{
		{"nil-features", func(r *request) { r.Target.Features = nil }, "NONCANONICAL_REQUEST"},
		{"too-many-features", func(r *request) {
			r.Target.Features = make([]string, 65)
			for i := range r.Target.Features {
				r.Target.Features[i] = "f" + string(rune('a'+i%26))
			}
		}, "LIMIT_EXCEEDED"},
		{"bad-handle", func(r *request) { r.Inputs[0].Handle = "bad handle" }, "INVALID_IDENTIFIER"},
		{"bad-input-family", func(r *request) { r.Inputs[0].Family = "bad family" }, "INVALID_IDENTIFIER"},
		{"bad-digest", func(r *request) { r.Inputs[0].SHA256 = "sha256:bad" }, "DIGEST_MISMATCH"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var r request
			if err := json.Unmarshal(framedRequest(t, valid), &r); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&r)
			frame, _ := json.Marshal(r)
			got := string(Analyze(append(frame, '\n')))
			want := `"reason":"` + tc.want + `"`
			if !strings.Contains(got, want) || strings.Contains(got, `"input_echoes"`) {
				t.Fatalf("%s", got)
			}
		})
	}
	for _, source := range []string{`<html><head></head><body><!--x--></body></html>`, `<html><head></head><body><a href="a.html">text</a></body></html>`, `<HTML><head></head><body></body></HTML>`} {
		in := fixtureInput("html-1", "html.document", "index.html", source)
		if got := string(Analyze(framedRequest(t, in))); !strings.Contains(got, `"reason":"UNSUPPORTED_SCHEMA"`) {
			t.Fatalf("html %q: %s", source, got)
		}
	}
	for _, source := range []string{`.x{--a:b;}@import "a.css";`, `.x {--a:b;}`} {
		in := fixtureInput("css-1", "css.stylesheet", "a.css", source)
		if got := string(Analyze(framedRequest(t, in))); !strings.Contains(got, `"reason":"UNSUPPORTED_SCHEMA"`) && !strings.Contains(got, `"reason":"MALFORMED_INPUT"`) {
			t.Fatalf("css %q: %s", source, got)
		}
	}
}

func TestKnownFieldShortestEscapeCanonicalization(t *testing.T) {
	valid := fixtureInput("html-1", "html.document", "index.html", `<html><head></head><body></body></html>`)
	frame := string(framedRequest(t, valid))
	cases := []struct {
		name    string
		literal string
		escaped string
	}{
		{"less-than", "<", `\u003c`},
		{"greater-than", ">", `\u003e`},
		{"ampersand", "&", `\u0026`},
		{"line-separator", "\u2028", `\u2028`},
		{"paragraph-separator", "\u2029", `\u2029`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			literalFrame := strings.Replace(frame, "index.html", "index"+tc.literal+".html", 1)
			if got := string(Analyze([]byte(literalFrame))); got != sentinelGolden("INVALID_PATH") {
				t.Fatalf("literal spelling got %q want INVALID_PATH sentinel", got)
			}
			escapedFrame := strings.Replace(frame, "index.html", "index"+tc.escaped+".html", 1)
			if got := string(Analyze([]byte(escapedFrame))); got != sentinelGolden("NONCANONICAL_REQUEST") {
				t.Fatalf("escaped spelling got %q want NONCANONICAL_REQUEST sentinel", got)
			}
		})
	}
}

func TestAnalyzeRejectsAmbiguityAndEchoesFullEnvelope(t *testing.T) {
	request := framedRequest(t, fixtureInput("html-1", "html.document", "index.html", `<html><head></head><body><a href="https://example.invalid"></a></body></html>`))
	got := Analyze(request)
	var out rejected
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatal(err)
	}
	if out.Reason != "UNSUPPORTED_SCHEMA" || out.ScopeID != "root" || out.CompilationUnitID != "unit-1" || len(out.InputEchoes) != 1 || out.Target.OS != "darwin" {
		t.Fatalf("missing full rejection echo: %s", got)
	}
}

func TestAnalyzeRejectsNoncanonicalAndDuplicateJSON(t *testing.T) {
	if got := string(Analyze([]byte(`{"profile":"corvint-analyzer-candidate/experimental","profile":"corvint-analyzer-candidate/experimental"}` + "\n"))); got != `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}`+"\n" {
		t.Fatalf("got %s", got)
	}
	request := framedRequest(t, fixtureInput("html-1", "html.document", "index.html", `<html><head></head><body></body></html>`))
	if got := string(Analyze(append([]byte(" "), request...))); !strings.Contains(got, `"reason":"NONCANONICAL_REQUEST"`) {
		t.Fatalf("got %s", got)
	}
}

func TestUnknownFieldFollowsCanonicalityAndSafeEnvelope(t *testing.T) {
	base := string(framedRequest(t, fixtureInput("html-1", "html.document", "index.html", `<html><head></head><body></body></html>`)))
	canonicalUnknown := strings.TrimSuffix(base, "}\n") + `,"future":"field"}` + "\n"
	if got := string(Analyze([]byte(canonicalUnknown))); !strings.Contains(got, `"reason":"UNKNOWN_FIELD"`) || !strings.Contains(got, `"path":"index.html"`) {
		t.Fatalf("canonical unknown field=%q", got)
	}
	for _, value := range []string{`"a\"b"`, `9007199254740993`, `-9007199254740993`, `[9007199254740993,{"nested":-9007199254740993}]`} {
		frame := strings.Replace(canonicalUnknown, `"future":"field"`, `"future":`+value, 1)
		if got := string(Analyze([]byte(frame))); !strings.Contains(got, `"reason":"UNKNOWN_FIELD"`) {
			t.Fatalf("canonical unknown extension %s=%q", value, got)
		}
	}
	for _, frame := range []string{
		" " + canonicalUnknown,
		strings.Replace(canonicalUnknown, `,"future"`, `, "future"`, 1),
		strings.Replace(canonicalUnknown, `"future":"field"`, `"future":"fi\u0065ld"`, 1),
		strings.Replace(canonicalUnknown, `"future":"field"`, `"future":1.0`, 1),
		strings.Replace(canonicalUnknown, `"future":"field"`, `"future":-0`, 1),
		strings.Replace(canonicalUnknown, `"future":"field"`, `"future":1e3`, 1),
		strings.Replace(canonicalUnknown, `"future":"field"`, `"future":"first","future":"second"`, 1),
	} {
		if got := string(Analyze([]byte(frame))); got != sentinelGolden("NONCANONICAL_REQUEST") {
			t.Fatalf("noncanonical unknown field=%q", got)
		}
	}
	unsafe := strings.Replace(canonicalUnknown, `"path":"index.html"`, `"path":"bad:path"`, 1)
	if got := string(Analyze([]byte(unsafe))); got != sentinelGolden("INVALID_PATH") {
		t.Fatalf("unsafe unknown field=%q", got)
	}
}

func TestPathsMustBeGloballyUnique(t *testing.T) {
	r := request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []input{
		fixtureInput("a", "html.document", "pages/same.html", `<html><head></head><body></body></html>`),
		fixtureInput("b", "html.document", "pages/other.html", `<html><head></head><body></body></html>`),
		fixtureInput("c", "html.document", "pages/same.html", `<html><head></head><body></body></html>`),
	}}
	if got, want := string(Analyze(rawRequest(t, r))), sentinelGolden("DUPLICATE_VALUE"); got != want {
		t.Fatalf("nonadjacent duplicate paths got %q want %q", got, want)
	}
}

func TestSentinelReasonsAreClosed(t *testing.T) {
	for _, reason := range []string{"MALFORMED_INPUT", "UNSUPPORTED_SCHEMA", "UNKNOWN_FIELD", "OUTPUT_LIMIT", "ANALYZER_FAILURE", ""} {
		if got := string(sentinel(reason)); got != sentinelGolden("NONCANONICAL_REQUEST") {
			t.Fatalf("open sentinel reason %q: %q", reason, got)
		}
	}
}

func TestCompleteRequestBindingAndDistinctInputProfiles(t *testing.T) {
	if HTMLInputProfile == CSSInputProfile || HTMLInputProfile == "" || CSSInputProfile == "" {
		t.Fatalf("input profiles must be independent: %q %q", HTMLInputProfile, CSSInputProfile)
	}
	if profile, ok := inputProfile("html.document"); !ok || profile != HTMLInputProfile {
		t.Fatalf("HTML profile=%q ok=%t", profile, ok)
	}
	if profile, ok := inputProfile("css.stylesheet"); !ok || profile != CSSInputProfile {
		t.Fatalf("CSS profile=%q ok=%t", profile, ok)
	}
	first := fixtureInput("html-1", "html.document", "index.html", `<html><head></head><body><a href="pages/about.html"></a></body></html>`)
	second := first
	second.Path = "pages/index.html"
	var firstOut, secondOut success
	if err := json.Unmarshal(Analyze(framedRequest(t, first)), &firstOut); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(Analyze(framedRequest(t, second)), &secondOut); err != nil {
		t.Fatal(err)
	}
	if firstOut.InputEchoes[0].Family != first.Family || firstOut.InputEchoes[0].Path != first.Path {
		t.Fatalf("first request binding=%#v", firstOut.InputEchoes[0])
	}
	if secondOut.InputEchoes[0].Path != second.Path || firstOut.Facts[0].EvidenceSHA256 == secondOut.Facts[0].EvidenceSHA256 {
		t.Fatalf("path must be output and evidence-bound: %s %s", firstOut.Facts[0].EvidenceSHA256, secondOut.Facts[0].EvidenceSHA256)
	}
}

func TestEvidenceBindingMetamorphicMatrix(t *testing.T) {
	base := request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []input{
		fixtureInput("html-1", "html.document", "index.html", `<html><head></head><body><a href="pages/about.html"></a></body></html>`),
	}}
	fact := htmlFact("html.link.static", base.Inputs[0], "links", "pages/about.html")
	baseEvidence := evidenceFor(t, base, fact)
	for _, tc := range []struct {
		name   string
		mutate func(*request)
	}{
		{"input-family", func(r *request) { r.Inputs[0].Family = "css.stylesheet" }},
		{"content-and-digest", func(r *request) {
			r.Inputs[0] = fixtureInput("html-1", "html.document", "index.html", `<html><head></head><body><a href="pages/contact.html"></a></body></html>`)
		}},
		{"target", func(r *request) { r.Target.OS = "linux" }},
		{"scope", func(r *request) { r.ScopeID = "child" }},
		{"compilation-unit", func(r *request) { r.CompilationUnitID = "unit-2" }},
		{"request-id", func(r *request) { r.RequestID = "request-2" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := cloneRequest(base)
			tc.mutate(&mutated)
			if got := evidenceFor(t, mutated, fact); got == baseEvidence {
				t.Fatalf("%s did not change evidence %q", tc.name, got)
			}
		})
	}
}

func evidenceFor(t *testing.T, r request, f fact) string {
	t.Helper()
	collector, reason := newFactCollector(r)
	if reason != "" {
		t.Fatal(reason)
	}
	evidence, ok := collector.evidence(f)
	if !ok {
		t.Fatal("missing evidence")
	}
	return evidence
}

func cloneRequest(r request) request {
	copy := r
	copy.Target.Features = slices.Clone(r.Target.Features)
	copy.Inputs = slices.Clone(r.Inputs)
	return copy
}

func TestHTMLCSSPositiveControlMatrix(t *testing.T) {
	css := fixtureInput("css-1", "css.stylesheet", "assets/site.css", `@import url("theme/base.css");.one{background:url("images/background.png");background-image:url("images/background-image.png");content:url("images/content.png");src:url("fonts/site.woff2");}`)
	html := fixtureInput("html-1", "html.document", "index.html", `<html><head><link rel="stylesheet" href="assets/site.css"><script type="module" src="app/head.js"></script></head><body><a href="pages/about.html"></a><script type="module" src="app/body.js"></script><img src="images/logo.png"><source src="media/source.mp4"><audio src="media/audio.mp3"></audio><video poster="images/poster.png"></video></body></html>`)
	var out success
	if err := json.Unmarshal(Analyze(framedRequest(t, css, html)), &out); err != nil {
		t.Fatal(err)
	}
	want := map[string]struct{}{
		"css.asset.reference|css-1|-|assets/site.css|references-asset|images/background.png|assets/site.css":       {},
		"css.asset.reference|css-1|-|assets/site.css|references-asset|images/background-image.png|assets/site.css": {},
		"css.asset.reference|css-1|-|assets/site.css|references-asset|images/content.png|assets/site.css":          {},
		"css.asset.reference|css-1|-|assets/site.css|references-asset|fonts/site.woff2|assets/site.css":            {},
		"css.import.static|css-1|-|assets/site.css|imports-stylesheet|theme/base.css|assets/site.css":              {},
		"html.asset.reference|html-1|-|index.html|references-asset|images/logo.png|index.html":                     {},
		"html.asset.reference|html-1|-|index.html|references-asset|media/source.mp4|index.html":                    {},
		"html.asset.reference|html-1|-|index.html|references-asset|media/audio.mp3|index.html":                     {},
		"html.asset.reference|html-1|-|index.html|references-asset|images/poster.png|index.html":                   {},
		"html.link.static|html-1|-|index.html|links|pages/about.html|index.html":                                   {},
		"html.module.import|html-1|-|index.html|imports-module|app/head.js|index.html":                             {},
		"html.module.import|html-1|-|index.html|imports-module|app/body.js|index.html":                             {},
		"html.stylesheet.link|html-1|-|index.html|imports-stylesheet|assets/site.css|index.html":                   {},
	}
	for _, got := range out.Facts {
		key := strings.Join([]string{got.Kind, got.InputHandle, got.RelatedHandle, got.Subject, got.Predicate, got.Value, got.InstanceID}, "|")
		if _, ok := want[key]; !ok {
			t.Fatalf("unexpected reference tuple %q", key)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing reference tuples %v", want)
	}
	for _, tc := range []struct {
		family string
		path   string
		body   string
	}{
		{"html.document", "index.html", `<html><head><link rel="stylesheet" href="assets/site.css" media="all"></head><body></body></html>`},
		{"html.document", "index.html", `<html><head><script type="module" src="app/head.js" defer="defer"></script></head><body></body></html>`},
		{"html.document", "index.html", `<html><head></head><body><a href="pages/about.html" target="_blank"></a></body></html>`},
		{"html.document", "index.html", `<html><head></head><body><script type="module" src="app/body.js" defer="defer"></script></body></html>`},
		{"html.document", "index.html", `<html><head></head><body><img src="images/logo.png" alt="logo"></body></html>`},
		{"html.document", "index.html", `<html><head></head><body><source src="media/source.mp4" type="video/mp4"></body></html>`},
		{"html.document", "index.html", `<html><head></head><body><audio src="media/audio.mp3" loop="loop"></audio></body></html>`},
		{"html.document", "index.html", `<html><head></head><body><video src="media/video.mp4"></video></body></html>`},
		{"css.stylesheet", "assets/site.css", `@import url("theme/base.css")`},
		{"css.stylesheet", "assets/site.css", `.one{background:"images/background.png";}`},
		{"css.stylesheet", "assets/site.css", `.one{background-image:"images/background-image.png";}`},
		{"css.stylesheet", "assets/site.css", `.one{content:"images/content.png";}`},
		{"css.stylesheet", "assets/site.css", `.one{src:"fonts/site.woff2";}`},
	} {
		in := fixtureInput("input-1", tc.family, tc.path, tc.body)
		r := request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []input{in}}
		if got, want := string(Analyze(framedRequest(t, in))), fullFailureGolden(r, "UNSUPPORTED_SCHEMA"); got != want {
			t.Fatalf("reference complement got %q want %q", got, want)
		}
	}
}

func TestParserSyntaxAlwaysUsesUnsupportedSchema(t *testing.T) {
	for _, tc := range []struct {
		family string
		path   string
		body   string
	}{
		{"html.document", "index.html", `<html><head><link rel="stylesheet" href="a.css"></head><body>`},
		{"html.document", "index.html", `<html><head></head><body><audio src="a.mp3"></body></html>`},
		{"css.stylesheet", "site.css", `@import "a.css"`},
		{"css.stylesheet", "site.css", `.x{background:url("a.png");`},
		{"css.stylesheet", "site.css", `.x{background:url("a.png");;}`},
	} {
		in := fixtureInput("input-1", tc.family, tc.path, tc.body)
		if got := string(Analyze(framedRequest(t, in))); !strings.Contains(got, `"reason":"UNSUPPORTED_SCHEMA"`) {
			t.Fatalf("%s syntax=%q", tc.family, got)
		}
	}
}

func TestValidationPrecedesEveryParser(t *testing.T) {
	invalidSyntax := fixtureInput("html-1", "html.document", "index.html", `<html><head></head><body><a href="a.html"></body></html>`)
	badDigest := fixtureInput("html-2", "html.document", "other.html", `<html><head></head><body></body></html>`)
	badDigest.SHA256 = "sha256:" + strings.Repeat("0", 64)
	if got := string(Analyze(framedRequest(t, invalidSyntax, badDigest))); !strings.Contains(got, `"reason":"DIGEST_MISMATCH"`) {
		t.Fatalf("validation must precede syntax parsing: %q", got)
	}
}

func TestAnalyzerIsBounded(t *testing.T) {
	large := strings.Repeat("a", maxContentBytes+1)
	in := fixtureInput("html-1", "html.document", "index.html", large)
	got := string(Analyze(framedRequest(t, in)))
	if !strings.Contains(got, `"reason":"MALFORMED_INPUT"`) && !strings.Contains(got, `"reason":"LIMIT_EXCEEDED"`) {
		t.Fatalf("got %s", got)
	}
}

func TestBenchmarkFixtureHasFourInputsAndThirtyTwoFacts(t *testing.T) {
	var r request
	if err := json.Unmarshal(benchmarkRequest(), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Inputs) != 4 {
		t.Fatalf("inputs=%d", len(r.Inputs))
	}
	var out success
	if err := json.Unmarshal(Analyze(benchmarkRequest()), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Facts) != 32 {
		t.Fatalf("facts=%d", len(out.Facts))
	}
}

func TestPinnedRepresentativeCoordinates(t *testing.T) {
	if SourceCoordinate != "beamfall-core@da38c59eb30b2121cbac37b912485b30b2e54841" || WebSourceCoordinate != "beamfall-web@b924fe0ae0d36af08b2a1d50be9383381f809cc0" || ToolchainCoordinate != "go1.27.0" {
		t.Fatalf("coordinates drifted: %q %q %q", SourceCoordinate, WebSourceCoordinate, ToolchainCoordinate)
	}
}

const canonicalSuccessGolden = `{"profile":"corvint-analyzer-candidate/experimental","family":"html-css","request_id":"request-1","status":"CANDIDATE","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"css-1","family":"css.stylesheet","path":"assets/site.css","sha256":"sha256:134b8c6f92b56609f6d949c823c7b6b34adefb3302bd8c006915ee1bcfa2e71a"},{"handle":"html-1","family":"html.document","path":"index.html","sha256":"sha256:8b205c754cae41c96603b8e0ecf37202e939b2a2f37578cc916fd015248d1bd8"}],"facts":[{"kind":"css.asset.reference","input_handle":"css-1","related_handle":"-","subject":"assets/site.css","predicate":"references-asset","value":"images/hero.png","instance_id":"assets/site.css","evidence_sha256":"sha256:1da4c640487f56b35b687a7982983b0614113521d6f33b77cbe7241683343fd3"},{"kind":"css.custom-property.declaration","input_handle":"css-1","related_handle":"-","subject":"--brand","predicate":"declares","value":"blue","instance_id":"assets/site.css","evidence_sha256":"sha256:b6e41090ac965e13bbae716a238ea47a579e6ff9fd777cba098e341651e03275"},{"kind":"css.import.static","input_handle":"css-1","related_handle":"-","subject":"assets/site.css","predicate":"imports-stylesheet","value":"theme/base.css","instance_id":"assets/site.css","evidence_sha256":"sha256:bc95da04f2ce27f7047e43f05bb6ae964779025a064293b91f8fe23dab18669c"},{"kind":"html.asset.reference","input_handle":"html-1","related_handle":"-","subject":"index.html","predicate":"references-asset","value":"images/logo.png","instance_id":"index.html","evidence_sha256":"sha256:97818846cceae43a025d40599cdc7294ce763d29b9b83e765ba8da883e8c2271"},{"kind":"html.link.static","input_handle":"html-1","related_handle":"-","subject":"index.html","predicate":"links","value":"pages/about.html","instance_id":"index.html","evidence_sha256":"sha256:d3c16689a928cec69b765ee853fa56ceafcb876747a74cb6b62ecf1d2531a343"},{"kind":"html.module.import","input_handle":"html-1","related_handle":"-","subject":"index.html","predicate":"imports-module","value":"app/main.js","instance_id":"index.html","evidence_sha256":"sha256:da904cc5a616eddf5fdf4bec3a80590bdf140e02302d0032ca20513a10640f15"},{"kind":"html.stylesheet.link","input_handle":"html-1","related_handle":"-","subject":"index.html","predicate":"imports-stylesheet","value":"assets/site.css","instance_id":"index.html","evidence_sha256":"sha256:1ff269f6b1fcf5720368b9c983343a22eaefdeee35effaeedb86aba3f146cd69"}]}` + "\n"

func TestExactSuccessAndEvidenceGolden(t *testing.T) {
	frame := framedRequest(t,
		fixtureInput("css-1", "css.stylesheet", "assets/site.css", `@import "theme/base.css";:root{--brand:blue;}.hero{background:url("images/hero.png");}`),
		fixtureInput("html-1", "html.document", "index.html", `<html><head><link rel="stylesheet" href="assets/site.css"></head><body><a href="pages/about.html"></a><script type="module" src="app/main.js"></script><img src="images/logo.png"></body></html>`),
	)
	if got := string(Analyze(frame)); got != canonicalSuccessGolden {
		t.Fatalf("success/evidence golden drifted:\n%s", got)
	}
}

func TestExactFailureGoldensAndEchoEligibility(t *testing.T) {
	base := fixtureInput("html-1", "html.document", "index.html", `<html><head></head><body></body></html>`)
	var r request
	if err := json.Unmarshal(framedRequest(t, base), &r); err != nil {
		t.Fatal(err)
	}
	sentinelCases := []struct {
		name   string
		mutate func(*request)
		reason string
	}{
		{"invalid-feature", func(r *request) { r.Target.Features = []string{"bad feature"} }, "INVALID_IDENTIFIER"},
		{"features-null", func(r *request) { r.Target.Features = nil }, "NONCANONICAL_REQUEST"},
		{"unknown-family", func(r *request) { r.Family = "future" }, "UNKNOWN_FAMILY"},
		{"invalid-path", func(r *request) { r.Inputs[0].Path = "bad:path" }, "INVALID_PATH"},
		{"duplicate-feature", func(r *request) { r.Target.Features = []string{"a", "a"} }, "DUPLICATE_VALUE"},
	}
	for _, tc := range sentinelCases {
		t.Run(tc.name, func(t *testing.T) {
			copy := r
			copy.Target.Features = slices.Clone(r.Target.Features)
			copy.Inputs = slices.Clone(r.Inputs)
			tc.mutate(&copy)
			if got, want := string(Analyze(rawRequest(t, copy))), sentinelGolden(tc.reason); got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
	fullCases := []struct {
		name   string
		mutate func(*request)
		reason string
	}{
		{"digest-mismatch", func(r *request) { r.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64) }, "DIGEST_MISMATCH"},
		{"malformed-base64", func(r *request) { r.Inputs[0].ContentBase64 = "!" }, "MALFORMED_INPUT"},
		{"unknown-profile-stays-bound", func(r *request) { r.Profile = "corvint-analyzer-candidate/future" }, "NONCANONICAL_REQUEST"},
		{"known-but-unsupported-family", func(r *request) { r.Family = "go" }, "UNSUPPORTED_SCHEMA"},
		{"closed-envelope-family-python", func(r *request) { r.Family = "python" }, "UNSUPPORTED_SCHEMA"},
		{"closed-envelope-family-swift-apple", func(r *request) { r.Family = "swift-apple" }, "UNSUPPORTED_SCHEMA"},
		{"imports-without-rule", func(r *request) {
			r.Inputs[0] = fixtureInput("html-1", "css.stylesheet", "index.html", `@import "a.css";`)
		}, "UNSUPPORTED_SCHEMA"},
		{"conflicting-declaration", func(r *request) {
			r.Inputs[0] = fixtureInput("html-1", "css.stylesheet", "index.html", `.x{--a:one;--a:two;}`)
		}, "CONFLICTING_VALUE"},
		{"conflicting-declaration-across-rules", func(r *request) {
			r.Inputs[0] = fixtureInput("html-1", "css.stylesheet", "index.html", `:root{--a:one;}.dark{--a:two;}`)
		}, "CONFLICTING_VALUE"},
		{"duplicate-declaration", func(r *request) {
			r.Inputs[0] = fixtureInput("html-1", "css.stylesheet", "index.html", `.x{--a:one;--a:one;}`)
		}, "DUPLICATE_VALUE"},
		{"asset-must-use-url", func(r *request) {
			r.Inputs[0] = fixtureInput("html-1", "css.stylesheet", "index.html", `.x{background:"a.png";}`)
		}, "UNSUPPORTED_SCHEMA"},
		{"unsupported-input-family", func(r *request) {
			r.Inputs[0].Family = "future.document"
		}, "UNSUPPORTED_SCHEMA"},
	}
	for _, tc := range fullCases {
		t.Run(tc.name, func(t *testing.T) {
			copy := r
			copy.Target.Features = slices.Clone(r.Target.Features)
			copy.Inputs = slices.Clone(r.Inputs)
			tc.mutate(&copy)
			if got, want := string(Analyze(rawRequest(t, copy))), fullFailureGolden(copy, tc.reason); got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
	unknown := strings.TrimSuffix(string(rawRequest(t, r)), "\n")
	unknown = strings.TrimSuffix(unknown, "}") + `,"future":"field"}` + "\n"
	if got, want := string(Analyze([]byte(unknown))), fullFailureGolden(r, "UNKNOWN_FIELD"); got != want {
		t.Fatalf("unknown field got %q want %q", got, want)
	}
	if got, want := string(Analyze(make([]byte, maxRequestBytes+1))), sentinelGolden("LIMIT_EXCEEDED"); got != want {
		t.Fatalf("request limit got %q want %q", got, want)
	}
}

func TestProspectiveJSONAndDecodedBounds(t *testing.T) {
	base := fixtureInput("html-1", "html.document", "index.html", `<html><head></head><body></body></html>`)
	var r request
	if err := json.Unmarshal(framedRequest(t, base), &r); err != nil {
		t.Fatal(err)
	}
	r.Profile = strings.Repeat("p", maxJSONOrdinaryString+1)
	if got, want := string(Analyze(rawRequest(t, r))), sentinelGolden("LIMIT_EXCEEDED"); got != want {
		t.Fatalf("ordinary string bound got %q want %q", got, want)
	}
	r.Profile = Profile
	r.Inputs[0].ContentBase64 = strings.Repeat("A", maxBase64Bytes+1)
	if got, want := string(Analyze(rawRequest(t, r))), sentinelGolden("LIMIT_EXCEEDED"); got != want {
		t.Fatalf("base64 bound got %q want %q", got, want)
	}
	r = request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []input{
		fixtureInput("a", "html.document", "a.html", strings.Repeat("x", maxContentBytes/2+1)),
		fixtureInput("b", "html.document", "b.html", strings.Repeat("x", maxContentBytes/2)),
	}}
	if got, want := string(Analyze(rawRequest(t, r))), fullFailureGolden(r, "LIMIT_EXCEEDED"); got != want {
		t.Fatalf("aggregate decoded bound got %q want %q", got, want)
	}
	tokenFrame := strings.TrimSuffix(string(framedRequest(t, base)), "\n")
	tokenFrame = strings.TrimSuffix(tokenFrame, "}") + `,"future":[` + strings.Repeat("null,", maxJSONTokens) + `null]}` + "\n"
	if got, want := string(Analyze([]byte(tokenFrame))), sentinelGolden("LIMIT_EXCEEDED"); got != want {
		t.Fatalf("token bound got %q want %q", got, want)
	}
	deep := strings.Repeat(`{"x":`, maxJSONDepth) + "null" + strings.Repeat("}", maxJSONDepth)
	depthFrame := strings.TrimSuffix(string(framedRequest(t, base)), "\n")
	depthFrame = strings.TrimSuffix(depthFrame, "}") + `,"future":` + deep + "}" + "\n"
	if got, want := string(Analyze([]byte(depthFrame))), sentinelGolden("LIMIT_EXCEEDED"); got != want {
		t.Fatalf("depth bound got %q want %q", got, want)
	}
}

func TestEveryGlobalBoundHasJustUnderAtAndFirstOverControl(t *testing.T) {
	base := fixtureInput("html-1", "html.document", "index.html", `<html><head></head><body></body></html>`)
	for _, size := range []int{maxJSONOrdinaryString - 1, maxJSONOrdinaryString, maxJSONOrdinaryString + 1} {
		r := request{Profile: strings.Repeat("p", size), Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []input{base}}
		got := string(Analyze(rawRequest(t, r)))
		want := fullFailureGolden(r, "NONCANONICAL_REQUEST")
		if size > maxJSONOrdinaryString {
			want = sentinelGolden("LIMIT_EXCEEDED")
		}
		if got != want {
			t.Fatalf("ordinary string=%d got=%q want=%q", size, got, want)
		}
	}
	for _, count := range []int{maxFeatures - 1, maxFeatures, maxFeatures + 1} {
		r := request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: identifiers(count)}, Inputs: []input{base}}
		got := safeEchoEnvelope(r)
		want := ""
		if count > maxFeatures {
			want = "LIMIT_EXCEEDED"
		}
		if got != want {
			t.Fatalf("features=%d got=%q want=%q", count, got, want)
		}
	}
	for _, count := range []int{maxInputs - 1, maxInputs, maxInputs + 1} {
		r := request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: inputs(count)}
		got := safeEchoEnvelope(r)
		want := ""
		if count > maxInputs {
			want = "LIMIT_EXCEEDED"
		}
		if got != want {
			t.Fatalf("inputs=%d got=%q want=%q", count, got, want)
		}
	}
	for _, size := range []int{maxContentBytes - 1, maxContentBytes, maxContentBytes + 1} {
		content := strings.Repeat("a", size)
		r := request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []input{fixtureInput("html-1", "html.document", "index.html", content)}}
		_, got := validateAndDecode(r)
		want := ""
		if size > maxContentBytes {
			want = "LIMIT_EXCEEDED"
		}
		if got != want {
			t.Fatalf("decoded=%d got=%q want=%q", size, got, want)
		}
	}
	for _, size := range []int{maxRequestBytes - 1, maxRequestBytes, maxRequestBytes + 1} {
		got := string(Analyze(bytes.Repeat([]byte("x"), size)))
		want := sentinelGolden("NONCANONICAL_REQUEST")
		if size > maxRequestBytes {
			want = sentinelGolden("LIMIT_EXCEEDED")
		}
		if got != want {
			t.Fatalf("request=%d got=%q want=%q", size, got, want)
		}
	}
	for _, count := range []int{maxJSONTokens - 1, maxJSONTokens, maxJSONTokens + 1} {
		scanner := jsonScanner{tokens: count}
		got := scanner.token()
		want := count < maxJSONTokens
		if got != want {
			t.Fatalf("tokens=%d got=%t want=%t", count, got, want)
		}
	}
	for _, depth := range []int{maxJSONDepth - 1, maxJSONDepth, maxJSONDepth + 1} {
		scanner := jsonScanner{data: []byte("{")}
		got := scanner.beginObject(depth)
		want := depth <= maxJSONDepth
		if got != want {
			t.Fatalf("depth=%d got=%t want=%t", depth, got, want)
		}
	}
	for _, total := range []int{maxOutputBytes - 1, maxOutputBytes, maxOutputBytes + 1} {
		got := withinOutputLimit(total, 0, 0)
		want := total <= maxOutputBytes
		if got != want {
			t.Fatalf("output=%d got=%t want=%t", total, got, want)
		}
	}
	if withinOutputLimit(maxOutputBytes, 1, 0) {
		t.Fatal("first output byte over the cap was retained")
	}
}

func TestBase64BoundJustUnderAtAndFirstOver(t *testing.T) {
	base := request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}}
	legalUnder := fixtureInput("html-1", "html.document", "index.html", strings.Repeat("a", maxContentBytes-1))
	if len(legalUnder.ContentBase64) != maxBase64Bytes-4 {
		t.Fatalf("legal under length=%d", len(legalUnder.ContentBase64))
	}
	paddedAt := fixtureInput("html-1", "html.document", "index.html", strings.Repeat("\x00", maxContentBytes))
	if len(paddedAt.ContentBase64) != maxBase64Bytes {
		t.Fatalf("padded at length=%d", len(paddedAt.ContentBase64))
	}
	for _, tc := range []struct {
		name     string
		encoded  string
		digest   string
		want     string
		sentinel bool
	}{
		{"legal-quartet-under", legalUnder.ContentBase64, legalUnder.SHA256, "UNSUPPORTED_SCHEMA", false},
		{"byte-under", strings.Repeat("A", maxBase64Bytes-1), "sha256:" + strings.Repeat("0", 64), "MALFORMED_INPUT", false},
		{"padded-at", paddedAt.ContentBase64, paddedAt.SHA256, "UNSUPPORTED_SCHEMA", false},
		{"at-unpadded-decoded-over", strings.Repeat("A", maxBase64Bytes), "sha256:" + strings.Repeat("0", 64), "LIMIT_EXCEEDED", false},
		{"first-over", strings.Repeat("A", maxBase64Bytes+1), "sha256:" + strings.Repeat("0", 64), "LIMIT_EXCEEDED", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			in := legalUnder
			in.ContentBase64 = tc.encoded
			in.SHA256 = tc.digest
			r.Inputs = []input{in}
			got := string(Analyze(rawRequest(t, r)))
			want := fullFailureGolden(r, tc.want)
			if tc.sentinel {
				want = sentinelGolden(tc.want)
			}
			if got != want {
				t.Fatalf("base64 length=%d got %q want %q", len(tc.encoded), got, want)
			}
		})
	}
}

func identifiers(count int) []string {
	values := make([]string, count)
	for i := range values {
		values[i] = fmt.Sprintf("f%03d", i)
	}
	return values
}

func inputs(count int) []input {
	values := make([]input, count)
	for i := range values {
		values[i] = fixtureInput(fmt.Sprintf("i%03d", i), "html.document", fmt.Sprintf("pages/%03d.html", i), "")
	}
	return values
}

func TestProspectiveFactAndOutputBounds(t *testing.T) {
	r := request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []input{fixtureInput("input-1", "html.document", "index.html", `<html><head></head><body></body></html>`)}}
	collector, reason := newFactCollector(r)
	if reason != "" {
		t.Fatal(reason)
	}
	collector.beginInput()
	for i := 0; i < maxFactsPerInput-1; i++ {
		if reason := collector.add(htmlFact("html.link.static", r.Inputs[0], "links", fmt.Sprintf("%04d", i))); reason != "" {
			t.Fatalf("per-input just-under=%q", reason)
		}
	}
	if reason := collector.add(htmlFact("html.link.static", r.Inputs[0], "links", "4095")); reason != "" {
		t.Fatalf("per-input at=%q", reason)
	}
	if got := collector.add(htmlFact("html.link.static", r.Inputs[0], "links", "over")); got != "LIMIT_EXCEEDED" {
		t.Fatalf("per-input first over=%q", got)
	}
	collector, reason = newFactCollector(r)
	if reason != "" {
		t.Fatal(reason)
	}
	collector.beginInput()
	for i := 0; i < maxFactsPerInput; i++ {
		if reason := collector.add(htmlFact("html.link.static", r.Inputs[0], "links", fmt.Sprintf("%04d", i))); reason != "" {
			t.Fatalf("per-input fact %d: %s", i, reason)
		}
	}
	if got := collector.add(htmlFact("html.link.static", r.Inputs[0], "links", "overflow.html")); got != "LIMIT_EXCEEDED" {
		t.Fatalf("per-input fact limit=%q", got)
	}
	globalCollector, reason := newFactCollector(r)
	if reason != "" {
		t.Fatal(reason)
	}
	for inputPart := 0; inputPart < 2; inputPart++ {
		globalCollector.beginInput()
		for i := 0; i < maxFacts/2; i++ {
			if reason := globalCollector.add(htmlFact("html.link.static", r.Inputs[0], "links", fmt.Sprintf("%d-%04d", inputPart, i))); reason != "" {
				t.Fatalf("global fact %d/%d: %s", inputPart, i, reason)
			}
		}
	}
	globalCollector.beginInput()
	if got := globalCollector.add(htmlFact("html.link.static", r.Inputs[0], "links", "global-overflow")); got != "LIMIT_EXCEEDED" {
		t.Fatalf("global fact limit=%q", got)
	}
	outputCollector, reason := newFactCollector(r)
	if reason != "" {
		t.Fatal(reason)
	}
	outputCollector.beginInput()
	large := strings.Repeat("x", maxJSONOrdinaryString)
	for {
		reason = outputCollector.add(htmlFact("html.link.static", r.Inputs[0], "links", large))
		if reason != "" {
			break
		}
	}
	if reason != "OUTPUT_LIMIT" || len(outputCollector.facts) == maxFacts {
		t.Fatalf("output bound reason=%q facts=%d", reason, len(outputCollector.facts))
	}
}

func TestTypedFactOrderingWorstCaseRatchet(t *testing.T) {
	const parentRevision = "404f2773b650749a0bb1c1c7bd4a9e241fece69e"
	parentRepository, reason := causalParentRepository(parentRevision)
	if reason != "" {
		t.Skip(reason)
	}
	parentSource, err := exec.Command("git", "-C", parentRepository, "show", parentRevision+":internal/analyzerhtmlcss/analyzer.go").Output()
	if err != nil {
		t.Fatalf("read causal parent: %v", err)
	}
	parentLoop := "for i := 1; i < len(facts); i++ {\n\t\tfor j := i; j > 0 && compareFact(facts[j], facts[j-1]) < 0; j-- {\n\t\t\tfacts[j], facts[j-1] = facts[j-1], facts[j]\n\t\t}\n\t}"
	if !strings.Contains(string(parentSource), parentLoop) {
		t.Fatal("recorded causal parent does not contain the restored insertion-sort control")
	}
	parentFacts := descendingFacts(maxFacts)
	parentComparisons := restoredParentInsertionSort(parentFacts)
	if parentComparisons <= 196608 {
		t.Fatalf("restored exact-parent negative control unexpectedly passed: %d", parentComparisons)
	}
	facts := descendingFacts(maxFacts)
	comparisons := sortFacts(facts)
	if comparisons > 196608 {
		t.Fatalf("production typed O(n log n) comparison ratchet failed: %d", comparisons)
	}
	t.Logf("production sortFacts comparisons=%d; restored-parent comparisons=%d", comparisons, parentComparisons)
	for i := 1; i < len(facts); i++ {
		if compareFact(facts[i-1], facts[i]) >= 0 {
			t.Fatal("typed ordering is not strict")
		}
	}
}

func causalParentRepository(revision string) (string, string) {
	rootOutput, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Sprintf("causal-parent fixture unavailable: cannot locate current Git checkout: %v", err)
	}
	repository := strings.TrimSpace(string(rootOutput))
	candidates := []string{repository}
	for root := filepath.Dir(repository); ; root = filepath.Dir(root) {
		candidates = append(candidates, filepath.Join(root, ".corvint-native-checkpoint"))
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
	}
	for _, candidate := range candidates {
		if exec.Command("git", "-C", candidate, "cat-file", "-e", revision+"^{commit}").Run() == nil {
			return candidate, ""
		}
	}
	return "", fmt.Sprintf("causal-parent fixture unavailable: commit %s is absent from the current checkout and ancestor .corvint-native-checkpoint repositories", revision)
}

func restoredParentInsertionSort(facts []fact) int {
	comparisons := 0
	for i := 1; i < len(facts); i++ {
		for j := i; j > 0 && restoredParentCompare(facts[j], facts[j-1], &comparisons) < 0; j-- {
			facts[j], facts[j-1] = facts[j-1], facts[j]
		}
	}
	return comparisons
}

func restoredParentCompare(left, right fact, comparisons *int) int {
	*comparisons++
	return compareFact(left, right)
}

func BenchmarkSortFactsWorstCase4096(b *testing.B) {
	seed := descendingFacts(maxFacts)
	work := make([]fact, len(seed))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		copy(work, seed)
		b.StartTimer()
		sortFacts(work)
	}
}

func descendingFacts(count int) []fact {
	facts := make([]fact, count)
	for i := range facts {
		facts[i] = fact{Kind: "html.asset.reference", InputHandle: "input-1", RelatedHandle: "-", Subject: "index.html", Predicate: "references-asset", Value: fmt.Sprintf("assets/%04d.png", count-i), InstanceID: "index.html", EvidenceSHA256: "sha256:" + strings.Repeat("0", 64)}
	}
	return facts
}
func quadraticComparisonCount(count int) int { return count * (count - 1) / 2 }
func rawRequest(t *testing.T, r request) []byte {
	t.Helper()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}
func sentinelGolden(reason string) string {
	return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"` + reason + `"}` + "\n"
}
func fullFailureGolden(r request, reason string) string {
	features := make([]string, len(r.Target.Features))
	for i, feature := range r.Target.Features {
		features[i] = `"` + feature + `"`
	}
	echoes := make([]string, len(r.Inputs))
	for i, in := range r.Inputs {
		echoes[i] = `{"handle":"` + in.Handle + `","family":"` + in.Family + `","path":"` + in.Path + `","sha256":"` + in.SHA256 + `"}`
	}
	return `{"profile":"` + r.Profile + `","family":"` + r.Family + `","request_id":"` + r.RequestID + `","status":"REJECTED","scope_id":"` + r.ScopeID + `","compilation_unit_id":"` + r.CompilationUnitID + `","target":{"os":"` + r.Target.OS + `","architecture":"` + r.Target.Architecture + `","abi":"` + r.Target.ABI + `","features":[` + strings.Join(features, ",") + `]},"input_echoes":[` + strings.Join(echoes, ",") + `],"reason":"` + reason + `"}` + "\n"
}

func benchmarkRequest() []byte {
	r := request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []input{
		fixtureInput("css-1", "css.stylesheet", "assets/one.css", `:root{--a:a;--b:b;--c:c;--d:d;--e:e;--f:f;--g:g;--h:h;}`),
		fixtureInput("css-2", "css.stylesheet", "assets/two.css", `@import "base.css";.x{--a:a;--b:b;--c:c;--d:d;--e:e;--f:f;--g:g;}`),
		fixtureInput("html-1", "html.document", "index.html", `<html><head></head><body><a href="a/1.html"></a><a href="a/2.html"></a><a href="a/3.html"></a><a href="a/4.html"></a><a href="a/5.html"></a><a href="a/6.html"></a><a href="a/7.html"></a><a href="a/8.html"></a></body></html>`),
		fixtureInput("html-2", "html.document", "other.html", `<html><head><link rel="stylesheet" href="a/1.css"><link rel="stylesheet" href="a/2.css"><link rel="stylesheet" href="a/3.css"><link rel="stylesheet" href="a/4.css"></head><body><script src="a/1.js" type="module"></script><script src="a/2.js" type="module"></script><script src="a/3.js" type="module"></script><script src="a/4.js" type="module"></script></body></html>`),
	}}
	b, _ := json.Marshal(r)
	return append(b, '\n')
}

func BenchmarkAnalyzeCandidate(b *testing.B) {
	request := benchmarkRequest()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Analyze(request)
	}
}
